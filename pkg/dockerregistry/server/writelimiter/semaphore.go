package writelimiter

type semaphore struct {
	c chan struct{}
}

func newSemaphore(count int) *semaphore {
	return &semaphore{
		c: make(chan struct{}, count),
	}
}

func (s *semaphore) Up() {
	<-s.c
}

func (s *semaphore) TryDown() bool {
	select {
	case s.c <- struct{}{}:
		return true
	default:
		return false
	}
}
