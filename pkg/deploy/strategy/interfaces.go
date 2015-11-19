package strategy

import (
	kapi "k8s.io/kubernetes/pkg/api"
)

// DeploymentStrategy knows how to make a deployment active.
type DeploymentStrategy interface {
	// Deploy transitions an old deployment to a new one.
	Deploy(from *kapi.ReplicationController, to *kapi.ReplicationController, desiredReplicas int) error
}

// UpdateAcceptor is given a chance to accept or reject the new controller
// during a deployment each time the controller is scaled up.
//
// After the successful scale-up of the controller, the controller is given to
// the UpdateAcceptor. If the UpdateAcceptor rejects the controller, the
// deployment is stopped with an error.
//
// DEPRECATED: Acceptance checking has been incorporated into the rolling
// strategy, but we still need this around to support Recreate.
type UpdateAcceptor interface {
	// Accept returns nil if the controller is okay, otherwise returns an error.
	Accept(*kapi.ReplicationController) error
}
