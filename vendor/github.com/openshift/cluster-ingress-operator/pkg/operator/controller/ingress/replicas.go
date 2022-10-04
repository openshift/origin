package ingress

import (
	configv1 "github.com/openshift/api/config/v1"
)

// DetermineReplicas implements the replicas choice algorithm as described in
// the documentation for the IngressController replicas parameter. Used both in
// determining the number of replicas for the default IngressController and in
// determining the number of replicas in the Deployments corresponding to
// IngressController resources in which the number of replicas is unset
func DetermineReplicas(ingressConfig *configv1.Ingress, infraConfig *configv1.Infrastructure) int32 {
	// DefaultPlacement affects which topology field we're interested in
	topology := infraConfig.Status.InfrastructureTopology
	if ingressConfig.Status.DefaultPlacement == configv1.DefaultPlacementControlPlane {
		topology = infraConfig.Status.ControlPlaneTopology
	}

	if topology == configv1.SingleReplicaTopologyMode {
		return 1
	}

	// TODO: Set the replicas value to the number of workers.
	return 2
}
