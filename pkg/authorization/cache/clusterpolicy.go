package cache

import (
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	errors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/client"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
)

type readOnlyClusterPolicyCache struct {
	registry  clusterpolicyregistry.WatchingRegistry
	indexer   cache.Indexer
	reflector cache.Reflector

	keyFunc cache.KeyFunc
}

func NewReadOnlyClusterPolicyCache(registry clusterpolicyregistry.WatchingRegistry) readOnlyClusterPolicyCache {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc})

	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return registry.ListClusterPolicies(ctx, labels.Everything(), fields.Everything())
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return registry.WatchClusterPolicies(ctx, labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&authorizationapi.ClusterPolicy{},
		indexer,
		2*time.Minute,
	)

	return readOnlyClusterPolicyCache{
		registry:  registry,
		indexer:   indexer,
		reflector: *reflector,

		keyFunc: cache.MetaNamespaceKeyFunc,
	}
}

// Run begins watching and synchronizing the cache
func (c *readOnlyClusterPolicyCache) Run() {
	c.reflector.Run()
}

// RunUntil starts a watch and handles watch events. Will restart the watch if it is closed.
// RunUntil starts a goroutine and returns immediately. It will exit when stopCh is closed.
func (c *readOnlyClusterPolicyCache) RunUntil(stopChannel <-chan struct{}) {
	c.reflector.RunUntil(stopChannel)
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (c *readOnlyClusterPolicyCache) LastSyncResourceVersion() string {
	return c.reflector.LastSyncResourceVersion()
}

func (c *readOnlyClusterPolicyCache) List(label labels.Selector, field fields.Selector) (*authorizationapi.ClusterPolicyList, error) {
	clusterPolicyList := &authorizationapi.ClusterPolicyList{}
	returnedList := c.indexer.List()
	for i := range returnedList {
		clusterPolicy, castOK := returnedList[i].(*authorizationapi.ClusterPolicy)
		if !castOK {
			return clusterPolicyList, errors.NewInvalid("ClusterPolicy", "clusterPolicy", []error{})
		}
		if label.Matches(labels.Set(clusterPolicy.Labels)) && field.Matches(ClusterPolicyToSelectableFields(clusterPolicy)) {
			clusterPolicyList.Items = append(clusterPolicyList.Items, *clusterPolicy)
		}
	}
	return clusterPolicyList, nil
}

func (c *readOnlyClusterPolicyCache) Get(name string) (*authorizationapi.ClusterPolicy, error) {
	keyObj := &authorizationapi.ClusterPolicy{ObjectMeta: kapi.ObjectMeta{Name: name}}
	key, _ := c.keyFunc(keyObj)

	item, exists, getErr := c.indexer.GetByKey(key)
	if getErr != nil {
		return &authorizationapi.ClusterPolicy{}, getErr
	}
	if !exists {
		existsErr := errors.NewNotFound("ClusterPolicy", name)
		return &authorizationapi.ClusterPolicy{}, existsErr
	}
	clusterPolicy, castOK := item.(*authorizationapi.ClusterPolicy)
	if !castOK {
		castErr := errors.NewInvalid("ClusterPolicy", name, []error{})
		return &authorizationapi.ClusterPolicy{}, castErr
	}
	return clusterPolicy, nil
}

func newReadOnlyClusterPolicies(cache readOnlyAuthorizationCache) client.ReadOnlyClusterPolicyInterface {
	return &cache.readOnlyClusterPolicyCache
}
