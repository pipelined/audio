package audio

import (
	"fmt"
	"io"
	"sync"

	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mutable"
	"pipelined.dev/signal"
)

// Track is a sequence of pipes which are executed one after another.
type Track struct {
	once     sync.Once
	channels int

	head *link
	tail *link
}

// stream is a sequence of Clips in track.
// It uses double-linked list structure.
type link struct {
	at   int
	data signal.Signal
	next *link
	prev *link
}

// End position of the link in the track.
func (l *link) End() int {
	if l == nil {
		return -1
	}
	return l.at + l.data.Length()
}

// Source implements track source with a sequence of not overlapped clips.
func (t *Track) Source(sampleRate signal.Frequency, start, end int) pipe.SourceAllocatorFunc {
	if end == 0 {
		end = t.endIndex()
	}
	return func(mut mutable.Context, bufferSize int) (pipe.Source, error) {
		return pipe.Source{
				SourceFunc: trackSource(t.head.nextAfter(start), start, end),
				Output: pipe.SignalProperties{
					Channels:   t.channels,
					SampleRate: sampleRate,
				},
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
				sliceEnd = current.data.Length()
			} else {
				sliceEnd = sliceStart + out.Length() - read
			}
			n := signal.AsFloating(signal.Slice(current.data, sliceStart, sliceEnd), out.Slice(read, out.Length()))
			if n == 0 {
				fmt.Printf("ZERO!")
			}
			read += n
			pos += n
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
	return t.tail.at + t.tail.data.Length()
}

// AddClip to the track. If clip has no asset or zero length, it
// won't be added to the track. Overlapped clips are realigned.
func (t *Track) AddClip(at int, data signal.Signal) {
	t.once.Do(func() {
		t.channels = data.Channels()
	})
	if t.channels != data.Channels() {
		panic(fmt.Sprintf("unexpected number of channels: %d want: %d", data.Channels(), t.channels))
	}
	// create a new link.
	l := &link{
		at:   at,
		data: data,
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
	overlap := l.End() - next.at
	if overlap > 0 {
		if next.data.Length() > overlap {
			// shorten next
			next.data = signal.Slice(next.data, overlap, next.data.Length())
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
		prevLen := prev.data.Length()
		prev.data = signal.Slice(prev.data, 0, prevLen-overlap)
		// need to split previous clip
		if overlap > l.data.Length() {
			at := l.at + l.data.Length()
			t.AddClip(at, signal.Slice(prev.data, prevLen-l.data.Length(), prevLen)) // -1 because slicing includes left index
		}
		// TODO: handle full overlap
	}
}
