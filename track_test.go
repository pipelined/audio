package audio_test

import (
	"testing"

	"github.com/pipelined/audio"
	"github.com/pipelined/signal"
	"github.com/stretchr/testify/assert"
)

func TestTrack(t *testing.T) {
	sampleRate := signal.SampleRate(44100)
	bufferSize := 2
	asset1 := audio.SignalAsset(sampleRate, [][]float64{{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}})
	asset2 := audio.SignalAsset(sampleRate, [][]float64{{20, 21, 22, 23, 24, 25, 26, 27, 28, 29}})
	asset3 := &audio.Asset{}
	tests := []struct {
		clips    []audio.Clip
		clipsAt  []int
		expected [][]float64
		msg      string
	}{
		{
			clips: []audio.Clip{
				asset1.Clip(3, 1),
				asset2.Clip(5, 3),
			},
			clipsAt:  []int{3, 4},
			expected: [][]float64{{0, 0, 0, 13, 25, 26, 27, 0}},
			msg:      "Sequence",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 1),
				asset2.Clip(5, 3),
			},
			clipsAt:  []int{3, 4},
			expected: [][]float64{{0, 0, 0, 13, 25, 26, 27, 0}},
			msg:      "Sequence increased bufferSize",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 1),
				asset2.Clip(5, 3),
			},
			clipsAt:  []int{2, 3},
			expected: [][]float64{{0, 0, 13, 25, 26, 27}},
			msg:      "Sequence shifted left",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 1),
				asset2.Clip(5, 3),
			},
			clipsAt:  []int{2, 4},
			expected: [][]float64{{0, 0, 13, 0, 25, 26, 27, 0}},
			msg:      "Sequence with interval",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 3),
				asset2.Clip(5, 2),
			},
			clipsAt:  []int{3, 2},
			expected: [][]float64{{0, 0, 25, 26, 14, 15}},
			msg:      "Overlap previous",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 3),
				asset2.Clip(5, 2),
			},
			clipsAt:  []int{2, 4},
			expected: [][]float64{{0, 0, 13, 14, 25, 26}},
			msg:      "Overlap next",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 5),
				asset2.Clip(5, 2),
			},
			clipsAt:  []int{2, 4},
			expected: [][]float64{{0, 0, 13, 14, 25, 26, 17, 0}},
			msg:      "Overlap single in the middle",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 2),
				asset1.Clip(3, 2),
				asset2.Clip(5, 2),
			},
			clipsAt:  []int{2, 5, 4},
			expected: [][]float64{{0, 0, 13, 14, 25, 26, 14, 0}},
			msg:      "Overlap two in the middle",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 2),
				asset1.Clip(5, 2),
				asset2.Clip(3, 2),
			},
			clipsAt:  []int{2, 5, 3},
			expected: [][]float64{{0, 0, 13, 23, 24, 15, 16, 0}},
			msg:      "Overlap two in the middle shifted",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 2),
				asset2.Clip(3, 5),
			},
			clipsAt:  []int{2, 2},
			expected: [][]float64{{0, 0, 23, 24, 25, 26, 27, 0}},
			msg:      "Overlap single completely",
		},
		{
			clips: []audio.Clip{
				asset1.Clip(3, 2),
				asset1.Clip(5, 2),
				asset2.Clip(1, 8),
			},
			clipsAt:  []int{2, 5, 1},
			expected: [][]float64{{0, 21, 22, 23, 24, 25, 26, 27, 28, 0}},
			msg:      "Overlap two completely",
		},
		{
			expected: [][]float64{},
			msg:      "Empty",
		},
		{
			clips: []audio.Clip{
				asset3.Clip(3, 2),
				asset3.Clip(5, 2),
				asset3.Clip(1, 8),
			},
			clipsAt:  []int{2, 5, 1},
			expected: [][]float64{},
			msg:      "Empty asset clips",
		},
	}

	for _, test := range tests {
		track := audio.NewTrack(sampleRate, asset1.NumChannels())
		err := track.Reset("")
		assert.Nil(t, err)

		for i, clip := range test.clips {
			track.AddClip(test.clipsAt[i], clip)
		}

		fn, pumpSampleRate, pumpNumChannels, err := track.Pump("")
		assert.Equal(t, int(sampleRate), pumpSampleRate)
		assert.Equal(t, asset1.NumChannels(), pumpNumChannels)
		assert.Nil(t, err)
		assert.NotNil(t, fn)

		var result signal.Float64
		for {
			buf := signal.Float64Buffer(pumpNumChannels, bufferSize)
			err = fn(buf)
			if err != nil {
				break
			}
			result = result.Append(buf)
		}
		assert.NotNil(t, err)

		assert.Equal(t, len(test.expected), result.NumChannels(), test.msg)
		for i := 0; i < len(result); i++ {
			assert.Equal(t, len(test.expected[i]), len(result[i]), test.msg)
			for j := 0; j < len(result[i]); j++ {
				assert.Equal(t, test.expected[i][j], result[i][j], test.msg)
			}
		}
	}
}
