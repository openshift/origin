package strategy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// DeploymentStrategy knows how to make a deployment active.
type DeploymentStrategy interface {
	// Deploy makes a deployment active.
	Deploy(deployment *kapi.ReplicationController, oldDeployments []*kapi.ReplicationController) error
}
