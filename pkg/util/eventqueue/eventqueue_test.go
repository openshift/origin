package eventqueue

import (
	"testing"

	"k8s.io/kubernetes/pkg/watch"
)

type cacheable struct {
	key   string
	value interface{}
}

func keyFunc(obj interface{}) (string, error) {
	return obj.(cacheable).key, nil
}

func TestEventQueue_basic(t *testing.T) {
	q := NewEventQueue(keyFunc)

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

func TestEventQueue_multipleEvents(t *testing.T) {
	q := NewEventQueue(keyFunc)

	testCases := []struct {
		Op  watch.EventType
		Obj cacheable
	}{
		{watch.Added, cacheable{"foo", 10}},
		{watch.Deleted, cacheable{key: "foo"}},
		{watch.Added, cacheable{"foo", 20}},
		{watch.Modified, cacheable{"foo", 21}},
		{watch.Modified, cacheable{"foo", 22}},
		{watch.Deleted, cacheable{key: "foo"}},
		{watch.Modified, cacheable{"foo", 23}},
		{watch.Added, cacheable{"foo", 30}},
		{watch.Modified, cacheable{"foo", 31}},
		{watch.Modified, cacheable{"foo", 32}},
		{watch.Modified, cacheable{"foo", 33}},
		{watch.Modified, cacheable{"foo", 34}},
	}

	for _, tc := range testCases {
		switch tc.Op {
		case watch.Added:
			q.Add(tc.Obj)
		case watch.Modified:
			q.Update(tc.Obj)
		case watch.Deleted:
			q.Delete(tc.Obj)
		default:
			t.Errorf("Invalid operation %v", tc.Op)
		}
	}

	for _, tc := range testCases {
		event, thing, _ := q.Pop()
		if event != tc.Op {
			t.Errorf("expected event %v, got %v", tc.Op, event)
		}

		if event != watch.Deleted {
			value := thing.(cacheable).value
			if value != tc.Obj.value {
				t.Fatalf("expected value %v, got %v", tc.Obj.value, value)
			}
		}
	}
}

func TestEventQueue_List(t *testing.T) {
	iq := NewEventQueue(keyFunc)
	if len(iq.List()) != 0 {
		t.Fatalf("expected List size to be 0 at queue creation")
	}

	single := []interface{}{cacheable{"single", 1}}
	items := []interface{}{
		cacheable{"c1", 1},
		cacheable{"c2", 2},
		cacheable{"c3", 3},
		cacheable{"c4", 4},
		cacheable{"c5", 5},
	}

	testCases := []struct {
		Name     string
		Objects  []interface{}
		Version  string
		Expected int
	}{
		{"without items", []interface{}{}, "1", 0},
		{"with single item", single, "2", 1},
		{"with multiple items", items, "3", len(items)},
	}

	for _, tc := range testCases {
		q := NewEventQueue(keyFunc)
		q.Replace(tc.Objects, tc.Version)
		list := q.List()
		if len(list) != tc.Expected {
			t.Fatalf("expected List size to be %v after Replace() %s, got %v", tc.Expected, tc.Name, len(list))
		}
	}
}

func TestEventQueue_ListKeys(t *testing.T) {
	iq := NewEventQueue(keyFunc)
	if len(iq.ListKeys()) != 0 {
		t.Fatalf("expected ListKeys to be 0 at queue creation")
	}

	single := []interface{}{cacheable{"single", 1}}
	items := []interface{}{
		cacheable{"c1", 1},
		cacheable{"c2", 2},
		cacheable{"c3", 3},
		cacheable{"c4", 4},
		cacheable{"c5", 5},
	}

	testCases := []struct {
		Name     string
		Objects  []interface{}
		Version  string
		Expected int
	}{
		{"without items", []interface{}{}, "1", 0},
		{"with single item", single, "2", 1},
		{"with multiple items", items, "3", len(items)},
	}

	for _, tc := range testCases {
		q := NewEventQueue(keyFunc)
		q.Replace(tc.Objects, tc.Version)
		keys := q.ListKeys()
		if len(keys) != tc.Expected {
			t.Fatalf("expected ListKeys to be %v after Replace() %s, got %v", tc.Expected, tc.Name, len(keys))
		}
	}
}

func TestEventQueue_ListCount(t *testing.T) {
	iq := NewEventQueue(keyFunc)
	if iq.ListCount() != 0 {
		t.Fatalf("expected ListCount to be 0 at queue creation")
	}

	single := []interface{}{cacheable{"single", 1}}
	items := []interface{}{
		cacheable{"c1", 1},
		cacheable{"c2", 2},
		cacheable{"c3", 3},
		cacheable{"c4", 4},
		cacheable{"c5", 5},
	}

	testCases := []struct {
		Name     string
		Objects  []interface{}
		Version  string
		Expected int
	}{
		{"without items", []interface{}{}, "1", 0},
		{"with single item", single, "2", 1},
		{"with multiple items", items, "3", len(items)},
	}

	for _, tc := range testCases {
		q := NewEventQueue(keyFunc)
		q.Replace(tc.Objects, tc.Version)
		itemCount := q.ListCount()
		if itemCount != tc.Expected {
			t.Fatalf("expected ListCount to be %v after Replace() %s, got %v", tc.Expected, tc.Name, itemCount)
		}
	}
}

func TestEventQueue_ListSuccessfulAtLeastOnce(t *testing.T) {
	iq := NewEventQueue(keyFunc)
	if iq.ListSuccessfulAtLeastOnce() {
		t.Fatalf("expected ListSuccessfulAtLeastOnce to be false at queue creation")
	}

	single := []interface{}{cacheable{"single", 1}}
	items := []interface{}{
		cacheable{"c1", 1},
		cacheable{"c2", 2},
		cacheable{"c3", 3},
		cacheable{"c4", 4},
		cacheable{"c5", 5},
	}

	testCases := []struct {
		Name     string
		Objects  []interface{}
		Version  string
		Expected bool
	}{
		{"without items", []interface{}{}, "1", true},
		{"with single item", single, "2", true},
		{"with multiple items", items, "3", true},
	}

	for _, tc := range testCases {
		q := NewEventQueue(keyFunc)
		q.Replace(tc.Objects, tc.Version)
		flag := q.ListSuccessfulAtLeastOnce()
		if flag != tc.Expected {
			t.Fatalf("expected ListCount to be %v after Replace() %s, got %v", tc.Expected, tc.Name, flag)
		}
	}
}

func TestEventQueue_ListConsumed(t *testing.T) {
	iq := NewEventQueue(keyFunc)
	if !iq.ListConsumed() {
		t.Fatalf("expected ListConsumed to be true after queue creation")
	}

	single := []interface{}{cacheable{"single", 1}}
	items := []interface{}{
		cacheable{"c1", 1},
		cacheable{"c2", 2},
		cacheable{"c3", 3},
		cacheable{"c4", 4},
		cacheable{"c5", 5},
	}

	testCases := []struct {
		Name     string
		Objects  []interface{}
		Version  string
		Expected bool
	}{
		{"without items", []interface{}{}, "1", true},
		{"with single item", single, "2", false},
		{"with multiple items", items, "3", false},
	}

	for _, tc := range testCases {
		q := NewEventQueue(keyFunc)
		q.Replace(tc.Objects, tc.Version)
		flag := q.ListConsumed()
		if flag != tc.Expected {
			t.Fatalf("expected ListConsumed to be %v after Replace() %s, got %v", tc.Expected, tc.Name, flag)
		}
	}

	pq := NewEventQueue(keyFunc)
	pq.Replace(single, "5")
	pq.Pop()
	if !pq.ListConsumed() {
		t.Fatalf("expected ListConsumed to be true after queued items read")
	}
}

func TestEventQueue_Resync(t *testing.T) {
	q := NewEventQueue(keyFunc)
	if len(q.ListKeys()) != 0 {
		t.Fatalf("expected count to be 0 at queue creation")
	}

	items := []interface{}{
		cacheable{"c1", 1},
		cacheable{"c2", 2},
		cacheable{"c3", 3},
		cacheable{"c4", 4},
		cacheable{"c5", 5},
	}

	expectation := 0
	for i := range items {
		q.Add(items[i])
		expectation++
	}

	qcount := len(q.ListKeys())
	if qcount != expectation {
		t.Fatalf("expected count to be %d after adding items, got %d", expectation, qcount)
	}

	q.Resync()
	expectation += len(items)
	qcount = len(q.ListKeys())
	if qcount != expectation {
		t.Fatalf("expected count to be %d after Resync(), got %d", expectation, qcount)
	}

	q.Add(items[0])
	expectation++
	q.Add(items[0])
	expectation++
	qcount = len(q.ListKeys())
	if qcount != expectation {
		t.Fatalf("expected count to be %d after adding same event twice, got %d", expectation, qcount)
	}

	q.Replace([]interface{}{}, "2")
	expectation = 0

	q.Resync()
	expectation += len(items)
	qcount = len(q.ListKeys())
	if qcount != expectation {
		t.Fatalf("expected count to be %d after second Resync(), got %d", expectation, qcount)
	}
}
