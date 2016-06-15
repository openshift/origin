package controllers

import (
	lru "github.com/hashicorp/golang-lru"

	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage/etcd"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
)

// MutationCache is able to take the result of update operations and stores them in an LRU
// that can be used to provide a more current view of a requested object.  It requires interpretting
// resourceVersions for comparisons.
// Implementations must be thread-safe.
type MutationCache interface {
	GetByKey(key string) (interface{}, bool, error)
	Mutation(interface{})
}

type ResourceVersionComparator interface {
	CompareResourceVersion(lhs, rhs runtime.Object) int
}

// NewEtcdMutationCache gives back a MutationCache that understands how to deal with etcd backed objects
func NewEtcdMutationCache(backingCache cache.Store) MutationCache {
	lru, err := lru.New(100)
	if err != nil {
		// errors only happen on invalid sizes, this would be programmer error
		panic(err)
	}

	return &mutationCache{
		backingCache:  backingCache,
		mutationCache: lru,
		comparator:    etcd.APIObjectVersioner{},
	}
}

// mutationCache doesn't guarantee that it returns values added via Mutation since they can page out and
// since you can't distinguish between, "didn't observe create" and "was deleted after create",
// if the key is missing from the backing cache, we always return it as missing
type mutationCache struct {
	backingCache  cache.Store
	mutationCache *lru.Cache

	comparator ResourceVersionComparator
}

// GetByKey is never guaranteed to return back the value set in Mutation.  It could be paged out, it could
// be older than another copy, the backingCache may be more recent or, you might have written twice into the same key.
// You get a value that was valid at some snapshot of time and will always return the newer of backingCache and mutationCache.
func (c *mutationCache) GetByKey(key string) (interface{}, bool, error) {
	obj, exists, err := c.backingCache.GetByKey(key)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		// we can't distinguish between, "didn't observe create" and "was deleted after create", so
		// if the key is missing, we always return it as missing
		return nil, false, nil
	}
	objRuntime, ok := obj.(runtime.Object)
	if !ok {
		return obj, true, nil
	}

	mutatedObj, exists := c.mutationCache.Get(key)
	if !exists {
		return obj, true, nil
	}
	mutatedObjRuntime, ok := mutatedObj.(runtime.Object)
	if !ok {
		return obj, true, nil
	}

	if c.comparator.CompareResourceVersion(objRuntime, mutatedObjRuntime) >= 0 {
		c.mutationCache.Remove(key)
		return obj, true, nil
	}

	return mutatedObj, true, nil
}

// Mutation adds a change to the cache that can be returned in GetByKey if it is newer than the backingCache
// copy.  If you call Mutation twice with the same object on different threads, one will win, but its not defined
// which one.  This doesn't affect correctness, since the GetByKey guaranteed of "later of these two caches" is
// preserved, but you may not get the version of the object you want.  The object you get is only guaranteed to
// "one that was valid at some point in time", not "the one that I want".
func (c *mutationCache) Mutation(obj interface{}) {
	key, err := controller.KeyFunc(obj)
	if err != nil {
		// this is a "nice to have", so failures shouldn't do anything weird
		utilruntime.HandleError(err)
		return
	}

	c.mutationCache.Add(key, obj)
}
