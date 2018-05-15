package v1

import (
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	"github.com/openshift/api/authorization/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	newer "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

func Convert_v1_SubjectAccessReview_To_authorization_SubjectAccessReview(in *v1.SubjectAccessReview, out *newer.SubjectAccessReview, s conversion.Scope) error {
	if err := autoConvert_v1_SubjectAccessReview_To_authorization_SubjectAccessReview(in, out, s); err != nil {
		return err
	}

	out.Groups = sets.NewString(in.GroupsSlice...)
	out.Scopes = []string(in.Scopes)
	return nil
}

func Convert_authorization_SubjectAccessReview_To_v1_SubjectAccessReview(in *newer.SubjectAccessReview, out *v1.SubjectAccessReview, s conversion.Scope) error {
	if err := autoConvert_authorization_SubjectAccessReview_To_v1_SubjectAccessReview(in, out, s); err != nil {
		return err
	}

	out.GroupsSlice = in.Groups.List()
	out.Scopes = v1.OptionalScopes(in.Scopes)
	return nil
}

func Convert_v1_SelfSubjectRulesReviewSpec_To_authorization_SelfSubjectRulesReviewSpec(in *v1.SelfSubjectRulesReviewSpec, out *newer.SelfSubjectRulesReviewSpec, s conversion.Scope) error {
	if err := autoConvert_v1_SelfSubjectRulesReviewSpec_To_authorization_SelfSubjectRulesReviewSpec(in, out, s); err != nil {
		return err
	}

	out.Scopes = []string(in.Scopes)
	return nil
}

func Convert_authorization_SelfSubjectRulesReviewSpec_To_v1_SelfSubjectRulesReviewSpec(in *newer.SelfSubjectRulesReviewSpec, out *v1.SelfSubjectRulesReviewSpec, s conversion.Scope) error {
	if err := autoConvert_authorization_SelfSubjectRulesReviewSpec_To_v1_SelfSubjectRulesReviewSpec(in, out, s); err != nil {
		return err
	}

	out.Scopes = v1.OptionalScopes(in.Scopes)
	return nil
}

func Convert_v1_LocalSubjectAccessReview_To_authorization_LocalSubjectAccessReview(in *v1.LocalSubjectAccessReview, out *newer.LocalSubjectAccessReview, s conversion.Scope) error {
	if err := autoConvert_v1_LocalSubjectAccessReview_To_authorization_LocalSubjectAccessReview(in, out, s); err != nil {
		return err
	}

	out.Groups = sets.NewString(in.GroupsSlice...)
	out.Scopes = []string(in.Scopes)
	return nil
}

func Convert_authorization_LocalSubjectAccessReview_To_v1_LocalSubjectAccessReview(in *newer.LocalSubjectAccessReview, out *v1.LocalSubjectAccessReview, s conversion.Scope) error {
	if err := autoConvert_authorization_LocalSubjectAccessReview_To_v1_LocalSubjectAccessReview(in, out, s); err != nil {
		return err
	}

	out.GroupsSlice = in.Groups.List()
	out.Scopes = v1.OptionalScopes(in.Scopes)
	return nil
}

func Convert_v1_ResourceAccessReviewResponse_To_authorization_ResourceAccessReviewResponse(in *v1.ResourceAccessReviewResponse, out *newer.ResourceAccessReviewResponse, s conversion.Scope) error {
	if err := autoConvert_v1_ResourceAccessReviewResponse_To_authorization_ResourceAccessReviewResponse(in, out, s); err != nil {
		return err
	}

	out.Users = sets.NewString(in.UsersSlice...)
	out.Groups = sets.NewString(in.GroupsSlice...)
	return nil
}

func Convert_authorization_ResourceAccessReviewResponse_To_v1_ResourceAccessReviewResponse(in *newer.ResourceAccessReviewResponse, out *v1.ResourceAccessReviewResponse, s conversion.Scope) error {
	if err := autoConvert_authorization_ResourceAccessReviewResponse_To_v1_ResourceAccessReviewResponse(in, out, s); err != nil {
		return err
	}

	out.UsersSlice = in.Users.List()
	out.GroupsSlice = in.Groups.List()
	return nil
}

func Convert_v1_PolicyRule_To_authorization_PolicyRule(in *v1.PolicyRule, out *newer.PolicyRule, s conversion.Scope) error {
	SetDefaults_PolicyRule(in)
	if err := apihelpers.Convert_runtime_RawExtension_To_runtime_Object(legacyscheme.Scheme, &in.AttributeRestrictions, &out.AttributeRestrictions, s); err != nil {
		return err
	}

	out.APIGroups = in.APIGroups

	out.Resources = sets.String{}
	out.Resources.Insert(in.Resources...)

	out.Verbs = sets.String{}
	out.Verbs.Insert(in.Verbs...)

	out.ResourceNames = sets.NewString(in.ResourceNames...)

	out.NonResourceURLs = sets.NewString(in.NonResourceURLsSlice...)

	return nil
}

func Convert_authorization_PolicyRule_To_v1_PolicyRule(in *newer.PolicyRule, out *v1.PolicyRule, s conversion.Scope) error {
	if err := apihelpers.Convert_runtime_Object_To_runtime_RawExtension(legacyscheme.Scheme, &in.AttributeRestrictions, &out.AttributeRestrictions, s); err != nil {
		return err
	}

	out.APIGroups = in.APIGroups

	out.Resources = []string{}
	out.Resources = append(out.Resources, in.Resources.List()...)

	out.Verbs = []string{}
	out.Verbs = append(out.Verbs, in.Verbs.List()...)

	out.ResourceNames = in.ResourceNames.List()

	out.NonResourceURLsSlice = in.NonResourceURLs.List()

	return nil
}

func Convert_v1_RoleBinding_To_authorization_RoleBinding(in *v1.RoleBinding, out *newer.RoleBinding, s conversion.Scope) error {
	if err := autoConvert_v1_RoleBinding_To_authorization_RoleBinding(in, out, s); err != nil {
		return err
	}

	// if the users and groups fields are cleared, then respect only subjects.  The field was set in the DefaultConvert above
	if in.UserNames == nil && in.GroupNames == nil {
		return nil
	}

	out.Subjects = newer.BuildSubjects(in.UserNames, in.GroupNames)

	return nil
}

func Convert_authorization_RoleBinding_To_v1_RoleBinding(in *newer.RoleBinding, out *v1.RoleBinding, s conversion.Scope) error {
	if err := autoConvert_authorization_RoleBinding_To_v1_RoleBinding(in, out, s); err != nil {
		return err
	}

	out.UserNames, out.GroupNames = newer.StringSubjectsFor(in.Namespace, in.Subjects)

	return nil
}

func Convert_v1_ClusterRoleBinding_To_authorization_ClusterRoleBinding(in *v1.ClusterRoleBinding, out *newer.ClusterRoleBinding, s conversion.Scope) error {
	if err := autoConvert_v1_ClusterRoleBinding_To_authorization_ClusterRoleBinding(in, out, s); err != nil {
		return err
	}

	// if the users and groups fields are cleared, then respect only subjects.  The field was set in the DefaultConvert above
	if in.UserNames == nil && in.GroupNames == nil {
		return nil
	}

	out.Subjects = newer.BuildSubjects(in.UserNames, in.GroupNames)

	return nil
}

func Convert_authorization_ClusterRoleBinding_To_v1_ClusterRoleBinding(in *newer.ClusterRoleBinding, out *v1.ClusterRoleBinding, s conversion.Scope) error {
	if err := autoConvert_authorization_ClusterRoleBinding_To_v1_ClusterRoleBinding(in, out, s); err != nil {
		return err
	}

	out.UserNames, out.GroupNames = newer.StringSubjectsFor(in.Namespace, in.Subjects)

	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) error {
	err := scheme.AddConversionFuncs(
		Convert_v1_SubjectAccessReview_To_authorization_SubjectAccessReview,
		Convert_authorization_SubjectAccessReview_To_v1_SubjectAccessReview,
		Convert_v1_LocalSubjectAccessReview_To_authorization_LocalSubjectAccessReview,
		Convert_authorization_LocalSubjectAccessReview_To_v1_LocalSubjectAccessReview,
		Convert_v1_ResourceAccessReview_To_authorization_ResourceAccessReview,
		Convert_authorization_ResourceAccessReview_To_v1_ResourceAccessReview,
		Convert_v1_LocalResourceAccessReview_To_authorization_LocalResourceAccessReview,
		Convert_authorization_LocalResourceAccessReview_To_v1_LocalResourceAccessReview,
		Convert_v1_ResourceAccessReviewResponse_To_authorization_ResourceAccessReviewResponse,
		Convert_authorization_ResourceAccessReviewResponse_To_v1_ResourceAccessReviewResponse,
		Convert_v1_PolicyRule_To_authorization_PolicyRule,
		Convert_authorization_PolicyRule_To_v1_PolicyRule,
		Convert_v1_RoleBinding_To_authorization_RoleBinding,
		Convert_authorization_RoleBinding_To_v1_RoleBinding,
		Convert_v1_ClusterRoleBinding_To_authorization_ClusterRoleBinding,
		Convert_authorization_ClusterRoleBinding_To_v1_ClusterRoleBinding,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		return err
	}

	return nil
}

func addFieldSelectorKeyConversions(scheme *runtime.Scheme) error {
	return nil
}
