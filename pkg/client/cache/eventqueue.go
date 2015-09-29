package cache

import (
	"fmt"
	"sync"

	kcache "k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"
)

// EventQueue is a Store implementation that provides a sequence of compressed events to a consumer
// along with event types.  This differs from the FIFO implementation in that FIFO does not provide
// events when an object is deleted and does not provide the type of event.  Events are compressed
// in a manner similar to FIFO, but accounting for event types and deletions.  The exact
// compression semantics are as follows:
//
// 1.  If a watch.Added is enqueued with state X and a watch.Modified with state Y is received,
//     these are compressed into (Added, Y)
//
// 2.  If a watch.Added is enqueued with state X and a watch.Deleted is received, these are
//     compressed and the item is removed from the queue
//
// 3.  If a watch.Modified is enqueued with state X and a watch.Modified with state Y is received,
//     these two events are compressed into (Modified, Y)
//
// 4.  If a watch.Modified is enqueued with state X and a watch.Deleted is received, these are
//     compressed into (Deleted, X)
//
// It should be noted that the scenario where an object is deleted and re-added is not handled by
// this type nor is it in scope; the reflector uses UIDs for the IDs passed to stores, so you will
// never see a delete and a re-add for the same ID.
//
// This type maintains a backing store in order to provide the deleted state on watch.Deleted
// events.  This is necessary because the Store API does not receive the deleted state on a
// watch.Deleted event (though this state is delivered by the watch API itself, it is not passed on
// to the reflector Store).
type EventQueue struct {
	lock   sync.RWMutex
	cond   sync.Cond
	store  kcache.Store
	keyFn  kcache.KeyFunc
	events map[string]watch.EventType
	queue  []string
}

// EventQueue implements kcache.Store
var _ kcache.Store = &EventQueue{}

// Describes the effect of processing a watch event on the event queue's state.
type watchEventEffect string

type EventQueueStopped struct{}

const (
	// The watch event should result in an add to the event queue
	watchEventEffectAdd watchEventEffect = "ADD"

	// The watch event should be compressed with an already enqueued event
	watchEventEffectCompress watchEventEffect = "COMPRESS"

	// The watch event should result in the ID being deleted from the queue
	watchEventEffectDelete watchEventEffect = "DELETE"
)

// The watch event effect matrix defines the valid event sequences and what their effects are on
// the state of the event queue.
//
// A watch event that produces an invalid sequence results in a panic.
var watchEventEffectMatrix = map[watch.EventType]map[watch.EventType]watchEventEffect{
	watch.Added: {
		watch.Modified: watchEventEffectCompress,
		watch.Deleted:  watchEventEffectDelete,
	},
	watch.Modified: {
		watch.Modified: watchEventEffectCompress,
		watch.Deleted:  watchEventEffectCompress,
	},
	watch.Deleted: {},
}

// The watch event compression matrix defines how two events should be compressed.
var watchEventCompressionMatrix = map[watch.EventType]map[watch.EventType]watch.EventType{
	watch.Added: {
		watch.Modified: watch.Added,
	},
	watch.Modified: {
		watch.Modified: watch.Modified,
		watch.Deleted:  watch.Deleted,
	},
	watch.Deleted: {},
}

func (es EventQueueStopped) Error() string {
	return fmt.Sprintf("Event queue was stopped.")
}

// handleEvent is called by Add, Update, and Delete to determine the effect
// of an event of the queue, realize that effect, and update the underlying store.
func (eq *EventQueue) handleEvent(obj interface{}, newEventType watch.EventType) error {
	key, err := eq.keyFn(obj)
	if err != nil {
		return err
	}

	eq.lock.Lock()
	defer eq.lock.Unlock()

	var (
		queuedEventType watch.EventType
		effect          watchEventEffect
		ok              bool
	)

	queuedEventType, ok = eq.events[key]
	if !ok {
		effect = watchEventEffectAdd
	} else {
		effect, ok = watchEventEffectMatrix[queuedEventType][newEventType]
		if !ok {
			panic(fmt.Sprintf("Invalid state transition: %v -> %v", queuedEventType, newEventType))
		}
	}

	if err := eq.updateStore(key, obj, newEventType); err != nil {
		return err
	}

	switch effect {
	case watchEventEffectAdd:
		eq.events[key] = newEventType
		eq.queue = append(eq.queue, key)
		eq.cond.Broadcast()
	case watchEventEffectCompress:
		newEventType, ok := watchEventCompressionMatrix[queuedEventType][newEventType]
		if !ok {
			panic(fmt.Sprintf("Invalid state transition: %v -> %v", queuedEventType, newEventType))
		}

		eq.events[key] = newEventType
	case watchEventEffectDelete:
		delete(eq.events, key)
		eq.queue = eq.queueWithout(key)
	}
	return nil
}

// Cancel function to force Pop function to unblock
func (eq *EventQueue) Cancel() {
	eq.cond.Broadcast()
}

// updateStore updates the stored value for the given key.  Note that deletions are not handled
// here; they are performed in Pop in order to provide the deleted value on watch.Deleted events.
func (eq *EventQueue) updateStore(key string, obj interface{}, eventType watch.EventType) error {
	if eventType == watch.Deleted {
		return nil
	}

	var err error
	if eventType == watch.Added {
		err = eq.store.Add(obj)
	} else {
		err = eq.store.Update(obj)
	}
	return err
}

// queueWithout returns the internal queue minus the given key.
func (eq *EventQueue) queueWithout(key string) []string {
	rq := make([]string, 0)
	for _, qkey := range eq.queue {
		if qkey == key {
			continue
		}

		rq = append(rq, qkey)
	}

	return rq
}

// Add enqueues a watch.Added event for the given state.
func (eq *EventQueue) Add(obj interface{}) error {
	return eq.handleEvent(obj, watch.Added)
}

// Update enqueues a watch.Modified event for the given state.
func (eq *EventQueue) Update(obj interface{}) error {
	return eq.handleEvent(obj, watch.Modified)
}

// Delete enqueues a watch.Delete event for the given object.
func (eq *EventQueue) Delete(obj interface{}) error {
	return eq.handleEvent(obj, watch.Deleted)
}

// List returns a list of all enqueued items.
func (eq *EventQueue) List() []interface{} {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	list := make([]interface{}, 0, len(eq.queue))
	for _, key := range eq.queue {
		item, ok, err := eq.store.GetByKey(key)
		if err != nil {
			panic(fmt.Sprintf("Failure to get by key %q: %v", key, err))
		}
		if !ok {
			panic(fmt.Sprintf("Tried to list an ID not in backing store: %v", key))
		}
		list = append(list, item)
	}

	return list
}

// ListKeys returns all enqueued keys.
func (eq *EventQueue) ListKeys() []string {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	list := make([]string, 0, len(eq.queue))
	copy(list, eq.queue)
	return list
}

// ContainedIDs returns a sets.String containing all IDs of the enqueued items.
// This is a snapshot of a moment in time, and one should keep in mind that
// other go routines can add or remove items after you call this.
func (eq *EventQueue) ContainedIDs() sets.String {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	s := sets.String{}
	for _, key := range eq.queue {
		s.Insert(key)
	}

	return s
}

// Get returns the requested item, or sets exists=false.
func (eq *EventQueue) Get(obj interface{}) (item interface{}, exists bool, err error) {
	key, err := eq.keyFn(obj)
	if err != nil {
		return nil, false, err
	}
	return eq.GetByKey(key)
}

// GetByKey returns the requested item, or sets exists=false.
func (eq *EventQueue) GetByKey(key string) (item interface{}, exists bool, err error) {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	_, ok := eq.events[key]
	if !ok {
		return nil, false, nil
	}

	return eq.store.GetByKey(key) // Should always be populated and succeed
}

// Pop gets the event and object at the head of the queue.  If the event
// is a delete event, Pop deletes the key from the underlying cache.
func (eq *EventQueue) Pop() (watch.EventType, interface{}, error) {
	eq.lock.Lock()
	defer eq.lock.Unlock()

	for {
		for len(eq.queue) == 0 {
			eq.cond.Wait()
		}

		if len(eq.queue) == 0 {
			return watch.Error, nil, EventQueueStopped{}
		}
		key := eq.queue[0]
		eq.queue = eq.queue[1:]

		eventType := eq.events[key]
		delete(eq.events, key)

		obj, exists, err := eq.store.GetByKey(key) // Should always succeed
		if err != nil {
			return watch.Error, nil, err
		}
		if !exists {
			panic(fmt.Sprintf("Pop() of key not in store: %v", key))
		}

		if eventType == watch.Deleted {
			if err := eq.store.Delete(obj); err != nil {
				return watch.Error, nil, err
			}
		}

		return eventType, obj, nil
	}
}

// Replace initializes 'eq' with the state contained in the given map and
// populates the queue with a watch.Modified event for each of the replaced
// objects.  The backing store takes ownership of keyToObjs; you should not
// reference the map again after calling this function.
func (eq *EventQueue) Replace(objects []interface{}, resourceVersion string) error {
	eq.lock.Lock()
	defer eq.lock.Unlock()

	eq.events = map[string]watch.EventType{}
	eq.queue = eq.queue[:0]

	for i := range objects {
		key, err := eq.keyFn(objects[i])
		if err != nil {
			return err
		}
		eq.queue = append(eq.queue, key)
		eq.events[key] = watch.Modified
	}
	if err := eq.store.Replace(objects, resourceVersion); err != nil {
		return err
	}

	if len(eq.queue) > 0 {
		eq.cond.Broadcast()
	}
	return nil
}

// NewEventQueue returns a new EventQueue.
func NewEventQueue(keyFn kcache.KeyFunc) *EventQueue {
	q := &EventQueue{
		store:  kcache.NewStore(keyFn),
		events: map[string]watch.EventType{},
		queue:  []string{},
		keyFn:  keyFn,
	}
	q.cond.L = &q.lock
	return q
}

// NewEventQueueForStore returns a new EventQueue that uses the provided store.
func NewEventQueueForStore(keyFn kcache.KeyFunc, store kcache.Store) *EventQueue {
	q := &EventQueue{
		store:  store,
		events: map[string]watch.EventType{},
		queue:  []string{},
		keyFn:  keyFn,
	}
	q.cond.L = &q.lock
	return q
}
