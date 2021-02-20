package audio

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

// Repeater sinks the signal and sources it to multiple pipelines.
type Repeater struct {
	m          sync.Mutex
	mut        mutable.Context
	bufferSize int
	sampleRate signal.Frequency
	channels   int
	sources    []chan *message
}

type message struct {
	buffer  signal.Floating
	sources int32
}

// Sink must be called once per repeater.
func (r *Repeater) Sink() pipe.SinkAllocatorFunc {
	return func(mut mutable.Context, bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		r.sampleRate = props.SampleRate
		r.channels = props.Channels
		r.bufferSize = bufferSize
		p := signal.GetPoolAllocator(props.Channels, bufferSize, bufferSize)
		return pipe.Sink{
			SinkFunc: func(in signal.Floating) error {
				r.m.Lock()
				defer r.m.Unlock()
				out := p.Float64()
				signal.FloatingAsFloating(in, out)
				for _, source := range r.sources {
					source <- &message{
						sources: int32(len(r.sources)),
						buffer:  out,
					}
				}
				return nil
			},
			FlushFunc: func(ctx context.Context) error {
				r.m.Lock()
				defer r.m.Unlock()
				for i := range r.sources {
					close(r.sources[i])
				}
				r.sources = nil
				return nil
			},
		}, nil
	}
}

// Source must be called at least once per repeater.
func (r *Repeater) Source() pipe.SourceAllocatorFunc {
	r.m.Lock()
	defer r.m.Unlock()
	source := make(chan *message, 1)
	r.sources = append(r.sources, source)
	return func(mut mutable.Context, bufferSize int) (pipe.Source, error) {
		p := signal.GetPoolAllocator(r.channels, bufferSize, bufferSize)
		var (
			messagePtr *message
			ok         bool
		)
		return pipe.Source{
				SourceFunc: func(b signal.Floating) (int, error) {
					messagePtr, ok = <-source
					if !ok {
						return 0, io.EOF
					}
					read := signal.FloatingAsFloating(messagePtr.buffer, b)
					if atomic.AddInt32(&messagePtr.sources, -1) == 0 {
						messagePtr.buffer.Free(p)
					}
					return read, nil
				},
				SignalProperties: pipe.SignalProperties{
					SampleRate: r.sampleRate,
					Channels:   r.channels,
				},
			},
			nil
	}
}
