package eventqueue

import (
	"fmt"
	"sync"

	"github.com/golang/glog"

	kcache "k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/util/uuid"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"
)

// Event is an item added to the EventQueue.
type Event struct {
	id        string
	eventType watch.EventType
	obj       interface{}
}

// EventQueue is a Store implementation that provides an ordered sequence of
// watched events (type and the affected object) to the consumer.
type EventQueue struct {
	lock sync.RWMutex
	cond sync.Cond

	store kcache.Store
	keyFn kcache.KeyFunc

	// List of generated events.
	events map[string]*Event

	// Queue containing the events ids to be processed.
	queue *workqueue.Type

	// Tracks whether Replace() has been called at least once.
	replaced bool

	// Tracks the last event added to the queue by the most recent
	// Replace() call. A reflector replaces the queue contents on a
	// re-list by calling Replace(), so a non-empty key indicates that
	// the event has been consumed (read via Pop()).
	lastReplaceEventId string

	// Tracks the number of items queued by the last Replace() call.
	lastReplaceCount int
}

// EventQueueStopped conveys that the EventQueue has been shutdown.
type EventQueueStopped struct{}

// EventQueue implements kcache.Store
var _ kcache.Store = &EventQueue{}

// Error returns a messsage indicating that the event queue was shutdown.
func (es EventQueueStopped) Error() string {
	return fmt.Sprintf("Event queue was stopped.")
}

// clearEvents resets the event queue - clearing all the events.
func (eq *EventQueue) clearEvents() {
	eq.lock.Lock()
	defer eq.lock.Unlock()

	eq.events = map[string]*Event{}
}

// getEvents returns a list of events currently in the event queue.
func (eq *EventQueue) eventList() []*Event {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	list := make([]*Event, 0, len(eq.events))
	for _, ev := range eq.events {
		list = append(list, ev)
	}

	return list
}

// updateStore updates the local object store based on the event type.
func (eq *EventQueue) updateStore(obj interface{}, eventType watch.EventType) error {
	switch eventType {
	case watch.Deleted:
		return eq.store.Delete(obj)
	case watch.Added:
		return eq.store.Add(obj)
	case watch.Modified:
		return eq.store.Update(obj)
	}

	return nil
}

// handleEvent is called by Add, Update, Delete, Reload and Resync to create
// an entry in the underlying queue and optionally update the object store.
func (eq *EventQueue) handleEvent(obj interface{}, eventType watch.EventType, update bool) (string, error) {
	eq.lock.Lock()
	defer eq.lock.Unlock()

	eventId := fmt.Sprintf("%s", uuid.NewUUID())
	eq.events[eventId] = &Event{eventId, eventType, obj}
	eq.queue.Add(eventId)

	if update {
		if err := eq.updateStore(obj, eventType); err != nil {
			return "", err
		}
	}

	return eventId, nil
}

// Add enqueues a watch.Added event for the given state.
func (eq *EventQueue) Add(obj interface{}) error {
	_, err := eq.handleEvent(obj, watch.Added, true)
	return err
}

// Update enqueues a watch.Modified event for the given state.
func (eq *EventQueue) Update(obj interface{}) error {
	_, err := eq.handleEvent(obj, watch.Modified, true)
	return err
}

// Delete enqueues a watch.Delete event for the given object.
func (eq *EventQueue) Delete(obj interface{}) error {
	_, err := eq.handleEvent(obj, watch.Deleted, true)
	return err
}

// List returns a list of objects associated with the enqueued events.
func (eq *EventQueue) List() []interface{} {
	events := eq.eventList()

	list := make([]interface{}, 0, len(events))
	for _, ev := range events {
		list = append(list, ev.obj)
	}

	return list
}

// List returns a list of keys of the objects associated with the enqueued events.
func (eq *EventQueue) ListKeys() []string {
	events := eq.eventList()

	list := make([]string, 0, len(events))
	for _, ev := range events {
		key, err := eq.keyFn(ev.obj)
		if err != nil {
			glog.V(4).Infof("invalid object %v, skipped for listing", ev.obj)
		} else {
			list = append(list, key)
		}
	}

	return list
}

// Get returns the requested item, or sets exists=false.
func (eq *EventQueue) Get(obj interface{}) (item interface{}, exists bool, err error) {
	key, err := eq.keyFn(obj)
	if err != nil {
		return nil, false, err
	}

	return eq.store.GetByKey(key)
}

// GetByKey returns the requested item, or sets exists=false.
func (eq *EventQueue) GetByKey(key string) (item interface{}, exists bool, err error) {
	return eq.store.GetByKey(key)
}

// Pop gets the event and object at the head of the queue.  If the event
// is a delete event, Pop deletes the key from the underlying cache.
func (eq *EventQueue) Pop() (watch.EventType, interface{}, error) {
	for {
		id, shutdown := eq.queue.Get()
		if shutdown {
			return watch.Error, nil, EventQueueStopped{}
		}

		eventId := id.(string)

		eq.lock.Lock()
		defer eq.lock.Unlock()

		qevent, ok := eq.events[eventId]
		if !ok {
			// Note: The work queue may have "dirty" entries or
			//       we have replaced the events.
			// If we don't have the event, then skip the work
			// queue entry but mark it as processed.
			eq.queue.Done(id)
			continue
		}

		// If this is the last replace event we are processing,
		// reset it indicating that we have now processed all the
		// replaced cache entries.
		if eq.lastReplaceEventId == eventId {
			eq.lastReplaceEventId = ""
		}

		// We have the event - remove it from the event queue and
		// mark the work queue entry as processed.
		delete(eq.events, eventId)
		eq.queue.Done(id)

		return qevent.eventType, qevent.obj, nil
	}
}

// Replace initializes 'eq' with the state contained in the given map and
// populates the events with a watch.Modified event for each of the replaced
// objects.
func (eq *EventQueue) Replace(objects []interface{}, resourceVersion string) error {
	eq.clearEvents()
	lastEventId := ""
	for i := range objects {
		lastEventId, _ = eq.handleEvent(objects[i], watch.Modified, false)
	}

	eq.lock.Lock()
	defer eq.lock.Unlock()

	eq.lastReplaceCount = len(objects)
	eq.lastReplaceEventId = lastEventId
	eq.replaced = true

	return nil
}

// ListSuccessfulAtLeastOnce indicates whether a List operation was
// successfully completed regardless of whether any items were queued.
func (eq *EventQueue) ListSuccessfulAtLeastOnce() bool {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	return eq.replaced
}

// ListCount returns how many objects were queued by the most recent List operation.
func (eq *EventQueue) ListCount() int {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	return eq.lastReplaceCount
}

// ListConsumed indicates whether the items queued by a List/Relist
// operation have been consumed.
func (eq *EventQueue) ListConsumed() bool {
	eq.lock.RLock()
	defer eq.lock.RUnlock()

	return eq.lastReplaceEventId == ""
}

// Resync will add all currently stored objects to the processing queue.
func (eq *EventQueue) Resync() error {
	lastEventId := ""
	storeKeys := eq.store.ListKeys()
	for i := range storeKeys {
		obj, exists, err := eq.store.GetByKey(storeKeys[i])
		if err == nil && exists {
			lastEventId, _ = eq.handleEvent(obj, watch.Modified, false)
		}
	}

	eq.lock.Lock()
	defer eq.lock.Unlock()

	eq.lastReplaceEventId = lastEventId

	return nil
}

// NewEventQueueForStore returns a new EventQueue that uses the provided store.
func NewEventQueueForStore(keyFn kcache.KeyFunc, store kcache.Store) *EventQueue {
	eq := &EventQueue{
		store:  store,
		keyFn:  keyFn,
		events: make(map[string]*Event, 0),
		queue:  workqueue.New(),
	}
	eq.cond.L = &eq.lock
	return eq
}

// NewEventQueue returns a new EventQueue.
func NewEventQueue(keyFn kcache.KeyFunc) *EventQueue {
	return NewEventQueueForStore(keyFn, kcache.NewStore(keyFn))
}
