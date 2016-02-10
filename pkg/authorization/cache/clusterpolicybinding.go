package cache

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	errors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
	kfield "k8s.io/kubernetes/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/client"
	clusterbindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
)

type readOnlyClusterPolicyBindingCache struct {
	registry  clusterbindingregistry.WatchingRegistry
	indexer   cache.Indexer
	reflector *cache.Reflector

	keyFunc cache.KeyFunc
}

func NewReadOnlyClusterPolicyBindingCache(registry clusterbindingregistry.WatchingRegistry) *readOnlyClusterPolicyBindingCache {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc})

	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return registry.ListClusterPolicyBindings(ctx, &options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return registry.WatchClusterPolicyBindings(ctx, &options)
			},
		},
		&authorizationapi.ClusterPolicyBinding{},
		indexer,
		2*time.Minute,
	)

	return &readOnlyClusterPolicyBindingCache{
		registry:  registry,
		indexer:   indexer,
		reflector: reflector,

		keyFunc: cache.MetaNamespaceKeyFunc,
	}
}

// Run begins watching and synchronizing the cache
func (c *readOnlyClusterPolicyBindingCache) Run() {
	c.reflector.Run()
}

// RunUntil starts a watch and handles watch events. Will restart the watch if it is closed.
// RunUntil starts a goroutine and returns immediately. It will exit when stopCh is closed.
func (c *readOnlyClusterPolicyBindingCache) RunUntil(stopChannel <-chan struct{}) {
	c.reflector.RunUntil(stopChannel)
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (c *readOnlyClusterPolicyBindingCache) LastSyncResourceVersion() string {
	return c.reflector.LastSyncResourceVersion()
}

func (c *readOnlyClusterPolicyBindingCache) List(options *kapi.ListOptions) (*authorizationapi.ClusterPolicyBindingList, error) {
	clusterPolicyBindingList := &authorizationapi.ClusterPolicyBindingList{}
	returnedList := c.indexer.List()
	matcher := clusterbindingregistry.Matcher(api.ListOptionsToSelectors(options))
	for i := range returnedList {
		clusterPolicyBinding, castOK := returnedList[i].(*authorizationapi.ClusterPolicyBinding)
		if !castOK {
			return clusterPolicyBindingList, errors.NewInvalid(authorizationapi.Kind("ClusterPolicyBinding"), "clusterPolicyBinding", kfield.ErrorList{})
		}
		if matches, err := matcher.Matches(clusterPolicyBinding); err == nil && matches {
			clusterPolicyBindingList.Items = append(clusterPolicyBindingList.Items, *clusterPolicyBinding)
		}
	}
	return clusterPolicyBindingList, nil
}

func (c *readOnlyClusterPolicyBindingCache) Get(name string) (*authorizationapi.ClusterPolicyBinding, error) {
	keyObj := &authorizationapi.ClusterPolicyBinding{ObjectMeta: kapi.ObjectMeta{Name: name}}
	key, _ := c.keyFunc(keyObj)

	item, exists, getErr := c.indexer.GetByKey(key)
	if getErr != nil {
		return &authorizationapi.ClusterPolicyBinding{}, getErr
	}
	if !exists {
		existsErr := errors.NewNotFound(authorizationapi.Resource("clusterpolicybinding"), name)
		return &authorizationapi.ClusterPolicyBinding{}, existsErr
	}
	clusterPolicyBinding, castOK := item.(*authorizationapi.ClusterPolicyBinding)
	if !castOK {
		castErr := errors.NewInvalid(authorizationapi.Kind("ClusterPolicyBinding"), name, kfield.ErrorList{})
		return &authorizationapi.ClusterPolicyBinding{}, castErr
	}
	return clusterPolicyBinding, nil
}

func newReadOnlyClusterPolicyBindings(cache readOnlyAuthorizationCache) client.ReadOnlyClusterPolicyBindingInterface {
	return cache.readOnlyClusterPolicyBindingCache
}
