package originimagereferencemutators

import (
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsapiv1 "github.com/openshift/api/apps/v1"
	securityapiv1 "github.com/openshift/api/security/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
	"k8s.io/kubernetes/openshift-kube-apiserver/admission/imagepolicy/imagereferencemutators"
)

// getPodSpec returns a mutable pod spec out of the provided object, including a field path
// to the field in the object, or an error if the object does not contain a pod spec.
// This only returns internal objects.
func getPodSpec(obj runtime.Object) (*kapi.PodSpec, *field.Path, error) {
	switch r := obj.(type) {
	case *securityapi.PodSecurityPolicySubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapi.PodSecurityPolicySelfSubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapi.PodSecurityPolicyReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *appsapi.DeploymentConfig:
		if r.Spec.Template != nil {
			return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
		}
	}
	return imagereferencemutators.GetPodSpec(obj)
}

// getPodSpecV1 returns a mutable pod spec out of the provided object, including a field path
// to the field in the object, or an error if the object does not contain a pod spec.
// This only returns pod specs for v1 compatible objects.
func getPodSpecV1(obj runtime.Object) (*kapiv1.PodSpec, *field.Path, error) {
	switch r := obj.(type) {
	case *securityapiv1.PodSecurityPolicySubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapiv1.PodSecurityPolicySelfSubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapiv1.PodSecurityPolicyReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *appsapiv1.DeploymentConfig:
		if r.Spec.Template != nil {
			return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
		}
	}
	return imagereferencemutators.GetPodSpecV1(obj)
}

// getTemplateMetaObject returns a mutable metav1.Object interface for the template
// the object contains, or false if no such object is available.
func getTemplateMetaObject(obj runtime.Object) (metav1.Object, bool) {
	switch r := obj.(type) {
	case *securityapiv1.PodSecurityPolicySubjectReview:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapiv1.PodSecurityPolicySelfSubjectReview:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapiv1.PodSecurityPolicyReview:
		return &r.Spec.Template.ObjectMeta, true
	case *appsapiv1.DeploymentConfig:
		if r.Spec.Template != nil {
			return &r.Spec.Template.ObjectMeta, true
		}

	case *securityapi.PodSecurityPolicySubjectReview:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapi.PodSecurityPolicySelfSubjectReview:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapi.PodSecurityPolicyReview:
		return &r.Spec.Template.ObjectMeta, true
	case *appsapi.DeploymentConfig:
		if r.Spec.Template != nil {
			return &r.Spec.Template.ObjectMeta, true
		}
	}
	return imagereferencemutators.GetTemplateMetaObject(obj)
}
