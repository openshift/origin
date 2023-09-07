package poll_service

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
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type PollServiceController struct {
	backendPrefix     string
	nodeName          string
	clusterIP         string
	port              uint16
	namespaceName     string
	stopConfigMapName string
	recorder          monitorapi.RecorderWriter
	outFile           io.Writer

	configmapLister corelisters.ConfigMapLister

	informersToSync []cache.InformerSynced

	watcherLock sync.Mutex
	watcher     *watcher

	syncHandler func(ctx context.Context, key string) error
	queue       workqueue.RateLimitingInterface
}

type watcher struct {
	address                 string
	port                    uint16
	newConnectionSampler    *backenddisruption.BackendSampler
	reusedConnectionSampler *backenddisruption.BackendSampler
}

func NewPollServiceWatcher(
	backendPrefix string,
	nodeName string,
	namespaceName string,
	clusterIP string,
	port uint16,
	recorder monitorapi.RecorderWriter,
	outFile io.Writer,
	stopConfigMapName string,
	configmapInformer coreinformers.ConfigMapInformer,
) *PollServiceController {

	c := &PollServiceController{
		backendPrefix:     backendPrefix,
		nodeName:          nodeName,
		namespaceName:     namespaceName,
		clusterIP:         clusterIP,
		port:              port,
		recorder:          recorder,
		stopConfigMapName: stopConfigMapName,
		outFile:           outFile,

		configmapLister: configmapInformer.Lister(),
		informersToSync: []cache.InformerSynced{
			configmapInformer.Informer().HasSynced,
		},

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ServicePoller"),
	}

	c.syncHandler = c.syncServicePoller

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

func (c *PollServiceController) syncServicePoller(ctx context.Context, key string) error {
	_, err := c.configmapLister.ConfigMaps(c.namespaceName).Get(c.stopConfigMapName)
	switch {
	case err == nil:
		c.removeAllWatchers()
		return nil
	case apierrors.IsNotFound(err):
		// did not find the stopConfigMap
	case err != nil:
		return err
	}

	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	if c.watcher == nil {
		url := fmt.Sprintf("http://%s", net.JoinHostPort(c.clusterIP, fmt.Sprintf("%d", c.port)))
		fmt.Fprintf(c.outFile, "Adding and starting: %v on node/%v\n", url, c.nodeName)

		// the interval locator is unique for every tuple of poller to target, but the backend is per connection type
		historicalBackendDisruptionDataForNewConnectionsName := fmt.Sprintf("%s-%v-connections", c.backendPrefix, monitorapi.NewConnectionType)
		historicalBackendDisruptionDataForReusedConnectionsName := fmt.Sprintf("%s-%v-connections", c.backendPrefix, monitorapi.ReusedConnectionType)
		intervalLocator := fmt.Sprintf("%s-to-service-from-node-%v-to-clusterIP-%v", c.backendPrefix, c.nodeName, c.clusterIP)
		c.watcher = &watcher{
			address: c.clusterIP,
			port:    c.port,
			newConnectionSampler: backenddisruption.NewSimpleBackendWithLocator(
				monitorapi.NewLocator().LocateDisruptionCheck(historicalBackendDisruptionDataForNewConnectionsName, intervalLocator, monitorapi.NewConnectionType),
				url,
				"",
				monitorapi.NewConnectionType,
			),
			reusedConnectionSampler: backenddisruption.NewSimpleBackendWithLocator(
				monitorapi.NewLocator().LocateDisruptionCheck(historicalBackendDisruptionDataForReusedConnectionsName, intervalLocator, monitorapi.ReusedConnectionType),
				url,
				"",
				monitorapi.ReusedConnectionType,
			),
		}
		c.watcher.newConnectionSampler.StartEndpointMonitoring(ctx, c.recorder, nil)
		c.watcher.reusedConnectionSampler.StartEndpointMonitoring(ctx, c.recorder, nil)

		fmt.Fprintf(c.outFile, "Successfully started: %v on node/%v\n", url, c.nodeName)
	}
	return nil
}

func (c *PollServiceController) removeAllWatchers() {
	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	fmt.Fprintf(c.outFile, "Stopping and removing: %v for node/%v\n", c.watcher.address, c.nodeName)
	c.watcher.newConnectionSampler.Stop()
	c.watcher.reusedConnectionSampler.Stop()
	c.watcher = nil
	fmt.Fprintf(c.outFile, "Stopped all watchers\n")
}

func (c *PollServiceController) Run(ctx context.Context, finishedCleanup chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	defer close(finishedCleanup)

	logger := klog.FromContext(ctx)
	logger.Info("Starting PollService controller")
	defer logger.Info("Shutting down PollService controller")

	if !cache.WaitForNamedCacheSync("ServicePoller", ctx.Done(), c.informersToSync...) {
		return
	}
	go wait.UntilWithContext(ctx, c.runWorker, time.Second)

	<-ctx.Done()
	c.removeAllWatchers()

}

func (c *PollServiceController) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *PollServiceController) processNextWorkItem(ctx context.Context) bool {
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
