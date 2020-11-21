package audio

import (
	"context"
	"errors"
	"io"
	"sync"

	"pipelined.dev/audio/internal/semaphore"
	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

var (
	// ErrDifferentSampleRates is returned when signals with different
	// sample rates are sinked into mixer.
	ErrDifferentSampleRates = errors.New("sinking different sample rates")
	// ErrDifferentChannels is returned when signals with different number
	// of channels are sinked into mixer.
	ErrDifferentChannels = errors.New("sinking different channels")
)

// Mixer summs up multiple signals. It has multiple sinks and a single
// source.
type Mixer struct {
	once         sync.Once
	sampleRate   signal.Frequency
	channels     int
	inputSignals chan inputSignal
	output       chan signal.Floating
	head         *frame

	pool   *signal.PoolAllocator
	inputs []input
}

type (
	// frame represents a slice of samples to mix.
	frame struct {
		next     *frame
		buffer   signal.Floating
		expected int
		added    int
		flushed  int
		length   int // https://github.com/pipelined/mixer/issues/5
	}

	inputSignal struct {
		input  int
		buffer signal.Floating
	}

	input struct {
		*frame
		sema semaphore.Semaphore
	}
)

func (m *Mixer) init(sampleRate signal.Frequency, channels int) func() {
	return func() {
		m.channels = channels
		m.sampleRate = sampleRate
		m.head = &frame{}
		m.inputSignals = make(chan inputSignal, 1)
	}
}

func mustInitialized() {
	panic("mixer sink must be bound before source")
}

// Source provides mixer source allocator. Mixer source outputs mixed
// signal. Only single source per mixer is allowed. Must be called after
// Sink, otherwise will panic.
func (m *Mixer) Source() pipe.SourceAllocatorFunc {
	return func(mut mutable.Context, bufferSize int) (pipe.Source, error) {
		m.once.Do(mustInitialized) // check that source is bound after sink.
		m.pool = signal.GetPoolAllocator(m.channels, bufferSize, bufferSize)
		return pipe.Source{
			Output: pipe.SignalProperties{
				Channels:   m.channels,
				SampleRate: m.sampleRate,
			},
			StartFunc: func(ctx context.Context) error {
				m.output = make(chan signal.Floating, 1)
				go mixer(ctx, m.pool, m.inputs, m.inputSignals, m.output)
				// this is needed to enable garbage collection
				m.inputs = nil
				m.head = nil
				return nil
			},
			SourceFunc: func(out signal.Floating) (int, error) {
				if sum, ok := <-m.output; ok {
					defer sum.Free(m.pool)
					return signal.FloatingAsFloating(sum, out), nil
				}
				return 0, io.EOF
			},
		}, nil
	}
}

func mixer(ctx context.Context, pool *signal.PoolAllocator, inputs []input, inputSignals <-chan inputSignal, output chan<- signal.Floating) {
	defer close(output)
	sinking := len(inputs)
	for sinking > 0 {
		var is inputSignal
		select {
		case is = <-inputSignals:
		case <-ctx.Done():
			return
		}
		input := inputs[is.input]

		// flush the signal
		if is.buffer == nil {
			inputs[is.input].frame = nil
			sinking--
			for current := input; current.frame != nil; current.frame = current.frame.next {
				current.flushed++
				if current.sum() {
					select {
					case output <- current.buffer:
					case <-ctx.Done():
						return
					}
				}
			}
			continue
		}

		if input.buffer == nil {
			input.buffer = pool.GetFloat64()
		}
		input.add(is.buffer)
		is.buffer.Free(pool)
		if input.sum() {
			for i := 0; i < input.expected; i++ {
				if input := inputs[i]; input.frame != nil {
					input.sema.Release()
				}
			}
			select {
			case output <- input.buffer:
			case <-ctx.Done():
				return
			}
		}
		if input.next == nil {
			// flushed sinks are not expected anymore
			input.next = &frame{
				expected: input.expected - input.flushed,
			}
		}
		inputs[is.input].frame = input.next
	}
}

// Sink provides mixer sink allocator. Mixer sink receives a signal for
// mixing. Multiple sinks per mixer is allowed.
func (m *Mixer) Sink() pipe.SinkAllocatorFunc {
	return func(mut mutable.Context, bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		m.once.Do(m.init(props.SampleRate, props.Channels))
		if m.sampleRate != props.SampleRate {
			return pipe.Sink{}, ErrDifferentSampleRates
		}
		if m.channels != props.Channels {
			return pipe.Sink{}, ErrDifferentChannels
		}
		sema := semaphore.New(2)
		m.inputs = append(m.inputs, input{
			frame: m.head,
			sema:  sema,
		})
		input := len(m.inputs) - 1
		m.head.expected++
		var startCtx context.Context
		return pipe.Sink{
			StartFunc: func(ctx context.Context) error {
				startCtx = ctx
				return nil
			},
			SinkFunc: func(floats signal.Floating) error {
				// sink new buffer
				sema.Acquire(startCtx)
				buf := m.pool.GetFloat64()
				copied := signal.FloatingAsFloating(floats, buf)
				if copied != buf.Length() {
					buf = buf.Slice(0, copied)
				}
				select {
				case m.inputSignals <- inputSignal{input: input, buffer: buf}:
				case <-startCtx.Done():
					return nil
				}

				return nil
			},
			FlushFunc: func(ctx context.Context) error {
				select {
				case m.inputSignals <- inputSignal{input: input}:
				case <-ctx.Done():
					return nil
				}
				return nil
			},
		}, nil
	}
}

func min(n1, n2 int) int {
	if n1 < n2 {
		return n1
	}
	return n2
}

// sum returns mixed samplein.
func (f *frame) sum() bool {
	if f.added == 0 || f.added+f.flushed != f.expected {
		return false
	}
	if f.buffer.Length() != f.length {
		f.buffer = f.buffer.Slice(0, f.length)
	}
	for i := 0; i < f.buffer.Len(); i++ {
		f.buffer.SetSample(i, f.buffer.Sample(i)/float64(f.added))
	}
	return true
}

func (f *frame) add(in signal.Floating) {
	f.added++
	l := min(f.buffer.Len(), in.Len())
	for i := 0; i < l; i++ {
		f.buffer.SetSample(i, f.buffer.Sample(i)+in.Sample(i))
	}
	if f.length < in.Length() {
		f.length = in.Length()
	}
	return
}
