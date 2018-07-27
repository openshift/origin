package legacy

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/apis/core"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/origin/pkg/security/apis/security"
	securityv1helpers "github.com/openshift/origin/pkg/security/apis/security/v1"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"
)

// InstallLegacySecurity this looks like a lot of duplication, but the code in the individual versions is living and may
// change. The code here should never change and needs to allow the other code to move independently.
func InstallInternalLegacySecurity(scheme *runtime.Scheme) {
	InstallExternalLegacySecurity(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedInternalSecurityTypes,
		core.AddToScheme,
		corev1conversions.AddToScheme,

		securityv1helpers.AddConversionFuncs,
		securityv1helpers.AddDefaultingFuncs,
		securityv1helpers.RegisterDefaults,
		securityv1helpers.RegisterConversions,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

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

func addUngroupifiedInternalSecurityTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&security.SecurityContextConstraints{},
		&security.SecurityContextConstraintsList{},
		&security.PodSecurityPolicySubjectReview{},
		&security.PodSecurityPolicySelfSubjectReview{},
		&security.PodSecurityPolicyReview{},
	}
	scheme.AddKnownTypes(InternalGroupVersion, types...)
	return nil
}
