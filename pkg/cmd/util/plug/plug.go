package plug

import (
	"sync"
)

// Plug represents a synchronization primitive that holds and releases
// execution for other objects.
type Plug interface {
	// Begins operation of the plug and unblocks WaitForStart().
	// May be invoked multiple times but only the first invocation has
	// an effect.
	Start()
	// Ends operation of the plug and unblocks WaitForStop()
	// May be invoked multiple times but only the first invocation has
	// an effect. Calling Stop() before Start() is undefined.
	Stop()
	// Blocks until Start() is invoked
	WaitForStart()
	// Blocks until Stop() is invoked
	WaitForStop()
	// Returns true if Start() has been invoked
	IsStarted() bool
}

// plug is the default implementation of Plug
type plug struct {
	start   sync.Once
	stop    sync.Once
	startCh chan struct{}
	stopCh  chan struct{}
}

// New returns a new plug that can begin in the Started state.
func New(started bool) Plug {
	p := &plug{
		startCh: make(chan struct{}),
		stopCh:  make(chan struct{}),
	}
	if started {
		p.Start()
	}
	return p
}

func (p *plug) Start() {
	p.start.Do(func() { close(p.startCh) })
}

func (p *plug) Stop() {
	p.stop.Do(func() { close(p.stopCh) })
}

func (p *plug) IsStarted() bool {
	select {
	case <-p.startCh:
		return true
	default:
		return false
	}
}

func (p *plug) WaitForStart() {
	<-p.startCh
}

func (p *plug) WaitForStop() {
	<-p.stopCh
}

// Leaser controls access to a lease
type Leaser interface {
	// AcquireAndHold tries to acquire the lease and hold it until it expires, the lease is deleted,
	// or we observe another party take the lease. The notify channel will be sent a value
	// when the lease is held, and closed when the lease is lost.
	AcquireAndHold(chan struct{})
	Release()
}

// leased uses a Leaser to control Start and Stop on a Plug
type Leased struct {
	Plug

	leaser Leaser
}

var _ Plug = &Leased{}

// NewLeased creates a Plug that starts when a lease is acquired
// and stops when it is lost.
func NewLeased(leaser Leaser) *Leased {
	return &Leased{
		Plug:   New(false),
		leaser: leaser,
	}
}

// Stop releases the acquired lease
func (l *Leased) Stop() {
	l.leaser.Release()
	l.Plug.Stop()
}

// Run tries to acquire and hold a lease, invoking Start()
// when the lease is held and invoking Stop() when the lease
// is lost.
func (l *Leased) Run() {
	ch := make(chan struct{}, 1)
	go l.leaser.AcquireAndHold(ch)
	defer l.Stop()
	for {
		_, ok := <-ch
		if !ok {
			return
		}
		l.Start()
	}
}
