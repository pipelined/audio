package audio

import (
	"io"

	"pipelined.dev/signal"
)

// Track is a sequence of pipes which are executed one after another.
type Track struct {
	numChannels int
	sampleRate  signal.SampleRate

	start   *link
	end     *link
	current *link // next link, that ends after index

	index int // last sent index
}

// stream is a sequence of Clips in track.
// It uses double-linked list structure.
type link struct {
	At int
	Clip
	Next *link
	Prev *link
}

// End position of the link in the track.
func (l *link) End() int {
	if l == nil {
		return -1
	}
	return l.At + l.len
}

// NewTrack creates a new track. Currently track is not threadsafe.
// It means that clips couldn't be added during pipe execution.
func NewTrack(sampleRate signal.SampleRate, numChannels int) (t *Track) {
	t = &Track{
		index:       0,
		sampleRate:  sampleRate,
		numChannels: numChannels,
	}
	return
}

// Pump implements track pump with a sequence of not overlapped clips.
func (t *Track) Pump(sourceID string) (func(signal.Float64) error, signal.SampleRate, int, error) {
	return func(b signal.Float64) error {
		if t.index >= t.endIndex() {
			return io.EOF
		}
		t.nextBuffer(b)
		t.index += b.Size()
		return nil
	}, t.sampleRate, t.numChannels, nil
}

// Reset sets track position to 0.
func (t *Track) Reset(sourceID string) error {
	t.index = 0
	return nil
}

func (t *Track) nextBuffer(b signal.Float64) {
	bufferEnd := t.index + b.Size()
	// number of read samples.
	var read int
	// read data until buffer is full or no more links till buffer end.
	for read < b.Size() {
		if t.current == nil || t.current.At >= bufferEnd {
			return
		}
		sliceStart := t.current.start
		// if link starts within buffer.
		if offset := t.current.At - (t.index + read); offset < b.Size() && offset > 0 {
			// don't read data in the offset.
			read += offset
		} else {
			sliceStart -= offset
		}

		var sliceEnd int
		// if current link ends withing buffer.
		if bufferEnd > t.current.End() {
			sliceEnd = t.current.start + t.current.len
		} else {
			sliceEnd = sliceStart + b.Size() - read
		}

		for i := range b {
			data := t.current.asset.data[i][sliceStart:sliceEnd]
			for j := range data {
				b[i][read+j] = data[j]
			}
		}
		read += (sliceEnd - sliceStart)

		if t.index+read >= t.current.End() {
			t.current = t.linkAfter(t.index + read)
		}
	}
}

// linkAfter searches for a first link, that ends after passed index.
func (t *Track) linkAfter(index int) *link {
	for l := t.start; l != nil; l = l.Next {
		if l.End() > index {
			return l
		}
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

// AddClip to the track. If clip has no asset or zero length, it
// won't be added to the track. Overlapped clips are realigned.
func (t *Track) AddClip(at int, c Clip) {
	// ignore empty clips.
	if c.asset == nil || c.len == 0 {
		return
	}

	// reset current clip after all realignments
	defer func() {
		t.current = t.linkAfter(t.index)
	}()

	// create a new link.
	l := &link{
		At:   at,
		Clip: c,
	}

	// if it's the first link.
	if t.start == nil {
		t.start = l
		t.end = l
		return
	}

	// connect new link with next link.
	var next, prev *link
	if next = t.linkAfter(at); next != nil {
		if next.At > at {
			// if next starts after
			prev = next.Prev
			next.Prev = l
		} else {
			// if next starts before
			prev = next
			next = next.Next
		}
	}

	if next == nil {
		prev = t.end
		t.end = l
	}

	// connect new link with previous link.
	if prev != nil {
		prev.Next = l
	} else {
		t.start = l
	}
	l.Next = next
	l.Prev = prev

	// resolve overlaps in the track.
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
