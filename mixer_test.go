package audio_test

import (
	"context"
	"testing"

	"pipelined.dev/audio"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/signal"
)

func TestMixer(t *testing.T) {
	type generator struct {
		messages int
		value    float64
	}
	tests := []struct {
		description string
		generators  []generator
		expected    []float64
	}{
		{
			description: "1st run",
			generators: []generator{
				{
					messages: 8,
					value:    0.7,
				},
				{
					messages: 6,
					value:    0.5,
				},
			},
			expected: []float64{0.6, 0.6, 0.6, 0.6, 0.6, 0.6, 0.7, 0.7},
		},
		{
			description: "2nd run",
			generators: []generator{
				{
					messages: 5,
					value:    0.5,
				},
				{
					messages: 4,
					value:    0.7,
				},
			},
			expected: []float64{0.6, 0.6, 0.6, 0.6, 0.5},
		},
	}

	const (
		// sampleRate  = signal.SampleRate(44100)
		numChannels = 1
		bufferSize  = 2
	)
	for _, test := range tests {
		mixer := audio.Mixer{}

		routes := make([]pipe.Routing, 0, len(test.generators)+1)
		for _, gen := range test.generators {
			sourceAllocator := mock.Source{
				Channels: numChannels,
				Limit:    gen.messages,
				Value:    gen.value,
			}
			routes = append(routes, pipe.Routing{
				Source: sourceAllocator.Source(),
				Sink:   mixer.Sink(),
			})
		}
		sink := mock.Sink{}
		routes = append(routes, pipe.Routing{
			Source: mixer.Source(),
			Sink:   sink.Sink(),
		})
		lines, err := pipe.Lines(bufferSize, routes...)
		assertEqual(t, "error", err, nil)

		err = pipe.New(context.Background(), pipe.WithLines(lines...)).Wait()
		assertEqual(t, "error", err, nil)

		result := make([]float64, sink.Values.Len())
		signal.ReadFloat64(sink.Values, result)
		assertEqual(t, "result", result, test.expected)
	}
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
	mixer := audio.Mixer{}

	var lines []pipe.Line
	valueMultiplier := 1.0 / float64(numLines)
	for i := 0; i < numLines; i++ {
		sourceAllocator := mock.Source{
			Channels: numChannels,
			Limit:    limit,
			Value:    float64(i) * valueMultiplier,
		}
		line, _ := pipe.Routing{
			Source: sourceAllocator.Source(),
			Sink:   mixer.Sink(),
		}.Line(bufferSize)
		lines = append(lines, line)
	}
	sink := mock.Sink{
		Discard: true,
	}
	line, _ := pipe.Routing{
		Source: mixer.Source(),
		Sink:   sink.Sink(),
	}.Line(bufferSize)
	lines = append(lines, line)

	pipe.New(context.Background(), pipe.WithLines(lines...)).Wait()
}
