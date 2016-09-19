package plugin

import (
	"fmt"
	"strings"
	"testing"

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

type initialDelta struct {
	deltaType cache.DeltaType
	object    interface{}
	// knownObjects should be given for Sync DeltaTypes
	knownObjects []interface{}
}

type eventQueueTest struct {
	initial      []initialDelta
	compressed   []cache.Delta
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
func addInitialDeltas(queue *EventQueue, deltas []initialDelta) (paniced bool, msg string) {
	defer func() {
		if r := recover(); r != nil {
			paniced = true
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
	}
	return
}

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
			compressed: []cache.Delta{
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
			compressed: []cache.Delta{
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
			compressed: []cache.Delta{
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
			compressed: []cache.Delta{
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
			compressed: []cache.Delta{},
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
			compressed: []cache.Delta{
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
			compressed: []cache.Delta{
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
			compressed: []cache.Delta{
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
			compressed: []cache.Delta{
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
			compressed: []cache.Delta{
				{Type: cache.Sync, Object: "obj1"},
			},
		},

		// 7.  Invalid combinations (eg, Sync + Added or Updated + Added) result in a panic.
		{
			initial: []initialDelta{
				{deltaType: cache.Sync, object: "obj1", knownObjects: []interface{}{"obj1"}},
				{deltaType: cache.Added, object: "obj1"},
			},
			compressed:  []cache.Delta{},
			expectPanic: true,
		},

		// 7.  Invalid combinations (eg, Sync + Added or Updated + Added) result in a panic.
		{
			initial: []initialDelta{
				{deltaType: cache.Updated, object: "obj1"},
				{deltaType: cache.Added, object: "obj1"},
			},
			compressed:  []cache.Delta{},
			expectPanic: true,
		},
	}

	for _, test := range tests {
		queue := NewEventQueue(testKeyFunc)

		paniced, msg := addInitialDeltas(queue, test.initial)
		if paniced != test.expectPanic {
			t.Fatalf("(%s) unexpected panic result %v (expected %v): %v", testDesc(test), paniced, test.expectPanic, msg)
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
			t.Fatalf("(%s) unexpected object", testDesc(test))
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
			})
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
		})
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
