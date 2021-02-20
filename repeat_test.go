package audio_test

import (
	"context"
	"testing"

	"pipelined.dev/audio"
	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
)

const bufferSize = 512

func TestPipe(t *testing.T) {
	source := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	proc1 := &mock.Processor{}
	proc2 := &mock.Processor{}
	repeater := &audio.Repeater{}
	sink1 := &mock.Sink{Discard: true}
	sink2 := &mock.Sink{Discard: true}
	p, err := pipe.New(bufferSize,
		pipe.Line{
			Source:     source.Source(),
			Processors: pipe.Processors(proc1.Processor(), proc2.Processor()),
			Sink:       repeater.Sink(),
		},
		pipe.Line{
			Source: repeater.Source(),
			Sink:   sink1.Sink(),
		},
		pipe.Line{
			Source: repeater.Source(),
			Sink:   sink2.Sink(),
		},
	)
	assertNil(t, "error", err)

	// start
	_ = pipe.Wait(p.Start(context.Background()))
	assertNil(t, "error", err)

	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)
}

func TestRepeaterAddOutput(t *testing.T) {
	repeater := &audio.Repeater{}
	sink1 := &mock.Sink{}

	p, _ := pipe.New(
		bufferSize,
		pipe.Line{
			Source: (&mock.Source{
				Limit:    10 * bufferSize,
				Channels: 2,
			}).Source(),
			Sink: repeater.Sink(),
		},
		pipe.Line{
			Source: repeater.Source(),
			Sink:   sink1.Sink(),
		},
	)
	errc := p.Start(context.Background())

	sink2 := &mock.Sink{}
	p.Push(p.AddLine(pipe.Line{
		Source: repeater.Source(),
		Sink:   sink2.Sink(),
	}))
	// start
	_ = pipe.Wait(errc)
	assertEqual(t, "sink1 messages", sink1.Counter.Messages, 10)
	assertEqual(t, "sink1 samples", sink1.Counter.Samples, 10*bufferSize)
	assertEqual(t, "sink2 messages", sink2.Counter.Messages > 0, true)
	assertEqual(t, "sink2 samples", sink2.Counter.Samples > 0, true)
}

// This benchmark runs the following pipe:
// 1 Source is repeated to 2 Sinks
func BenchmarkRepeat(b *testing.B) {
	source := &mock.Source{
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	repeater := audio.Repeater{}
	p, _ := pipe.New(
		bufferSize,
		pipe.Line{
			Source: source.Source(),
			Sink:   repeater.Sink(),
		},
		pipe.Line{
			Source: repeater.Source(),
			Sink:   (&mock.Sink{Discard: true}).Sink(),
		},
		pipe.Line{
			Source: repeater.Source(),
			Sink:   (&mock.Sink{Discard: true}).Sink(),
		},
	)
	for i := 0; i < b.N; i++ {
		_ = pipe.Wait(p.Start(context.Background(), source.Reset()))
	}
}

func assertNil(t *testing.T, name string, result interface{}) {
	t.Helper()
	assertEqual(t, name, result, nil)
}
