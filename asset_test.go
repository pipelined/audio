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
	sampleRate := signal.Frequency(44100)
	tests := []struct {
		source      pipe.SourceAllocatorFunc
		asset       *audio.Asset
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
			asset:       &audio.Asset{},
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
			asset: &audio.Asset{
				Signal: signal.Allocator{
					Channels: 2,
				}.Int64(signal.MaxBitDepth),
			},
			numChannels: 2,
			samples:     1000,
		},
		{
			source: (&mock.Source{
				Channels:   1,
				Value:      0.5,
				Limit:      100,
				SampleRate: sampleRate,
			}).Source(),
			asset: &audio.Asset{
				Signal: signal.Allocator{
					Channels: 1,
				}.Int64(signal.MaxBitDepth),
			},
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
			asset: &audio.Asset{
				Signal: signal.Allocator{
					Channels: 2,
				}.Int64(signal.MaxBitDepth),
			},
			numChannels: 2,
			samples:     1000,
		},
		{
			source: (&mock.Source{
				Channels:   1,
				Value:      0.5,
				Limit:      100,
				SampleRate: sampleRate,
			}).Source(),
			asset: &audio.Asset{
				Signal: signal.Allocator{
					Channels: 1,
				}.Uint64(signal.MaxBitDepth),
			},
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
			asset: &audio.Asset{
				Signal: signal.Allocator{
					Channels: 2,
				}.Uint64(signal.MaxBitDepth),
			},
			numChannels: 2,
			samples:     1000,
		},
	}
	bufferSize := 10

	for _, test := range tests {
		p, _ := pipe.New(context.Background(),
			bufferSize,
			&pipe.Line{
				Source: test.source,
				Sink:   test.asset.Sink(),
			},
		)

		p.Run().Wait()

		assertEqual(t, "channels", test.asset.Signal.Channels(), test.numChannels)
		assertEqual(t, "sample rate", test.asset.SampleRate(), sampleRate)
		assertEqual(t, "samples", test.asset.Signal.Length(), test.samples)
	}
}
