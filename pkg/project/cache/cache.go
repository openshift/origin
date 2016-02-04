package cache

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	projectapi "github.com/openshift/origin/pkg/project/api"
	"github.com/openshift/origin/pkg/util/labelselector"
)

// NewProjectCache returns a non-initialized ProjectCache. The cache needs to be run to begin functioning
func NewProjectCache(client client.NamespaceInterface, defaultNodeSelector string) *ProjectCache {
	return &ProjectCache{
		Client:              client,
		DefaultNodeSelector: defaultNodeSelector,
	}
}

type ProjectCache struct {
	Client              client.NamespaceInterface
	Store               cache.Indexer
	DefaultNodeSelector string
}

func (p *ProjectCache) GetNamespace(name string) (*kapi.Namespace, error) {
	// check for namespace in the cache
	namespaceObj, exists, err := p.Store.Get(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name:      name,
			Namespace: "",
		},
		Status: kapi.NamespaceStatus{},
	})
	if err != nil {
		return nil, err
	}

	var namespace *kapi.Namespace
	if exists {
		namespace = namespaceObj.(*kapi.Namespace)
	} else {
		// Our watch maybe latent, so we make a best effort to get the object, and only fail if not found
		namespace, err = p.Client.Get(name)
		// the namespace does not exist, so prevent create and update in that namespace
		if err != nil {
			return nil, fmt.Errorf("namespace %s does not exist", name)
		}
	}
	return namespace, nil
}

func (p *ProjectCache) GetNodeSelector(namespace *kapi.Namespace) string {
	selector := ""
	found := false
	if len(namespace.ObjectMeta.Annotations) > 0 {
		if ns, ok := namespace.ObjectMeta.Annotations[projectapi.ProjectNodeSelector]; ok {
			selector = ns
			found = true
		}
	}
	if !found {
		selector = p.DefaultNodeSelector
	}
	return selector
}

func (p *ProjectCache) GetNodeSelectorMap(namespace *kapi.Namespace) (map[string]string, error) {
	selector := p.GetNodeSelector(namespace)
	labelsMap, err := labelselector.Parse(selector)
	if err != nil {
		return map[string]string{}, err
	}
	return labelsMap, nil
}

// Run builds the store that backs this cache and runs the backing reflector
func (c *ProjectCache) Run() {
	store := NewCacheStore(cache.MetaNamespaceKeyFunc)
	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return c.Client.List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return c.Client.Watch(options)
			},
		},
		&kapi.Namespace{},
		store,
		0,
	)
	reflector.Run()
	c.Store = store
}

// Running determines if the cache is initialized and running
func (c *ProjectCache) Running() bool {
	return c.Store != nil
}

// NewFake is used for testing purpose only
func NewFake(c client.NamespaceInterface, store cache.Indexer, defaultNodeSelector string) *ProjectCache {
	return &ProjectCache{
		Client:              c,
		Store:               store,
		DefaultNodeSelector: defaultNodeSelector,
	}
}

// NewCacheStore creates an Indexer store with the given key function
func NewCacheStore(keyFn cache.KeyFunc) cache.Indexer {
	return cache.NewIndexer(keyFn, cache.Indexers{
		"requester": indexNamespaceByRequester,
	})
}

// indexNamespaceByRequester returns the requester for a given namespace object as an index value
func indexNamespaceByRequester(obj interface{}) ([]string, error) {
	requester := obj.(*kapi.Namespace).Annotations[projectapi.ProjectRequester]
	return []string{requester}, nil
}
