package registry

import (
	"errors"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

var (
	// ErrUnknownDeploymentPhase is returned for WaitForRunningDeployment if an unknown phase is returned.
	ErrUnknownDeploymentPhase = errors.New("unknown deployment phase")
)

// WaitForRunningDeployment waits until the specified deployment is no longer New or Pending. Returns true if
// the deployment became running, complete, or failed within timeout, false if it did not, and an error if any
// other error state occurred. The last observed deployment state is returned.
func WaitForRunningDeployment(rn kclient.ReplicationControllersNamespacer, observed *kapi.ReplicationController, timeout time.Duration) (*kapi.ReplicationController, bool, error) {
	fieldSelector := fields.Set{"metadata.name": observed.Name}.AsSelector()
	w, err := rn.ReplicationControllers(observed.Namespace).Watch(labels.Everything(), fieldSelector, observed.ResourceVersion)
	if err != nil {
		return observed, false, err
	}
	defer w.Stop()

	ch := w.ResultChan()
	// Passing time.After like this (vs receiving directly in a select) will trigger the channel
	// and the timeout will have full effect here.
	expire := time.After(timeout)
	for {
		select {
		case event := <-ch:
			obj, ok := event.Object.(*kapi.ReplicationController)
			if !ok {
				return observed, false, errors.New("received unknown object while watching for deployments")
			}
			observed = obj

			switch deployutil.DeploymentStatusFor(observed) {
			case api.DeploymentStatusRunning, api.DeploymentStatusFailed, api.DeploymentStatusComplete:
				return observed, true, nil
			case api.DeploymentStatusNew, api.DeploymentStatusPending:
			default:
				return observed, false, ErrUnknownDeploymentPhase
			}
		case <-expire:
			return observed, false, nil
		}
	}
}
