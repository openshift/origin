package staleconditions

import (
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const workQueueKey = "key"

type RemoveStaleConditions struct {
	conditions []string

	operatorClient v1helpers.OperatorClient
	cachesToSync   []cache.InformerSynced

	eventRecorder events.Recorder
	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewRemoveStaleConditions(
	conditions []string,
	operatorClient v1helpers.OperatorClient,
	eventRecorder events.Recorder,
) *RemoveStaleConditions {
	c := &RemoveStaleConditions{
		conditions: conditions,

		operatorClient: operatorClient,
		eventRecorder:  eventRecorder,
		cachesToSync:   []cache.InformerSynced{operatorClient.Informer().HasSynced},

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "RemoveStaleConditions"),
	}

	operatorClient.Informer().AddEventHandler(c.eventHandler())

	return c
}

func (c RemoveStaleConditions) sync() error {
	removeStaleConditionsFn := func(status *operatorv1.OperatorStatus) error {
		for _, condition := range c.conditions {
			v1helpers.RemoveOperatorCondition(&status.Conditions, condition)
		}
		return nil
	}

	if _, _, err := v1helpers.UpdateStatus(c.operatorClient, removeStaleConditionsFn); err != nil {
		return err
	}

	return nil
}

// Run starts the kube-scheduler and blocks until stopCh is closed.
func (c *RemoveStaleConditions) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting RemoveStaleConditions")
	defer klog.Infof("Shutting down RemoveStaleConditions")

	if !cache.WaitForCacheSync(stopCh, c.cachesToSync...) {
		utilruntime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *RemoveStaleConditions) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *RemoveStaleConditions) processNextWorkItem() bool {
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

// eventHandler queues the operator to check spec and status
func (c *RemoveStaleConditions) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}
