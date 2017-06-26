package cache

import (
	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// StoreToImageStreamLister gives a store List and Exists methods. The store must contain only image streams.
type StoreToImageStreamLister struct {
	cache.Indexer
}

// List all image streams in the store.
func (s *StoreToImageStreamLister) List() ([]*imageapi.ImageStream, error) {
	streams := []*imageapi.ImageStream{}
	for _, obj := range s.Indexer.List() {
		streams = append(streams, obj.(*imageapi.ImageStream))
	}
	return streams, nil
}

func (s *StoreToImageStreamLister) ImageStreams(namespace string) storeImageStreamsNamespacer {
	return storeImageStreamsNamespacer{s.Indexer, namespace}
}

// GetStreamsForConfig returns all the image streams that the provided deployment config points to.
func (s *StoreToImageStreamLister) GetStreamsForConfig(config *deployapi.DeploymentConfig) []*imageapi.ImageStream {
	var streams []*imageapi.ImageStream

	for _, t := range config.Spec.Triggers {
		if t.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}

		from := t.ImageChangeParams.From
		name, _, _ := imageapi.SplitImageStreamTag(from.Name)
		stream, err := s.ImageStreams(from.Namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			glog.Infof("Cannot retrieve image stream %s/%s: %v", from.Namespace, name, err)
			continue
		}
		streams = append(streams, stream)
	}

	return streams
}

type storeImageStreamsNamespacer struct {
	indexer   cache.Indexer
	namespace string
}

// Get the image stream matching the name from the cache.
func (s storeImageStreamsNamespacer) Get(name string, options metav1.GetOptions) (*imageapi.ImageStream, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, kapierrors.NewNotFound(imageapi.Resource("imagestream"), name)
	}
	return obj.(*imageapi.ImageStream), nil
}

// List all the image streams that match the provided selector using a namespace index.
// If the indexed list fails then we will fallback to listing from all namespaces and filter
// by the namespace we want.
func (s storeImageStreamsNamespacer) List(selector labels.Selector) ([]*imageapi.ImageStream, error) {
	streams := []*imageapi.ImageStream{}

	if s.namespace == metav1.NamespaceAll {
		for _, obj := range s.indexer.List() {
			stream := obj.(*imageapi.ImageStream)
			if selector.Matches(labels.Set(stream.Labels)) {
				streams = append(streams, stream)
			}
		}
		return streams, nil
	}

	items, err := s.indexer.ByIndex(cache.NamespaceIndex, s.namespace)
	if err != nil {
		return nil, err
	}
	for _, obj := range items {
		stream := obj.(*imageapi.ImageStream)
		if selector.Matches(labels.Set(stream.Labels)) {
			streams = append(streams, stream)
		}
	}
	return streams, nil
}
