package server

import (
	"fmt"
	"time"

	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
)

// projectObjectListStore represents a cache of objects indexed by a project name.
// Used to store a list of items per namespace.
type projectObjectListStore interface {
	add(namespace string, obj runtime.Object) error
	get(namespace string) (obj runtime.Object, exists bool, err error)
}

// projectObjectListCache implements projectObjectListStore.
type projectObjectListCache struct {
	store cache.Store
}

var _ projectObjectListStore = &projectObjectListCache{}

// newProjectObjectListCache creates a cache to hold object list objects that will expire with the given ttl.
func newProjectObjectListCache(ttl time.Duration) projectObjectListStore {
	return &projectObjectListCache{
		store: cache.NewTTLStore(metaProjectObjectListKeyFunc, ttl),
	}
}

// add stores given list object under the given namespace. Any prior object under this
// key will be replaced.
func (c *projectObjectListCache) add(namespace string, obj runtime.Object) error {
	if namespace == "" {
		return fmt.Errorf("namespace cannot be empty")
	}
	no := &namespacedObject{
		namespace: namespace,
		object:    obj,
	}
	return c.store.Add(no)
}

// get retrieves a cached list object if present and not expired.
func (c *projectObjectListCache) get(namespace string) (runtime.Object, bool, error) {
	entry, exists, err := c.store.GetByKey(namespace)
	if err != nil {
		return nil, exists, err
	}
	if !exists {
		return nil, false, err
	}
	no, ok := entry.(*namespacedObject)
	if !ok {
		return nil, false, fmt.Errorf("%T is not a namespaced object", entry)
	}
	return no.object, true, nil
}

// namespacedObject is a container associating an object with a namespace.
type namespacedObject struct {
	namespace string
	object    runtime.Object
}

// metaProjectObjectListKeyFunc returns a key for given namespaced object. The key is object's namespace.
func metaProjectObjectListKeyFunc(obj interface{}) (string, error) {
	if key, ok := obj.(cache.ExplicitKey); ok {
		return string(key), nil
	}
	no, ok := obj.(*namespacedObject)
	if !ok {
		return "", fmt.Errorf("object %T is not a namespaced object", obj)
	}
	return no.namespace, nil
}
