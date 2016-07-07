package controller

import (
	"fmt"
	"reflect"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/controller/framework"
	kresourcequota "k8s.io/kubernetes/pkg/controller/resourcequota"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/controller/shared"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// replenishmentControllerFactory implements ReplenishmentControllerFactory
type replenishmentControllerFactory struct {
	osClient osclient.Interface
}

var _ kresourcequota.ReplenishmentControllerFactory = &replenishmentControllerFactory{}

// NewReplenishmentControllerFactory returns a factory that knows how to build controllers
// to replenish resources when updated or deleted
func NewReplenishmentControllerFactory(osClient osclient.Interface) kresourcequota.ReplenishmentControllerFactory {
	return &replenishmentControllerFactory{
		osClient: osClient,
	}
}

func (r *replenishmentControllerFactory) NewController(options *kresourcequota.ReplenishmentControllerOptions) (framework.ControllerInterface, error) {
	switch options.GroupKind {
	case imageapi.Kind("ImageStream"):
		_, result := framework.NewInformer(
			&cache.ListWatch{
				ListFunc: func(options api.ListOptions) (runtime.Object, error) {
					return r.osClient.ImageStreams(api.NamespaceAll).List(options)
				},
				WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
					return r.osClient.ImageStreams(api.NamespaceAll).Watch(options)
				},
			},
			&imageapi.ImageStream{},
			options.ResyncPeriod(),
			framework.ResourceEventHandlerFuncs{
				UpdateFunc: ImageStreamReplenishmentUpdateFunc(options),
				DeleteFunc: kresourcequota.ObjectReplenishmentDeleteFunc(options),
			},
		)
		return result, nil
	default:
		return nil, fmt.Errorf("no replenishment controller available for %s", options.GroupKind)
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
		kresourcequota.NewReplenishmentControllerFactory(informerFactory.Pods().Informer(), kubeClientSet),
		NewReplenishmentControllerFactory(osClient),
	}
}
