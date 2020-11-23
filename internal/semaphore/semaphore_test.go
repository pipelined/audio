package semaphore_test

import (
	"context"
	"testing"
	"time"

	"pipelined.dev/audio/internal/semaphore"
)

func TestSema(t *testing.T) {
	sema := semaphore.New(1)
	sema.Release()
	ctx, cancelFn := context.WithTimeout(context.Background(), time.Second*1)
	defer cancelFn()
	if !sema.Acquire(ctx) {
		t.Fatalf("acquire should have succeeded")
	}
	if sema.Acquire(ctx) {
		t.Fatalf("acquire should have failed")
	}
}
