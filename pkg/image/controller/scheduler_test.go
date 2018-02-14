package controller

import (
	"container/heap"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/juju/ratelimit"
	flowcontrol "k8s.io/client-go/util/flowcontrol"
)

func TestScheduler(t *testing.T) {
	keys := []string{}
	s := newScheduler(2, flowcontrol.NewFakeAlwaysRateLimiter(), func(key, value interface{}) {
		keys = append(keys, key.(string))
	})

	for i := 0; i < 6; i++ {
		s.RunOnce()
		if len(keys) > 0 {
			t.Fatal(keys)
		}
		if s.position != (i+1)%3 {
			t.Fatal(s.position)
		}
	}

	s.Add("first", "test")
	found := false
	for i, buckets := range s.buckets {
		if _, ok := buckets["first"]; ok {
			found = true
		} else {
			continue
		}
		if i == s.position {
			t.Fatal("should not insert into current bucket")
		}
	}
	if !found {
		t.Fatal("expected to find key in a bucket")
	}

	s.Delay("first")

	for i := 0; i < 2; i++ { // Delay shouldn't have put the item in the current bucket
		s.RunOnce()
	}
	if len(keys) != 0 {
		t.Fatal(keys)
	}
	s.RunOnce()
	if !reflect.DeepEqual(keys, []string{"first"}) {
		t.Fatal(keys)
	}
}

func TestSchedulerAddAndDelay(t *testing.T) {
	s := newScheduler(3, flowcontrol.NewFakeAlwaysRateLimiter(), func(key, value interface{}) {})
	// 3 is the last bucket, 0 is the current bucket
	s.Add("first", "other")
	if s.buckets[3]["first"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Add("second", "other")
	if s.buckets[2]["second"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Add("third", "other")
	if s.buckets[1]["third"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Add("fourth", "other")
	if s.buckets[3]["fourth"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Add("fifth", "other")
	if s.buckets[2]["fifth"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}
	s.Remove("third", "other")
	s.Add("sixth", "other")
	if s.buckets[1]["sixth"] != "other" {
		t.Fatalf("placed key in wrong bucket: %#v", s.buckets)
	}

	// delaying an item moves it to the last bucket
	s.Delay("second")
	if s.buckets[3]["second"] != "other" {
		t.Fatalf("delay placed key in wrong bucket: %#v", s.buckets)
	}
	// delaying an item that is not in the map does nothing
	s.Delay("third")
	if _, ok := s.buckets[3]["third"]; ok {
		t.Fatalf("delay placed key in wrong bucket: %#v", s.buckets)
	}
	// delaying an item that is already in the latest bucket does nothing
	s.Delay("fourth")
	if s.buckets[3]["fourth"] != "other" {
		t.Fatalf("delay placed key in wrong bucket: %#v", s.buckets)
	}
}

func TestSchedulerRemove(t *testing.T) {
	s := newScheduler(2, flowcontrol.NewFakeAlwaysRateLimiter(), func(key, value interface{}) {})
	s.Add("test", "other")
	if s.Remove("test", "value") {
		t.Fatal(s)
	}
	if !s.Remove("test", "other") {
		t.Fatal(s)
	}
	if s.Len() != 0 {
		t.Fatal(s)
	}
	s.Add("test", "other")
	s.Add("test", "new")
	if s.Len() != 1 {
		t.Fatal(s)
	}
	if s.Remove("test", "other") {
		t.Fatal(s)
	}
	if !s.Remove("test", "new") {
		t.Fatal(s)
	}
}

type int64Heap []int64

var _ heap.Interface = &int64Heap{}

func (h int64Heap) Len() int            { return len(h) }
func (h int64Heap) Less(i, j int) bool  { return h[i] < h[j] }
func (h int64Heap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *int64Heap) Push(x interface{}) { *h = append(*h, x.(int64)) }
func (h *int64Heap) Pop() interface{} {
	x := (*h)[len(*h)-1]
	*h = (*h)[:len(*h)-1]
	return x
}

type wallClock struct{}

var _ ratelimit.Clock = &wallClock{}

func (c wallClock) Now() time.Time        { return time.Now() }
func (c wallClock) Sleep(d time.Duration) { time.Sleep(d) }

// fakeClock implements ratelimit.Clock.  Its time starts at the UNIX epoch.
// When all known threads are in Sleep(), its time advances just enough to wake
// the first thread (or threads) due to wake.
type fakeClock struct {
	c       sync.Cond
	threads int
	now     int64
	wake    int64Heap
}

var _ ratelimit.Clock = &fakeClock{}

func newFakeClock(threads int) ratelimit.Clock {
	return &fakeClock{threads: threads, c: sync.Cond{L: &sync.Mutex{}}}
}

func (c *fakeClock) Now() time.Time {
	c.c.L.Lock()
	defer c.c.L.Unlock()
	return time.Unix(0, c.now)
}

func (c *fakeClock) Sleep(d time.Duration) {
	c.c.L.Lock()
	defer c.c.L.Unlock()
	wake := c.now + int64(d)
	heap.Push(&c.wake, wake)
	if len(c.wake) == c.threads {
		// everyone is asleep, advance the clock.
		c.now = heap.Pop(&c.wake).(int64)
		for len(c.wake) > 0 && c.wake[0] == c.now {
			// pop any additional threads waiting on the same time.
			heap.Pop(&c.wake)
		}
		c.c.Broadcast()
	}
	for c.now < wake {
		c.c.Wait()
	}
}

func TestSchedulerSanity(t *testing.T) {
	const (
		buckets = 4
		items   = 10
	)

	// if needed for testing, you can revert to using the wall clock via
	// clock := &wallClock{}
	clock := newFakeClock(2) // 2 threads: us and the scheduler.

	// 1 token per second => one bucket's worth of items should get scheduled
	// per second.
	limiter := flowcontrol.NewTokenBucketRateLimiterWithClock(1, 1, clock)

	m := map[int]int{}
	s := newScheduler(buckets, limiter, func(key, value interface{}) {
		fmt.Printf("%v: %v\n", clock.Now().UTC(), key)
		m[key.(int)]++
	})
	for i := 0; i < items; i++ {
		s.Add(i, nil)
	}

	go s.RunUntil(make(chan struct{}))

	// run the clock just long enough to expect to have scheduled each item
	// exactly twice.
	clock.Sleep((2*buckets-1)*time.Second + 1)

	expected := map[int]int{}
	for i := 0; i < items; i++ {
		expected[i] = 2
	}

	if !reflect.DeepEqual(m, expected) {
		t.Errorf("m did not match expected: %#v\n", m)
	}
}
