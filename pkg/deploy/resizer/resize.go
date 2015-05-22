package resizer

import (
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentConfigResizer is a wrapper for the kubectl Resizer client
type DeploymentConfigResizer struct {
	c kubectl.ResizerClient
}

// ResizerFor returns the appropriate Resizer client depending on the provided
// kind of resource (Replication controllers and deploymentConfigs supported)
func ResizerFor(kind string, osc client.Interface, kc kclient.Interface) (kubectl.Resizer, error) {
	if kind != "DeploymentConfig" {
		return kubectl.ResizerFor(kind, kubectl.NewResizerClient(kc))

	}
	return &DeploymentConfigResizer{NewResizerClient(osc, kc)}, nil
}

// Resize updates a replication controller created by the DeploymentConfig with the provided namespace/name,
// to a new size, with optional precondition check (if preconditions is not nil),optional retries (if retry
//  is not nil), and then optionally waits for it's replica count to reach the new value (if wait is not nil).
func (resizer *DeploymentConfigResizer) Resize(namespace, name string, newSize uint, preconditions *kubectl.ResizePrecondition, retry, waitForReplicas *kubectl.RetryParams) error {
	if preconditions == nil {
		preconditions = &kubectl.ResizePrecondition{-1, ""}
	}
	if retry == nil {
		// Make it try only once, immediately
		retry = &kubectl.RetryParams{Interval: time.Millisecond, Timeout: time.Millisecond}
	}
	cond := kubectl.ResizeCondition(resizer, preconditions, namespace, name, newSize)
	if err := wait.Poll(retry.Interval, retry.Timeout, cond); err != nil {
		return err
	}
	if waitForReplicas != nil {
		rc := &kapi.ReplicationController{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: rcName}}
		return wait.Poll(waitForReplicas.Interval, waitForReplicas.Timeout,
			resizer.c.ControllerHasDesiredReplicas(rc))
	}
	return nil
}

// ResizeSimple does a simple one-shot attempt at resizing - not useful on it's own, but
// a necessary building block for Resize
func (resizer *DeploymentConfigResizer) ResizeSimple(namespace, name string, preconditions *kubectl.ResizePrecondition, newSize uint) (string, error) {
	const resized = "resized"
	controller, err := resizer.c.GetReplicationController(namespace, name)
	if err != nil {
		return "", kubectl.ControllerResizeError{kubectl.ControllerResizeGetFailure, "Unknown", err}
	}
	if preconditions != nil {
		if err := preconditions.Validate(controller); err != nil {
			return "", err
		}
	}
	controller.Spec.Replicas = int(newSize)
	// TODO: do retry on 409 errors here?
	if _, err := resizer.c.UpdateReplicationController(namespace, controller); err != nil {
		return "", kubectl.ControllerResizeError{kubectl.ControllerResizeUpdateFailure, controller.ResourceVersion, err}
	}
	// TODO: do a better job of printing objects here.
	return resized, nil
}

// NewResizerClient returns a new Resizer client bundling both the OpenShift and
// Kubernetes clients
func NewResizerClient(osc client.Interface, kc kclient.Interface) kubectl.ResizerClient {
	return &realResizerClient{osc: osc, kc: kc}
}

// realResizerClient is a ResizerClient which uses an OpenShift and a Kube client.
type realResizerClient struct {
	osc client.Interface
	kc  kclient.Interface
}

var rcName string

// GetReplicationController returns the most recent replication controller associated with the deploymentConfig
// with the provided namespace/name combination
func (c *realResizerClient) GetReplicationController(namespace, name string) (*kapi.ReplicationController, error) {
	dc, err := c.osc.DeploymentConfigs(namespace).Get(name)
	if err != nil {
		return nil, err
	}
	rcName = util.LatestDeploymentNameForConfig(dc)
	return c.kc.ReplicationControllers(namespace).Get(rcName)
}

// UpdateReplicationController updates the provided replication controller
func (c *realResizerClient) UpdateReplicationController(namespace string, rc *kapi.ReplicationController) (*kapi.ReplicationController, error) {
	return c.kc.ReplicationControllers(namespace).Update(rc)
}

// ControllerHasDesiredReplicas checks whether the provided replication controller has the desired replicas
// number set
func (c *realResizerClient) ControllerHasDesiredReplicas(rc *kapi.ReplicationController) wait.ConditionFunc {
	return kclient.ControllerHasDesiredReplicas(c.kc, rc)
}
