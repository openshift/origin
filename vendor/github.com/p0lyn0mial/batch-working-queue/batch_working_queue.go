package batch_working_queue

import (
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

type Queue struct {
	lock  sync.Mutex

	store      map[string][]interface{}
	dirty      map[string][]interface{}
	inProgress sets.String

	q []string
}

func New() *Queue {
	return &Queue{
		store:      map[string][]interface{}{},
		dirty:      map[string][]interface{}{},
		inProgress: sets.NewString(),
	}
}

func (q *Queue) Add(key string, item interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if q.inProgress.Has(key) {
		q.dirty[key] = append(q.dirty[key], item)
		return
	}
	if len(q.store[key]) == 0 {
		q.q = append(q.q, key)
	}
	q.store[key] = append(q.store[key], item)
}

func (q *Queue) Get() (key string, items []interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()
	if len(q.q) == 0 {
		// TODO: block until we have something in Queue ?
		return "", nil
	}
	workKey := q.q[0]
	q.q = q.q[1:]
	work := q.store[workKey]
	q.store[workKey] = []interface{}{}
	q.inProgress.Insert(workKey)

	return workKey, work
}

func (q *Queue) Done(key string) {
	q.lock.Lock()
	defer q.lock.Unlock()

	if !q.inProgress.Has(key) {
		return
	}
	if len(q.dirty[key]) == 0 {
		q.inProgress.Delete(key)
		return
	}

	q.store[key] = q.dirty[key]
	delete(q.dirty, key)
	q.q = append(q.q, key)
	q.inProgress.Delete(key)
}

