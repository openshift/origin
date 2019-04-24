package loglevel

import (
	"fmt"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/operator/events"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

var workQueueKey = "instance"

type LogLevelController struct {
	operatorClient operatorv1helpers.OperatorClient

	cachesToSync  []cache.InformerSynced
	queue         workqueue.RateLimitingInterface
	eventRecorder events.Recorder
}

// sets the klog level based on desired state
func NewClusterOperatorLoggingController(
	operatorClient operatorv1helpers.OperatorClient,
	recorder events.Recorder,
) *LogLevelController {
	c := &LogLevelController{
		operatorClient: operatorClient,
		eventRecorder:  recorder.WithComponentSuffix("loglevel-controller"),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "LoggingSyncer"),
	}

	operatorClient.Informer().AddEventHandler(c.eventHandler())

	c.cachesToSync = append(c.cachesToSync, operatorClient.Informer().HasSynced)

	return c
}

// sync reacts to a change in prereqs by finding information that is required to match another value in the cluster. This
// must be information that is logically "owned" by another component.
func (c LogLevelController) sync() error {
	detailedSpec, _, _, err := c.operatorClient.GetOperatorState()
	if err != nil {
		return err
	}

	logLevel := fmt.Sprintf("%d", LogLevelToKlog(detailedSpec.OperatorLogLevel))

	var level klog.Level

	oldLevel, ok := level.Get().(klog.Level)
	if !ok {
		oldLevel = level
	}

	if err := level.Set(logLevel); err != nil {
		c.eventRecorder.Warningf("LoglevelChangeFailed", "Unable to set loglevel level %v", err)
		return err
	}

	if oldLevel.String() != logLevel {
		c.eventRecorder.Eventf("LoglevelChange", "Changed loglevel level to %q", logLevel)
	}
	return nil
}

func (c *LogLevelController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting LogLevelController")
	defer klog.Infof("Shutting down LogLevelController")
	if !cache.WaitForCacheSync(stopCh, c.cachesToSync...) {
		return
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *LogLevelController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *LogLevelController) processNextWorkItem() bool {
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

// eventHandler queues the operator to check spec and loglevel
func (c *LogLevelController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}
