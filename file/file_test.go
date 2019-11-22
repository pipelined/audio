package file_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pipelined.dev/audio/file"
)

func TestFilePump(t *testing.T) {
	var tests = []struct {
		fileName string
		negative bool
	}{
		{
			fileName: "test.wav",
		},
		{
			fileName: "test.mp3",
		},
		{
			fileName: "test.flac",
		},
		{
			fileName: "",
			negative: true,
		},
	}

	for _, test := range tests {
		format, err := file.FormatByPath(test.fileName)
		if test.negative {
			assert.NotNil(t, err)
		} else {
			assert.NotNil(t, format)
			pump := format.Pump(nil)
			assert.NotNil(t, pump)
		}
	}
}

func TestExtensions(t *testing.T) {
	var tests = []struct {
		format   file.Format
		expected int
	}{
		{
			file.WAV,
			2,
		},
		{
			file.MP3,
			1,
		},
		{
			file.FLAC,
			1,
		},
	}

	for _, test := range tests {
		exts := test.format.Extensions()
		assert.Equal(t, test.expected, len(exts))
	}
}
