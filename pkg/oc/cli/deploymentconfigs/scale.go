package deploymentconfigs

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/kubectl"

	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	appsinternal "github.com/openshift/origin/pkg/apps/generated/internalclientset/typed/apps/internalversion"
	"github.com/openshift/origin/pkg/apps/util"
)

// NewDeploymentConfigScaler returns a new scaler for deploymentConfigs
func NewDeploymentConfigScaler(appsClient appsclient.Interface, kc kclientset.Interface) kubectl.Scaler {
	return &DeploymentConfigScaler{rcClient: kc.Core(), dcClient: appsClient.Apps(), clientInterface: kc}
}

// DeploymentConfigScaler is a wrapper for the kubectl Scaler client
type DeploymentConfigScaler struct {
	rcClient kcoreclient.ReplicationControllersGetter
	dcClient appsinternal.DeploymentConfigsGetter

	clientInterface kclientset.Interface
}

// Scale updates the DeploymentConfig with the provided namespace/name, to a
// new size, with optional precondition check (if preconditions is not nil),
// optional retries (if retry is not nil), and then optionally waits for its
// deployment replica count to reach the new value (if wait is not nil).
func (scaler *DeploymentConfigScaler) Scale(namespace, name string, newSize uint, preconditions *kubectl.ScalePrecondition, retry, waitForReplicas *kubectl.RetryParams) error {
	if preconditions == nil {
		preconditions = &kubectl.ScalePrecondition{Size: -1, ResourceVersion: ""}
	}
	if retry == nil {
		// Make it try only once, immediately
		retry = &kubectl.RetryParams{Interval: time.Millisecond, Timeout: time.Millisecond}
	}
	cond := kubectl.ScaleCondition(scaler, preconditions, namespace, name, newSize, nil)
	if err := wait.Poll(retry.Interval, retry.Timeout, cond); err != nil {
		return err
	}
	// TODO: convert to a watch and use resource version from the ScaleCondition - kubernetes/kubernetes#31051
	if waitForReplicas != nil {
		dc, err := scaler.dcClient.DeploymentConfigs(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		rc, err := scaler.rcClient.ReplicationControllers(namespace).Get(util.LatestDeploymentNameForConfig(dc), metav1.GetOptions{})
		if err != nil {
			return err
		}
		return wait.Poll(waitForReplicas.Interval, waitForReplicas.Timeout, controllerHasSpecifiedReplicas(scaler.clientInterface, rc, dc.Spec.Replicas))
	}
	return nil
}

// ScaleSimple does a simple one-shot attempt at scaling - not useful on its
// own, but a necessary building block for Scale.
func (scaler *DeploymentConfigScaler) ScaleSimple(namespace, name string, preconditions *kubectl.ScalePrecondition, newSize uint) (string, error) {
	scale, err := scaler.dcClient.DeploymentConfigs(namespace).GetScale(name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	scale.Spec.Replicas = int32(newSize)
	updated, err := scaler.dcClient.DeploymentConfigs(namespace).UpdateScale(name, scale)
	if err != nil {
		return "", kubectl.ScaleError{FailureType: kubectl.ScaleUpdateFailure, ResourceVersion: "Unknown", ActualError: err}
	}
	return updated.ResourceVersion, nil
}

// controllerHasSpecifiedReplicas returns a condition that will be true if and
// only if the specified replica count for a controller's ReplicaSelector
// equals the Replicas count.
//
// This is a slightly modified version of
// metav1.ControllerHasDesiredReplicas. This  is necessary because when
// scaling an RC via a DC, the RC spec replica count is not immediately
// updated to match the owning DC.
func controllerHasSpecifiedReplicas(c kclientset.Interface, controller *kapi.ReplicationController, specifiedReplicas int32) wait.ConditionFunc {
	// If we're given a controller where the status lags the spec, it either means that the controller is stale,
	// or that the rc manager hasn't noticed the update yet. Polling status.Replicas is not safe in the latter case.
	desiredGeneration := controller.Generation

	return func() (bool, error) {
		ctrl, err := c.Core().ReplicationControllers(controller.Namespace).Get(controller.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		// There's a chance a concurrent update modifies the Spec.Replicas causing this check to pass,
		// or, after this check has passed, a modification causes the rc manager to create more pods.
		// This will not be an issue once we've implemented graceful delete for rcs, but till then
		// concurrent stop operations on the same rc might have unintended side effects.
		return ctrl.Status.ObservedGeneration >= desiredGeneration && ctrl.Status.Replicas == specifiedReplicas, nil
	}
}
