package apiservicecabundle

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/golang/glog"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	apiregistrationapiv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiserviceclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset/typed/apiregistration/v1"
	apiserviceinformer "k8s.io/kube-aggregator/pkg/client/informers/externalversions/apiregistration/v1"
	apiservicelister "k8s.io/kube-aggregator/pkg/client/listers/apiregistration/v1"
)

const (
	InjectCABundleAnnotationName = "service.alpha.openshift.io/inject-cabundle"
)

type ServiceServingCertUpdateController struct {
	apiServiceClient apiserviceclient.APIServicesGetter

	// Services that need to be checked
	queue workqueue.RateLimitingInterface

	apiServiceLister    apiservicelister.APIServiceLister
	apiServiceHasSynced cache.InformerSynced

	caBundle []byte

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

func NewAPIServiceCABundleInjector(apiServiceInformer apiserviceinformer.APIServiceInformer, apiServiceClient apiserviceclient.APIServicesGetter, caBundle []byte) *ServiceServingCertUpdateController {
	sc := &ServiceServingCertUpdateController{
		apiServiceClient: apiServiceClient,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		caBundle: caBundle,
	}

	sc.apiServiceLister = apiServiceInformer.Lister()
	apiServiceInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    sc.addAPIService,
			UpdateFunc: sc.updateAPIService,
		},
	)
	sc.apiServiceHasSynced = apiServiceInformer.Informer().GetController().HasSynced

	sc.syncHandler = sc.syncAPIService

	return sc
}

func (c *ServiceServingCertUpdateController) syncAPIService(key string) error {
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
	if strings.ToLower(apiService.Annotations[InjectCABundleAnnotationName]) != "true" {
		return nil
	}
	if reflect.DeepEqual(apiService.Spec.CABundle, c.caBundle) {
		return nil
	}

	apiServiceToUpdate := apiService.DeepCopy()
	apiServiceToUpdate.Spec.CABundle = c.caBundle
	_, err = c.apiServiceClient.APIServices().Update(apiServiceToUpdate)
	return err
}

// Run begins watching and syncing.
func (c *ServiceServingCertUpdateController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, c.apiServiceHasSynced) {
		return
	}

	glog.V(5).Infof("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
	glog.V(1).Infof("Shutting down")
}

func (c *ServiceServingCertUpdateController) addAPIService(obj interface{}) {
	apiService := obj.(*apiregistrationapiv1.APIService)
	if strings.ToLower(apiService.Annotations[InjectCABundleAnnotationName]) != "true" {
		return
	}

	glog.V(4).Infof("adding %s", apiService.Name)
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", obj, err))
	}
	c.queue.Add(key)
}

func (c *ServiceServingCertUpdateController) updateAPIService(old, cur interface{}) {
	obj := cur.(*apiregistrationapiv1.APIService)
	if strings.ToLower(obj.Annotations[InjectCABundleAnnotationName]) != "true" {
		return
	}

	glog.V(4).Infof("updating %s", obj.Name)
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", obj, err))
	}
	c.queue.Add(key)
}

func (c *ServiceServingCertUpdateController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (c *ServiceServingCertUpdateController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}
