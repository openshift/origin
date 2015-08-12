package strategy

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// DeploymentStrategy knows how to make a deployment active.
type DeploymentStrategy interface {
	// Deploy transitions an old deployment to a new one.
	Deploy(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error
}
