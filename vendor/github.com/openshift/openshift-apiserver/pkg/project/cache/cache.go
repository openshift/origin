package cache

import (
	"fmt"
	"time"

	"k8s.io/klog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"

	projectapi "github.com/openshift/openshift-apiserver/pkg/project/apis/project"
)

// NewProjectCache returns a non-initialized ProjectCache. The cache needs to be run to begin functioning
func NewProjectCache(namespaces cache.SharedIndexInformer, client corev1client.NamespaceInterface, defaultNodeSelector string) *ProjectCache {
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
	Client              corev1client.NamespaceInterface
	Store               cache.Indexer
	HasSynced           cache.InformerSynced
	DefaultNodeSelector string
}

func (p *ProjectCache) GetNamespace(name string) (*corev1.Namespace, error) {
	key := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}

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
			klog.V(4).Infof("found %s in cache after waiting", name)
		}
	}

	var namespace *corev1.Namespace
	if exists {
		namespace = namespaceObj.(*corev1.Namespace)
	} else {
		// Our watch maybe latent, so we make a best effort to get the object, and only fail if not found
		namespace, err = p.Client.Get(name, metav1.GetOptions{})
		// the namespace does not exist, so prevent create and update in that namespace
		if err != nil {
			return nil, fmt.Errorf("namespace %s does not exist", name)
		}
		klog.V(4).Infof("found %s via storage lookup", name)
	}
	return namespace, nil
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
func NewFake(c corev1client.NamespaceInterface, store cache.Indexer, defaultNodeSelector string) *ProjectCache {
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
	requester := obj.(*corev1.Namespace).Annotations[projectapi.ProjectRequester]
	return []string{requester}, nil
}
