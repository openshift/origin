package maxconnections

import (
	"context"
	"time"
)

// A Limiter controls starting of jobs.
type Limiter interface {
	// Start decides whether a new job can be started. The decision may be
	// returned after a delay if the limiter wants to throttle jobs.
	Start(context.Context) bool

	// Done must be called when a job is finished.
	Done()
}

// limiter ensures that there are no more than maxRunning jobs at the same
// time. It can enqueue up to maxInQueue jobs awaiting to be run, for other
// jobs Start will return false immediately.
type limiter struct {
	// running is a buffered channel. Before starting a job, an empty struct is
	// sent to the channel. When the job is finished, one element is received
	// back from the channel. If the channel's buffer is full, the job is
	// enqueued.
	running chan struct{}

	// queue is a buffered channel. An empty struct is placed into the channel
	// while a job is waiting for a spot in the running channel's buffer.
	// If the queue channel's buffer is full, the job is declined.
	queue chan struct{}

	// maxWaitInQueue is a maximum wait time in the queue, zero means forever.
	maxWaitInQueue time.Duration

	// newTimer allows to override the function time.NewTimer for tests.
	newTimer func(d time.Duration) *time.Timer
}

// NewLimiter return a limiter that allows no more than maxRunning jobs at the
// same time. It can enqueue up to maxInQueue jobs awaiting to be run, and a
// job may wait in the queue no more than maxWaitInQueue.
func NewLimiter(maxRunning, maxInQueue int, maxWaitInQueue time.Duration) Limiter {
	return &limiter{
		running:        make(chan struct{}, maxRunning),
		queue:          make(chan struct{}, maxInQueue),
		maxWaitInQueue: maxWaitInQueue,
		newTimer:       time.NewTimer,
	}
}

func (l *limiter) Start(ctx context.Context) bool {
	select {
	case l.running <- struct{}{}:
		return true
	default:
	}

	// Slow-path.
	select {
	case l.queue <- struct{}{}:
		defer func() {
			<-l.queue
		}()
	default:
		return false
	}

	var timeout <-chan time.Time
	// if l.maxWaitInQueue is 0, timeout will stay nil which practically means wait forever.
	if l.maxWaitInQueue > 0 {
		timer := l.newTimer(l.maxWaitInQueue)
		defer timer.Stop()
		timeout = timer.C
	}

	select {
	case l.running <- struct{}{}:
		return true
	case <-timeout:
	case <-ctx.Done():
	}
	return false
}

func (l *limiter) Done() {
	<-l.running
}
