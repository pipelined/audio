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

func TestMixer(t *testing.T) {
	type generator struct {
		limit int
		value float64
	}
	const (
		numChannels = 1
		bufferSize  = 2
	)
	mixer := func(generators []generator, expected []float64) func(*testing.T) {
		return func(t *testing.T) {
			t.Helper()
			mixer := audio.Mixer{}

			routes := make([]pipe.Routing, 0, len(generators)+1)
			for _, gen := range generators {
				routes = append(routes, pipe.Routing{
					Source: (&mock.Source{
						Channels: numChannels,
						Limit:    gen.limit,
						Value:    gen.value,
					}).Source(),
					Sink: mixer.Sink(),
				})
			}
			sink := mock.Sink{}
			routes = append(routes, pipe.Routing{
				Source: mixer.Source(),
				Sink:   sink.Sink(),
			})

			p, err := pipe.New(bufferSize, routes...)
			assertEqual(t, "error", err, nil)

			err = p.Async(context.Background()).Await()
			assertEqual(t, "error", err, nil)

			result := make([]float64, sink.Values.Len())
			signal.ReadFloat64(sink.Values, result)
			assertEqual(t, "result", result, expected)
			assertEqual(t, "messages", int(math.Ceil(float64(len(result))/float64(bufferSize)))+1, sink.Messages)
		}
	}
	t.Run("single channel",
		mixer(
			[]generator{
				{
					limit: 4,
					value: 0.7,
				},
			},
			[]float64{0.7, 0.7, 0.7, 0.7},
		),
	)
	t.Run("two channels same length",
		mixer(
			[]generator{
				{
					limit: 6,
					value: 0.7,
				},
				{
					limit: 6,
					value: 0.5,
				},
			},
			[]float64{0.6, 0.6, 0.6, 0.6, 0.6, 0.6},
		),
	)
	t.Run("two channels short buffer",
		mixer(
			[]generator{
				{
					limit: 5,
					value: 0.5,
				},
				{
					limit: 4,
					value: 0.7,
				},
			},
			[]float64{0.6, 0.6, 0.6, 0.6, 0.5},
		),
	)
}

func Test100Lines(t *testing.T) {
	run(1, 512, 51200, 100)
}

func BenchmarkMixerLimit(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(1, 512, i*2, 10)
	}
}

func BenchmarkMixerLines(b *testing.B) {
	for i := 0; i < b.N; i++ {
		run(1, 512, 51200, i+1)
	}
}

func run(numChannels, bufferSize, limit, numLines int) {
	var (
		lines []pipe.Routing
		mixer audio.Mixer
	)
	valueMultiplier := 1.0 / float64(numLines)
	for i := 0; i < numLines; i++ {
		lines = append(lines,
			pipe.Routing{
				Source: (&mock.Source{
					Channels: numChannels,
					Limit:    limit,
					Value:    float64(i) * valueMultiplier,
				}).Source(),
				Sink: mixer.Sink(),
			},
		)
	}
	lines = append(lines, pipe.Routing{
		Source: mixer.Source(),
		Sink: (&mock.Sink{
			Discard: true,
		}).Sink(),
	})

	p, _ := pipe.New(bufferSize, lines...)
	p.Async(context.Background()).Await()
}
