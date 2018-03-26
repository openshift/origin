package resourcemerge

import (
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
)

func EnsureDeployment(modified *bool, existing *appsv1.Deployment, required appsv1.Deployment) {
	EnsureObjectMeta(modified, &existing.ObjectMeta, required.ObjectMeta)

	if existing.Spec.Selector == nil {
		*modified = true
		existing.Spec.Selector = required.Spec.Selector
	}
	if !reflect.DeepEqual(existing.Spec.Selector, required.Spec.Selector) {
		*modified = true
		existing.Spec.Selector = required.Spec.Selector
	}

	ensurePodTemplateSpec(modified, &existing.Spec.Template, required.Spec.Template)
}
