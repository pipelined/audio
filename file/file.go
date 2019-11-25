// Package file provides functionality to process audio files with pipelined framework.
package file

import (
	"fmt"
	"io"
	"os"
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
		extensions       []string
	}
)

var (
	// WAV represents Waveform Audio file format.
	WAV = &format{
		defaultExtension: ".wav",
		extensions: []string{
			".wav",
			".wave",
		},
	}

	// MP3 represents MPEG-1 or MPEG-2 Audio Layer III file format.
	MP3 = &format{
		defaultExtension: ".mp3",
		extensions: []string{
			".mp3",
		},
	}

	// FLAC represents Free Lossless Audio Codec file format.
	FLAC = &format{
		defaultExtension: ".flac",
		extensions: []string{
			".flac",
		},
	}

	// formatByExtension = mapFormatByExtension(WAV, MP3, FLAC)
	formatByExtension = func(formats ...Format) map[string]Format {
		m := make(map[string]Format)
		for _, format := range formats {
			for _, ext := range format.Extensions() {
				if _, ok := m[ext]; ok {
					panic(fmt.Sprintf("multiple formats have same extension: %s", ext))
				}
				m[ext] = format
			}
		}
		return m
	}(WAV, MP3, FLAC)
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
func (f *format) MatchExtension(ext string) bool {
	format, ok := formatByExtension[strings.ToLower(ext)]
	if !ok {
		return false
	}
	return f == format
}

// Pump returns pipe.Pump for corresponding format
// with injected ReadSeeker.
func (f *format) Pump(rs io.ReadSeeker) pipe.Pump {
	switch f {
	case WAV:
		return &wav.Pump{ReadSeeker: rs}
	case MP3:
		return &mp3.Pump{Reader: rs}
	case FLAC:
		return &flac.Pump{Reader: rs}
	}
	return nil
}

// DefaultExtension of the format.
func (f *format) DefaultExtension() string {
	return f.defaultExtension
}

// Extensions returns a slice of format's extensions.
func (f *format) Extensions() []string {
	return append(f.extensions[:0:0], f.extensions...)
}

// PipeFunc is user-defined function that takes pipe.Pump as argument to execute pipe.
type PipeFunc func(pump pipe.Pump) error

// WalkPipe takes user-defined pipe function and return filepath.WalkFunc. It allows
// to use it with filepath.Walk function and execute pipe func with every file in a path.
// This function will try to parse file format from it's extension. User can limit input
// formats by providing allowed formats as argument.
func WalkPipe(fn PipeFunc, recursive bool, inputFormats ...Format) filepath.WalkFunc {
	var allowedFormats map[Format]struct{}
	if inputFormats != nil {
		allowedFormats = make(map[Format]struct{})
		for _, f := range inputFormats {
			allowedFormats[f] = struct{}{}
		}
	}
	return func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error during walk: %w", err)
		}
		if fi.IsDir() {
			// skip processing subdirs
			if !recursive {
				return filepath.SkipDir
			}
			return nil
		}

		format, ok := FormatByPath(path)
		if !ok {
			return nil
		}

		if allowedFormats != nil {
			if _, ok := allowedFormats[format]; !ok {
				return nil
			}
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error opening file: %w", err)
		}
		defer f.Close() // since we only read file, it's ok to close it with defer

		if err = fn(format.Pump(f)); err != nil {
			return fmt.Errorf("error execution pipe func: %w", err)
		}
		return nil
	}
}
