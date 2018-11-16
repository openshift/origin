package controller

import (
	"bytes"
	"fmt"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	apiregistrationapiv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiserviceclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	apiserviceinformer "k8s.io/kube-aggregator/pkg/client/informers/externalversions/apiregistration/v1"
	apiservicelister "k8s.io/kube-aggregator/pkg/client/listers/apiregistration/v1"

	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controller"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
)

type ServiceServingCertUpdateController struct {
	apiServiceClient apiserviceclient.APIServicesGetter
	apiServiceLister apiservicelister.APIServiceLister

	caBundle []byte

	// services that need to be checked
	queue workqueue.RateLimitingInterface

	// standard controller loop
	*controller.Controller
}

func NewAPIServiceCABundleInjector(apiServiceInformer apiserviceinformer.APIServiceInformer, apiServiceClient apiserviceclient.APIServicesGetter, caBundle []byte) *ServiceServingCertUpdateController {
	sc := &ServiceServingCertUpdateController{
		apiServiceClient: apiServiceClient,
		apiServiceLister: apiServiceInformer.Lister(),
		caBundle:         caBundle,
	}

	apiServiceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sc.addAPIService,
			UpdateFunc: sc.updateAPIService,
		},
	)

	internalController, queue := controller.New("APIServiceCABundleInjector", sc.syncAPIService, apiServiceInformer.Informer().GetController().HasSynced)

	sc.Controller = internalController
	sc.queue = queue

	return sc
}

func (c *ServiceServingCertUpdateController) syncAPIService(obj interface{}) error {
	key := obj.(string)
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	apiService, err := c.apiServiceLister.Get(name)
	if kapierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !api.HasInjectCABundleAnnotation(apiService.Annotations) {
		return nil
	}
	if bytes.Equal(apiService.Spec.CABundle, c.caBundle) {
		return nil
	}

	// avoid mutating our cache
	apiServiceToUpdate := apiService.DeepCopy()
	apiServiceToUpdate.Spec.CABundle = c.caBundle
	_, err = c.apiServiceClient.APIServices().Update(apiServiceToUpdate)
	return err
}

func (c *ServiceServingCertUpdateController) handleAPIService(obj interface{}, event string) {
	apiService := obj.(*apiregistrationapiv1.APIService)
	if !api.HasInjectCABundleAnnotation(apiService.Annotations) {
		return
	}

	glog.V(4).Infof("%s %s", event, apiService.Name)
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("could not get key for object %+v: %v", obj, err))
		return
	}

	c.queue.Add(key)
}

func (c *ServiceServingCertUpdateController) addAPIService(obj interface{}) {
	c.handleAPIService(obj, "adding")
}

func (c *ServiceServingCertUpdateController) updateAPIService(old, cur interface{}) {
	c.handleAPIService(cur, "updating")
}
