package watch_endpointslice

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/origin/pkg/monitor/backenddisruption"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	discoveryv1 "k8s.io/api/discovery/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	discoveryinformers "k8s.io/client-go/informers/discovery/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/controller"
)

type EndpointSliceController struct {
	backendPrefix      string
	namespaceName      string
	serviceName        string
	myNodeName         string
	stopConfigMapName  string
	scheme             string
	path               string
	expectedStatusCode int
	recorder           monitorapi.RecorderWriter
	outFile            io.Writer

	endpointSliceLister discoverylisters.EndpointSliceLister
	configmapLister     corelisters.ConfigMapLister
	informersToSync     []cache.InformerSynced

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
	namespaceName string,
	serviceName string,
	stopConfigMapName string,
	myNodeName string,
	scheme string,
	path string,
	expectedStatusCode int,
	recorder monitorapi.RecorderWriter,
	outFile io.Writer,

	endpointSliceInformer discoveryinformers.EndpointSliceInformer,
	configmapInformer coreinformers.ConfigMapInformer,
) *EndpointSliceController {
	c := &EndpointSliceController{
		backendPrefix:      backendPrefix,
		serviceName:        serviceName,
		myNodeName:         myNodeName,
		namespaceName:      namespaceName,
		stopConfigMapName:  stopConfigMapName,
		scheme:             scheme,
		path:               path,
		expectedStatusCode: expectedStatusCode,
		recorder:           recorder,
		outFile:            outFile,

		endpointSliceLister: endpointSliceInformer.Lister(),
		configmapLister:     configmapInformer.Lister(),
		informersToSync: []cache.InformerSynced{
			configmapInformer.Informer().HasSynced,
			endpointSliceInformer.Informer().HasSynced,
		},

		watchers: map[string]*watcher{},

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "EndpointWatcher"),
	}
	c.syncHandler = c.syncEndpointSlice

	endpointSliceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.queue.Add("checK")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.queue.Add("checK")
		},
		DeleteFunc: func(obj interface{}) {
			c.queue.Add("checK")
		},
	})
	configmapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.queue.Add("checK")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.queue.Add("checK")
		},
		DeleteFunc: func(obj interface{}) {
			c.queue.Add("checK")
		},
	})
	return c
}

func (c *EndpointSliceController) syncEndpointSlice(ctx context.Context, key string) error {
	_, err := c.configmapLister.ConfigMaps(c.namespaceName).Get(c.stopConfigMapName)
	switch {
	case err == nil:
		c.removeAllWatchers()
		return nil
	case apierrors.IsNotFound(err):
	// good
	case err != nil:
		return err
	}

	targetServiceLabel, err := labels.NewRequirement("kubernetes.io/service-name", selection.Equals, []string{c.serviceName})
	if err != nil {
		return err
	}
	endpointSlices, err := c.endpointSliceLister.EndpointSlices(c.namespaceName).List(labels.NewSelector().Add(*targetServiceLabel))
	if err != nil {
		return err
	}

	// watchersForCurrEndpoints holds the metadata (but not the samplers) for all watchers that should be present
	// based on the current state of the endpointslices.
	// realistically, we expect at most one endpointslice, but more could exist
	watchersForCurrEndpoints := map[string]*watcher{}
	port := ""
	for _, endpointSlice := range endpointSlices {
		for _, currPort := range endpointSlice.Ports {
			if currPort.Port == nil {
				continue
			}
			port = fmt.Sprintf("%d", *currPort.Port)
		}
		for _, endpoint := range endpointSlice.Endpoints {
			if endpoint.Conditions.Serving == nil || !*endpoint.Conditions.Serving {
				continue
			}
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
	}
	if len(watchersForCurrEndpoints) == 0 {
		c.removeAllWatchers()
		return nil
	}

	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	// watchersToDelete holds all the watchers present in the running controller and not needed based on the endpointslice state.
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
		url := fmt.Sprintf("%s://%s%s", c.scheme, net.JoinHostPort(newWatcher.address, newWatcher.port), c.path)
		fmt.Fprintf(c.outFile, "Adding and starting: %v on node/%v\n", url, newWatcher.nodeName)

		// the interval locator is unique for every tuple of poller to target, but the backend is per connection type
		historicalBackendDisruptionDataForNewConnectionsName := fmt.Sprintf("%s-%v-connections", c.backendPrefix, monitorapi.NewConnectionType)
		historicalBackendDisruptionDataForReusedConnectionsName := fmt.Sprintf("%s-%v-connections", c.backendPrefix, monitorapi.ReusedConnectionType)
		intervalLocator := fmt.Sprintf("%s-from-node-%v-to-node-%v-endpoint-%v", c.backendPrefix, c.myNodeName, newWatcher.nodeName, newWatcher.address)
		newWatcher.newConnectionSampler = backenddisruption.NewSimpleBackendWithLocator(
			monitorapi.NewLocator().LocateDisruptionCheck(historicalBackendDisruptionDataForNewConnectionsName, intervalLocator, monitorapi.NewConnectionType),
			url,
			"",
			monitorapi.NewConnectionType,
		)
		if c.expectedStatusCode > 0 {
			newWatcher.newConnectionSampler = newWatcher.newConnectionSampler.WithExpectedStatusCode(c.expectedStatusCode)
		}
		newWatcher.newConnectionSampler.StartEndpointMonitoring(ctx, c.recorder, nil)

		newWatcher.reusedConnectionSampler = backenddisruption.NewSimpleBackendWithLocator(
			monitorapi.NewLocator().LocateDisruptionCheck(historicalBackendDisruptionDataForReusedConnectionsName, intervalLocator, monitorapi.ReusedConnectionType),
			url,
			"",
			monitorapi.ReusedConnectionType,
		)
		if c.expectedStatusCode > 0 {
			newWatcher.reusedConnectionSampler = newWatcher.reusedConnectionSampler.WithExpectedStatusCode(c.expectedStatusCode)
		}
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

	if !cache.WaitForNamedCacheSync("EndpointWatcher", ctx.Done(), c.informersToSync...) {
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
