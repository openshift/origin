package test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	appsv1 "github.com/openshift/api/apps/v1"
)

const (
	ImageStreamName      = "test-image-stream"
	ImageID              = "0000000000000000000000000000000000000000000000000000000000000001"
	DockerImageReference = "registry:5000/openshift/test-image-stream@sha256:0000000000000000000000000000000000000000000000000000000000000001"
)

func OkDeploymentConfig(version int64) *appsv1.DeploymentConfig {
	return &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: corev1.NamespaceDefault,
			SelfLink:  "/apis/apps.openshift.io/v1/deploymentConfig/config",
		},
		Spec:   OkDeploymentConfigSpec(),
		Status: OkDeploymentConfigStatus(version),
	}
}

func OkDeploymentConfigSpec() appsv1.DeploymentConfigSpec {
	return appsv1.DeploymentConfigSpec{
		Replicas: 1,
		Selector: OkSelector(),
		Strategy: OkStrategy(),
		Template: OkPodTemplate(),
		Triggers: []appsv1.DeploymentTriggerPolicy{
			OkImageChangeTrigger(),
			OkConfigChangeTrigger(),
		},
	}
}

func OkDeploymentConfigStatus(version int64) appsv1.DeploymentConfigStatus {
	return appsv1.DeploymentConfigStatus{
		LatestVersion: version,
	}
}

func OkImageChangeDetails() *appsv1.DeploymentDetails {
	return &appsv1.DeploymentDetails{
		Causes: []appsv1.DeploymentCause{{
			Type: appsv1.DeploymentTriggerOnImageChange,
			ImageTrigger: &appsv1.DeploymentCauseImageTrigger{
				From: corev1.ObjectReference{
					Name: ImageStreamName + ":latest",
					Kind: "ImageStreamTag",
				}}}}}
}

func OkConfigChangeDetails() *appsv1.DeploymentDetails {
	return &appsv1.DeploymentDetails{
		Causes: []appsv1.DeploymentCause{{
			Type: appsv1.DeploymentTriggerOnConfigChange,
		}}}
}

func OkStrategy() appsv1.DeploymentStrategy {
	return appsv1.DeploymentStrategy{
		Type: appsv1.DeploymentStrategyTypeRecreate,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("10"),
				corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("10G"),
			},
		},
		RecreateParams: &appsv1.RecreateDeploymentStrategyParams{
			TimeoutSeconds: mkintp(20),
		},
		ActiveDeadlineSeconds: mkintp(21600),
	}
}

func OkCustomStrategy() appsv1.DeploymentStrategy {
	return appsv1.DeploymentStrategy{
		Type:         appsv1.DeploymentStrategyTypeCustom,
		CustomParams: OkCustomParams(),
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("10"),
				corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("10G"),
			},
		},
	}
}

func OkCustomParams() *appsv1.CustomDeploymentStrategyParams {
	return &appsv1.CustomDeploymentStrategyParams{
		Image: "openshift/origin-deployer",
		Environment: []corev1.EnvVar{
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

func OkRollingStrategy() appsv1.DeploymentStrategy {
	return appsv1.DeploymentStrategy{
		Type: appsv1.DeploymentStrategyTypeRolling,
		RollingParams: &appsv1.RollingDeploymentStrategyParams{
			UpdatePeriodSeconds: mkintp(1),
			IntervalSeconds:     mkintp(1),
			TimeoutSeconds:      mkintp(20),
		},
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceName(corev1.ResourceCPU):    resource.MustParse("10"),
				corev1.ResourceName(corev1.ResourceMemory): resource.MustParse("10G"),
			},
		},
	}
}

func OkSelector() map[string]string {
	return map[string]string{"a": "b"}
}

func OkPodTemplate() *corev1.PodTemplateSpec {
	one := int64(1)
	return &corev1.PodTemplateSpec{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "container1",
					Image: "registry:8080/repo1:ref1",
					Env: []corev1.EnvVar{
						{
							Name:  "ENV1",
							Value: "VAL1",
						},
					},
					ImagePullPolicy:          corev1.PullIfNotPresent,
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: corev1.TerminationMessageReadFile,
				},
				{
					Name:                     "container2",
					Image:                    "registry:8080/repo1:ref2",
					ImagePullPolicy:          corev1.PullIfNotPresent,
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: corev1.TerminationMessageReadFile,
				},
			},
			RestartPolicy:                 corev1.RestartPolicyAlways,
			DNSPolicy:                     corev1.DNSClusterFirst,
			TerminationGracePeriodSeconds: &one,
			SchedulerName:                 corev1.DefaultSchedulerName,
			SecurityContext:               &corev1.PodSecurityContext{},
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: OkSelector(),
		},
	}
}

func OkPodTemplateChanged() *corev1.PodTemplateSpec {
	template := OkPodTemplate()
	template.Spec.Containers[0].Image = DockerImageReference
	return template
}

func OkPodTemplateMissingImage(missing ...string) *corev1.PodTemplateSpec {
	set := sets.NewString(missing...)
	template := OkPodTemplate()
	for i, c := range template.Spec.Containers {
		if set.Has(c.Name) {
			// remember that slices use copies, so have to ref array entry explicitly
			template.Spec.Containers[i].Image = ""
		}
	}
	return template
}

func OkConfigChangeTrigger() appsv1.DeploymentTriggerPolicy {
	return appsv1.DeploymentTriggerPolicy{
		Type: appsv1.DeploymentTriggerOnConfigChange,
	}
}

func OkImageChangeTrigger() appsv1.DeploymentTriggerPolicy {
	return appsv1.DeploymentTriggerPolicy{
		Type: appsv1.DeploymentTriggerOnImageChange,
		ImageChangeParams: &appsv1.DeploymentTriggerImageChangeParams{
			Automatic: true,
			ContainerNames: []string{
				"container1",
			},
			From: corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: ImageStreamName + ":latest",
			},
		},
	}
}

func OkTriggeredImageChange() appsv1.DeploymentTriggerPolicy {
	ict := OkImageChangeTrigger()
	ict.ImageChangeParams.LastTriggeredImage = DockerImageReference
	return ict
}

func OkNonAutomaticICT() appsv1.DeploymentTriggerPolicy {
	ict := OkImageChangeTrigger()
	ict.ImageChangeParams.Automatic = false
	return ict
}

func OkTriggeredNonAutomatic() appsv1.DeploymentTriggerPolicy {
	ict := OkNonAutomaticICT()
	ict.ImageChangeParams.LastTriggeredImage = DockerImageReference
	return ict
}

func TestDeploymentConfig(config *appsv1.DeploymentConfig) *appsv1.DeploymentConfig {
	config.Spec.Test = true
	return config
}

func RemoveTriggerTypes(config *appsv1.DeploymentConfig, triggerTypes ...appsv1.DeploymentTriggerType) {
	types := sets.NewString()
	for _, triggerType := range triggerTypes {
		types.Insert(string(triggerType))
	}

	remaining := []appsv1.DeploymentTriggerPolicy{}
	for _, trigger := range config.Spec.Triggers {
		if types.Has(string(trigger.Type)) {
			continue
		}
		remaining = append(remaining, trigger)
	}

	config.Spec.Triggers = remaining
}

func RoundTripConfig(t *testing.T, config *appsv1.DeploymentConfig) *appsv1.DeploymentConfig {
	versioned, err := legacyscheme.Scheme.ConvertToVersion(config, appsv1.SchemeGroupVersion)
	if err != nil {
		t.Errorf("unexpected conversion error: %v", err)
		return nil
	}
	defaulted, err := legacyscheme.Scheme.ConvertToVersion(versioned, appsv1.SchemeGroupVersion)
	if err != nil {
		t.Errorf("unexpected conversion error: %v", err)
		return nil
	}
	return defaulted.(*appsv1.DeploymentConfig)
}
