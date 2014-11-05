package test

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/deploy/api"
)

func OkStrategy() api.DeploymentStrategy {
	return api.DeploymentStrategy{
		Type: api.DeploymentStrategyTypeBasic,
	}
}

func OkCustomPodStrategy() api.DeploymentStrategy {
	return api.DeploymentStrategy{
		Type:      api.DeploymentStrategyTypeCustomPod,
		CustomPod: OkCustomPod(),
	}
}

func OkCustomPod() *api.CustomPodDeploymentStrategy {
	return &api.CustomPodDeploymentStrategy{
		Image: "openshift/origin-deployer",
	}
}

func OkControllerTemplate() kapi.ReplicationControllerState {
	return kapi.ReplicationControllerState{
		ReplicaSelector: OkSelector(),
		PodTemplate:     OkPodTemplate(),
	}
}

func OkDeploymentTemplate() api.DeploymentTemplate {
	return api.DeploymentTemplate{
		Strategy:           OkStrategy(),
		ControllerTemplate: OkControllerTemplate(),
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
