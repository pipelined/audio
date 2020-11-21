package semaphore_test

import (
	"context"
	"testing"

	"pipelined.dev/audio/internal/semaphore"
)

func TestSema(t *testing.T) {
	sema := semaphore.New(1)
	sema.Acquire(context.Background())
	ctx, cancelFn := context.WithCancel(context.Background())
	cancelFn()
	sema.Acquire(ctx)
	sema.Release()
}
