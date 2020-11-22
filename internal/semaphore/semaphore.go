package semaphore

import "context"

// Semaphore implements semaphore synchronization primitive.
type Semaphore struct {
	limit chan struct{}
}

// New returns new initialized semaphore.
func New(l int) Semaphore {
	limit := make(chan struct{}, l)
	// for i := 0; i < l; i++ {
	// 	limit <- struct{}{}
	// }
	return Semaphore{
		limit: limit,
	}
}

// Acquire the lock.
func (s *Semaphore) Acquire(ctx context.Context) {
	select {
	case <-s.limit:
	case <-ctx.Done():
	}
}

// Release the lock.
func (s *Semaphore) Release() {
	s.limit <- struct{}{}
}
