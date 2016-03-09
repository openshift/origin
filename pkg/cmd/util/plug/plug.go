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
	// an effect. Calling Stop() before Start() is undefined. An error
	// may be returned with the stop.
	Stop(err error)
	// Blocks until Start() is invoked
	WaitForStart()
	// Blocks until Stop() is invoked
	WaitForStop() error
	// Returns true if Start() has been invoked
	IsStarted() bool
}

// plug is the default implementation of Plug
type plug struct {
	start   sync.Once
	stop    sync.Once
	startCh chan struct{}
	stopCh  chan error
}

// New returns a new plug that can begin in the Started state.
func New(started bool) Plug {
	p := &plug{
		startCh: make(chan struct{}),
		stopCh:  make(chan error, 1),
	}
	if started {
		p.Start()
	}
	return p
}

func (p *plug) Start() {
	p.start.Do(func() { close(p.startCh) })
}

func (p *plug) Stop(err error) {
	p.stop.Do(func() {
		if err != nil {
			p.stopCh <- err
		}
		close(p.stopCh)
	})
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

func (p *plug) WaitForStop() error {
	err, ok := <-p.stopCh
	if !ok {
		return nil
	}
	return err
}

// Leaser controls access to a lease
type Leaser interface {
	// AcquireAndHold tries to acquire the lease and hold it until it expires, the lease is deleted,
	// or we observe another party take the lease. The notify channel will be sent a nil value
	// when the lease is held, and closed when the lease is lost. If an error is sent the lease
	// is also considered lost.
	AcquireAndHold(chan error)
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
func (l *Leased) Stop(err error) {
	l.leaser.Release()
	l.Plug.Stop(err)
}

// Run tries to acquire and hold a lease, invoking Start()
// when the lease is held and invoking Stop() when the lease
// is lost. If the lease was lost gracefully, nil is returned.
// If the lease was lost due to an error, the error is returned.
func (l *Leased) Run() error {
	ch := make(chan error, 1)
	go l.leaser.AcquireAndHold(ch)
	var err error
	defer l.Stop(err)
	for {
		var ok bool
		err, ok = <-ch
		if !ok {
			return nil
		}
		if err != nil {
			for range ch {
				// read the rest of the channel
			}
			return err
		}
		l.Start()
	}
}
