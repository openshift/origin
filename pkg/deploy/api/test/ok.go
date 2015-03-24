package test

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

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
					Name:  "container1",
					Image: "registry:8080/repo1:ref1",
					Env: []kapi.EnvVar{
						{
							Name:  "ENV1",
							Value: "VAL1",
						},
					},
					ImagePullPolicy: kapi.PullIfNotPresent,
				},
				{
					Name:            "container2",
					Image:           "registry:8080/repo1:ref2",
					ImagePullPolicy: kapi.PullIfNotPresent,
				},
			},
			RestartPolicy: kapi.RestartPolicyAlways,
			DNSPolicy:     kapi.DNSClusterFirst,
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
			From: kapi.ObjectReference{
				Kind: "ImageRepository",
				Name: "test-image-repo",
			},
			Tag: "latest",
		},
	}
}

func OkImageChangeTriggerDeprecated() deployapi.DeploymentTriggerPolicy {
	return deployapi.DeploymentTriggerPolicy{
		Type: deployapi.DeploymentTriggerOnImageChange,
		ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
			Automatic: true,
			ContainerNames: []string{
				"container1",
			},
			RepositoryName: "registry:8080/repo1:ref1",
			Tag:            "latest",
		},
	}
}

func OkDeploymentConfig(version int) *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "config",
		},
		LatestVersion: version,
		Triggers: []deployapi.DeploymentTriggerPolicy{
			OkImageChangeTrigger(),
		},
		Template: OkDeploymentTemplate(),
	}
}
