package legacy

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	authorizationv1 "github.com/openshift/api/authorization/v1"
)

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
