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

// Acquire the lock. Calling this method blocks until lock is obtained or
// context is expired. Returns true if lock is obtained, false if context
// is done.
func (s *Semaphore) Acquire(ctx context.Context) bool {
	select {
	case <-s.limit:
		return true
	case <-ctx.Done():
		return false
	}
}

// Release the lock.
func (s *Semaphore) Release() {
	s.limit <- struct{}{}
}
