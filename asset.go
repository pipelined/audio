package audio

import (
	"fmt"
	"io"

	"github.com/pipelined/signal"
)

// Asset is a sink which uses a regular buffer as underlying storage.
// It can be used to make Clips and use them for processing input.
type Asset struct {
	sampleRate signal.SampleRate
	data       signal.Float64
}

// SignalAsset creates new asset from signal.Float64 buffer.
func SignalAsset(sampleRate signal.SampleRate, data signal.Float64) *Asset {
	return &Asset{
		sampleRate: sampleRate,
		data:       data,
	}
}

// Sink appends buffers to asset.
func (a *Asset) Sink(sourceID string, sampleRate signal.SampleRate, numChannels int) (func(signal.Float64) error, error) {
	a.sampleRate = signal.SampleRate(sampleRate)
	return func(b signal.Float64) error {
		a.data = a.data.Append(b)
		return nil
	}, nil
}

// Data returns asset's data.
func (a *Asset) Data() signal.Float64 {
	if a == nil {
		return nil
	}
	return a.data
}

// NumChannels returns a number of channels of the asset data.
func (a *Asset) NumChannels() int {
	if a == nil || a.data == nil {
		return 0
	}
	return a.data.NumChannels()
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
func (a *Asset) Clip(start int, len int) Clip {
	size := a.data.Size()
	if a.data == nil || start >= size || start < 0 {
		return Clip{
			asset: a,
		}
	}
	end := start + len
	if end >= size {
		len = size - start
	}
	return Clip{
		asset: a,
		start: start,
		len:   len,
	}
}

// Pump implements clip pump with a data from the asset.
func (c Clip) Pump(sourceID string) (func(signal.Float64) error, signal.SampleRate, int, error) {
	if c.asset == nil || c.asset.Data() == nil {
		return nil, 0, 0, fmt.Errorf("clip refers to empty asset")
	}
	// position where read starts.
	pos := c.start
	// end of clip.
	end := c.start + c.len
	return func(b signal.Float64) error {
		if pos >= end {
			return io.EOF
		}
		// not enough samples left to make full read, trim.
		if end-pos < b.Size() {
			for i := range b {
				b[i] = b[i][:end-pos]
			}
		}

		// copy values.
		for i := range b {
			copy(b[i], c.asset.data[i][pos:])
		}
		pos += b.Size()
		return nil
	}, c.asset.SampleRate(), c.asset.NumChannels(), nil
}
