package recorder

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsv1beta1informer "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1beta1"
	apiextensionsv1beta1lister "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1beta1"
)

const (
	controllerWorkQueueKey = "key"
	defaultResyncDuration  = 5 * time.Minute
)

type ConfigObserverController struct {
	cachesToSync []cache.InformerSynced
	queue        workqueue.RateLimitingInterface
	stopCh       <-chan struct{}

	monitoredCustomResources []schema.GroupVersion

	crdLister       apiextensionsv1beta1lister.CustomResourceDefinitionLister
	crdInformer     cache.SharedIndexInformer
	dynamicClient   dynamic.Interface
	cachedDiscovery discovery.CachedDiscoveryInterface

	dynamicInformers []*dynamicConfigInformer
	storageHandler   cache.ResourceEventHandler
}

func New(
	dynamicClient dynamic.Interface,
	extensionsClient apiextensionsclient.Interface,
	discoveryClient *discovery.DiscoveryClient,
	configStorage cache.ResourceEventHandler,
) (*ConfigObserverController, error) {
	c := &ConfigObserverController{
		dynamicClient:  dynamicClient,
		queue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ConfigObserverController"),
		crdInformer:    apiextensionsv1beta1informer.NewCustomResourceDefinitionInformer(extensionsClient, defaultResyncDuration, cache.Indexers{}),
		storageHandler: configStorage,
	}

	c.cachedDiscovery = memory.NewMemCacheClient(discoveryClient)
	c.crdLister = apiextensionsv1beta1lister.NewCustomResourceDefinitionLister(c.crdInformer.GetIndexer())
	c.crdInformer.AddEventHandler(c.eventHandler())

	c.cachesToSync = []cache.InformerSynced{
		c.crdInformer.HasSynced,
	}

	return c, nil
}

func (c *ConfigObserverController) AddMonitoredCustomResourceGroup(gv schema.GroupVersion) {
	c.monitoredCustomResources = append(c.monitoredCustomResources, gv)
}

// currentResourceKinds returns list of group version configKind for OpenShift configuration types.
func (c *ConfigObserverController) currentResourceKinds() ([]schema.GroupVersionKind, error) {
	observedCrds, err := c.crdLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}
	var (
		currentConfigResources []schema.GroupVersionKind
		currentKinds           = sets.NewString()
	)
	for _, crd := range observedCrds {
		for _, gv := range c.monitoredCustomResources {
			if !strings.HasSuffix(crd.GetName(), "."+gv.Group) {
				continue
			}
			for _, version := range crd.Spec.Versions {
				if !version.Served {
					continue
				}
				gvk := schema.GroupVersionKind{
					Group:   gv.Group,
					Version: gv.Version,
					Kind:    crd.Spec.Names.Kind,
				}
				if currentKinds.Has(gvk.Kind) {
					continue
				}
				currentKinds.Insert(gvk.Kind)
				currentConfigResources = append(currentConfigResources, gvk)
			}
		}

	}
	return currentConfigResources, nil
}

func (c *ConfigObserverController) sync() error {
	current, err := c.currentResourceKinds()
	if err != nil {
		return err
	}

	// TODO: The CRD delete case is not handled
	var (
		currentList      []string
		needObserverList []string
		kindNeedObserver []schema.GroupVersionKind
	)
	for _, configKind := range current {
		currentList = append(currentList, configKind.String())
		hasObserver := false
		for _, o := range c.dynamicInformers {
			if o.isKind(configKind) {
				hasObserver = true
				break
			}
		}
		if !hasObserver {
			kindNeedObserver = append(kindNeedObserver, configKind)
			needObserverList = append(needObserverList, configKind.String())
		}
	}

	var (
		waitForCacheSyncFn  []cache.InformerSynced
		syntheticRequeueErr error
	)

	// If we have new CRD refresh the discovery info and update the mapper
	if len(kindNeedObserver) > 0 {
		// NOTE: this is very time expensive, only do this when we have new kinds
		c.cachedDiscovery.Invalidate()
		gr, err := restmapper.GetAPIGroupResources(c.cachedDiscovery)
		if err != nil {
			return err
		}

		mapper := restmapper.NewDiscoveryRESTMapper(gr)
		for _, kind := range kindNeedObserver {
			mapping, err := mapper.RESTMapping(kind.GroupKind(), kind.Version)
			if err != nil {
				klog.Warningf("Unable to find REST mapping for %s/%s: %v (will retry)", kind.GroupKind(), kind.Version, err)
				// better luck next time
				syntheticRequeueErr = err
				continue
			}

			// we got mapping, lets run the dynamicInformer for the config and install GIT storageHandler event handlers
			dynamicInformer := newDynamicConfigInformer(kind.Kind, mapping.Resource, c.dynamicClient, c.storageHandler)
			waitForCacheSyncFn = append(waitForCacheSyncFn, dynamicInformer.hasSynced)

			go func(k schema.GroupVersionKind) {
				defer klog.V(3).Infof("Shutting down dynamic informer for %q ...", k.String())
				klog.V(3).Infof("Starting dynamic informer for %q ...", k.String())
				dynamicInformer.run(c.stopCh)
			}(kind)
			c.dynamicInformers = append(c.dynamicInformers, dynamicInformer)
		}
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	if !cache.WaitForCacheSync(ctx.Done(), waitForCacheSyncFn...) {
		return fmt.Errorf("timeout while waiting for dynamic informers to start: %#v", kindNeedObserver)
	}

	return syntheticRequeueErr
}

// eventHandler queues the operator to check spec and status
func (c *ConfigObserverController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(controllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(controllerWorkQueueKey) },
	}
}

// Run starts the kube-apiserver and blocks until stopCh is closed.
func (c *ConfigObserverController) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	// Passed to individual dynamic informers
	c.stopCh = stopCh

	klog.Infof("Starting ConfigObserver")
	defer klog.Infof("Shutting down ConfigObserver")

	go func() {
		klog.V(3).Infof("Starting CRD informer ...")
		defer klog.V(3).Infof("Shutting down CRD informer ...")
		c.crdInformer.Run(stopCh)
	}()

	klog.Infof("Waiting for caches to sync ...")
	if !cache.WaitForCacheSync(stopCh, c.cachesToSync...) {
		panic("Failed to wait for caches to sync ...")
	}
	klog.V(5).Infof("Successfully synchronized caches")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *ConfigObserverController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ConfigObserverController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}
