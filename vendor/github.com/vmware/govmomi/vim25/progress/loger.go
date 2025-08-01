// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"sync"
	"time"
)

type LogFunc func(msg string) (int, error)

type ProgressLogger struct {
	log    LogFunc
	prefix string

	wg sync.WaitGroup

	sink chan chan Report
	done chan struct{}
}

func NewProgressLogger(log LogFunc, prefix string) *ProgressLogger {
	p := &ProgressLogger{
		log:    log,
		prefix: prefix,

		sink: make(chan chan Report),
		done: make(chan struct{}),
	}

	p.wg.Add(1)

	go p.loopA()

	return p
}

// loopA runs before Sink() has been called.
func (p *ProgressLogger) loopA() {
	var err error

	defer p.wg.Done()

	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()

	called := false

	for stop := false; !stop; {
		select {
		case ch := <-p.sink:
			err = p.loopB(tick, ch)
			stop = true
			called = true
		case <-p.done:
			stop = true
		case <-tick.C:
			line := fmt.Sprintf("\r%s", p.prefix)
			p.log(line)
		}
	}

	if err != nil && err != io.EOF {
		p.log(fmt.Sprintf("\r%sError: %s\n", p.prefix, err))
	} else if called {
		p.log(fmt.Sprintf("\r%sOK\n", p.prefix))
	}
}

// loopA runs after Sink() has been called.
func (p *ProgressLogger) loopB(tick *time.Ticker, ch <-chan Report) error {
	var r Report
	var ok bool
	var err error

	for ok = true; ok; {
		select {
		case r, ok = <-ch:
			if !ok {
				break
			}
			err = r.Error()
		case <-tick.C:
			line := fmt.Sprintf("\r%s", p.prefix)
			if r != nil {
				line += fmt.Sprintf("(%.0f%%", r.Percentage())
				detail := r.Detail()
				if detail != "" {
					line += fmt.Sprintf(", %s", detail)
				}
				line += ")"
			}
			p.log(line)
		}
	}

	return err
}

func (p *ProgressLogger) Sink() chan<- Report {
	ch := make(chan Report)
	p.sink <- ch
	return ch
}

func (p *ProgressLogger) Wait() {
	close(p.done)
	p.wg.Wait()
}
