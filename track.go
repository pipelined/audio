package audio

import (
	"io"

	"pipelined.dev/pipe"
	"pipelined.dev/signal"
)

// Track is a sequence of pipes which are executed one after another.
type Track struct {
	numChannels int
	sampleRate  signal.SampleRate

	head *link
	tail *link
}

// stream is a sequence of Clips in track.
// It uses double-linked list structure.
type link struct {
	at   int
	clip Clip
	next *link
	prev *link
}

// End position of the link in the track.
func (l *link) End() int {
	if l == nil {
		return -1
	}
	return l.at + l.clip.data.Length()
}

// NewTrack creates a new track. Currently track is not threadsafe.
// It means that clips couldn't be added during pipe execution.
func NewTrack(sampleRate signal.SampleRate, numChannels int) (t *Track) {
	t = &Track{
		sampleRate:  sampleRate,
		numChannels: numChannels,
	}
	return
}

// Source implements track source with a sequence of not overlapped clips.
func (t *Track) Source(start, end int) pipe.SourceAllocatorFunc {
	if end == 0 {
		end = t.endIndex()
	}
	return func(bufferSize int) (pipe.Source, pipe.SignalProperties, error) {
		return pipe.Source{
				SourceFunc: trackSource(t.head.nextAfter(start), start, end),
			},
			pipe.SignalProperties{
				Channels:   t.numChannels,
				SampleRate: t.sampleRate,
			},
			nil
	}
}

func trackSource(current *link, start, end int) pipe.SourceFunc {
	pos := start
	return func(out signal.Floating) (int, error) {
		if current == nil {
			return 0, io.EOF
		}

		// track index where source buffer will end
		bufferEnd := pos + out.Length()
		// number of samples read per channel
		read := 0
		for pos < bufferEnd {
			if current == nil {
				return read, nil
			}
			// current clip starts after buffer end
			if current.at >= bufferEnd {
				pos = bufferEnd
				return out.Length(), nil
			}

			sliceStart := 0
			// if link starts within buffer.
			if offset := current.at - pos; offset < out.Length() && offset > 0 {
				// don't read data in the offset.
				read += offset
				pos += offset
			} else {
				sliceStart -= offset
			}

			var sliceEnd int
			// if current link ends withing buffer.
			if bufferEnd > current.End() {
				sliceEnd = current.clip.data.Length()
			} else {
				sliceEnd = sliceStart + out.Length() - read
			}

			for i := sliceStart; i < sliceEnd; i++ {
				for c := 0; c < current.clip.data.Channels(); c++ {
					out.SetSample(signal.BufferIndex(out.Channels(), c, read), current.clip.data.Sample(signal.BufferIndex(current.clip.data.Channels(), c, i)))
				}
				read++
				pos++
			}
			if pos >= current.End() {
				current = current.nextAfter(pos)
			}
		}
		return read, nil
	}
}

// linkAfter searches for a first link, that ends after passed index.
func (l *link) nextAfter(index int) *link {
	for l != nil {
		if l.End() > index {
			return l
		}
		l = l.next
	}
	return nil
}

// endIndex returns index of last value of last link.
func (t *Track) endIndex() int {
	if t.tail == nil {
		return -1
	}
	return t.tail.at + t.tail.clip.data.Length()
}

// AddClip to the track. If clip has no asset or zero length, it
// won't be added to the track. Overlapped clips are realigned.
func (t *Track) AddClip(at int, c Clip) {
	// create a new link.
	l := &link{
		at:   at,
		clip: c,
	}

	// if it's the first link.
	if t.head == nil {
		t.head = l
		t.tail = l
		return
	}

	// connect new link with next link.
	var next, prev *link
	if next = t.head.nextAfter(at); next != nil {
		if next.at > at {
			// if next starts after
			prev = next.prev
			next.prev = l
		} else {
			// if next starts before
			prev = next
			next = next.next
		}
	}

	if next == nil {
		prev = t.tail
		t.tail = l
	}

	// connect new link with previous link.
	if prev != nil {
		prev.next = l
	} else {
		t.head = l
	}
	l.next = next
	l.prev = prev

	// resolve overlaps in the track.
	t.resolveOverlaps(l)
}

// resolveOverlaps resolves overlaps
func (t *Track) resolveOverlaps(l *link) {
	t.alignNextLink(l)
	t.alignPrevLink(l)
}

func (t *Track) alignNextLink(l *link) {
	next := l.next
	if next == nil {
		return
	}
	overlap := l.at - next.at + l.clip.data.Length()
	if overlap > 0 {
		if next.clip.data.Length() > overlap {
			// shorten next
			next.clip.data = next.clip.data.Slice(overlap, next.clip.data.Length())
			next.at = next.at + overlap
		} else {
			// remove next
			l.next = next.next
			if l.next != nil {
				l.next.prev = l
			} else {
				t.tail = l
			}
			t.alignNextLink(l)
		}
	}
}

func (t *Track) alignPrevLink(l *link) {
	prev := l.prev
	if prev == nil {
		return
	}
	overlap := prev.End() - l.at
	if overlap > 0 {
		prev.clip.data = prev.clip.data.Slice(0, prev.clip.data.Length()-overlap)
		if overlap > l.clip.data.Length() {
			at := l.at + l.clip.data.Length()
			t.AddClip(at, prev.clip.Clip(overlap+l.clip.data.Length()-1, overlap-l.clip.data.Length())) // -1 because slicing includes left index
		}
	}
}
