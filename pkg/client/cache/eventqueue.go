package cache

import (
	"fmt"
	"sync"

	kcache "github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
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
	events map[string]watch.EventType
	queue  []string
}

// Describes the effect of processing a watch event on the event queue's state.
type watchEventEffect string

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

// handleEvent is called by Add, Update, and Delete to determine the effect
// of an event of the queue, realize that effect, and update the underlying store.
func (eq *EventQueue) handleEvent(id string, obj interface{}, newEventType watch.EventType) {
	eq.lock.Lock()
	defer eq.lock.Unlock()

	var (
		queuedEventType watch.EventType
		effect          watchEventEffect
		ok              bool
	)

	queuedEventType, ok = eq.events[id]
	if !ok {
		effect = watchEventEffectAdd
	} else {
		effect, ok = watchEventEffectMatrix[queuedEventType][newEventType]
		if !ok {
			panic(fmt.Sprintf("Invalid state transition: %v -> %v", queuedEventType, newEventType))
		}
	}

	eq.updateStore(id, obj, newEventType)

	switch effect {
	case watchEventEffectAdd:
		eq.events[id] = newEventType
		eq.queue = append(eq.queue, id)
		eq.cond.Broadcast()
	case watchEventEffectCompress:
		newEventType, ok := watchEventCompressionMatrix[queuedEventType][newEventType]
		if !ok {
			panic(fmt.Sprintf("Invalid state transition: %v -> %v", queuedEventType, newEventType))
		}

		eq.events[id] = newEventType
	case watchEventEffectDelete:
		delete(eq.events, id)
		eq.queue = eq.queueWithout(id)
	}
}

// updateStore updates the stored value for the given id.  Note that deletions are not handled
// here; they are performed in Pop in order to provide the deleted value on watch.Deleted events.
func (eq *EventQueue) updateStore(id string, obj interface{}, eventType watch.EventType) {
	if eventType == watch.Deleted {
		return
	}

	if eventType == watch.Added {
		eq.store.Add(id, obj)
	} else {
		eq.store.Update(id, obj)
	}
}

// queueWithout returns the internal queue minus the given id.
func (eq *EventQueue) queueWithout(id string) []string {
	rq := make([]string, 0)
	for _, qid := range eq.queue {
		if qid == id {
			continue
		}

		rq = append(rq, qid)
	}

	return rq
}

// Add enqueues a watch.Added event for the given id and state.
func (eq *EventQueue) Add(id string, obj interface{}) {
	eq.handleEvent(id, obj, watch.Added)
}

// Update enqueues a watch.Modified event for the given id and state.
func (eq *EventQueue) Update(id string, obj interface{}) {
	eq.handleEvent(id, obj, watch.Modified)
}

// Delete enqueues a watch.Delete event for the given id.
func (eq *EventQueue) Delete(id string) {
	eq.handleEvent(id, nil, watch.Deleted)
}

// List returns a list of all enqueued items.
func (eq *EventQueue) List() []interface{} {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	var (
		item interface{}
		ok   bool
	)

	list := make([]interface{}, 0, len(eq.queue))
	for _, id := range eq.queue {
		item, ok = eq.store.Get(id)
		if !ok {
			panic(fmt.Sprintf("Tried to list an ID not in backing store: %v", id))
		}
		list = append(list, item)
	}

	return list
}

// ContainedIDs returns a util.StringSet containing all IDs of the enqueued items.
// This is a snapshot of a moment in time, and one should keep in mind that
// other go routines can add or remove items after you call this.
func (eq *EventQueue) ContainedIDs() util.StringSet {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	s := util.StringSet{}
	for _, id := range eq.queue {
		s.Insert(id)
	}

	return s
}

// Get returns the requested item, or sets exists=false.
func (eq *EventQueue) Get(id string) (item interface{}, exists bool) {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	_, ok := eq.events[id]
	if !ok {
		return nil, false
	}

	return eq.store.Get(id) // Should always be populated and succeed
}

// Pop gets the event and object at the head of the queue.  If the event
// is a delete event, Pop deletes the id from the underlying cache.
func (eq *EventQueue) Pop() (watch.EventType, interface{}) {
	eq.lock.Lock()
	defer eq.lock.Unlock()

	for {
		for len(eq.queue) == 0 {
			eq.cond.Wait()
		}

		id := eq.queue[0]
		eq.queue = eq.queue[1:]

		eventType := eq.events[id]
		delete(eq.events, id)

		obj, exists := eq.store.Get(id) // Should always succeed
		if !exists {
			panic(fmt.Sprintf("Pop() of id not in store: %v", id))
		}

		if eventType == watch.Deleted {
			eq.store.Delete(id)
		}

		return eventType, obj
	}
}

// Replace initializes 'eq' with the state contained in the given map and
// populates the queue with a watch.Modified event for each of the replaced
// objects.  The backing store takes ownership of idToObjs; you should not
// reference the map again after calling this function.
func (eq *EventQueue) Replace(idToObjs map[string]interface{}) {
	eq.lock.Lock()
	defer eq.lock.Unlock()

	eq.events = map[string]watch.EventType{}
	eq.queue = eq.queue[:0]

	for id := range idToObjs {
		eq.queue = append(eq.queue, id)
		eq.events[id] = watch.Modified
	}
	eq.store.Replace(idToObjs)

	if len(eq.queue) > 0 {
		eq.cond.Broadcast()
	}
}

// NewEventQueue returns a new EventQueue ready for action.
func NewEventQueue() *EventQueue {
	q := &EventQueue{
		store:  kcache.NewStore(),
		events: map[string]watch.EventType{},
		queue:  []string{},
	}
	q.cond.L = &q.lock
	return q
}
