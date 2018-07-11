package support

import (
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
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

	watcher, err := c.kclient.ReplicationControllers(rc.Namespace).Watch(metav1.SingleObject(metav1.ObjectMeta{Name: rc.Name, ResourceVersion: rc.ResourceVersion}))
	if err != nil {
		return fmt.Errorf("acceptAvailablePods failed to watch ReplicationController %s/%s: %v", rc.Namespace, rc.Name, err)
	}

	_, err = watch.Until(c.timeout, watcher, func(event watch.Event) (bool, error) {
		if t := event.Type; t != watch.Modified {
			return false, fmt.Errorf("acceptAvailablePods failed watching for ReplicationController %s/%s: received event %v", rc.Namespace, rc.Name, t)
		}
		newRc, ok := event.Object.(*corev1.ReplicationController)
		if !ok {
			return false, fmt.Errorf("unknown event object %#v", event.Object)
		}
		return allReplicasAvailable(newRc), nil
	})
	// Handle acceptance failure.
	if err != nil {
		if err == wait.ErrWaitTimeout {
			return fmt.Errorf("pods for rc '%s/%s' took longer than %.f seconds to become available", rc.Namespace, rc.Name, c.timeout.Seconds())
		}
		return err
	}
	return nil
}
