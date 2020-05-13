package ttl_cache

import (
	"container/list"
	"time"

	"k8s.io/apimachinery/pkg/util/clock"
)

type item struct {
	obj       interface{}
	timestamp time.Time
	key string
}

type evictionStore struct {
	store            map[string]*list.Element
	queue            *list.List
	ttl              time.Duration
	lastEvictionTime time.Time
	clock            clock.Clock
}

func New(ttl time.Duration, clock clock.Clock) *evictionStore {
	return &evictionStore{
		store:   map[string]*list.Element{},
		queue:   list.New(),
		ttl:     ttl,
		clock:   clock,
	}
}

func (s *evictionStore) Add(key string, obj interface{}) {
	ts := s.clock.Now()
	defer s.evict(ts)

	if e, ok := s.store[key]; ok {
		e.Value.(*item).timestamp = ts
		s.queue.MoveToFront(e)
		return
	}
	s.store[key] = s.queue.PushFront(&item{obj: obj, timestamp: ts, key: key})
}

func (s *evictionStore) Get(key string) interface{} {
	ts := s.clock.Now()
	defer s.evict(ts)

	if e, ok := s.store[key]; ok {
		e.Value.(*item).timestamp = ts
		s.queue.MoveToFront(e)
		return e.Value.(*item).obj
	}

	return nil
}

func (s *evictionStore) List() []interface{} {
	ret := []interface{}{}
	for key, _ := range s.store {
		if obj := s.Get(key); obj != nil {
			ret = append(ret, obj)
		}
	}

	return ret
}

func (s *evictionStore) evict(timestamp time.Time) {
	if s.lastEvictionTime.Add(s.ttl).After(timestamp) {
		return
	}
	for {
		if s.queue.Len() == 0 {
			break
		}
		e := s.queue.Back()
		if e.Value.(*item).timestamp.Add(s.ttl).After(timestamp) {
			break
		}
		delete(s.store, e.Value.(*item).key)
		s.queue.Remove(e)
	}
	s.lastEvictionTime = timestamp
}
