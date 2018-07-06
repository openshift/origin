package originpolymorphichelpers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/kubectl/polymorphichelpers"

	appsapiv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func NewUpdatePodSpecForObjectFn(delegate polymorphichelpers.UpdatePodSpecForObjectFunc) polymorphichelpers.UpdatePodSpecForObjectFunc {
	return func(obj runtime.Object, fn func(*corev1.PodSpec) error) (bool, error) {
		switch t := obj.(type) {
		case *appsapi.DeploymentConfig:
			template := t.Spec.Template
			if template == nil {
				t.Spec.Template = template
				template = &kapi.PodTemplateSpec{}
			}
			if err := convertExteralPodSpecToInternal(fn)(&template.Spec); err != nil {
				return true, err
			}
			return true, nil

		case *appsapiv1.DeploymentConfig:
			template := t.Spec.Template
			if template == nil {
				template = &corev1.PodTemplateSpec{}
				t.Spec.Template = template
			}
			return true, fn(&template.Spec)

		// TODO: we need to get rid of these:
		// k8s internals
		case *kapi.Pod:
			return true, convertExteralPodSpecToInternal(fn)(&t.Spec)
		case *kapi.ReplicationController:
			return true, convertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
		case *extensions.Deployment:
			return true, convertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
		case *extensions.DaemonSet:
			return true, convertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
		case *extensions.ReplicaSet:
			return true, convertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
		case *apps.StatefulSet:
			return true, convertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
		case *batch.Job:
			return true, convertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
		case *batch.CronJob:
			return true, convertExteralPodSpecToInternal(fn)(&t.Spec.JobTemplate.Spec.Template.Spec)

		default:
			return delegate(obj, fn)
		}
	}
}

func convertExteralPodSpecToInternal(inFn func(*corev1.PodSpec) error) func(*kapi.PodSpec) error {
	return func(specToMutate *kapi.PodSpec) error {
		externalPodSpec := &corev1.PodSpec{}
		if err := legacyscheme.Scheme.Convert(specToMutate, externalPodSpec, nil); err != nil {
			return err
		}
		if err := inFn(externalPodSpec); err != nil {
			return err
		}
		internalPodSpec := &kapi.PodSpec{}
		if err := legacyscheme.Scheme.Convert(externalPodSpec, internalPodSpec, nil); err != nil {
			return err
		}
		*specToMutate = *internalPodSpec
		return nil
	}
}
