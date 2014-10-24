package test

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/deploy/api"
)

func OkStrategy() api.DeploymentStrategy {
	return api.DeploymentStrategy{
		Type:      api.DeploymentStrategyTypeCustomPod,
		CustomPod: OkCustomPod(),
	}
}

func OkCustomPod() *api.CustomPodDeploymentStrategy {
	return &api.CustomPodDeploymentStrategy{
		Image: "openshift/kube-deploy",
	}
}

func OkControllerTemplate() kapi.ReplicationControllerState {
	return kapi.ReplicationControllerState{
		ReplicaSelector: OkSelector(),
		PodTemplate:     OkPodTemplate(),
	}
}

func OkSelector() map[string]string {
	return map[string]string{"a": "b"}
}

func OkPodTemplate() kapi.PodTemplate {
	return kapi.PodTemplate{
		DesiredState: kapi.PodState{
			Manifest: kapi.ContainerManifest{
				Version: "v1beta1",
			},
		},
		Labels: OkSelector(),
	}
}
