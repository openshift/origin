package plugin

import (
	"fmt"
	"reflect"

	"k8s.io/kubernetes/pkg/client/cache"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

// EventQueue is an enhanced DeltaFIFO that provides reliable Deleted deltas
// even if no knownObjects store is given, and compresses multiple deltas
// to reduce duplicate events.
//
// Without a store, DeltaFIFO will drop Deleted deltas when its queue is empty
// because the deleted object is not present in the queue and DeltaFIFO tries
// to protect against duplicate Deleted deltas resulting from Replace().
//
// To get reliable deletion, a store must be provided, and EventQueue provides
// one if the caller does not.
type EventQueue struct {
	*cache.DeltaFIFO

	// Private store if not intitialized with one to ensure deletion
	// events are always recognized.
	knownObjects cache.Store
}

func DeletionHandlingMetaNamespaceKeyFunc(obj interface{}) (string, error) {
	if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		return d.Key, nil
	}
	return cache.MetaNamespaceKeyFunc(obj)
}

func NewEventQueue(keyFunc cache.KeyFunc) *EventQueue {
	knownObjects := cache.NewStore(keyFunc)
	return &EventQueue{
		DeltaFIFO: cache.NewDeltaFIFO(
			keyFunc,
			cache.DeltaCompressorFunc(func(d cache.Deltas) cache.Deltas {
				return deltaCompressor(d, keyFunc)
			}),
			knownObjects),
		knownObjects: knownObjects,
	}
}

func NewEventQueueForStore(keyFunc cache.KeyFunc, knownObjects cache.KeyListerGetter) *EventQueue {
	return &EventQueue{
		DeltaFIFO: cache.NewDeltaFIFO(
			keyFunc,
			cache.DeltaCompressorFunc(func(d cache.Deltas) cache.Deltas {
				return deltaCompressor(d, keyFunc)
			}),
			knownObjects),
	}
}

func (queue *EventQueue) updateKnownObjects(delta cache.Delta) {
	switch delta.Type {
	case cache.Added:
		queue.knownObjects.Add(delta.Object)
	case cache.Updated:
		queue.knownObjects.Update(delta.Object)
	case cache.Sync:
		if _, ok, _ := queue.knownObjects.Get(delta.Object); ok {
			queue.knownObjects.Update(delta.Object)
		} else {
			queue.knownObjects.Add(delta.Object)
		}
	case cache.Deleted:
		queue.knownObjects.Delete(delta.Object)
	}
}

// Function should process one object delta, which represents a change notification
// for a single object. Function is passed the delta, which contains the
// changed object or the deleted final object state. The deleted final object
// state is extracted from the DeletedFinalStateUnknown passed by DeltaFIFO.
type ProcessEventFunc func(delta cache.Delta) error

// Process queued changes for an object.  The 'process' function is called
// repeatedly with each available cache.Delta that describes state changes
// for that object. If the process function returns an error queued changes
// for that object are dropped but processing continues with the next available
// object's cache.Deltas.  The error is logged with call stack information.
func (queue *EventQueue) Pop(process ProcessEventFunc, expectedType interface{}) (interface{}, error) {
	return queue.DeltaFIFO.Pop(func(obj interface{}) error {
		// Oldest to newest delta lists
		for _, delta := range obj.(cache.Deltas) {
			// Update private store to track object deletion
			if queue.knownObjects != nil {
				queue.updateKnownObjects(delta)
			}

			// Handle DeletedFinalStateUnknown delta objects
			var err error
			if expectedType != nil {
				delta.Object, err = extractDeltaObject(delta, expectedType)
				if err != nil {
					utilruntime.HandleError(err)
					return nil
				}
			}

			// Process one delta for the object
			if err = process(delta); err != nil {
				utilruntime.HandleError(fmt.Errorf("event processing failed: %v", err))
				return nil
			}
		}
		return nil
	})
}

// Helper function to extract the object from a Delta (including special handling
// of DeletedFinalStateUnknown delta objects) and check its type against
// an expected type.  The contained object is only returned if it matches the
// expected type, otherwise an error is returned.
func extractDeltaObject(delta cache.Delta, expectedType interface{}) (interface{}, error) {
	deltaObject := delta.Object
	if deleted, ok := deltaObject.(cache.DeletedFinalStateUnknown); ok {
		deltaObject = deleted.Obj
	}
	if reflect.TypeOf(deltaObject) != reflect.TypeOf(expectedType) {
		return nil, fmt.Errorf("event processing failed: got delta object type %T but wanted type %T", deltaObject, expectedType)
	}
	return deltaObject, nil
}

// Describes the action to take for a given combination of deltas
type actionType string

const (
	// The delta combination should result in the delta being added to the compressor cache
	actionAdd actionType = "ADD"

	// The delta combination should should be compressed into a single delta
	actionCompress actionType = "COMPRESS"

	// The delta combination should result in the object being deleted from the compressor cache
	actionDelete actionType = "DELETE"
)

type deltaAction struct {
	// The action to take for the delta combination
	action actionType
	// The type for the new compressed delta
	deltaType cache.DeltaType
}

// The delta combination action matrix defines the valid delta sequences and
// how to compress specific combinations of deltas.
//
// A delta combination that produces an invalid sequence results in a panic.
var deltaActionMatrix = map[cache.DeltaType]map[cache.DeltaType]deltaAction{
	cache.Added: {
		cache.Sync:    {actionCompress, cache.Added},
		cache.Updated: {actionCompress, cache.Added},
		cache.Deleted: {actionDelete, cache.Deleted},
	},
	cache.Sync: {
		cache.Sync:    {actionCompress, cache.Sync},
		cache.Updated: {actionCompress, cache.Sync},
		cache.Deleted: {actionCompress, cache.Deleted},
	},
	cache.Updated: {
		cache.Updated: {actionCompress, cache.Updated},
		cache.Deleted: {actionCompress, cache.Deleted},
	},
	cache.Deleted: {
		cache.Added: {actionCompress, cache.Updated},
		cache.Sync:  {actionCompress, cache.Sync},
	},
}

func removeDeltasWithKey(deltas cache.Deltas, removeKey string, keyFunc cache.KeyFunc) cache.Deltas {
	newDeltas := cache.Deltas{}
	for _, d := range deltas {
		key, err := keyFunc(d.Object)
		if err == nil && key != removeKey {
			newDeltas = append(newDeltas, d)
		}
	}
	return newDeltas
}

// This DeltaFIFO compressor combines deltas for the same object, the exact
// compression semantics of which are as follows:
//
// 1.  If a cache.Added/cache.Sync is enqueued with state X and a cache.Updated with state Y
//     is received, these are compressed into (Added/Sync, Y)
//
// 2.  If a cache.Added is enqueued with state X and a cache.Deleted is received with state Y,
//     these are dropped and consumers will not see either event
//
// 3.  If a cache.Sync/cache.Updated is enqueued with state X and a cache.Deleted
//     is received with state Y, these are compressed into (Deleted, Y)
//
// 4.  If a cache.Updated is enqueued with state X and a cache.Updated with state Y is received,
//     these two events are compressed into (Updated, Y)
//
// 5.  If a cache.Added is enqueued with state X and a cache.Sync with state Y is received,
//     these are compressed into (Added, Y)
//
// 6.  If a cache.Sync is enqueued with state X and a cache.Sync with state Y is received,
//     these are compressed into (Sync, Y)
//
// 7.  Invalid combinations (eg, Sync + Added or Updated + Added) result in a panic.
//
// This function will compress all events for the same object into a single delta.
func deltaCompressor(deltas cache.Deltas, keyFunc cache.KeyFunc) cache.Deltas {
	// Final compressed deltas list
	newDeltas := cache.Deltas{}

	// Cache of object's current state including previous deltas
	objects := make(map[string]cache.DeltaType)

	// Deltas range from oldest (index 0) to newest (last index)
	for _, d := range deltas {
		key, err := keyFunc(d.Object)
		if err != nil {
			panic(fmt.Sprintf("unkeyable object: %v, %v", d.Object, err))
		}

		var compressAction deltaAction
		if oldType, ok := objects[key]; !ok {
			compressAction = deltaAction{actionAdd, d.Type}
		} else {
			// Older event exists; combine them
			compressAction, ok = deltaActionMatrix[oldType][d.Type]
			if !ok {
				panic(fmt.Sprintf("invalid state transition: %v -> %v", oldType, d.Type))
			}
		}

		switch compressAction.action {
		case actionAdd:
			newDeltas = append(newDeltas, d)
			objects[key] = d.Type
		case actionCompress:
			newDelta := cache.Delta{
				Type:   compressAction.deltaType,
				Object: d.Object,
			}
			objects[key] = newDelta.Type
			newDeltas = removeDeltasWithKey(newDeltas, key, keyFunc)
			newDeltas = append(newDeltas, newDelta)
		case actionDelete:
			delete(objects, key)
			newDeltas = removeDeltasWithKey(newDeltas, key, keyFunc)
		}
	}

	return newDeltas
}
