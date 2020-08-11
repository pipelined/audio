package audio

import (
	"context"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"
)

// Asset is a sink which uses a regular buffer as underlying storage. It
// can be used to slice signal data and use it as processing input. Buffer type can
type Asset struct {
	signal.Signal
	sampleRate signal.SampleRate
}

// SampleRate returns a sample rate of the asset.
func (a *Asset) SampleRate() signal.SampleRate {
	return a.sampleRate
}

// Sink uses signal.Floating buffer to store signal data.
func (a *Asset) Sink() pipe.SinkAllocatorFunc {
	switch a.Signal.(type) {
	case signal.Signed:
		return a.sinkSigned()
	case signal.Unsigned:
	default:
		return a.sinkFloating()
	}
	return nil
}

func (a *Asset) sinkFloating() pipe.SinkAllocatorFunc {
	return func(bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		a.sampleRate = props.SampleRate
		data := floatingAsset(a.Signal, props.Channels, bufferSize)
		return pipe.Sink{
			SinkFunc: func(in signal.Floating) error {
				data = data.Append(in)
				return nil
			},
			FlushFunc: func(context.Context) error {
				a.Signal = data
				return nil
			},
		}, nil
	}
}

// floatingAsset returns preallocated bufer if provided otherwise allocates new.
func floatingAsset(s signal.Signal, channels, bufferSize int) signal.Floating {
	if s != nil {
		return s.(signal.Floating)
	}
	return signal.Allocator{
		Channels: channels,
		Capacity: bufferSize,
	}.Float64()
}

func (a *Asset) sinkSigned() pipe.SinkAllocatorFunc {
	return func(bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		a.sampleRate = props.SampleRate
		data := a.Signal.(signal.Signed)
		// increment buffer is used only to grow the capacity of the data slice
		inc := signal.Allocator{
			Channels: props.Channels,
			Capacity: bufferSize,
			Length:   bufferSize,
		}.Int8(signal.MaxBitDepth)
		pos := 0
		return pipe.Sink{
			SinkFunc: func(in signal.Floating) error {
				data = data.Append(inc)
				pos += signal.FloatingAsSigned(in, data.Slice(pos, pos+bufferSize))
				return nil
			},
			FlushFunc: func(context.Context) error {
				a.Signal = data
				return nil
			},
		}, nil
	}
}

func (a *Asset) sinkUnsigned() pipe.SinkAllocatorFunc {
	return func(bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		a.sampleRate = props.SampleRate
		data := a.Signal.(signal.Unsigned)
		// increment buffer is used only to grow the capacity of the data slice
		inc := signal.Allocator{
			Channels: props.Channels,
			Capacity: bufferSize,
			Length:   bufferSize,
		}.Uint8(signal.MaxBitDepth)
		pos := 0
		return pipe.Sink{
			SinkFunc: func(in signal.Floating) error {
				data = data.Append(inc)
				pos += signal.FloatingAsUnsigned(in, data.Slice(pos, pos+bufferSize))
				return nil
			},
			FlushFunc: func(context.Context) error {
				a.Signal = data
				return nil
			},
		}, nil
	}
}
