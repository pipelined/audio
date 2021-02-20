package audio_test

import (
	"context"
	"math"
	"testing"

	"pipelined.dev/audio"
	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/signal"
)

func TestSource(t *testing.T) {
	alloc := signal.Allocator{
		Channels: 1,
		Length:   3,
		Capacity: 3,
	}
	floats := alloc.Float64()
	signal.WriteStripedFloat64([][]float64{{-1, 0, 1}}, floats)
	ints := alloc.Int64(signal.MaxBitDepth)
	signal.WriteStripedInt64([][]int64{{math.MinInt64, 0, math.MaxInt64}}, ints)
	uints := alloc.Uint64(signal.MaxBitDepth)
	signal.WriteStripedUint64([][]uint64{{0, math.MaxInt64 + 1, math.MaxUint64}}, uints)

	sampleRate := signal.Frequency(44100)
	tests := []struct {
		source   pipe.SourceAllocatorFunc
		expected []float64
		msg      string
	}{
		{
			source:   audio.Source(sampleRate, floats),
			expected: []float64{-1, 0, 1},
			msg:      "Floats full asset",
		},
		{
			source:   audio.Source(sampleRate, ints),
			expected: []float64{-1, 0, 1},
			msg:      "Ints full asset",
		},
		{
			source:   audio.Source(sampleRate, uints),
			expected: []float64{-1, 0, 1},
			msg:      "Ints full asset",
		},
	}

	bufferSize := 2
	for _, test := range tests {
		sink := mock.Sink{}

		p, _ := pipe.New(bufferSize,
			pipe.Line{
				Source: test.source,
				Sink:   sink.Sink(),
			},
		)
		_ = pipe.Wait(p.Start(context.Background()))

		result := make([]float64, sink.Values.Len())
		signal.ReadFloat64(sink.Values, result)

		assertEqual(t, test.msg, result, test.expected)
	}

}
