package resourcemerge

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
)

func EnsureService(modified *bool, existing *corev1.Service, required corev1.Service) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	if !reflect.DeepEqual(existing.Spec.Selector, required.Spec.Selector) {
		*modified = true
		existing.Spec.Selector = required.Spec.Selector
	}

	// any port we specify, we require
	for _, required := range required.Spec.Ports {
		var existingCurr *corev1.ServicePort
		for j, curr := range existing.Spec.Ports {
			if curr.Name == required.Name {
				existingCurr = &existing.Spec.Ports[j]
				break
			}
		}
		if existingCurr == nil {
			*modified = true
			existing.Spec.Ports = append(existing.Spec.Ports, corev1.ServicePort{})
			existingCurr = &existing.Spec.Ports[len(existing.Spec.Ports)-1]
		}
		ensureServicePort(modified, existingCurr, required)
	}
}

func ensureServicePort(modified *bool, existing *corev1.ServicePort, required corev1.ServicePort) {
	if !equality.Semantic.DeepEqual(required, *existing) {
		*modified = true
		*existing = required
	}
}
