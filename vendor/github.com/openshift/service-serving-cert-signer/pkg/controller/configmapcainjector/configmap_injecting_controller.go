package configmapcainjector

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	InjectCABundleAnnotation = "service.alpha.openshift.io/inject-cabundle"
	InjectionDataKey         = "service-ca.crt"
)

// ConfigMapCABundleInjectionController is responsible for injecting a CA bundle into configMaps annotated with
// "service.alpha.openshift.io/inject-cabundle"
type ConfigMapCABundleInjectionController struct {
	configMapClient kcoreclient.ConfigMapsGetter

	// configMaps that need to be checked
	queue workqueue.RateLimitingInterface

	configMapLister    listers.ConfigMapLister
	configMapHasSynced cache.InformerSynced

	ca string

	// syncHandler does the work. It's factored out for unit testing
	syncHandler func(serviceKey string) error
}

// NewConfigMapCABundleInjectionController creates a new ServiceServingCertUpdateController.
// TODO this should accept a shared informer
func NewConfigMapCABundleInjectionController(configMaps informers.ConfigMapInformer, configMapsClient kcoreclient.ConfigMapsGetter, ca []byte, resyncInterval time.Duration) *ConfigMapCABundleInjectionController {
	ic := &ConfigMapCABundleInjectionController{
		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		ca:    string(ca),
	}

	ic.configMapLister = configMaps.Lister()
	configMaps.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    ic.addConfigMap,
			UpdateFunc: ic.updateConfigMap,
		},
		resyncInterval,
	)
	ic.configMapHasSynced = configMaps.Informer().HasSynced
	ic.configMapClient = configMapsClient
	ic.syncHandler = ic.syncConfigMap

	return ic
}

// Run begins watching and syncing.
func (ic *ConfigMapCABundleInjectionController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer ic.queue.ShutDown()

	// Wait for the stores to fill
	if !cache.WaitForCacheSync(stopCh, ic.configMapHasSynced) {
		return
	}

	glog.V(1).Infof("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(ic.runWorker, time.Second, stopCh)
	}
	<-stopCh
	glog.V(1).Infof("Shutting down")
}

func (ic *ConfigMapCABundleInjectionController) enqueueConfigMap(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %+v: %v", obj, err))
		return
	}

	ic.queue.Add(key)
}

func (ic *ConfigMapCABundleInjectionController) addConfigMap(obj interface{}) {
	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		// We should only ever get configMaps from the event handler.
		glog.V(2).Infof("added object not configMap type: %T", obj)
		return
	}

	if !hasCABundleAnnotation(cm) {
		return
	}

	glog.V(4).Infof("adding %s", cm.Name)
	ic.enqueueConfigMap(cm)
}

func (ic *ConfigMapCABundleInjectionController) updateConfigMap(old, cur interface{}) {
	cm, ok := cur.(*v1.ConfigMap)
	if !ok {
		// We should only ever get configMaps from the event handler.
		glog.V(2).Infof("updated object not configMap type: %T", cur)
		return
	}

	if !hasCABundleAnnotation(cm) {
		return
	}

	glog.V(4).Infof("updating %s", cm.Name)
	ic.enqueueConfigMap(cm)
}

func (ic *ConfigMapCABundleInjectionController) runWorker() {
	for ic.processNextWorkItem() {
	}
}

// processNextWorkItem deals with one key off the queue.  It returns false when it's time to quit.
func (ic *ConfigMapCABundleInjectionController) processNextWorkItem() bool {
	key, quit := ic.queue.Get()
	if quit {
		return false
	}
	defer ic.queue.Done(key)

	err := ic.syncHandler(key.(string))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
		ic.queue.AddRateLimited(key)
		return true
	}

	ic.queue.Forget(key)
	return true
}

// syncSecret will sync the configMap with the CA.
// This function is not meant to be invoked concurrently with the same key.
func (ic *ConfigMapCABundleInjectionController) syncConfigMap(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	sharedConfigMap, err := ic.configMapLister.ConfigMaps(namespace).Get(name)
	if kapierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	// skip updating when the CA bundle is already there
	if data, ok := sharedConfigMap.Data[InjectionDataKey]; ok && data == ic.ca {
		return nil
	}

	configMapCopy := sharedConfigMap.DeepCopy()
	// make a copy to avoid mutating cache state
	configMapCopy.Data[InjectionDataKey] = ic.ca

	_, err = ic.configMapClient.ConfigMaps(configMapCopy.Namespace).Update(configMapCopy)
	return err
}

func hasCABundleAnnotation(cm *v1.ConfigMap) bool {
	return strings.ToLower(strings.TrimSpace(cm.Annotations[InjectCABundleAnnotation])) == "true"
}
