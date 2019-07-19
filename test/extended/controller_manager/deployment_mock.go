package controller_manager

import (
	appsv1 "github.com/openshift/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ImageStreamName = "test-image-stream"
	ImageID         = "0000000000000000000000000000000000000000000000000000000000000001"
)

func OkDeploymentConfig(version int64) *appsv1.DeploymentConfig {
	one := int64(1)
	return &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: corev1.NamespaceDefault,
			SelfLink:  "/apis/apps.openshift.io/v1/deploymentConfig/config",
		},
		Spec: appsv1.DeploymentConfigSpec{
			Replicas: 1,
			Selector: map[string]string{"a": "b"},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.DeploymentStrategyTypeRecreate,
				RecreateParams: &appsv1.RecreateDeploymentStrategyParams{
					TimeoutSeconds: mkintp(20),
				},
				ActiveDeadlineSeconds: mkintp(21600),
			},
			Template: &corev1.PodTemplateSpec{
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
					Labels: map[string]string{"a": "b"},
				},
			},
			Triggers: []appsv1.DeploymentTriggerPolicy{
				OkImageChangeTrigger(),
				{
					Type: appsv1.DeploymentTriggerOnConfigChange,
				},
			},
		},
		Status: appsv1.DeploymentConfigStatus{
			LatestVersion: version,
		},
	}
}

func mkintp(i int) *int64 {
	v := int64(i)
	return &v
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
