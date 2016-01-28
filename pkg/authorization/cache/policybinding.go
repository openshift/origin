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
	bindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

type readOnlyPolicyBindingCache struct {
	registry  bindingregistry.WatchingRegistry
	indexer   cache.Indexer
	reflector *cache.Reflector

	keyFunc cache.KeyFunc
}

func NewReadOnlyPolicyBindingCache(registry bindingregistry.WatchingRegistry) *readOnlyPolicyBindingCache {
	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)

	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc})

	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return registry.ListPolicyBindings(ctx, &options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return registry.WatchPolicyBindings(ctx, &options)
			},
		},
		&authorizationapi.PolicyBinding{},
		indexer,
		2*time.Minute,
	)

	return &readOnlyPolicyBindingCache{
		registry:  registry,
		indexer:   indexer,
		reflector: reflector,

		keyFunc: cache.MetaNamespaceKeyFunc,
	}
}

// Run begins watching and synchronizing the cache
func (c *readOnlyPolicyBindingCache) Run() {
	c.reflector.Run()
}

// RunUntil starts a watch and handles watch events. Will restart the watch if it is closed.
// RunUntil starts a goroutine and returns immediately. It will exit when stopCh is closed.
func (c *readOnlyPolicyBindingCache) RunUntil(stopChannel <-chan struct{}) {
	c.reflector.RunUntil(stopChannel)
}

// LastSyncResourceVersion exposes the LastSyncResourceVersion of the internal reflector
func (c *readOnlyPolicyBindingCache) LastSyncResourceVersion() string {
	return c.reflector.LastSyncResourceVersion()
}

func (c *readOnlyPolicyBindingCache) List(options *kapi.ListOptions, namespace string) (*authorizationapi.PolicyBindingList, error) {
	var returnedList []interface{}
	if namespace == kapi.NamespaceAll {
		returnedList = c.indexer.List()
	} else {
		items, err := c.indexer.Index("namespace", &authorizationapi.PolicyBinding{ObjectMeta: kapi.ObjectMeta{Namespace: namespace}})
		returnedList = items
		if err != nil {
			return &authorizationapi.PolicyBindingList{}, errors.NewInvalid(authorizationapi.Kind("PolicyBindingList"), "policyBindingList", kfield.ErrorList{kfield.Invalid(kfield.NewPath("policyBindingList"), nil, err.Error())})
		}
	}
	policyBindingList := &authorizationapi.PolicyBindingList{}
	matcher := bindingregistry.Matcher(api.ListOptionsToSelectors(options))
	for i := range returnedList {
		policyBinding, castOK := returnedList[i].(*authorizationapi.PolicyBinding)
		if !castOK {
			return policyBindingList, errors.NewInvalid(authorizationapi.Kind("PolicyBindingList"), "policyBindingList", kfield.ErrorList{})
		}
		if matches, err := matcher.Matches(policyBinding); err == nil && matches {
			policyBindingList.Items = append(policyBindingList.Items, *policyBinding)
		}
	}
	return policyBindingList, nil
}

func (c *readOnlyPolicyBindingCache) Get(name, namespace string) (*authorizationapi.PolicyBinding, error) {
	keyObj := &authorizationapi.PolicyBinding{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: name}}
	key, _ := c.keyFunc(keyObj)

	item, exists, getErr := c.indexer.GetByKey(key)
	if getErr != nil {
		return &authorizationapi.PolicyBinding{}, getErr
	}
	if !exists {
		existsErr := errors.NewNotFound(authorizationapi.Resource("policybinding"), name)
		return &authorizationapi.PolicyBinding{}, existsErr
	}
	policyBinding, castOK := item.(*authorizationapi.PolicyBinding)
	if !castOK {
		castErr := errors.NewInvalid(authorizationapi.Kind("PolicyBinding"), name, kfield.ErrorList{})
		return &authorizationapi.PolicyBinding{}, castErr
	}
	return policyBinding, nil
}

// readOnlyPolicyBindings wraps readOnlyPolicyBindingCache in order to expose only List() and Get()
type readOnlyPolicyBindings struct {
	readOnlyPolicyBindingCache *readOnlyPolicyBindingCache
	namespace                  string
}

func newReadOnlyPolicyBindings(cache readOnlyAuthorizationCache, namespace string) client.ReadOnlyPolicyBindingInterface {
	return &readOnlyPolicyBindings{
		readOnlyPolicyBindingCache: cache.readOnlyPolicyBindingCache,
		namespace:                  namespace,
	}
}

func (p *readOnlyPolicyBindings) List(options *kapi.ListOptions) (*authorizationapi.PolicyBindingList, error) {
	return p.readOnlyPolicyBindingCache.List(options, p.namespace)
}

func (p *readOnlyPolicyBindings) Get(name string) (*authorizationapi.PolicyBinding, error) {
	return p.readOnlyPolicyBindingCache.Get(name, p.namespace)
}
