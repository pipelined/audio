package audio

import (
	"pipelined.dev/pipe"
	"pipelined.dev/signal"
)

// Asset is a sink which uses a regular buffer as underlying storage.
// It can be used to slice signals and use it for processing input.
type Asset struct {
	Signal     signal.Floating
	sampleRate signal.SampleRate
}

// SampleRate returns a sample rate of the asset.
func (a *Asset) SampleRate() signal.SampleRate {
	return a.sampleRate
}

// Sink appends buffers to asset.
func (a *Asset) Sink() pipe.SinkAllocatorFunc {
	return func(bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		a.sampleRate = props.SampleRate
		if a.Signal == nil {
			a.Signal = signal.Allocator{
				Channels: props.Channels,
				Capacity: bufferSize,
			}.Float64()
		}
		return pipe.Sink{
			SinkFunc: func(in signal.Floating) error {
				a.Signal = a.Signal.Append(in)
				return nil
			},
		}, nil
	}
}
