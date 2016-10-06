package test

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/util/sets"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	ImageStreamName      = "test-image-stream"
	ImageID              = "0000000000000000000000000000000000000000000000000000000000000001"
	DockerImageReference = "registry:5000/openshift/test-image-stream@sha256:0000000000000000000000000000000000000000000000000000000000000001"
)

func OkDeploymentConfig(version int64) *deployapi.DeploymentConfig {
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

func OkDeploymentConfigStatus(version int64) deployapi.DeploymentConfigStatus {
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

func OkPodTemplateChanged() *kapi.PodTemplateSpec {
	template := OkPodTemplate()
	template.Spec.Containers[0].Image = DockerImageReference
	return template
}

func OkPodTemplateMissingImage(missing ...string) *kapi.PodTemplateSpec {
	set := sets.NewString(missing...)
	template := OkPodTemplate()
	for i, c := range template.Spec.Containers {
		if set.Has(c.Name) {
			// rememeber that slices use copies, so have to ref array entry explicitly
			template.Spec.Containers[i].Image = ""
		}
	}
	return template
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

func OkTriggeredImageChange() deployapi.DeploymentTriggerPolicy {
	ict := OkImageChangeTrigger()
	ict.ImageChangeParams.LastTriggeredImage = DockerImageReference
	return ict
}

func OkNonAutomaticICT() deployapi.DeploymentTriggerPolicy {
	ict := OkImageChangeTrigger()
	ict.ImageChangeParams.Automatic = false
	return ict
}

func OkTriggeredNonAutomatic() deployapi.DeploymentTriggerPolicy {
	ict := OkNonAutomaticICT()
	ict.ImageChangeParams.LastTriggeredImage = DockerImageReference
	return ict
}

func TestDeploymentConfig(config *deployapi.DeploymentConfig) *deployapi.DeploymentConfig {
	config.Spec.Test = true
	return config
}

func OkHPAForDeploymentConfig(config *deployapi.DeploymentConfig, min, max int) *autoscaling.HorizontalPodAutoscaler {
	newMin := int32(min)
	return &autoscaling.HorizontalPodAutoscaler{
		ObjectMeta: kapi.ObjectMeta{Name: config.Name, Namespace: config.Namespace},
		Spec: autoscaling.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscaling.CrossVersionObjectReference{
				Name: config.Name,
				Kind: "DeploymentConfig",
			},
			MinReplicas: &newMin,
			MaxReplicas: int32(max),
		},
	}
}

func OkStreamForConfig(config *deployapi.DeploymentConfig) *imageapi.ImageStream {
	for _, t := range config.Spec.Triggers {
		if t.Type != deployapi.DeploymentTriggerOnImageChange {
			continue
		}

		ref := t.ImageChangeParams.From
		name, tag, _ := imageapi.SplitImageStreamTag(ref.Name)

		return &imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Name:      name,
				Namespace: ref.Namespace,
			},
			Status: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					tag: {
						Items: []imageapi.TagEvent{{DockerImageReference: t.ImageChangeParams.LastTriggeredImage}}}}},
		}
	}
	return nil
}

func RemoveTriggerTypes(config *deployapi.DeploymentConfig, triggerTypes ...deployapi.DeploymentTriggerType) {
	types := sets.NewString()
	for _, triggerType := range triggerTypes {
		types.Insert(string(triggerType))
	}

	remaining := []deployapi.DeploymentTriggerPolicy{}
	for _, trigger := range config.Spec.Triggers {
		if types.Has(string(trigger.Type)) {
			continue
		}
		remaining = append(remaining, trigger)
	}

	config.Spec.Triggers = remaining
}

func RoundTripConfig(t *testing.T, config *deployapi.DeploymentConfig) *deployapi.DeploymentConfig {
	versioned, err := kapi.Scheme.ConvertToVersion(config, deployv1.SchemeGroupVersion)
	if err != nil {
		t.Errorf("unexpected conversion error: %v", err)
		return nil
	}
	defaulted, err := kapi.Scheme.ConvertToVersion(versioned, deployapi.SchemeGroupVersion)
	if err != nil {
		t.Errorf("unexpected conversion error: %v", err)
		return nil
	}
	return defaulted.(*deployapi.DeploymentConfig)
}
