package cache

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	"github.com/openshift/origin/pkg/util/labelselector"
)

// NewProjectCache returns a non-initialized ProjectCache. The cache needs to be run to begin functioning
func NewProjectCache(namespaces cache.SharedIndexInformer, client kcoreclient.NamespaceInterface, defaultNodeSelector string) *ProjectCache {
	if err := namespaces.GetIndexer().AddIndexers(cache.Indexers{
		"requester": indexNamespaceByRequester,
	}); err != nil {
		panic(err)
	}
	return &ProjectCache{
		Client:              client,
		Store:               namespaces.GetIndexer(),
		HasSynced:           namespaces.GetController().HasSynced,
		DefaultNodeSelector: defaultNodeSelector,
	}
}

type ProjectCache struct {
	Client              kcoreclient.NamespaceInterface
	Store               cache.Indexer
	HasSynced           cache.InformerSynced
	DefaultNodeSelector string
}

func (p *ProjectCache) GetNamespace(name string) (*kapi.Namespace, error) {
	key := &kapi.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}

	// check for namespace in the cache
	namespaceObj, exists, err := p.Store.Get(key)
	if err != nil {
		return nil, err
	}

	if !exists {
		// give the cache time to observe a recent namespace creation
		time.Sleep(50 * time.Millisecond)
		namespaceObj, exists, err = p.Store.Get(key)
		if err != nil {
			return nil, err
		}
		if exists {
			glog.V(4).Infof("found %s in cache after waiting", name)
		}
	}

	var namespace *kapi.Namespace
	if exists {
		namespace = namespaceObj.(*kapi.Namespace)
	} else {
		// Our watch maybe latent, so we make a best effort to get the object, and only fail if not found
		namespace, err = p.Client.Get(name, metav1.GetOptions{})
		// the namespace does not exist, so prevent create and update in that namespace
		if err != nil {
			return nil, fmt.Errorf("namespace %s does not exist", name)
		}
		glog.V(4).Infof("found %s via storage lookup", name)
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

// Run waits until the cache has synced.
func (c *ProjectCache) Run(stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	if !cache.WaitForCacheSync(stopCh, c.HasSynced) {
		return
	}
	<-stopCh
}

// Running determines if the cache is initialized and running
func (c *ProjectCache) Running() bool {
	return c.Store != nil
}

// NewFake is used for testing purpose only
func NewFake(c kcoreclient.NamespaceInterface, store cache.Indexer, defaultNodeSelector string) *ProjectCache {
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
