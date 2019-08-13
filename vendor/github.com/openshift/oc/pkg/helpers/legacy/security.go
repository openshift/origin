package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	securityv1 "github.com/openshift/api/security/v1"
)

func InstallExternalLegacySecurity(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedSecurityTypes,
		corev1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedSecurityTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&securityv1.SecurityContextConstraints{},
		&securityv1.SecurityContextConstraintsList{},
		&securityv1.PodSecurityPolicySubjectReview{},
		&securityv1.PodSecurityPolicySelfSubjectReview{},
		&securityv1.PodSecurityPolicyReview{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}
