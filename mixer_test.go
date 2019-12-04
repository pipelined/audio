package audio_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"pipelined.dev/audio"
	"pipelined.dev/signal"
)

func TestMixer(t *testing.T) {
	const numTracks = 2
	tests := []struct {
		description string
		messages    [numTracks]int
		values      [numTracks]float64
		expected    [][]float64
	}{
		{
			description: "1st run",
			messages: [numTracks]int{
				4,
				3,
			},
			values: [numTracks]float64{
				0.7,
				0.5,
			},
			expected: [][]float64{{0.6, 0.6, 0.6, 0.6, 0.6, 0.6, 0.7, 0.7}},
		},
		// {
		// 	description: "2nd run",
		// 	messages: [numTracks]int{
		// 		5,
		// 		4,
		// 	},
		// 	values: [numTracks]float64{
		// 		0.5,
		// 		0.7,
		// 	},
		// 	expected: [][]float64{{0.6, 0.6, 0.6, 0.6, 0.6, 0.6, 0.6, 0.6, 0.5, 0.5}},
		// },
	}

	var (
		sampleRate  = signal.SampleRate(44100)
		numChannels = 1
		bufferSize  = 2
		pumpID      = string("pumpID")
	)

	mixer := audio.NewMixer(numChannels)

	// init sink funcs
	sinks := make([]func(signal.Float64) error, numTracks)
	for i := 0; i < numTracks; i++ {
		var err error
		sinks[i], err = mixer.Sink(string(i), sampleRate, numChannels)
		assert.Nil(t, err)
	}
	// init pump func
	pump, _, _, err := mixer.Pump(pumpID)
	assert.Nil(t, err)

	for _, test := range tests {
		result := signal.Float64(make([][]float64, numChannels))
		// mixing cycle
		for i := 0; i < numTracks; i++ {
			go func(i int, messages int, value float64, sinkFn func(signal.Float64) error) {
				// err := audio.Reset(string(i))
				// assert.Nil(t, err)
				for i := 0; i < messages; i++ {
					buf := buf(numChannels, bufferSize, value)
					err := sinkFn(buf)
					assert.NoError(t, err)
				}
				err := mixer.Flush(string(i))
				assert.NoError(t, err)
			}(i, test.messages[i], test.values[i], sinks[i])
		}
		// err = audio.Reset(pumpID)
		// assert.Nil(t, err)
		for {
			buffer := signal.Float64Buffer(numChannels, bufferSize)
			if err := pump(buffer); err != nil {
				assert.Equal(t, io.EOF, err)
				break
			}
			if buffer != nil {
				result = result.Append(buffer)
			}
		}
		err = mixer.Flush(pumpID)
		assert.NoError(t, err)
		// fmt.Printf("%+v", result)

		assert.Equal(t, len(test.expected), result.NumChannels(), "Incorrect result num channels")
		for i := range test.expected {
			assert.Equal(t, len(test.expected[i]), len(result[i]), "Incorrect result channel length")
			for j, val := range result[i] {
				assert.Equal(t, val, result[i][j])
			}
		}
	}
}

func buf(numChannels, size int, value float64) [][]float64 {
	result := make([][]float64, numChannels)
	for i := range result {
		result[i] = make([]float64, size)
		for j := range result[i] {
			result[i][j] = value
		}
	}
	return result
}
