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
	initialize   sync.Once
	sampleRate   signal.Frequency
	channels     int
	inputSignals chan inputSignal
	output       chan signal.Floating
	current      int
	frames

	inputs []input
}

const (
	flushed   = -1
	numFrames = len(frames{})
)

type (
	frames [2]frame

	// frame represents a slice of samples to mix.
	frame struct {
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
		frame int
		sema  semaphore.Semaphore
	}
)

func (m *Mixer) init(sampleRate signal.Frequency, channels int) func() {
	return func() {
		m.channels = channels
		m.sampleRate = sampleRate
		m.inputSignals = make(chan inputSignal, 1)
	}
}

func mustAfterSink() {
	panic("mixer source bound before sink")
}

// Source provides mixer source allocator. Mixer source outputs mixed
// signal. Only single source per mixer is allowed. Must be called after
// Sink, otherwise will panic.
func (m *Mixer) Source() pipe.SourceAllocatorFunc {
	return func(mut mutable.Context, bufferSize int) (pipe.Source, error) {
		m.initialize.Do(mustAfterSink) // check that source is bound after sink.
		pool := signal.GetPoolAllocator(m.channels, bufferSize, bufferSize)
		m.frames[0].buffer = pool.GetFloat64()
		return pipe.Source{
			Output: pipe.SignalProperties{
				Channels:   m.channels,
				SampleRate: m.sampleRate,
			},
			StartFunc: func(ctx context.Context) error {
				m.output = make(chan signal.Floating, 1)
				go mix(ctx, m.frames, pool, m.inputs, m.inputSignals, m.output)
				// this is needed to enable garbage collection
				return nil
			},
			SourceFunc: func(out signal.Floating) (int, error) {
				if sum, ok := <-m.output; ok {
					defer sum.Free(pool)
					return signal.FloatingAsFloating(sum, out), nil
				}
				return 0, io.EOF
			},
		}, nil
	}
}

func mix(ctx context.Context, frames frames, pool *signal.PoolAllocator, inputs []input, inputSignals <-chan inputSignal, output chan<- signal.Floating) {
	defer close(output)
	sinking := len(inputs)
	head := 0
	for sinking > 0 {
		var is inputSignal
		select {
		case is = <-inputSignals:
		case <-ctx.Done():
			return
		}
		input := &inputs[is.input]
		frame := &frames[input.frame]

		// flush the signal
		if is.buffer == nil {
			sinking--
			for {
				frame.flushed++
				if frame.sum() {
					frame.release(inputs)
					select {
					case output <- frame.buffer:
						frame.buffer = nil
					case <-ctx.Done():
						return
					}
				}
				if input.frame == head {
					input.frame = flushed
					break
				}
				input.frame = frames.next(input.frame)
				frame = &frames[input.frame]
			}
			continue
		}

		frame.add(is.buffer)
		if frame.sum() {
			frame.release(inputs)
			select {
			case output <- frame.buffer:
				frame.buffer = nil
			case <-ctx.Done():
				return
			}
		}
		next := frames.next(input.frame)
		// is this input first to proceed to the next frame
		if input.frame == head {
			frames[next].expected = frames[head].expected - frames[head].flushed
			frames[next].flushed = 0
			frames[next].length = 0
			frames[next].buffer = pool.GetFloat64()
			head = next
		}
		input.frame = next
	}
}

// Sink provides mixer sink allocator. Mixer sink receives a signal for
// mixing. Multiple sinks per mixer is allowed.
func (m *Mixer) Sink() pipe.SinkAllocatorFunc {
	return func(mut mutable.Context, bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		m.initialize.Do(m.init(props.SampleRate, props.Channels))
		if m.sampleRate != props.SampleRate {
			return pipe.Sink{}, ErrDifferentSampleRates
		}
		if m.channels != props.Channels {
			return pipe.Sink{}, ErrDifferentChannels
		}
		sema := semaphore.New(numFrames - 1)
		m.inputs = append(m.inputs,
			input{
				frame: 0,
				sema:  sema,
			},
		)
		input := len(m.inputs) - 1
		m.frames[0].expected++
		var startCtx context.Context
		return pipe.Sink{
			StartFunc: func(ctx context.Context) error {
				startCtx = ctx
				return nil
			},
			SinkFunc: func(floats signal.Floating) error {
				// sink new buffer
				select {
				case m.inputSignals <- inputSignal{input: input, buffer: floats}:
					sema.Acquire(startCtx)
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
	if f.buffer.Len() != f.length {
		// fmt.Printf("slice!")
		f.buffer = f.buffer.Slice(0, f.length/f.buffer.Channels())
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
	if f.length < in.Len() {
		f.length = in.Len()
	}
	return
}

func (f *frame) release(inputs []input) {
	// f.expected = 0
	f.added = 0
	f.flushed = 0
	for i := 0; i < f.expected; i++ {
		if input := inputs[i]; input.frame != flushed {
			input.sema.Release()
		}
	}
}

func (f frames) next(c int) int {
	next := c + 1
	if next == len(f) {
		return 0
	}
	return next
}
