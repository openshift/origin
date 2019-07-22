package clusterquotareconciliation

import (
	"sync"

	"k8s.io/client-go/util/workqueue"
)

// BucketingWorkQueue gives a way to add items related to a single entry in a work queue
// this allows you work on a set of related work in a single UOW-style way
type BucketingWorkQueue interface {
	AddWithData(key interface{}, data ...interface{})
	AddWithDataRateLimited(key interface{}, data ...interface{})
	GetWithData() (key interface{}, data []interface{}, quit bool)
	Done(key interface{})
	Forget(key interface{})

	ShutDown()
}

func NewBucketingWorkQueue(name string) BucketingWorkQueue {
	return &workQueueBucket{
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
		work:       map[interface{}][]interface{}{},
		dirtyWork:  map[interface{}][]interface{}{},
		inProgress: map[interface{}]bool{},
	}
}

type workQueueBucket struct {
	// TODO these are used together to bucket items by namespace and then batch them up for processing.
	// The technique is valuable for rollup activities to avoid fanout and reduce resource contention.
	// We could move this into a library if another component needed it.
	// queue is indexed by namespace, so that we bundle up on a per-namespace basis
	queue      workqueue.RateLimitingInterface
	workLock   sync.Mutex
	work       map[interface{}][]interface{}
	dirtyWork  map[interface{}][]interface{}
	inProgress map[interface{}]bool
}

func (e *workQueueBucket) AddWithData(key interface{}, data ...interface{}) {
	e.workLock.Lock()
	defer e.workLock.Unlock()

	// this Add can trigger a Get BEFORE the work is added to a list, but this is ok because the getWork routine
	// waits the worklock before retrieving the work to do, so the writes in this method will be observed
	e.queue.Add(key)

	if e.inProgress[key] {
		e.dirtyWork[key] = append(e.dirtyWork[key], data...)
		return
	}

	e.work[key] = append(e.work[key], data...)
}

func (e *workQueueBucket) AddWithDataRateLimited(key interface{}, data ...interface{}) {
	e.workLock.Lock()
	defer e.workLock.Unlock()

	// this Add can trigger a Get BEFORE the work is added to a list, but this is ok because the getWork routine
	// waits the worklock before retrieving the work to do, so the writes in this method will be observed
	e.queue.AddRateLimited(key)

	if e.inProgress[key] {
		e.dirtyWork[key] = append(e.dirtyWork[key], data...)
		return
	}

	e.work[key] = append(e.work[key], data...)
}

func (e *workQueueBucket) Done(key interface{}) {
	e.workLock.Lock()
	defer e.workLock.Unlock()

	e.queue.Done(key)
	e.work[key] = e.dirtyWork[key]
	delete(e.dirtyWork, key)
	delete(e.inProgress, key)
}

func (e *workQueueBucket) Forget(key interface{}) {
	e.queue.Forget(key)
}

func (e *workQueueBucket) GetWithData() (interface{}, []interface{}, bool) {
	key, shutdown := e.queue.Get()
	if shutdown {
		return nil, []interface{}{}, shutdown
	}

	e.workLock.Lock()
	defer e.workLock.Unlock()
	// at this point, we know we have a coherent view of e.work.  It is entirely possible
	// that our workqueue has another item requeued to it, but we'll pick it up early.  This ok
	// because the next time will go into our dirty list

	work := e.work[key]
	delete(e.work, key)
	delete(e.dirtyWork, key)
	e.inProgress[key] = true

	if len(work) != 0 {
		return key, work, false
	}

	return key, []interface{}{}, false
}

func (e *workQueueBucket) ShutDown() {
	e.queue.ShutDown()
}
