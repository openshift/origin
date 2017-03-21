package controller

import (
	"fmt"
	"reflect"

	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/controller/shared"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// replenishmentControllerFactory implements ReplenishmentControllerFactory
type replenishmentControllerFactory struct {
	isInformer shared.ImageStreamInformer
}

var _ kresourcequota.ReplenishmentControllerFactory = &replenishmentControllerFactory{}

// NewReplenishmentControllerFactory returns a factory that knows how to build controllers
// to replenish resources when updated or deleted
func NewReplenishmentControllerFactory(isInformer shared.ImageStreamInformer) kresourcequota.ReplenishmentControllerFactory {
	return &replenishmentControllerFactory{
		isInformer: isInformer,
	}
}

func (r *replenishmentControllerFactory) NewController(options *kresourcequota.ReplenishmentControllerOptions) (cache.ControllerInterface, error) {
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
func NewAllResourceReplenishmentControllerFactory(informerFactory shared.InformerFactory, osClient osclient.Interface, kubeClientSet clientset.Interface) kresourcequota.ReplenishmentControllerFactory {
	return kresourcequota.UnionReplenishmentControllerFactory{
		kresourcequota.NewReplenishmentControllerFactory(informerFactory.KubernetesInformers(), kubeClientSet),
		NewReplenishmentControllerFactory(informerFactory.ImageStreams()),
	}
}
