package controller

import (
	"fmt"
	"reflect"

	"k8s.io/client-go/tools/cache"
	kexternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinternalversion "github.com/openshift/origin/pkg/image/generated/informers/internalversion/image/internalversion"
)

// replenishmentControllerFactory implements ReplenishmentControllerFactory
type replenishmentControllerFactory struct {
	isInformer imageinternalversion.ImageStreamInformer
}

var _ kresourcequota.ReplenishmentControllerFactory = &replenishmentControllerFactory{}

// NewReplenishmentControllerFactory returns a factory that knows how to build controllers
// to replenish resources when updated or deleted
func NewReplenishmentControllerFactory(isInformer imageinternalversion.ImageStreamInformer) kresourcequota.ReplenishmentControllerFactory {
	return &replenishmentControllerFactory{
		isInformer: isInformer,
	}
}

func (r *replenishmentControllerFactory) NewController(options *kresourcequota.ReplenishmentControllerOptions) (cache.Controller, error) {
	gk := options.GroupKind
	switch {
	case imageapi.IsKindOrLegacy("ImageStream", gk):
		r.isInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			UpdateFunc: ImageStreamReplenishmentUpdateFunc(options),
			DeleteFunc: kresourcequota.ObjectReplenishmentDeleteFunc(options),
		})
		return r.isInformer.Informer().GetController(), nil
	default:
		return nil, fmt.Errorf("no replenishment controller available for %s", gk)
	}
}

// ImageStreamReplenishmentUpdateFunc will replenish if the old image stream was quota tracked but the new is not
func ImageStreamReplenishmentUpdateFunc(options *kresourcequota.ReplenishmentControllerOptions) func(oldObj, newObj interface{}) {
	return func(oldObj, newObj interface{}) {
		oldIS := oldObj.(*imageapi.ImageStream)
		newIS := newObj.(*imageapi.ImageStream)
		if !reflect.DeepEqual(oldIS.Status.Tags, newIS.Status.Tags) {
			options.ReplenishmentFunc(options.GroupKind, newIS.Namespace, newIS)
		}
	}
}

// NewAllResourceReplenishmentControllerFactory returns a ReplenishmentControllerFactory  that knows how to replenish all known resources
func NewAllResourceReplenishmentControllerFactory(informerFactory kexternalinformers.SharedInformerFactory, imageStreamInformer imageinternalversion.ImageStreamInformer, osClient osclient.Interface) kresourcequota.ReplenishmentControllerFactory {
	return kresourcequota.UnionReplenishmentControllerFactory{
		kresourcequota.NewReplenishmentControllerFactory(informerFactory),
		NewReplenishmentControllerFactory(imageStreamInformer),
	}
}
