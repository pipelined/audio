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

// Mixer summs up multiple signals. It has multiple sinks and a single
// source. Recommended size for input buffer is expected number of sinks
// the mixer will have. Default value is two.
type Mixer struct {
	InputBuffer int
	initialize  sync.Once
	sampleRate  signal.Frequency
	channels    int
	input       chan signal.Floating
	mut         inversedRWMutex
	cond        sync.Cond
	mix         frame
}

type (
	// frame represents a slice of samples to mix.
	frame struct {
		buffer   signal.Floating
		expected int
		added    int
	}

	// inversed RW mutex, i.e. defaul Lock/Unlock methods are read-level
	// and new methods Wlock/Wunlock are write-level.
	inversedRWMutex sync.RWMutex
)

func (m *inversedRWMutex) Lock() {
	(*sync.RWMutex)(m).RLock()
}

func (m *inversedRWMutex) Unlock() {
	(*sync.RWMutex)(m).RUnlock()
}

func (m *inversedRWMutex) Wlock() {
	(*sync.RWMutex)(m).Lock()
}

func (m *inversedRWMutex) Wunlock() {
	(*sync.RWMutex)(m).Unlock()
}

func (m *Mixer) init(sampleRate signal.Frequency, channels int) func() {
	return func() {
		m.mut = inversedRWMutex{}
		m.cond.L = &m.mut
		m.channels = channels
		m.sampleRate = sampleRate
		if m.InputBuffer == 0 {
			m.InputBuffer = defaultInputBuffer
		}
		m.input = make(chan signal.Floating, m.InputBuffer)
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
		m.mix.expected++
		var startCtx context.Context
		return pipe.Sink{
			StartFunc: func(ctx context.Context) error {
				startCtx = ctx
				return nil
			},
			SinkFunc: func(floats signal.Floating) error {
				// sink new buffer
				m.mut.Lock()
				select {
				case m.input <- floats:
					m.cond.Wait()
					m.mut.Unlock()
					return nil
				case <-startCtx.Done():
					m.mut.Unlock()
					return nil
				}
			},
			FlushFunc: func(ctx context.Context) error {
				select {
				case m.input <- nil:
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
					m.mut.Wlock()
					m.mix.added = 0
					m.mix.buffer = m.mix.buffer.Slice(0, 0)
					m.cond.Broadcast()
					m.mut.Wunlock()
					return read, nil
				}
				return 0, io.EOF
			},
			FlushFunc: func(ctx context.Context) error {
				m.mut.Wlock()
				m.cond.Broadcast()
				m.mut.Wunlock()
				return nil
			},
		}, nil
	}
}

func (m *Mixer) source(ctx context.Context, out signal.Floating) (int, bool) {
	var in signal.Floating
	for {
		if m.mix.expected == 0 {
			return 0, false
		}
		select {
		case in = <-m.input:
		case <-ctx.Done():
			return 0, false
		}

		if in != nil {
			m.mix.add(in)
		} else {
			m.mix.expected--
		}
		if m.mix.sum() {
			return signal.FloatingAsFloating(m.mix.buffer, out), true
		}
	}
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
