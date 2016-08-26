package registry

import (
	"errors"
	"fmt"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/watch"

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
	options := kapi.ListOptions{FieldSelector: fieldSelector, ResourceVersion: observed.ResourceVersion}
	w, err := rn.ReplicationControllers(observed.Namespace).Watch(options)
	if err != nil {
		return observed, false, err
	}
	defer w.Stop()

	if _, err := watch.Until(timeout, w, func(e watch.Event) (bool, error) {
		if e.Type == watch.Error {
			return false, fmt.Errorf("encountered error while watching for replication controller: %v", e.Object)
		}
		obj, isController := e.Object.(*kapi.ReplicationController)
		if !isController {
			return false, fmt.Errorf("received unknown object while watching for deployments: %v", obj)
		}
		observed = obj
		switch deployutil.DeploymentStatusFor(observed) {
		case api.DeploymentStatusRunning, api.DeploymentStatusFailed, api.DeploymentStatusComplete:
			return true, nil
		case api.DeploymentStatusNew, api.DeploymentStatusPending:
			return false, nil
		default:
			return false, ErrUnknownDeploymentPhase
		}
	}); err != nil {
		return observed, false, err
	}

	return observed, true, nil
}
