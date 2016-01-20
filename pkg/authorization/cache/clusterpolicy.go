package cache

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/client"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
)

type readOnlyClusterPolicyCache struct {
	registry  clusterpolicyregistry.WatchingRegistry
	indexer   cache.Indexer
	reflector *cache.Reflector

	keyFunc cache.KeyFunc
}

func NewReadOnlyClusterPolicyCache(registry clusterpolicyregistry.WatchingRegistry) *readOnlyClusterPolicyCache {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc})

	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				opts := &unversioned.ListOptions{
					LabelSelector:   unversioned.LabelSelector{Selector: options.LabelSelector},
					FieldSelector:   unversioned.FieldSelector{Selector: options.FieldSelector},
					ResourceVersion: options.ResourceVersion,
				}
				return registry.ListClusterPolicies(ctx, opts)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				opts := &unversioned.ListOptions{
					LabelSelector:   unversioned.LabelSelector{Selector: options.LabelSelector},
					FieldSelector:   unversioned.FieldSelector{Selector: options.FieldSelector},
					ResourceVersion: options.ResourceVersion,
				}
				return registry.WatchClusterPolicies(ctx, opts)
			},
		},
		&authorizationapi.ClusterPolicy{},
		indexer,
		2*time.Minute,
	)

	return &readOnlyClusterPolicyCache{
		registry:  registry,
		indexer:   indexer,
		reflector: reflector,

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

func (c *readOnlyClusterPolicyCache) List(options *unversioned.ListOptions) (*authorizationapi.ClusterPolicyList, error) {
	clusterPolicyList := &authorizationapi.ClusterPolicyList{}
	returnedList := c.indexer.List()
	matcher := clusterpolicyregistry.Matcher(api.ListOptionsToSelectors(options))
	for i := range returnedList {
		clusterPolicy, castOK := returnedList[i].(*authorizationapi.ClusterPolicy)
		if !castOK {
			return clusterPolicyList, errors.NewInvalid("ClusterPolicy", "clusterPolicy", []error{})
		}
		if matches, err := matcher.Matches(clusterPolicy); err == nil && matches {
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
	return cache.readOnlyClusterPolicyCache
}
