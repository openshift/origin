package genericinformers

import (
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
)

type GenericResourceInformer interface {
	ForResource(resource schema.GroupVersionResource) (informers.GenericInformer, error)
	Start(stopCh <-chan struct{})
}

// GenericInternalResourceInformerFunc will return an internal informer for any resource matching
// its group resource, instead of the external version. Only valid for use where the type is accessed
// via generic interfaces, such as the garbage collector with ObjectMeta.
type GenericInternalResourceInformerFunc func(resource schema.GroupVersionResource) (informers.GenericInformer, error)

func (fn GenericInternalResourceInformerFunc) ForResource(resource schema.GroupVersionResource) (informers.GenericInformer, error) {
	resource.Version = runtime.APIVersionInternal
	return fn(resource)
}

// this is a temporary condition until we rewrite enough of generation to auto-conform to the required interface and no longer need the internal version shim
func (fn GenericInternalResourceInformerFunc) Start(stopCh <-chan struct{}) {}

// genericResourceInformerFunc will handle a cast to a matching type
type GenericResourceInformerFunc func(resource schema.GroupVersionResource) (informers.GenericInformer, error)

func (fn GenericResourceInformerFunc) ForResource(resource schema.GroupVersionResource) (informers.GenericInformer, error) {
	return fn(resource)
}

// this is a temporary condition until we rewrite enough of generation to auto-conform to the required interface and no longer need the internal version shim
func (fn GenericResourceInformerFunc) Start(stopCh <-chan struct{}) {}

type genericInformers struct {
	// this is a temporary condition until we rewrite enough of generation to auto-conform to the required interface and no longer need the internal version shim
	startFn func(stopCh <-chan struct{})
	generic []GenericResourceInformer
	// bias is a map that tries loading an informer from another GVR before using the original
	bias map[schema.GroupVersionResource]schema.GroupVersionResource
}

func NewGenericInformers(startFn func(stopCh <-chan struct{}), informers ...GenericResourceInformer) genericInformers {
	return genericInformers{
		startFn: startFn,
		generic: informers,
	}
}

func (i genericInformers) ForResource(resource schema.GroupVersionResource) (informers.GenericInformer, error) {
	if try, ok := i.bias[resource]; ok {
		if res, err := i.ForResource(try); err == nil {
			return res, nil
		}
	}

	var firstErr error
	for _, generic := range i.generic {
		informer, err := generic.ForResource(resource)
		if err == nil {
			return informer, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	glog.V(4).Infof("Couldn't find informer for %v", resource)
	return nil, firstErr
}

func (i genericInformers) Start(stopCh <-chan struct{}) {
	i.startFn(stopCh)
	for _, generic := range i.generic {
		generic.Start(stopCh)
	}
}
