package audio

import (
	"io"

	"github.com/pipelined/signal"
)

// Track is a sequence of pipes which are executed one after another.
type Track struct {
	numChannels int
	sampleRate  signal.SampleRate

	start   *link
	end     *link
	current *link

	// last sent index
	nextIndex int
}

// stream is a sequence of Clips in track.
// It uses double-linked list structure.
type link struct {
	At int
	Clip
	Next *link
	Prev *link
}

// End returns an end index of link.
func (l *link) End() int {
	if l == nil {
		return -1
	}
	return l.At + l.len
}

// NewTrack creates a new track in a session.
func NewTrack(sampleRate signal.SampleRate, numChannels int) (t *Track) {
	t = &Track{
		nextIndex:   0,
		sampleRate:  sampleRate,
		numChannels: numChannels,
	}
	return
}

// Pump implements track pump with a sequence of not overlapped clips.
func (t *Track) Pump(sourceID string) (func(bufferSize int) ([][]float64, error), int, int, error) {
	return func(bufferSize int) ([][]float64, error) {
		if t.nextIndex >= t.endIndex() {
			return nil, io.EOF
		}
		b := t.bufferAt(t.nextIndex, bufferSize)
		t.nextIndex += bufferSize
		if b.Size() < bufferSize {
			return b, io.ErrUnexpectedEOF
		}
		return b, nil
	}, int(t.sampleRate), t.numChannels, nil
}

// Reset flushes all links from track.
func (t *Track) Reset(sourceID string) error {
	t.nextIndex = 0
	return nil
}

func (t *Track) bufferAt(index, bufferSize int) (result signal.Float64) {
	if t.current == nil {
		t.current = t.linkAfter(index)
	}
	var buf signal.Float64
	bufferEnd := index + bufferSize
	for bufferSize > result.Size() {
		// if current link starts after frame then append empty buffer
		if t.current == nil || t.current.At >= bufferEnd {
			result = result.Append(signal.Float64Buffer(t.numChannels, bufferSize-result.Size()))
		} else {
			// if link starts in current frame
			if t.current.At >= index {
				// calculate offset buffer size
				// offset buffer is needed to align a link start within a buffer
				offsetBufSize := t.current.At - index
				result = result.Append(signal.Float64Buffer(t.numChannels, offsetBufSize))
				if bufferEnd >= t.current.End() {
					buf = t.current.asset.data.Slice(t.current.start, t.current.len)
				} else {
					buf = t.current.asset.data.Slice(t.current.start, bufferSize-result.Size())
				}
			} else {
				start := index - t.current.At + t.current.start
				if bufferEnd >= t.current.End() {
					buf = t.current.asset.data.Slice(start, t.current.End()-index)
				} else {
					buf = t.current.asset.data.Slice(start, bufferSize)
				}
			}
			index += buf.Size()
			result = result.Append(buf)
			if index >= t.current.End() {
				t.current = t.current.Next
			}
		}
	}
	return result
}

// linkAfter searches for a first link after passed index.
func (t *Track) linkAfter(index int) *link {
	slice := t.start
	for slice != nil {
		if slice.At >= index {
			return slice
		}
		slice = slice.Next
	}
	return nil
}

// endIndex returns index of last value of last link.
func (t *Track) endIndex() int {
	if t.end == nil {
		return -1
	}
	return t.end.At + t.end.len
}

// AddClip assigns a frame to a track.
func (t *Track) AddClip(at int, c Clip) {
	if c.asset == nil || c.len == 0 {
		return
	}
	t.current = nil
	l := &link{
		At:   at,
		Clip: c,
	}

	if t.start == nil {
		t.start = l
		t.end = l
		return
	}

	var next, prev *link
	if next = t.linkAfter(at); next != nil {
		prev = next.Prev
		next.Prev = l
	} else {
		prev = t.end
		t.end = l
	}

	if prev != nil {
		prev.Next = l
	} else {
		t.start = l
	}
	l.Next = next
	l.Prev = prev

	t.resolveOverlaps(l)
}

// resolveOverlaps resolves overlaps
func (t *Track) resolveOverlaps(l *link) {
	t.alignNextLink(l)
	t.alignPrevLink(l)
}

func (t *Track) alignNextLink(l *link) {
	next := l.Next
	if next == nil {
		return
	}
	overlap := l.At - next.At + l.len
	if overlap > 0 {
		if next.len > overlap {
			// shorten next
			next.start = next.start + overlap
			next.len = next.len - overlap
			next.At = next.At + overlap
		} else {
			// remove next
			l.Next = next.Next
			if l.Next != nil {
				l.Next.Prev = l
			} else {
				t.end = l
			}
			t.alignNextLink(l)
		}
	}
}

func (t *Track) alignPrevLink(l *link) {
	prev := l.Prev
	if prev == nil {
		return
	}
	overlap := prev.At - l.At + prev.len
	if overlap > 0 {
		prev.len = prev.len - overlap
		if overlap > l.len {
			at := l.At + l.len
			start := overlap + l.len + l.At - prev.At
			len := overlap - l.len
			t.AddClip(at, prev.asset.Clip(start, len))
		}
	}
}
