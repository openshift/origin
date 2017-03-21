package factory

import (
	"testing"

	oscache "github.com/openshift/origin/pkg/client/cache"
	"k8s.io/kubernetes/pkg/watch"
)

type cacheable struct {
	key   string
	value interface{}
}

func keyFunc(obj interface{}) (string, error) {
	return obj.(cacheable).key, nil
}

func createNewInstance() *EventQueueTrapper {
	q := oscache.NewEventQueue(keyFunc)
	return NewEventQueueTrapper(q, nil)
}

func EventQueueTrapper_basic(t *testing.T) {
	q := createNewInstance()

	const amount = 500
	go func() {
		for i := 0; i < amount; i++ {
			q.Add(cacheable{string([]rune{'a', rune(i)}), i + 1})
		}
	}()
	go func() {
		for u := uint(0); u < amount; u++ {
			q.Add(cacheable{string([]rune{'b', rune(u)}), u + 1})
		}
	}()

	lastInt := int(0)
	lastUint := uint(0)
	for i := 0; i < amount*2; i++ {
		_, obj, _ := q.Pop()
		value := obj.(cacheable).value
		switch v := value.(type) {
		case int:
			if v <= lastInt {
				t.Errorf("got %v (int) out of order, last was %v", v, lastInt)
			}
			lastInt = v
		case uint:
			if v <= lastUint {
				t.Errorf("got %v (uint) out of order, last was %v", v, lastUint)
			} else {
				lastUint = v
			}
		default:
			t.Fatalf("unexpected type %#v", obj)
		}
	}
}

func EventQueueTrapper_initialEventIsDelete(t *testing.T) {
	q := createNewInstance()

	q.Replace([]interface{}{
		cacheable{"foo", 2},
	}, "1")

	q.Delete(cacheable{key: "foo"})

	event, thing, _ := q.Pop()

	value := thing.(cacheable).value
	if value != 2 {
		t.Fatalf("expected %v, got %v", 2, thing)
	}

	if event != watch.Deleted {
		t.Fatalf("expected %s, got %s", watch.Added, event)
	}
}

func EventQueueTrapper_compressAddDelete(t *testing.T) {
	q := createNewInstance()

	q.Add(cacheable{"foo", 10})
	q.Delete(cacheable{key: "foo"})
	q.Add(cacheable{"zab", 30})

	event, thing, _ := q.Pop()

	value := thing.(cacheable).value
	if value != 30 {
		t.Fatalf("expected %v, got %v", 30, value)
	}

	if event != watch.Added {
		t.Fatalf("expected %s, got %s", watch.Added, event)
	}
}

func EventQueueTrapper_compressAddUpdate(t *testing.T) {
	q := createNewInstance()

	q.Add(cacheable{"foo", 10})
	q.Update(cacheable{"foo", 11})

	event, thing, _ := q.Pop()
	value := thing.(cacheable).value
	if value != 11 {
		t.Fatalf("expected %v, got %v", 11, value)
	}

	if event != watch.Added {
		t.Fatalf("expected %s, got %s", watch.Added, event)
	}
}

func EventQueueTrapper_compressTwoUpdates(t *testing.T) {
	q := createNewInstance()

	q.Replace([]interface{}{
		cacheable{"foo", 2},
	}, "1")

	q.Update(cacheable{"foo", 3})
	q.Update(cacheable{"foo", 4})

	event, thing, _ := q.Pop()
	value := thing.(cacheable).value
	if value != 4 {
		t.Fatalf("expected %v, got %v", 4, value)
	}

	if event != watch.Modified {
		t.Fatalf("expected %s, got %s", watch.Modified, event)
	}
}

func EventQueueTrapper_compressUpdateDelete(t *testing.T) {
	q := createNewInstance()

	q.Replace([]interface{}{
		cacheable{"foo", 2},
	}, "1")

	q.Update(cacheable{"foo", 3})
	q.Delete(cacheable{key: "foo"})

	event, thing, _ := q.Pop()
	value := thing.(cacheable).value
	if value != 3 {
		t.Fatalf("expected %v, got %v", 3, value)
	}

	if event != watch.Deleted {
		t.Fatalf("expected %s, got %s", watch.Deleted, event)
	}
}

func EventQueueTrapper_modifyEventsFromReplace(t *testing.T) {
	q := createNewInstance()

	q.Replace([]interface{}{
		cacheable{"foo", 2},
	}, "1")

	q.Update(cacheable{"foo", 2})

	event, thing, _ := q.Pop()
	value := thing.(cacheable).value
	if value != 2 {
		t.Fatalf("expected %v, got %v", 2, value)
	}

	if event != watch.Modified {
		t.Fatalf("expected %s, got %s", watch.Modified, event)
	}
}

func EventQueueTrapper_ListConsumed(t *testing.T) {
	q := createNewInstance()
	if !q.ListConsumed() {
		t.Fatalf("expected ListConsumed to be true after queue creation")
	}

	q.Replace([]interface{}{}, "1")
	if !q.ListConsumed() {
		t.Fatalf("expected ListConsumed to be true after Replace() without items")
	}

	items := []interface{}{
		cacheable{"foo", 2},
	}
	q.Replace(items, "1")
	if q.ListConsumed() {
		t.Fatalf("expected ListConsumed to be false after Replace() with items")
	}

	// Delete() only results in the removal of a queued item if it is
	// of event type watch.Add.  Since items added by Replace() are of
	// type watch.Modified, calling Delete() on those items will
	// change the event type but not remove them from the queue.
	q.Delete(items[0])
	if q.ListConsumed() {
		t.Fatalf("expected ListConsumed to be false after Delete()")
	}

	q.Pop()
	if !q.ListConsumed() {
		t.Fatalf("expected ListConsumed to be true after queued items read")
	}
}

func EventQueueTrapper_trapErrors(t *testing.T) {
	occurred := false
	trapper := func() {
		occurred = true
	}

	unsafeQueue := oscache.NewEventQueue(keyFunc)
	q := NewEventQueueTrapper(unsafeQueue, trapper)

	var err error

	if err = q.Add(cacheable{"trap1", 1.0}); err != nil {
		t.Fatalf("initial queue add failed: %s", err)
	}

	if err = q.Delete(cacheable{key: "trap1"}); err != nil {
		t.Fatalf("queue delete failed: %s", err)
	}

	if err = q.Update(cacheable{"trap1", 1.1}); err != nil {
		t.Fatalf("queue update failed: %s", err)
	}
	if err = q.Update(cacheable{"trap1", 1.2}); err != nil {
		t.Fatalf("queue update #2 failed: %s", err)
	}

	if err = q.Delete(cacheable{key: "trap1"}); err != nil {
		t.Fatalf("queue delete #2 failed: %s", err)
	}

	if err = q.Add(cacheable{"trap1", 2.0}); err != nil {
		t.Fatalf("queue add #2 failed: %s", err)
	}

	if err = q.Delete(cacheable{key: "trap1"}); err != nil {
		t.Fatalf("queue delete #3 failed: %s", err)
	}

	if !occurred {
		t.Fatalf("expected the store trapper to catch an error")
	}
}
