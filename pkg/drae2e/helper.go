package drae2e

import (
	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
)

func NewHelper(f *framework.Framework, driverName string) Helper {
	return Helper{f: f, DeviceClassName: driverName}
}

type Helper struct {
	f               *framework.Framework
	DeviceClassName string
}

func (h Helper) NewResourceClaimTemplate(name string) *resourceapi.ResourceClaimTemplate {
	return &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: h.f.Namespace.Name,
			Name:      name,
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			Spec: resourceapi.ResourceClaimSpec{
				Devices: resourceapi.DeviceClaim{
					Requests: []resourceapi.DeviceRequest{
						{
							Name:            "gpu",
							DeviceClassName: h.DeviceClassName,
						},
					},
				},
			},
		},
	}
}

func (h Helper) NewContainer(name string) corev1.Container {
	return corev1.Container{
		Name:            name,
		Image:           e2epodutil.GetDefaultTestImage(),
		Command:         e2epodutil.GenerateScriptCmd("env && sleep 100000"),
		SecurityContext: e2epodutil.GetRestrictedContainerSecurityContext(),
	}
}

func (h Helper) NewPod(name string) *corev1.Pod {
	pod := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   h.f.Namespace.Name,
			Labels:      make(map[string]string),
			Annotations: make(map[string]string),
		},
		Spec: corev1.PodSpec{
			RestartPolicy:                 corev1.RestartPolicyNever,
			TerminationGracePeriodSeconds: ptr.To[int64](1),
			SecurityContext:               e2epodutil.GetRestrictedPodSecurityContext(),
		},
	}
	return pod
}
