package audio_test

import (
	"testing"

	"github.com/pipelined/audio"
	"github.com/pipelined/signal"
	"github.com/stretchr/testify/assert"
)

func TestAsset(t *testing.T) {
	tests := []struct {
		asset       *audio.Asset
		numChannels int
		sampleRate  signal.SampleRate
		value       float64
		messages    int
		samples     int
	}{
		{
			asset:       &audio.Asset{},
			numChannels: 1,
			value:       0.5,
			messages:    10,
			samples:     100,
		},
		{
			asset:       &audio.Asset{},
			numChannels: 2,
			value:       0.7,
			messages:    100,
			samples:     1000,
		},
		{
			asset:       &audio.Asset{},
			numChannels: 0,
			value:       0.7,
			messages:    0,
			samples:     0,
		},
	}
	bufferSize := 10

	for _, test := range tests {
		fn, err := test.asset.Sink("", test.sampleRate, test.numChannels)
		assert.Nil(t, err)
		assert.NotNil(t, fn)
		for i := 0; i < test.messages; i++ {
			buf := signal.Float64Buffer(test.numChannels, bufferSize)
			err := fn(buf)
			assert.Nil(t, err)
		}
		assert.Equal(t, test.numChannels, test.asset.NumChannels())
		assert.Equal(t, test.sampleRate, test.asset.SampleRate())
		assert.Equal(t, test.samples, test.asset.Data().Size())
	}
}

func TestClip(t *testing.T) {
	sampleRate := signal.SampleRate(44100)
	bufferSize := 2
	a := audio.SignalAsset(sampleRate, [][]float64{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}})
	tests := []struct {
		clip     audio.Clip
		expected signal.Float64
		msg      string
	}{
		{
			clip:     a.Clip(0, a.Data().Size()),
			expected: [][]float64{{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
			msg:      "Full asset",
		},
		{
			clip:     a.Clip(0, 3),
			expected: [][]float64{{0, 1, 2}},
			msg:      "First three",
		},
		{
			clip:     a.Clip(1, 2),
			expected: [][]float64{{1, 2}},
			msg:      "Two from within",
		},
		{
			clip:     a.Clip(5, 10),
			expected: [][]float64{{5, 6, 7, 8, 9}},
			msg:      "Last five",
		},
		{
			clip:     a.Clip(-1, 10),
			expected: [][]float64{},
			msg:      "Negative start",
		},
	}

	for _, test := range tests {
		fn, pumpSampleRate, pumpNumChannels, err := test.clip.Pump("")
		assert.Equal(t, sampleRate, pumpSampleRate)
		assert.Equal(t, a.NumChannels(), pumpNumChannels)
		assert.Nil(t, err)
		assert.NotNil(t, fn)

		var result signal.Float64
		buf := signal.Float64Buffer(pumpNumChannels, bufferSize)
		for {
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
