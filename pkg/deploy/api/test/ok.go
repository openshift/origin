package test

import (
	"speter.net/go/exp/math/dec/inf"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/resource"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func OkStrategy() deployapi.DeploymentStrategy {
	return deployapi.DeploymentStrategy{
		Type: deployapi.DeploymentStrategyTypeRecreate,
	}
}

func OkCustomStrategy() deployapi.DeploymentStrategy {
	return deployapi.DeploymentStrategy{
		Type:         deployapi.DeploymentStrategyTypeCustom,
		CustomParams: OkCustomParams(),
	}
}

func OkCustomParams() *deployapi.CustomDeploymentStrategyParams {
	return &deployapi.CustomDeploymentStrategyParams{
		Image: "openshift/origin-deployer",
		Environment: []kapi.EnvVar{
			{
				Name:  "ENV1",
				Value: "VAL1",
			},
		},
		Command: []string{"/bin/echo", "hello", "world"},
	}
}

func OkControllerTemplate() kapi.ReplicationControllerSpec {
	return kapi.ReplicationControllerSpec{
		Replicas: 1,
		Selector: OkSelector(),
		Template: OkPodTemplate(),
	}
}

func OkDeploymentTemplate() deployapi.DeploymentTemplate {
	return deployapi.DeploymentTemplate{
		Strategy:           OkStrategy(),
		ControllerTemplate: OkControllerTemplate(),
	}
}

func OkSelector() map[string]string {
	return map[string]string{"a": "b"}
}

func OkPodTemplate() *kapi.PodTemplateSpec {
	return &kapi.PodTemplateSpec{
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:   "container1",
					Image:  "registry:8080/repo1:ref1",
					CPU:    resource.Quantity{Amount: inf.NewDec(0, 3), Format: "DecimalSI"},
					Memory: resource.Quantity{Amount: inf.NewDec(0, 0), Format: "DecimalSI"},
					Env: []kapi.EnvVar{
						{
							Name:  "ENV1",
							Value: "VAL1",
						},
					},
				},
				{
					Name:   "container2",
					Image:  "registry:8080/repo1:ref2",
					CPU:    resource.Quantity{Amount: inf.NewDec(0, 3), Format: "DecimalSI"},
					Memory: resource.Quantity{Amount: inf.NewDec(0, 0), Format: "DecimalSI"},
				},
			},
		},
		ObjectMeta: kapi.ObjectMeta{
			Labels: OkSelector(),
		},
	}
}

func OkConfigChangeTrigger() deployapi.DeploymentTriggerPolicy {
	return deployapi.DeploymentTriggerPolicy{
		Type: deployapi.DeploymentTriggerOnConfigChange,
	}
}

func OkImageChangeTrigger() deployapi.DeploymentTriggerPolicy {
	return deployapi.DeploymentTriggerPolicy{
		Type: deployapi.DeploymentTriggerOnImageChange,
		ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
			Automatic: true,
			ContainerNames: []string{
				"container1",
			},
			RepositoryName: "registry:8080/repo1",
			Tag:            "tag1",
		},
	}
}

func OkImageChangeTriggerNew() deployapi.DeploymentTriggerPolicy {
	return deployapi.DeploymentTriggerPolicy{
		Type: deployapi.DeploymentTriggerOnImageChange,
		ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
			Automatic: true,
			ContainerNames: []string{
				"container1",
			},
			From: kapi.ObjectReference{
				Namespace: kapi.NamespaceDefault,
				Name:      "imageRepo",
			},
			Tag: "tag1",
		},
	}
}

func OkDeploymentConfig(version int) *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: kapi.NamespaceDefault,
			Name:      "config",
		},
		LatestVersion: version,
		Triggers: []deployapi.DeploymentTriggerPolicy{
			OkImageChangeTrigger(),
		},
		Template: OkDeploymentTemplate(),
	}
}
