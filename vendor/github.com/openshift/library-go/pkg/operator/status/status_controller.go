package status

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configv1helpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

var workQueueKey = "instance"

type OperatorStatusProvider interface {
	Informer() cache.SharedIndexInformer
	CurrentStatus() (operatorv1.OperatorStatus, error)
}

type StatusSyncer struct {
	clusterOperatorName string

	// TODO use a generated client when it moves to openshift/api
	clusterOperatorClient configv1client.ClusterOperatorsGetter
	eventRecorder         events.Recorder

	operatorStatusProvider OperatorStatusProvider

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewClusterOperatorStatusController(
	name string,
	clusterOperatorClient configv1client.ClusterOperatorsGetter,
	operatorStatusProvider OperatorStatusProvider,
	recorder events.Recorder,
) *StatusSyncer {
	c := &StatusSyncer{
		clusterOperatorName:    name,
		clusterOperatorClient:  clusterOperatorClient,
		operatorStatusProvider: operatorStatusProvider,
		eventRecorder:          recorder,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "StatusSyncer-"+name),
	}

	operatorStatusProvider.Informer().AddEventHandler(c.eventHandler())
	// TODO watch clusterOperator.status changes when it moves to openshift/api

	return c
}

// sync reacts to a change in prereqs by finding information that is required to match another value in the cluster. This
// must be information that is logically "owned" by another component.
func (c StatusSyncer) sync() error {
	currentDetailedStatus, err := c.operatorStatusProvider.CurrentStatus()
	if apierrors.IsNotFound(err) {
		glog.Infof("operator.status not found")
		c.eventRecorder.Warningf("StatusNotFound", "Unable to determine current operator status for %s", c.clusterOperatorName)
		return c.clusterOperatorClient.ClusterOperators().Delete(c.clusterOperatorName, nil)
	}
	if err != nil {
		return err
	}

	originalClusterOperatorObj, err := c.clusterOperatorClient.ClusterOperators().Get(c.clusterOperatorName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		c.eventRecorder.Warningf("StatusFailed", "Unable to get current operator status for %s: %v", c.clusterOperatorName, err)
		return err
	}
	clusterOperatorObj := originalClusterOperatorObj.DeepCopy()

	if clusterOperatorObj == nil || apierrors.IsNotFound(err) {
		glog.Infof("clusteroperator/%s not found", c.clusterOperatorName)
		clusterOperatorObj = &configv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{Name: c.clusterOperatorName},
		}
	}
	clusterOperatorObj.Status.Conditions = nil

	var failingConditions []operatorv1.OperatorCondition
	for _, condition := range currentDetailedStatus.Conditions {
		if strings.HasSuffix(condition.Type, "Failing") && condition.Status == operatorv1.ConditionTrue {
			failingConditions = append(failingConditions, condition)
		}
	}
	failingCondition := operatorv1.OperatorCondition{Type: operatorv1.OperatorStatusTypeFailing, Status: operatorv1.ConditionUnknown}
	if len(failingConditions) > 0 {
		failingCondition.Status = operatorv1.ConditionTrue
		var messages []string
		latestTransitionTime := metav1.Time{}
		for _, condition := range failingConditions {
			if latestTransitionTime.Before(&condition.LastTransitionTime) {
				latestTransitionTime = condition.LastTransitionTime
			}

			if len(condition.Message) == 0 {
				continue
			}
			for _, message := range strings.Split(condition.Message, "\n") {
				messages = append(messages, fmt.Sprintf("%s: %s", condition.Type, message))
			}
		}
		if len(messages) > 0 {
			failingCondition.Message = strings.Join(messages, "\n")
		}
		if len(failingConditions) == 1 {
			failingCondition.Reason = failingConditions[0].Type
		} else {
			failingCondition.Reason = "MultipleConditionsFailing"
		}
		failingCondition.LastTransitionTime = latestTransitionTime

	} else {
		failingCondition.Status = operatorv1.ConditionFalse
	}
	configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, OperatorConditionToClusterOperatorCondition(failingCondition))

	if condition := operatorv1helpers.FindOperatorCondition(currentDetailedStatus.Conditions, operatorv1.OperatorStatusTypeAvailable); condition != nil {
		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, OperatorConditionToClusterOperatorCondition(*condition))
	} else {
		configv1helpers.RemoveStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterStatusConditionType(operatorv1.OperatorStatusTypeAvailable))
	}
	if condition := operatorv1helpers.FindOperatorCondition(currentDetailedStatus.Conditions, operatorv1.OperatorStatusTypeProgressing); condition != nil {
		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, OperatorConditionToClusterOperatorCondition(*condition))
	} else {
		configv1helpers.RemoveStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterStatusConditionType(operatorv1.OperatorStatusTypeProgressing))
	}

	if equality.Semantic.DeepEqual(clusterOperatorObj, originalClusterOperatorObj) {
		return nil
	}

	glog.V(4).Infof("clusteroperator/%s set to %v", c.clusterOperatorName, runtime.EncodeOrDie(unstructured.UnstructuredJSONScheme, clusterOperatorObj))

	if len(clusterOperatorObj.ResourceVersion) != 0 {
		if _, updateErr := c.clusterOperatorClient.ClusterOperators().UpdateStatus(clusterOperatorObj); err != nil {
			return updateErr
		}
		c.eventRecorder.Eventf("OperatorStatusChanged", "Status for operator %s changed", c.clusterOperatorName)
		return nil
	}

	freshOperatorConfig, createErr := c.clusterOperatorClient.ClusterOperators().Create(clusterOperatorObj)
	if apierrors.IsNotFound(createErr) {
		// this means that the API isn't present.  We did not fail.  Try again later
		glog.Infof("ClusterOperator API not created")
		c.queue.AddRateLimited(workQueueKey)
		return nil
	}
	if createErr != nil {
		c.eventRecorder.Warningf("StatusCreateFailed", "Failed to create operator status: %v", err)
		return createErr
	}

	if condition := configv1helpers.FindStatusCondition(clusterOperatorObj.Status.Conditions, configv1.OperatorAvailable); condition != nil {
		configv1helpers.SetStatusCondition(&freshOperatorConfig.Status.Conditions, *condition)
	} else {
		configv1helpers.RemoveStatusCondition(&freshOperatorConfig.Status.Conditions, configv1.OperatorAvailable)
	}
	if condition := configv1helpers.FindStatusCondition(clusterOperatorObj.Status.Conditions, configv1.OperatorProgressing); condition != nil {
		configv1helpers.SetStatusCondition(&freshOperatorConfig.Status.Conditions, *condition)
	} else {
		configv1helpers.RemoveStatusCondition(&freshOperatorConfig.Status.Conditions, configv1.OperatorProgressing)
	}
	if condition := configv1helpers.FindStatusCondition(clusterOperatorObj.Status.Conditions, configv1.OperatorFailing); condition != nil {
		configv1helpers.SetStatusCondition(&freshOperatorConfig.Status.Conditions, *condition)
	} else {
		configv1helpers.RemoveStatusCondition(&freshOperatorConfig.Status.Conditions, configv1.OperatorFailing)
	}

	if _, updateErr := c.clusterOperatorClient.ClusterOperators().UpdateStatus(freshOperatorConfig); updateErr != nil {
		return updateErr
	}
	c.eventRecorder.Eventf("OperatorStatusChanged", "Status for operator %s changed", c.clusterOperatorName)

	return nil
}

func OperatorConditionToClusterOperatorCondition(condition operatorv1.OperatorCondition) configv1.ClusterOperatorStatusCondition {
	return configv1.ClusterOperatorStatusCondition{
		Type:               configv1.ClusterStatusConditionType(condition.Type),
		Status:             configv1.ConditionStatus(condition.Status),
		LastTransitionTime: condition.LastTransitionTime,
		Reason:             condition.Reason,
		Message:            condition.Message,
	}
}

func (c *StatusSyncer) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting StatusSyncer-" + c.clusterOperatorName)
	defer glog.Infof("Shutting down StatusSyncer-" + c.clusterOperatorName)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *StatusSyncer) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *StatusSyncer) processNextWorkItem() bool {
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
func (c *StatusSyncer) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}
