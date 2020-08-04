package audio

import (
	"io"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"
)

// Asset is a sink which uses a regular buffer as underlying storage.
// It can be used to slice signals and use it for processing input.
type Asset struct {
	signal.SampleRate
	signal.Floating
}

// Source implements clip source with a data from the asset.
func (a Asset) Source(start, end int) pipe.SourceAllocatorFunc {
	return func(bufferSize int) (pipe.Source, pipe.SignalProperties, error) {
		return pipe.Source{
				SourceFunc: assetSource(a.Floating.Slice(start, end)),
			}, pipe.SignalProperties{
				Channels:   a.Channels(),
				SampleRate: a.SampleRate,
			}, nil
	}
}

func assetSource(data signal.Floating) pipe.SourceFunc {
	pos := 0
	return func(out signal.Floating) (int, error) {
		if pos == data.Len() {
			return 0, io.EOF
		}
		end := pos + out.Len()
		if end > data.Len() {
			end = data.Len()
		}
		read := 0
		for pos < end {
			out.SetSample(read, data.Sample(pos))
			read++
			pos++
		}
		return read / data.Channels(), nil
	}
}

// Sink appends buffers to asset.
func (a *Asset) Sink() pipe.SinkAllocatorFunc {
	return func(bufferSize int, props pipe.SignalProperties) (pipe.Sink, error) {
		a.SampleRate = props.SampleRate
		a.Floating = signal.Allocator{
			Channels: props.Channels,
			Capacity: bufferSize,
		}.Float64()
		return pipe.Sink{
			SinkFunc: func(in signal.Floating) error {
				a.Floating = a.Floating.Append(in)
				return nil
			},
		}, nil
	}
}
