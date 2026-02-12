package nvidia

import (
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultDeviceClassName = "nvidia-gpu"
	resourceBuilderDriver  = "gpu.nvidia.com"
	defaultCudaImage       = "nvcr.io/nvidia/cuda:12.0.0-base-ubuntu22.04"
)

// ResourceBuilder helps build DRA resource objects
type ResourceBuilder struct {
	namespace string
}

// NewResourceBuilder creates a new builder
func NewResourceBuilder(namespace string) *ResourceBuilder {
	return &ResourceBuilder{namespace: namespace}
}

// BuildDeviceClass creates a DeviceClass for NVIDIA GPUs
func (rb *ResourceBuilder) BuildDeviceClass(name string) *resourceapi.DeviceClass {
	if name == "" {
		name = defaultDeviceClassName
	}

	return &resourceapi.DeviceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: resourceapi.DeviceClassSpec{
			Selectors: []resourceapi.DeviceSelector{
				{
					CEL: &resourceapi.CELDeviceSelector{
						Expression: "device.driver == \"" + resourceBuilderDriver + "\"",
					},
				},
			},
		},
	}
}

// BuildResourceClaim creates a ResourceClaim requesting GPUs
func (rb *ResourceBuilder) BuildResourceClaim(name, deviceClassName string, count int) *resourceapi.ResourceClaim {
	if deviceClassName == "" {
		deviceClassName = defaultDeviceClassName
	}

	deviceRequests := []resourceapi.DeviceRequest{
		{
			Name: "gpu",
			Exactly: &resourceapi.ExactDeviceRequest{
				DeviceClassName: deviceClassName,
				Count:           int64(count),
			},
		},
	}

	return &resourceapi.ResourceClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rb.namespace,
		},
		Spec: resourceapi.ResourceClaimSpec{
			Devices: resourceapi.DeviceClaim{
				Requests: deviceRequests,
			},
		},
	}
}

// BuildPodWithClaim creates a Pod that uses a ResourceClaim
func (rb *ResourceBuilder) BuildPodWithClaim(name, claimName, image string) *corev1.Pod {
	if image == "" {
		image = defaultCudaImage
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
					Name:    "gpu-container",
					Image:   image,
					Command: []string{"sh", "-c", "nvidia-smi && sleep infinity"},
					Resources: corev1.ResourceRequirements{
						Claims: []corev1.ResourceClaim{
							{
								Name: "gpu",
							},
						},
					},
				},
			},
			ResourceClaims: []corev1.PodResourceClaim{
				{
					Name:              "gpu",
					ResourceClaimName: &claimName,
				},
			},
		},
	}
}

// BuildPodWithInlineClaim creates a Pod with inline ResourceClaim
// Note: Inline claims via ResourceClaimTemplate are not directly supported in pod spec
// This creates a pod that references a ResourceClaimTemplateName
func (rb *ResourceBuilder) BuildPodWithInlineClaim(name, deviceClassName string, gpuCount int) *corev1.Pod {
	if deviceClassName == "" {
		deviceClassName = defaultDeviceClassName
	}

	// Note: The actual ResourceClaimTemplate must be created separately
	templateName := name + "-template"

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rb.namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "gpu-container",
					Image:   defaultCudaImage,
					Command: []string{"sh", "-c", "nvidia-smi && sleep infinity"},
					Resources: corev1.ResourceRequirements{
						Claims: []corev1.ResourceClaim{
							{
								Name: "gpu",
							},
						},
					},
				},
			},
			ResourceClaims: []corev1.PodResourceClaim{
				{
					Name:                      "gpu",
					ResourceClaimTemplateName: &templateName,
				},
			},
		},
	}
}

// BuildPodWithCommand creates a Pod with a custom command
func (rb *ResourceBuilder) BuildPodWithCommand(name, claimName, image string, command []string) *corev1.Pod {
	if image == "" {
		image = defaultCudaImage
	}

	pod := rb.BuildPodWithClaim(name, claimName, image)
	pod.Spec.Containers[0].Command = command
	return pod
}

// BuildLongRunningPodWithClaim creates a long-running Pod for testing
func (rb *ResourceBuilder) BuildLongRunningPodWithClaim(name, claimName, image string) *corev1.Pod {
	if image == "" {
		image = defaultCudaImage
	}

	pod := rb.BuildPodWithClaim(name, claimName, image)
	pod.Spec.Containers[0].Command = []string{"sh", "-c", "while true; do nvidia-smi; sleep 60; done"}
	return pod
}

// BuildMultiGPUClaim creates a ResourceClaim for multiple GPUs
func (rb *ResourceBuilder) BuildMultiGPUClaim(name, deviceClassName string, gpuCount int) *resourceapi.ResourceClaim {
	return rb.BuildResourceClaim(name, deviceClassName, gpuCount)
}

// BuildSharedClaim creates a shareable ResourceClaim (if supported)
func (rb *ResourceBuilder) BuildSharedClaim(name, deviceClassName string, count int) *resourceapi.ResourceClaim {
	claim := rb.BuildResourceClaim(name, deviceClassName, count)
	// Add shareable configuration if needed based on NVIDIA driver capabilities
	// This may require additional fields in the ResourceClaim spec
	return claim
}

// BuildDeviceClassWithConfig creates a DeviceClass with additional configuration
func (rb *ResourceBuilder) BuildDeviceClassWithConfig(name string, config *resourceapi.DeviceClassConfiguration) *resourceapi.DeviceClass {
	dc := rb.BuildDeviceClass(name)
	if config != nil {
		dc.Spec.Config = []resourceapi.DeviceClassConfiguration{*config}
	}
	return dc
}

// BuildDeviceClassWithConstraints creates a DeviceClass with constraints
func (rb *ResourceBuilder) BuildDeviceClassWithConstraints(name, constraints string) *resourceapi.DeviceClass {
	dc := rb.BuildDeviceClass(name)
	if constraints != "" {
		dc.Spec.Selectors = []resourceapi.DeviceSelector{
			{
				CEL: &resourceapi.CELDeviceSelector{
					Expression: constraints,
				},
			},
		}
	}
	return dc
}
