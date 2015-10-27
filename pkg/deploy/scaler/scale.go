package scaler

import (
	"time"

	"github.com/golang/glog"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/deploy/util"
)

// NewDeploymentConfigScaler returns a new scaler for deploymentConfigs
func NewDeploymentConfigScaler(oc client.Interface, kc kclient.Interface) kubectl.Scaler {
	return &DeploymentConfigScaler{rcClient: kc, dcClient: oc, clientInterface: kc}
}

// DeploymentConfigScaler is a wrapper for the kubectl Scaler client
type DeploymentConfigScaler struct {
	rcClient kclient.ReplicationControllersNamespacer
	dcClient client.DeploymentConfigsNamespacer

	clientInterface kclient.Interface
}

// Scale updates a replication controller created by the DeploymentConfig with the provided namespace/name,
// to a new size, with optional precondition check (if preconditions is not nil),optional retries (if retry
//  is not nil), and then optionally waits for its replica count to reach the new value (if wait is not nil).
func (scaler *DeploymentConfigScaler) Scale(namespace, name string, newSize uint, preconditions *kubectl.ScalePrecondition, retry, waitForReplicas *kubectl.RetryParams) error {
	if preconditions == nil {
		preconditions = &kubectl.ScalePrecondition{Size: -1, ResourceVersion: ""}
	}
	if retry == nil {
		// Make it try only once, immediately
		retry = &kubectl.RetryParams{Interval: time.Millisecond, Timeout: time.Millisecond}
	}
	cond := kubectl.ScaleCondition(scaler, preconditions, namespace, name, newSize)
	if err := wait.Poll(retry.Interval, retry.Timeout, cond); err != nil {
		if scaleErr := err.(kubectl.ControllerScaleError); kerrors.IsNotFound(scaleErr.ActualError) {
			glog.Infof("No deployment found for dc/%s. Scaling the deployment configuration template...", name)
			dc, err := scaler.dcClient.DeploymentConfigs(namespace).Get(name)
			if err != nil {
				return err
			}
			dc.Template.ControllerTemplate.Replicas = int(newSize)

			if _, err := scaler.dcClient.DeploymentConfigs(namespace).Update(dc); err != nil {
				return err
			}
			return nil
		}
		return err
	}
	if waitForReplicas != nil {
		rc, err := scaler.rcClient.ReplicationControllers(namespace).Get(name)
		if err != nil {
			return err
		}
		return wait.Poll(waitForReplicas.Interval, waitForReplicas.Timeout, kclient.ControllerHasDesiredReplicas(scaler.clientInterface, rc))
	}
	return nil
}

// ScaleSimple does a simple one-shot attempt at scaling - not useful on it's own, but
// a necessary building block for Scale
func (scaler *DeploymentConfigScaler) ScaleSimple(namespace, name string, preconditions *kubectl.ScalePrecondition, newSize uint) error {
	dc, err := scaler.dcClient.DeploymentConfigs(namespace).Get(name)
	if err != nil {
		return err
	}
	controller, err := scaler.rcClient.ReplicationControllers(namespace).Get(util.LatestDeploymentNameForConfig(dc))
	if err != nil {
		return kubectl.ControllerScaleError{FailureType: kubectl.ControllerScaleGetFailure, ResourceVersion: "Unknown", ActualError: err}
	}

	if preconditions != nil {
		if err := preconditions.ValidateReplicationController(controller); err != nil {
			return err
		}
	}
	controller.Spec.Replicas = int(newSize)
	// TODO: do retry on 409 errors here?
	if _, err := scaler.rcClient.ReplicationControllers(namespace).Update(controller); err != nil {
		return kubectl.ControllerScaleError{FailureType: kubectl.ControllerScaleUpdateFailure, ResourceVersion: controller.ResourceVersion, ActualError: err}
	}
	// TODO: do a better job of printing objects here.
	return nil
}
