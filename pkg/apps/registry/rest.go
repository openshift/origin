package registry

import (
	"errors"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	"github.com/golang/glog"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

var (
	// ErrUnknownDeploymentPhase is returned for WaitForRunningDeployment if an unknown phase is returned.
	ErrUnknownDeploymentPhase = errors.New("unknown deployment phase")
	ErrTooOldResourceVersion  = errors.New("too old resource version")
)

// WaitForRunningDeployment waits until the specified deployment is no longer New or Pending. Returns true if
// the deployment became running, complete, or failed within timeout, false if it did not, and an error if any
// other error state occurred. The last observed deployment state is returned.
func WaitForRunningDeployment(rn kcoreclient.ReplicationControllersGetter, observed *kapi.ReplicationController, timeout time.Duration) (*kapi.ReplicationController, bool, error) {
	options := metav1.SingleObject(observed.ObjectMeta)
	w, err := rn.ReplicationControllers(observed.Namespace).Watch(options)
	if err != nil {
		return observed, false, err
	}
	defer w.Stop()

	if _, err := watch.Until(timeout, w, func(e watch.Event) (bool, error) {
		if e.Type == watch.Error {
			// When we send too old resource version in observed replication controller to
			// watcher, restart the watch with latest available controller.
			switch t := e.Object.(type) {
			case *metav1.Status:
				if t.Reason == metav1.StatusReasonGone {
					glog.V(5).Infof("encountered error while watching for replication controller: %v (retrying)", t)
					return false, ErrTooOldResourceVersion
				}
			}
			return false, fmt.Errorf("encountered error while watching for replication controller: %v", e.Object)
		}
		obj, isController := e.Object.(*kapi.ReplicationController)
		if !isController {
			return false, fmt.Errorf("received unknown object while watching for deployments: %v", obj)
		}
		observed = obj
		switch appsutil.DeploymentStatusFor(observed) {
		case appsapi.DeploymentStatusRunning, appsapi.DeploymentStatusFailed, appsapi.DeploymentStatusComplete:
			return true, nil
		case appsapi.DeploymentStatusNew, appsapi.DeploymentStatusPending:
			return false, nil
		default:
			return false, ErrUnknownDeploymentPhase
		}
	}); err != nil {
		if err == ErrTooOldResourceVersion {
			latestRC, err := rn.ReplicationControllers(observed.Namespace).Get(observed.Name, metav1.GetOptions{})
			if err != nil {
				return observed, false, err
			}
			return WaitForRunningDeployment(rn, latestRC, timeout)
		}
		return observed, false, err
	}

	return observed, true, nil
}
