package controller

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informers "k8s.io/client-go/informers/core/v1"
	kcoreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/openshift/service-serving-cert-signer/pkg/boilerplate/controller"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/api"
)

// ConfigMapCABundleInjectionController is responsible for injecting a CA bundle into configMaps annotated with
// "service.alpha.openshift.io/inject-cabundle"
type ConfigMapCABundleInjectionController struct {
	configMapClient kcoreclient.ConfigMapsGetter
	configMapLister listers.ConfigMapLister

	ca string

	// configMaps that need to be checked
	queue workqueue.RateLimitingInterface

	// standard controller loop
	*controller.Controller
}

func NewConfigMapCABundleInjectionController(configMaps informers.ConfigMapInformer, configMapsClient kcoreclient.ConfigMapsGetter, ca string, resyncInterval time.Duration) *ConfigMapCABundleInjectionController {
	ic := &ConfigMapCABundleInjectionController{
		configMapClient: configMapsClient,
		configMapLister: configMaps.Lister(),
		ca:              ca,
	}

	configMaps.Informer().AddEventHandlerWithResyncPeriod(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    ic.addConfigMap,
			UpdateFunc: ic.updateConfigMap,
		},
		resyncInterval,
	)

	internalController, queue := controller.New("ConfigMapCABundleInjectionController", ic.syncConfigMap, configMaps.Informer().HasSynced)

	ic.Controller = internalController
	ic.queue = queue

	return ic
}

func (ic *ConfigMapCABundleInjectionController) syncConfigMap(obj interface{}) error {
	key := obj.(string)
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
	if data, ok := sharedConfigMap.Data[api.InjectionDataKey]; ok && data == ic.ca {
		return nil
	}

	// make a copy to avoid mutating cache state
	configMapCopy := sharedConfigMap.DeepCopy()

	if configMapCopy.Data == nil {
		configMapCopy.Data = map[string]string{}
	}

	configMapCopy.Data[api.InjectionDataKey] = ic.ca

	_, err = ic.configMapClient.ConfigMaps(configMapCopy.Namespace).Update(configMapCopy)
	return err
}

func (c *ConfigMapCABundleInjectionController) handleConfigMap(obj interface{}, event string) {
	cm := obj.(*v1.ConfigMap)
	if !api.HasInjectCABundleAnnotation(cm.Annotations) {
		return
	}

	glog.V(4).Infof("%s %s", event, cm.Name)
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("could not get key for object %+v: %v", obj, err))
		return
	}

	c.queue.Add(key)
}

func (c *ConfigMapCABundleInjectionController) addConfigMap(obj interface{}) {
	c.handleConfigMap(obj, "adding")
}

func (c *ConfigMapCABundleInjectionController) updateConfigMap(old, cur interface{}) {
	c.handleConfigMap(cur, "updating")
}
