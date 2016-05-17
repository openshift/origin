package cache

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/golang/glog"
)

type SecurityCache struct {
	client clientset.Interface
	Store  cache.Store
}

// NewSecurityCache builds and initializes a SecurityCache object
func NewSecurityCache(client clientset.Interface) *SecurityCache {
	return &SecurityCache{client: client}
}

// Run builds the sthe store that backs this cache and run the backing reflector
func (c *SecurityCache) Run() {
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return c.client.Core().SecurityContextConstraints().List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return c.client.Core().SecurityContextConstraints().Watch(options)
			},
		},
		&kapi.SecurityContextConstraints{},
		store,
		0,
	)
	reflector.Run()
	c.Store = store
	glog.Infof("securityCache  is running...")
}

// Running determines if the case is initialized and running
func (c *SecurityCache) Running() bool {
	return c.Store != nil
}

// List returns SecurityContextConstraints list
func (c *SecurityCache) List() (constraints []*kapi.SecurityContextConstraints, err error) {
	err = nil
	for _, c := range c.Store.List() {
		constraint, ok := c.(*kapi.SecurityContextConstraints)
		if !ok {
			return nil, errors.NewInternalError(fmt.Errorf("error converting object from store to a security context constraint: %v", c))
		}
		constraints = append(constraints, constraint)
	}
	return constraints, err
}
