package controller

import (
	"sync"

	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/flowcontrol"
)

// NOTE: scheduler's semantics do not lend it for reuse elsewhere and its use in
// this package quite probably has some odd corner cases/race conditions.  If
// these cause problems in the future, this implementation should be replaced
// with a new and simpler one based on container/heap.  End users looking for a
// component like this: see if k8s.io/client-go/util/workqueue.NewDelayingQueue
// suits your needs.

// scheduler is a self-balancing, rate-limited, bucketed queue that can periodically invoke
// an action on all items in a bucket before moving to the next bucket. A ratelimiter sets
// an upper bound on the number of buckets processed per unit time. The queue has a key and a
// value, so both uniqueness and equality can be tested (key must be unique, value can carry
// info for the next processing). Items remain in the queue until removed by a call to Remove().
type scheduler struct {
	handle   func(key, value interface{})
	position int
	limiter  flowcontrol.RateLimiter

	mu      sync.Mutex
	buckets []bucket
}

type bucket map[interface{}]interface{}

// newScheduler creates a scheduler with bucketCount buckets, a rate limiter for restricting
// the rate at which buckets are processed, and a function to invoke when items are scanned in
// a bucket.
// TODO: remove DEBUG statements from this file once this logic has been adequately validated.
func newScheduler(bucketCount int, bucketLimiter flowcontrol.RateLimiter, fn func(key, value interface{})) *scheduler {
	// Add one more bucket to serve as the "current" bucket
	bucketCount++
	buckets := make([]bucket, bucketCount)
	for i := range buckets {
		buckets[i] = make(bucket)
	}
	return &scheduler{
		handle:  fn,
		buckets: buckets,
		limiter: bucketLimiter,
	}
}

// RunUntil launches the scheduler until ch is closed.
func (s *scheduler) RunUntil(ch <-chan struct{}) {
	go utilwait.Until(s.RunOnce, 0, ch)
}

// RunOnce takes a single item out of the current bucket and processes it. If
// the bucket is empty, we wait for the rate limiter before returning.
func (s *scheduler) RunOnce() {
	key, value, last := s.next()
	if last {
		s.limiter.Accept()
		return
	}
	s.handle(key, value)
}

// at returns the bucket index relative to the current bucket.
func (s *scheduler) at(inc int) int {
	return (s.position + inc + len(s.buckets)) % len(s.buckets)
}

// next takes a key from the current bucket and places it in the last bucket, returns the
// removed key. Returns true if the current bucket is empty and no key and value were returned.
func (s *scheduler) next() (interface{}, interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	last := s.buckets[s.position]
	// Grab the first item in the bucket, move it to the end and return it.
	for k, v := range last {
		delete(last, k)
		s.buckets[s.at(-1)][k] = v
		return k, v, false
	}
	// The bucket was empty.  Advance to the next bucket.
	s.position = s.at(1)
	return nil, nil, true
}

// Add places the key in the bucket with the least entries (except the current bucket). The key is used to
// determine uniqueness, while value can be used to associate additional data for later retrieval. An Add
// removes the previous key and value and will place the item in a new bucket. This allows callers to ensure
// that Add'ing a new item to the queue purges old versions of the item, while Remove can be conditional on
// removing only the known old version.
func (s *scheduler) Add(key, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, bucket := range s.buckets {
		delete(bucket, key)
	}

	// Pick the bucket with the least entries that is furthest from the current position
	n := len(s.buckets)
	base := s.position + n
	target, least := 0, 0
	for i := n - 1; i > 0; i-- {
		position := (base + i) % n
		size := len(s.buckets[position])
		if size == 0 {
			target = position
			break
		}
		if size < least || least == 0 {
			target = position
			least = size
		}
	}
	s.buckets[target][key] = value
}

// Remove takes the key out of all buckets. If value is non-nil, the key will only be removed if it has
// the same value. Returns true if the key was removed.
func (s *scheduler) Remove(key, value interface{}) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	match := true
	for _, bucket := range s.buckets {
		if value != nil {
			if old, ok := bucket[key]; ok && old != value {
				match = false
				continue
			}
		}
		delete(bucket, key)
	}
	return match
}

// Delay moves the key to the end of the chain if it exists.
func (s *scheduler) Delay(key interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	last := s.at(-1)
	for i, bucket := range s.buckets {
		if i == last {
			continue
		}
		if value, ok := bucket[key]; ok {
			delete(bucket, key)
			s.buckets[last][key] = value
		}
	}
}

// Len returns the number of scheduled items.
func (s *scheduler) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, bucket := range s.buckets {
		count += len(bucket)
	}
	return count
}

// Map returns a copy of the scheduler contents, but does not copy the keys or values themselves.
// If values and keys are not immutable, changing the value will affect the value in the queue.
func (s *scheduler) Map() map[interface{}]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make(map[interface{}]interface{})
	for _, bucket := range s.buckets {
		for k, v := range bucket {
			out[k] = v
		}
	}
	return out
}
