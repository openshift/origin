package legacy

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/apis/core"
	corev1conversions "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacv1conversions "k8s.io/kubernetes/pkg/apis/rbac/v1"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	"github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationv1helpers "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
)

// InstallLegacyAuthorization this looks like a lot of duplication, but the code in the individual versions is living and may
// change. The code here should never change and needs to allow the other code to move independently.
func InstallInternalLegacyAuthorization(scheme *runtime.Scheme) {
	InstallExternalLegacyAuthorization(scheme)

	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedInternalAuthorizationTypes,
		core.AddToScheme,
		rbac.AddToScheme,
		corev1conversions.AddToScheme,
		rbacv1conversions.AddToScheme,

		authorizationv1helpers.AddConversionFuncs,
		authorizationv1helpers.AddFieldSelectorKeyConversions,
		authorizationv1helpers.RegisterDefaults,
		authorizationv1helpers.RegisterConversions,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func InstallExternalLegacyAuthorization(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedAuthorizationTypes,
		corev1.AddToScheme,
		rbacv1.AddToScheme,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedAuthorizationTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&authorizationv1.Role{},
		&authorizationv1.RoleBinding{},
		&authorizationv1.RoleBindingList{},
		&authorizationv1.RoleList{},

		&authorizationv1.SelfSubjectRulesReview{},
		&authorizationv1.SubjectRulesReview{},
		&authorizationv1.ResourceAccessReview{},
		&authorizationv1.SubjectAccessReview{},
		&authorizationv1.LocalResourceAccessReview{},
		&authorizationv1.LocalSubjectAccessReview{},
		&authorizationv1.ResourceAccessReviewResponse{},
		&authorizationv1.SubjectAccessReviewResponse{},
		&authorizationv1.IsPersonalSubjectAccessReview{},

		&authorizationv1.ClusterRole{},
		&authorizationv1.ClusterRoleBinding{},
		&authorizationv1.ClusterRoleBindingList{},
		&authorizationv1.ClusterRoleList{},

		&authorizationv1.RoleBindingRestriction{},
		&authorizationv1.RoleBindingRestrictionList{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}

func addUngroupifiedInternalAuthorizationTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(InternalGroupVersion,
		&authorization.Role{},
		&authorization.RoleBinding{},
		&authorization.RoleBindingList{},
		&authorization.RoleList{},

		&authorization.SelfSubjectRulesReview{},
		&authorization.SubjectRulesReview{},
		&authorization.ResourceAccessReview{},
		&authorization.SubjectAccessReview{},
		&authorization.LocalResourceAccessReview{},
		&authorization.LocalSubjectAccessReview{},
		&authorization.ResourceAccessReviewResponse{},
		&authorization.SubjectAccessReviewResponse{},
		&authorization.IsPersonalSubjectAccessReview{},

		&authorization.ClusterRole{},
		&authorization.ClusterRoleBinding{},
		&authorization.ClusterRoleBindingList{},
		&authorization.ClusterRoleList{},

		&authorization.RoleBindingRestriction{},
		&authorization.RoleBindingRestrictionList{},
	)
	return nil
}
