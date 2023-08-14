package watch_endpointslice

import (
	"context"
	"fmt"
	"io"
	"net"
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
	backendPrefix string
	serviceName   string
	recorder      monitorapi.RecorderWriter
	outFile       io.Writer

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
	serviceName string,
	recorder monitorapi.RecorderWriter,
	outFile io.Writer,

	endpointSliceInformer discoveryinformers.EndpointSliceInformer,
) *EndpointSliceController {
	c := &EndpointSliceController{
		backendPrefix: backendPrefix,
		serviceName:   serviceName,
		recorder:      recorder,
		outFile:       outFile,

		endpointSliceLister:  endpointSliceInformer.Lister(),
		endpointSlicesSynced: endpointSliceInformer.Informer().HasSynced,

		watchers: map[string]*watcher{},

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EndpointWatcher"),
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
		c.removeAllWatchers()
		return nil
	}
	if err != nil {
		return err
	}

	watchersForCurrEndpoints := map[string]*watcher{}
	port := ""
	for _, currPort := range endpointSlice.Ports {
		if currPort.Port == nil {
			continue
		}
		port = fmt.Sprintf("%d", *currPort.Port)
	}
	for _, endpoint := range endpointSlice.Endpoints {
		for _, address := range endpoint.Addresses {
			watchersForCurrEndpoints[address] = &watcher{
				address: address,
				port:    port,
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
		fmt.Fprintf(c.outFile, "Stopping and removing: %v for node/%v\n", watcherToDelete.address, watcherToDelete.nodeName)
		watcherToDelete.newConnectionSampler.Stop()
		watcherToDelete.reusedConnectionSampler.Stop()
		delete(c.watchers, watcherKey)
	}

	for watcherKey := range watchersForCurrEndpoints {
		if _, ok := c.watchers[watcherKey]; ok {
			continue
		}
		newWatcher := watchersForCurrEndpoints[watcherKey]
		url := fmt.Sprintf("http://%s", net.JoinHostPort(newWatcher.address, newWatcher.port))
		fmt.Fprintf(c.outFile, "Adding and starting: %v on node/%v\n", url, newWatcher.nodeName)

		newWatcher.newConnectionSampler = backenddisruption.NewSimpleBackend(
			url,
			fmt.Sprintf("%s-new-connection-node-%v-endpoint-%v", c.backendPrefix, newWatcher.nodeName, newWatcher.address),
			"",
			monitorapi.NewConnectionType,
		)
		newWatcher.newConnectionSampler.StartEndpointMonitoring(ctx, c.recorder, nil)
		newWatcher.reusedConnectionSampler = backenddisruption.NewSimpleBackend(
			url,
			fmt.Sprintf("%s-reused-connection-node-%v-endpoint-%v", c.backendPrefix, newWatcher.nodeName, newWatcher.address),
			"",
			monitorapi.ReusedConnectionType,
		)
		newWatcher.reusedConnectionSampler.StartEndpointMonitoring(ctx, c.recorder, nil)

		c.watchers[watcherKey] = newWatcher

		fmt.Fprintf(c.outFile, "Successfully started: %v on node/%v\n", url, newWatcher.nodeName)
	}

	return err
}

func (c *EndpointSliceController) removeAllWatchers() {
	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	for _, watcherToDelete := range c.watchers {
		fmt.Fprintf(c.outFile, "Stopping and removing: %v for node/%v\n", watcherToDelete.address, watcherToDelete.nodeName)

		watcherToDelete.newConnectionSampler.Stop()
		watcherToDelete.reusedConnectionSampler.Stop()
	}

	fmt.Fprintf(c.outFile, "Stopped all watchers\n")
	c.watchers = map[string]*watcher{}
}

// Run starts the controller and blocks until stopCh is closed.
func (c *EndpointSliceController) Run(ctx context.Context, finishedCleanup chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	defer close(finishedCleanup)

	logger := klog.FromContext(ctx)
	logger.Info("Starting EndpointWatcher controller")
	defer logger.Info("Shutting down EndpointWatcher controller")

	if !cache.WaitForNamedCacheSync("EndpointWatcher", ctx.Done(), c.endpointSlicesSynced) {
		return
	}

	go wait.UntilWithContext(ctx, c.runWorker, time.Second)

	<-ctx.Done()

	c.removeAllWatchers()

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
	if endpointSlice.Labels["kubernetes.io/service-name"] != c.serviceName {
		return
	}

	c.enqueueEndpointSlice(endpointSlice)
}

func (c *EndpointSliceController) updateEndpointSlice(old, cur interface{}) {
	endpointSlice := cur.(*discoveryv1.EndpointSlice)
	if endpointSlice.Labels["kubernetes.io/service-name"] != c.serviceName {
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
	if endpointSlice.Labels["kubernetes.io/service-name"] != c.serviceName {
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
