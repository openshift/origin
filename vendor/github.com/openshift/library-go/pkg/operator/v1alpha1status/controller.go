package v1alpha1status

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/v1alpha1helpers"
)

var workQueueKey = "instance"

type OperatorStatusProvider interface {
	Informer() cache.SharedIndexInformer
	CurrentStatus() (operatorv1alpha1.OperatorStatus, error)
}

type StatusSyncer struct {
	clusterOperatorNamespace string
	clusterOperatorName      string

	// TODO use a generated client when it moves to openshift/api
	clusterOperatorClient dynamic.ResourceInterface

	operatorStatusProvider OperatorStatusProvider

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewClusterOperatorStatusController(
	namespace, name string,
	clusterOperatorClient dynamic.Interface,
	operatorStatusProvider OperatorStatusProvider,
) *StatusSyncer {
	c := &StatusSyncer{
		clusterOperatorNamespace: namespace,
		clusterOperatorName:      name,
		clusterOperatorClient:    clusterOperatorClient.Resource(schema.GroupVersionResource{Group: "config.openshift.io", Version: "v1", Resource: "clusteroperators"}).Namespace(namespace),
		operatorStatusProvider:   operatorStatusProvider,

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
		return c.clusterOperatorClient.Delete(c.clusterOperatorName, nil)
	}
	if err != nil {
		return err
	}

	originalConfig, err := c.clusterOperatorClient.Get(c.clusterOperatorName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	operatorConfig := originalConfig.DeepCopy()

	if operatorConfig == nil {
		glog.Infof("clusterOperator %s/%s not found", c.clusterOperatorNamespace, c.clusterOperatorName)
		operatorConfig = &unstructured.Unstructured{Object: map[string]interface{}{}}
	}
	unstructured.RemoveNestedField(operatorConfig.Object, "status")
	unstructured.SetNestedField(operatorConfig.Object, "ClusterOperator", "kind")
	unstructured.SetNestedField(operatorConfig.Object, "config.openshift.io/v1", "apiVersion")
	unstructured.SetNestedField(operatorConfig.Object, c.clusterOperatorNamespace, "metadata", "namespace")
	unstructured.SetNestedField(operatorConfig.Object, c.clusterOperatorName, "metadata", "name")

	errorMessages := []string{}
	if currentDetailedStatus.TargetAvailability != nil {
		errorMessages = append(errorMessages, currentDetailedStatus.TargetAvailability.Errors...)
	}
	if currentDetailedStatus.CurrentAvailability != nil {
		unstructured.SetNestedField(operatorConfig.Object, currentDetailedStatus.CurrentAvailability.Version, "status", "version")
		errorMessages = append(errorMessages, currentDetailedStatus.CurrentAvailability.Errors...)
	}

	conditions := []interface{}{}
	availableCondition, err := OperatorConditionToClusterOperatorCondition(v1alpha1helpers.FindOperatorCondition(currentDetailedStatus.Conditions, operatorv1alpha1.OperatorStatusTypeAvailable))
	if err != nil {
		return err
	}
	if availableCondition != nil {
		conditions = append(conditions, availableCondition)
	}
	failingCondition, err := OperatorConditionToClusterOperatorCondition(v1alpha1helpers.FindOperatorCondition(currentDetailedStatus.Conditions, operatorv1alpha1.OperatorStatusTypeFailing))
	if err != nil {
		return err
	}
	if failingCondition != nil {
		conditions = append(conditions, failingCondition)
	}
	progressingCondition, err := OperatorConditionToClusterOperatorCondition(v1alpha1helpers.FindOperatorCondition(currentDetailedStatus.Conditions, operatorv1alpha1.OperatorStatusTypeProgressing))
	if err != nil {
		return err
	}
	if progressingCondition != nil {
		conditions = append(conditions, progressingCondition)
	}
	unstructured.SetNestedSlice(operatorConfig.Object, conditions, "status", "conditions")

	if equality.Semantic.DeepEqual(operatorConfig, originalConfig) {
		return nil
	}

	glog.V(4).Infof("clusterOperator %s/%s set to %v", c.clusterOperatorNamespace, c.clusterOperatorName, runtime.EncodeOrDie(unstructured.UnstructuredJSONScheme, operatorConfig))
	_, updateErr := c.clusterOperatorClient.UpdateStatus(operatorConfig)
	if apierrors.IsNotFound(updateErr) {
		freshOperatorConfig, createErr := c.clusterOperatorClient.Create(operatorConfig)
		if apierrors.IsNotFound(createErr) {
			// this means that the API isn't present.  We did not fail.  Try again later
			glog.Infof("ClusterOperator API not created")
			c.queue.AddRateLimited(workQueueKey)
			return nil
		}
		if createErr != nil {
			return createErr
		}
		if err := unstructured.SetNestedMap(freshOperatorConfig.Object, operatorConfig.Object["status"].(map[string]interface{}), "status"); err != nil {
			return err
		}
		_, updateErr = c.clusterOperatorClient.UpdateStatus(operatorConfig)
	}
	if updateErr != nil {
		return updateErr
	}

	return nil
}

func OperatorConditionToClusterOperatorCondition(condition *operatorv1alpha1.OperatorCondition) (map[string]interface{}, error) {
	if condition == nil {
		return nil, nil
	}
	buf := &bytes.Buffer{}
	if err := json.NewEncoder(buf).Encode(condition); err != nil {
		return nil, err
	}
	ret := map[string]interface{}{}
	if err := json.NewDecoder(buf).Decode(&ret); err != nil {
		return nil, err
	}

	return ret, nil
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
