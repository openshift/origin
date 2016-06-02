package cache

import (
	"fmt"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/labels"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// StoreToImageStreamLister gives a store List and Exists methods. The store must contain only image streams.
type StoreToImageStreamLister struct {
	cache.Indexer
}

// Exists checks if the given image stream exists in the store.
func (s *StoreToImageStreamLister) Exists(stream *imageapi.ImageStream) (bool, error) {
	_, exists, err := s.Indexer.Get(stream)
	return exists, err
}

// List all image streams in the store.
func (s *StoreToImageStreamLister) List() ([]imageapi.ImageStream, error) {
	streams := []imageapi.ImageStream{}
	for _, obj := range s.Indexer.List() {
		streams = append(streams, *(obj.(*imageapi.ImageStream)))
	}
	return streams, nil
}

// GetStreamsForDeploymentConfig returns all the image streams for the provided deployment config.
func (s *StoreToImageStreamLister) GetStreamsForDeploymentConfig(dc *deployapi.DeploymentConfig) ([]*imageapi.ImageStream, error) {
	streams := []*imageapi.ImageStream{}

	for _, key := range deployutil.ImageStreamKeysFor(dc) {
		obj, exists, err := s.Indexer.GetByKey(key)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, fmt.Errorf("image stream %q not found", key)
		}
		streams = append(streams, obj.(*imageapi.ImageStream))
	}

	return streams, nil
}

func (s *StoreToImageStreamLister) ImageStreams(namespace string) storeImageStreamsNamespacer {
	return storeImageStreamsNamespacer{s.Indexer, namespace}
}

type storeImageStreamsNamespacer struct {
	indexer   cache.Indexer
	namespace string
}

// List all the image stream that match the provided selector using a namespace index.
// If the indexed list fails then we will fallback to listing from all namespaces and filter
// by the namespace we want.
func (s storeImageStreamsNamespacer) List(selector labels.Selector) ([]imageapi.ImageStream, error) {
	streams := []imageapi.ImageStream{}

	if s.namespace == kapi.NamespaceAll {
		for _, obj := range s.indexer.List() {
			stream := *(obj.(*imageapi.ImageStream))
			if selector.Matches(labels.Set(stream.Labels)) {
				streams = append(streams, stream)
			}
		}
		return streams, nil
	}

	key := &imageapi.ImageStream{ObjectMeta: kapi.ObjectMeta{Namespace: s.namespace}}
	// TODO: Use cache.NamespaceIndex once the rebase lands
	items, err := s.indexer.Index("namespace", key)
	if err != nil {
		// Ignore error; do slow search without index.
		glog.Warningf("can not retrieve list of objects using index : %v", err)
		for _, obj := range s.indexer.List() {
			stream := *(obj.(*imageapi.ImageStream))
			if s.namespace == stream.Namespace && selector.Matches(labels.Set(stream.Labels)) {
				streams = append(streams, stream)
			}
		}
		return streams, nil
	}
	for _, obj := range items {
		stream := *(obj.(*imageapi.ImageStream))
		if selector.Matches(labels.Set(stream.Labels)) {
			streams = append(streams, stream)
		}
	}
	return streams, nil
}
