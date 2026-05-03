package example

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/openshift/origin/test/extended/util/image"
)

const (
	exampleDriverName  = "gpu.example.com"
	defaultDeviceClass = "gpu.example.com"
	deviceRequestName  = "device"
)

// ResourceBuilder constructs Kubernetes DRA objects for the dra-example-driver.
type ResourceBuilder struct {
	namespace string
}

// NewResourceBuilder creates a ResourceBuilder scoped to the given namespace.
func NewResourceBuilder(namespace string) *ResourceBuilder {
	return &ResourceBuilder{namespace: namespace}
}

// BuildDeviceClass creates a DeviceClass with a CEL selector for the example driver.
func (rb *ResourceBuilder) BuildDeviceClass(name string) *resourceapi.DeviceClass {
	if name == "" {
		name = defaultDeviceClass
	}

	return &resourceapi.DeviceClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: resourceapi.DeviceClassSpec{
			Selectors: []resourceapi.DeviceSelector{
				{
					CEL: &resourceapi.CELDeviceSelector{
						Expression: fmt.Sprintf("device.driver == %q", exampleDriverName),
					},
				},
			},
		},
	}
}

// BuildResourceClaim creates a ResourceClaim requesting the specified number of devices.
func (rb *ResourceBuilder) BuildResourceClaim(name, deviceClassName string, count int) *resourceapi.ResourceClaim {
	if count <= 0 {
		panic(fmt.Sprintf("BuildResourceClaim: count must be > 0, got %d", count))
	}
	if deviceClassName == "" {
		deviceClassName = defaultDeviceClass
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
						Name: deviceRequestName,
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

// BuildPodWithClaim creates a Pod that references an existing ResourceClaim.
// If img is empty, the OpenShift release payload tools image is used.
func (rb *ResourceBuilder) BuildPodWithClaim(name, claimName, img string) *corev1.Pod {
	if img == "" {
		img = image.ShellImage()
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
					Command: []string{"sh", "-c", "echo DRA device allocated && sleep infinity"},
					Resources: corev1.ResourceRequirements{
						Claims: []corev1.ResourceClaim{
							{
								Name: deviceRequestName,
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
					Name:              deviceRequestName,
					ResourceClaimName: &claimName,
				},
			},
		},
	}
}

// BuildLongRunningPodWithClaim creates a Pod with a periodic heartbeat loop instead of sleep infinity.
func (rb *ResourceBuilder) BuildLongRunningPodWithClaim(name, claimName, img string) *corev1.Pod {
	pod := rb.BuildPodWithClaim(name, claimName, img)
	pod.Spec.Containers[0].Command = []string{"sh", "-c", "while true; do echo DRA device active; sleep 60; done"}
	return pod
}

// BuildPodWithInlineClaim creates a Pod that references a ResourceClaimTemplate for inline allocation.
func (rb *ResourceBuilder) BuildPodWithInlineClaim(name string) *corev1.Pod {
	templateName := name + "-template"
	pod := rb.BuildPodWithClaim(name, "", "")
	pod.Spec.ResourceClaims[0].ResourceClaimName = nil
	pod.Spec.ResourceClaims[0].ResourceClaimTemplateName = &templateName
	return pod
}

// BuildResourceClaimTemplate creates a ResourceClaimTemplate for inline claim generation.
func (rb *ResourceBuilder) BuildResourceClaimTemplate(name, deviceClassName string, count int) *resourceapi.ResourceClaimTemplate {
	if count <= 0 {
		panic(fmt.Sprintf("BuildResourceClaimTemplate: count must be > 0, got %d", count))
	}
	if deviceClassName == "" {
		deviceClassName = defaultDeviceClass
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
							Name: deviceRequestName,
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
