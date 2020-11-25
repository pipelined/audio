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

// Mixer summs up multiple signals. It has multiple sinks and a single
// source.
type Mixer struct {
	initialize   sync.Once
	sampleRate   signal.Frequency
	channels     int
	inputSignals chan inputSignal
	// output       chan signal.Floating
	// current      int
	// frames
	cond   sync.Cond
	mix    frame
	inputs int
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
	}

	inputSignal struct {
		input  int
		buffer signal.Floating
	}

	input struct {
		frame int
		// sema  semaphore.Semaphore
	}
)

func (m *Mixer) init(sampleRate signal.Frequency, channels int) func() {
	return func() {
		m.cond.L = &sync.Mutex{}
		m.channels = channels
		m.sampleRate = sampleRate
		m.inputSignals = make(chan inputSignal, 1)
	}
}

func mustAfterSink() {
	panic("mixer source bound before sink")
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
		m.inputs++
		input := m.inputs
		// m.cond.L.Lock()
		m.mix.expected++
		// m.cond.L.Unlock()
		var startCtx context.Context
		return pipe.Sink{
			StartFunc: func(ctx context.Context) error {
				startCtx = ctx
				return nil
			},
			SinkFunc: func(floats signal.Floating) error {
				// sink new buffer
				m.cond.L.Lock()
				select {
				case m.inputSignals <- inputSignal{input: input, buffer: floats}:
					// sema.Acquire(startCtx)
				case <-startCtx.Done():
					return nil
				}
				// for !m.mix.ready() {
				m.cond.Wait()
				// }
				m.cond.L.Unlock()
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

// Source provides mixer source allocator. Mixer source outputs mixed
// signal. Only single source per mixer is allowed. Must be called after
// Sink, otherwise will panic.
func (m *Mixer) Source() pipe.SourceAllocatorFunc {
	return func(mut mutable.Context, bufferSize int) (pipe.Source, error) {
		m.initialize.Do(mustAfterSink) // check that source is bound after sink.
		pool := signal.GetPoolAllocator(m.channels, bufferSize, bufferSize)
		m.mix.buffer = pool.GetFloat64()
		var startCtx context.Context
		return pipe.Source{
			Output: pipe.SignalProperties{
				Channels:   m.channels,
				SampleRate: m.sampleRate,
			},
			StartFunc: func(ctx context.Context) error {
				startCtx = ctx
				return nil
			},
			SourceFunc: func(out signal.Floating) (int, error) {
				if read, ok := m.source(startCtx, out); ok {
					m.cond.L.Lock()
					m.mix.added = 0
					m.mix.buffer = m.mix.buffer.Slice(0, 0)
					m.cond.Broadcast()
					m.cond.L.Unlock()
					return read, nil
				}
				return 0, io.EOF
			},
			FlushFunc: func(ctx context.Context) error {
				m.cond.Broadcast()
				return nil
			},
		}, nil
	}
}

func (m *Mixer) source(ctx context.Context, out signal.Floating) (int, bool) {
	var (
		is inputSignal
		ok bool
	)
	for {
		select {
		case is, ok = <-m.inputSignals:
		case <-ctx.Done():
			return 0, false
		}
		if !ok {
			return 0, false
		}

		if is.buffer != nil {
			m.mix.add(is.buffer)
		} else {
			m.mix.expected--
			if m.mix.expected == 0 {
				close(m.inputSignals)
			}
			// continue
		}
		if m.mix.sum() {
			return signal.FloatingAsFloating(m.mix.buffer, out), true
		}
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
	if f.added != f.expected {
		return false
	}
	for i := 0; i < f.buffer.Len(); i++ {
		f.buffer.SetSample(i, f.buffer.Sample(i)/float64(f.added))
	}
	return true
}

func (f *frame) add(in signal.Floating) {
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

// func (f *frame) release(inputs []input) {
// 	// f.expected = 0
// 	f.added = 0
// 	f.flushed = 0
// 	for i := 0; i < f.expected; i++ {
// 		if input := inputs[i]; input.frame != flushed {
// 			// input.sema.Release()
// 		}
// 	}
// }

func (f frames) next(c int) int {
	next := c + 1
	if next == len(f) {
		return 0
	}
	return next
}
