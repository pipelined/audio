package file

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/pipelined/flac"
	"github.com/pipelined/mp3"
	"github.com/pipelined/pipe"
	"github.com/pipelined/wav"
)

type (
	// Format of the file that contains audio signal.
	Format interface {
		Pump(io.ReadSeeker) pipe.Pump
		DefaultExtension() string
		MatchExtension(string) bool
		Extensions() []string
	}

	// generic struct that implements Format interface.
	format struct {
		defaultExtension string
		extensions       map[string]struct{}
	}
)

var (
	// WAV represents Waveform Audio file format.
	WAV = format{
		defaultExtension: ".wav",
		extensions: map[string]struct{}{
			".wav":  {},
			".wave": {},
		},
	}

	// MP3 represents MPEG-1 or MPEG-2 Audio Layer III file format.
	MP3 = format{
		defaultExtension: ".mp3",
		extensions: map[string]struct{}{
			".mp3": {},
		},
	}

	// FLAC represents Free Lossless Audio Codec file format.
	FLAC = format{
		defaultExtension: ".flac",
		extensions: map[string]struct{}{
			".flac": {},
		},
	}
)

// FormatByPath determines file format by file extension
// extracted from path. If extension belongs to unsupported
// format, second return argument will be false.
func FormatByPath(path string) (Format, bool) {
	ext := filepath.Ext(path)
	switch {
	case WAV.MatchExtension(ext):
		return WAV, true
	case MP3.MatchExtension(ext):
		return MP3, true
	case FLAC.MatchExtension(ext):
		return FLAC, true
	default:
		return nil, false
	}
}

// MatchExtension checks if ext matches to one of the format's
// extensions. Case is ignored.
func (f format) MatchExtension(ext string) bool {
	if f.extensions == nil {
		return false
	}
	_, ok := f.extensions[strings.ToLower(ext)]
	return ok
}

// Pump returns pipe.Pump for corresponding format
// with injected ReadSeeker.
func (f format) Pump(rs io.ReadSeeker) pipe.Pump {
	switch f.DefaultExtension() {
	case WAV.defaultExtension:
		return &wav.Pump{ReadSeeker: rs}
	case MP3.defaultExtension:
		return &mp3.Pump{Reader: rs}
	case FLAC.defaultExtension:
		return &flac.Pump{Reader: rs}
	}
	return nil
}

// DefaultExtension of the format.
func (f format) DefaultExtension() string {
	return f.defaultExtension
}

// Extensions returns a slice of format's extensions.
func (f format) Extensions() []string {
	exts := make([]string, 0, len(f.extensions))
	for k := range f.extensions {
		exts = append(exts, k)
	}
	return exts
}
