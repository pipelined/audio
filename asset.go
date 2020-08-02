package audio

import (
	"io"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"
)

// Asset is a sink which uses a regular buffer as underlying storage.
// It can be used to make Clips and use them for processing input.
type Asset struct {
	sampleRate signal.SampleRate
	data       signal.Floating
}

// SignalAsset creates new asset with signal.Floating buffer data.
func SignalAsset(sampleRate signal.SampleRate, data signal.Floating) *Asset {
	return &Asset{
		sampleRate: sampleRate,
		data:       data,
	}
}

// Source implements clip source with a data from the asset.
func (a *Asset) Source(start, length int) pipe.SourceAllocatorFunc {
	return func(bufferSize int) (pipe.Source, pipe.SignalProperties, error) {
		return pipe.Source{
				SourceFunc: assetSource(a.data.Slice(start, start+length)),
			}, pipe.SignalProperties{
				Channels:   a.Channels(),
				SampleRate: a.SampleRate(),
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
		a.sampleRate = props.SampleRate
		a.data = signal.Allocator{
			Channels: props.Channels,
			Capacity: bufferSize,
		}.Float64()
		return pipe.Sink{
			SinkFunc: func(in signal.Floating) error {
				a.data = a.data.Append(in)
				return nil
			},
		}, nil
	}
}

// Data returns asset's data.
func (a *Asset) Data() signal.Floating {
	if a == nil {
		return nil
	}
	return a.data
}

// Channels returns a number of channels of the asset data.
func (a *Asset) Channels() int {
	if a == nil || a.data == nil {
		return 0
	}
	return a.data.Channels()
}

// SampleRate returns a sample rate of the asset.
func (a *Asset) SampleRate() signal.SampleRate {
	if a == nil {
		return 0
	}
	return a.sampleRate
}

// Clip represents a segment of an asset. It keeps reference to the
// asset, but doesn't copy its data.
type Clip struct {
	asset *Asset
	start int
	len   int
}

// Clip creates a new clip from the asset with defined start and length.
// If start position less than zero or more than asset's size, Clip with
// zero length is returned. If Clip len goes beyond the asset, it's
// truncated up to length of the asset.
func (a *Asset) Clip(start int, length int) Clip {
	// TODO: not allow empty clips
	dlen := a.data.Length()
	if a.data == nil || start >= dlen || start < 0 {
		return Clip{
			asset: a,
		}
	}
	end := start + length
	if end >= dlen {
		length = dlen - start
	}
	return Clip{
		asset: a,
		start: start,
		len:   length,
	}
}
