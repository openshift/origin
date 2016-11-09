package plugin

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
)

func testKeyFunc(obj interface{}) (string, error) {
	if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		return d.Key, nil
	}
	key, ok := obj.(string)
	if !ok {
		return "", cache.KeyError{Obj: obj, Err: fmt.Errorf("object not a string")}
	}
	return key, nil
}

func deltaCompress(deltas cache.Deltas, keyFunc cache.KeyFunc) (newDeltas cache.Deltas, panicked bool, msg string) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			msg = fmt.Sprintf("%#v", r)
		}
	}()

	newDeltas = deltaCompressor(deltas, keyFunc)
	return
}

func compressTestDesc(test compressTest) string {
	var start, result []string
	for _, delta := range test.initial {
		start = append(start, string(delta.Type))
	}
	for _, delta := range test.compressed {
		result = append(result, string(delta.Type))
	}
	return strings.Join(start, "+") + "=" + strings.Join(result, "+")
}

type compressTest struct {
	initial     cache.Deltas
	compressed  cache.Deltas
	expectPanic bool
}

// Test the delta compressor on its own
func TestEventQueueDeltaCompressor(t *testing.T) {
	tests := []compressTest{
		// 1.  If a cache.Added/cache.Sync is enqueued with state X and a cache.Updated with state Y
		//     is received, these are compressed into (Added/Sync, Y)
		{
			initial: cache.Deltas{
				{Type: cache.Added, Object: "obj1"},
				{Type: cache.Updated, Object: "obj1"},
				{Type: cache.Updated, Object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Added, Object: "obj1"},
			},
		},

		// 1.  If a cache.Added/cache.Sync is enqueued with state X and a cache.Updated with state Y
		//     is received, these are compressed into (Added/Sync, Y)
		{
			initial: cache.Deltas{
				{Type: cache.Added, Object: "obj1"},
				// test that a second object doesn't affect compression of the first
				{Type: cache.Added, Object: "obj2"},
				{Type: cache.Updated, Object: "obj2"},
				{Type: cache.Updated, Object: "obj1"},
				{Type: cache.Updated, Object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Added, Object: "obj2"},
				{Type: cache.Added, Object: "obj1"},
			},
		},

		// 1.  If a cache.Added/cache.Sync is enqueued with state X and a cache.Updated with state Y
		//     is received, these are compressed into (Added/Sync, Y)
		{
			initial: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
				{Type: cache.Updated, Object: "obj1"},
				{Type: cache.Updated, Object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
			},
		},

		// 1.  If a cache.Added/cache.Sync is enqueued with state X and a cache.Updated with state Y
		//     is received, these are compressed into (Added/Sync, Y)
		{
			initial: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
				{Type: cache.Updated, Object: "obj1"},
				// test that a second object doesn't affect compression of the first
				{Type: cache.Added, Object: "obj2"},
				{Type: cache.Updated, Object: "obj2"},
				{Type: cache.Updated, Object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Added, Object: "obj2"},
				{Type: cache.Sync, Object: "obj1"},
			},
		},

		// 2.  If a cache.Added is enqueued with state X and a cache.Deleted is received with state Y,
		//     these are dropped and consumers will not see either event
		{
			initial: cache.Deltas{
				{Type: cache.Added, Object: "obj1"},
				// test that a second object doesn't affect compression of the first
				{Type: cache.Added, Object: "obj2"},
				{Type: cache.Deleted, Object: "obj2"},
				{Type: cache.Deleted, Object: "obj1"},
			},
			compressed: cache.Deltas{},
		},

		// 3.  If a cache.Sync/cache.Updated is enqueued with state X and a cache.Deleted
		//     is received with state Y, these are compressed into (Deleted, Y)
		{
			initial: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
				// test that a second object doesn't affect compression of the first
				{Type: cache.Sync, Object: "obj2"},
				{Type: cache.Updated, Object: "obj2"},
				{Type: cache.Deleted, Object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Sync, Object: "obj2"},
				{Type: cache.Deleted, Object: "obj1"},
			},
		},

		// 3.  If a cache.Sync/cache.Updated is enqueued with state X and a cache.Deleted
		//     is received with state Y, these are compressed into (Deleted, Y)
		{
			initial: cache.Deltas{
				{Type: cache.Updated, Object: "obj1"},
				{Type: cache.Deleted, Object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Deleted, Object: "obj1"},
			},
		},

		// 4.  If a cache.Updated is enqueued with state X and a cache.Updated with state Y is received,
		//     these two events are compressed into (Updated, Y)
		{
			initial: cache.Deltas{
				{Type: cache.Updated, Object: "obj1"},
				{Type: cache.Updated, Object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Updated, Object: "obj1"},
			},
		},

		// 5.  If a cache.Added is enqueued with state X and a cache.Sync with state Y is received,
		//     these are compressed into (Added, Y)
		{
			initial: cache.Deltas{
				{Type: cache.Added, Object: "obj1"},
				{Type: cache.Sync, Object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Added, Object: "obj1"},
			},
		},

		// 6.  If a cache.Sync is enqueued with state X and a cache.Sync with state Y is received,
		//     these are compressed into (Sync, Y)
		{
			initial: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
				{Type: cache.Sync, Object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
			},
		},

		// 7.  Invalid combinations (eg, Sync + Added or Updated + Added) result in a panic.
		{
			initial: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
				{Type: cache.Added, Object: "obj1"},
			},
			compressed:  cache.Deltas{},
			expectPanic: true,
		},

		// 7.  Invalid combinations (eg, Sync + Added or Updated + Added) result in a panic.
		{
			initial: cache.Deltas{
				{Type: cache.Updated, Object: "obj1"},
				{Type: cache.Added, Object: "obj1"},
			},
			compressed:  cache.Deltas{},
			expectPanic: true,
		},
	}

	for _, test := range tests {
		newDeltas, panicked, msg := deltaCompress(test.initial, testKeyFunc)
		if panicked != test.expectPanic {
			t.Fatalf("(%s) unexpected panic result %v (expected %v): %v", compressTestDesc(test), panicked, test.expectPanic, msg)
		}
		if test.expectPanic {
			continue
		}

		if len(newDeltas) != len(test.compressed) {
			t.Fatalf("(%s) wrong number of compressed deltas (got %d, expected %d): %v", compressTestDesc(test), len(newDeltas), len(test.compressed), newDeltas)
		}
		for j, expected := range test.compressed {
			have := newDeltas[j]
			if expected.Type != have.Type {
				t.Fatalf("(%s) wrong delta type (got %s, expected %s): %v", compressTestDesc(test), have.Type, expected.Type, newDeltas)
			}
			if expected.Object.(string) != have.Object.(string) {
				t.Fatalf("(%s) wrong delta object key (got %s, expected %s)", compressTestDesc(test), have.Object.(string), expected.Object.(string))
			}
		}
	}
}

func TestEventQueueDeltaCompressorDeletedFinalStateUnknown(t *testing.T) {
	deletedObj := cache.DeletedFinalStateUnknown{
		Key: "namespace1/obj1",
		Obj: &api.ObjectMeta{Name: "obj1", Namespace: "namespace1"},
	}
	initial := cache.Deltas{
		{
			Type:   cache.Deleted,
			Object: deletedObj,
		},
	}

	newDeltas, panicked, msg := deltaCompress(initial, DeletionHandlingMetaNamespaceKeyFunc)
	if panicked {
		t.Fatalf("unexpected panic: %v", msg)
	}

	if len(newDeltas) != 1 {
		t.Fatalf("wrong number of compressed deltas (got %d, expected 1): %v", len(newDeltas), newDeltas)
	}
	if newDeltas[0].Type != cache.Deleted {
		t.Fatalf("unexpected delta type %v (expected Deleted)", newDeltas[0].Type)
	}
	if !reflect.DeepEqual(newDeltas[0].Object, deletedObj) {
		t.Fatalf("unexpected delta object %v (expected %v)", newDeltas[0].Object, deletedObj)
	}
}

// Ensure the compressor panics when its given a keyFunc that can't handle the delta object
func TestEventQueueDeltaCompressorDeletedFinalStateUnknown2(t *testing.T) {
	initial := cache.Deltas{
		{
			Type: cache.Deleted,
			Object: cache.DeletedFinalStateUnknown{
				Key: "namespace1/obj1",
				Obj: &api.ObjectMeta{Name: "obj1", Namespace: "namespace1"},
			},
		},
	}

	_, panicked, _ := deltaCompress(initial, cache.MetaNamespaceKeyFunc)
	if !panicked {
		t.Fatalf("expected panic but didn't get one")
	}
}

type initialDelta struct {
	deltaType cache.DeltaType
	object    interface{}
	// knownObjects should be given for Sync DeltaTypes
	knownObjects []interface{}
}

type eventQueueTest struct {
	initial      []initialDelta
	compressed   cache.Deltas
	knownObjects []interface{}
	expectPanic  bool
}

func testDesc(test eventQueueTest) string {
	var start, result []string
	for _, delta := range test.initial {
		start = append(start, string(delta.deltaType))
	}
	for _, delta := range test.compressed {
		result = append(result, string(delta.Type))
	}
	return strings.Join(start, "+") + "=" + strings.Join(result, "+")
}

// Returns false on success, true on panic
func addInitialDeltas(queue *EventQueue, deltas []initialDelta) (panicked bool, msg string) {
	defer func() {
		if r := recover(); r != nil {
			panicked = true
			msg = fmt.Sprintf("%#v", r)
		}
	}()

	for _, initial := range deltas {
		switch initial.deltaType {
		case cache.Added:
			queue.Add(initial.object)
		case cache.Updated:
			queue.Update(initial.object)
		case cache.Deleted:
			queue.Delete(initial.object)
		case cache.Sync:
			// knownObjects should be valid for Sync operations
			queue.Replace(initial.knownObjects, "123")
		}
		if initial.object != nil {
			queue.updateKnownObjects(cache.Delta{Type: initial.deltaType, Object: initial.object})
		}
	}
	return
}

// Test the whole event queue, not just the compressor itself; this will exercise
// DeltaFIFO constructs like DeletedFinalStateUnknown, the EventQueue internal
// store, the DeltaFIFO deletion compression, and the DeltaFIFO knownObjects array
func TestEventQueueCompress(t *testing.T) {
	tests := []eventQueueTest{
		// 1.  If a cache.Added/cache.Sync is enqueued with state X and a cache.Updated with state Y
		//     is received, these are compressed into (Added/Sync, Y)
		{
			initial: []initialDelta{
				{deltaType: cache.Added, object: "obj1"},
				{deltaType: cache.Updated, object: "obj1"},
				{deltaType: cache.Updated, object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Added, Object: "obj1"},
			},
		},

		// 1.  If a cache.Added/cache.Sync is enqueued with state X and a cache.Updated with state Y
		//     is received, these are compressed into (Added/Sync, Y)
		{
			initial: []initialDelta{
				{deltaType: cache.Added, object: "obj1"},
				// test that a second object doesn't affect compression of the first
				{deltaType: cache.Added, object: "obj2"},
				{deltaType: cache.Updated, object: "obj2"},
				{deltaType: cache.Updated, object: "obj1"},
				{deltaType: cache.Updated, object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Added, Object: "obj1"},
			},
		},

		// 1.  If a cache.Added/cache.Sync is enqueued with state X and a cache.Updated with state Y
		//     is received, these are compressed into (Added/Sync, Y)
		{
			initial: []initialDelta{
				{deltaType: cache.Sync, object: "obj1", knownObjects: []interface{}{"obj1"}},
				{deltaType: cache.Updated, object: "obj1"},
				{deltaType: cache.Updated, object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
			},
		},

		// 1.  If a cache.Added/cache.Sync is enqueued with state X and a cache.Updated with state Y
		//     is received, these are compressed into (Added/Sync, Y)
		{
			initial: []initialDelta{
				{deltaType: cache.Sync, object: "obj1", knownObjects: []interface{}{"obj1"}},
				{deltaType: cache.Updated, object: "obj1"},
				// test that a second object doesn't affect compression of the first
				{deltaType: cache.Added, object: "obj2"},
				{deltaType: cache.Updated, object: "obj2"},
				{deltaType: cache.Updated, object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
			},
		},

		// 2.  If a cache.Added is enqueued with state X and a cache.Deleted is received with state Y,
		//     these are dropped and consumers will not see either event
		{
			initial: []initialDelta{
				{deltaType: cache.Added, object: "obj1"},
				// test that a second object doesn't affect compression of the first
				{deltaType: cache.Added, object: "obj2"},
				{deltaType: cache.Deleted, object: "obj2"},
				{deltaType: cache.Deleted, object: "obj1"},
			},
			compressed: cache.Deltas{},
		},

		// 3.  If a cache.Sync/cache.Updated is enqueued with state X and a cache.Deleted
		//     is received with state Y, these are compressed into (Deleted, Y)
		{
			initial: []initialDelta{
				{deltaType: cache.Sync, object: "obj1", knownObjects: []interface{}{"obj1"}},
				// test that a second object doesn't affect compression of the first
				{deltaType: cache.Sync, object: "obj2", knownObjects: []interface{}{"obj1", "obj2"}},
				{deltaType: cache.Updated, object: "obj2"},
				{deltaType: cache.Deleted, object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Deleted, Object: "obj1"},
			},
		},

		// 3.  If a cache.Sync/cache.Updated is enqueued with state X and a cache.Deleted
		//     is received with state Y, these are compressed into (Deleted, Y)
		{
			initial: []initialDelta{
				{deltaType: cache.Updated, object: "obj1"},
				{deltaType: cache.Deleted, object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Deleted, Object: "obj1"},
			},
		},

		// 4.  If a cache.Updated is enqueued with state X and a cache.Updated with state Y is received,
		//     these two events are compressed into (Updated, Y)
		{
			initial: []initialDelta{
				{deltaType: cache.Updated, object: "obj1"},
				{deltaType: cache.Updated, object: "obj1"},
			},
			compressed: cache.Deltas{
				{Type: cache.Updated, Object: "obj1"},
			},
		},

		// 5.  If a cache.Added is enqueued with state X and a cache.Sync with state Y is received,
		//     these are compressed into (Added, Y)
		{
			initial: []initialDelta{
				{deltaType: cache.Added, object: "obj1"},
				{deltaType: cache.Sync, object: "obj1", knownObjects: []interface{}{"obj1"}},
			},
			compressed: cache.Deltas{
				{Type: cache.Added, Object: "obj1"},
			},
		},

		// 6.  If a cache.Sync is enqueued with state X and a cache.Sync with state Y is received,
		//     these are compressed into (Sync, Y)
		{
			initial: []initialDelta{
				{deltaType: cache.Sync, object: "obj1", knownObjects: []interface{}{"obj1"}},
				{deltaType: cache.Sync, object: "obj1", knownObjects: []interface{}{"obj1"}},
			},
			compressed: cache.Deltas{
				{Type: cache.Sync, Object: "obj1"},
			},
		},

		// 7.  Invalid combinations (eg, Sync + Added or Updated + Added) result in a panic.
		{
			initial: []initialDelta{
				{deltaType: cache.Sync, object: "obj1", knownObjects: []interface{}{"obj1"}},
				{deltaType: cache.Added, object: "obj1"},
			},
			compressed:  cache.Deltas{},
			expectPanic: true,
		},

		// 7.  Invalid combinations (eg, Sync + Added or Updated + Added) result in a panic.
		{
			initial: []initialDelta{
				{deltaType: cache.Updated, object: "obj1"},
				{deltaType: cache.Added, object: "obj1"},
			},
			compressed:  cache.Deltas{},
			expectPanic: true,
		},

		// Ensure DeletedFinalStateUnknown objects can be compressed
		{
			initial: []initialDelta{
				{deltaType: cache.Added, object: "obj1"},
				{deltaType: cache.Sync, knownObjects: []interface{}{}},
			},
			compressed: cache.Deltas{},
		},
	}

	for _, test := range tests {
		queue := NewEventQueue(testKeyFunc)

		panicked, msg := addInitialDeltas(queue, test.initial)
		if panicked != test.expectPanic {
			t.Fatalf("(%s) unexpected panic result %v (expected %v): %v", testDesc(test), panicked, test.expectPanic, msg)
		}
		if test.expectPanic {
			continue
		}

		items, ok, err := queue.Get("obj1")
		if err != nil {
			t.Fatalf("(%s) error getting expected object: %v", testDesc(test), err)
		}
		if len(test.compressed) > 0 {
			if !ok {
				t.Fatalf("(%s) expected object doesn't exist", testDesc(test))
			}
			compressedDeltas := items.(cache.Deltas)
			if len(compressedDeltas) != len(test.compressed) {
				t.Fatalf("(%s) wrong number of compressed deltas (got %d, expected %d)", testDesc(test), len(compressedDeltas), len(test.compressed))
			}
			for j, expected := range test.compressed {
				have := compressedDeltas[j]
				if expected.Type != have.Type {
					t.Fatalf("(%s) wrong delta type (got %s, expected %s)", testDesc(test), have.Type, expected.Type)
				}
				if expected.Object.(string) != have.Object.(string) {
					t.Fatalf("(%s) wrong delta object key (got %s, expected %s)", testDesc(test), have.Object.(string), expected.Object.(string))
				}
			}
		} else if ok {
			t.Fatalf("(%s) unexpected object %v", testDesc(test), items)
		}
	}
}

// Test that single events are passed through uncompressed
func TestEventQueueUncompressed(t *testing.T) {
	obj := "obj1"

	for _, dtype := range []cache.DeltaType{cache.Added, cache.Updated, cache.Deleted, cache.Sync} {
		queue := NewEventQueue(testKeyFunc)

		// Deleted requires the object to already be in the known objects
		// list, and we must pop that cache.Added off before testing
		// to ensure the Deleted delta comes through even when the queue
		// is empty.
		if dtype == cache.Deleted {
			queue.Add(obj)
			items, err := queue.Pop(func(delta cache.Delta) error {
				return nil
			}, nil)
			if err != nil {
				t.Fatalf("(%s) unexpected error popping initial Added delta: %v", dtype, err)
			}
			deltas := items.(cache.Deltas)
			if len(deltas) != 1 {
				t.Fatalf("(%s) expected 1 delta popping initial Added, got %d", dtype, len(deltas))
			}
			if deltas[0].Type != cache.Added {
				t.Fatalf("(%s) expected initial Added delta, got %v", dtype, deltas[0].Type)
			}
		}

		// Now add the real delta type under test
		switch dtype {
		case cache.Added:
			queue.Add(obj)
		case cache.Updated:
			queue.Update(obj)
		case cache.Deleted:
			queue.Delete(obj)
		case cache.Sync:
			queue.Replace([]interface{}{obj}, "123")
		}

		// And pop the expected item out of the queue
		items, err := queue.Pop(func(delta cache.Delta) error {
			return nil
		}, nil)
		if err != nil {
			t.Fatalf("(%s) unexpected error popping delta: %v", dtype, err)
		}
		deltas := items.(cache.Deltas)
		if len(deltas) != 1 {
			t.Fatalf("(%s) expected 1 delta popping delta, got %d", dtype, len(deltas))
		}
		if deltas[0].Type != dtype {
			t.Fatalf("(%s) expected same delta, got %v", dtype, deltas[0].Type)
		}
	}
}

// Test that DeletedFinalStateUnknown objects are handled correctly
func TestEventQueueDeletedFinalStateUnknown(t *testing.T) {
	queue := NewEventQueue(DeletionHandlingMetaNamespaceKeyFunc)

	obj1 := &api.ObjectMeta{Name: "obj1", Namespace: "namespace1"}
	obj2 := &api.ObjectMeta{Name: "obj2", Namespace: "namespace1"}

	// Make sure objects are in knownObjects but not in the delta queue,
	// to ensure we get DeletedFinalStateUnknown delta objects
	queue.knownObjects.Add(obj1)
	queue.knownObjects.Add(obj2)

	// This should create two DeletedFinalStateUnknown objects
	queue.Replace([]interface{}{}, "123")

	// First test that we get actual DeletedFinalStateUnknown objects
	var called bool
	var processErr error
	if _, err := queue.Pop(func(delta cache.Delta) error {
		called = true
		if _, ok := delta.Object.(cache.DeletedFinalStateUnknown); !ok {
			// Capture error that Pop() logs the error but doesn't return
			processErr = fmt.Errorf("Unexpected item type %T", delta.Object)
			return processErr
		}
		return nil
	}, nil); err != nil {
		t.Fatalf(fmt.Sprintf("%v", err))
	}
	if !called {
		t.Fatalf("Delta pop function wasn't called")
	}
	if processErr != nil {
		t.Fatalf("Delta pop function returned error %v", processErr)
	}

	// Repeat but this time make sure we get the objects we want, not DeletedFinalStateUnknown
	queue = NewEventQueue(DeletionHandlingMetaNamespaceKeyFunc)
	queue.knownObjects.Add(obj1)
	queue.knownObjects.Add(obj2)

	// This should create two DeletedFinalStateUnknown objects
	queue.Replace([]interface{}{}, "123")

	// Now test that we only get api.ObjectMeta objects since we passed that
	// as the expected type
	called = false
	if _, err := queue.Pop(func(delta cache.Delta) error {
		called = true
		if _, ok := delta.Object.(*api.ObjectMeta); !ok {
			// Capture error that Pop() logs the error but doesn't return
			processErr = fmt.Errorf("Unexpected item type %T", delta.Object)
			return processErr
		}
		return nil
	}, &api.ObjectMeta{}); err != nil {
		t.Fatalf(fmt.Sprintf("%v", err))
	}
	if !called {
		t.Fatalf("Delta pop function wasn't called")
	}
	if processErr != nil {
		t.Fatalf("Delta pop function returned error %v", processErr)
	}
}
