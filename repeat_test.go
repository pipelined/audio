package audio_test

import (
	"context"
	"testing"

	"pipelined.dev/audio"
	"pipelined.dev/pipe"
	"pipelined.dev/pipe/mock"
	"pipelined.dev/pipe/mutability"
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

	in, err := pipe.Routing{
		Source:     source.Source(),
		Processors: pipe.Processors(proc1.Processor(), proc2.Processor()),
		Sink:       repeater.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)
	out1, err := pipe.Routing{
		Source: repeater.Source(),
		Sink:   sink1.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)
	out2, err := pipe.Routing{
		Source: repeater.Source(),
		Sink:   sink2.Sink(),
	}.Line(bufferSize)
	assertNil(t, "error", err)

	p := pipe.New(context.Background(), pipe.WithLines(in, out1, out2))
	// start
	err = p.Wait()
	assertNil(t, "error", err)

	assertEqual(t, "messages", source.Counter.Messages, 862)
	assertEqual(t, "samples", source.Counter.Samples, 862*bufferSize)
}

func TestAddRouting(t *testing.T) {
	repeater := &audio.Repeater{
		Mutability: mutability.Mutable(),
	}
	source, _ := pipe.Routing{
		Source: (&mock.Source{
			Limit:    10 * bufferSize,
			Channels: 2,
		}).Source(),
		Sink: repeater.Sink(),
	}.Line(bufferSize)

	sink1 := &mock.Sink{}
	destination1, _ := pipe.Routing{
		Source: repeater.Source(),
		Sink:   sink1.Sink(),
	}.Line(bufferSize)

	p := pipe.New(
		context.Background(),
		pipe.WithLines(source, destination1),
	)
	sink2 := &mock.Sink{}
	line := pipe.Routing{
		Sink: sink2.Sink(),
	}
	p.Push(repeater.AddOutput(p, line))

	// start
	_ = p.Wait()
	assertEqual(t, "sink1 messages", sink1.Counter.Messages, 10)
	assertEqual(t, "sink1 samples", sink1.Counter.Samples, 10*bufferSize)
	assertEqual(t, "sink2 messages", sink2.Counter.Messages > 0, true)
	assertEqual(t, "sink2 samples", sink2.Counter.Samples > 0, true)
}

// This benchmark runs the following pipe:
// 1 Source is repeated to 2 Sinks
func BenchmarkRepeat(b *testing.B) {
	source := &mock.Source{
		Mutator: mock.Mutator{
			Mutability: mutability.Mutable(),
		},
		Limit:    862 * bufferSize,
		Channels: 2,
	}
	repeater := audio.Repeater{}
	l1, _ := pipe.Routing{
		Source: source.Source(),
		Sink:   repeater.Sink(),
	}.Line(bufferSize)
	l2, _ := pipe.Routing{
		Source: repeater.Source(),
		Sink:   (&mock.Sink{Discard: true}).Sink(),
	}.Line(bufferSize)
	l3, _ := pipe.Routing{
		Source: repeater.Source(),
		Sink:   (&mock.Sink{Discard: true}).Sink(),
	}.Line(bufferSize)
	for i := 0; i < b.N; i++ {
		p := pipe.New(
			context.Background(),
			pipe.WithLines(l1, l2, l3),
			pipe.WithMutations(source.Reset()),
		)
		_ = p.Wait()
	}
}

func assertNil(t *testing.T, name string, result interface{}) {
	t.Helper()
	assertEqual(t, name, result, nil)
}
