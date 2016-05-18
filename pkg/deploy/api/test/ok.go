package test

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/apis/extensions"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	ImageStreamName      = "test-image-stream"
	ImageID              = "00000000000000000000000000000001"
	DockerImageReference = "registry:5000/openshift/test-image-stream@sha256:00000000000000000000000000000001"
)

func OkDeploymentConfig(version int) *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "config",
		},
		Spec:   OkDeploymentConfigSpec(),
		Status: OkDeploymentConfigStatus(version),
	}
}

func OkDeploymentConfigSpec() deployapi.DeploymentConfigSpec {
	return deployapi.DeploymentConfigSpec{
		Replicas: 1,
		Selector: OkSelector(),
		Strategy: OkStrategy(),
		Template: OkPodTemplate(),
		Triggers: []deployapi.DeploymentTriggerPolicy{
			OkImageChangeTrigger(),
			OkConfigChangeTrigger(),
		},
	}
}

func OkDeploymentConfigStatus(version int) deployapi.DeploymentConfigStatus {
	return deployapi.DeploymentConfigStatus{
		LatestVersion: version,
	}
}

func OkImageChangeDetails() *deployapi.DeploymentDetails {
	return &deployapi.DeploymentDetails{
		Causes: []deployapi.DeploymentCause{{
			Type: deployapi.DeploymentTriggerOnImageChange,
			ImageTrigger: &deployapi.DeploymentCauseImageTrigger{
				From: kapi.ObjectReference{
					Name: imageapi.JoinImageStreamTag(ImageStreamName, imageapi.DefaultImageTag),
					Kind: "ImageStreamTag",
				}}}}}
}

func OkConfigChangeDetails() *deployapi.DeploymentDetails {
	return &deployapi.DeploymentDetails{
		Causes: []deployapi.DeploymentCause{{
			Type: deployapi.DeploymentTriggerOnConfigChange,
		}}}
}

func OkStrategy() deployapi.DeploymentStrategy {
	return deployapi.DeploymentStrategy{
		Type: deployapi.DeploymentStrategyTypeRecreate,
		Resources: kapi.ResourceRequirements{
			Limits: kapi.ResourceList{
				kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
				kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
			},
		},
		RecreateParams: &deployapi.RecreateDeploymentStrategyParams{
			TimeoutSeconds: mkintp(20),
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

func mkintp(i int) *int64 {
	v := int64(i)
	return &v
}

func OkRollingStrategy() deployapi.DeploymentStrategy {
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
				Kind: "ImageStreamTag",
				Name: imageapi.JoinImageStreamTag(ImageStreamName, imageapi.DefaultImageTag),
			},
		},
	}
}

func TestDeploymentConfig(config *deployapi.DeploymentConfig) *deployapi.DeploymentConfig {
	config.Spec.Test = true
	return config
}

func OkHPAForDeploymentConfig(config *deployapi.DeploymentConfig, min, max int) *extensions.HorizontalPodAutoscaler {
	return &extensions.HorizontalPodAutoscaler{
		ObjectMeta: kapi.ObjectMeta{Name: config.Name, Namespace: config.Namespace},
		Spec: extensions.HorizontalPodAutoscalerSpec{
			ScaleRef: extensions.SubresourceReference{
				Name: config.Name,
				Kind: "DeploymentConfig",
			},
			MinReplicas: &min,
			MaxReplicas: max,
		},
	}
}
