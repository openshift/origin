package workqueue

import (
	"sync"

	"github.com/golang/glog"
)

type Work interface {
	Parallel(fn func())
}

type Try interface {
	Try(fn func() error)
}

type Interface interface {
	Batch(func(Work))
	Try(func(Try)) error
	Queue(func(Work))
	Done()
}

type workQueue struct {
	ch chan workUnit
	wg *sync.WaitGroup
}

func New(workers int, stopCh <-chan struct{}) Interface {
	q := &workQueue{
		ch: make(chan workUnit, 100),
		wg: &sync.WaitGroup{},
	}
	go q.run(workers, stopCh)
	return q
}

func (q *workQueue) run(workers int, stopCh <-chan struct{}) {
	for i := 0; i < workers; i++ {
		go func(i int) {
			defer glog.V(4).Infof("worker %d stopping", i)
			for {
				select {
				case work, ok := <-q.ch:
					if !ok {
						return
					}
					work.fn()
					work.wg.Done()
				case <-stopCh:
					return
				}
			}
		}(i)
	}
	<-stopCh
}

func (q *workQueue) Batch(fn func(Work)) {
	w := &worker{
		wg: &sync.WaitGroup{},
		ch: q.ch,
	}
	fn(w)
	w.wg.Wait()
}

func (q *workQueue) Try(fn func(Try)) error {
	w := &worker{
		wg:  &sync.WaitGroup{},
		ch:  q.ch,
		err: make(chan error),
	}
	fn(w)
	return w.FirstError()
}

func (q *workQueue) Queue(fn func(Work)) {
	w := &worker{
		wg: q.wg,
		ch: q.ch,
	}
	fn(w)
}

func (q *workQueue) Done() {
	q.wg.Wait()
}

type workUnit struct {
	fn func()
	wg *sync.WaitGroup
}

type worker struct {
	wg  *sync.WaitGroup
	ch  chan workUnit
	err chan error
}

func (w *worker) FirstError() error {
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	for {
		select {
		case err := <-w.err:
			if err != nil {
				return err
			}
		case <-done:
			return nil
		}
	}
}

func (w *worker) Parallel(fn func()) {
	w.wg.Add(1)
	w.ch <- workUnit{wg: w.wg, fn: fn}
}

func (w *worker) Try(fn func() error) {
	w.wg.Add(1)
	w.ch <- workUnit{
		wg: w.wg,
		fn: func() {
			err := fn()
			if w.err == nil {
				// TODO: have the work queue accumulate errors and release them with Done()
				glog.Errorf("Worker error: %v", err)
				return
			}
			w.err <- err
		},
	}
}
