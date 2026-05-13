package common

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// DriverConfig holds driver-specific parameters for building DRA test resources
type DriverConfig struct {
	DriverName       string
	DefaultClass     string
	RequestName      string
	ContainerImage   string
	ContainerCommand []string
	LongRunCommand   []string
}

// ResourceBuilder helps build DRA resource objects
type ResourceBuilder struct {
	namespace string
	config    DriverConfig
}

// NewResourceBuilder creates a new builder
func NewResourceBuilder(namespace string, config DriverConfig) *ResourceBuilder {
	return &ResourceBuilder{namespace: namespace, config: config}
}

// BuildDeviceClass creates a DeviceClass for the configured driver
func (rb *ResourceBuilder) BuildDeviceClass(name string) *resourceapi.DeviceClass {
	if name == "" {
		name = rb.config.DefaultClass
	}

	return &resourceapi.DeviceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: resourceapi.DeviceClassSpec{
			Selectors: []resourceapi.DeviceSelector{
				{
					CEL: &resourceapi.CELDeviceSelector{
						Expression: fmt.Sprintf("device.driver == %q", rb.config.DriverName),
					},
				},
			},
		},
	}
}

// BuildResourceClaim creates a ResourceClaim requesting devices
func (rb *ResourceBuilder) BuildResourceClaim(name, deviceClassName string, count int) *resourceapi.ResourceClaim {
	if count <= 0 {
		panic(fmt.Sprintf("BuildResourceClaim: count must be > 0, got %d", count))
	}
	if deviceClassName == "" {
		deviceClassName = rb.config.DefaultClass
	}

	return &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rb.namespace,
		},
		Spec: resourceapi.ResourceClaimSpec{
			Devices: resourceapi.DeviceClaim{
				Requests: []resourceapi.DeviceRequest{
					{
						Name: rb.config.RequestName,
						Exactly: &resourceapi.ExactDeviceRequest{
							DeviceClassName: deviceClassName,
							Count:           int64(count),
						},
					},
				},
			},
		},
	}
}

// BuildPodWithClaim creates a Pod that uses a ResourceClaim
func (rb *ResourceBuilder) BuildPodWithClaim(name, claimName, img string) *corev1.Pod {
	if img == "" {
		img = rb.config.ContainerImage
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rb.namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "test-container",
					Image:   img,
					Command: rb.config.ContainerCommand,
					Resources: corev1.ResourceRequirements{
						Claims: []corev1.ResourceClaim{
							{
								Name: rb.config.RequestName,
							},
						},
					},
					SecurityContext: &corev1.SecurityContext{
						AllowPrivilegeEscalation: ptr.To(false),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
					},
				},
			},
			ResourceClaims: []corev1.PodResourceClaim{
				{
					Name:              rb.config.RequestName,
					ResourceClaimName: &claimName,
				},
			},
		},
	}
}

// BuildLongRunningPodWithClaim creates a long-running Pod for testing
func (rb *ResourceBuilder) BuildLongRunningPodWithClaim(name, claimName, img string) *corev1.Pod {
	pod := rb.BuildPodWithClaim(name, claimName, img)
	pod.Spec.Containers[0].Command = rb.config.LongRunCommand
	return pod
}

// BuildPodWithInlineClaim creates a Pod with inline ResourceClaim
func (rb *ResourceBuilder) BuildPodWithInlineClaim(name string) *corev1.Pod {
	templateName := name + "-template"
	pod := rb.BuildPodWithClaim(name, "", "")
	pod.Spec.ResourceClaims[0].ResourceClaimName = nil
	pod.Spec.ResourceClaims[0].ResourceClaimTemplateName = &templateName
	return pod
}

// BuildResourceClaimTemplate creates a ResourceClaimTemplate
func (rb *ResourceBuilder) BuildResourceClaimTemplate(name, deviceClassName string, count int) *resourceapi.ResourceClaimTemplate {
	if count <= 0 {
		panic(fmt.Sprintf("BuildResourceClaimTemplate: count must be > 0, got %d", count))
	}
	if deviceClassName == "" {
		deviceClassName = rb.config.DefaultClass
	}

	return &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rb.namespace,
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			Spec: resourceapi.ResourceClaimSpec{
				Devices: resourceapi.DeviceClaim{
					Requests: []resourceapi.DeviceRequest{
						{
							Name: rb.config.RequestName,
							Exactly: &resourceapi.ExactDeviceRequest{
								DeviceClassName: deviceClassName,
								Count:           int64(count),
							},
						},
					},
				},
			},
		},
	}
}
