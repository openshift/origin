package controller

import (
	"fmt"
	"sync"
	"testing"

	kcache "k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
)

func TestRetryController_handleOneRetryableError(t *testing.T) {
	retried := false
	retryErr := fmt.Errorf("retryable error")

	controller := &RetryController{
		Handle: func(obj interface{}) error {
			return retryErr
		},
		RetryManager: &testRetryManager{
			RetryFunc: func(resource interface{}, err error) {
				if err != retryErr {
					t.Fatalf("unexpected error: %v", err)
				}
				retried = true
			},
			ForgetFunc: func(resource interface{}) {
				t.Fatalf("unexpected call to forget %v", resource)
			},
		},
	}

	controller.handleOne(struct{}{})

	if !retried {
		t.Fatalf("expected a retry")
	}
}

func TestRetryController_handleOneNoError(t *testing.T) {
	forgotten := false

	controller := &RetryController{
		Handle: func(obj interface{}) error {
			return nil
		},
		RetryManager: &testRetryManager{
			RetryFunc: func(resource interface{}, err error) {
				t.Fatalf("unexpected call to retry %v", resource)
			},
			ForgetFunc: func(resource interface{}) {
				forgotten = true
			},
		},
	}

	controller.handleOne(struct{}{})

	if !forgotten {
		t.Fatalf("expected to forget")
	}
}

func TestQueueRetryManager_retries(t *testing.T) {
	retries := 5
	requeued := map[string]int{}

	manager := &QueueRetryManager{
		queue: &testFifo{
			// Track re-queues
			AddIfNotPresentFunc: func(obj interface{}) error {
				id := obj.(testObj).id
				if _, exists := requeued[id]; !exists {
					requeued[id] = 0
				}
				requeued[id] = requeued[id] + 1
				return nil
			},
		},
		keyFunc: func(obj interface{}) (string, error) {
			return obj.(testObj).id, nil
		},
		retryFunc: func(obj interface{}, err error, r Retry) bool {
			return r.Count < 5 && !r.StartTimestamp.IsZero()
		},
		retries: make(map[string]Retry),
		limiter: flowcontrol.NewTokenBucketRateLimiter(1000, 1000),
	}

	objects := []testObj{
		{"a", 1},
		{"b", 2},
		{"c", 3},
	}

	// Retry one more than the max
	for _, obj := range objects {
		for i := 0; i < retries+1; i++ {
			manager.Retry(obj, nil)
		}
	}

	// Should only have re-queued up to the max retry setting
	for _, obj := range objects {
		if e, a := retries, requeued[obj.id]; e != a {
			t.Fatalf("expected requeue count %d for obj %s, got %d", e, obj.id, a)
		}
	}

	// Should have no more state since all objects were retried beyond max
	if e, a := 0, len(manager.retries); e != a {
		t.Fatalf("expected retry len %d, got %d", e, a)
	}
}

// This test ensures that when an asynchronous state update is received
// on the queue during failed event handling, that the updated state is
// retried, NOT the event that failed (which is now stale).
func TestRetryController_realFifoEventOrdering(t *testing.T) {
	keyFunc := func(obj interface{}) (string, error) {
		return obj.(testObj).id, nil
	}

	fifo := kcache.NewResyncableFIFO(keyFunc)

	wg := sync.WaitGroup{}
	wg.Add(1)

	controller := &RetryController{
		Queue:        fifo,
		RetryManager: NewQueueRetryManager(fifo, keyFunc, func(_ interface{}, _ error, _ Retry) bool { return true }, flowcontrol.NewTokenBucketRateLimiter(1000, 10)),
		Handle: func(obj interface{}) error {
			if e, a := 1, obj.(testObj).value; e != a {
				t.Fatalf("expected to handle test value %d, got %d", e, a)
			}

			go func() {
				fifo.Add(testObj{"a", 2})
				wg.Done()
			}()
			wg.Wait()
			return fmt.Errorf("retryable error")
		},
	}

	fifo.Add(testObj{"a", 1})
	controller.handleOne(kcache.Pop(fifo))

	if e, a := 1, len(fifo.List()); e != a {
		t.Fatalf("expected queue length %d, got %d", e, a)
	}

	obj := kcache.Pop(fifo)
	if e, a := 2, obj.(testObj).value; e != a {
		t.Fatalf("expected queued value %d, got %d", e, a)
	}
}

// This test ensures that when events are retried, the
// requeue rate does not exceed the configured rate limit,
// including burst behavior.
func TestRetryController_ratelimit(t *testing.T) {
	keyFunc := func(obj interface{}) (string, error) {
		return "key", nil
	}
	fifo := kcache.NewResyncableFIFO(keyFunc)
	limiter := &mockLimiter{}
	retryManager := NewQueueRetryManager(fifo,
		keyFunc,
		func(_ interface{}, _ error, r Retry) bool {
			return r.Count < 15
		},
		limiter,
	)
	for i := 0; i < 10; i++ {
		retryManager.Retry("key", nil)
	}

	if limiter.count != 10 {
		t.Fatalf("Retries did not invoke rate limiter, expected %d got %d", 10, limiter.count)
	}
}

type mockLimiter struct {
	count int
}

func (l *mockLimiter) TryAccept() bool {
	return true
}

func (l *mockLimiter) Accept() {
	l.count++
}

func (l *mockLimiter) Stop() {}

func (l *mockLimiter) Saturation() float64 {
	return 0.0
}

func (l *mockLimiter) QPS() float32 {
	return 0.0
}

type testObj struct {
	id    string
	value int
}

type testFifo struct {
	AddIfNotPresentFunc func(interface{}) error
	PopFunc             func() interface{}
}

func (t *testFifo) AddIfNotPresent(obj interface{}) error {
	return t.AddIfNotPresentFunc(obj)
}

func (t *testFifo) Pop(fn kcache.PopProcessFunc) (interface{}, error) {
	obj := t.PopFunc()
	err := fn(obj)
	return obj, err
}

type testRetryManager struct {
	RetryFunc  func(resource interface{}, err error)
	ForgetFunc func(resource interface{})
}

func (m *testRetryManager) Retry(resource interface{}, err error) {
	m.RetryFunc(resource, err)
}

func (m *testRetryManager) Forget(resource interface{}) {
	m.ForgetFunc(resource)
}
