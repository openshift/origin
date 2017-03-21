package factory

import (
	"os"

	"github.com/golang/glog"
	oscache "github.com/openshift/origin/pkg/client/cache"
	"k8s.io/kubernetes/pkg/watch"
)

// TrapperFunc defines the signature for a EventQueueTrapperFunc.
type TrapperFunc func()

// EventQueueTrapper is a Store implementation that catches failures in
// the underlying EventQueue and calls the trapper function.
type EventQueueTrapper struct {
	queue   *oscache.EventQueue
	trapper TrapperFunc
}

// NewEventQueueTrapper returns a new EventQueueTrapper.
func NewEventQueueTrapper(queue *oscache.EventQueue, trapper TrapperFunc) *EventQueueTrapper {
	return &EventQueueTrapper{queue, trapper}
}

// trapHandler checks if the event queue code executed in a goroutine traps
// and if so, exits the program.
func (t *EventQueueTrapper) trapHandler() {
	if r := recover(); r != nil {
		glog.Errorf("EventQueueTrapper handler caught a panic")

		if t.trapper == nil {
			os.Exit(70) // Internal software error.
		}

		t.trapper()
	}
}

// Add enqueues a watch.Added event for the given state.
func (t *EventQueueTrapper) Add(obj interface{}) error {
	defer t.trapHandler()
	return t.queue.Add(obj)
}

// Update enqueues a watch.Modified event for the given state.
func (t *EventQueueTrapper) Update(obj interface{}) error {
	defer t.trapHandler()
	return t.queue.Update(obj)
}

// Delete enqueues a watch.Delete event for the given object.
func (t *EventQueueTrapper) Delete(obj interface{}) error {
	defer t.trapHandler()
	return t.queue.Delete(obj)
}

// List returns a list of all enqueued items.
func (t *EventQueueTrapper) List() []interface{} {
	defer t.trapHandler()
	return t.queue.List()
}

// ListKeys returns all enqueued keys.
func (t *EventQueueTrapper) ListKeys() []string {
	defer t.trapHandler()
	return t.queue.ListKeys()
}

// Get returns the requested item, or sets exists=false.
func (t *EventQueueTrapper) Get(obj interface{}) (item interface{}, exists bool, err error) {
	defer t.trapHandler()
	return t.queue.Get(obj)
}

// GetByKey returns the requested item, or sets exists=false.
func (t *EventQueueTrapper) GetByKey(key string) (item interface{}, exists bool, err error) {
	defer t.trapHandler()
	return t.queue.GetByKey(key)
}

// Replace initializes 'eq' with the state contained in the given map and
// populates the queue with a watch.Modified event for each of the replaced
// objects.  The backing store takes ownership of keyToObjs; you should not
// reference the map again after calling this function.
func (t *EventQueueTrapper) Replace(objects []interface{}, resourceVersion string) error {
	defer t.trapHandler()
	return t.queue.Replace(objects, resourceVersion)
}

// Resync will touch all objects to put them into the processing queue
func (t *EventQueueTrapper) Resync() error {
	defer t.trapHandler()
	return t.queue.Resync()
}

// Pop gets the event and object at the head of the queue.  If the event
// is a delete event, Pop deletes the key from the underlying cache.
func (t *EventQueueTrapper) Pop() (watch.EventType, interface{}, error) {
	defer t.trapHandler()
	return t.queue.Pop()
}

// ListSuccessfulAtLeastOnce indicates whether a List operation was
// successfully completed regardless of whether any items were queued.
func (t *EventQueueTrapper) ListSuccessfulAtLeastOnce() bool {
	defer t.trapHandler()
	return t.queue.ListSuccessfulAtLeastOnce()
}

// ListCount returns how many objects were queued by the most recent List operation.
func (t *EventQueueTrapper) ListCount() int {
	defer t.trapHandler()
	return t.queue.ListCount()
}

// ListConsumed indicates whether the items queued by a List/Relist
// operation have been consumed.
func (t *EventQueueTrapper) ListConsumed() bool {
	defer t.trapHandler()
	return t.queue.ListConsumed()
}
