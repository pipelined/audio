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
		assertEqual(t, "sample rate", asset.SampleRate(), sampleRate)
		assertEqual(t, "samples", asset.Signal.Length(), test.samples)
	}
}
