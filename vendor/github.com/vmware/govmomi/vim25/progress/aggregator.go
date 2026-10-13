// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package progress

import "sync"

type Aggregator struct {
	downstream Sinker
	upstream   chan (<-chan Report)

	done chan struct{}
	w    sync.WaitGroup
}

func NewAggregator(s Sinker) *Aggregator {
	a := &Aggregator{
		downstream: s,
		upstream:   make(chan (<-chan Report)),

		done: make(chan struct{}),
	}

	a.w.Add(1)
	go a.loop()

	return a
}

func (a *Aggregator) loop() {
	defer a.w.Done()

	dch := a.downstream.Sink()
	defer close(dch)

	for {
		select {
		case uch := <-a.upstream:
			// Drain upstream channel
			for e := range uch {
				dch <- e
			}
		case <-a.done:
			return
		}
	}
}

func (a *Aggregator) Sink() chan<- Report {
	ch := make(chan Report)
	a.upstream <- ch
	return ch
}

// Done marks the aggregator as done. No more calls to Sink() may be made and
// the downstream progress report channel will be closed when Done() returns.
func (a *Aggregator) Done() {
	close(a.done)
	a.w.Wait()
}
