package ginkgo

import (
	"container/ring"
	"context"
	"strings"
	"sync"
)

// parallelByFileTestQueue runs tests in parallel unless they have
// the `[Serial]` tag on their name or if another test with the
// testExclusion field is currently running. Serial tests are
// defered until all other tests are completed.
type parallelByFileTestQueue struct {
	cond   *sync.Cond
	lock   sync.Mutex
	queue  *ring.Ring
	active map[string]struct{}
}

type nopLock struct{}

func (nopLock) Lock()   {}
func (nopLock) Unlock() {}

type TestFunc func(ctx context.Context, test *testCase)

func newParallelTestQueue(tests []*testCase) *parallelByFileTestQueue {
	r := ring.New(len(tests))
	for _, test := range tests {
		r.Value = test
		r = r.Next()
	}
	q := &parallelByFileTestQueue{
		cond:   sync.NewCond(nopLock{}),
		queue:  r,
		active: make(map[string]struct{}),
	}
	return q
}

func (q *parallelByFileTestQueue) pop() (*testCase, bool) {
	q.lock.Lock()
	defer q.lock.Unlock()
	r := q.queue
	l := r.Len()
	if l == 0 {
		q.cond.Broadcast()
		return nil, true
	}
	for i := 0; i < l; i++ {
		t := r.Value.(*testCase)
		if _, ok := q.active[t.testExclusion]; ok {
			r = r.Next()
			continue
		}
		if len(t.testExclusion) > 0 {
			q.active[t.testExclusion] = struct{}{}
		}
		if l == 1 {
			q.queue = nil
		} else {
			q.queue = r.Prev()
			q.queue.Unlink(1)
		}
		return t, true
	}
	return nil, false
}

func (q *parallelByFileTestQueue) done(t *testCase) {
	q.lock.Lock()
	defer q.lock.Unlock()
	delete(q.active, t.testExclusion)
	q.cond.Broadcast()
}

func (q *parallelByFileTestQueue) Close() {
	q.lock.Lock()
	defer q.lock.Unlock()
	q.queue = nil
	q.active = make(map[string]struct{})
	q.cond.Broadcast()
}

func (q *parallelByFileTestQueue) Take(ctx context.Context, fn TestFunc) bool {
	for {
		test, ok := q.pop()
		if !ok {
			q.cond.Wait()
			continue
		}
		if test == nil {
			return false
		}
		defer q.done(test)
		fn(ctx, test)
		return true
	}
}

func (q *parallelByFileTestQueue) Execute(parentCtx context.Context, parallelism int, fn TestFunc) {
	go func() {
		<-parentCtx.Done()
		q.Close()
	}()
	var serial []*testCase
	var lock sync.Mutex
	var wg sync.WaitGroup
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func(i int) {
			for q.Take(parentCtx, func(ctx context.Context, test *testCase) {
				if strings.Contains(test.name, "[Serial]") {
					lock.Lock()
					defer lock.Unlock()
					serial = append(serial, test)
					return
				}
				fn(ctx, test)
			}) {
				// no-op
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	for _, test := range serial {
		select {
		case <-parentCtx.Done():
			return
		default:
		}
		fn(parentCtx, test)
	}
}

func setTestExclusion(tests []*testCase, fn func(suitePath string, t *testCase) bool) {
	for _, test := range tests {
		summary := test.spec.Summary("")
		var suitePath string
		for _, loc := range summary.ComponentCodeLocations {
			if len(loc.FileName) > 0 {
				if !strings.HasSuffix(loc.FileName, "/k8s.io/kubernetes/test/e2e/framework/framework.go") {
					suitePath = loc.FileName
				}
			}
		}
		if fn(suitePath, test) {
			test.testExclusion = suitePath
		}
	}
}

func splitTests(tests []*testCase, fn func(*testCase) bool) (a, b []*testCase) {
	for _, t := range tests {
		if fn(t) {
			a = append(a, t)
		} else {
			b = append(b, t)
		}
	}
	return a, b
}
