package audio

import (
	"io"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

// Source implements signal source for any signal type.
func Source(sr signal.Frequency, s signal.Signal) pipe.SourceAllocatorFunc {
	return func(mut mutable.Context, bufferSize int) (pipe.Source, error) {
		return pipe.Source{
			SourceFunc: signalSource(s),
			Output: pipe.SignalProperties{
				Channels:   s.Channels(),
				SampleRate: sr,
			},
		}, nil
	}
}

func signalSource(s signal.Signal) (sourceFn pipe.SourceFunc) {
	switch v := s.(type) {
	case signal.Signed:
		sourceFn = signedSource(v)
	case signal.Unsigned:
		sourceFn = unsignedSource(v)
	case signal.Floating:
		sourceFn = floatingSource(v)
	}
	return
}

func floatingSource(data signal.Floating) pipe.SourceFunc {
	pos := 0
	return func(out signal.Floating) (int, error) {
		if pos == data.Length() {
			return 0, io.EOF
		}
		end := pos + out.Length()
		if end > data.Length() {
			end = data.Length()
		}
		read := signal.FloatingAsFloating(data.Slice(pos, end), out)
		pos += read
		return read, nil
	}
}

func signedSource(data signal.Signed) pipe.SourceFunc {
	pos := 0
	return func(out signal.Floating) (int, error) {
		if pos == data.Length() {
			return 0, io.EOF
		}
		end := pos + out.Length()
		if end > data.Length() {
			end = data.Length()
		}
		read := signal.SignedAsFloating(data.Slice(pos, end), out)
		pos += read
		return read, nil
	}
}

func unsignedSource(data signal.Unsigned) pipe.SourceFunc {
	pos := 0
	return func(out signal.Floating) (int, error) {
		if pos == data.Length() {
			return 0, io.EOF
		}
		end := pos + out.Length()
		if end > data.Length() {
			end = data.Length()
		}
		read := signal.UnsignedAsFloating(data.Slice(pos, end), out)
		pos += read
		return read, nil
	}
}
