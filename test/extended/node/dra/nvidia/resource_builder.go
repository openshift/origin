package nvidia

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
	if count <= 0 {
		panic(fmt.Sprintf("BuildResourceClaim: count must be > 0, got %d", count))
	}
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
// The GPU count is specified in the ResourceClaimTemplate, not in this pod spec
func (rb *ResourceBuilder) BuildPodWithInlineClaim(name, deviceClassName string) *corev1.Pod {
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
					Name:                      "gpu",
					ResourceClaimTemplateName: &templateName,
				},
			},
		},
	}
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

// BuildMultiContainerPodWithClaim creates a Pod with two containers sharing the same GPU claim
func (rb *ResourceBuilder) BuildMultiContainerPodWithClaim(name, claimName, image string) *corev1.Pod {
	if image == "" {
		image = defaultCudaImage
	}

	secCtx := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
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
					Name:    "gpu-container-1",
					Image:   image,
					Command: []string{"sh", "-c", "nvidia-smi && sleep infinity"},
					Resources: corev1.ResourceRequirements{
						Claims: []corev1.ResourceClaim{
							{
								Name: "gpu",
							},
						},
					},
					SecurityContext: secCtx,
				},
				{
					Name:    "gpu-container-2",
					Image:   image,
					Command: []string{"sh", "-c", "nvidia-smi && sleep infinity"},
					Resources: corev1.ResourceRequirements{
						Claims: []corev1.ResourceClaim{
							{
								Name: "gpu",
							},
						},
					},
					SecurityContext: secCtx,
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

// BuildPodWithInitContainerAndClaim creates a Pod with an init container and main container both using the GPU claim
func (rb *ResourceBuilder) BuildPodWithInitContainerAndClaim(name, claimName, image string) *corev1.Pod {
	if image == "" {
		image = defaultCudaImage
	}

	secCtx := &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rb.namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			InitContainers: []corev1.Container{
				{
					Name:    "gpu-init",
					Image:   image,
					Command: []string{"nvidia-smi"},
					Resources: corev1.ResourceRequirements{
						Claims: []corev1.ResourceClaim{
							{
								Name: "gpu",
							},
						},
					},
					SecurityContext: secCtx,
				},
			},
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
					SecurityContext: secCtx,
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

// BuildNonRootPodWithClaim creates a Pod that runs as a non-root user with the GPU claim
func (rb *ResourceBuilder) BuildNonRootPodWithClaim(name, claimName, image string) *corev1.Pod {
	pod := rb.BuildPodWithClaim(name, claimName, image)
	pod.Spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: ptr.To(true),
		RunAsUser:    ptr.To[int64](1000),
		RunAsGroup:   ptr.To[int64](1000),
	}
	return pod
}

// BuildResourceClaimWithCELSelector creates a ResourceClaim with a direct CEL selector instead of a DeviceClass
func (rb *ResourceBuilder) BuildResourceClaimWithCELSelector(name, celExpression string, count int) *resourceapi.ResourceClaim {
	if count <= 0 {
		panic(fmt.Sprintf("BuildResourceClaimWithCELSelector: count must be > 0, got %d", count))
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
						Name: "gpu",
						Exactly: &resourceapi.ExactDeviceRequest{
							Count: int64(count),
							Selectors: []resourceapi.DeviceSelector{
								{
									CEL: &resourceapi.CELDeviceSelector{
										Expression: celExpression,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// BuildResourceClaimTemplate creates a ResourceClaimTemplate
func (rb *ResourceBuilder) BuildResourceClaimTemplate(name, deviceClassName string, count int) *resourceapi.ResourceClaimTemplate {
	if count <= 0 {
		panic(fmt.Sprintf("BuildResourceClaimTemplate: count must be > 0, got %d", count))
	}
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

	return &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: rb.namespace,
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			Spec: resourceapi.ResourceClaimSpec{
				Devices: resourceapi.DeviceClaim{
					Requests: deviceRequests,
				},
			},
		},
	}
}
