package status

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/klog"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configv1listers "github.com/openshift/client-go/config/listers/config/v1"

	configv1helpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

var workQueueKey = "instance"

type VersionGetter interface {
	// SetVersion is a way to set the version for an operand.  It must be thread-safe
	SetVersion(operandName, version string)
	// GetVersion is way to get the versions for all operands.  It must be thread-safe and return an object that doesn't mutate
	GetVersions() map[string]string
	// VersionChangedChannel is a channel that will get an item whenever SetVersion has been called
	VersionChangedChannel() <-chan struct{}
}

type StatusSyncer struct {
	clusterOperatorName string
	relatedObjects      []configv1.ObjectReference

	versionGetter         VersionGetter
	operatorClient        operatorv1helpers.OperatorClient
	clusterOperatorClient configv1client.ClusterOperatorsGetter
	clusterOperatorLister configv1listers.ClusterOperatorLister

	cachesToSync  []cache.InformerSynced
	queue         workqueue.RateLimitingInterface
	eventRecorder events.Recorder
}

func NewClusterOperatorStatusController(
	name string,
	relatedObjects []configv1.ObjectReference,
	clusterOperatorClient configv1client.ClusterOperatorsGetter,
	clusterOperatorInformer configv1informers.ClusterOperatorInformer,
	operatorClient operatorv1helpers.OperatorClient,
	versionGetter VersionGetter,
	recorder events.Recorder,
) *StatusSyncer {
	c := &StatusSyncer{
		clusterOperatorName:   name,
		relatedObjects:        relatedObjects,
		versionGetter:         versionGetter,
		clusterOperatorClient: clusterOperatorClient,
		clusterOperatorLister: clusterOperatorInformer.Lister(),
		operatorClient:        operatorClient,
		eventRecorder:         recorder.WithComponentSuffix("status-controller"),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "StatusSyncer_"+strings.Replace(name, "-", "_", -1)),
	}

	operatorClient.Informer().AddEventHandler(c.eventHandler())
	clusterOperatorInformer.Informer().AddEventHandler(c.eventHandler())

	c.cachesToSync = append(c.cachesToSync, operatorClient.Informer().HasSynced)
	c.cachesToSync = append(c.cachesToSync, clusterOperatorInformer.Informer().HasSynced)

	return c
}

// sync reacts to a change in prereqs by finding information that is required to match another value in the cluster. This
// must be information that is logically "owned" by another component.
func (c StatusSyncer) sync() error {
	detailedSpec, currentDetailedStatus, _, err := c.operatorClient.GetOperatorState()
	if apierrors.IsNotFound(err) {
		c.eventRecorder.Warningf("StatusNotFound", "Unable to determine current operator status for clusteroperator/%s", c.clusterOperatorName)
		if err := c.clusterOperatorClient.ClusterOperators().Delete(c.clusterOperatorName, nil); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}

	originalClusterOperatorObj, err := c.clusterOperatorLister.Get(c.clusterOperatorName)
	if err != nil && !apierrors.IsNotFound(err) {
		c.eventRecorder.Warningf("StatusFailed", "Unable to get current operator status for clusteroperator/%s: %v", c.clusterOperatorName, err)
		return err
	}

	// ensure that we have a clusteroperator resource
	if originalClusterOperatorObj == nil || apierrors.IsNotFound(err) {
		klog.Infof("clusteroperator/%s not found", c.clusterOperatorName)
		var createErr error
		originalClusterOperatorObj, createErr = c.clusterOperatorClient.ClusterOperators().Create(&configv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{Name: c.clusterOperatorName},
		})
		if apierrors.IsNotFound(createErr) {
			// this means that the API isn't present.  We did not fail.  Try again later
			klog.Infof("ClusterOperator API not created")
			c.queue.AddRateLimited(workQueueKey)
			return nil
		}
		if createErr != nil {
			c.eventRecorder.Warningf("StatusCreateFailed", "Failed to create operator status: %v", err)
			return createErr
		}
	}
	clusterOperatorObj := originalClusterOperatorObj.DeepCopy()

	if detailedSpec.ManagementState == operatorv1.Unmanaged && !management.IsOperatorAlwaysManaged() {
		clusterOperatorObj.Status = configv1.ClusterOperatorStatus{}

		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterOperatorStatusCondition{Type: configv1.OperatorAvailable, Status: configv1.ConditionUnknown, Reason: "Unmanaged"})
		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterOperatorStatusCondition{Type: configv1.OperatorProgressing, Status: configv1.ConditionUnknown, Reason: "Unmanaged"})
		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterOperatorStatusCondition{Type: configv1.OperatorDegraded, Status: configv1.ConditionUnknown, Reason: "Unmanaged"})
		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterOperatorStatusCondition{Type: configv1.OperatorUpgradeable, Status: configv1.ConditionUnknown, Reason: "Unmanaged"})

		if equality.Semantic.DeepEqual(clusterOperatorObj, originalClusterOperatorObj) {
			return nil
		}
		if _, updateErr := c.clusterOperatorClient.ClusterOperators().UpdateStatus(clusterOperatorObj); err != nil {
			return updateErr
		}
		c.eventRecorder.Eventf("OperatorStatusChanged", "Status for operator %s changed: %s", c.clusterOperatorName, configv1helpers.GetStatusDiff(originalClusterOperatorObj.Status, clusterOperatorObj.Status))
		return nil
	}

	clusterOperatorObj.Status.RelatedObjects = c.relatedObjects
	configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, unionInertialCondition("Degraded", operatorv1.ConditionFalse, currentDetailedStatus.Conditions...))
	configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, unionCondition("Progressing", operatorv1.ConditionFalse, currentDetailedStatus.Conditions...))
	configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, unionCondition("Available", operatorv1.ConditionTrue, currentDetailedStatus.Conditions...))
	configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, unionCondition("Upgradeable", operatorv1.ConditionTrue, currentDetailedStatus.Conditions...))

	// TODO work out removal.  We don't always know the existing value, so removing early seems like a bad idea.  Perhaps a remove flag.
	versions := c.versionGetter.GetVersions()
	for operand, version := range versions {
		previousVersion := operatorv1helpers.SetOperandVersion(&clusterOperatorObj.Status.Versions, configv1.OperandVersion{Name: operand, Version: version})
		if previousVersion != version {
			// having this message will give us a marker in events when the operator updated compared to when the operand is updated
			c.eventRecorder.Eventf("OperatorVersionChanged", "clusteroperator/%s version %q changed from %q to %q", c.clusterOperatorName, operand, previousVersion, version)
		}
	}

	// if we have no diff, just return
	if equality.Semantic.DeepEqual(clusterOperatorObj, originalClusterOperatorObj) {
		return nil
	}
	klog.V(2).Infof("clusteroperator/%s diff %v", c.clusterOperatorName, resourceapply.JSONPatch(originalClusterOperatorObj, clusterOperatorObj))

	if _, updateErr := c.clusterOperatorClient.ClusterOperators().UpdateStatus(clusterOperatorObj); err != nil {
		return updateErr
	}
	c.eventRecorder.Eventf("OperatorStatusChanged", "Status for clusteroperator/%s changed: %s", c.clusterOperatorName, configv1helpers.GetStatusDiff(originalClusterOperatorObj.Status, clusterOperatorObj.Status))
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

	klog.Infof("Starting StatusSyncer-" + c.clusterOperatorName)
	defer klog.Infof("Shutting down StatusSyncer-" + c.clusterOperatorName)
	if !cache.WaitForCacheSync(stopCh, c.cachesToSync...) {
		return
	}

	// start watching for version changes
	go c.watchVersionGetter(stopCh)

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *StatusSyncer) watchVersionGetter(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	versionCh := c.versionGetter.VersionChangedChannel()
	// always kick at least once
	c.queue.Add(workQueueKey)

	for {
		select {
		case <-stopCh:
			return
		case <-versionCh:
			c.queue.Add(workQueueKey)
		}
	}
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
