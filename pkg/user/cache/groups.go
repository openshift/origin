package cache

import (
	"fmt"
	"time"

	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/origin/pkg/controller"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	groupregistry "github.com/openshift/origin/pkg/user/registry/group"
)

type GroupCache struct {
	indexer   cache.Indexer
	reflector *cache.Reflector
}

const byUserIndexName = "ByUser"

// ByUserIndexKeys is cache.IndexFunc for Groups that will index groups by User, so that a direct cache lookup
// using a User.Name will return all Groups that User is a member of
func ByUserIndexKeys(obj interface{}) ([]string, error) {
	group, ok := obj.(*userapi.Group)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %v", obj)
	}

	return group.Users, nil
}

func NewGroupCache(groupRegistry groupregistry.Registry) *GroupCache {
	allNamespaceContext := apirequest.WithNamespace(apirequest.NewContext(), metav1.NamespaceAll)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{byUserIndexName: ByUserIndexKeys})
	reflector := cache.NewReflector(
		&controller.InternalListWatch{
			ListFunc: func(options metainternal.ListOptions) (runtime.Object, error) {
				return groupRegistry.ListGroups(allNamespaceContext, &options)
			},
			WatchFunc: func(options metainternal.ListOptions) (watch.Interface, error) {
				return groupRegistry.WatchGroups(allNamespaceContext, &options)
			},
		},
		&userapi.Group{},
		indexer,
		// TODO this was chosen via copy/paste.  If or when we choose to standardize these in some way, be sure to update this.
		2*time.Minute,
	)

	return &GroupCache{
		indexer:   indexer,
		reflector: reflector,
	}
}

// Run begins watching and synchronizing the cache
func (c *GroupCache) Run() {
	c.reflector.Run()
}

// RunUntil starts a watch and handles watch events. Will restart the watch if it is closed.
// RunUntil starts a goroutine and returns immediately. It will exit when stopCh is closed.
func (c *GroupCache) RunUntil(stopChannel <-chan struct{}) {
	c.reflector.RunUntil(stopChannel)
}

// Running determines if the cache is initialized and running.
func (c *GroupCache) Running() bool {
	return c.indexer != nil
}

// LastSyncResourceVersioner exposes the LastSyncResourceVersion of the internal
// reflector.
func (c *GroupCache) LastSyncResourceVersion() string {
	return c.reflector.LastSyncResourceVersion()
}

func (c *GroupCache) GroupsFor(username string) ([]*userapi.Group, error) {
	objs, err := c.indexer.ByIndex(byUserIndexName, username)
	if err != nil {
		return nil, err
	}

	groups := make([]*userapi.Group, len(objs))
	for i := range objs {
		groups[i] = objs[i].(*userapi.Group)
	}

	return groups, nil
}
