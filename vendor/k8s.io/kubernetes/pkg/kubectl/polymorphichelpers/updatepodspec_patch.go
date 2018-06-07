package polymorphichelpers

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"

	appsapiv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

func updatePodSpecForObjectOrigin(obj runtime.Object, fn func(*v1.PodSpec) error) (bool, error) {
	switch t := obj.(type) {
	case *appsapi.DeploymentConfig:
		template := t.Spec.Template
		if template == nil {
			t.Spec.Template = template
			template = &core.PodTemplateSpec{}
		}
		if err := ConvertExteralPodSpecToInternal(fn)(&template.Spec); err != nil {
			return true, err
		}
		return true, nil
	case *appsapiv1.DeploymentConfig:
		template := t.Spec.Template
		if template == nil {
			template = &v1.PodTemplateSpec{}
			t.Spec.Template = template
		}
		return true, fn(&template.Spec)

	// FIXME-REBASE: we should probably get rid of these:
	// k8s internals
	case *extensions.Deployment:
		return true, ConvertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
	case *extensions.DaemonSet:
		return true, ConvertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
	case *extensions.ReplicaSet:
		return true, ConvertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
	case *apps.StatefulSet:
		return true, ConvertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
	case *batch.Job:
		return true, ConvertExteralPodSpecToInternal(fn)(&t.Spec.Template.Spec)
	case *batch.CronJob:
		return true, ConvertExteralPodSpecToInternal(fn)(&t.Spec.JobTemplate.Spec.Template.Spec)

	default:
		return updatePodSpecForObject(obj, fn)
	}
}

func ConvertInteralPodSpecToExternal(inFn func(*core.PodSpec) error) func(*v1.PodSpec) error {
	return func(specToMutate *v1.PodSpec) error {
		internalPodSpec := &core.PodSpec{}
		if err := legacyscheme.Scheme.Convert(specToMutate, internalPodSpec, nil); err != nil {
			return err
		}
		if err := inFn(internalPodSpec); err != nil {
			return err
		}
		externalPodSpec := &v1.PodSpec{}
		if err := legacyscheme.Scheme.Convert(internalPodSpec, externalPodSpec, nil); err != nil {
			return err
		}
		*specToMutate = *externalPodSpec
		return nil
	}
}

func ConvertExteralPodSpecToInternal(inFn func(*v1.PodSpec) error) func(*core.PodSpec) error {
	return func(specToMutate *core.PodSpec) error {
		externalPodSpec := &v1.PodSpec{}
		if err := legacyscheme.Scheme.Convert(specToMutate, externalPodSpec, nil); err != nil {
			return err
		}
		if err := inFn(externalPodSpec); err != nil {
			return err
		}
		internalPodSpec := &core.PodSpec{}
		if err := legacyscheme.Scheme.Convert(externalPodSpec, internalPodSpec, nil); err != nil {
			return err
		}
		*specToMutate = *internalPodSpec
		return nil
	}
}
