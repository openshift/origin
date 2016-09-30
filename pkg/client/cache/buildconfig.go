package cache

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

// StoreToBuildConfigLister gives a store List and Exists methods. The store must contain only buildconfigs.
type StoreToBuildConfigLister interface {
	List() ([]*buildapi.BuildConfig, error)
	GetConfigsForImageStreamTrigger(namespace, name string) ([]*buildapi.BuildConfig, error)
}

// StoreToBuildConfigListerImpl implements a StoreToBuildConfigLister
type StoreToBuildConfigListerImpl struct {
	cache.Indexer
}

// List all buildconfigs in the store.
func (s *StoreToBuildConfigListerImpl) List() ([]*buildapi.BuildConfig, error) {
	configs := []*buildapi.BuildConfig{}
	for _, c := range s.Indexer.List() {
		configs = append(configs, c.(*buildapi.BuildConfig))
	}
	return configs, nil
}

// GetConfigsForImageStream returns all the build configs that are triggered by the provided image stream
// by searching through using the ImageStreamReferenceIndex (build configs are indexed in the cache
// by image stream references).
func (s *StoreToBuildConfigListerImpl) GetConfigsForImageStreamTrigger(namespace, name string) ([]*buildapi.BuildConfig, error) {
	items, err := s.Indexer.ByIndex(ImageStreamReferenceIndex, namespace+"/"+name)
	if err != nil {
		return nil, err
	}

	var configs []*buildapi.BuildConfig
	for _, obj := range items {
		config := obj.(*buildapi.BuildConfig)
		configs = append(configs, config)
	}

	return configs, nil
}

func (s *StoreToBuildConfigListerImpl) BuildConfigs(namespace string) storeBuildConfigsNamespacer {
	return storeBuildConfigsNamespacer{s.Indexer, namespace}
}

// storeBuildConfigsNamespacer provides a way to get and list BuildConfigs from a specific namespace.
type storeBuildConfigsNamespacer struct {
	indexer   cache.Indexer
	namespace string
}

// Get the build config matching the name from the cache.
func (s storeBuildConfigsNamespacer) Get(name string) (*buildapi.BuildConfig, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, kapierrors.NewNotFound(buildapi.Resource("buildconfigs"), name)
	}
	return obj.(*buildapi.BuildConfig), nil
}

// List all the buildconfigs that match the provided selector using a namespace index.
// If the indexed list fails then we will fallback to listing from all namespaces and filter
// by the namespace we want.
func (s storeBuildConfigsNamespacer) List(selector labels.Selector) ([]*buildapi.BuildConfig, error) {
	configs := []*buildapi.BuildConfig{}

	if s.namespace == kapi.NamespaceAll {
		for _, obj := range s.indexer.List() {
			bc := obj.(*buildapi.BuildConfig)
			if selector.Matches(labels.Set(bc.Labels)) {
				configs = append(configs, bc)
			}
		}
		return configs, nil
	}

	items, err := s.indexer.ByIndex(cache.NamespaceIndex, s.namespace)
	if err != nil {
		return nil, err
	}
	for _, obj := range items {
		bc := obj.(*buildapi.BuildConfig)
		if selector.Matches(labels.Set(bc.Labels)) {
			configs = append(configs, bc)
		}
	}
	return configs, nil
}
