package loglevel

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

	currentLogLevel := CurrentLogLevel()
	desiredLogLevel := detailedSpec.OperatorLogLevel

	if len(desiredLogLevel) == 0 {
		desiredLogLevel = operatorv1.Normal
	}

	// When the current loglevel is the desired one, do nothing
	if currentLogLevel == desiredLogLevel {
		return nil
	}

	// Set the new loglevel if the operator spec changed
	if err := SetVerbosityValue(desiredLogLevel); err != nil {
		c.eventRecorder.Warningf("OperatorLoglevelChangeFailed", "Unable to change operator log level from %q to %q: %v", currentLogLevel, desiredLogLevel, err)
		return err
	}

	c.eventRecorder.Eventf("OperatorLoglevelChange", "Operator log level changed from %q to %q", currentLogLevel, desiredLogLevel)
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
