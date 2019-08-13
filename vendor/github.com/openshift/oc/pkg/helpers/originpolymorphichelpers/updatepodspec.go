package originpolymorphichelpers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	oappsv1 "github.com/openshift/api/apps/v1"
)

func NewUpdatePodSpecForObjectFn(delegate polymorphichelpers.UpdatePodSpecForObjectFunc) polymorphichelpers.UpdatePodSpecForObjectFunc {
	return func(obj runtime.Object, fn func(*corev1.PodSpec) error) (bool, error) {
		switch t := obj.(type) {
		case *oappsv1.DeploymentConfig:
			template := t.Spec.Template
			if template == nil {
				template = &corev1.PodTemplateSpec{}
				t.Spec.Template = template
			}
			return true, fn(&template.Spec)

		default:
			return delegate(obj, fn)
		}
	}
}
