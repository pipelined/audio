package audio

import (
	"io"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"
)

// Source implements signal source for any signal type.
func Source(sr signal.SampleRate, s signal.Signal) pipe.SourceAllocatorFunc {
	return func(bufferSize int) (pipe.Source, pipe.SignalProperties, error) {
		return pipe.Source{
				SourceFunc: signalSource(s),
			}, pipe.SignalProperties{
				Channels:   s.Channels(),
				SampleRate: sr,
			}, nil
	}
}

func signalSource(s signal.Signal) pipe.SourceFunc {
	switch v := s.(type) {
	case signal.Signed:
		return signedSource(v)
	case signal.Unsigned:
		return unsignedSource(v)
	case signal.Floating:
		return floatingSource(v)
	}
	panic("should never happen")
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
