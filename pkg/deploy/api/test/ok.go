package test

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/deploy/api"
)

func OkStrategy() api.DeploymentStrategy {
	return api.DeploymentStrategy{
		Type: api.DeploymentStrategyTypeRecreate,
	}
}

func OkCustomStrategy() api.DeploymentStrategy {
	return api.DeploymentStrategy{
		Type:         api.DeploymentStrategyTypeCustom,
		CustomParams: OkCustomParams(),
	}
}

func OkCustomParams() *api.CustomDeploymentStrategyParams {
	return &api.CustomDeploymentStrategyParams{
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
