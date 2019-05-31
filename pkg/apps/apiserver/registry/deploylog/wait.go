package deploylog

import (
	"context"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"

	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/library-go/pkg/apps/appsutil"
)

var (
	// ErrUnknownDeploymentPhase is returned for WaitForRunningDeployment if an unknown phase is returned.
	ErrUnknownDeploymentPhase = errors.New("unknown deployment phase")
)

// WaitForRunningDeployment waits until the specified deployment is no longer New or Pending. Returns true if
// the deployment became running, complete, or failed within timeout, false if it did not, and an error if any
// other error state occurred. The last observed deployment state is returned.
func WaitForRunningDeployment(rn corev1client.ReplicationControllersGetter, observed *corev1.ReplicationController, timeout time.Duration) (*corev1.ReplicationController, error) {
	fieldSelector := fields.OneTermEqualSelector("metadata.name", observed.Name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return rn.ReplicationControllers(observed.Namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return rn.ReplicationControllers(observed.Namespace).Watch(options)
		},
	}

	preconditionFunc := func(store cache.Store) (bool, error) {
		item, exists, err := store.Get(&metav1.ObjectMeta{Namespace: observed.Namespace, Name: observed.Name})
		if err != nil {
			return true, err
		}
		if !exists {
			// We need to make sure we see the object in the cache before we start waiting for events
			// or we would be waiting for the timeout if such object didn't exist.
			return true, fmt.Errorf("%s '%s/%s' not found", corev1.Resource("replicationcontrollers"), observed.Namespace, observed.Name)
		}

		// Check that the objects UID match for cases of recreation
		storeRc, ok := item.(*corev1.ReplicationController)
		if !ok {
			return true, fmt.Errorf("unexpected store item type: %#v", item)
		}
		if observed.UID != storeRc.UID {
			return true, fmt.Errorf("%s '%s/%s' no longer exists, expected UID %q, got UID %q", corev1.Resource("replicationcontrollers"), observed.Namespace, observed.Name, observed.UID, storeRc.UID)
		}

		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	event, err := watchtools.UntilWithSync(ctx, lw, &corev1.ReplicationController{}, preconditionFunc, func(e watch.Event) (bool, error) {
		switch e.Type {
		case watch.Added, watch.Modified:
			newRc, ok := e.Object.(*corev1.ReplicationController)
			if !ok {
				return true, fmt.Errorf("unknown event object %#v", e.Object)
			}

			switch appsutil.DeploymentStatusFor(newRc) {
			case appsv1.DeploymentStatusRunning, appsv1.DeploymentStatusFailed, appsv1.DeploymentStatusComplete:
				return true, nil

			case appsv1.DeploymentStatusNew, appsv1.DeploymentStatusPending:
				return false, nil

			default:
				return true, ErrUnknownDeploymentPhase
			}

		case watch.Deleted:
			return true, fmt.Errorf("replicationController got deleted %#v", e.Object)

		case watch.Error:
			return true, fmt.Errorf("unexpected error %#v", e.Object)

		default:
			return true, fmt.Errorf("unexpected event type: %T", e.Type)
		}
	})
	if err != nil {
		return nil, err
	}

	return event.Object.(*corev1.ReplicationController), nil
}
