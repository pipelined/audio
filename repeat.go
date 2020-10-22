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
	sources    []chan message
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
				for _, source := range r.sources {
					out := p.GetFloat64()
					signal.FloatingAsFloating(in, out)
					source <- message{
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
	source := make(chan message, 1)
	r.sources = append(r.sources, source)
	return func(mut mutable.Context, bufferSize int) (pipe.Source, error) {
		p := signal.GetPoolAllocator(r.channels, bufferSize, bufferSize)
		var (
			message message
			ok      bool
		)
		return pipe.Source{
				SourceFunc: func(b signal.Floating) (int, error) {
					message, ok = <-source
					if !ok {
						return 0, io.EOF
					}
					read := signal.FloatingAsFloating(message.buffer, b)
					if atomic.AddInt32(&message.sources, -1) == 0 {
						message.buffer.Free(p)
					}
					return read, nil
				},
				Output: pipe.SignalProperties{
					SampleRate: r.sampleRate,
					Channels:   r.channels,
				},
			},
			nil
	}
}
