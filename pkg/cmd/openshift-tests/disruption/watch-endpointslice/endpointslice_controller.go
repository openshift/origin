package watch_endpointslice

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	discoveryv1 "k8s.io/api/discovery/v1"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	discoveryinformers "k8s.io/client-go/informers/discovery/v1"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/controller"
)

type EndpointSliceController struct {
	backendPrefix    string
	targetLabelValue string
	recorder         monitorapi.RecorderWriter

	endpointSliceLister  discoverylisters.EndpointSliceLister
	endpointSlicesSynced cache.InformerSynced

	watcherLock sync.Mutex
	watchers    map[string]*watcher

	syncHandler func(ctx context.Context, key string) error
	queue       workqueue.RateLimitingInterface
}

type watcher struct {
	address                 string
	port                    string
	nodeName                string
	newConnectionSampler    *backenddisruption.BackendSampler
	reusedConnectionSampler *backenddisruption.BackendSampler
}

func NewEndpointWatcher(
	backendPrefix string,
	targetLabelValue string,
	recorder monitorapi.RecorderWriter,

	endpointSliceInformer discoveryinformers.EndpointSliceInformer,
) *EndpointSliceController {
	c := &EndpointSliceController{
		backendPrefix:    backendPrefix,
		targetLabelValue: targetLabelValue,
		recorder:         recorder,

		endpointSliceLister:  endpointSliceInformer.Lister(),
		endpointSlicesSynced: endpointSliceInformer.Informer().HasSynced,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ClusterRoleAggregator"),
	}
	c.syncHandler = c.syncEndpointSlice

	endpointSliceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addEndpointSlice,
		UpdateFunc: c.updateEndpointSlice,
		DeleteFunc: c.deleteEndpointSlice,
	})
	return c
}

func (c *EndpointSliceController) syncEndpointSlice(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	endpointSlice, err := c.endpointSliceLister.EndpointSlices(namespace).Get(name)
	if errors.IsNotFound(err) {
		// TODO remove all watchers
		return nil
	}
	if err != nil {
		return err
	}

	watchersForCurrEndpoints := map[string]*watcher{}
	for _, endpoint := range endpointSlice.Endpoints {
		for _, address := range endpoint.Addresses {
			watchersForCurrEndpoints[address] = &watcher{
				address: address,
				port:    "???",
			}
			if endpoint.NodeName != nil {
				watchersForCurrEndpoints[address].nodeName = *endpoint.NodeName
			}
		}
	}

	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	watchersToDelete := map[string]*watcher{}
	for watcherKey := range c.watchers {
		knownWatcher := c.watchers[watcherKey]
		if _, ok := watchersForCurrEndpoints[knownWatcher.address]; !ok {
			watchersToDelete[watcherKey] = knownWatcher
		}
	}
	for watcherKey, watcherToDelete := range watchersToDelete {
		watcherToDelete.newConnectionSampler.Stop()
		watcherToDelete.reusedConnectionSampler.Stop()
		delete(c.watchers, watcherKey)
	}

	for watcherKey := range watchersForCurrEndpoints {
		if _, ok := c.watchers[watcherKey]; ok {
			continue
		}
		newWatcher := watchersForCurrEndpoints[watcherKey]

		newWatcher.newConnectionSampler = backenddisruption.NewSimpleBackend(
			fmt.Sprintf("http://%s", newWatcher.address),
			fmt.Sprintf("%s-new-connection-node-%v-endpoint-%v", c.backendPrefix, newWatcher.nodeName, newWatcher.address),
			"",
			monitorapi.NewConnectionType,
		)
		newWatcher.newConnectionSampler.StartEndpointMonitoring(ctx, c.recorder, nil)
		newWatcher.reusedConnectionSampler = backenddisruption.NewSimpleBackend(
			fmt.Sprintf("http://%s", newWatcher.address),
			fmt.Sprintf("%s-reused-connection-node-%v-endpoint-%v", c.backendPrefix, newWatcher.nodeName, newWatcher.address),
			"",
			monitorapi.ReusedConnectionType,
		)
		newWatcher.reusedConnectionSampler.StartEndpointMonitoring(ctx, c.recorder, nil)

		c.watchers[watcherKey] = newWatcher
	}

	return err
}

// Run starts the controller and blocks until stopCh is closed.
func (c *EndpointSliceController) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	logger := klog.FromContext(ctx)
	logger.Info("Starting ClusterRoleAggregator controller")
	defer logger.Info("Shutting down ClusterRoleAggregator controller")

	if !cache.WaitForNamedCacheSync("ClusterRoleAggregator", ctx.Done(), c.endpointSlicesSynced) {
		return
	}

	go wait.UntilWithContext(ctx, c.runWorker, time.Second)

	<-ctx.Done()

	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	for _, watcherToDelete := range c.watchers {
		watcherToDelete.newConnectionSampler.Stop()
		watcherToDelete.reusedConnectionSampler.Stop()
	}
	c.watchers = map[string]*watcher{}
}

func (c *EndpointSliceController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *EndpointSliceController) processNextWorkItem(ctx context.Context) bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.syncHandler(ctx, dsKey.(string))
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

func (c *EndpointSliceController) addEndpointSlice(obj interface{}) {
	endpointSlice := obj.(*discoveryv1.EndpointSlice)
	if endpointSlice.Labels["target-label"] != c.targetLabelValue {
		return
	}

	c.enqueueEndpointSlice(endpointSlice)
}

func (c *EndpointSliceController) updateEndpointSlice(old, cur interface{}) {
	endpointSlice := cur.(*discoveryv1.EndpointSlice)
	if endpointSlice.Labels["target-label"] != c.targetLabelValue {
		return
	}

	c.enqueueEndpointSlice(endpointSlice)
}

func (c *EndpointSliceController) deleteEndpointSlice(obj interface{}) {
	endpointSlice, ok := obj.(*discoveryv1.EndpointSlice)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		endpointSlice, ok = tombstone.Obj.(*discoveryv1.EndpointSlice)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Deployment %#v", obj))
			return
		}
	}
	if endpointSlice.Labels["target-label"] != c.targetLabelValue {
		return
	}
	c.enqueueEndpointSlice(endpointSlice)
}
func (c *EndpointSliceController) enqueueEndpointSlice(endpointSlice *discoveryv1.EndpointSlice) {
	key, err := controller.KeyFunc(endpointSlice)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", endpointSlice, err))
		return
	}

	c.queue.Add(key)
}
