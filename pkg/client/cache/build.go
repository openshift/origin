package cache

import (
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	kcache "k8s.io/client-go/tools/cache"
	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// StoreToBuildLister gives a store a List method. The store must contain only builds.
type StoreToBuildLister struct {
	Indexer kcache.Indexer
}

// List all builds in the store.
func (s *StoreToBuildLister) List() ([]*buildapi.Build, error) {
	builds := []*buildapi.Build{}
	for _, c := range s.Indexer.List() {
		builds = append(builds, c.(*buildapi.Build))
	}
	return builds, nil
}

func (s *StoreToBuildLister) Builds(namespace string) storeBuildsNamespacer {
	return storeBuildsNamespacer{s.Indexer, namespace}
}

type storeBuildsNamespacer struct {
	Indexer   kcache.Indexer
	namespace string
}

func (s storeBuildsNamespacer) List(selector labels.Selector) (ret []*buildapi.Build, err error) {
	err = kcache.ListAllByNamespace(s.Indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*buildapi.Build))
	})
	return ret, err
}

func (s storeBuildsNamespacer) Get(name string) (*buildapi.Build, error) {
	obj, exists, err := s.Indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, kapierrors.NewNotFound(kapi.Resource("build"), name)
	}
	return obj.(*buildapi.Build), nil
}
