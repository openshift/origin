package controller

import (
	"sync"

	"github.com/golang/glog"

	kutil "k8s.io/kubernetes/pkg/util"
)

type Scheduler struct {
	handle   func(key, value interface{})
	position int
	limiter  kutil.RateLimiter

	mu      sync.Mutex
	buckets []bucket
}

type bucket map[interface{}]interface{}

func NewScheduler(bucketCount int, bucketLimiter kutil.RateLimiter, fn func(key, value interface{})) *Scheduler {
	// add one more bucket to serve as the "current" bucket
	bucketCount++
	buckets := make([]bucket, bucketCount)
	for i := range buckets {
		buckets[i] = make(bucket)
	}
	return &Scheduler{
		handle:  fn,
		buckets: buckets,
		limiter: bucketLimiter,
	}
}

func (s *Scheduler) RunUntil(ch <-chan struct{}) {
	go kutil.Until(s.RunOnce, 0, ch)
}

func (s *Scheduler) RunOnce() {
	key, value, last := s.next()
	if last {
		glog.V(5).Infof("DEBUG: scheduler: waiting for limit")
		s.limiter.Accept()
		return
	}
	glog.V(5).Infof("DEBUG: scheduler: handle %s", key)
	s.handle(key, value)
}

func (s *Scheduler) at(inc int) int {
	return (s.position + inc + len(s.buckets)) % len(s.buckets)
}

// next takes a key from the current bucket and places it in the last bucket
func (s *Scheduler) next() (interface{}, interface{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	glog.V(5).Infof("DEBUG: scheduler: queue (%d):\n %#v", s.position, s.buckets)

	last := s.buckets[s.position]
	if len(last) == 0 {
		s.position = s.at(1)
		glog.V(5).Infof("DEBUG: scheduler: position: %d %d", s.position, len(s.buckets))
		last = s.buckets[s.position]
	}

	for k, v := range last {
		delete(last, k)
		s.buckets[s.at(-1)][k] = v
		return k, v, false
	}
	return nil, nil, true
}

// Add places the key in the bucket with the least entries (except the current bucket). The key is used to
// determine uniqueness, while value can be used to associate additional data for later retrieval. An Add
// removes the previous key and value and will place the item in a new bucket.
func (s *Scheduler) Add(key, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, bucket := range s.buckets {
		delete(bucket, key)
	}

	// pick the bucket with the least entries that is furthest from the current position
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
func (s *Scheduler) Remove(key, value interface{}) bool {
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
func (s *Scheduler) Delay(key interface{}) {
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
func (s *Scheduler) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, bucket := range s.buckets {
		count += len(bucket)
	}
	return count
}

// Map returns a copy of the scheduler contents for testing purposes.
func (s *Scheduler) Map() map[interface{}]interface{} {
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
