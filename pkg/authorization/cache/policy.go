package cache

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/runtime"
	kfield "k8s.io/kubernetes/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/watch"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/client"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
)

type readOnlyPolicyCache struct {
	registry  policyregistry.WatchingRegistry
	indexer   cache.Indexer
	reflector *cache.Reflector

	keyFunc cache.KeyFunc
}

func NewReadOnlyPolicyCache(registry policyregistry.WatchingRegistry) *readOnlyPolicyCache {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc})

	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return registry.ListPolicies(ctx, &options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return registry.WatchPolicies(ctx, &options)
			},
		},
		&authorizationapi.Policy{},
		indexer,
		2*time.Minute,
	)

	return &readOnlyPolicyCache{
		registry:  registry,
		indexer:   indexer,
		reflector: reflector,

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

func (c *readOnlyPolicyCache) List(options *kapi.ListOptions, namespace string) (*authorizationapi.PolicyList, error) {
	var returnedList []interface{}
	if namespace == kapi.NamespaceAll {
		returnedList = c.indexer.List()
	} else {
		items, err := c.indexer.Index("namespace", &authorizationapi.Policy{ObjectMeta: kapi.ObjectMeta{Namespace: namespace}})
		returnedList = items
		if err != nil {
			return &authorizationapi.PolicyList{}, errors.NewInvalid(authorizationapi.Kind("PolicyList"), "policyList", kfield.ErrorList{kfield.Invalid(kfield.NewPath("policyList"), nil, err.Error())})
		}
	}
	policyList := &authorizationapi.PolicyList{}
	matcher := policyregistry.Matcher(oapi.ListOptionsToSelectors(options))
	for i := range returnedList {
		policy, castOK := returnedList[i].(*authorizationapi.Policy)
		if !castOK {
			return policyList, errors.NewInvalid(authorizationapi.Kind("PolicyList"), "policyList", kfield.ErrorList{})
		}
		if matches, err := matcher.Matches(policy); err == nil && matches {
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
		existsErr := errors.NewNotFound(authorizationapi.Resource("policy"), name)
		return &authorizationapi.Policy{}, existsErr
	}
	policy, castOK := item.(*authorizationapi.Policy)
	if !castOK {
		castErr := errors.NewInvalid(authorizationapi.Kind("Policy"), name, kfield.ErrorList{})
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
		readOnlyPolicyCache: cache.readOnlyPolicyCache,
		namespace:           namespace,
	}
}

func (p *readOnlyPolicies) List(options *kapi.ListOptions) (*authorizationapi.PolicyList, error) {
	return p.readOnlyPolicyCache.List(options, p.namespace)
}

func (p *readOnlyPolicies) Get(name string) (*authorizationapi.Policy, error) {
	return p.readOnlyPolicyCache.Get(name, p.namespace)
}
