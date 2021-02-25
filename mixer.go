package audio

import (
	"context"
	"errors"
	"io"
	"sync"

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

// buffer size for input channel. since we only mix single frame at the
// time, it's just to prevent from blocking on select for too long.
const defaultInputBuffer = 2

type (
	// Mixer summs up multiple signals. It has multiple sinks and a single
	// source.
	Mixer struct {
		// InputBuffer int
		initialize sync.Once
		sampleRate signal.Frequency
		channels   int
		inputs     []*mixerInput
		pool       *signal.PoolAllocator
	}
	// mixerOutput represents a slice of samples to mix.
	mixerOutput struct {
		buffer signal.Floating
		added  int
	}

	mixerInput struct {
		write  chan struct{}
		read   chan struct{}
		buffer signal.Floating
	}

	chanMutex chan struct{}
)

func newMixerInput(buf signal.Floating) mixerInput {
	write := make(chan struct{}, 1)
	write <- struct{}{}
	read := make(chan struct{}, 1)
	return mixerInput{
		write:  write,
		read:   read,
		buffer: buf,
	}
}

func wait(ctx context.Context, m chanMutex) bool {
	select {
	case <-ctx.Done():
		return false
	case _, ok := <-m:
		return ok
	}
}

func notify(ctx context.Context, m chanMutex) bool {
	select {
	case <-ctx.Done():
		return false
	case m <- struct{}{}:
		return true
	}
}

func (m *Mixer) init(sampleRate signal.Frequency, channels, bufferSize int) func() {
	return func() {
		m.channels = channels
		m.sampleRate = sampleRate
		m.pool = signal.GetPoolAllocator(channels, bufferSize, bufferSize)
	}
}

func mustAfterSink() {
	panic("mixer source bound before sink")
}

// Sink provides mixer sink allocator. Mixer sink receives a signal for
// mixing. Multiple sinks per mixer is allowed.
func (m *Mixer) Sink() pipe.SinkAllocatorFunc {
	return func(mut mutable.Context, bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		m.initialize.Do(m.init(props.SampleRate, props.Channels, bufferSize))
		if m.sampleRate != props.SampleRate {
			return pipe.Sink{}, ErrDifferentSampleRates
		}
		if m.channels != props.Channels {
			return pipe.Sink{}, ErrDifferentChannels
		}
		input := newMixerInput(m.pool.Float64())
		m.inputs = append(m.inputs, &input)
		var sinkCtx context.Context
		return pipe.Sink{
			StartFunc: func(ctx context.Context) error {
				sinkCtx = ctx
				return nil
			},
			SinkFunc: func(floats signal.Floating) error {
				if ok := wait(sinkCtx, input.write); !ok {
					return nil
				}
				n := signal.FloatingAsFloating(floats, input.buffer)
				if n != bufferSize {
					input.buffer = input.buffer.Slice(0, n)
				}
				notify(sinkCtx, input.read)
				return nil
			},
			FlushFunc: func(ctx context.Context) error {
				close(input.read)
				return nil
			},
		}, nil
	}
}

// Source provides mixer source allocator. Mixer source outputs mixed
// signal. Only single source per mixer is allowed. Must be called after
// Sink, otherwise will panic.
func (m *Mixer) Source() pipe.SourceAllocatorFunc {
	return func(mut mutable.Context, bufferSize int) (pipe.Source, error) {
		m.initialize.Do(mustAfterSink) // check that source is bound after sink.
		output := &mixerOutput{buffer: m.pool.Float64()}
		var sourceCtx context.Context
		return pipe.Source{
			SignalProperties: pipe.SignalProperties{
				Channels:   m.channels,
				SampleRate: m.sampleRate,
			},
			StartFunc: func(ctx context.Context) error {
				sourceCtx = ctx
				return nil
			},
			SourceFunc: func(out signal.Floating) (int, error) {
				for i := 0; i < len(m.inputs); {
					if ok := wait(sourceCtx, m.inputs[i].read); !ok {
						m.inputs[i].buffer.Free(m.pool)
						m.inputs = append(m.inputs[:i], m.inputs[i+1:]...)
						continue
					}
					output.add(m.inputs[i].buffer)
					notify(sourceCtx, m.inputs[i].write)
					i++
				}
				if len(m.inputs) == 0 {
					return 0, io.EOF
				}
				output.sum()
				n := signal.FloatingAsFloating(output.buffer, out)
				output.buffer = output.buffer.Slice(0, 0)
				output.added = 0
				return n, nil
			},
			FlushFunc: func(ctx context.Context) error {
				output.buffer.Free(m.pool)
				return nil
			},
		}, nil
	}
}

// sum returns mixed samplein.
func (f *mixerOutput) sum() {
	for i := 0; i < f.buffer.Len(); i++ {
		f.buffer.SetSample(i, f.buffer.Sample(i)/float64(f.added))
	}
}

func (f *mixerOutput) add(in signal.Floating) {
	f.added++
	if f.buffer.Len() == 0 {
		for i := 0; i < in.Len(); i++ {
			f.buffer.AppendSample(in.Sample(i))
		}
		return
	}

	if f.buffer.Len() >= in.Len() {
		for i := 0; i < in.Len(); i++ {
			f.buffer.SetSample(i, f.buffer.Sample(i)+in.Sample(i))
		}
		return
	}

	for i := 0; i < f.buffer.Len(); i++ {
		f.buffer.SetSample(i, f.buffer.Sample(i)+in.Sample(i))
	}
	for i := f.buffer.Len(); i < in.Len(); i++ {
		f.buffer.AppendSample(in.Sample(i))
	}
}
