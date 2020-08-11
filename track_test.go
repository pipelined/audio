package audio_test

import (
	"context"
	"testing"

	"pipelined.dev/audio"
	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/signal"
)

func TestTrack(t *testing.T) {
	channels := 1
	alloc := signal.Allocator{
		Channels: channels,
		Capacity: 10,
		Length:   10,
	}
	sample1 := alloc.Float64()
	signal.WriteFloat64([]float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}, sample1)
	sample2 := alloc.Float64()
	signal.WriteFloat64([]float64{20, 21, 22, 23, 24, 25, 26, 27, 28, 29}, sample2)
	sampleRate := signal.SampleRate(44100)

	type clip struct {
		position int
		data     signal.Floating
	}
	tests := []struct {
		clips    []clip
		expected []float64
		msg      string
	}{
		{
			clips: []clip{
				{3, sample1.Slice(3, 4)},
				{4, sample2.Slice(5, 8)},
			},
			expected: []float64{0, 0, 0, 13, 25, 26, 27},
			msg:      "Sequence",
		},
		{
			clips: []clip{
				{2, sample1.Slice(3, 4)},
				{3, sample2.Slice(5, 8)},
			},
			expected: []float64{0, 0, 13, 25, 26, 27},
			msg:      "Sequence shifted left",
		},
		{
			clips: []clip{
				{2, sample1.Slice(3, 4)},
				{4, sample2.Slice(5, 8)},
			},
			expected: []float64{0, 0, 13, 0, 25, 26, 27},
			msg:      "Sequence with interval",
		},
		{
			clips: []clip{
				{3, sample1.Slice(3, 6)},
				{2, sample2.Slice(5, 7)},
			},
			expected: []float64{0, 0, 25, 26, 14, 15},
			msg:      "Overlap previous",
		},
		{
			clips: []clip{
				{2, sample1.Slice(3, 6)},
				{4, sample2.Slice(5, 7)},
			},
			expected: []float64{0, 0, 13, 14, 25, 26},
			msg:      "Overlap next",
		},
		{
			clips: []clip{
				{2, sample1.Slice(3, 9)},
				{4, sample2.Slice(5, 7)},
			},
			expected: []float64{0, 0, 13, 14, 25, 26, 17, 18},
			msg:      "Overlap single in the middle",
		},
		{
			clips: []clip{
				{2, sample1.Slice(3, 5)},
				{5, sample1.Slice(3, 5)},
				{4, sample2.Slice(5, 7)},
			},
			expected: []float64{0, 0, 13, 14, 25, 26, 14},
			msg:      "Overlap two in the middle",
		},
		{
			clips: []clip{
				{2, sample1.Slice(3, 5)},
				{5, sample1.Slice(5, 7)},
				{3, sample2.Slice(3, 5)},
			},
			expected: []float64{0, 0, 13, 23, 24, 15, 16},
			msg:      "Overlap two in the middle shifted",
		},
		{
			clips: []clip{
				{2, sample1.Slice(3, 5)},
				{2, sample2.Slice(3, 8)},
			},
			expected: []float64{0, 0, 23, 24, 25, 26, 27},
			msg:      "Overlap single completely",
		},
		{
			clips: []clip{
				{2, sample1.Slice(3, 5)},
				{5, sample1.Slice(5, 7)},
				{1, sample2.Slice(1, 9)},
			},
			expected: []float64{0, 21, 22, 23, 24, 25, 26, 27, 28},
			msg:      "Overlap two completely",
		},
	}

	for _, test := range tests {
		track := audio.Track{
			SampleRate: sampleRate,
			Channels:   channels,
		}
		for _, clip := range test.clips {
			track.AddClip(clip.position, clip.data)
		}

		sink := &mock.Sink{}

		l, _ := pipe.Routing{
			Source: track.Source(0, 0),
			Sink:   sink.Sink(),
		}.Line(2)

		pipe.New(context.Background(), pipe.WithLines(l)).Wait()

		result := make([]float64, sink.Values.Len())
		signal.ReadFloat64(sink.Values, result)

		assertEqual(t, test.msg, result, test.expected)
	}
}
