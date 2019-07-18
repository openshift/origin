package test

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/library-go/pkg/image/imageutil"
	appsapi "github.com/openshift/openshift-apiserver/pkg/apps/apis/apps"
)

const (
	ImageStreamName      = "test-image-stream"
	ImageID              = "0000000000000000000000000000000000000000000000000000000000000001"
	DockerImageReference = "registry:5000/openshift/test-image-stream@sha256:0000000000000000000000000000000000000000000000000000000000000001"
)

func OkDeploymentConfig(version int64) *appsapi.DeploymentConfig {
	return &appsapi.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: kapi.NamespaceDefault,
		},
		Spec:   OkDeploymentConfigSpec(),
		Status: OkDeploymentConfigStatus(version),
	}
}

func OkDeploymentConfigSpec() appsapi.DeploymentConfigSpec {
	return appsapi.DeploymentConfigSpec{
		Replicas: 1,
		Selector: OkSelector(),
		Strategy: OkStrategy(),
		Template: OkPodTemplate(),
		Triggers: []appsapi.DeploymentTriggerPolicy{
			OkImageChangeTrigger(),
			OkConfigChangeTrigger(),
		},
	}
}

func OkDeploymentConfigStatus(version int64) appsapi.DeploymentConfigStatus {
	return appsapi.DeploymentConfigStatus{
		LatestVersion: version,
	}
}

func OkImageChangeDetails() *appsapi.DeploymentDetails {
	return &appsapi.DeploymentDetails{
		Causes: []appsapi.DeploymentCause{{
			Type: appsapi.DeploymentTriggerOnImageChange,
			ImageTrigger: &appsapi.DeploymentCauseImageTrigger{
				From: kapi.ObjectReference{
					Name: imageutil.JoinImageStreamTag(ImageStreamName, imageutil.DefaultImageTag),
					Kind: "ImageStreamTag",
				}}}}}
}

func OkConfigChangeDetails() *appsapi.DeploymentDetails {
	return &appsapi.DeploymentDetails{
		Causes: []appsapi.DeploymentCause{{
			Type: appsapi.DeploymentTriggerOnConfigChange,
		}}}
}

func OkStrategy() appsapi.DeploymentStrategy {
	return appsapi.DeploymentStrategy{
		Type: appsapi.DeploymentStrategyTypeRecreate,
		Resources: kapi.ResourceRequirements{
			Limits: kapi.ResourceList{
				kapi.ResourceName(kapi.ResourceCPU):    resource.MustParse("10"),
				kapi.ResourceName(kapi.ResourceMemory): resource.MustParse("10G"),
			},
		},
		RecreateParams: &appsapi.RecreateDeploymentStrategyParams{
			TimeoutSeconds: mkintp(20),
		},
		ActiveDeadlineSeconds: mkintp(int(appsapi.MaxDeploymentDurationSeconds)),
	}
}

func OkCustomParams() *appsapi.CustomDeploymentStrategyParams {
	return &appsapi.CustomDeploymentStrategyParams{
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

func OkSelector() map[string]string {
	return map[string]string{"a": "b"}
}

func OkPodTemplate() *kapi.PodTemplateSpec {
	one := int64(1)
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
					ImagePullPolicy:          kapi.PullIfNotPresent,
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: kapi.TerminationMessageReadFile,
				},
				{
					Name:                     "container2",
					Image:                    "registry:8080/repo1:ref2",
					ImagePullPolicy:          kapi.PullIfNotPresent,
					TerminationMessagePath:   "/dev/termination-log",
					TerminationMessagePolicy: kapi.TerminationMessageReadFile,
				},
			},
			RestartPolicy:                 kapi.RestartPolicyAlways,
			DNSPolicy:                     kapi.DNSClusterFirst,
			TerminationGracePeriodSeconds: &one,
			SchedulerName:                 kapi.DefaultSchedulerName,
			SecurityContext:               &kapi.PodSecurityContext{},
		},
		ObjectMeta: metav1.ObjectMeta{
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
			// remember that slices use copies, so have to ref array entry explicitly
			template.Spec.Containers[i].Image = ""
		}
	}
	return template
}

func OkConfigChangeTrigger() appsapi.DeploymentTriggerPolicy {
	return appsapi.DeploymentTriggerPolicy{
		Type: appsapi.DeploymentTriggerOnConfigChange,
	}
}

func OkImageChangeTrigger() appsapi.DeploymentTriggerPolicy {
	return appsapi.DeploymentTriggerPolicy{
		Type: appsapi.DeploymentTriggerOnImageChange,
		ImageChangeParams: &appsapi.DeploymentTriggerImageChangeParams{
			Automatic: true,
			ContainerNames: []string{
				"container1",
			},
			From: kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: imageutil.JoinImageStreamTag(ImageStreamName, imageutil.DefaultImageTag),
			},
		},
	}
}
