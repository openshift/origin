package cache

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/util/labelselector"
)

type ProjectCache struct {
	Client              client.Interface
	Store               cache.Store
	DefaultNodeSelector string
}

var pcache *ProjectCache

func (p *ProjectCache) GetNamespaceObject(name string) (*kapi.Namespace, error) {
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
		namespace, err = p.Client.Namespaces().Get(name)
		// the namespace does not exist, so prevent create and update in that namespace
		if err != nil {
			return nil, fmt.Errorf("namespace %s does not exist", name)
		}
	}
	return namespace, nil
}

func (p *ProjectCache) GetNodeSelector(namespace *kapi.Namespace) string {
	selector := ""
	if len(namespace.ObjectMeta.Annotations) > 0 {
		if ns, ok := namespace.ObjectMeta.Annotations["openshift.io/node-selector"]; ok {
			selector = ns
		}
	}
	if len(selector) == 0 {
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

func GetProjectCache() (*ProjectCache, error) {
	if pcache == nil {
		return nil, fmt.Errorf("project cache not initialized")
	}
	return pcache, nil
}

func RunProjectCache(c client.Interface, defaultNodeSelector string) {
	if pcache != nil {
		return
	}

	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	reflector := cache.NewReflector(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return c.Namespaces().List(labels.Everything(), fields.Everything())
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return c.Namespaces().Watch(labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&kapi.Namespace{},
		store,
		0,
	)
	reflector.Run()
	pcache = &ProjectCache{
		Client:              c,
		Store:               store,
		DefaultNodeSelector: defaultNodeSelector,
	}
}

// Used for testing purpose only
func FakeProjectCache(c client.Interface, store cache.Store, defaultNodeSelector string) {
	pcache = &ProjectCache{
		Client:              c,
		Store:               store,
		DefaultNodeSelector: defaultNodeSelector,
	}
}
