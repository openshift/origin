package status

import (
	"context"
	"time"

	"k8s.io/klog/v2"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	configv1informers "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configv1listers "github.com/openshift/client-go/config/listers/config/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	configv1helpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/management"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorv1helpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

type VersionGetter interface {
	// SetVersion is a way to set the version for an operand.  It must be thread-safe
	SetVersion(operandName, version string)
	// GetVersion is way to get the versions for all operands.  It must be thread-safe and return an object that doesn't mutate
	GetVersions() map[string]string
	// VersionChangedChannel is a channel that will get an item whenever SetVersion has been called
	VersionChangedChannel() <-chan struct{}
}

type RelatedObjectsFunc func() (isset bool, objs []configv1.ObjectReference)

type StatusSyncer struct {
	clusterOperatorName string
	relatedObjects      []configv1.ObjectReference
	relatedObjectsFunc  RelatedObjectsFunc

	versionGetter         VersionGetter
	operatorClient        operatorv1helpers.OperatorClient
	clusterOperatorClient configv1client.ClusterOperatorsGetter
	clusterOperatorLister configv1listers.ClusterOperatorLister

	controllerFactory *factory.Factory
	recorder          events.Recorder
	degradedInertia   Inertia
}

var _ factory.Controller = &StatusSyncer{}

func (c *StatusSyncer) Name() string {
	return c.clusterOperatorName
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
	return &StatusSyncer{
		clusterOperatorName:   name,
		relatedObjects:        relatedObjects,
		versionGetter:         versionGetter,
		clusterOperatorClient: clusterOperatorClient,
		clusterOperatorLister: clusterOperatorInformer.Lister(),
		operatorClient:        operatorClient,
		degradedInertia:       MustNewInertia(2 * time.Minute).Inertia,
		controllerFactory: factory.New().ResyncEvery(time.Minute).WithInformers(
			operatorClient.Informer(),
			clusterOperatorInformer.Informer(),
		),
		recorder: recorder.WithComponentSuffix("status-controller"),
	}
}

// WithRelatedObjectsFunc allows the set of related objects to be dynamically
// determined.
//
// The function returns (isset, objects)
//
// If isset is false, then the set of related objects is copied over from the
// existing ClusterOperator object. This is useful in cases where an operator
// has just restarted, and hasn't yet reconciled.
//
// Any statically-defined related objects (in NewClusterOperatorStatusController)
// will always be included in the result.
func (c *StatusSyncer) WithRelatedObjectsFunc(f RelatedObjectsFunc) {
	c.relatedObjectsFunc = f
}

func (c *StatusSyncer) Run(ctx context.Context, workers int) {
	c.controllerFactory.WithPostStartHooks(c.watchVersionGetterPostRunHook).WithSync(c.Sync).ToController("StatusSyncer_"+c.Name(), c.recorder).Run(ctx, workers)
}

// WithDegradedInertia returns a copy of the StatusSyncer with the
// requested inertia function for degraded conditions.
func (c *StatusSyncer) WithDegradedInertia(inertia Inertia) *StatusSyncer {
	output := *c
	output.degradedInertia = inertia
	return &output
}

// sync reacts to a change in prereqs by finding information that is required to match another value in the cluster. This
// must be information that is logically "owned" by another component.
func (c StatusSyncer) Sync(ctx context.Context, syncCtx factory.SyncContext) error {
	detailedSpec, currentDetailedStatus, _, err := c.operatorClient.GetOperatorState()
	if apierrors.IsNotFound(err) {
		syncCtx.Recorder().Warningf("StatusNotFound", "Unable to determine current operator status for clusteroperator/%s", c.clusterOperatorName)
		if err := c.clusterOperatorClient.ClusterOperators().Delete(ctx, c.clusterOperatorName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}

	originalClusterOperatorObj, err := c.clusterOperatorLister.Get(c.clusterOperatorName)
	if err != nil && !apierrors.IsNotFound(err) {
		syncCtx.Recorder().Warningf("StatusFailed", "Unable to get current operator status for clusteroperator/%s: %v", c.clusterOperatorName, err)
		return err
	}

	// ensure that we have a clusteroperator resource
	if originalClusterOperatorObj == nil || apierrors.IsNotFound(err) {
		klog.Infof("clusteroperator/%s not found", c.clusterOperatorName)
		var createErr error
		originalClusterOperatorObj, createErr = c.clusterOperatorClient.ClusterOperators().Create(ctx, &configv1.ClusterOperator{
			ObjectMeta: metav1.ObjectMeta{Name: c.clusterOperatorName},
		}, metav1.CreateOptions{})
		if apierrors.IsNotFound(createErr) {
			// this means that the API isn't present.  We did not fail.  Try again later
			klog.Infof("ClusterOperator API not created")
			syncCtx.Queue().AddRateLimited(factory.DefaultQueueKey)
			return nil
		}
		if createErr != nil {
			syncCtx.Recorder().Warningf("StatusCreateFailed", "Failed to create operator status: %v", err)
			return createErr
		}
	}
	clusterOperatorObj := originalClusterOperatorObj.DeepCopy()

	if detailedSpec.ManagementState == operatorv1.Unmanaged && !management.IsOperatorAlwaysManaged() {
		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterOperatorStatusCondition{Type: configv1.OperatorAvailable, Status: configv1.ConditionUnknown, Reason: "Unmanaged"})
		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterOperatorStatusCondition{Type: configv1.OperatorProgressing, Status: configv1.ConditionUnknown, Reason: "Unmanaged"})
		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterOperatorStatusCondition{Type: configv1.OperatorDegraded, Status: configv1.ConditionUnknown, Reason: "Unmanaged"})
		configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, configv1.ClusterOperatorStatusCondition{Type: configv1.OperatorUpgradeable, Status: configv1.ConditionUnknown, Reason: "Unmanaged"})

		if equality.Semantic.DeepEqual(clusterOperatorObj, originalClusterOperatorObj) {
			return nil
		}
		if _, err := c.clusterOperatorClient.ClusterOperators().UpdateStatus(ctx, clusterOperatorObj, metav1.UpdateOptions{}); err != nil {
			return err
		}
		syncCtx.Recorder().Eventf("OperatorStatusChanged", "Status for operator %s changed: %s", c.clusterOperatorName, configv1helpers.GetStatusDiff(originalClusterOperatorObj.Status, clusterOperatorObj.Status))
		return nil
	}

	if c.relatedObjectsFunc != nil {
		isSet, ro := c.relatedObjectsFunc()
		if !isSet { // temporarily unknown - copy over from existing object
			ro = clusterOperatorObj.Status.RelatedObjects
		}

		// merge in any static objects
		for _, obj := range c.relatedObjects {
			found := false
			for _, existingObj := range ro {
				if obj == existingObj {
					found = true
					break
				}
			}
			if !found {
				ro = append(ro, obj)
			}
		}
		clusterOperatorObj.Status.RelatedObjects = ro
	} else {
		clusterOperatorObj.Status.RelatedObjects = c.relatedObjects
	}

	configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, UnionClusterCondition("Degraded", operatorv1.ConditionFalse, c.degradedInertia, currentDetailedStatus.Conditions...))
	configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, UnionClusterCondition("Progressing", operatorv1.ConditionFalse, nil, currentDetailedStatus.Conditions...))
	configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, UnionClusterCondition("Available", operatorv1.ConditionTrue, nil, currentDetailedStatus.Conditions...))
	configv1helpers.SetStatusCondition(&clusterOperatorObj.Status.Conditions, UnionClusterCondition("Upgradeable", operatorv1.ConditionTrue, nil, currentDetailedStatus.Conditions...))

	// TODO work out removal.  We don't always know the existing value, so removing early seems like a bad idea.  Perhaps a remove flag.
	versions := c.versionGetter.GetVersions()
	for operand, version := range versions {
		previousVersion := operatorv1helpers.SetOperandVersion(&clusterOperatorObj.Status.Versions, configv1.OperandVersion{Name: operand, Version: version})
		if previousVersion != version {
			// having this message will give us a marker in events when the operator updated compared to when the operand is updated
			syncCtx.Recorder().Eventf("OperatorVersionChanged", "clusteroperator/%s version %q changed from %q to %q", c.clusterOperatorName, operand, previousVersion, version)
		}
	}

	// if we have no diff, just return
	if equality.Semantic.DeepEqual(clusterOperatorObj, originalClusterOperatorObj) {
		return nil
	}
	klog.V(2).Infof("clusteroperator/%s diff %v", c.clusterOperatorName, resourceapply.JSONPatchNoError(originalClusterOperatorObj, clusterOperatorObj))

	if _, updateErr := c.clusterOperatorClient.ClusterOperators().UpdateStatus(ctx, clusterOperatorObj, metav1.UpdateOptions{}); err != nil {
		return updateErr
	}
	syncCtx.Recorder().Eventf("OperatorStatusChanged", "Status for clusteroperator/%s changed: %s", c.clusterOperatorName, configv1helpers.GetStatusDiff(originalClusterOperatorObj.Status, clusterOperatorObj.Status))
	return nil
}

func (c *StatusSyncer) watchVersionGetterPostRunHook(ctx context.Context, syncCtx factory.SyncContext) error {
	defer utilruntime.HandleCrash()

	versionCh := c.versionGetter.VersionChangedChannel()
	// always kick at least once
	syncCtx.Queue().Add(factory.DefaultQueueKey)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-versionCh:
			syncCtx.Queue().Add(factory.DefaultQueueKey)
		}
	}
}
