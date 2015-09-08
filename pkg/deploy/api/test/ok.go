package test

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func OkStrategy() deployapi.DeploymentStrategy {
	return deployapi.DeploymentStrategy{
		Type: deployapi.DeploymentStrategyTypeRecreate,
		Resources: kapi.ResourceRequirements{
			Limits: kapi.ResourceList{
				kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
				kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
			},
		},
	}
}

func OkCustomStrategy() deployapi.DeploymentStrategy {
	return deployapi.DeploymentStrategy{
		Type:         deployapi.DeploymentStrategyTypeCustom,
		CustomParams: OkCustomParams(),
		Resources: kapi.ResourceRequirements{
			Limits: kapi.ResourceList{
				kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
				kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
			},
		},
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

func OkRollingStrategy() deployapi.DeploymentStrategy {
	mkintp := func(i int) *int64 {
		v := int64(i)
		return &v
	}
	return deployapi.DeploymentStrategy{
		Type: deployapi.DeploymentStrategyTypeRolling,
		RollingParams: &deployapi.RollingDeploymentStrategyParams{
			UpdatePeriodSeconds: mkintp(1),
			IntervalSeconds:     mkintp(1),
			TimeoutSeconds:      mkintp(20),
		},
		Resources: kapi.ResourceRequirements{
			Limits: kapi.ResourceList{
				kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
				kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
			},
		},
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
				Kind: "ImageStream",
				Name: "test-image-stream",
			},
			Tag: imageapi.DefaultImageTag,
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
			Tag:            imageapi.DefaultImageTag,
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
