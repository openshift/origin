package writelimiter

import (
	"sync/atomic"

	"github.com/docker/distribution/context"
)

type LockFinalizer func()

type cancellableLock struct {
	sem chan struct{}
}

func newCancellableLock(size int) *cancellableLock {
	return &cancellableLock{
		sem: make(chan struct{}, size),
	}
}

func (l cancellableLock) Acquire(ctx context.Context) (LockFinalizer, bool) {
	// LOCK_BEGIN++
	sem := l.sem
	// LOCK_WAIT_BEGIN++
	select {
	case sem <- struct{}{}:
		// LOCK_WAIT_END++
		done := int32(0)
		return func() {
			if atomic.SwapInt32(&done, 1) == 0 {
				<-sem
				// LOCK_END++
			}
		}, true
	case <-ctx.Done():
		return nil, false
	}
}
