package latch

import "sync/atomic"

// Interface funcs are safe to invoke concurrently.
type Interface interface {
	// Done returns a chan that blocks until Close is called. It never returns data.
	Done() <-chan struct{}
	// Close closes the latch; all future calls to Closed return true. Safe to invoke multiple times.
	Close()
	// Closed returns false while the latch is "open" and true after it has been closed via Close.
	Closed() bool
}

type L struct {
	value int32
	line  chan struct{}
}

// New returns a new "open" latch such that Closed returns false until Close is invoked.
func New() Interface {
	return new(L).Reset()
}

func (l *L) Done() <-chan struct{} { return l.line }

// Close may panic for an uninitialized L
func (l *L) Close() {
	if atomic.AddInt32(&l.value, 1) == 1 {
		close(l.line)
	}
	<-l.line // concurrent calls to Close block until the latch is actually closed
}

func (l *L) Closed() (result bool) {
	select {
	case <-l.line:
		result = true
	default:
	}
	return
}

// Reset clears the state of the latch, not safe to execute concurrently with other L methods.
func (l *L) Reset() *L {
	l.line, l.value = make(chan struct{}), 0
	return l
}
