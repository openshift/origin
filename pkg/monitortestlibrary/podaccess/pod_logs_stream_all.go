package podaccess

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/labels"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/util/wait"
)

type PodsStreamer struct {
	kubeClient    kubernetes.Interface
	namespaceName string
	selector      labels.Selector
	containerName string
	podLister     corelisters.PodLister

	logHandler LogHandler

	watcherLock sync.Mutex
	watchers    map[podKey]*watcher

	syncHandler func(ctx context.Context) error
	queue       workqueue.RateLimitingInterface
}

type watcher struct {
	pod           *corev1.Pod
	containerName string
	podStreamer   *PodStreamer
}

// intentionally avoid UID since the client connects with this key, not a UID key
type podKey struct {
	namespace string
	name      string
}

type LogHandler interface {
	HandleLogLine(logLine LogLineContent)
}

func NewPodsStreamer(
	kubeClient kubernetes.Interface,
	selector labels.Selector,
	namespaceName string,
	containerName string,
	logHandler LogHandler,

	podInformer coreinformers.PodInformer,
) *PodsStreamer {
	c := &PodsStreamer{
		kubeClient:    kubeClient,
		namespaceName: namespaceName,
		selector:      selector,
		containerName: containerName,
		podLister:     podInformer.Lister(),
		logHandler:    logHandler,

		watcherLock: sync.Mutex{},
		watchers:    map[podKey]*watcher{},
		queue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "PodsLogsStream"),
	}
	c.syncHandler = c.syncPods

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.queue.Add("check")
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			c.queue.Add("check")
		},
		DeleteFunc: func(obj interface{}) {
			c.queue.Add("check")
		},
	})
	return c
}

func (c *PodsStreamer) Stop(ctx context.Context) {
	c.removeAllWatchers(ctx)
}

func (c *PodsStreamer) syncPods(ctx context.Context) error {
	pods, err := c.podLister.Pods(c.namespaceName).List(c.selector)
	if err != nil {
		return err
	}

	watchersForCurrPods := map[podKey]*watcher{}
	for i := range pods {
		pod := pods[i]
		// skip pods that are not on nodes
		if len(pod.Spec.NodeName) == 0 {
			continue
		}

		currKey := podKey{
			namespace: pod.Namespace,
			name:      pod.Name,
		}
		watchersForCurrPods[currKey] = &watcher{
			pod:           pod,
			containerName: c.containerName,
		}
	}
	if len(watchersForCurrPods) == 0 {
		c.removeAllWatchers(ctx)
		return nil
	}

	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	// watchersToDelete holds all the watchers present in the running controller and not needed based on the endpointslice state.
	watchersToDelete := map[podKey]*watcher{}
	for watcherKey := range c.watchers {
		knownWatcher := c.watchers[watcherKey]
		if _, ok := watchersForCurrPods[watcherKey]; !ok {
			watchersToDelete[watcherKey] = knownWatcher
		}
	}
	for watcherKey, watcherToDelete := range watchersToDelete {
		watcherToDelete.podStreamer.Stop(ctx)
		delete(c.watchers, watcherKey)
	}

	for watcherKey := range watchersForCurrPods {
		if _, ok := c.watchers[watcherKey]; ok {
			continue
		}
		newWatcher := watchersForCurrPods[watcherKey]
		newWatcher.podStreamer = NewPodStreamer(c.kubeClient, watcherKey.namespace, watcherKey.name, c.containerName)
		go newWatcher.podStreamer.Run(ctx)

		in, inErr := newWatcher.podStreamer.Output()
		go c.forwardLogLines(ctx, in)
		go c.forwardLogErrors(ctx, inErr)

		c.watchers[watcherKey] = newWatcher
	}

	return err
}

// forwardLogLines exits when ctx is done or in is finished and we've consumed everything
func (c *PodsStreamer) forwardLogLines(ctx context.Context, in chan LogLineContent) {
	defer utilruntime.HandleCrash()

	for {
		select {
		case curr, ok := <-in:
			if !ok {
				return
			}
			c.logHandler.HandleLogLine(curr)
		case <-ctx.Done():
			return
		}
	}
}

func (c *PodsStreamer) forwardLogErrors(ctx context.Context, inErr chan LogError) {
	defer utilruntime.HandleCrash()

	for {
		select {
		case curr, ok := <-inErr:
			if !ok {
				return
			}
			klog.Error(curr)
		case <-ctx.Done():
			return
		}
	}
}

func (c *PodsStreamer) removeAllWatchers(ctx context.Context) {
	c.watcherLock.Lock()
	defer c.watcherLock.Unlock()

	wg := sync.WaitGroup{}
	for i := range c.watchers {
		watcherToDelete := c.watchers[i]
		wg.Add(1)

		// these can take a bit to shutdown, shut them down in parallel.
		go func(ctx context.Context, watcherToDelete *watcher) {
			defer utilruntime.HandleCrash()
			defer wg.Done()
			watcherToDelete.podStreamer.Stop(ctx)
		}(ctx, watcherToDelete)
	}
	wg.Wait()

	c.watchers = map[podKey]*watcher{}
}

// Run starts the controller and blocks until stopCh is closed.
func (c *PodsStreamer) Run(ctx context.Context, finishedCleanup chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()
	defer close(finishedCleanup)

	logger := klog.FromContext(ctx)
	logger.Info("Starting PodsLogStreamer")
	defer logger.Info("Shutting down PodsLogStreamer controller")

	go wait.UntilWithContext(ctx, c.runWorker, time.Second)

	<-ctx.Done()

	// TODO set a timeout?
	c.removeAllWatchers(context.TODO())
}

func (c *PodsStreamer) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *PodsStreamer) processNextWorkItem(ctx context.Context) bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(ctx)
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}
