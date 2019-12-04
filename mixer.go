package audio

import (
	"io"
	"sync"

	"pipelined.dev/signal"
)

// Mixer summs up multiple channels of messages into a single channel.
type Mixer struct {
	sampleRate  signal.SampleRate
	numChannels int

	// frames ready for mix
	output chan *frame
	// signal that all sinks are done
	done chan struct{}

	// mutex is needed to synchronize access to the mixer
	m sync.Mutex
	// inputs
	inputs map[string]*input
	// number of active inputs, used to fill the frames
	activeInputs int
	// first frame, used to initialize inputs
	firstFrame *frame
	// id of the pipe which is output of mixer
	outputID string
}

type message struct {
	inputID string
	buffer  signal.Float64
}

type input struct {
	id          string
	numChannels int
	frame       *frame
}

// frame represents a slice of samples to mix.
type frame struct {
	sync.Mutex
	buffer   signal.Float64
	summed   int
	expected int
	next     *frame
}

// sum returns mixed samplein.
func (f *frame) copySum(b signal.Float64) {
	// shrink result buffer if needed.
	if b.Size() > f.buffer.Size() {
		for i := range f.buffer {
			b[i] = b[i][:f.buffer.Size()]
		}
	}
	// copy summed data.
	for i := 0; i < b.NumChannels(); i++ {
		for j := 0; j < b.Size(); j++ {
			b[i][j] = f.buffer[i][j] / float64(f.summed)
		}
	}
}

func (f *frame) add(b signal.Float64) {
	// expand frame buffer if needed.
	if diff := b.Size() - f.buffer.Size(); diff > 0 {
		for i := range f.buffer {
			f.buffer[i] = append(f.buffer[i], make([]float64, diff)...)
		}
	}

	// copy summed data.
	for i := 0; i < b.NumChannels(); i++ {
		for j := 0; j < b.Size(); j++ {
			f.buffer[i][j] += b[i][j]
		}
	}
	f.summed++
}

const (
	maxInputs = 1024
)

// NewMixer returns new mixer.
func NewMixer(numChannels int) *Mixer {
	m := Mixer{
		inputs:      make(map[string]*input),
		numChannels: numChannels,
	}
	m.reset()
	return &m
}

// reset structures required for new run.
func (m *Mixer) reset() {
	m.output = make(chan *frame)
	m.done = make(chan struct{})
	m.firstFrame = newFrame(len(m.inputs), m.numChannels)
}

// Sink registers new input. All inputs should have same number of channels.
// If different number of channels is provided, error will be returned.
func (m *Mixer) Sink(inputID string, sampleRate signal.SampleRate, numChannels int) (func(signal.Float64) error, error) {
	m.sampleRate = sampleRate
	m.firstFrame.expected++
	in := input{
		id:          inputID,
		frame:       m.firstFrame,
		numChannels: numChannels,
	}
	// add new input.
	m.inputs[inputID] = &in
	m.activeInputs++
	var (
		done    bool
		current *frame
	)
	return func(b signal.Float64) error {
		current = in.frame
		current.Lock()
		current.add(b)
		done = current.done()
		// move input to the next frame.
		if current.next == nil {
			current.next = newFrame(current.expected, m.numChannels)
		}
		current.Unlock()
		in.frame = current.next

		// send if done.
		if done {
			m.output <- current
		}
		return nil
	}, nil
}

// Pump returns a pump function which allows to read the out channel.
func (m *Mixer) Pump(outputID string) (func(signal.Float64) error, signal.SampleRate, int, error) {
	numChannels := m.numChannels
	m.outputID = outputID
	return func(b signal.Float64) error {
		select {
		case f := <-m.output: // receive new frame.
			f.copySum(b)
			return nil
		case <-m.done: // recieve done signal.
			select {
			case f := <-m.output: // try to receive flushed frames.
				f.copySum(b)
				return nil
			default:
				return io.EOF
			}
		}

	}, m.sampleRate, numChannels, nil
}

// Flush mixer data for defined source.
// All Flush calls are synchronized. It doesn't affect Sink/Pump goroutines.
func (m *Mixer) Flush(sourceID string) error {
	m.m.Lock()
	defer m.m.Unlock()

	// flush pump.
	if m.isOutput(sourceID) {
		m.reset()
		for _, in := range m.inputs {
			in.frame = m.firstFrame
		}
		return nil
	}

	var (
		done    bool
		current *frame
		in      = m.inputs[sourceID]
	)
	// reset expectations for remaining frames.
	for in.frame != nil {
		current = in.frame
		current.Lock()
		current.expected--
		done = current.done()
		in.frame = current.next
		current.Unlock()

		// send if done.
		if done {
			m.output <- current
		}
	}
	// remove input from actives.
	m.activeInputs--
	if m.activeInputs == 0 {
		close(m.done)
	}
	return nil
}

func (m *Mixer) isOutput(sourceID string) bool {
	return sourceID == m.outputID
}

// isReady checks if frame is completed.
func (f *frame) done() bool {
	return f.expected > 0 && f.expected == f.summed
}

// newFrame generates new frame based on number of inputs.
func newFrame(numInputs, numChannels int) *frame {
	return &frame{
		expected: numInputs,
		buffer:   make([][]float64, numChannels),
	}
}
