package support

import (
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

// NewAcceptAvailablePods makes a new acceptAvailablePods from a real client.
func NewAcceptAvailablePods(
	out io.Writer,
	kclient corev1client.ReplicationControllersGetter,
	timeout time.Duration,
) *acceptAvailablePods {
	return &acceptAvailablePods{
		out:     out,
		kclient: kclient,
		timeout: timeout,
	}
}

// acceptAvailablePods will accept a replication controller if all the pods
// for the replication controller become available.
type acceptAvailablePods struct {
	out     io.Writer
	kclient corev1client.ReplicationControllersGetter
	// timeout is how long to wait for pods to become available from ready state.
	timeout time.Duration
}

// Accept all pods for a replication controller once they are available.
func (c *acceptAvailablePods) Accept(rc *corev1.ReplicationController) error {
	allReplicasAvailable := func(r *corev1.ReplicationController) bool {
		return r.Status.AvailableReplicas == *r.Spec.Replicas
	}

	if allReplicasAvailable(rc) {
		return nil
	}

	fieldSelector := fields.OneTermEqualSelector("metadata.name", rc.Name).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return c.kclient.ReplicationControllers(rc.Namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return c.kclient.ReplicationControllers(rc.Namespace).Watch(options)
		},
	}

	preconditionFunc := func(store cache.Store) (bool, error) {
		item, exists, err := store.Get(&metav1.ObjectMeta{Namespace: rc.Namespace, Name: rc.Name})
		if err != nil {
			return true, err
		}
		if !exists {
			// We need to make sure we see the object in the cache before we start waiting for events
			// or we would be waiting for the timeout if such object didn't exist.
			return true, fmt.Errorf("%s '%s/%s' not found", corev1.Resource("replicationcontrollers"), rc.Namespace, rc.Name)
		}

		// Check that the objects UID match for cases of recreation
		storeRc, ok := item.(*corev1.ReplicationController)
		if !ok {
			return true, fmt.Errorf("unexpected store item type: %#v", item)
		}
		if rc.UID != storeRc.UID {
			return true, fmt.Errorf("%s '%s/%s' no longer exists, expected UID %q, got UID %q", corev1.Resource("replicationcontrollers"), rc.Namespace, rc.Name, rc.UID, storeRc.UID)
		}

		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()
	_, err := watchtools.UntilWithSync(ctx, lw, &corev1.ReplicationController{}, preconditionFunc, func(event watch.Event) (bool, error) {
		switch event.Type {
		case watch.Added, watch.Modified:
			newRc, ok := event.Object.(*corev1.ReplicationController)
			if !ok {
				return true, fmt.Errorf("unknown event object %#v", event.Object)
			}
			return allReplicasAvailable(newRc), nil

		case watch.Deleted:
			return true, fmt.Errorf("replicationController got deleted %#v", event.Object)

		case watch.Error:
			return true, fmt.Errorf("unexpected error %#v", event.Object)

		default:
			return true, fmt.Errorf("unexpected event type: %T", event.Type)
		}
	})
	// Handle acceptance failure.
	if err == wait.ErrWaitTimeout {
		return fmt.Errorf("pods for rc '%s/%s' took longer than %.f seconds to become available", rc.Namespace, rc.Name, c.timeout.Seconds())
	}

	if err != nil {
		return err
	}

	return nil
}
