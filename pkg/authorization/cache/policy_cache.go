package cache

import (
	"fmt"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterbindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	bindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

// PolicyCache maintains a cache of PolicyRules
type PolicyCache struct {
	policyBindingIndexer        cache.Indexer
	policyIndexer               cache.Indexer
	clusterPolicyBindingIndexer cache.Indexer
	clusterPolicyIndexer        cache.Indexer

	bindingRegistry        bindingregistry.WatchingRegistry
	policyRegistry         policyregistry.WatchingRegistry
	clusterBindingRegistry clusterbindingregistry.WatchingRegistry
	clusterPolicyRegistry  clusterpolicyregistry.WatchingRegistry

	keyFunc cache.KeyFunc
}

// TODO: Eliminate listWatch when this merges upstream: https://github.com/GoogleCloudPlatform/kubernetes/pull/4453
type listFunc func() (runtime.Object, error)
type watchFunc func(resourceVersion string) (watch.Interface, error)
type listWatch struct {
	listFunc  listFunc
	watchFunc watchFunc
}

func (lw *listWatch) List() (runtime.Object, error) {
	return lw.listFunc()
}

func (lw *listWatch) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.watchFunc(resourceVersion)
}

// NewPolicyCache creates a new PolicyCache.  You cannot use a normal client, because you don't want policy guarding the policy from the authorizer
func NewPolicyCache(bindingRegistry bindingregistry.WatchingRegistry, policyRegistry policyregistry.WatchingRegistry, clusterBindingRegistry clusterbindingregistry.WatchingRegistry, clusterPolicyRegistry clusterpolicyregistry.WatchingRegistry) *PolicyCache {
	result := &PolicyCache{
		policyIndexer:               cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc}),
		policyBindingIndexer:        cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc}),
		clusterPolicyIndexer:        cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc}),
		clusterPolicyBindingIndexer: cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc}),

		keyFunc: cache.MetaNamespaceKeyFunc,

		bindingRegistry:        bindingRegistry,
		policyRegistry:         policyRegistry,
		clusterBindingRegistry: clusterBindingRegistry,
		clusterPolicyRegistry:  clusterPolicyRegistry,
	}
	return result
}

// Run begins watching and synchronizing the cache
func (c *PolicyCache) Run() {
	policyBindingReflector, policyReflector, clusterPolicyBindingReflector, clusterPolicyReflector := c.configureReflectors()

	policyBindingReflector.Run()
	policyReflector.Run()
	clusterPolicyBindingReflector.Run()
	clusterPolicyReflector.Run()
}

// RunUntil starts a watch and handles watch events. Will restart the watch if it is closed.
// RunUntil starts a goroutine and returns immediately. It will exit when stopCh is closed.
func (c *PolicyCache) RunUntil(bindingStopCh <-chan struct{}, policyStopCh <-chan struct{}) {
	policyBindingReflector, policyReflector, clusterPolicyBindingReflector, clusterPolicyReflector := c.configureReflectors()

	policyBindingReflector.RunUntil(bindingStopCh)
	policyReflector.RunUntil(policyStopCh)
	clusterPolicyBindingReflector.RunUntil(bindingStopCh)
	clusterPolicyReflector.RunUntil(policyStopCh)
}

func (c *PolicyCache) configureReflectors() (*cache.Reflector, *cache.Reflector, *cache.Reflector, *cache.Reflector) {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	policyBindingReflector := cache.NewReflector(
		&listWatch{
			listFunc: func() (runtime.Object, error) {
				return c.bindingRegistry.ListPolicyBindings(ctx, labels.Everything(), fields.Everything())
			},
			watchFunc: func(resourceVersion string) (watch.Interface, error) {
				return c.bindingRegistry.WatchPolicyBindings(ctx, labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&authorizationapi.PolicyBinding{},
		c.policyBindingIndexer,
		2*time.Minute,
	)

	policyReflector := cache.NewReflector(
		&listWatch{
			listFunc: func() (runtime.Object, error) {
				return c.policyRegistry.ListPolicies(ctx, labels.Everything(), fields.Everything())
			},
			watchFunc: func(resourceVersion string) (watch.Interface, error) {
				return c.policyRegistry.WatchPolicies(ctx, labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&authorizationapi.Policy{},
		c.policyIndexer,
		2*time.Minute,
	)

	clusterPolicyBindingReflector := cache.NewReflector(
		&listWatch{
			listFunc: func() (runtime.Object, error) {
				return c.clusterBindingRegistry.ListClusterPolicyBindings(ctx, labels.Everything(), fields.Everything())
			},
			watchFunc: func(resourceVersion string) (watch.Interface, error) {
				return c.clusterBindingRegistry.WatchClusterPolicyBindings(ctx, labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&authorizationapi.ClusterPolicyBinding{},
		c.clusterPolicyBindingIndexer,
		2*time.Minute,
	)

	clusterPolicyReflector := cache.NewReflector(
		&listWatch{
			listFunc: func() (runtime.Object, error) {
				return c.clusterPolicyRegistry.ListClusterPolicies(ctx, labels.Everything(), fields.Everything())
			},
			watchFunc: func(resourceVersion string) (watch.Interface, error) {
				return c.clusterPolicyRegistry.WatchClusterPolicies(ctx, labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&authorizationapi.ClusterPolicy{},
		c.clusterPolicyIndexer,
		2*time.Minute,
	)

	return policyBindingReflector, policyReflector, clusterPolicyBindingReflector, clusterPolicyReflector
}

// GetPolicy retrieves a specific policy.  It conforms to rulevalidation.PolicyGetter.
func (c *PolicyCache) GetPolicy(ctx kapi.Context, name string) (*authorizationapi.Policy, error) {
	namespace, _ := kapi.NamespaceFrom(ctx)

	switch {
	case len(namespace) == 0:
		keyObj := &authorizationapi.ClusterPolicy{ObjectMeta: kapi.ObjectMeta{Name: name}}
		key, _ := c.keyFunc(keyObj)

		policy, exists, err := c.clusterPolicyIndexer.GetByKey(key)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("%v not found", key)
		}

		return authorizationapi.ToPolicy(policy.(*authorizationapi.ClusterPolicy)), nil

	default:
		keyObj := &authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: name}}
		key, _ := c.keyFunc(keyObj)

		policy, exists, err := c.policyIndexer.GetByKey(key)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("%v not found", key)
		}

		return policy.(*authorizationapi.Policy), nil
	}
}

// ListPolicyBindings obtains list of policyBindings that match a selector.  It conforms to rulevalidation.BindingLister
func (c *PolicyCache) ListPolicyBindings(ctx kapi.Context, label labels.Selector, field fields.Selector) (*authorizationapi.PolicyBindingList, error) {
	namespace, _ := kapi.NamespaceFrom(ctx)

	switch {
	case len(namespace) == 0:
		bindings, err := c.clusterPolicyBindingIndexer.Index("namespace", &authorizationapi.ClusterPolicyBinding{})
		if err != nil {
			return nil, err
		}

		ret := &authorizationapi.PolicyBindingList{
			Items: make([]authorizationapi.PolicyBinding, 0, len(bindings)),
		}
		for i := range bindings {
			ret.Items = append(ret.Items, *authorizationapi.ToPolicyBinding(bindings[i].(*authorizationapi.ClusterPolicyBinding)))
		}

		return ret, nil

	default:
		bindings, err := c.policyBindingIndexer.Index("namespace", &authorizationapi.PolicyBinding{ObjectMeta: kapi.ObjectMeta{Namespace: namespace}})
		if err != nil {
			return nil, err
		}

		ret := &authorizationapi.PolicyBindingList{
			Items: make([]authorizationapi.PolicyBinding, 0, len(bindings)),
		}
		for i := range bindings {
			ret.Items = append(ret.Items, *bindings[i].(*authorizationapi.PolicyBinding))
		}

		return ret, nil
	}
}
