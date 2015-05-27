package cache

import (
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/client"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
)

type readOnlyPolicyCache struct {
	registry  policyregistry.WatchingRegistry
	indexer   cache.Indexer
	reflector cache.Reflector

	keyFunc cache.KeyFunc
}

func NewReadOnlyPolicyCache(registry policyregistry.WatchingRegistry) readOnlyPolicyCache {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc})

	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return registry.ListPolicies(ctx, labels.Everything(), fields.Everything())
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return registry.WatchPolicies(ctx, labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&authorizationapi.Policy{},
		indexer,
		2*time.Minute,
	)

	return readOnlyPolicyCache{
		registry:  registry,
		indexer:   indexer,
		reflector: *reflector,

		keyFunc: cache.MetaNamespaceKeyFunc,
	}
}

// Run begins watching and synchronizing the cache
func (c *readOnlyPolicyCache) Run() {
	c.reflector.Run()
}

// RunUntil starts a watch and handles watch events. Will restart the watch if it is closed.
// RunUntil starts a goroutine and returns immediately. It will exit when stopCh is closed.
func (c *readOnlyPolicyCache) RunUntil(stopChannel <-chan struct{}) {
	c.reflector.RunUntil(stopChannel)
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (c *readOnlyPolicyCache) LastSyncResourceVersion() string {
	return c.reflector.LastSyncResourceVersion()
}

func (c *readOnlyPolicyCache) List(label labels.Selector, field fields.Selector, namespace string) (*authorizationapi.PolicyList, error) {
	var returnedList []interface{}
	if namespace == kapi.NamespaceAll {
		returnedList = c.indexer.List()
	} else {
		items, err := c.indexer.Index("namespace", &authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Namespace: namespace}})
		returnedList = items
		if err != nil {
			return &authorizationapi.PolicyList{}, errors.NewInvalid("PolicyList", "policyList", []error{err})
		}
	}
	policyList := &authorizationapi.PolicyList{}
	for i := range returnedList {
		policy, castOK := returnedList[i].(*authorizationapi.Policy)
		if !castOK {
			return policyList, errors.NewInvalid("PolicyList", "policyList", []error{})
		}
		if label.Matches(labels.Set(policy.Labels)) && field.Matches(PolicyToSelectableFields(policy)) {
			policyList.Items = append(policyList.Items, *policy)
		}
	}
	return policyList, nil
}

func (c *readOnlyPolicyCache) Get(name, namespace string) (*authorizationapi.Policy, error) {
	keyObj := &authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: name}}
	key, _ := c.keyFunc(keyObj)

	item, exists, getErr := c.indexer.GetByKey(key)
	if getErr != nil {
		return &authorizationapi.Policy{}, getErr
	}
	if !exists {
		existsErr := errors.NewNotFound("Policy", name)
		return &authorizationapi.Policy{}, existsErr
	}
	policy, castOK := item.(*authorizationapi.Policy)
	if !castOK {
		castErr := errors.NewInvalid("Policy", name, []error{})
		return &authorizationapi.Policy{}, castErr
	}
	return policy, nil
}

// readOnlyPolicies wraps the readOnlyPolicyCache to expose only List() and Get()
type readOnlyPolicies struct {
	readOnlyPolicyCache *readOnlyPolicyCache
	namespace           string
}

func newReadOnlyPolicies(cache readOnlyAuthorizationCache, namespace string) client.ReadOnlyPolicyInterface {
	return &readOnlyPolicies{
		readOnlyPolicyCache: &cache.readOnlyPolicyCache,
		namespace:           namespace,
	}
}

func (p *readOnlyPolicies) List(label labels.Selector, field fields.Selector) (*authorizationapi.PolicyList, error) {
	return p.readOnlyPolicyCache.List(label, field, p.namespace)
}

func (p *readOnlyPolicies) Get(name string) (*authorizationapi.Policy, error) {
	return p.readOnlyPolicyCache.Get(name, p.namespace)
}
