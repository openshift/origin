package controller

import (
	kcache "github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// RunnableController is a controller which implements a Run loop.
type RunnableController interface {
	// Run starts the asynchronous controller loop.
	Run()
}

// RetryController is a RunnableController which delegates resource
// handling to a function and knows how to safely manage retries of a resource
// which failed to be successfully handled.
type RetryController struct {
	// Queue is where work is retrieved for Handle.
	Queue

	// Handle is expected to process the next resource from the queue.
	Handle func(interface{}) error

	// RetryManager is fed the handled resource if Handle returns a Retryable
	// error. If Handle returns no error, the RetryManager is asked to forget
	// the resource.
	RetryManager
}

// Queue is a narrow abstraction of a cache.FIFO.
type Queue interface {
	Pop() interface{}
}

// Run begins processing resources from Queue asynchronously.
func (c *RetryController) Run() {
	go kutil.Forever(func() { c.handleOne(c.Queue.Pop()) }, 0)
}

// RunUntil begins processing resources from Queue asynchronously until stopCh is closed.
func (c *RetryController) RunUntil(stopCh <-chan struct{}) {
	go kutil.Until(func() { c.handleOne(c.Queue.Pop()) }, 0, stopCh)
}

// handleOne processes resource with Handle. If Handle returns a retryable
// error, the handled resource is passed to the RetryManager. If no error is
// returned from Handle, the RetryManager is asked to forget the processed
// resource.
func (c *RetryController) handleOne(resource interface{}) {
	if err := c.Handle(resource); err != nil {
		c.Retry(resource, err)
		return
	}
	c.Forget(resource)
}

// RetryManager knows how to retry processing of a resource, and how to forget
// a resource it may be tracking the state of.
type RetryManager interface {
	// Retry will cause resource processing to be retried (for example, by
	// requeueing resource)
	Retry(resource interface{}, err error)

	// Forget will cause the manager to erase all prior knowledge of resource
	// and reclaim internal resources associated with state tracking of
	// resource.
	Forget(resource interface{})
}

// RetryFunc should return true if the given object and error should be retried after
// the provided number of times.
type RetryFunc func(obj interface{}, err error, retries Retry) bool

// RetryNever is a RetryFunc implementation that will never retry
func RetryNever(obj interface{}, err error, retries Retry) bool {
	return false
}

// RetryNever is a RetryFunc implementation that will always retry
func RetryAlways(obj interface{}, err error, retries Retry) bool {
	return true
}

// QueueRetryManager retries a resource by re-queueing it into a ReQueue as long as
// retryFunc returns true.
type QueueRetryManager struct {
	// queue is where resources are re-queued.
	queue ReQueue

	// keyFunc is used to index resources.
	keyFunc kcache.KeyFunc

	// retryFunc returns true if the resource and error returned should be retried.
	retryFunc RetryFunc

	// retries maps resources to their current retry
	retries map[string]Retry

	// limits how fast retries can be enqueued to ensure you can't tight
	// loop on retries.
	limiter kutil.RateLimiter
}

// Retry describes provides additional information regarding retries.
type Retry struct {
	// Count is the number of retries
	Count int

	// StartTimestamp is retry start timestamp
	StartTimestamp kutil.Time
}

// ReQueue is a queue that allows an object to be requeued
type ReQueue interface {
	Queue
	AddIfNotPresent(interface{}) error
}

// NewQueueRetryManager safely creates a new QueueRetryManager.
func NewQueueRetryManager(queue ReQueue, keyFn kcache.KeyFunc, retryFn RetryFunc, limiter kutil.RateLimiter) *QueueRetryManager {
	return &QueueRetryManager{
		queue:     queue,
		keyFunc:   keyFn,
		retryFunc: retryFn,
		retries:   make(map[string]Retry),
		limiter:   limiter,
	}
}

// Retry will enqueue resource until retryFunc returns false for that resource has been
// exceeded, at which point resource will be forgotten and no longer retried. The current
// retry count will be passed to each invocation of retryFunc.
func (r *QueueRetryManager) Retry(resource interface{}, err error) {
	id, _ := r.keyFunc(resource)

	if _, exists := r.retries[id]; !exists {
		r.retries[id] = Retry{0, kutil.Now()}
	}
	tries := r.retries[id]

	if r.retryFunc(resource, err, tries) {
		r.limiter.Accept()
		// It's important to use AddIfNotPresent to prevent overwriting newer
		// state in the queue which may have arrived asynchronously.
		r.queue.AddIfNotPresent(resource)
		tries.Count = tries.Count + 1
		r.retries[id] = tries
	} else {
		r.Forget(resource)
	}
}

// Forget resets the retry count for resource.
func (r *QueueRetryManager) Forget(resource interface{}) {
	id, _ := r.keyFunc(resource)
	delete(r.retries, id)
}
