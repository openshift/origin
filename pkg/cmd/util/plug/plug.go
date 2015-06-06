package plug

import (
	"sync"
)

type Plug interface {
	Start()
	Stop()
	WaitForStart()
	WaitForStop()
	IsStarted() bool
}

type plug struct {
	start   sync.Once
	stop    sync.Once
	startCh chan struct{}
	stopCh  chan struct{}
}

func NewPlug(started bool) Plug {
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
