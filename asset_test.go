package audio_test

import (
	"context"
	"testing"

	"pipelined.dev/audio"
	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/signal"
)

func TestAssetSink(t *testing.T) {
	sampleRate := signal.SampleRate(44100)
	tests := []struct {
		source      pipe.SourceAllocatorFunc
		numChannels int
		samples     int
	}{
		{
			source: (&mock.Source{
				Channels:   1,
				Value:      0.5,
				Limit:      100,
				SampleRate: sampleRate,
			}).Source(),
			numChannels: 1,
			samples:     100,
		},
		{
			source: (&mock.Source{
				Channels:   2,
				Value:      0.7,
				Limit:      1000,
				SampleRate: sampleRate,
			}).Source(),
			numChannels: 2,
			samples:     1000,
		},
	}
	bufferSize := 10

	for _, test := range tests {
		asset := &audio.Asset{}
		l, _ := pipe.Routing{
			Source: test.source,
			Sink:   asset.Sink(),
		}.Line(bufferSize)

		pipe.New(context.Background(), pipe.WithLines(l)).Wait()

		assertEqual(t, "channels", asset.Signal.Channels(), test.numChannels)
		assertEqual(t, "sample rate", asset.SampleRate, sampleRate)
		assertEqual(t, "samples", asset.Signal.Length(), test.samples)
	}
}

func TestSource(t *testing.T) {
	sampleData := [][]float64{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}}
	buf := signal.Allocator{
		Channels: len(sampleData),
		Length:   len(sampleData[0]),
		Capacity: len(sampleData[0]),
	}.Float64()
	signal.WriteStripedFloat64(sampleData, buf)
	sampleRate := signal.SampleRate(44100)

	tests := []struct {
		source   pipe.SourceAllocatorFunc
		expected []float64
		msg      string
	}{
		{
			source:   audio.Source(sampleRate, buf),
			expected: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
			msg:      "Full asset",
		},
		{
			source:   audio.Source(sampleRate, buf.Slice(0, 3)),
			expected: []float64{0, 1, 2},
			msg:      "First three",
		},
		{
			source:   audio.Source(sampleRate, buf.Slice(1, 3)),
			expected: []float64{1, 2},
			msg:      "Two from within",
		},
		{
			source:   audio.Source(sampleRate, buf.Slice(5, 10)),
			expected: []float64{5, 6, 7, 8, 9},
			msg:      "Last five",
		},
	}

	bufferSize := 2
	for _, test := range tests {
		sink := mock.Sink{}
		l, _ := pipe.Routing{
			Source: test.source,
			Sink:   sink.Sink(),
		}.Line(bufferSize)

		p := pipe.New(context.Background(), pipe.WithLines(l))
		_ = p.Wait()

		result := make([]float64, sink.Values.Len())
		signal.ReadFloat64(sink.Values, result)

		assertEqual(t, test.msg, result, test.expected)
	}

}
