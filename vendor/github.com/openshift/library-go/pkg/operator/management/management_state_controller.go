package management

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

var workQueueKey = "instance"

// ManagementStateController watches changes of `managementState` field and react in case that field is set to an unsupported value.
// As each operator can opt-out from supporting `unmanaged` or `removed` states, this controller will add failing condition when the
// value for this field is set to this values for those operators.
type ManagementStateController struct {
	operatorName   string
	operatorClient operatorv1helpers.OperatorClient
	eventRecorder  events.Recorder

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewOperatorManagementStateController(
	name string,
	operatorStatusProvider operatorv1helpers.OperatorClient,
	recorder events.Recorder,
) *ManagementStateController {
	c := &ManagementStateController{
		operatorName:   name,
		operatorClient: operatorStatusProvider,
		eventRecorder:  recorder,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "ManagementStateController-"+name),
	}

	operatorStatusProvider.Informer().AddEventHandler(c.eventHandler())
	// TODO watch clusterOperator.status changes when it moves to openshift/api

	return c
}

func (c ManagementStateController) sync() error {
	detailedSpec, _, _, err := c.operatorClient.GetOperatorState()
	if apierrors.IsNotFound(err) {
		c.eventRecorder.Warningf("StatusNotFound", "Unable to determine current operator status for %s", c.operatorName)
		return nil
	}

	cond := operatorv1.OperatorCondition{
		Type:   "ManagementStateFailing",
		Status: operatorv1.ConditionFalse,
	}

	if IsOperatorAlwaysManaged() && detailedSpec.ManagementState == operatorv1.Unmanaged {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Unmanaged"
		cond.Message = fmt.Sprintf("Unmanaged is not supported for %s operator", c.operatorName)
	}

	if IsOperatorNotRemovable() && detailedSpec.ManagementState == operatorv1.Removed {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Removed"
		cond.Message = fmt.Sprintf("Removed is not supported for %s operator", c.operatorName)
	}

	if IsOperatorUnknownState(detailedSpec.ManagementState) {
		cond.Status = operatorv1.ConditionTrue
		cond.Reason = "Unknown"
		cond.Message = fmt.Sprintf("Unsupported management state %q for %s operator", detailedSpec.ManagementState, c.operatorName)
	}

	if _, _, updateError := v1helpers.UpdateStatus(c.operatorClient, v1helpers.UpdateConditionFn(cond)); updateError != nil {
		if err == nil {
			return updateError
		}
	}

	return nil
}

func (c *ManagementStateController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting management-state-controller-" + c.operatorName)
	defer glog.Infof("Shutting down management-state-controller-" + c.operatorName)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *ManagementStateController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *ManagementStateController) processNextWorkItem() bool {
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
func (c *ManagementStateController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}
