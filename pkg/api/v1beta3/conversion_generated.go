package v1beta3

// AUTO-GENERATED FUNCTIONS START HERE
import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationapiv1beta3 "github.com/openshift/origin/pkg/authorization/api/v1beta3"
	buildapi "github.com/openshift/origin/pkg/build/api"
	v1beta3 "github.com/openshift/origin/pkg/build/api/v1beta3"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapiv1beta3 "github.com/openshift/origin/pkg/deploy/api/v1beta3"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imageapiv1beta3 "github.com/openshift/origin/pkg/image/api/v1beta3"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	oauthapiv1beta3 "github.com/openshift/origin/pkg/oauth/api/v1beta3"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectapiv1beta3 "github.com/openshift/origin/pkg/project/api/v1beta3"
	routeapi "github.com/openshift/origin/pkg/route/api"
	routeapiv1beta3 "github.com/openshift/origin/pkg/route/api/v1beta3"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	sdnapiv1beta3 "github.com/openshift/origin/pkg/sdn/api/v1beta3"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templateapiv1beta3 "github.com/openshift/origin/pkg/template/api/v1beta3"
	userapi "github.com/openshift/origin/pkg/user/api"
	userapiv1beta3 "github.com/openshift/origin/pkg/user/api/v1beta3"
	api "k8s.io/kubernetes/pkg/api"
	resource "k8s.io/kubernetes/pkg/api/resource"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	apiv1beta3 "k8s.io/kubernetes/pkg/api/v1beta3"
	conversion "k8s.io/kubernetes/pkg/conversion"
	runtime "k8s.io/kubernetes/pkg/runtime"
	reflect "reflect"
)

func autoConvert_api_ClusterPolicy_To_v1beta3_ClusterPolicy(in *authorizationapi.ClusterPolicy, out *authorizationapiv1beta3.ClusterPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterPolicy))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Roles, &out.Roles, 0); err != nil {
		return err
	}
	return nil
}

func Convert_api_ClusterPolicy_To_v1beta3_ClusterPolicy(in *authorizationapi.ClusterPolicy, out *authorizationapiv1beta3.ClusterPolicy, s conversion.Scope) error {
	return autoConvert_api_ClusterPolicy_To_v1beta3_ClusterPolicy(in, out, s)
}

func autoConvert_api_ClusterPolicyBinding_To_v1beta3_ClusterPolicyBinding(in *authorizationapi.ClusterPolicyBinding, out *authorizationapiv1beta3.ClusterPolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterPolicyBinding))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ClusterPolicyBindingList_To_v1beta3_ClusterPolicyBindingList(in *authorizationapi.ClusterPolicyBindingList, out *authorizationapiv1beta3.ClusterPolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterPolicyBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1beta3.ClusterPolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_api_ClusterPolicyBinding_To_v1beta3_ClusterPolicyBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterPolicyBindingList_To_v1beta3_ClusterPolicyBindingList(in *authorizationapi.ClusterPolicyBindingList, out *authorizationapiv1beta3.ClusterPolicyBindingList, s conversion.Scope) error {
	return autoConvert_api_ClusterPolicyBindingList_To_v1beta3_ClusterPolicyBindingList(in, out, s)
}

func autoConvert_api_ClusterPolicyList_To_v1beta3_ClusterPolicyList(in *authorizationapi.ClusterPolicyList, out *authorizationapiv1beta3.ClusterPolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterPolicyList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1beta3.ClusterPolicy, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_api_ClusterPolicy_To_v1beta3_ClusterPolicy(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterPolicyList_To_v1beta3_ClusterPolicyList(in *authorizationapi.ClusterPolicyList, out *authorizationapiv1beta3.ClusterPolicyList, s conversion.Scope) error {
	return autoConvert_api_ClusterPolicyList_To_v1beta3_ClusterPolicyList(in, out, s)
}

func autoConvert_api_ClusterRole_To_v1beta3_ClusterRole(in *authorizationapi.ClusterRole, out *authorizationapiv1beta3.ClusterRole, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterRole))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]authorizationapiv1beta3.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := authorizationapiv1beta3.Convert_api_PolicyRule_To_v1beta3_PolicyRule(&in.Rules[i], &out.Rules[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_api_ClusterRole_To_v1beta3_ClusterRole(in *authorizationapi.ClusterRole, out *authorizationapiv1beta3.ClusterRole, s conversion.Scope) error {
	return autoConvert_api_ClusterRole_To_v1beta3_ClusterRole(in, out, s)
}

func autoConvert_api_ClusterRoleBinding_To_v1beta3_ClusterRoleBinding(in *authorizationapi.ClusterRoleBinding, out *authorizationapiv1beta3.ClusterRoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterRoleBinding))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		out.Subjects = make([]apiv1beta3.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ClusterRoleBindingList_To_v1beta3_ClusterRoleBindingList(in *authorizationapi.ClusterRoleBindingList, out *authorizationapiv1beta3.ClusterRoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterRoleBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1beta3.ClusterRoleBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_api_ClusterRoleBinding_To_v1beta3_ClusterRoleBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterRoleBindingList_To_v1beta3_ClusterRoleBindingList(in *authorizationapi.ClusterRoleBindingList, out *authorizationapiv1beta3.ClusterRoleBindingList, s conversion.Scope) error {
	return autoConvert_api_ClusterRoleBindingList_To_v1beta3_ClusterRoleBindingList(in, out, s)
}

func autoConvert_api_ClusterRoleList_To_v1beta3_ClusterRoleList(in *authorizationapi.ClusterRoleList, out *authorizationapiv1beta3.ClusterRoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterRoleList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1beta3.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_ClusterRole_To_v1beta3_ClusterRole(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterRoleList_To_v1beta3_ClusterRoleList(in *authorizationapi.ClusterRoleList, out *authorizationapiv1beta3.ClusterRoleList, s conversion.Scope) error {
	return autoConvert_api_ClusterRoleList_To_v1beta3_ClusterRoleList(in, out, s)
}

func autoConvert_api_IsPersonalSubjectAccessReview_To_v1beta3_IsPersonalSubjectAccessReview(in *authorizationapi.IsPersonalSubjectAccessReview, out *authorizationapiv1beta3.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.IsPersonalSubjectAccessReview))(in)
	}
	return nil
}

func Convert_api_IsPersonalSubjectAccessReview_To_v1beta3_IsPersonalSubjectAccessReview(in *authorizationapi.IsPersonalSubjectAccessReview, out *authorizationapiv1beta3.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoConvert_api_IsPersonalSubjectAccessReview_To_v1beta3_IsPersonalSubjectAccessReview(in, out, s)
}

func autoConvert_api_LocalResourceAccessReview_To_v1beta3_LocalResourceAccessReview(in *authorizationapi.LocalResourceAccessReview, out *authorizationapiv1beta3.LocalResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.LocalResourceAccessReview))(in)
	}
	// in.Action has no peer in out
	return nil
}

func autoConvert_api_LocalSubjectAccessReview_To_v1beta3_LocalSubjectAccessReview(in *authorizationapi.LocalSubjectAccessReview, out *authorizationapiv1beta3.LocalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.LocalSubjectAccessReview))(in)
	}
	// in.Action has no peer in out
	out.User = in.User
	// in.Groups has no peer in out
	return nil
}

func autoConvert_api_Policy_To_v1beta3_Policy(in *authorizationapi.Policy, out *authorizationapiv1beta3.Policy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.Policy))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Roles, &out.Roles, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_PolicyBinding_To_v1beta3_PolicyBinding(in *authorizationapi.PolicyBinding, out *authorizationapiv1beta3.PolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.PolicyBinding))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_PolicyBindingList_To_v1beta3_PolicyBindingList(in *authorizationapi.PolicyBindingList, out *authorizationapiv1beta3.PolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.PolicyBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1beta3.PolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_api_PolicyBinding_To_v1beta3_PolicyBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_PolicyBindingList_To_v1beta3_PolicyBindingList(in *authorizationapi.PolicyBindingList, out *authorizationapiv1beta3.PolicyBindingList, s conversion.Scope) error {
	return autoConvert_api_PolicyBindingList_To_v1beta3_PolicyBindingList(in, out, s)
}

func autoConvert_api_PolicyList_To_v1beta3_PolicyList(in *authorizationapi.PolicyList, out *authorizationapiv1beta3.PolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.PolicyList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1beta3.Policy, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_api_Policy_To_v1beta3_Policy(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_PolicyList_To_v1beta3_PolicyList(in *authorizationapi.PolicyList, out *authorizationapiv1beta3.PolicyList, s conversion.Scope) error {
	return autoConvert_api_PolicyList_To_v1beta3_PolicyList(in, out, s)
}

func autoConvert_api_PolicyRule_To_v1beta3_PolicyRule(in *authorizationapi.PolicyRule, out *authorizationapiv1beta3.PolicyRule, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.PolicyRule))(in)
	}
	// in.Verbs has no peer in out
	if err := runtime.Convert_runtime_Object_To_runtime_RawExtension(&in.AttributeRestrictions, &out.AttributeRestrictions, s); err != nil {
		return err
	}
	if in.APIGroups != nil {
		out.APIGroups = make([]string, len(in.APIGroups))
		for i := range in.APIGroups {
			out.APIGroups[i] = in.APIGroups[i]
		}
	} else {
		out.APIGroups = nil
	}
	// in.Resources has no peer in out
	// in.ResourceNames has no peer in out
	// in.NonResourceURLs has no peer in out
	return nil
}

func autoConvert_api_ResourceAccessReview_To_v1beta3_ResourceAccessReview(in *authorizationapi.ResourceAccessReview, out *authorizationapiv1beta3.ResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ResourceAccessReview))(in)
	}
	// in.Action has no peer in out
	return nil
}

func autoConvert_api_ResourceAccessReviewResponse_To_v1beta3_ResourceAccessReviewResponse(in *authorizationapi.ResourceAccessReviewResponse, out *authorizationapiv1beta3.ResourceAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ResourceAccessReviewResponse))(in)
	}
	out.Namespace = in.Namespace
	// in.Users has no peer in out
	// in.Groups has no peer in out
	return nil
}

func autoConvert_api_Role_To_v1beta3_Role(in *authorizationapi.Role, out *authorizationapiv1beta3.Role, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.Role))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]authorizationapiv1beta3.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := authorizationapiv1beta3.Convert_api_PolicyRule_To_v1beta3_PolicyRule(&in.Rules[i], &out.Rules[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_api_Role_To_v1beta3_Role(in *authorizationapi.Role, out *authorizationapiv1beta3.Role, s conversion.Scope) error {
	return autoConvert_api_Role_To_v1beta3_Role(in, out, s)
}

func autoConvert_api_RoleBinding_To_v1beta3_RoleBinding(in *authorizationapi.RoleBinding, out *authorizationapiv1beta3.RoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.RoleBinding))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		out.Subjects = make([]apiv1beta3.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_RoleBindingList_To_v1beta3_RoleBindingList(in *authorizationapi.RoleBindingList, out *authorizationapiv1beta3.RoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.RoleBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1beta3.RoleBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_api_RoleBinding_To_v1beta3_RoleBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_RoleBindingList_To_v1beta3_RoleBindingList(in *authorizationapi.RoleBindingList, out *authorizationapiv1beta3.RoleBindingList, s conversion.Scope) error {
	return autoConvert_api_RoleBindingList_To_v1beta3_RoleBindingList(in, out, s)
}

func autoConvert_api_RoleList_To_v1beta3_RoleList(in *authorizationapi.RoleList, out *authorizationapiv1beta3.RoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.RoleList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1beta3.Role, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Role_To_v1beta3_Role(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_RoleList_To_v1beta3_RoleList(in *authorizationapi.RoleList, out *authorizationapiv1beta3.RoleList, s conversion.Scope) error {
	return autoConvert_api_RoleList_To_v1beta3_RoleList(in, out, s)
}

func autoConvert_api_SubjectAccessReview_To_v1beta3_SubjectAccessReview(in *authorizationapi.SubjectAccessReview, out *authorizationapiv1beta3.SubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.SubjectAccessReview))(in)
	}
	// in.Action has no peer in out
	out.User = in.User
	// in.Groups has no peer in out
	return nil
}

func autoConvert_api_SubjectAccessReviewResponse_To_v1beta3_SubjectAccessReviewResponse(in *authorizationapi.SubjectAccessReviewResponse, out *authorizationapiv1beta3.SubjectAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.SubjectAccessReviewResponse))(in)
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func Convert_api_SubjectAccessReviewResponse_To_v1beta3_SubjectAccessReviewResponse(in *authorizationapi.SubjectAccessReviewResponse, out *authorizationapiv1beta3.SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoConvert_api_SubjectAccessReviewResponse_To_v1beta3_SubjectAccessReviewResponse(in, out, s)
}

func autoConvert_v1beta3_ClusterPolicy_To_api_ClusterPolicy(in *authorizationapiv1beta3.ClusterPolicy, out *authorizationapi.ClusterPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ClusterPolicy))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Roles, &out.Roles, 0); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_ClusterPolicy_To_api_ClusterPolicy(in *authorizationapiv1beta3.ClusterPolicy, out *authorizationapi.ClusterPolicy, s conversion.Scope) error {
	return autoConvert_v1beta3_ClusterPolicy_To_api_ClusterPolicy(in, out, s)
}

func autoConvert_v1beta3_ClusterPolicyBinding_To_api_ClusterPolicyBinding(in *authorizationapiv1beta3.ClusterPolicyBinding, out *authorizationapi.ClusterPolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ClusterPolicyBinding))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta3_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *authorizationapiv1beta3.ClusterPolicyBindingList, out *authorizationapi.ClusterPolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ClusterPolicyBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.ClusterPolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_v1beta3_ClusterPolicyBinding_To_api_ClusterPolicyBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *authorizationapiv1beta3.ClusterPolicyBindingList, out *authorizationapi.ClusterPolicyBindingList, s conversion.Scope) error {
	return autoConvert_v1beta3_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in, out, s)
}

func autoConvert_v1beta3_ClusterPolicyList_To_api_ClusterPolicyList(in *authorizationapiv1beta3.ClusterPolicyList, out *authorizationapi.ClusterPolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ClusterPolicyList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.ClusterPolicy, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_ClusterPolicy_To_api_ClusterPolicy(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_ClusterPolicyList_To_api_ClusterPolicyList(in *authorizationapiv1beta3.ClusterPolicyList, out *authorizationapi.ClusterPolicyList, s conversion.Scope) error {
	return autoConvert_v1beta3_ClusterPolicyList_To_api_ClusterPolicyList(in, out, s)
}

func autoConvert_v1beta3_ClusterRole_To_api_ClusterRole(in *authorizationapiv1beta3.ClusterRole, out *authorizationapi.ClusterRole, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ClusterRole))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]authorizationapi.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := authorizationapiv1beta3.Convert_v1beta3_PolicyRule_To_api_PolicyRule(&in.Rules[i], &out.Rules[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_v1beta3_ClusterRole_To_api_ClusterRole(in *authorizationapiv1beta3.ClusterRole, out *authorizationapi.ClusterRole, s conversion.Scope) error {
	return autoConvert_v1beta3_ClusterRole_To_api_ClusterRole(in, out, s)
}

func autoConvert_v1beta3_ClusterRoleBinding_To_api_ClusterRoleBinding(in *authorizationapiv1beta3.ClusterRoleBinding, out *authorizationapi.ClusterRoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ClusterRoleBinding))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.UserNames has no peer in out
	// in.GroupNames has no peer in out
	if in.Subjects != nil {
		out.Subjects = make([]api.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta3_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *authorizationapiv1beta3.ClusterRoleBindingList, out *authorizationapi.ClusterRoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ClusterRoleBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.ClusterRoleBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_v1beta3_ClusterRoleBinding_To_api_ClusterRoleBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *authorizationapiv1beta3.ClusterRoleBindingList, out *authorizationapi.ClusterRoleBindingList, s conversion.Scope) error {
	return autoConvert_v1beta3_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in, out, s)
}

func autoConvert_v1beta3_ClusterRoleList_To_api_ClusterRoleList(in *authorizationapiv1beta3.ClusterRoleList, out *authorizationapi.ClusterRoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ClusterRoleList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_ClusterRole_To_api_ClusterRole(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_ClusterRoleList_To_api_ClusterRoleList(in *authorizationapiv1beta3.ClusterRoleList, out *authorizationapi.ClusterRoleList, s conversion.Scope) error {
	return autoConvert_v1beta3_ClusterRoleList_To_api_ClusterRoleList(in, out, s)
}

func autoConvert_v1beta3_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *authorizationapiv1beta3.IsPersonalSubjectAccessReview, out *authorizationapi.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.IsPersonalSubjectAccessReview))(in)
	}
	return nil
}

func Convert_v1beta3_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *authorizationapiv1beta3.IsPersonalSubjectAccessReview, out *authorizationapi.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoConvert_v1beta3_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in, out, s)
}

func autoConvert_v1beta3_LocalResourceAccessReview_To_api_LocalResourceAccessReview(in *authorizationapiv1beta3.LocalResourceAccessReview, out *authorizationapi.LocalResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.LocalResourceAccessReview))(in)
	}
	// in.AuthorizationAttributes has no peer in out
	return nil
}

func autoConvert_v1beta3_LocalSubjectAccessReview_To_api_LocalSubjectAccessReview(in *authorizationapiv1beta3.LocalSubjectAccessReview, out *authorizationapi.LocalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.LocalSubjectAccessReview))(in)
	}
	// in.AuthorizationAttributes has no peer in out
	out.User = in.User
	// in.GroupsSlice has no peer in out
	return nil
}

func autoConvert_v1beta3_Policy_To_api_Policy(in *authorizationapiv1beta3.Policy, out *authorizationapi.Policy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.Policy))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Roles, &out.Roles, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta3_PolicyBinding_To_api_PolicyBinding(in *authorizationapiv1beta3.PolicyBinding, out *authorizationapi.PolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.PolicyBinding))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta3_PolicyBindingList_To_api_PolicyBindingList(in *authorizationapiv1beta3.PolicyBindingList, out *authorizationapi.PolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.PolicyBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.PolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_v1beta3_PolicyBinding_To_api_PolicyBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_PolicyBindingList_To_api_PolicyBindingList(in *authorizationapiv1beta3.PolicyBindingList, out *authorizationapi.PolicyBindingList, s conversion.Scope) error {
	return autoConvert_v1beta3_PolicyBindingList_To_api_PolicyBindingList(in, out, s)
}

func autoConvert_v1beta3_PolicyList_To_api_PolicyList(in *authorizationapiv1beta3.PolicyList, out *authorizationapi.PolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.PolicyList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.Policy, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_v1beta3_Policy_To_api_Policy(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_PolicyList_To_api_PolicyList(in *authorizationapiv1beta3.PolicyList, out *authorizationapi.PolicyList, s conversion.Scope) error {
	return autoConvert_v1beta3_PolicyList_To_api_PolicyList(in, out, s)
}

func autoConvert_v1beta3_PolicyRule_To_api_PolicyRule(in *authorizationapiv1beta3.PolicyRule, out *authorizationapi.PolicyRule, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.PolicyRule))(in)
	}
	// in.Verbs has no peer in out
	if err := runtime.Convert_runtime_RawExtension_To_runtime_Object(&in.AttributeRestrictions, &out.AttributeRestrictions, s); err != nil {
		return err
	}
	if in.APIGroups != nil {
		out.APIGroups = make([]string, len(in.APIGroups))
		for i := range in.APIGroups {
			out.APIGroups[i] = in.APIGroups[i]
		}
	} else {
		out.APIGroups = nil
	}
	// in.ResourceKinds has no peer in out
	// in.Resources has no peer in out
	// in.ResourceNames has no peer in out
	// in.NonResourceURLsSlice has no peer in out
	return nil
}

func autoConvert_v1beta3_ResourceAccessReview_To_api_ResourceAccessReview(in *authorizationapiv1beta3.ResourceAccessReview, out *authorizationapi.ResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ResourceAccessReview))(in)
	}
	// in.AuthorizationAttributes has no peer in out
	return nil
}

func autoConvert_v1beta3_ResourceAccessReviewResponse_To_api_ResourceAccessReviewResponse(in *authorizationapiv1beta3.ResourceAccessReviewResponse, out *authorizationapi.ResourceAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.ResourceAccessReviewResponse))(in)
	}
	out.Namespace = in.Namespace
	// in.UsersSlice has no peer in out
	// in.GroupsSlice has no peer in out
	return nil
}

func autoConvert_v1beta3_Role_To_api_Role(in *authorizationapiv1beta3.Role, out *authorizationapi.Role, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.Role))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]authorizationapi.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := authorizationapiv1beta3.Convert_v1beta3_PolicyRule_To_api_PolicyRule(&in.Rules[i], &out.Rules[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_v1beta3_Role_To_api_Role(in *authorizationapiv1beta3.Role, out *authorizationapi.Role, s conversion.Scope) error {
	return autoConvert_v1beta3_Role_To_api_Role(in, out, s)
}

func autoConvert_v1beta3_RoleBinding_To_api_RoleBinding(in *authorizationapiv1beta3.RoleBinding, out *authorizationapi.RoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.RoleBinding))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.UserNames has no peer in out
	// in.GroupNames has no peer in out
	if in.Subjects != nil {
		out.Subjects = make([]api.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta3_RoleBindingList_To_api_RoleBindingList(in *authorizationapiv1beta3.RoleBindingList, out *authorizationapi.RoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.RoleBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.RoleBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1beta3.Convert_v1beta3_RoleBinding_To_api_RoleBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_RoleBindingList_To_api_RoleBindingList(in *authorizationapiv1beta3.RoleBindingList, out *authorizationapi.RoleBindingList, s conversion.Scope) error {
	return autoConvert_v1beta3_RoleBindingList_To_api_RoleBindingList(in, out, s)
}

func autoConvert_v1beta3_RoleList_To_api_RoleList(in *authorizationapiv1beta3.RoleList, out *authorizationapi.RoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.RoleList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.Role, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_Role_To_api_Role(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_RoleList_To_api_RoleList(in *authorizationapiv1beta3.RoleList, out *authorizationapi.RoleList, s conversion.Scope) error {
	return autoConvert_v1beta3_RoleList_To_api_RoleList(in, out, s)
}

func autoConvert_v1beta3_SubjectAccessReview_To_api_SubjectAccessReview(in *authorizationapiv1beta3.SubjectAccessReview, out *authorizationapi.SubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.SubjectAccessReview))(in)
	}
	// in.AuthorizationAttributes has no peer in out
	out.User = in.User
	// in.GroupsSlice has no peer in out
	return nil
}

func autoConvert_v1beta3_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *authorizationapiv1beta3.SubjectAccessReviewResponse, out *authorizationapi.SubjectAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1beta3.SubjectAccessReviewResponse))(in)
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func Convert_v1beta3_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *authorizationapiv1beta3.SubjectAccessReviewResponse, out *authorizationapi.SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoConvert_v1beta3_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in, out, s)
}

func autoConvert_api_BinaryBuildRequestOptions_To_v1beta3_BinaryBuildRequestOptions(in *buildapi.BinaryBuildRequestOptions, out *v1beta3.BinaryBuildRequestOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BinaryBuildRequestOptions))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.AsFile = in.AsFile
	out.Commit = in.Commit
	out.Message = in.Message
	out.AuthorName = in.AuthorName
	out.AuthorEmail = in.AuthorEmail
	out.CommitterName = in.CommitterName
	out.CommitterEmail = in.CommitterEmail
	return nil
}

func Convert_api_BinaryBuildRequestOptions_To_v1beta3_BinaryBuildRequestOptions(in *buildapi.BinaryBuildRequestOptions, out *v1beta3.BinaryBuildRequestOptions, s conversion.Scope) error {
	return autoConvert_api_BinaryBuildRequestOptions_To_v1beta3_BinaryBuildRequestOptions(in, out, s)
}

func autoConvert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource(in *buildapi.BinaryBuildSource, out *v1beta3.BinaryBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BinaryBuildSource))(in)
	}
	out.AsFile = in.AsFile
	return nil
}

func Convert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource(in *buildapi.BinaryBuildSource, out *v1beta3.BinaryBuildSource, s conversion.Scope) error {
	return autoConvert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource(in, out, s)
}

func autoConvert_api_Build_To_v1beta3_Build(in *buildapi.Build, out *v1beta3.Build, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.Build))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_BuildSpec_To_v1beta3_BuildSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_BuildStatus_To_v1beta3_BuildStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_Build_To_v1beta3_Build(in *buildapi.Build, out *v1beta3.Build, s conversion.Scope) error {
	return autoConvert_api_Build_To_v1beta3_Build(in, out, s)
}

func autoConvert_api_BuildConfig_To_v1beta3_BuildConfig(in *buildapi.BuildConfig, out *v1beta3.BuildConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfig))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_BuildConfigList_To_v1beta3_BuildConfigList(in *buildapi.BuildConfigList, out *v1beta3.BuildConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := v1beta3.Convert_api_BuildConfig_To_v1beta3_BuildConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_BuildConfigList_To_v1beta3_BuildConfigList(in *buildapi.BuildConfigList, out *v1beta3.BuildConfigList, s conversion.Scope) error {
	return autoConvert_api_BuildConfigList_To_v1beta3_BuildConfigList(in, out, s)
}

func autoConvert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec(in *buildapi.BuildConfigSpec, out *v1beta3.BuildConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigSpec))(in)
	}
	if in.Triggers != nil {
		out.Triggers = make([]v1beta3.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := v1beta3.Convert_api_BuildTriggerPolicy_To_v1beta3_BuildTriggerPolicy(&in.Triggers[i], &out.Triggers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := Convert_api_BuildSpec_To_v1beta3_BuildSpec(&in.BuildSpec, &out.BuildSpec, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec(in *buildapi.BuildConfigSpec, out *v1beta3.BuildConfigSpec, s conversion.Scope) error {
	return autoConvert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec(in, out, s)
}

func autoConvert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus(in *buildapi.BuildConfigStatus, out *v1beta3.BuildConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigStatus))(in)
	}
	out.LastVersion = in.LastVersion
	return nil
}

func Convert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus(in *buildapi.BuildConfigStatus, out *v1beta3.BuildConfigStatus, s conversion.Scope) error {
	return autoConvert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus(in, out, s)
}

func autoConvert_api_BuildList_To_v1beta3_BuildList(in *buildapi.BuildList, out *v1beta3.BuildList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.Build, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Build_To_v1beta3_Build(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_BuildList_To_v1beta3_BuildList(in *buildapi.BuildList, out *v1beta3.BuildList, s conversion.Scope) error {
	return autoConvert_api_BuildList_To_v1beta3_BuildList(in, out, s)
}

func autoConvert_api_BuildLog_To_v1beta3_BuildLog(in *buildapi.BuildLog, out *v1beta3.BuildLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildLog))(in)
	}
	return nil
}

func Convert_api_BuildLog_To_v1beta3_BuildLog(in *buildapi.BuildLog, out *v1beta3.BuildLog, s conversion.Scope) error {
	return autoConvert_api_BuildLog_To_v1beta3_BuildLog(in, out, s)
}

func autoConvert_api_BuildLogOptions_To_v1beta3_BuildLogOptions(in *buildapi.BuildLogOptions, out *v1beta3.BuildLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildLogOptions))(in)
	}
	out.Container = in.Container
	out.Follow = in.Follow
	out.Previous = in.Previous
	if in.SinceSeconds != nil {
		out.SinceSeconds = new(int64)
		*out.SinceSeconds = *in.SinceSeconds
	} else {
		out.SinceSeconds = nil
	}
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.SinceTime != nil {
		out.SinceTime = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.SinceTime, out.SinceTime, s); err != nil {
			return err
		}
	} else {
		out.SinceTime = nil
	}
	out.Timestamps = in.Timestamps
	if in.TailLines != nil {
		out.TailLines = new(int64)
		*out.TailLines = *in.TailLines
	} else {
		out.TailLines = nil
	}
	if in.LimitBytes != nil {
		out.LimitBytes = new(int64)
		*out.LimitBytes = *in.LimitBytes
	} else {
		out.LimitBytes = nil
	}
	out.NoWait = in.NoWait
	if in.Version != nil {
		out.Version = new(int64)
		*out.Version = *in.Version
	} else {
		out.Version = nil
	}
	return nil
}

func Convert_api_BuildLogOptions_To_v1beta3_BuildLogOptions(in *buildapi.BuildLogOptions, out *v1beta3.BuildLogOptions, s conversion.Scope) error {
	return autoConvert_api_BuildLogOptions_To_v1beta3_BuildLogOptions(in, out, s)
}

func autoConvert_api_BuildOutput_To_v1beta3_BuildOutput(in *buildapi.BuildOutput, out *v1beta3.BuildOutput, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildOutput))(in)
	}
	// unable to generate simple pointer conversion for api.ObjectReference -> v1beta3.ObjectReference
	if in.To != nil {
		out.To = new(apiv1beta3.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1beta3.LocalObjectReference
	if in.PushSecret != nil {
		out.PushSecret = new(apiv1beta3.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.PushSecret, out.PushSecret, s); err != nil {
			return err
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func autoConvert_api_BuildPostCommitSpec_To_v1beta3_BuildPostCommitSpec(in *buildapi.BuildPostCommitSpec, out *v1beta3.BuildPostCommitSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildPostCommitSpec))(in)
	}
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	if in.Args != nil {
		out.Args = make([]string, len(in.Args))
		for i := range in.Args {
			out.Args[i] = in.Args[i]
		}
	} else {
		out.Args = nil
	}
	out.Script = in.Script
	return nil
}

func Convert_api_BuildPostCommitSpec_To_v1beta3_BuildPostCommitSpec(in *buildapi.BuildPostCommitSpec, out *v1beta3.BuildPostCommitSpec, s conversion.Scope) error {
	return autoConvert_api_BuildPostCommitSpec_To_v1beta3_BuildPostCommitSpec(in, out, s)
}

func autoConvert_api_BuildSource_To_v1beta3_BuildSource(in *buildapi.BuildSource, out *v1beta3.BuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildSource))(in)
	}
	// unable to generate simple pointer conversion for api.BinaryBuildSource -> v1beta3.BinaryBuildSource
	if in.Binary != nil {
		out.Binary = new(v1beta3.BinaryBuildSource)
		if err := Convert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
			return err
		}
	} else {
		out.Binary = nil
	}
	if in.Dockerfile != nil {
		out.Dockerfile = new(string)
		*out.Dockerfile = *in.Dockerfile
	} else {
		out.Dockerfile = nil
	}
	// unable to generate simple pointer conversion for api.GitBuildSource -> v1beta3.GitBuildSource
	if in.Git != nil {
		out.Git = new(v1beta3.GitBuildSource)
		if err := Convert_api_GitBuildSource_To_v1beta3_GitBuildSource(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	if in.Images != nil {
		out.Images = make([]v1beta3.ImageSource, len(in.Images))
		for i := range in.Images {
			if err := Convert_api_ImageSource_To_v1beta3_ImageSource(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	out.ContextDir = in.ContextDir
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1beta3.LocalObjectReference
	if in.SourceSecret != nil {
		out.SourceSecret = new(apiv1beta3.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.SourceSecret, out.SourceSecret, s); err != nil {
			return err
		}
	} else {
		out.SourceSecret = nil
	}
	if in.Secrets != nil {
		out.Secrets = make([]v1beta3.SecretBuildSource, len(in.Secrets))
		for i := range in.Secrets {
			if err := Convert_api_SecretBuildSource_To_v1beta3_SecretBuildSource(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func autoConvert_api_BuildSpec_To_v1beta3_BuildSpec(in *buildapi.BuildSpec, out *v1beta3.BuildSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildSpec))(in)
	}
	out.ServiceAccount = in.ServiceAccount
	if err := v1beta3.Convert_api_BuildSource_To_v1beta3_BuildSource(&in.Source, &out.Source, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.SourceRevision -> v1beta3.SourceRevision
	if in.Revision != nil {
		out.Revision = new(v1beta3.SourceRevision)
		if err := v1beta3.Convert_api_SourceRevision_To_v1beta3_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := v1beta3.Convert_api_BuildStrategy_To_v1beta3_BuildStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if err := v1beta3.Convert_api_BuildOutput_To_v1beta3_BuildOutput(&in.Output, &out.Output, s); err != nil {
		return err
	}
	if err := Convert_api_ResourceRequirements_To_v1beta3_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if err := Convert_api_BuildPostCommitSpec_To_v1beta3_BuildPostCommitSpec(&in.PostCommit, &out.PostCommit, s); err != nil {
		return err
	}
	if in.CompletionDeadlineSeconds != nil {
		out.CompletionDeadlineSeconds = new(int64)
		*out.CompletionDeadlineSeconds = *in.CompletionDeadlineSeconds
	} else {
		out.CompletionDeadlineSeconds = nil
	}
	return nil
}

func Convert_api_BuildSpec_To_v1beta3_BuildSpec(in *buildapi.BuildSpec, out *v1beta3.BuildSpec, s conversion.Scope) error {
	return autoConvert_api_BuildSpec_To_v1beta3_BuildSpec(in, out, s)
}

func autoConvert_api_BuildStatus_To_v1beta3_BuildStatus(in *buildapi.BuildStatus, out *v1beta3.BuildStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildStatus))(in)
	}
	out.Phase = v1beta3.BuildPhase(in.Phase)
	out.Cancelled = in.Cancelled
	out.Reason = v1beta3.StatusReason(in.Reason)
	out.Message = in.Message
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.StartTimestamp != nil {
		out.StartTimestamp = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.StartTimestamp, out.StartTimestamp, s); err != nil {
			return err
		}
	} else {
		out.StartTimestamp = nil
	}
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.CompletionTimestamp != nil {
		out.CompletionTimestamp = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.CompletionTimestamp, out.CompletionTimestamp, s); err != nil {
			return err
		}
	} else {
		out.CompletionTimestamp = nil
	}
	out.Duration = in.Duration
	out.OutputDockerImageReference = in.OutputDockerImageReference
	// unable to generate simple pointer conversion for api.ObjectReference -> v1beta3.ObjectReference
	if in.Config != nil {
		out.Config = new(apiv1beta3.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(in.Config, out.Config, s); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	return nil
}

func Convert_api_BuildStatus_To_v1beta3_BuildStatus(in *buildapi.BuildStatus, out *v1beta3.BuildStatus, s conversion.Scope) error {
	return autoConvert_api_BuildStatus_To_v1beta3_BuildStatus(in, out, s)
}

func autoConvert_api_BuildStrategy_To_v1beta3_BuildStrategy(in *buildapi.BuildStrategy, out *v1beta3.BuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildStrategy))(in)
	}
	// unable to generate simple pointer conversion for api.DockerBuildStrategy -> v1beta3.DockerBuildStrategy
	if in.DockerStrategy != nil {
		out.DockerStrategy = new(v1beta3.DockerBuildStrategy)
		if err := v1beta3.Convert_api_DockerBuildStrategy_To_v1beta3_DockerBuildStrategy(in.DockerStrategy, out.DockerStrategy, s); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	// unable to generate simple pointer conversion for api.SourceBuildStrategy -> v1beta3.SourceBuildStrategy
	if in.SourceStrategy != nil {
		out.SourceStrategy = new(v1beta3.SourceBuildStrategy)
		if err := v1beta3.Convert_api_SourceBuildStrategy_To_v1beta3_SourceBuildStrategy(in.SourceStrategy, out.SourceStrategy, s); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	// unable to generate simple pointer conversion for api.CustomBuildStrategy -> v1beta3.CustomBuildStrategy
	if in.CustomStrategy != nil {
		out.CustomStrategy = new(v1beta3.CustomBuildStrategy)
		if err := v1beta3.Convert_api_CustomBuildStrategy_To_v1beta3_CustomBuildStrategy(in.CustomStrategy, out.CustomStrategy, s); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	return nil
}

func autoConvert_api_BuildTriggerPolicy_To_v1beta3_BuildTriggerPolicy(in *buildapi.BuildTriggerPolicy, out *v1beta3.BuildTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildTriggerPolicy))(in)
	}
	out.Type = v1beta3.BuildTriggerType(in.Type)
	// unable to generate simple pointer conversion for api.WebHookTrigger -> v1beta3.WebHookTrigger
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(v1beta3.WebHookTrigger)
		if err := Convert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in.GitHubWebHook, out.GitHubWebHook, s); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	// unable to generate simple pointer conversion for api.WebHookTrigger -> v1beta3.WebHookTrigger
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(v1beta3.WebHookTrigger)
		if err := Convert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in.GenericWebHook, out.GenericWebHook, s); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	// unable to generate simple pointer conversion for api.ImageChangeTrigger -> v1beta3.ImageChangeTrigger
	if in.ImageChange != nil {
		out.ImageChange = new(v1beta3.ImageChangeTrigger)
		if err := Convert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger(in.ImageChange, out.ImageChange, s); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func autoConvert_api_CustomBuildStrategy_To_v1beta3_CustomBuildStrategy(in *buildapi.CustomBuildStrategy, out *v1beta3.CustomBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.CustomBuildStrategy))(in)
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1beta3.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(apiv1beta3.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]apiv1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := s.Convert(&in.Env[i], &out.Env[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ExposeDockerSocket = in.ExposeDockerSocket
	out.ForcePull = in.ForcePull
	if in.Secrets != nil {
		out.Secrets = make([]v1beta3.SecretSpec, len(in.Secrets))
		for i := range in.Secrets {
			if err := Convert_api_SecretSpec_To_v1beta3_SecretSpec(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	out.BuildAPIVersion = in.BuildAPIVersion
	return nil
}

func autoConvert_api_DockerBuildStrategy_To_v1beta3_DockerBuildStrategy(in *buildapi.DockerBuildStrategy, out *v1beta3.DockerBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.DockerBuildStrategy))(in)
	}
	// unable to generate simple pointer conversion for api.ObjectReference -> v1beta3.ObjectReference
	if in.From != nil {
		out.From = new(apiv1beta3.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1beta3.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(apiv1beta3.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]apiv1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := s.Convert(&in.Env[i], &out.Env[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ForcePull = in.ForcePull
	out.DockerfilePath = in.DockerfilePath
	return nil
}

func autoConvert_api_GitBuildSource_To_v1beta3_GitBuildSource(in *buildapi.GitBuildSource, out *v1beta3.GitBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.GitBuildSource))(in)
	}
	out.URI = in.URI
	out.Ref = in.Ref
	if in.HTTPProxy != nil {
		out.HTTPProxy = new(string)
		*out.HTTPProxy = *in.HTTPProxy
	} else {
		out.HTTPProxy = nil
	}
	if in.HTTPSProxy != nil {
		out.HTTPSProxy = new(string)
		*out.HTTPSProxy = *in.HTTPSProxy
	} else {
		out.HTTPSProxy = nil
	}
	return nil
}

func Convert_api_GitBuildSource_To_v1beta3_GitBuildSource(in *buildapi.GitBuildSource, out *v1beta3.GitBuildSource, s conversion.Scope) error {
	return autoConvert_api_GitBuildSource_To_v1beta3_GitBuildSource(in, out, s)
}

func autoConvert_api_GitSourceRevision_To_v1beta3_GitSourceRevision(in *buildapi.GitSourceRevision, out *v1beta3.GitSourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.GitSourceRevision))(in)
	}
	out.Commit = in.Commit
	if err := Convert_api_SourceControlUser_To_v1beta3_SourceControlUser(&in.Author, &out.Author, s); err != nil {
		return err
	}
	if err := Convert_api_SourceControlUser_To_v1beta3_SourceControlUser(&in.Committer, &out.Committer, s); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func Convert_api_GitSourceRevision_To_v1beta3_GitSourceRevision(in *buildapi.GitSourceRevision, out *v1beta3.GitSourceRevision, s conversion.Scope) error {
	return autoConvert_api_GitSourceRevision_To_v1beta3_GitSourceRevision(in, out, s)
}

func autoConvert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger(in *buildapi.ImageChangeTrigger, out *v1beta3.ImageChangeTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageChangeTrigger))(in)
	}
	out.LastTriggeredImageID = in.LastTriggeredImageID
	// unable to generate simple pointer conversion for api.ObjectReference -> v1beta3.ObjectReference
	if in.From != nil {
		out.From = new(apiv1beta3.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	return nil
}

func Convert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger(in *buildapi.ImageChangeTrigger, out *v1beta3.ImageChangeTrigger, s conversion.Scope) error {
	return autoConvert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger(in, out, s)
}

func autoConvert_api_ImageSource_To_v1beta3_ImageSource(in *buildapi.ImageSource, out *v1beta3.ImageSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageSource))(in)
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.Paths != nil {
		out.Paths = make([]v1beta3.ImageSourcePath, len(in.Paths))
		for i := range in.Paths {
			if err := Convert_api_ImageSourcePath_To_v1beta3_ImageSourcePath(&in.Paths[i], &out.Paths[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Paths = nil
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1beta3.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(apiv1beta3.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	return nil
}

func Convert_api_ImageSource_To_v1beta3_ImageSource(in *buildapi.ImageSource, out *v1beta3.ImageSource, s conversion.Scope) error {
	return autoConvert_api_ImageSource_To_v1beta3_ImageSource(in, out, s)
}

func autoConvert_api_ImageSourcePath_To_v1beta3_ImageSourcePath(in *buildapi.ImageSourcePath, out *v1beta3.ImageSourcePath, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageSourcePath))(in)
	}
	out.SourcePath = in.SourcePath
	out.DestinationDir = in.DestinationDir
	return nil
}

func Convert_api_ImageSourcePath_To_v1beta3_ImageSourcePath(in *buildapi.ImageSourcePath, out *v1beta3.ImageSourcePath, s conversion.Scope) error {
	return autoConvert_api_ImageSourcePath_To_v1beta3_ImageSourcePath(in, out, s)
}

func autoConvert_api_SecretBuildSource_To_v1beta3_SecretBuildSource(in *buildapi.SecretBuildSource, out *v1beta3.SecretBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SecretBuildSource))(in)
	}
	if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(&in.Secret, &out.Secret, s); err != nil {
		return err
	}
	out.DestinationDir = in.DestinationDir
	return nil
}

func Convert_api_SecretBuildSource_To_v1beta3_SecretBuildSource(in *buildapi.SecretBuildSource, out *v1beta3.SecretBuildSource, s conversion.Scope) error {
	return autoConvert_api_SecretBuildSource_To_v1beta3_SecretBuildSource(in, out, s)
}

func autoConvert_api_SecretSpec_To_v1beta3_SecretSpec(in *buildapi.SecretSpec, out *v1beta3.SecretSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SecretSpec))(in)
	}
	if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(&in.SecretSource, &out.SecretSource, s); err != nil {
		return err
	}
	out.MountPath = in.MountPath
	return nil
}

func Convert_api_SecretSpec_To_v1beta3_SecretSpec(in *buildapi.SecretSpec, out *v1beta3.SecretSpec, s conversion.Scope) error {
	return autoConvert_api_SecretSpec_To_v1beta3_SecretSpec(in, out, s)
}

func autoConvert_api_SourceBuildStrategy_To_v1beta3_SourceBuildStrategy(in *buildapi.SourceBuildStrategy, out *v1beta3.SourceBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceBuildStrategy))(in)
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1beta3.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(apiv1beta3.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]apiv1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := s.Convert(&in.Env[i], &out.Env[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.Scripts = in.Scripts
	out.Incremental = in.Incremental
	out.ForcePull = in.ForcePull
	return nil
}

func autoConvert_api_SourceControlUser_To_v1beta3_SourceControlUser(in *buildapi.SourceControlUser, out *v1beta3.SourceControlUser, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceControlUser))(in)
	}
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func Convert_api_SourceControlUser_To_v1beta3_SourceControlUser(in *buildapi.SourceControlUser, out *v1beta3.SourceControlUser, s conversion.Scope) error {
	return autoConvert_api_SourceControlUser_To_v1beta3_SourceControlUser(in, out, s)
}

func autoConvert_api_SourceRevision_To_v1beta3_SourceRevision(in *buildapi.SourceRevision, out *v1beta3.SourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceRevision))(in)
	}
	// unable to generate simple pointer conversion for api.GitSourceRevision -> v1beta3.GitSourceRevision
	if in.Git != nil {
		out.Git = new(v1beta3.GitSourceRevision)
		if err := Convert_api_GitSourceRevision_To_v1beta3_GitSourceRevision(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func autoConvert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in *buildapi.WebHookTrigger, out *v1beta3.WebHookTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.WebHookTrigger))(in)
	}
	out.Secret = in.Secret
	return nil
}

func Convert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in *buildapi.WebHookTrigger, out *v1beta3.WebHookTrigger, s conversion.Scope) error {
	return autoConvert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in, out, s)
}

func autoConvert_v1beta3_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in *v1beta3.BinaryBuildRequestOptions, out *buildapi.BinaryBuildRequestOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BinaryBuildRequestOptions))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.AsFile = in.AsFile
	out.Commit = in.Commit
	out.Message = in.Message
	out.AuthorName = in.AuthorName
	out.AuthorEmail = in.AuthorEmail
	out.CommitterName = in.CommitterName
	out.CommitterEmail = in.CommitterEmail
	return nil
}

func Convert_v1beta3_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in *v1beta3.BinaryBuildRequestOptions, out *buildapi.BinaryBuildRequestOptions, s conversion.Scope) error {
	return autoConvert_v1beta3_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in, out, s)
}

func autoConvert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource(in *v1beta3.BinaryBuildSource, out *buildapi.BinaryBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BinaryBuildSource))(in)
	}
	out.AsFile = in.AsFile
	return nil
}

func Convert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource(in *v1beta3.BinaryBuildSource, out *buildapi.BinaryBuildSource, s conversion.Scope) error {
	return autoConvert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource(in, out, s)
}

func autoConvert_v1beta3_Build_To_api_Build(in *v1beta3.Build, out *buildapi.Build, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.Build))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_BuildSpec_To_api_BuildSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_BuildStatus_To_api_BuildStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_Build_To_api_Build(in *v1beta3.Build, out *buildapi.Build, s conversion.Scope) error {
	return autoConvert_v1beta3_Build_To_api_Build(in, out, s)
}

func autoConvert_v1beta3_BuildConfig_To_api_BuildConfig(in *v1beta3.BuildConfig, out *buildapi.BuildConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildConfig))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta3_BuildConfigList_To_api_BuildConfigList(in *v1beta3.BuildConfigList, out *buildapi.BuildConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildConfigList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]buildapi.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := v1beta3.Convert_v1beta3_BuildConfig_To_api_BuildConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_BuildConfigList_To_api_BuildConfigList(in *v1beta3.BuildConfigList, out *buildapi.BuildConfigList, s conversion.Scope) error {
	return autoConvert_v1beta3_BuildConfigList_To_api_BuildConfigList(in, out, s)
}

func autoConvert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec(in *v1beta3.BuildConfigSpec, out *buildapi.BuildConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildConfigSpec))(in)
	}
	if in.Triggers != nil {
		out.Triggers = make([]buildapi.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := v1beta3.Convert_v1beta3_BuildTriggerPolicy_To_api_BuildTriggerPolicy(&in.Triggers[i], &out.Triggers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := Convert_v1beta3_BuildSpec_To_api_BuildSpec(&in.BuildSpec, &out.BuildSpec, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec(in *v1beta3.BuildConfigSpec, out *buildapi.BuildConfigSpec, s conversion.Scope) error {
	return autoConvert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec(in, out, s)
}

func autoConvert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus(in *v1beta3.BuildConfigStatus, out *buildapi.BuildConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildConfigStatus))(in)
	}
	out.LastVersion = in.LastVersion
	return nil
}

func Convert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus(in *v1beta3.BuildConfigStatus, out *buildapi.BuildConfigStatus, s conversion.Scope) error {
	return autoConvert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus(in, out, s)
}

func autoConvert_v1beta3_BuildList_To_api_BuildList(in *v1beta3.BuildList, out *buildapi.BuildList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]buildapi.Build, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_Build_To_api_Build(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_BuildList_To_api_BuildList(in *v1beta3.BuildList, out *buildapi.BuildList, s conversion.Scope) error {
	return autoConvert_v1beta3_BuildList_To_api_BuildList(in, out, s)
}

func autoConvert_v1beta3_BuildLog_To_api_BuildLog(in *v1beta3.BuildLog, out *buildapi.BuildLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildLog))(in)
	}
	return nil
}

func Convert_v1beta3_BuildLog_To_api_BuildLog(in *v1beta3.BuildLog, out *buildapi.BuildLog, s conversion.Scope) error {
	return autoConvert_v1beta3_BuildLog_To_api_BuildLog(in, out, s)
}

func autoConvert_v1beta3_BuildLogOptions_To_api_BuildLogOptions(in *v1beta3.BuildLogOptions, out *buildapi.BuildLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildLogOptions))(in)
	}
	out.Container = in.Container
	out.Follow = in.Follow
	out.Previous = in.Previous
	if in.SinceSeconds != nil {
		out.SinceSeconds = new(int64)
		*out.SinceSeconds = *in.SinceSeconds
	} else {
		out.SinceSeconds = nil
	}
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.SinceTime != nil {
		out.SinceTime = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.SinceTime, out.SinceTime, s); err != nil {
			return err
		}
	} else {
		out.SinceTime = nil
	}
	out.Timestamps = in.Timestamps
	if in.TailLines != nil {
		out.TailLines = new(int64)
		*out.TailLines = *in.TailLines
	} else {
		out.TailLines = nil
	}
	if in.LimitBytes != nil {
		out.LimitBytes = new(int64)
		*out.LimitBytes = *in.LimitBytes
	} else {
		out.LimitBytes = nil
	}
	out.NoWait = in.NoWait
	if in.Version != nil {
		out.Version = new(int64)
		*out.Version = *in.Version
	} else {
		out.Version = nil
	}
	return nil
}

func Convert_v1beta3_BuildLogOptions_To_api_BuildLogOptions(in *v1beta3.BuildLogOptions, out *buildapi.BuildLogOptions, s conversion.Scope) error {
	return autoConvert_v1beta3_BuildLogOptions_To_api_BuildLogOptions(in, out, s)
}

func autoConvert_v1beta3_BuildOutput_To_api_BuildOutput(in *v1beta3.BuildOutput, out *buildapi.BuildOutput, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildOutput))(in)
	}
	// unable to generate simple pointer conversion for v1beta3.ObjectReference -> api.ObjectReference
	if in.To != nil {
		out.To = new(api.ObjectReference)
		if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	// unable to generate simple pointer conversion for v1beta3.LocalObjectReference -> api.LocalObjectReference
	if in.PushSecret != nil {
		out.PushSecret = new(api.LocalObjectReference)
		if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.PushSecret, out.PushSecret, s); err != nil {
			return err
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func autoConvert_v1beta3_BuildPostCommitSpec_To_api_BuildPostCommitSpec(in *v1beta3.BuildPostCommitSpec, out *buildapi.BuildPostCommitSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildPostCommitSpec))(in)
	}
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	if in.Args != nil {
		out.Args = make([]string, len(in.Args))
		for i := range in.Args {
			out.Args[i] = in.Args[i]
		}
	} else {
		out.Args = nil
	}
	out.Script = in.Script
	return nil
}

func Convert_v1beta3_BuildPostCommitSpec_To_api_BuildPostCommitSpec(in *v1beta3.BuildPostCommitSpec, out *buildapi.BuildPostCommitSpec, s conversion.Scope) error {
	return autoConvert_v1beta3_BuildPostCommitSpec_To_api_BuildPostCommitSpec(in, out, s)
}

func autoConvert_v1beta3_BuildSource_To_api_BuildSource(in *v1beta3.BuildSource, out *buildapi.BuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildSource))(in)
	}
	// in.Type has no peer in out
	// unable to generate simple pointer conversion for v1beta3.BinaryBuildSource -> api.BinaryBuildSource
	if in.Binary != nil {
		out.Binary = new(buildapi.BinaryBuildSource)
		if err := Convert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
			return err
		}
	} else {
		out.Binary = nil
	}
	if in.Dockerfile != nil {
		out.Dockerfile = new(string)
		*out.Dockerfile = *in.Dockerfile
	} else {
		out.Dockerfile = nil
	}
	// unable to generate simple pointer conversion for v1beta3.GitBuildSource -> api.GitBuildSource
	if in.Git != nil {
		out.Git = new(buildapi.GitBuildSource)
		if err := Convert_v1beta3_GitBuildSource_To_api_GitBuildSource(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	if in.Images != nil {
		out.Images = make([]buildapi.ImageSource, len(in.Images))
		for i := range in.Images {
			if err := Convert_v1beta3_ImageSource_To_api_ImageSource(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	out.ContextDir = in.ContextDir
	// unable to generate simple pointer conversion for v1beta3.LocalObjectReference -> api.LocalObjectReference
	if in.SourceSecret != nil {
		out.SourceSecret = new(api.LocalObjectReference)
		if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.SourceSecret, out.SourceSecret, s); err != nil {
			return err
		}
	} else {
		out.SourceSecret = nil
	}
	if in.Secrets != nil {
		out.Secrets = make([]buildapi.SecretBuildSource, len(in.Secrets))
		for i := range in.Secrets {
			if err := Convert_v1beta3_SecretBuildSource_To_api_SecretBuildSource(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func autoConvert_v1beta3_BuildSpec_To_api_BuildSpec(in *v1beta3.BuildSpec, out *buildapi.BuildSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildSpec))(in)
	}
	out.ServiceAccount = in.ServiceAccount
	if err := v1beta3.Convert_v1beta3_BuildSource_To_api_BuildSource(&in.Source, &out.Source, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1beta3.SourceRevision -> api.SourceRevision
	if in.Revision != nil {
		out.Revision = new(buildapi.SourceRevision)
		if err := v1beta3.Convert_v1beta3_SourceRevision_To_api_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := v1beta3.Convert_v1beta3_BuildStrategy_To_api_BuildStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if err := v1beta3.Convert_v1beta3_BuildOutput_To_api_BuildOutput(&in.Output, &out.Output, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_ResourceRequirements_To_api_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_BuildPostCommitSpec_To_api_BuildPostCommitSpec(&in.PostCommit, &out.PostCommit, s); err != nil {
		return err
	}
	if in.CompletionDeadlineSeconds != nil {
		out.CompletionDeadlineSeconds = new(int64)
		*out.CompletionDeadlineSeconds = *in.CompletionDeadlineSeconds
	} else {
		out.CompletionDeadlineSeconds = nil
	}
	return nil
}

func Convert_v1beta3_BuildSpec_To_api_BuildSpec(in *v1beta3.BuildSpec, out *buildapi.BuildSpec, s conversion.Scope) error {
	return autoConvert_v1beta3_BuildSpec_To_api_BuildSpec(in, out, s)
}

func autoConvert_v1beta3_BuildStatus_To_api_BuildStatus(in *v1beta3.BuildStatus, out *buildapi.BuildStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildStatus))(in)
	}
	out.Phase = buildapi.BuildPhase(in.Phase)
	out.Cancelled = in.Cancelled
	out.Reason = buildapi.StatusReason(in.Reason)
	out.Message = in.Message
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.StartTimestamp != nil {
		out.StartTimestamp = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.StartTimestamp, out.StartTimestamp, s); err != nil {
			return err
		}
	} else {
		out.StartTimestamp = nil
	}
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.CompletionTimestamp != nil {
		out.CompletionTimestamp = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.CompletionTimestamp, out.CompletionTimestamp, s); err != nil {
			return err
		}
	} else {
		out.CompletionTimestamp = nil
	}
	out.Duration = in.Duration
	out.OutputDockerImageReference = in.OutputDockerImageReference
	// unable to generate simple pointer conversion for v1beta3.ObjectReference -> api.ObjectReference
	if in.Config != nil {
		out.Config = new(api.ObjectReference)
		if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(in.Config, out.Config, s); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	return nil
}

func Convert_v1beta3_BuildStatus_To_api_BuildStatus(in *v1beta3.BuildStatus, out *buildapi.BuildStatus, s conversion.Scope) error {
	return autoConvert_v1beta3_BuildStatus_To_api_BuildStatus(in, out, s)
}

func autoConvert_v1beta3_BuildStrategy_To_api_BuildStrategy(in *v1beta3.BuildStrategy, out *buildapi.BuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildStrategy))(in)
	}
	// in.Type has no peer in out
	// unable to generate simple pointer conversion for v1beta3.DockerBuildStrategy -> api.DockerBuildStrategy
	if in.DockerStrategy != nil {
		out.DockerStrategy = new(buildapi.DockerBuildStrategy)
		if err := v1beta3.Convert_v1beta3_DockerBuildStrategy_To_api_DockerBuildStrategy(in.DockerStrategy, out.DockerStrategy, s); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	// unable to generate simple pointer conversion for v1beta3.SourceBuildStrategy -> api.SourceBuildStrategy
	if in.SourceStrategy != nil {
		out.SourceStrategy = new(buildapi.SourceBuildStrategy)
		if err := v1beta3.Convert_v1beta3_SourceBuildStrategy_To_api_SourceBuildStrategy(in.SourceStrategy, out.SourceStrategy, s); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	// unable to generate simple pointer conversion for v1beta3.CustomBuildStrategy -> api.CustomBuildStrategy
	if in.CustomStrategy != nil {
		out.CustomStrategy = new(buildapi.CustomBuildStrategy)
		if err := v1beta3.Convert_v1beta3_CustomBuildStrategy_To_api_CustomBuildStrategy(in.CustomStrategy, out.CustomStrategy, s); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	return nil
}

func autoConvert_v1beta3_BuildTriggerPolicy_To_api_BuildTriggerPolicy(in *v1beta3.BuildTriggerPolicy, out *buildapi.BuildTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.BuildTriggerPolicy))(in)
	}
	out.Type = buildapi.BuildTriggerType(in.Type)
	// unable to generate simple pointer conversion for v1beta3.WebHookTrigger -> api.WebHookTrigger
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(buildapi.WebHookTrigger)
		if err := Convert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in.GitHubWebHook, out.GitHubWebHook, s); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	// unable to generate simple pointer conversion for v1beta3.WebHookTrigger -> api.WebHookTrigger
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(buildapi.WebHookTrigger)
		if err := Convert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in.GenericWebHook, out.GenericWebHook, s); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	// unable to generate simple pointer conversion for v1beta3.ImageChangeTrigger -> api.ImageChangeTrigger
	if in.ImageChange != nil {
		out.ImageChange = new(buildapi.ImageChangeTrigger)
		if err := Convert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger(in.ImageChange, out.ImageChange, s); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func autoConvert_v1beta3_CustomBuildStrategy_To_api_CustomBuildStrategy(in *v1beta3.CustomBuildStrategy, out *buildapi.CustomBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.CustomBuildStrategy))(in)
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1beta3.LocalObjectReference -> api.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(api.LocalObjectReference)
		if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := s.Convert(&in.Env[i], &out.Env[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ExposeDockerSocket = in.ExposeDockerSocket
	out.ForcePull = in.ForcePull
	if in.Secrets != nil {
		out.Secrets = make([]buildapi.SecretSpec, len(in.Secrets))
		for i := range in.Secrets {
			if err := Convert_v1beta3_SecretSpec_To_api_SecretSpec(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	out.BuildAPIVersion = in.BuildAPIVersion
	return nil
}

func autoConvert_v1beta3_DockerBuildStrategy_To_api_DockerBuildStrategy(in *v1beta3.DockerBuildStrategy, out *buildapi.DockerBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.DockerBuildStrategy))(in)
	}
	// unable to generate simple pointer conversion for v1beta3.ObjectReference -> api.ObjectReference
	if in.From != nil {
		out.From = new(api.ObjectReference)
		if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	// unable to generate simple pointer conversion for v1beta3.LocalObjectReference -> api.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(api.LocalObjectReference)
		if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := s.Convert(&in.Env[i], &out.Env[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ForcePull = in.ForcePull
	out.DockerfilePath = in.DockerfilePath
	return nil
}

func autoConvert_v1beta3_GitBuildSource_To_api_GitBuildSource(in *v1beta3.GitBuildSource, out *buildapi.GitBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.GitBuildSource))(in)
	}
	out.URI = in.URI
	out.Ref = in.Ref
	if in.HTTPProxy != nil {
		out.HTTPProxy = new(string)
		*out.HTTPProxy = *in.HTTPProxy
	} else {
		out.HTTPProxy = nil
	}
	if in.HTTPSProxy != nil {
		out.HTTPSProxy = new(string)
		*out.HTTPSProxy = *in.HTTPSProxy
	} else {
		out.HTTPSProxy = nil
	}
	return nil
}

func Convert_v1beta3_GitBuildSource_To_api_GitBuildSource(in *v1beta3.GitBuildSource, out *buildapi.GitBuildSource, s conversion.Scope) error {
	return autoConvert_v1beta3_GitBuildSource_To_api_GitBuildSource(in, out, s)
}

func autoConvert_v1beta3_GitSourceRevision_To_api_GitSourceRevision(in *v1beta3.GitSourceRevision, out *buildapi.GitSourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.GitSourceRevision))(in)
	}
	out.Commit = in.Commit
	if err := Convert_v1beta3_SourceControlUser_To_api_SourceControlUser(&in.Author, &out.Author, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_SourceControlUser_To_api_SourceControlUser(&in.Committer, &out.Committer, s); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func Convert_v1beta3_GitSourceRevision_To_api_GitSourceRevision(in *v1beta3.GitSourceRevision, out *buildapi.GitSourceRevision, s conversion.Scope) error {
	return autoConvert_v1beta3_GitSourceRevision_To_api_GitSourceRevision(in, out, s)
}

func autoConvert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger(in *v1beta3.ImageChangeTrigger, out *buildapi.ImageChangeTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ImageChangeTrigger))(in)
	}
	out.LastTriggeredImageID = in.LastTriggeredImageID
	// unable to generate simple pointer conversion for v1beta3.ObjectReference -> api.ObjectReference
	if in.From != nil {
		out.From = new(api.ObjectReference)
		if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	return nil
}

func Convert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger(in *v1beta3.ImageChangeTrigger, out *buildapi.ImageChangeTrigger, s conversion.Scope) error {
	return autoConvert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger(in, out, s)
}

func autoConvert_v1beta3_ImageSource_To_api_ImageSource(in *v1beta3.ImageSource, out *buildapi.ImageSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ImageSource))(in)
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.Paths != nil {
		out.Paths = make([]buildapi.ImageSourcePath, len(in.Paths))
		for i := range in.Paths {
			if err := Convert_v1beta3_ImageSourcePath_To_api_ImageSourcePath(&in.Paths[i], &out.Paths[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Paths = nil
	}
	// unable to generate simple pointer conversion for v1beta3.LocalObjectReference -> api.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(api.LocalObjectReference)
		if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	return nil
}

func Convert_v1beta3_ImageSource_To_api_ImageSource(in *v1beta3.ImageSource, out *buildapi.ImageSource, s conversion.Scope) error {
	return autoConvert_v1beta3_ImageSource_To_api_ImageSource(in, out, s)
}

func autoConvert_v1beta3_ImageSourcePath_To_api_ImageSourcePath(in *v1beta3.ImageSourcePath, out *buildapi.ImageSourcePath, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ImageSourcePath))(in)
	}
	out.SourcePath = in.SourcePath
	out.DestinationDir = in.DestinationDir
	return nil
}

func Convert_v1beta3_ImageSourcePath_To_api_ImageSourcePath(in *v1beta3.ImageSourcePath, out *buildapi.ImageSourcePath, s conversion.Scope) error {
	return autoConvert_v1beta3_ImageSourcePath_To_api_ImageSourcePath(in, out, s)
}

func autoConvert_v1beta3_SecretBuildSource_To_api_SecretBuildSource(in *v1beta3.SecretBuildSource, out *buildapi.SecretBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.SecretBuildSource))(in)
	}
	if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(&in.Secret, &out.Secret, s); err != nil {
		return err
	}
	out.DestinationDir = in.DestinationDir
	return nil
}

func Convert_v1beta3_SecretBuildSource_To_api_SecretBuildSource(in *v1beta3.SecretBuildSource, out *buildapi.SecretBuildSource, s conversion.Scope) error {
	return autoConvert_v1beta3_SecretBuildSource_To_api_SecretBuildSource(in, out, s)
}

func autoConvert_v1beta3_SecretSpec_To_api_SecretSpec(in *v1beta3.SecretSpec, out *buildapi.SecretSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.SecretSpec))(in)
	}
	if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(&in.SecretSource, &out.SecretSource, s); err != nil {
		return err
	}
	out.MountPath = in.MountPath
	return nil
}

func Convert_v1beta3_SecretSpec_To_api_SecretSpec(in *v1beta3.SecretSpec, out *buildapi.SecretSpec, s conversion.Scope) error {
	return autoConvert_v1beta3_SecretSpec_To_api_SecretSpec(in, out, s)
}

func autoConvert_v1beta3_SourceBuildStrategy_To_api_SourceBuildStrategy(in *v1beta3.SourceBuildStrategy, out *buildapi.SourceBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.SourceBuildStrategy))(in)
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1beta3.LocalObjectReference -> api.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(api.LocalObjectReference)
		if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := s.Convert(&in.Env[i], &out.Env[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.Scripts = in.Scripts
	out.Incremental = in.Incremental
	out.ForcePull = in.ForcePull
	return nil
}

func autoConvert_v1beta3_SourceControlUser_To_api_SourceControlUser(in *v1beta3.SourceControlUser, out *buildapi.SourceControlUser, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.SourceControlUser))(in)
	}
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func Convert_v1beta3_SourceControlUser_To_api_SourceControlUser(in *v1beta3.SourceControlUser, out *buildapi.SourceControlUser, s conversion.Scope) error {
	return autoConvert_v1beta3_SourceControlUser_To_api_SourceControlUser(in, out, s)
}

func autoConvert_v1beta3_SourceRevision_To_api_SourceRevision(in *v1beta3.SourceRevision, out *buildapi.SourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.SourceRevision))(in)
	}
	// in.Type has no peer in out
	// unable to generate simple pointer conversion for v1beta3.GitSourceRevision -> api.GitSourceRevision
	if in.Git != nil {
		out.Git = new(buildapi.GitSourceRevision)
		if err := Convert_v1beta3_GitSourceRevision_To_api_GitSourceRevision(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func autoConvert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in *v1beta3.WebHookTrigger, out *buildapi.WebHookTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.WebHookTrigger))(in)
	}
	out.Secret = in.Secret
	return nil
}

func Convert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in *v1beta3.WebHookTrigger, out *buildapi.WebHookTrigger, s conversion.Scope) error {
	return autoConvert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in, out, s)
}

func autoConvert_api_DeploymentCause_To_v1beta3_DeploymentCause(in *deployapi.DeploymentCause, out *deployapiv1beta3.DeploymentCause, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentCause))(in)
	}
	out.Type = deployapiv1beta3.DeploymentTriggerType(in.Type)
	// unable to generate simple pointer conversion for api.DeploymentCauseImageTrigger -> v1beta3.DeploymentCauseImageTrigger
	if in.ImageTrigger != nil {
		out.ImageTrigger = new(deployapiv1beta3.DeploymentCauseImageTrigger)
		if err := Convert_api_DeploymentCauseImageTrigger_To_v1beta3_DeploymentCauseImageTrigger(in.ImageTrigger, out.ImageTrigger, s); err != nil {
			return err
		}
	} else {
		out.ImageTrigger = nil
	}
	return nil
}

func Convert_api_DeploymentCause_To_v1beta3_DeploymentCause(in *deployapi.DeploymentCause, out *deployapiv1beta3.DeploymentCause, s conversion.Scope) error {
	return autoConvert_api_DeploymentCause_To_v1beta3_DeploymentCause(in, out, s)
}

func autoConvert_api_DeploymentCauseImageTrigger_To_v1beta3_DeploymentCauseImageTrigger(in *deployapi.DeploymentCauseImageTrigger, out *deployapiv1beta3.DeploymentCauseImageTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentCauseImageTrigger))(in)
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_DeploymentCauseImageTrigger_To_v1beta3_DeploymentCauseImageTrigger(in *deployapi.DeploymentCauseImageTrigger, out *deployapiv1beta3.DeploymentCauseImageTrigger, s conversion.Scope) error {
	return autoConvert_api_DeploymentCauseImageTrigger_To_v1beta3_DeploymentCauseImageTrigger(in, out, s)
}

func autoConvert_api_DeploymentConfigRollback_To_v1beta3_DeploymentConfigRollback(in *deployapi.DeploymentConfigRollback, out *deployapiv1beta3.DeploymentConfigRollback, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigRollback))(in)
	}
	if err := Convert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_DeploymentConfigRollback_To_v1beta3_DeploymentConfigRollback(in *deployapi.DeploymentConfigRollback, out *deployapiv1beta3.DeploymentConfigRollback, s conversion.Scope) error {
	return autoConvert_api_DeploymentConfigRollback_To_v1beta3_DeploymentConfigRollback(in, out, s)
}

func autoConvert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec(in *deployapi.DeploymentConfigRollbackSpec, out *deployapiv1beta3.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigRollbackSpec))(in)
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func Convert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec(in *deployapi.DeploymentConfigRollbackSpec, out *deployapiv1beta3.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	return autoConvert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec(in, out, s)
}

func autoConvert_api_DeploymentDetails_To_v1beta3_DeploymentDetails(in *deployapi.DeploymentDetails, out *deployapiv1beta3.DeploymentDetails, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentDetails))(in)
	}
	out.Message = in.Message
	if in.Causes != nil {
		out.Causes = make([]deployapiv1beta3.DeploymentCause, len(in.Causes))
		for i := range in.Causes {
			if err := Convert_api_DeploymentCause_To_v1beta3_DeploymentCause(&in.Causes[i], &out.Causes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Causes = nil
	}
	return nil
}

func Convert_api_DeploymentDetails_To_v1beta3_DeploymentDetails(in *deployapi.DeploymentDetails, out *deployapiv1beta3.DeploymentDetails, s conversion.Scope) error {
	return autoConvert_api_DeploymentDetails_To_v1beta3_DeploymentDetails(in, out, s)
}

func autoConvert_api_DeploymentLog_To_v1beta3_DeploymentLog(in *deployapi.DeploymentLog, out *deployapiv1beta3.DeploymentLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentLog))(in)
	}
	return nil
}

func Convert_api_DeploymentLog_To_v1beta3_DeploymentLog(in *deployapi.DeploymentLog, out *deployapiv1beta3.DeploymentLog, s conversion.Scope) error {
	return autoConvert_api_DeploymentLog_To_v1beta3_DeploymentLog(in, out, s)
}

func autoConvert_api_DeploymentLogOptions_To_v1beta3_DeploymentLogOptions(in *deployapi.DeploymentLogOptions, out *deployapiv1beta3.DeploymentLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentLogOptions))(in)
	}
	out.Container = in.Container
	out.Follow = in.Follow
	out.Previous = in.Previous
	if in.SinceSeconds != nil {
		out.SinceSeconds = new(int64)
		*out.SinceSeconds = *in.SinceSeconds
	} else {
		out.SinceSeconds = nil
	}
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.SinceTime != nil {
		out.SinceTime = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.SinceTime, out.SinceTime, s); err != nil {
			return err
		}
	} else {
		out.SinceTime = nil
	}
	out.Timestamps = in.Timestamps
	if in.TailLines != nil {
		out.TailLines = new(int64)
		*out.TailLines = *in.TailLines
	} else {
		out.TailLines = nil
	}
	if in.LimitBytes != nil {
		out.LimitBytes = new(int64)
		*out.LimitBytes = *in.LimitBytes
	} else {
		out.LimitBytes = nil
	}
	out.NoWait = in.NoWait
	if in.Version != nil {
		out.Version = new(int64)
		*out.Version = *in.Version
	} else {
		out.Version = nil
	}
	return nil
}

func Convert_api_DeploymentLogOptions_To_v1beta3_DeploymentLogOptions(in *deployapi.DeploymentLogOptions, out *deployapiv1beta3.DeploymentLogOptions, s conversion.Scope) error {
	return autoConvert_api_DeploymentLogOptions_To_v1beta3_DeploymentLogOptions(in, out, s)
}

func autoConvert_api_DeploymentTriggerImageChangeParams_To_v1beta3_DeploymentTriggerImageChangeParams(in *deployapi.DeploymentTriggerImageChangeParams, out *deployapiv1beta3.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentTriggerImageChangeParams))(in)
	}
	out.Automatic = in.Automatic
	if in.ContainerNames != nil {
		out.ContainerNames = make([]string, len(in.ContainerNames))
		for i := range in.ContainerNames {
			out.ContainerNames[i] = in.ContainerNames[i]
		}
	} else {
		out.ContainerNames = nil
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.LastTriggeredImage = in.LastTriggeredImage
	return nil
}

func autoConvert_api_DeploymentTriggerPolicy_To_v1beta3_DeploymentTriggerPolicy(in *deployapi.DeploymentTriggerPolicy, out *deployapiv1beta3.DeploymentTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentTriggerPolicy))(in)
	}
	out.Type = deployapiv1beta3.DeploymentTriggerType(in.Type)
	// unable to generate simple pointer conversion for api.DeploymentTriggerImageChangeParams -> v1beta3.DeploymentTriggerImageChangeParams
	if in.ImageChangeParams != nil {
		out.ImageChangeParams = new(deployapiv1beta3.DeploymentTriggerImageChangeParams)
		if err := deployapiv1beta3.Convert_api_DeploymentTriggerImageChangeParams_To_v1beta3_DeploymentTriggerImageChangeParams(in.ImageChangeParams, out.ImageChangeParams, s); err != nil {
			return err
		}
	} else {
		out.ImageChangeParams = nil
	}
	return nil
}

func Convert_api_DeploymentTriggerPolicy_To_v1beta3_DeploymentTriggerPolicy(in *deployapi.DeploymentTriggerPolicy, out *deployapiv1beta3.DeploymentTriggerPolicy, s conversion.Scope) error {
	return autoConvert_api_DeploymentTriggerPolicy_To_v1beta3_DeploymentTriggerPolicy(in, out, s)
}

func autoConvert_api_RollingDeploymentStrategyParams_To_v1beta3_RollingDeploymentStrategyParams(in *deployapi.RollingDeploymentStrategyParams, out *deployapiv1beta3.RollingDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.RollingDeploymentStrategyParams))(in)
	}
	if in.UpdatePeriodSeconds != nil {
		out.UpdatePeriodSeconds = new(int64)
		*out.UpdatePeriodSeconds = *in.UpdatePeriodSeconds
	} else {
		out.UpdatePeriodSeconds = nil
	}
	if in.IntervalSeconds != nil {
		out.IntervalSeconds = new(int64)
		*out.IntervalSeconds = *in.IntervalSeconds
	} else {
		out.IntervalSeconds = nil
	}
	if in.TimeoutSeconds != nil {
		out.TimeoutSeconds = new(int64)
		*out.TimeoutSeconds = *in.TimeoutSeconds
	} else {
		out.TimeoutSeconds = nil
	}
	if err := s.Convert(&in.MaxUnavailable, &out.MaxUnavailable, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.MaxSurge, &out.MaxSurge, 0); err != nil {
		return err
	}
	if in.UpdatePercent != nil {
		out.UpdatePercent = new(int)
		*out.UpdatePercent = *in.UpdatePercent
	} else {
		out.UpdatePercent = nil
	}
	// unable to generate simple pointer conversion for api.LifecycleHook -> v1beta3.LifecycleHook
	if in.Pre != nil {
		if err := s.Convert(&in.Pre, &out.Pre, 0); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	// unable to generate simple pointer conversion for api.LifecycleHook -> v1beta3.LifecycleHook
	if in.Post != nil {
		if err := s.Convert(&in.Post, &out.Post, 0); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func autoConvert_api_TagImageHook_To_v1beta3_TagImageHook(in *deployapi.TagImageHook, out *deployapiv1beta3.TagImageHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.TagImageHook))(in)
	}
	out.ContainerName = in.ContainerName
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_TagImageHook_To_v1beta3_TagImageHook(in *deployapi.TagImageHook, out *deployapiv1beta3.TagImageHook, s conversion.Scope) error {
	return autoConvert_api_TagImageHook_To_v1beta3_TagImageHook(in, out, s)
}

func autoConvert_v1beta3_DeploymentCause_To_api_DeploymentCause(in *deployapiv1beta3.DeploymentCause, out *deployapi.DeploymentCause, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentCause))(in)
	}
	out.Type = deployapi.DeploymentTriggerType(in.Type)
	// unable to generate simple pointer conversion for v1beta3.DeploymentCauseImageTrigger -> api.DeploymentCauseImageTrigger
	if in.ImageTrigger != nil {
		out.ImageTrigger = new(deployapi.DeploymentCauseImageTrigger)
		if err := Convert_v1beta3_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in.ImageTrigger, out.ImageTrigger, s); err != nil {
			return err
		}
	} else {
		out.ImageTrigger = nil
	}
	return nil
}

func Convert_v1beta3_DeploymentCause_To_api_DeploymentCause(in *deployapiv1beta3.DeploymentCause, out *deployapi.DeploymentCause, s conversion.Scope) error {
	return autoConvert_v1beta3_DeploymentCause_To_api_DeploymentCause(in, out, s)
}

func autoConvert_v1beta3_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in *deployapiv1beta3.DeploymentCauseImageTrigger, out *deployapi.DeploymentCauseImageTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentCauseImageTrigger))(in)
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in *deployapiv1beta3.DeploymentCauseImageTrigger, out *deployapi.DeploymentCauseImageTrigger, s conversion.Scope) error {
	return autoConvert_v1beta3_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in, out, s)
}

func autoConvert_v1beta3_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in *deployapiv1beta3.DeploymentConfigRollback, out *deployapi.DeploymentConfigRollback, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentConfigRollback))(in)
	}
	if err := Convert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in *deployapiv1beta3.DeploymentConfigRollback, out *deployapi.DeploymentConfigRollback, s conversion.Scope) error {
	return autoConvert_v1beta3_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in, out, s)
}

func autoConvert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in *deployapiv1beta3.DeploymentConfigRollbackSpec, out *deployapi.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentConfigRollbackSpec))(in)
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func Convert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in *deployapiv1beta3.DeploymentConfigRollbackSpec, out *deployapi.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	return autoConvert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in, out, s)
}

func autoConvert_v1beta3_DeploymentConfigStatus_To_api_DeploymentConfigStatus(in *deployapiv1beta3.DeploymentConfigStatus, out *deployapi.DeploymentConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentConfigStatus))(in)
	}
	out.LatestVersion = in.LatestVersion
	// unable to generate simple pointer conversion for v1beta3.DeploymentDetails -> api.DeploymentDetails
	if in.Details != nil {
		out.Details = new(deployapi.DeploymentDetails)
		if err := Convert_v1beta3_DeploymentDetails_To_api_DeploymentDetails(in.Details, out.Details, s); err != nil {
			return err
		}
	} else {
		out.Details = nil
	}
	return nil
}

func Convert_v1beta3_DeploymentConfigStatus_To_api_DeploymentConfigStatus(in *deployapiv1beta3.DeploymentConfigStatus, out *deployapi.DeploymentConfigStatus, s conversion.Scope) error {
	return autoConvert_v1beta3_DeploymentConfigStatus_To_api_DeploymentConfigStatus(in, out, s)
}

func autoConvert_v1beta3_DeploymentDetails_To_api_DeploymentDetails(in *deployapiv1beta3.DeploymentDetails, out *deployapi.DeploymentDetails, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentDetails))(in)
	}
	out.Message = in.Message
	if in.Causes != nil {
		out.Causes = make([]deployapi.DeploymentCause, len(in.Causes))
		for i := range in.Causes {
			if err := Convert_v1beta3_DeploymentCause_To_api_DeploymentCause(&in.Causes[i], &out.Causes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Causes = nil
	}
	return nil
}

func Convert_v1beta3_DeploymentDetails_To_api_DeploymentDetails(in *deployapiv1beta3.DeploymentDetails, out *deployapi.DeploymentDetails, s conversion.Scope) error {
	return autoConvert_v1beta3_DeploymentDetails_To_api_DeploymentDetails(in, out, s)
}

func autoConvert_v1beta3_DeploymentLog_To_api_DeploymentLog(in *deployapiv1beta3.DeploymentLog, out *deployapi.DeploymentLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentLog))(in)
	}
	return nil
}

func Convert_v1beta3_DeploymentLog_To_api_DeploymentLog(in *deployapiv1beta3.DeploymentLog, out *deployapi.DeploymentLog, s conversion.Scope) error {
	return autoConvert_v1beta3_DeploymentLog_To_api_DeploymentLog(in, out, s)
}

func autoConvert_v1beta3_DeploymentLogOptions_To_api_DeploymentLogOptions(in *deployapiv1beta3.DeploymentLogOptions, out *deployapi.DeploymentLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentLogOptions))(in)
	}
	out.Container = in.Container
	out.Follow = in.Follow
	out.Previous = in.Previous
	if in.SinceSeconds != nil {
		out.SinceSeconds = new(int64)
		*out.SinceSeconds = *in.SinceSeconds
	} else {
		out.SinceSeconds = nil
	}
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.SinceTime != nil {
		out.SinceTime = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.SinceTime, out.SinceTime, s); err != nil {
			return err
		}
	} else {
		out.SinceTime = nil
	}
	out.Timestamps = in.Timestamps
	if in.TailLines != nil {
		out.TailLines = new(int64)
		*out.TailLines = *in.TailLines
	} else {
		out.TailLines = nil
	}
	if in.LimitBytes != nil {
		out.LimitBytes = new(int64)
		*out.LimitBytes = *in.LimitBytes
	} else {
		out.LimitBytes = nil
	}
	out.NoWait = in.NoWait
	if in.Version != nil {
		out.Version = new(int64)
		*out.Version = *in.Version
	} else {
		out.Version = nil
	}
	return nil
}

func Convert_v1beta3_DeploymentLogOptions_To_api_DeploymentLogOptions(in *deployapiv1beta3.DeploymentLogOptions, out *deployapi.DeploymentLogOptions, s conversion.Scope) error {
	return autoConvert_v1beta3_DeploymentLogOptions_To_api_DeploymentLogOptions(in, out, s)
}

func autoConvert_v1beta3_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams(in *deployapiv1beta3.DeploymentTriggerImageChangeParams, out *deployapi.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentTriggerImageChangeParams))(in)
	}
	out.Automatic = in.Automatic
	if in.ContainerNames != nil {
		out.ContainerNames = make([]string, len(in.ContainerNames))
		for i := range in.ContainerNames {
			out.ContainerNames[i] = in.ContainerNames[i]
		}
	} else {
		out.ContainerNames = nil
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.LastTriggeredImage = in.LastTriggeredImage
	return nil
}

func autoConvert_v1beta3_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(in *deployapiv1beta3.DeploymentTriggerPolicy, out *deployapi.DeploymentTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentTriggerPolicy))(in)
	}
	out.Type = deployapi.DeploymentTriggerType(in.Type)
	// unable to generate simple pointer conversion for v1beta3.DeploymentTriggerImageChangeParams -> api.DeploymentTriggerImageChangeParams
	if in.ImageChangeParams != nil {
		out.ImageChangeParams = new(deployapi.DeploymentTriggerImageChangeParams)
		if err := deployapiv1beta3.Convert_v1beta3_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams(in.ImageChangeParams, out.ImageChangeParams, s); err != nil {
			return err
		}
	} else {
		out.ImageChangeParams = nil
	}
	return nil
}

func Convert_v1beta3_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(in *deployapiv1beta3.DeploymentTriggerPolicy, out *deployapi.DeploymentTriggerPolicy, s conversion.Scope) error {
	return autoConvert_v1beta3_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(in, out, s)
}

func autoConvert_v1beta3_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams(in *deployapiv1beta3.RollingDeploymentStrategyParams, out *deployapi.RollingDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.RollingDeploymentStrategyParams))(in)
	}
	if in.UpdatePeriodSeconds != nil {
		out.UpdatePeriodSeconds = new(int64)
		*out.UpdatePeriodSeconds = *in.UpdatePeriodSeconds
	} else {
		out.UpdatePeriodSeconds = nil
	}
	if in.IntervalSeconds != nil {
		out.IntervalSeconds = new(int64)
		*out.IntervalSeconds = *in.IntervalSeconds
	} else {
		out.IntervalSeconds = nil
	}
	if in.TimeoutSeconds != nil {
		out.TimeoutSeconds = new(int64)
		*out.TimeoutSeconds = *in.TimeoutSeconds
	} else {
		out.TimeoutSeconds = nil
	}
	// in.MaxUnavailable has no peer in out
	// in.MaxSurge has no peer in out
	if in.UpdatePercent != nil {
		out.UpdatePercent = new(int)
		*out.UpdatePercent = *in.UpdatePercent
	} else {
		out.UpdatePercent = nil
	}
	// unable to generate simple pointer conversion for v1beta3.LifecycleHook -> api.LifecycleHook
	if in.Pre != nil {
		if err := s.Convert(&in.Pre, &out.Pre, 0); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	// unable to generate simple pointer conversion for v1beta3.LifecycleHook -> api.LifecycleHook
	if in.Post != nil {
		if err := s.Convert(&in.Post, &out.Post, 0); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func autoConvert_v1beta3_TagImageHook_To_api_TagImageHook(in *deployapiv1beta3.TagImageHook, out *deployapi.TagImageHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.TagImageHook))(in)
	}
	out.ContainerName = in.ContainerName
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_TagImageHook_To_api_TagImageHook(in *deployapiv1beta3.TagImageHook, out *deployapi.TagImageHook, s conversion.Scope) error {
	return autoConvert_v1beta3_TagImageHook_To_api_TagImageHook(in, out, s)
}

func autoConvert_api_Image_To_v1beta3_Image(in *imageapi.Image, out *imageapiv1beta3.Image, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.Image))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DockerImageReference = in.DockerImageReference
	if err := s.Convert(&in.DockerImageMetadata, &out.DockerImageMetadata, 0); err != nil {
		return err
	}
	out.DockerImageMetadataVersion = in.DockerImageMetadataVersion
	out.DockerImageManifest = in.DockerImageManifest
	if in.DockerImageLayers != nil {
		out.DockerImageLayers = make([]imageapiv1beta3.ImageLayer, len(in.DockerImageLayers))
		for i := range in.DockerImageLayers {
			if err := s.Convert(&in.DockerImageLayers[i], &out.DockerImageLayers[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.DockerImageLayers = nil
	}
	return nil
}

func autoConvert_api_ImageList_To_v1beta3_ImageList(in *imageapi.ImageList, out *imageapiv1beta3.ImageList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1beta3.Image, len(in.Items))
		for i := range in.Items {
			if err := imageapiv1beta3.Convert_api_Image_To_v1beta3_Image(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ImageList_To_v1beta3_ImageList(in *imageapi.ImageList, out *imageapiv1beta3.ImageList, s conversion.Scope) error {
	return autoConvert_api_ImageList_To_v1beta3_ImageList(in, out, s)
}

func autoConvert_api_ImageStream_To_v1beta3_ImageStream(in *imageapi.ImageStream, out *imageapiv1beta3.ImageStream, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStream))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := imageapiv1beta3.Convert_api_ImageStreamSpec_To_v1beta3_ImageStreamSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := imageapiv1beta3.Convert_api_ImageStreamStatus_To_v1beta3_ImageStreamStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ImageStreamImage_To_v1beta3_ImageStreamImage(in *imageapi.ImageStreamImage, out *imageapiv1beta3.ImageStreamImage, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamImage))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := imageapiv1beta3.Convert_api_Image_To_v1beta3_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ImageStreamList_To_v1beta3_ImageStreamList(in *imageapi.ImageStreamList, out *imageapiv1beta3.ImageStreamList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1beta3.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := imageapiv1beta3.Convert_api_ImageStream_To_v1beta3_ImageStream(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ImageStreamList_To_v1beta3_ImageStreamList(in *imageapi.ImageStreamList, out *imageapiv1beta3.ImageStreamList, s conversion.Scope) error {
	return autoConvert_api_ImageStreamList_To_v1beta3_ImageStreamList(in, out, s)
}

func autoConvert_api_ImageStreamMapping_To_v1beta3_ImageStreamMapping(in *imageapi.ImageStreamMapping, out *imageapiv1beta3.ImageStreamMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamMapping))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.DockerImageRepository has no peer in out
	if err := imageapiv1beta3.Convert_api_Image_To_v1beta3_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func autoConvert_api_ImageStreamSpec_To_v1beta3_ImageStreamSpec(in *imageapi.ImageStreamSpec, out *imageapiv1beta3.ImageStreamSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamSpec))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ImageStreamStatus_To_v1beta3_ImageStreamStatus(in *imageapi.ImageStreamStatus, out *imageapiv1beta3.ImageStreamStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamStatus))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ImageStreamTag_To_v1beta3_ImageStreamTag(in *imageapi.ImageStreamTag, out *imageapiv1beta3.ImageStreamTag, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamTag))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.Tag has no peer in out
	out.Generation = in.Generation
	// in.Conditions has no peer in out
	if err := imageapiv1beta3.Convert_api_Image_To_v1beta3_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ImageStreamTagList_To_v1beta3_ImageStreamTagList(in *imageapi.ImageStreamTagList, out *imageapiv1beta3.ImageStreamTagList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamTagList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1beta3.ImageStreamTag, len(in.Items))
		for i := range in.Items {
			if err := imageapiv1beta3.Convert_api_ImageStreamTag_To_v1beta3_ImageStreamTag(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ImageStreamTagList_To_v1beta3_ImageStreamTagList(in *imageapi.ImageStreamTagList, out *imageapiv1beta3.ImageStreamTagList, s conversion.Scope) error {
	return autoConvert_api_ImageStreamTagList_To_v1beta3_ImageStreamTagList(in, out, s)
}

func autoConvert_v1beta3_Image_To_api_Image(in *imageapiv1beta3.Image, out *imageapi.Image, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.Image))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DockerImageReference = in.DockerImageReference
	if err := s.Convert(&in.DockerImageMetadata, &out.DockerImageMetadata, 0); err != nil {
		return err
	}
	out.DockerImageMetadataVersion = in.DockerImageMetadataVersion
	out.DockerImageManifest = in.DockerImageManifest
	if in.DockerImageLayers != nil {
		out.DockerImageLayers = make([]imageapi.ImageLayer, len(in.DockerImageLayers))
		for i := range in.DockerImageLayers {
			if err := s.Convert(&in.DockerImageLayers[i], &out.DockerImageLayers[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.DockerImageLayers = nil
	}
	return nil
}

func autoConvert_v1beta3_ImageList_To_api_ImageList(in *imageapiv1beta3.ImageList, out *imageapi.ImageList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapi.Image, len(in.Items))
		for i := range in.Items {
			if err := imageapiv1beta3.Convert_v1beta3_Image_To_api_Image(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_ImageList_To_api_ImageList(in *imageapiv1beta3.ImageList, out *imageapi.ImageList, s conversion.Scope) error {
	return autoConvert_v1beta3_ImageList_To_api_ImageList(in, out, s)
}

func autoConvert_v1beta3_ImageStream_To_api_ImageStream(in *imageapiv1beta3.ImageStream, out *imageapi.ImageStream, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStream))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := imageapiv1beta3.Convert_v1beta3_ImageStreamSpec_To_api_ImageStreamSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := imageapiv1beta3.Convert_v1beta3_ImageStreamStatus_To_api_ImageStreamStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta3_ImageStreamImage_To_api_ImageStreamImage(in *imageapiv1beta3.ImageStreamImage, out *imageapi.ImageStreamImage, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamImage))(in)
	}
	if err := imageapiv1beta3.Convert_v1beta3_Image_To_api_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	// in.ImageName has no peer in out
	return nil
}

func autoConvert_v1beta3_ImageStreamList_To_api_ImageStreamList(in *imageapiv1beta3.ImageStreamList, out *imageapi.ImageStreamList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapi.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := imageapiv1beta3.Convert_v1beta3_ImageStream_To_api_ImageStream(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_ImageStreamList_To_api_ImageStreamList(in *imageapiv1beta3.ImageStreamList, out *imageapi.ImageStreamList, s conversion.Scope) error {
	return autoConvert_v1beta3_ImageStreamList_To_api_ImageStreamList(in, out, s)
}

func autoConvert_v1beta3_ImageStreamMapping_To_api_ImageStreamMapping(in *imageapiv1beta3.ImageStreamMapping, out *imageapi.ImageStreamMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamMapping))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := imageapiv1beta3.Convert_v1beta3_Image_To_api_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func autoConvert_v1beta3_ImageStreamSpec_To_api_ImageStreamSpec(in *imageapiv1beta3.ImageStreamSpec, out *imageapi.ImageStreamSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamSpec))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta3_ImageStreamStatus_To_api_ImageStreamStatus(in *imageapiv1beta3.ImageStreamStatus, out *imageapi.ImageStreamStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamStatus))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1beta3_ImageStreamTag_To_api_ImageStreamTag(in *imageapiv1beta3.ImageStreamTag, out *imageapi.ImageStreamTag, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamTag))(in)
	}
	if err := imageapiv1beta3.Convert_v1beta3_Image_To_api_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	// in.ImageName has no peer in out
	return nil
}

func autoConvert_v1beta3_ImageStreamTagList_To_api_ImageStreamTagList(in *imageapiv1beta3.ImageStreamTagList, out *imageapi.ImageStreamTagList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamTagList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapi.ImageStreamTag, len(in.Items))
		for i := range in.Items {
			if err := imageapiv1beta3.Convert_v1beta3_ImageStreamTag_To_api_ImageStreamTag(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_ImageStreamTagList_To_api_ImageStreamTagList(in *imageapiv1beta3.ImageStreamTagList, out *imageapi.ImageStreamTagList, s conversion.Scope) error {
	return autoConvert_v1beta3_ImageStreamTagList_To_api_ImageStreamTagList(in, out, s)
}

func autoConvert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken(in *oauthapi.OAuthAccessToken, out *oauthapiv1beta3.OAuthAccessToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAccessToken))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ClientName = in.ClientName
	out.ExpiresIn = in.ExpiresIn
	if in.Scopes != nil {
		out.Scopes = make([]string, len(in.Scopes))
		for i := range in.Scopes {
			out.Scopes[i] = in.Scopes[i]
		}
	} else {
		out.Scopes = nil
	}
	out.RedirectURI = in.RedirectURI
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	out.AuthorizeToken = in.AuthorizeToken
	out.RefreshToken = in.RefreshToken
	return nil
}

func Convert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken(in *oauthapi.OAuthAccessToken, out *oauthapiv1beta3.OAuthAccessToken, s conversion.Scope) error {
	return autoConvert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken(in, out, s)
}

func autoConvert_api_OAuthAccessTokenList_To_v1beta3_OAuthAccessTokenList(in *oauthapi.OAuthAccessTokenList, out *oauthapiv1beta3.OAuthAccessTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAccessTokenList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_OAuthAccessTokenList_To_v1beta3_OAuthAccessTokenList(in *oauthapi.OAuthAccessTokenList, out *oauthapiv1beta3.OAuthAccessTokenList, s conversion.Scope) error {
	return autoConvert_api_OAuthAccessTokenList_To_v1beta3_OAuthAccessTokenList(in, out, s)
}

func autoConvert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken(in *oauthapi.OAuthAuthorizeToken, out *oauthapiv1beta3.OAuthAuthorizeToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAuthorizeToken))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ClientName = in.ClientName
	out.ExpiresIn = in.ExpiresIn
	if in.Scopes != nil {
		out.Scopes = make([]string, len(in.Scopes))
		for i := range in.Scopes {
			out.Scopes[i] = in.Scopes[i]
		}
	} else {
		out.Scopes = nil
	}
	out.RedirectURI = in.RedirectURI
	out.State = in.State
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	return nil
}

func Convert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken(in *oauthapi.OAuthAuthorizeToken, out *oauthapiv1beta3.OAuthAuthorizeToken, s conversion.Scope) error {
	return autoConvert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken(in, out, s)
}

func autoConvert_api_OAuthAuthorizeTokenList_To_v1beta3_OAuthAuthorizeTokenList(in *oauthapi.OAuthAuthorizeTokenList, out *oauthapiv1beta3.OAuthAuthorizeTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAuthorizeTokenList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_OAuthAuthorizeTokenList_To_v1beta3_OAuthAuthorizeTokenList(in *oauthapi.OAuthAuthorizeTokenList, out *oauthapiv1beta3.OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoConvert_api_OAuthAuthorizeTokenList_To_v1beta3_OAuthAuthorizeTokenList(in, out, s)
}

func autoConvert_api_OAuthClient_To_v1beta3_OAuthClient(in *oauthapi.OAuthClient, out *oauthapiv1beta3.OAuthClient, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClient))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Secret = in.Secret
	out.RespondWithChallenges = in.RespondWithChallenges
	if in.RedirectURIs != nil {
		out.RedirectURIs = make([]string, len(in.RedirectURIs))
		for i := range in.RedirectURIs {
			out.RedirectURIs[i] = in.RedirectURIs[i]
		}
	} else {
		out.RedirectURIs = nil
	}
	return nil
}

func Convert_api_OAuthClient_To_v1beta3_OAuthClient(in *oauthapi.OAuthClient, out *oauthapiv1beta3.OAuthClient, s conversion.Scope) error {
	return autoConvert_api_OAuthClient_To_v1beta3_OAuthClient(in, out, s)
}

func autoConvert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization(in *oauthapi.OAuthClientAuthorization, out *oauthapiv1beta3.OAuthClientAuthorization, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientAuthorization))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ClientName = in.ClientName
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	if in.Scopes != nil {
		out.Scopes = make([]string, len(in.Scopes))
		for i := range in.Scopes {
			out.Scopes[i] = in.Scopes[i]
		}
	} else {
		out.Scopes = nil
	}
	return nil
}

func Convert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization(in *oauthapi.OAuthClientAuthorization, out *oauthapiv1beta3.OAuthClientAuthorization, s conversion.Scope) error {
	return autoConvert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization(in, out, s)
}

func autoConvert_api_OAuthClientAuthorizationList_To_v1beta3_OAuthClientAuthorizationList(in *oauthapi.OAuthClientAuthorizationList, out *oauthapiv1beta3.OAuthClientAuthorizationList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientAuthorizationList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_OAuthClientAuthorizationList_To_v1beta3_OAuthClientAuthorizationList(in *oauthapi.OAuthClientAuthorizationList, out *oauthapiv1beta3.OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoConvert_api_OAuthClientAuthorizationList_To_v1beta3_OAuthClientAuthorizationList(in, out, s)
}

func autoConvert_api_OAuthClientList_To_v1beta3_OAuthClientList(in *oauthapi.OAuthClientList, out *oauthapiv1beta3.OAuthClientList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_OAuthClient_To_v1beta3_OAuthClient(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_OAuthClientList_To_v1beta3_OAuthClientList(in *oauthapi.OAuthClientList, out *oauthapiv1beta3.OAuthClientList, s conversion.Scope) error {
	return autoConvert_api_OAuthClientList_To_v1beta3_OAuthClientList(in, out, s)
}

func autoConvert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken(in *oauthapiv1beta3.OAuthAccessToken, out *oauthapi.OAuthAccessToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthAccessToken))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ClientName = in.ClientName
	out.ExpiresIn = in.ExpiresIn
	if in.Scopes != nil {
		out.Scopes = make([]string, len(in.Scopes))
		for i := range in.Scopes {
			out.Scopes[i] = in.Scopes[i]
		}
	} else {
		out.Scopes = nil
	}
	out.RedirectURI = in.RedirectURI
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	out.AuthorizeToken = in.AuthorizeToken
	out.RefreshToken = in.RefreshToken
	return nil
}

func Convert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken(in *oauthapiv1beta3.OAuthAccessToken, out *oauthapi.OAuthAccessToken, s conversion.Scope) error {
	return autoConvert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken(in, out, s)
}

func autoConvert_v1beta3_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *oauthapiv1beta3.OAuthAccessTokenList, out *oauthapi.OAuthAccessTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthAccessTokenList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *oauthapiv1beta3.OAuthAccessTokenList, out *oauthapi.OAuthAccessTokenList, s conversion.Scope) error {
	return autoConvert_v1beta3_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in, out, s)
}

func autoConvert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *oauthapiv1beta3.OAuthAuthorizeToken, out *oauthapi.OAuthAuthorizeToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthAuthorizeToken))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ClientName = in.ClientName
	out.ExpiresIn = in.ExpiresIn
	if in.Scopes != nil {
		out.Scopes = make([]string, len(in.Scopes))
		for i := range in.Scopes {
			out.Scopes[i] = in.Scopes[i]
		}
	} else {
		out.Scopes = nil
	}
	out.RedirectURI = in.RedirectURI
	out.State = in.State
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	return nil
}

func Convert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *oauthapiv1beta3.OAuthAuthorizeToken, out *oauthapi.OAuthAuthorizeToken, s conversion.Scope) error {
	return autoConvert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in, out, s)
}

func autoConvert_v1beta3_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *oauthapiv1beta3.OAuthAuthorizeTokenList, out *oauthapi.OAuthAuthorizeTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthAuthorizeTokenList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *oauthapiv1beta3.OAuthAuthorizeTokenList, out *oauthapi.OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoConvert_v1beta3_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in, out, s)
}

func autoConvert_v1beta3_OAuthClient_To_api_OAuthClient(in *oauthapiv1beta3.OAuthClient, out *oauthapi.OAuthClient, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthClient))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Secret = in.Secret
	out.RespondWithChallenges = in.RespondWithChallenges
	if in.RedirectURIs != nil {
		out.RedirectURIs = make([]string, len(in.RedirectURIs))
		for i := range in.RedirectURIs {
			out.RedirectURIs[i] = in.RedirectURIs[i]
		}
	} else {
		out.RedirectURIs = nil
	}
	return nil
}

func Convert_v1beta3_OAuthClient_To_api_OAuthClient(in *oauthapiv1beta3.OAuthClient, out *oauthapi.OAuthClient, s conversion.Scope) error {
	return autoConvert_v1beta3_OAuthClient_To_api_OAuthClient(in, out, s)
}

func autoConvert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *oauthapiv1beta3.OAuthClientAuthorization, out *oauthapi.OAuthClientAuthorization, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthClientAuthorization))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ClientName = in.ClientName
	out.UserName = in.UserName
	out.UserUID = in.UserUID
	if in.Scopes != nil {
		out.Scopes = make([]string, len(in.Scopes))
		for i := range in.Scopes {
			out.Scopes[i] = in.Scopes[i]
		}
	} else {
		out.Scopes = nil
	}
	return nil
}

func Convert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *oauthapiv1beta3.OAuthClientAuthorization, out *oauthapi.OAuthClientAuthorization, s conversion.Scope) error {
	return autoConvert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in, out, s)
}

func autoConvert_v1beta3_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *oauthapiv1beta3.OAuthClientAuthorizationList, out *oauthapi.OAuthClientAuthorizationList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthClientAuthorizationList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *oauthapiv1beta3.OAuthClientAuthorizationList, out *oauthapi.OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoConvert_v1beta3_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in, out, s)
}

func autoConvert_v1beta3_OAuthClientList_To_api_OAuthClientList(in *oauthapiv1beta3.OAuthClientList, out *oauthapi.OAuthClientList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthClientList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_OAuthClient_To_api_OAuthClient(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_OAuthClientList_To_api_OAuthClientList(in *oauthapiv1beta3.OAuthClientList, out *oauthapi.OAuthClientList, s conversion.Scope) error {
	return autoConvert_v1beta3_OAuthClientList_To_api_OAuthClientList(in, out, s)
}

func autoConvert_api_Project_To_v1beta3_Project(in *projectapi.Project, out *projectapiv1beta3.Project, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.Project))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_ProjectSpec_To_v1beta3_ProjectSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_ProjectStatus_To_v1beta3_ProjectStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_Project_To_v1beta3_Project(in *projectapi.Project, out *projectapiv1beta3.Project, s conversion.Scope) error {
	return autoConvert_api_Project_To_v1beta3_Project(in, out, s)
}

func autoConvert_api_ProjectList_To_v1beta3_ProjectList(in *projectapi.ProjectList, out *projectapiv1beta3.ProjectList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]projectapiv1beta3.Project, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Project_To_v1beta3_Project(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ProjectList_To_v1beta3_ProjectList(in *projectapi.ProjectList, out *projectapiv1beta3.ProjectList, s conversion.Scope) error {
	return autoConvert_api_ProjectList_To_v1beta3_ProjectList(in, out, s)
}

func autoConvert_api_ProjectRequest_To_v1beta3_ProjectRequest(in *projectapi.ProjectRequest, out *projectapiv1beta3.ProjectRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectRequest))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func Convert_api_ProjectRequest_To_v1beta3_ProjectRequest(in *projectapi.ProjectRequest, out *projectapiv1beta3.ProjectRequest, s conversion.Scope) error {
	return autoConvert_api_ProjectRequest_To_v1beta3_ProjectRequest(in, out, s)
}

func autoConvert_api_ProjectSpec_To_v1beta3_ProjectSpec(in *projectapi.ProjectSpec, out *projectapiv1beta3.ProjectSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectSpec))(in)
	}
	if in.Finalizers != nil {
		out.Finalizers = make([]apiv1beta3.FinalizerName, len(in.Finalizers))
		for i := range in.Finalizers {
			out.Finalizers[i] = apiv1beta3.FinalizerName(in.Finalizers[i])
		}
	} else {
		out.Finalizers = nil
	}
	return nil
}

func Convert_api_ProjectSpec_To_v1beta3_ProjectSpec(in *projectapi.ProjectSpec, out *projectapiv1beta3.ProjectSpec, s conversion.Scope) error {
	return autoConvert_api_ProjectSpec_To_v1beta3_ProjectSpec(in, out, s)
}

func autoConvert_api_ProjectStatus_To_v1beta3_ProjectStatus(in *projectapi.ProjectStatus, out *projectapiv1beta3.ProjectStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectStatus))(in)
	}
	out.Phase = apiv1beta3.NamespacePhase(in.Phase)
	return nil
}

func Convert_api_ProjectStatus_To_v1beta3_ProjectStatus(in *projectapi.ProjectStatus, out *projectapiv1beta3.ProjectStatus, s conversion.Scope) error {
	return autoConvert_api_ProjectStatus_To_v1beta3_ProjectStatus(in, out, s)
}

func autoConvert_v1beta3_Project_To_api_Project(in *projectapiv1beta3.Project, out *projectapi.Project, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.Project))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_ProjectSpec_To_api_ProjectSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_ProjectStatus_To_api_ProjectStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_Project_To_api_Project(in *projectapiv1beta3.Project, out *projectapi.Project, s conversion.Scope) error {
	return autoConvert_v1beta3_Project_To_api_Project(in, out, s)
}

func autoConvert_v1beta3_ProjectList_To_api_ProjectList(in *projectapiv1beta3.ProjectList, out *projectapi.ProjectList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.ProjectList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]projectapi.Project, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_Project_To_api_Project(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_ProjectList_To_api_ProjectList(in *projectapiv1beta3.ProjectList, out *projectapi.ProjectList, s conversion.Scope) error {
	return autoConvert_v1beta3_ProjectList_To_api_ProjectList(in, out, s)
}

func autoConvert_v1beta3_ProjectRequest_To_api_ProjectRequest(in *projectapiv1beta3.ProjectRequest, out *projectapi.ProjectRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.ProjectRequest))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func Convert_v1beta3_ProjectRequest_To_api_ProjectRequest(in *projectapiv1beta3.ProjectRequest, out *projectapi.ProjectRequest, s conversion.Scope) error {
	return autoConvert_v1beta3_ProjectRequest_To_api_ProjectRequest(in, out, s)
}

func autoConvert_v1beta3_ProjectSpec_To_api_ProjectSpec(in *projectapiv1beta3.ProjectSpec, out *projectapi.ProjectSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.ProjectSpec))(in)
	}
	if in.Finalizers != nil {
		out.Finalizers = make([]api.FinalizerName, len(in.Finalizers))
		for i := range in.Finalizers {
			out.Finalizers[i] = api.FinalizerName(in.Finalizers[i])
		}
	} else {
		out.Finalizers = nil
	}
	return nil
}

func Convert_v1beta3_ProjectSpec_To_api_ProjectSpec(in *projectapiv1beta3.ProjectSpec, out *projectapi.ProjectSpec, s conversion.Scope) error {
	return autoConvert_v1beta3_ProjectSpec_To_api_ProjectSpec(in, out, s)
}

func autoConvert_v1beta3_ProjectStatus_To_api_ProjectStatus(in *projectapiv1beta3.ProjectStatus, out *projectapi.ProjectStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.ProjectStatus))(in)
	}
	out.Phase = api.NamespacePhase(in.Phase)
	return nil
}

func Convert_v1beta3_ProjectStatus_To_api_ProjectStatus(in *projectapiv1beta3.ProjectStatus, out *projectapi.ProjectStatus, s conversion.Scope) error {
	return autoConvert_v1beta3_ProjectStatus_To_api_ProjectStatus(in, out, s)
}

func autoConvert_api_Route_To_v1beta3_Route(in *routeapi.Route, out *routeapiv1beta3.Route, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.Route))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_RouteSpec_To_v1beta3_RouteSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_RouteStatus_To_v1beta3_RouteStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_Route_To_v1beta3_Route(in *routeapi.Route, out *routeapiv1beta3.Route, s conversion.Scope) error {
	return autoConvert_api_Route_To_v1beta3_Route(in, out, s)
}

func autoConvert_api_RouteIngress_To_v1beta3_RouteIngress(in *routeapi.RouteIngress, out *routeapiv1beta3.RouteIngress, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteIngress))(in)
	}
	out.Host = in.Host
	out.RouterName = in.RouterName
	if in.Conditions != nil {
		out.Conditions = make([]routeapiv1beta3.RouteIngressCondition, len(in.Conditions))
		for i := range in.Conditions {
			if err := Convert_api_RouteIngressCondition_To_v1beta3_RouteIngressCondition(&in.Conditions[i], &out.Conditions[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Conditions = nil
	}
	return nil
}

func Convert_api_RouteIngress_To_v1beta3_RouteIngress(in *routeapi.RouteIngress, out *routeapiv1beta3.RouteIngress, s conversion.Scope) error {
	return autoConvert_api_RouteIngress_To_v1beta3_RouteIngress(in, out, s)
}

func autoConvert_api_RouteIngressCondition_To_v1beta3_RouteIngressCondition(in *routeapi.RouteIngressCondition, out *routeapiv1beta3.RouteIngressCondition, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteIngressCondition))(in)
	}
	out.Type = routeapiv1beta3.RouteIngressConditionType(in.Type)
	out.Status = apiv1beta3.ConditionStatus(in.Status)
	out.Reason = in.Reason
	out.Message = in.Message
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.LastTransitionTime != nil {
		out.LastTransitionTime = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.LastTransitionTime, out.LastTransitionTime, s); err != nil {
			return err
		}
	} else {
		out.LastTransitionTime = nil
	}
	return nil
}

func Convert_api_RouteIngressCondition_To_v1beta3_RouteIngressCondition(in *routeapi.RouteIngressCondition, out *routeapiv1beta3.RouteIngressCondition, s conversion.Scope) error {
	return autoConvert_api_RouteIngressCondition_To_v1beta3_RouteIngressCondition(in, out, s)
}

func autoConvert_api_RouteList_To_v1beta3_RouteList(in *routeapi.RouteList, out *routeapiv1beta3.RouteList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]routeapiv1beta3.Route, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Route_To_v1beta3_Route(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_RouteList_To_v1beta3_RouteList(in *routeapi.RouteList, out *routeapiv1beta3.RouteList, s conversion.Scope) error {
	return autoConvert_api_RouteList_To_v1beta3_RouteList(in, out, s)
}

func autoConvert_api_RoutePort_To_v1beta3_RoutePort(in *routeapi.RoutePort, out *routeapiv1beta3.RoutePort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RoutePort))(in)
	}
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.TargetPort, &out.TargetPort, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_RoutePort_To_v1beta3_RoutePort(in *routeapi.RoutePort, out *routeapiv1beta3.RoutePort, s conversion.Scope) error {
	return autoConvert_api_RoutePort_To_v1beta3_RoutePort(in, out, s)
}

func autoConvert_api_RouteSpec_To_v1beta3_RouteSpec(in *routeapi.RouteSpec, out *routeapiv1beta3.RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.RoutePort -> v1beta3.RoutePort
	if in.Port != nil {
		out.Port = new(routeapiv1beta3.RoutePort)
		if err := Convert_api_RoutePort_To_v1beta3_RoutePort(in.Port, out.Port, s); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	// unable to generate simple pointer conversion for api.TLSConfig -> v1beta3.TLSConfig
	if in.TLS != nil {
		out.TLS = new(routeapiv1beta3.TLSConfig)
		if err := Convert_api_TLSConfig_To_v1beta3_TLSConfig(in.TLS, out.TLS, s); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func Convert_api_RouteSpec_To_v1beta3_RouteSpec(in *routeapi.RouteSpec, out *routeapiv1beta3.RouteSpec, s conversion.Scope) error {
	return autoConvert_api_RouteSpec_To_v1beta3_RouteSpec(in, out, s)
}

func autoConvert_api_RouteStatus_To_v1beta3_RouteStatus(in *routeapi.RouteStatus, out *routeapiv1beta3.RouteStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteStatus))(in)
	}
	if in.Ingress != nil {
		out.Ingress = make([]routeapiv1beta3.RouteIngress, len(in.Ingress))
		for i := range in.Ingress {
			if err := Convert_api_RouteIngress_To_v1beta3_RouteIngress(&in.Ingress[i], &out.Ingress[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ingress = nil
	}
	return nil
}

func Convert_api_RouteStatus_To_v1beta3_RouteStatus(in *routeapi.RouteStatus, out *routeapiv1beta3.RouteStatus, s conversion.Scope) error {
	return autoConvert_api_RouteStatus_To_v1beta3_RouteStatus(in, out, s)
}

func autoConvert_api_TLSConfig_To_v1beta3_TLSConfig(in *routeapi.TLSConfig, out *routeapiv1beta3.TLSConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.TLSConfig))(in)
	}
	out.Termination = routeapiv1beta3.TLSTerminationType(in.Termination)
	out.Certificate = in.Certificate
	out.Key = in.Key
	out.CACertificate = in.CACertificate
	out.DestinationCACertificate = in.DestinationCACertificate
	out.InsecureEdgeTerminationPolicy = routeapiv1beta3.InsecureEdgeTerminationPolicyType(in.InsecureEdgeTerminationPolicy)
	return nil
}

func Convert_api_TLSConfig_To_v1beta3_TLSConfig(in *routeapi.TLSConfig, out *routeapiv1beta3.TLSConfig, s conversion.Scope) error {
	return autoConvert_api_TLSConfig_To_v1beta3_TLSConfig(in, out, s)
}

func autoConvert_v1beta3_Route_To_api_Route(in *routeapiv1beta3.Route, out *routeapi.Route, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.Route))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_RouteSpec_To_api_RouteSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_RouteStatus_To_api_RouteStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_Route_To_api_Route(in *routeapiv1beta3.Route, out *routeapi.Route, s conversion.Scope) error {
	return autoConvert_v1beta3_Route_To_api_Route(in, out, s)
}

func autoConvert_v1beta3_RouteIngress_To_api_RouteIngress(in *routeapiv1beta3.RouteIngress, out *routeapi.RouteIngress, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RouteIngress))(in)
	}
	out.Host = in.Host
	out.RouterName = in.RouterName
	if in.Conditions != nil {
		out.Conditions = make([]routeapi.RouteIngressCondition, len(in.Conditions))
		for i := range in.Conditions {
			if err := Convert_v1beta3_RouteIngressCondition_To_api_RouteIngressCondition(&in.Conditions[i], &out.Conditions[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Conditions = nil
	}
	return nil
}

func Convert_v1beta3_RouteIngress_To_api_RouteIngress(in *routeapiv1beta3.RouteIngress, out *routeapi.RouteIngress, s conversion.Scope) error {
	return autoConvert_v1beta3_RouteIngress_To_api_RouteIngress(in, out, s)
}

func autoConvert_v1beta3_RouteIngressCondition_To_api_RouteIngressCondition(in *routeapiv1beta3.RouteIngressCondition, out *routeapi.RouteIngressCondition, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RouteIngressCondition))(in)
	}
	out.Type = routeapi.RouteIngressConditionType(in.Type)
	out.Status = api.ConditionStatus(in.Status)
	out.Reason = in.Reason
	out.Message = in.Message
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.LastTransitionTime != nil {
		out.LastTransitionTime = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.LastTransitionTime, out.LastTransitionTime, s); err != nil {
			return err
		}
	} else {
		out.LastTransitionTime = nil
	}
	return nil
}

func Convert_v1beta3_RouteIngressCondition_To_api_RouteIngressCondition(in *routeapiv1beta3.RouteIngressCondition, out *routeapi.RouteIngressCondition, s conversion.Scope) error {
	return autoConvert_v1beta3_RouteIngressCondition_To_api_RouteIngressCondition(in, out, s)
}

func autoConvert_v1beta3_RouteList_To_api_RouteList(in *routeapiv1beta3.RouteList, out *routeapi.RouteList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RouteList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]routeapi.Route, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_Route_To_api_Route(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_RouteList_To_api_RouteList(in *routeapiv1beta3.RouteList, out *routeapi.RouteList, s conversion.Scope) error {
	return autoConvert_v1beta3_RouteList_To_api_RouteList(in, out, s)
}

func autoConvert_v1beta3_RoutePort_To_api_RoutePort(in *routeapiv1beta3.RoutePort, out *routeapi.RoutePort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RoutePort))(in)
	}
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.TargetPort, &out.TargetPort, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_RoutePort_To_api_RoutePort(in *routeapiv1beta3.RoutePort, out *routeapi.RoutePort, s conversion.Scope) error {
	return autoConvert_v1beta3_RoutePort_To_api_RoutePort(in, out, s)
}

func autoConvert_v1beta3_RouteSpec_To_api_RouteSpec(in *routeapiv1beta3.RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1beta3.RoutePort -> api.RoutePort
	if in.Port != nil {
		out.Port = new(routeapi.RoutePort)
		if err := Convert_v1beta3_RoutePort_To_api_RoutePort(in.Port, out.Port, s); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	// unable to generate simple pointer conversion for v1beta3.TLSConfig -> api.TLSConfig
	if in.TLS != nil {
		out.TLS = new(routeapi.TLSConfig)
		if err := Convert_v1beta3_TLSConfig_To_api_TLSConfig(in.TLS, out.TLS, s); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func Convert_v1beta3_RouteSpec_To_api_RouteSpec(in *routeapiv1beta3.RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	return autoConvert_v1beta3_RouteSpec_To_api_RouteSpec(in, out, s)
}

func autoConvert_v1beta3_RouteStatus_To_api_RouteStatus(in *routeapiv1beta3.RouteStatus, out *routeapi.RouteStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RouteStatus))(in)
	}
	if in.Ingress != nil {
		out.Ingress = make([]routeapi.RouteIngress, len(in.Ingress))
		for i := range in.Ingress {
			if err := Convert_v1beta3_RouteIngress_To_api_RouteIngress(&in.Ingress[i], &out.Ingress[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ingress = nil
	}
	return nil
}

func Convert_v1beta3_RouteStatus_To_api_RouteStatus(in *routeapiv1beta3.RouteStatus, out *routeapi.RouteStatus, s conversion.Scope) error {
	return autoConvert_v1beta3_RouteStatus_To_api_RouteStatus(in, out, s)
}

func autoConvert_v1beta3_TLSConfig_To_api_TLSConfig(in *routeapiv1beta3.TLSConfig, out *routeapi.TLSConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.TLSConfig))(in)
	}
	out.Termination = routeapi.TLSTerminationType(in.Termination)
	out.Certificate = in.Certificate
	out.Key = in.Key
	out.CACertificate = in.CACertificate
	out.DestinationCACertificate = in.DestinationCACertificate
	out.InsecureEdgeTerminationPolicy = routeapi.InsecureEdgeTerminationPolicyType(in.InsecureEdgeTerminationPolicy)
	return nil
}

func Convert_v1beta3_TLSConfig_To_api_TLSConfig(in *routeapiv1beta3.TLSConfig, out *routeapi.TLSConfig, s conversion.Scope) error {
	return autoConvert_v1beta3_TLSConfig_To_api_TLSConfig(in, out, s)
}

func autoConvert_api_ClusterNetwork_To_v1beta3_ClusterNetwork(in *sdnapi.ClusterNetwork, out *sdnapiv1beta3.ClusterNetwork, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.ClusterNetwork))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func Convert_api_ClusterNetwork_To_v1beta3_ClusterNetwork(in *sdnapi.ClusterNetwork, out *sdnapiv1beta3.ClusterNetwork, s conversion.Scope) error {
	return autoConvert_api_ClusterNetwork_To_v1beta3_ClusterNetwork(in, out, s)
}

func autoConvert_api_ClusterNetworkList_To_v1beta3_ClusterNetworkList(in *sdnapi.ClusterNetworkList, out *sdnapiv1beta3.ClusterNetworkList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.ClusterNetworkList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1beta3.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_ClusterNetwork_To_v1beta3_ClusterNetwork(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterNetworkList_To_v1beta3_ClusterNetworkList(in *sdnapi.ClusterNetworkList, out *sdnapiv1beta3.ClusterNetworkList, s conversion.Scope) error {
	return autoConvert_api_ClusterNetworkList_To_v1beta3_ClusterNetworkList(in, out, s)
}

func autoConvert_api_HostSubnet_To_v1beta3_HostSubnet(in *sdnapi.HostSubnet, out *sdnapiv1beta3.HostSubnet, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.HostSubnet))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func Convert_api_HostSubnet_To_v1beta3_HostSubnet(in *sdnapi.HostSubnet, out *sdnapiv1beta3.HostSubnet, s conversion.Scope) error {
	return autoConvert_api_HostSubnet_To_v1beta3_HostSubnet(in, out, s)
}

func autoConvert_api_HostSubnetList_To_v1beta3_HostSubnetList(in *sdnapi.HostSubnetList, out *sdnapiv1beta3.HostSubnetList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.HostSubnetList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1beta3.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_HostSubnet_To_v1beta3_HostSubnet(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_HostSubnetList_To_v1beta3_HostSubnetList(in *sdnapi.HostSubnetList, out *sdnapiv1beta3.HostSubnetList, s conversion.Scope) error {
	return autoConvert_api_HostSubnetList_To_v1beta3_HostSubnetList(in, out, s)
}

func autoConvert_api_NetNamespace_To_v1beta3_NetNamespace(in *sdnapi.NetNamespace, out *sdnapiv1beta3.NetNamespace, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.NetNamespace))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func Convert_api_NetNamespace_To_v1beta3_NetNamespace(in *sdnapi.NetNamespace, out *sdnapiv1beta3.NetNamespace, s conversion.Scope) error {
	return autoConvert_api_NetNamespace_To_v1beta3_NetNamespace(in, out, s)
}

func autoConvert_api_NetNamespaceList_To_v1beta3_NetNamespaceList(in *sdnapi.NetNamespaceList, out *sdnapiv1beta3.NetNamespaceList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.NetNamespaceList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1beta3.NetNamespace, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_NetNamespace_To_v1beta3_NetNamespace(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_NetNamespaceList_To_v1beta3_NetNamespaceList(in *sdnapi.NetNamespaceList, out *sdnapiv1beta3.NetNamespaceList, s conversion.Scope) error {
	return autoConvert_api_NetNamespaceList_To_v1beta3_NetNamespaceList(in, out, s)
}

func autoConvert_v1beta3_ClusterNetwork_To_api_ClusterNetwork(in *sdnapiv1beta3.ClusterNetwork, out *sdnapi.ClusterNetwork, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.ClusterNetwork))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func Convert_v1beta3_ClusterNetwork_To_api_ClusterNetwork(in *sdnapiv1beta3.ClusterNetwork, out *sdnapi.ClusterNetwork, s conversion.Scope) error {
	return autoConvert_v1beta3_ClusterNetwork_To_api_ClusterNetwork(in, out, s)
}

func autoConvert_v1beta3_ClusterNetworkList_To_api_ClusterNetworkList(in *sdnapiv1beta3.ClusterNetworkList, out *sdnapi.ClusterNetworkList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.ClusterNetworkList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_ClusterNetwork_To_api_ClusterNetwork(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_ClusterNetworkList_To_api_ClusterNetworkList(in *sdnapiv1beta3.ClusterNetworkList, out *sdnapi.ClusterNetworkList, s conversion.Scope) error {
	return autoConvert_v1beta3_ClusterNetworkList_To_api_ClusterNetworkList(in, out, s)
}

func autoConvert_v1beta3_HostSubnet_To_api_HostSubnet(in *sdnapiv1beta3.HostSubnet, out *sdnapi.HostSubnet, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.HostSubnet))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func Convert_v1beta3_HostSubnet_To_api_HostSubnet(in *sdnapiv1beta3.HostSubnet, out *sdnapi.HostSubnet, s conversion.Scope) error {
	return autoConvert_v1beta3_HostSubnet_To_api_HostSubnet(in, out, s)
}

func autoConvert_v1beta3_HostSubnetList_To_api_HostSubnetList(in *sdnapiv1beta3.HostSubnetList, out *sdnapi.HostSubnetList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.HostSubnetList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_HostSubnet_To_api_HostSubnet(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_HostSubnetList_To_api_HostSubnetList(in *sdnapiv1beta3.HostSubnetList, out *sdnapi.HostSubnetList, s conversion.Scope) error {
	return autoConvert_v1beta3_HostSubnetList_To_api_HostSubnetList(in, out, s)
}

func autoConvert_v1beta3_NetNamespace_To_api_NetNamespace(in *sdnapiv1beta3.NetNamespace, out *sdnapi.NetNamespace, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.NetNamespace))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func Convert_v1beta3_NetNamespace_To_api_NetNamespace(in *sdnapiv1beta3.NetNamespace, out *sdnapi.NetNamespace, s conversion.Scope) error {
	return autoConvert_v1beta3_NetNamespace_To_api_NetNamespace(in, out, s)
}

func autoConvert_v1beta3_NetNamespaceList_To_api_NetNamespaceList(in *sdnapiv1beta3.NetNamespaceList, out *sdnapi.NetNamespaceList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.NetNamespaceList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.NetNamespace, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_NetNamespace_To_api_NetNamespace(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_NetNamespaceList_To_api_NetNamespaceList(in *sdnapiv1beta3.NetNamespaceList, out *sdnapi.NetNamespaceList, s conversion.Scope) error {
	return autoConvert_v1beta3_NetNamespaceList_To_api_NetNamespaceList(in, out, s)
}

func autoConvert_api_Parameter_To_v1beta3_Parameter(in *templateapi.Parameter, out *templateapiv1beta3.Parameter, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapi.Parameter))(in)
	}
	out.Name = in.Name
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	out.Value = in.Value
	out.Generate = in.Generate
	out.From = in.From
	out.Required = in.Required
	return nil
}

func Convert_api_Parameter_To_v1beta3_Parameter(in *templateapi.Parameter, out *templateapiv1beta3.Parameter, s conversion.Scope) error {
	return autoConvert_api_Parameter_To_v1beta3_Parameter(in, out, s)
}

func autoConvert_api_Template_To_v1beta3_Template(in *templateapi.Template, out *templateapiv1beta3.Template, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapi.Template))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Parameters != nil {
		out.Parameters = make([]templateapiv1beta3.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := Convert_api_Parameter_To_v1beta3_Parameter(&in.Parameters[i], &out.Parameters[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Parameters = nil
	}
	if in.Objects != nil {
		out.Objects = make([]runtime.RawExtension, len(in.Objects))
		for i := range in.Objects {
			if err := runtime.Convert_runtime_Object_To_runtime_RawExtension(&in.Objects[i], &out.Objects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Objects = nil
	}
	// in.ObjectLabels has no peer in out
	return nil
}

func autoConvert_api_TemplateList_To_v1beta3_TemplateList(in *templateapi.TemplateList, out *templateapiv1beta3.TemplateList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapi.TemplateList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]templateapiv1beta3.Template, len(in.Items))
		for i := range in.Items {
			if err := templateapiv1beta3.Convert_api_Template_To_v1beta3_Template(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_TemplateList_To_v1beta3_TemplateList(in *templateapi.TemplateList, out *templateapiv1beta3.TemplateList, s conversion.Scope) error {
	return autoConvert_api_TemplateList_To_v1beta3_TemplateList(in, out, s)
}

func autoConvert_v1beta3_Parameter_To_api_Parameter(in *templateapiv1beta3.Parameter, out *templateapi.Parameter, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1beta3.Parameter))(in)
	}
	out.Name = in.Name
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	out.Value = in.Value
	out.Generate = in.Generate
	out.From = in.From
	out.Required = in.Required
	return nil
}

func Convert_v1beta3_Parameter_To_api_Parameter(in *templateapiv1beta3.Parameter, out *templateapi.Parameter, s conversion.Scope) error {
	return autoConvert_v1beta3_Parameter_To_api_Parameter(in, out, s)
}

func autoConvert_v1beta3_Template_To_api_Template(in *templateapiv1beta3.Template, out *templateapi.Template, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1beta3.Template))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Objects != nil {
		out.Objects = make([]runtime.Object, len(in.Objects))
		for i := range in.Objects {
			if err := runtime.Convert_runtime_RawExtension_To_runtime_Object(&in.Objects[i], &out.Objects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Objects = nil
	}
	if in.Parameters != nil {
		out.Parameters = make([]templateapi.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := Convert_v1beta3_Parameter_To_api_Parameter(&in.Parameters[i], &out.Parameters[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Parameters = nil
	}
	if in.Labels != nil {
		out.Labels = make(map[string]string)
		for key, val := range in.Labels {
			out.Labels[key] = val
		}
	} else {
		out.Labels = nil
	}
	return nil
}

func autoConvert_v1beta3_TemplateList_To_api_TemplateList(in *templateapiv1beta3.TemplateList, out *templateapi.TemplateList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1beta3.TemplateList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]templateapi.Template, len(in.Items))
		for i := range in.Items {
			if err := templateapiv1beta3.Convert_v1beta3_Template_To_api_Template(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_TemplateList_To_api_TemplateList(in *templateapiv1beta3.TemplateList, out *templateapi.TemplateList, s conversion.Scope) error {
	return autoConvert_v1beta3_TemplateList_To_api_TemplateList(in, out, s)
}

func autoConvert_api_Group_To_v1beta3_Group(in *userapi.Group, out *userapiv1beta3.Group, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.Group))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Users != nil {
		out.Users = make([]string, len(in.Users))
		for i := range in.Users {
			out.Users[i] = in.Users[i]
		}
	} else {
		out.Users = nil
	}
	return nil
}

func Convert_api_Group_To_v1beta3_Group(in *userapi.Group, out *userapiv1beta3.Group, s conversion.Scope) error {
	return autoConvert_api_Group_To_v1beta3_Group(in, out, s)
}

func autoConvert_api_GroupList_To_v1beta3_GroupList(in *userapi.GroupList, out *userapiv1beta3.GroupList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.GroupList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapiv1beta3.Group, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Group_To_v1beta3_Group(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_GroupList_To_v1beta3_GroupList(in *userapi.GroupList, out *userapiv1beta3.GroupList, s conversion.Scope) error {
	return autoConvert_api_GroupList_To_v1beta3_GroupList(in, out, s)
}

func autoConvert_api_Identity_To_v1beta3_Identity(in *userapi.Identity, out *userapiv1beta3.Identity, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.Identity))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	if in.Extra != nil {
		out.Extra = make(map[string]string)
		for key, val := range in.Extra {
			out.Extra[key] = val
		}
	} else {
		out.Extra = nil
	}
	return nil
}

func Convert_api_Identity_To_v1beta3_Identity(in *userapi.Identity, out *userapiv1beta3.Identity, s conversion.Scope) error {
	return autoConvert_api_Identity_To_v1beta3_Identity(in, out, s)
}

func autoConvert_api_IdentityList_To_v1beta3_IdentityList(in *userapi.IdentityList, out *userapiv1beta3.IdentityList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.IdentityList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapiv1beta3.Identity, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Identity_To_v1beta3_Identity(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_IdentityList_To_v1beta3_IdentityList(in *userapi.IdentityList, out *userapiv1beta3.IdentityList, s conversion.Scope) error {
	return autoConvert_api_IdentityList_To_v1beta3_IdentityList(in, out, s)
}

func autoConvert_api_User_To_v1beta3_User(in *userapi.User, out *userapiv1beta3.User, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.User))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.FullName = in.FullName
	if in.Identities != nil {
		out.Identities = make([]string, len(in.Identities))
		for i := range in.Identities {
			out.Identities[i] = in.Identities[i]
		}
	} else {
		out.Identities = nil
	}
	if in.Groups != nil {
		out.Groups = make([]string, len(in.Groups))
		for i := range in.Groups {
			out.Groups[i] = in.Groups[i]
		}
	} else {
		out.Groups = nil
	}
	return nil
}

func Convert_api_User_To_v1beta3_User(in *userapi.User, out *userapiv1beta3.User, s conversion.Scope) error {
	return autoConvert_api_User_To_v1beta3_User(in, out, s)
}

func autoConvert_api_UserIdentityMapping_To_v1beta3_UserIdentityMapping(in *userapi.UserIdentityMapping, out *userapiv1beta3.UserIdentityMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.UserIdentityMapping))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := Convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_UserIdentityMapping_To_v1beta3_UserIdentityMapping(in *userapi.UserIdentityMapping, out *userapiv1beta3.UserIdentityMapping, s conversion.Scope) error {
	return autoConvert_api_UserIdentityMapping_To_v1beta3_UserIdentityMapping(in, out, s)
}

func autoConvert_api_UserList_To_v1beta3_UserList(in *userapi.UserList, out *userapiv1beta3.UserList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.UserList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapiv1beta3.User, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_User_To_v1beta3_User(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_UserList_To_v1beta3_UserList(in *userapi.UserList, out *userapiv1beta3.UserList, s conversion.Scope) error {
	return autoConvert_api_UserList_To_v1beta3_UserList(in, out, s)
}

func autoConvert_v1beta3_Group_To_api_Group(in *userapiv1beta3.Group, out *userapi.Group, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.Group))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Users != nil {
		out.Users = make([]string, len(in.Users))
		for i := range in.Users {
			out.Users[i] = in.Users[i]
		}
	} else {
		out.Users = nil
	}
	return nil
}

func Convert_v1beta3_Group_To_api_Group(in *userapiv1beta3.Group, out *userapi.Group, s conversion.Scope) error {
	return autoConvert_v1beta3_Group_To_api_Group(in, out, s)
}

func autoConvert_v1beta3_GroupList_To_api_GroupList(in *userapiv1beta3.GroupList, out *userapi.GroupList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.GroupList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapi.Group, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_Group_To_api_Group(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_GroupList_To_api_GroupList(in *userapiv1beta3.GroupList, out *userapi.GroupList, s conversion.Scope) error {
	return autoConvert_v1beta3_GroupList_To_api_GroupList(in, out, s)
}

func autoConvert_v1beta3_Identity_To_api_Identity(in *userapiv1beta3.Identity, out *userapi.Identity, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.Identity))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	if in.Extra != nil {
		out.Extra = make(map[string]string)
		for key, val := range in.Extra {
			out.Extra[key] = val
		}
	} else {
		out.Extra = nil
	}
	return nil
}

func Convert_v1beta3_Identity_To_api_Identity(in *userapiv1beta3.Identity, out *userapi.Identity, s conversion.Scope) error {
	return autoConvert_v1beta3_Identity_To_api_Identity(in, out, s)
}

func autoConvert_v1beta3_IdentityList_To_api_IdentityList(in *userapiv1beta3.IdentityList, out *userapi.IdentityList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.IdentityList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapi.Identity, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_Identity_To_api_Identity(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_IdentityList_To_api_IdentityList(in *userapiv1beta3.IdentityList, out *userapi.IdentityList, s conversion.Scope) error {
	return autoConvert_v1beta3_IdentityList_To_api_IdentityList(in, out, s)
}

func autoConvert_v1beta3_User_To_api_User(in *userapiv1beta3.User, out *userapi.User, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.User))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.FullName = in.FullName
	if in.Identities != nil {
		out.Identities = make([]string, len(in.Identities))
		for i := range in.Identities {
			out.Identities[i] = in.Identities[i]
		}
	} else {
		out.Identities = nil
	}
	if in.Groups != nil {
		out.Groups = make([]string, len(in.Groups))
		for i := range in.Groups {
			out.Groups[i] = in.Groups[i]
		}
	} else {
		out.Groups = nil
	}
	return nil
}

func Convert_v1beta3_User_To_api_User(in *userapiv1beta3.User, out *userapi.User, s conversion.Scope) error {
	return autoConvert_v1beta3_User_To_api_User(in, out, s)
}

func autoConvert_v1beta3_UserIdentityMapping_To_api_UserIdentityMapping(in *userapiv1beta3.UserIdentityMapping, out *userapi.UserIdentityMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.UserIdentityMapping))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := Convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_UserIdentityMapping_To_api_UserIdentityMapping(in *userapiv1beta3.UserIdentityMapping, out *userapi.UserIdentityMapping, s conversion.Scope) error {
	return autoConvert_v1beta3_UserIdentityMapping_To_api_UserIdentityMapping(in, out, s)
}

func autoConvert_v1beta3_UserList_To_api_UserList(in *userapiv1beta3.UserList, out *userapi.UserList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.UserList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapi.User, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_User_To_api_User(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_UserList_To_api_UserList(in *userapiv1beta3.UserList, out *userapi.UserList, s conversion.Scope) error {
	return autoConvert_v1beta3_UserList_To_api_UserList(in, out, s)
}

func autoConvert_api_AWSElasticBlockStoreVolumeSource_To_v1beta3_AWSElasticBlockStoreVolumeSource(in *api.AWSElasticBlockStoreVolumeSource, out *apiv1beta3.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.AWSElasticBlockStoreVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.Partition = in.Partition
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_AWSElasticBlockStoreVolumeSource_To_v1beta3_AWSElasticBlockStoreVolumeSource(in *api.AWSElasticBlockStoreVolumeSource, out *apiv1beta3.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	return autoConvert_api_AWSElasticBlockStoreVolumeSource_To_v1beta3_AWSElasticBlockStoreVolumeSource(in, out, s)
}

func autoConvert_api_Capabilities_To_v1beta3_Capabilities(in *api.Capabilities, out *apiv1beta3.Capabilities, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Capabilities))(in)
	}
	if in.Add != nil {
		out.Add = make([]apiv1beta3.Capability, len(in.Add))
		for i := range in.Add {
			out.Add[i] = apiv1beta3.Capability(in.Add[i])
		}
	} else {
		out.Add = nil
	}
	if in.Drop != nil {
		out.Drop = make([]apiv1beta3.Capability, len(in.Drop))
		for i := range in.Drop {
			out.Drop[i] = apiv1beta3.Capability(in.Drop[i])
		}
	} else {
		out.Drop = nil
	}
	return nil
}

func Convert_api_Capabilities_To_v1beta3_Capabilities(in *api.Capabilities, out *apiv1beta3.Capabilities, s conversion.Scope) error {
	return autoConvert_api_Capabilities_To_v1beta3_Capabilities(in, out, s)
}

func autoConvert_api_CinderVolumeSource_To_v1beta3_CinderVolumeSource(in *api.CinderVolumeSource, out *apiv1beta3.CinderVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.CinderVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_CinderVolumeSource_To_v1beta3_CinderVolumeSource(in *api.CinderVolumeSource, out *apiv1beta3.CinderVolumeSource, s conversion.Scope) error {
	return autoConvert_api_CinderVolumeSource_To_v1beta3_CinderVolumeSource(in, out, s)
}

func autoConvert_api_Container_To_v1beta3_Container(in *api.Container, out *apiv1beta3.Container, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Container))(in)
	}
	out.Name = in.Name
	out.Image = in.Image
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	if in.Args != nil {
		out.Args = make([]string, len(in.Args))
		for i := range in.Args {
			out.Args[i] = in.Args[i]
		}
	} else {
		out.Args = nil
	}
	out.WorkingDir = in.WorkingDir
	if in.Ports != nil {
		out.Ports = make([]apiv1beta3.ContainerPort, len(in.Ports))
		for i := range in.Ports {
			if err := Convert_api_ContainerPort_To_v1beta3_ContainerPort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ports = nil
	}
	if in.Env != nil {
		out.Env = make([]apiv1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := s.Convert(&in.Env[i], &out.Env[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	if err := Convert_api_ResourceRequirements_To_v1beta3_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if in.VolumeMounts != nil {
		out.VolumeMounts = make([]apiv1beta3.VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			if err := Convert_api_VolumeMount_To_v1beta3_VolumeMount(&in.VolumeMounts[i], &out.VolumeMounts[i], s); err != nil {
				return err
			}
		}
	} else {
		out.VolumeMounts = nil
	}
	// unable to generate simple pointer conversion for api.Probe -> v1beta3.Probe
	if in.LivenessProbe != nil {
		if err := s.Convert(&in.LivenessProbe, &out.LivenessProbe, 0); err != nil {
			return err
		}
	} else {
		out.LivenessProbe = nil
	}
	// unable to generate simple pointer conversion for api.Probe -> v1beta3.Probe
	if in.ReadinessProbe != nil {
		if err := s.Convert(&in.ReadinessProbe, &out.ReadinessProbe, 0); err != nil {
			return err
		}
	} else {
		out.ReadinessProbe = nil
	}
	// unable to generate simple pointer conversion for api.Lifecycle -> v1beta3.Lifecycle
	if in.Lifecycle != nil {
		if err := s.Convert(&in.Lifecycle, &out.Lifecycle, 0); err != nil {
			return err
		}
	} else {
		out.Lifecycle = nil
	}
	out.TerminationMessagePath = in.TerminationMessagePath
	out.ImagePullPolicy = apiv1beta3.PullPolicy(in.ImagePullPolicy)
	// unable to generate simple pointer conversion for api.SecurityContext -> v1beta3.SecurityContext
	if in.SecurityContext != nil {
		if err := s.Convert(&in.SecurityContext, &out.SecurityContext, 0); err != nil {
			return err
		}
	} else {
		out.SecurityContext = nil
	}
	out.Stdin = in.Stdin
	out.StdinOnce = in.StdinOnce
	out.TTY = in.TTY
	return nil
}

func autoConvert_api_ContainerPort_To_v1beta3_ContainerPort(in *api.ContainerPort, out *apiv1beta3.ContainerPort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ContainerPort))(in)
	}
	out.Name = in.Name
	out.HostPort = in.HostPort
	out.ContainerPort = in.ContainerPort
	out.Protocol = apiv1beta3.Protocol(in.Protocol)
	out.HostIP = in.HostIP
	return nil
}

func Convert_api_ContainerPort_To_v1beta3_ContainerPort(in *api.ContainerPort, out *apiv1beta3.ContainerPort, s conversion.Scope) error {
	return autoConvert_api_ContainerPort_To_v1beta3_ContainerPort(in, out, s)
}

func autoConvert_api_DownwardAPIVolumeFile_To_v1beta3_DownwardAPIVolumeFile(in *api.DownwardAPIVolumeFile, out *apiv1beta3.DownwardAPIVolumeFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.DownwardAPIVolumeFile))(in)
	}
	out.Path = in.Path
	if err := Convert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_DownwardAPIVolumeFile_To_v1beta3_DownwardAPIVolumeFile(in *api.DownwardAPIVolumeFile, out *apiv1beta3.DownwardAPIVolumeFile, s conversion.Scope) error {
	return autoConvert_api_DownwardAPIVolumeFile_To_v1beta3_DownwardAPIVolumeFile(in, out, s)
}

func autoConvert_api_DownwardAPIVolumeSource_To_v1beta3_DownwardAPIVolumeSource(in *api.DownwardAPIVolumeSource, out *apiv1beta3.DownwardAPIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.DownwardAPIVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.DownwardAPIVolumeFile, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_DownwardAPIVolumeFile_To_v1beta3_DownwardAPIVolumeFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_DownwardAPIVolumeSource_To_v1beta3_DownwardAPIVolumeSource(in *api.DownwardAPIVolumeSource, out *apiv1beta3.DownwardAPIVolumeSource, s conversion.Scope) error {
	return autoConvert_api_DownwardAPIVolumeSource_To_v1beta3_DownwardAPIVolumeSource(in, out, s)
}

func autoConvert_api_EmptyDirVolumeSource_To_v1beta3_EmptyDirVolumeSource(in *api.EmptyDirVolumeSource, out *apiv1beta3.EmptyDirVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.EmptyDirVolumeSource))(in)
	}
	out.Medium = apiv1beta3.StorageMedium(in.Medium)
	return nil
}

func Convert_api_EmptyDirVolumeSource_To_v1beta3_EmptyDirVolumeSource(in *api.EmptyDirVolumeSource, out *apiv1beta3.EmptyDirVolumeSource, s conversion.Scope) error {
	return autoConvert_api_EmptyDirVolumeSource_To_v1beta3_EmptyDirVolumeSource(in, out, s)
}

func autoConvert_api_ExecAction_To_v1beta3_ExecAction(in *api.ExecAction, out *apiv1beta3.ExecAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ExecAction))(in)
	}
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	return nil
}

func Convert_api_ExecAction_To_v1beta3_ExecAction(in *api.ExecAction, out *apiv1beta3.ExecAction, s conversion.Scope) error {
	return autoConvert_api_ExecAction_To_v1beta3_ExecAction(in, out, s)
}

func autoConvert_api_FCVolumeSource_To_v1beta3_FCVolumeSource(in *api.FCVolumeSource, out *apiv1beta3.FCVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.FCVolumeSource))(in)
	}
	if in.TargetWWNs != nil {
		out.TargetWWNs = make([]string, len(in.TargetWWNs))
		for i := range in.TargetWWNs {
			out.TargetWWNs[i] = in.TargetWWNs[i]
		}
	} else {
		out.TargetWWNs = nil
	}
	if in.Lun != nil {
		out.Lun = new(int)
		*out.Lun = *in.Lun
	} else {
		out.Lun = nil
	}
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_FCVolumeSource_To_v1beta3_FCVolumeSource(in *api.FCVolumeSource, out *apiv1beta3.FCVolumeSource, s conversion.Scope) error {
	return autoConvert_api_FCVolumeSource_To_v1beta3_FCVolumeSource(in, out, s)
}

func autoConvert_api_FlockerVolumeSource_To_v1beta3_FlockerVolumeSource(in *api.FlockerVolumeSource, out *apiv1beta3.FlockerVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.FlockerVolumeSource))(in)
	}
	out.DatasetName = in.DatasetName
	return nil
}

func Convert_api_FlockerVolumeSource_To_v1beta3_FlockerVolumeSource(in *api.FlockerVolumeSource, out *apiv1beta3.FlockerVolumeSource, s conversion.Scope) error {
	return autoConvert_api_FlockerVolumeSource_To_v1beta3_FlockerVolumeSource(in, out, s)
}

func autoConvert_api_GCEPersistentDiskVolumeSource_To_v1beta3_GCEPersistentDiskVolumeSource(in *api.GCEPersistentDiskVolumeSource, out *apiv1beta3.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.GCEPersistentDiskVolumeSource))(in)
	}
	out.PDName = in.PDName
	out.FSType = in.FSType
	out.Partition = in.Partition
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_GCEPersistentDiskVolumeSource_To_v1beta3_GCEPersistentDiskVolumeSource(in *api.GCEPersistentDiskVolumeSource, out *apiv1beta3.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	return autoConvert_api_GCEPersistentDiskVolumeSource_To_v1beta3_GCEPersistentDiskVolumeSource(in, out, s)
}

func autoConvert_api_GlusterfsVolumeSource_To_v1beta3_GlusterfsVolumeSource(in *api.GlusterfsVolumeSource, out *apiv1beta3.GlusterfsVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.GlusterfsVolumeSource))(in)
	}
	out.EndpointsName = in.EndpointsName
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_GlusterfsVolumeSource_To_v1beta3_GlusterfsVolumeSource(in *api.GlusterfsVolumeSource, out *apiv1beta3.GlusterfsVolumeSource, s conversion.Scope) error {
	return autoConvert_api_GlusterfsVolumeSource_To_v1beta3_GlusterfsVolumeSource(in, out, s)
}

func autoConvert_api_HostPathVolumeSource_To_v1beta3_HostPathVolumeSource(in *api.HostPathVolumeSource, out *apiv1beta3.HostPathVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.HostPathVolumeSource))(in)
	}
	out.Path = in.Path
	return nil
}

func Convert_api_HostPathVolumeSource_To_v1beta3_HostPathVolumeSource(in *api.HostPathVolumeSource, out *apiv1beta3.HostPathVolumeSource, s conversion.Scope) error {
	return autoConvert_api_HostPathVolumeSource_To_v1beta3_HostPathVolumeSource(in, out, s)
}

func autoConvert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in *api.LocalObjectReference, out *apiv1beta3.LocalObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.LocalObjectReference))(in)
	}
	out.Name = in.Name
	return nil
}

func Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in *api.LocalObjectReference, out *apiv1beta3.LocalObjectReference, s conversion.Scope) error {
	return autoConvert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in, out, s)
}

func autoConvert_api_NFSVolumeSource_To_v1beta3_NFSVolumeSource(in *api.NFSVolumeSource, out *apiv1beta3.NFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.NFSVolumeSource))(in)
	}
	out.Server = in.Server
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_NFSVolumeSource_To_v1beta3_NFSVolumeSource(in *api.NFSVolumeSource, out *apiv1beta3.NFSVolumeSource, s conversion.Scope) error {
	return autoConvert_api_NFSVolumeSource_To_v1beta3_NFSVolumeSource(in, out, s)
}

func autoConvert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector(in *api.ObjectFieldSelector, out *apiv1beta3.ObjectFieldSelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ObjectFieldSelector))(in)
	}
	out.APIVersion = in.APIVersion
	out.FieldPath = in.FieldPath
	return nil
}

func Convert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector(in *api.ObjectFieldSelector, out *apiv1beta3.ObjectFieldSelector, s conversion.Scope) error {
	return autoConvert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector(in, out, s)
}

func autoConvert_api_ObjectMeta_To_v1beta3_ObjectMeta(in *api.ObjectMeta, out *apiv1beta3.ObjectMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ObjectMeta))(in)
	}
	out.Name = in.Name
	out.GenerateName = in.GenerateName
	out.Namespace = in.Namespace
	out.SelfLink = in.SelfLink
	out.UID = in.UID
	out.ResourceVersion = in.ResourceVersion
	out.Generation = in.Generation
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.CreationTimestamp, &out.CreationTimestamp, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.DeletionTimestamp != nil {
		out.DeletionTimestamp = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.DeletionTimestamp, out.DeletionTimestamp, s); err != nil {
			return err
		}
	} else {
		out.DeletionTimestamp = nil
	}
	if in.DeletionGracePeriodSeconds != nil {
		out.DeletionGracePeriodSeconds = new(int64)
		*out.DeletionGracePeriodSeconds = *in.DeletionGracePeriodSeconds
	} else {
		out.DeletionGracePeriodSeconds = nil
	}
	if in.Labels != nil {
		out.Labels = make(map[string]string)
		for key, val := range in.Labels {
			out.Labels[key] = val
		}
	} else {
		out.Labels = nil
	}
	if in.Annotations != nil {
		out.Annotations = make(map[string]string)
		for key, val := range in.Annotations {
			out.Annotations[key] = val
		}
	} else {
		out.Annotations = nil
	}
	return nil
}

func Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(in *api.ObjectMeta, out *apiv1beta3.ObjectMeta, s conversion.Scope) error {
	return autoConvert_api_ObjectMeta_To_v1beta3_ObjectMeta(in, out, s)
}

func autoConvert_api_ObjectReference_To_v1beta3_ObjectReference(in *api.ObjectReference, out *apiv1beta3.ObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ObjectReference))(in)
	}
	out.Kind = in.Kind
	out.Namespace = in.Namespace
	out.Name = in.Name
	out.UID = in.UID
	out.APIVersion = in.APIVersion
	out.ResourceVersion = in.ResourceVersion
	out.FieldPath = in.FieldPath
	return nil
}

func Convert_api_ObjectReference_To_v1beta3_ObjectReference(in *api.ObjectReference, out *apiv1beta3.ObjectReference, s conversion.Scope) error {
	return autoConvert_api_ObjectReference_To_v1beta3_ObjectReference(in, out, s)
}

func autoConvert_api_PersistentVolumeClaimVolumeSource_To_v1beta3_PersistentVolumeClaimVolumeSource(in *api.PersistentVolumeClaimVolumeSource, out *apiv1beta3.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PersistentVolumeClaimVolumeSource))(in)
	}
	out.ClaimName = in.ClaimName
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_PersistentVolumeClaimVolumeSource_To_v1beta3_PersistentVolumeClaimVolumeSource(in *api.PersistentVolumeClaimVolumeSource, out *apiv1beta3.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	return autoConvert_api_PersistentVolumeClaimVolumeSource_To_v1beta3_PersistentVolumeClaimVolumeSource(in, out, s)
}

func autoConvert_api_PodSecurityContext_To_v1beta3_PodSecurityContext(in *api.PodSecurityContext, out *apiv1beta3.PodSecurityContext, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PodSecurityContext))(in)
	}
	// in.HostNetwork has no peer in out
	// in.HostPID has no peer in out
	// in.HostIPC has no peer in out
	// unable to generate simple pointer conversion for api.SELinuxOptions -> v1beta3.SELinuxOptions
	if in.SELinuxOptions != nil {
		out.SELinuxOptions = new(apiv1beta3.SELinuxOptions)
		if err := Convert_api_SELinuxOptions_To_v1beta3_SELinuxOptions(in.SELinuxOptions, out.SELinuxOptions, s); err != nil {
			return err
		}
	} else {
		out.SELinuxOptions = nil
	}
	if in.RunAsUser != nil {
		out.RunAsUser = new(int64)
		*out.RunAsUser = *in.RunAsUser
	} else {
		out.RunAsUser = nil
	}
	if in.RunAsNonRoot != nil {
		out.RunAsNonRoot = new(bool)
		*out.RunAsNonRoot = *in.RunAsNonRoot
	} else {
		out.RunAsNonRoot = nil
	}
	if in.SupplementalGroups != nil {
		out.SupplementalGroups = make([]int64, len(in.SupplementalGroups))
		for i := range in.SupplementalGroups {
			out.SupplementalGroups[i] = in.SupplementalGroups[i]
		}
	} else {
		out.SupplementalGroups = nil
	}
	if in.FSGroup != nil {
		out.FSGroup = new(int64)
		*out.FSGroup = *in.FSGroup
	} else {
		out.FSGroup = nil
	}
	return nil
}

func autoConvert_api_PodSpec_To_v1beta3_PodSpec(in *api.PodSpec, out *apiv1beta3.PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]apiv1beta3.Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := Convert_api_Volume_To_v1beta3_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]apiv1beta3.Container, len(in.Containers))
		for i := range in.Containers {
			if err := s.Convert(&in.Containers[i], &out.Containers[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Containers = nil
	}
	out.RestartPolicy = apiv1beta3.RestartPolicy(in.RestartPolicy)
	if in.TerminationGracePeriodSeconds != nil {
		out.TerminationGracePeriodSeconds = new(int64)
		*out.TerminationGracePeriodSeconds = *in.TerminationGracePeriodSeconds
	} else {
		out.TerminationGracePeriodSeconds = nil
	}
	if in.ActiveDeadlineSeconds != nil {
		out.ActiveDeadlineSeconds = new(int64)
		*out.ActiveDeadlineSeconds = *in.ActiveDeadlineSeconds
	} else {
		out.ActiveDeadlineSeconds = nil
	}
	out.DNSPolicy = apiv1beta3.DNSPolicy(in.DNSPolicy)
	if in.NodeSelector != nil {
		out.NodeSelector = make(map[string]string)
		for key, val := range in.NodeSelector {
			out.NodeSelector[key] = val
		}
	} else {
		out.NodeSelector = nil
	}
	// in.ServiceAccountName has no peer in out
	// in.NodeName has no peer in out
	// unable to generate simple pointer conversion for api.PodSecurityContext -> v1beta3.PodSecurityContext
	if in.SecurityContext != nil {
		if err := s.Convert(&in.SecurityContext, &out.SecurityContext, 0); err != nil {
			return err
		}
	} else {
		out.SecurityContext = nil
	}
	if in.ImagePullSecrets != nil {
		out.ImagePullSecrets = make([]apiv1beta3.LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}
	return nil
}

func autoConvert_api_PodTemplateSpec_To_v1beta3_PodTemplateSpec(in *api.PodTemplateSpec, out *apiv1beta3.PodTemplateSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PodTemplateSpec))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec, &out.Spec, 0); err != nil {
		return err
	}
	return nil
}

func Convert_api_PodTemplateSpec_To_v1beta3_PodTemplateSpec(in *api.PodTemplateSpec, out *apiv1beta3.PodTemplateSpec, s conversion.Scope) error {
	return autoConvert_api_PodTemplateSpec_To_v1beta3_PodTemplateSpec(in, out, s)
}

func autoConvert_api_RBDVolumeSource_To_v1beta3_RBDVolumeSource(in *api.RBDVolumeSource, out *apiv1beta3.RBDVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.RBDVolumeSource))(in)
	}
	if in.CephMonitors != nil {
		out.CephMonitors = make([]string, len(in.CephMonitors))
		for i := range in.CephMonitors {
			out.CephMonitors[i] = in.CephMonitors[i]
		}
	} else {
		out.CephMonitors = nil
	}
	out.RBDImage = in.RBDImage
	out.FSType = in.FSType
	out.RBDPool = in.RBDPool
	out.RadosUser = in.RadosUser
	out.Keyring = in.Keyring
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1beta3.LocalObjectReference
	if in.SecretRef != nil {
		out.SecretRef = new(apiv1beta3.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_RBDVolumeSource_To_v1beta3_RBDVolumeSource(in *api.RBDVolumeSource, out *apiv1beta3.RBDVolumeSource, s conversion.Scope) error {
	return autoConvert_api_RBDVolumeSource_To_v1beta3_RBDVolumeSource(in, out, s)
}

func autoConvert_api_ResourceRequirements_To_v1beta3_ResourceRequirements(in *api.ResourceRequirements, out *apiv1beta3.ResourceRequirements, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ResourceRequirements))(in)
	}
	if in.Limits != nil {
		out.Limits = make(apiv1beta3.ResourceList)
		for key, val := range in.Limits {
			newVal := resource.Quantity{}
			if err := api.Convert_resource_Quantity_To_resource_Quantity(&val, &newVal, s); err != nil {
				return err
			}
			out.Limits[apiv1beta3.ResourceName(key)] = newVal
		}
	} else {
		out.Limits = nil
	}
	if in.Requests != nil {
		out.Requests = make(apiv1beta3.ResourceList)
		for key, val := range in.Requests {
			newVal := resource.Quantity{}
			if err := api.Convert_resource_Quantity_To_resource_Quantity(&val, &newVal, s); err != nil {
				return err
			}
			out.Requests[apiv1beta3.ResourceName(key)] = newVal
		}
	} else {
		out.Requests = nil
	}
	return nil
}

func Convert_api_ResourceRequirements_To_v1beta3_ResourceRequirements(in *api.ResourceRequirements, out *apiv1beta3.ResourceRequirements, s conversion.Scope) error {
	return autoConvert_api_ResourceRequirements_To_v1beta3_ResourceRequirements(in, out, s)
}

func autoConvert_api_SELinuxOptions_To_v1beta3_SELinuxOptions(in *api.SELinuxOptions, out *apiv1beta3.SELinuxOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.SELinuxOptions))(in)
	}
	out.User = in.User
	out.Role = in.Role
	out.Type = in.Type
	out.Level = in.Level
	return nil
}

func Convert_api_SELinuxOptions_To_v1beta3_SELinuxOptions(in *api.SELinuxOptions, out *apiv1beta3.SELinuxOptions, s conversion.Scope) error {
	return autoConvert_api_SELinuxOptions_To_v1beta3_SELinuxOptions(in, out, s)
}

func autoConvert_api_SecretVolumeSource_To_v1beta3_SecretVolumeSource(in *api.SecretVolumeSource, out *apiv1beta3.SecretVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.SecretVolumeSource))(in)
	}
	out.SecretName = in.SecretName
	return nil
}

func Convert_api_SecretVolumeSource_To_v1beta3_SecretVolumeSource(in *api.SecretVolumeSource, out *apiv1beta3.SecretVolumeSource, s conversion.Scope) error {
	return autoConvert_api_SecretVolumeSource_To_v1beta3_SecretVolumeSource(in, out, s)
}

func autoConvert_api_TCPSocketAction_To_v1beta3_TCPSocketAction(in *api.TCPSocketAction, out *apiv1beta3.TCPSocketAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.TCPSocketAction))(in)
	}
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.Port, &out.Port, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_TCPSocketAction_To_v1beta3_TCPSocketAction(in *api.TCPSocketAction, out *apiv1beta3.TCPSocketAction, s conversion.Scope) error {
	return autoConvert_api_TCPSocketAction_To_v1beta3_TCPSocketAction(in, out, s)
}

func autoConvert_api_Volume_To_v1beta3_Volume(in *api.Volume, out *apiv1beta3.Volume, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Volume))(in)
	}
	out.Name = in.Name
	if err := s.Convert(&in.VolumeSource, &out.VolumeSource, 0); err != nil {
		return err
	}
	return nil
}

func Convert_api_Volume_To_v1beta3_Volume(in *api.Volume, out *apiv1beta3.Volume, s conversion.Scope) error {
	return autoConvert_api_Volume_To_v1beta3_Volume(in, out, s)
}

func autoConvert_api_VolumeMount_To_v1beta3_VolumeMount(in *api.VolumeMount, out *apiv1beta3.VolumeMount, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.VolumeMount))(in)
	}
	out.Name = in.Name
	out.ReadOnly = in.ReadOnly
	out.MountPath = in.MountPath
	return nil
}

func Convert_api_VolumeMount_To_v1beta3_VolumeMount(in *api.VolumeMount, out *apiv1beta3.VolumeMount, s conversion.Scope) error {
	return autoConvert_api_VolumeMount_To_v1beta3_VolumeMount(in, out, s)
}

func autoConvert_api_VolumeSource_To_v1beta3_VolumeSource(in *api.VolumeSource, out *apiv1beta3.VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.VolumeSource))(in)
	}
	// unable to generate simple pointer conversion for api.HostPathVolumeSource -> v1beta3.HostPathVolumeSource
	if in.HostPath != nil {
		out.HostPath = new(apiv1beta3.HostPathVolumeSource)
		if err := Convert_api_HostPathVolumeSource_To_v1beta3_HostPathVolumeSource(in.HostPath, out.HostPath, s); err != nil {
			return err
		}
	} else {
		out.HostPath = nil
	}
	// unable to generate simple pointer conversion for api.EmptyDirVolumeSource -> v1beta3.EmptyDirVolumeSource
	if in.EmptyDir != nil {
		out.EmptyDir = new(apiv1beta3.EmptyDirVolumeSource)
		if err := Convert_api_EmptyDirVolumeSource_To_v1beta3_EmptyDirVolumeSource(in.EmptyDir, out.EmptyDir, s); err != nil {
			return err
		}
	} else {
		out.EmptyDir = nil
	}
	// unable to generate simple pointer conversion for api.GCEPersistentDiskVolumeSource -> v1beta3.GCEPersistentDiskVolumeSource
	if in.GCEPersistentDisk != nil {
		out.GCEPersistentDisk = new(apiv1beta3.GCEPersistentDiskVolumeSource)
		if err := Convert_api_GCEPersistentDiskVolumeSource_To_v1beta3_GCEPersistentDiskVolumeSource(in.GCEPersistentDisk, out.GCEPersistentDisk, s); err != nil {
			return err
		}
	} else {
		out.GCEPersistentDisk = nil
	}
	// unable to generate simple pointer conversion for api.AWSElasticBlockStoreVolumeSource -> v1beta3.AWSElasticBlockStoreVolumeSource
	if in.AWSElasticBlockStore != nil {
		out.AWSElasticBlockStore = new(apiv1beta3.AWSElasticBlockStoreVolumeSource)
		if err := Convert_api_AWSElasticBlockStoreVolumeSource_To_v1beta3_AWSElasticBlockStoreVolumeSource(in.AWSElasticBlockStore, out.AWSElasticBlockStore, s); err != nil {
			return err
		}
	} else {
		out.AWSElasticBlockStore = nil
	}
	// unable to generate simple pointer conversion for api.GitRepoVolumeSource -> v1beta3.GitRepoVolumeSource
	if in.GitRepo != nil {
		if err := s.Convert(&in.GitRepo, &out.GitRepo, 0); err != nil {
			return err
		}
	} else {
		out.GitRepo = nil
	}
	// unable to generate simple pointer conversion for api.SecretVolumeSource -> v1beta3.SecretVolumeSource
	if in.Secret != nil {
		out.Secret = new(apiv1beta3.SecretVolumeSource)
		if err := Convert_api_SecretVolumeSource_To_v1beta3_SecretVolumeSource(in.Secret, out.Secret, s); err != nil {
			return err
		}
	} else {
		out.Secret = nil
	}
	// unable to generate simple pointer conversion for api.NFSVolumeSource -> v1beta3.NFSVolumeSource
	if in.NFS != nil {
		out.NFS = new(apiv1beta3.NFSVolumeSource)
		if err := Convert_api_NFSVolumeSource_To_v1beta3_NFSVolumeSource(in.NFS, out.NFS, s); err != nil {
			return err
		}
	} else {
		out.NFS = nil
	}
	// unable to generate simple pointer conversion for api.ISCSIVolumeSource -> v1beta3.ISCSIVolumeSource
	if in.ISCSI != nil {
		if err := s.Convert(&in.ISCSI, &out.ISCSI, 0); err != nil {
			return err
		}
	} else {
		out.ISCSI = nil
	}
	// unable to generate simple pointer conversion for api.GlusterfsVolumeSource -> v1beta3.GlusterfsVolumeSource
	if in.Glusterfs != nil {
		out.Glusterfs = new(apiv1beta3.GlusterfsVolumeSource)
		if err := Convert_api_GlusterfsVolumeSource_To_v1beta3_GlusterfsVolumeSource(in.Glusterfs, out.Glusterfs, s); err != nil {
			return err
		}
	} else {
		out.Glusterfs = nil
	}
	// unable to generate simple pointer conversion for api.PersistentVolumeClaimVolumeSource -> v1beta3.PersistentVolumeClaimVolumeSource
	if in.PersistentVolumeClaim != nil {
		out.PersistentVolumeClaim = new(apiv1beta3.PersistentVolumeClaimVolumeSource)
		if err := Convert_api_PersistentVolumeClaimVolumeSource_To_v1beta3_PersistentVolumeClaimVolumeSource(in.PersistentVolumeClaim, out.PersistentVolumeClaim, s); err != nil {
			return err
		}
	} else {
		out.PersistentVolumeClaim = nil
	}
	// unable to generate simple pointer conversion for api.RBDVolumeSource -> v1beta3.RBDVolumeSource
	if in.RBD != nil {
		out.RBD = new(apiv1beta3.RBDVolumeSource)
		if err := Convert_api_RBDVolumeSource_To_v1beta3_RBDVolumeSource(in.RBD, out.RBD, s); err != nil {
			return err
		}
	} else {
		out.RBD = nil
	}
	// in.FlexVolume has no peer in out
	// unable to generate simple pointer conversion for api.CinderVolumeSource -> v1beta3.CinderVolumeSource
	if in.Cinder != nil {
		out.Cinder = new(apiv1beta3.CinderVolumeSource)
		if err := Convert_api_CinderVolumeSource_To_v1beta3_CinderVolumeSource(in.Cinder, out.Cinder, s); err != nil {
			return err
		}
	} else {
		out.Cinder = nil
	}
	// unable to generate simple pointer conversion for api.CephFSVolumeSource -> v1beta3.CephFSVolumeSource
	if in.CephFS != nil {
		if err := s.Convert(&in.CephFS, &out.CephFS, 0); err != nil {
			return err
		}
	} else {
		out.CephFS = nil
	}
	// unable to generate simple pointer conversion for api.FlockerVolumeSource -> v1beta3.FlockerVolumeSource
	if in.Flocker != nil {
		out.Flocker = new(apiv1beta3.FlockerVolumeSource)
		if err := Convert_api_FlockerVolumeSource_To_v1beta3_FlockerVolumeSource(in.Flocker, out.Flocker, s); err != nil {
			return err
		}
	} else {
		out.Flocker = nil
	}
	// unable to generate simple pointer conversion for api.DownwardAPIVolumeSource -> v1beta3.DownwardAPIVolumeSource
	if in.DownwardAPI != nil {
		out.DownwardAPI = new(apiv1beta3.DownwardAPIVolumeSource)
		if err := Convert_api_DownwardAPIVolumeSource_To_v1beta3_DownwardAPIVolumeSource(in.DownwardAPI, out.DownwardAPI, s); err != nil {
			return err
		}
	} else {
		out.DownwardAPI = nil
	}
	// unable to generate simple pointer conversion for api.FCVolumeSource -> v1beta3.FCVolumeSource
	if in.FC != nil {
		out.FC = new(apiv1beta3.FCVolumeSource)
		if err := Convert_api_FCVolumeSource_To_v1beta3_FCVolumeSource(in.FC, out.FC, s); err != nil {
			return err
		}
	} else {
		out.FC = nil
	}
	// in.AzureFile has no peer in out
	// in.ConfigMap has no peer in out
	return nil
}

func autoConvert_v1beta3_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in *apiv1beta3.AWSElasticBlockStoreVolumeSource, out *api.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.AWSElasticBlockStoreVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.Partition = in.Partition
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1beta3_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in *apiv1beta3.AWSElasticBlockStoreVolumeSource, out *api.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in, out, s)
}

func autoConvert_v1beta3_Capabilities_To_api_Capabilities(in *apiv1beta3.Capabilities, out *api.Capabilities, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.Capabilities))(in)
	}
	if in.Add != nil {
		out.Add = make([]api.Capability, len(in.Add))
		for i := range in.Add {
			out.Add[i] = api.Capability(in.Add[i])
		}
	} else {
		out.Add = nil
	}
	if in.Drop != nil {
		out.Drop = make([]api.Capability, len(in.Drop))
		for i := range in.Drop {
			out.Drop[i] = api.Capability(in.Drop[i])
		}
	} else {
		out.Drop = nil
	}
	return nil
}

func Convert_v1beta3_Capabilities_To_api_Capabilities(in *apiv1beta3.Capabilities, out *api.Capabilities, s conversion.Scope) error {
	return autoConvert_v1beta3_Capabilities_To_api_Capabilities(in, out, s)
}

func autoConvert_v1beta3_CinderVolumeSource_To_api_CinderVolumeSource(in *apiv1beta3.CinderVolumeSource, out *api.CinderVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.CinderVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1beta3_CinderVolumeSource_To_api_CinderVolumeSource(in *apiv1beta3.CinderVolumeSource, out *api.CinderVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_CinderVolumeSource_To_api_CinderVolumeSource(in, out, s)
}

func autoConvert_v1beta3_Container_To_api_Container(in *apiv1beta3.Container, out *api.Container, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.Container))(in)
	}
	out.Name = in.Name
	out.Image = in.Image
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	if in.Args != nil {
		out.Args = make([]string, len(in.Args))
		for i := range in.Args {
			out.Args[i] = in.Args[i]
		}
	} else {
		out.Args = nil
	}
	out.WorkingDir = in.WorkingDir
	if in.Ports != nil {
		out.Ports = make([]api.ContainerPort, len(in.Ports))
		for i := range in.Ports {
			if err := Convert_v1beta3_ContainerPort_To_api_ContainerPort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ports = nil
	}
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := s.Convert(&in.Env[i], &out.Env[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	if err := Convert_v1beta3_ResourceRequirements_To_api_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if in.VolumeMounts != nil {
		out.VolumeMounts = make([]api.VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			if err := Convert_v1beta3_VolumeMount_To_api_VolumeMount(&in.VolumeMounts[i], &out.VolumeMounts[i], s); err != nil {
				return err
			}
		}
	} else {
		out.VolumeMounts = nil
	}
	// unable to generate simple pointer conversion for v1beta3.Probe -> api.Probe
	if in.LivenessProbe != nil {
		if err := s.Convert(&in.LivenessProbe, &out.LivenessProbe, 0); err != nil {
			return err
		}
	} else {
		out.LivenessProbe = nil
	}
	// unable to generate simple pointer conversion for v1beta3.Probe -> api.Probe
	if in.ReadinessProbe != nil {
		if err := s.Convert(&in.ReadinessProbe, &out.ReadinessProbe, 0); err != nil {
			return err
		}
	} else {
		out.ReadinessProbe = nil
	}
	// unable to generate simple pointer conversion for v1beta3.Lifecycle -> api.Lifecycle
	if in.Lifecycle != nil {
		if err := s.Convert(&in.Lifecycle, &out.Lifecycle, 0); err != nil {
			return err
		}
	} else {
		out.Lifecycle = nil
	}
	out.TerminationMessagePath = in.TerminationMessagePath
	// in.Privileged has no peer in out
	out.ImagePullPolicy = api.PullPolicy(in.ImagePullPolicy)
	// in.Capabilities has no peer in out
	// unable to generate simple pointer conversion for v1beta3.SecurityContext -> api.SecurityContext
	if in.SecurityContext != nil {
		if err := s.Convert(&in.SecurityContext, &out.SecurityContext, 0); err != nil {
			return err
		}
	} else {
		out.SecurityContext = nil
	}
	out.Stdin = in.Stdin
	out.StdinOnce = in.StdinOnce
	out.TTY = in.TTY
	return nil
}

func autoConvert_v1beta3_ContainerPort_To_api_ContainerPort(in *apiv1beta3.ContainerPort, out *api.ContainerPort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.ContainerPort))(in)
	}
	out.Name = in.Name
	out.HostPort = in.HostPort
	out.ContainerPort = in.ContainerPort
	out.Protocol = api.Protocol(in.Protocol)
	out.HostIP = in.HostIP
	return nil
}

func Convert_v1beta3_ContainerPort_To_api_ContainerPort(in *apiv1beta3.ContainerPort, out *api.ContainerPort, s conversion.Scope) error {
	return autoConvert_v1beta3_ContainerPort_To_api_ContainerPort(in, out, s)
}

func autoConvert_v1beta3_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(in *apiv1beta3.DownwardAPIVolumeFile, out *api.DownwardAPIVolumeFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.DownwardAPIVolumeFile))(in)
	}
	out.Path = in.Path
	if err := Convert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(in *apiv1beta3.DownwardAPIVolumeFile, out *api.DownwardAPIVolumeFile, s conversion.Scope) error {
	return autoConvert_v1beta3_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(in, out, s)
}

func autoConvert_v1beta3_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in *apiv1beta3.DownwardAPIVolumeSource, out *api.DownwardAPIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.DownwardAPIVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]api.DownwardAPIVolumeFile, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1beta3_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1beta3_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in *apiv1beta3.DownwardAPIVolumeSource, out *api.DownwardAPIVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in, out, s)
}

func autoConvert_v1beta3_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in *apiv1beta3.EmptyDirVolumeSource, out *api.EmptyDirVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.EmptyDirVolumeSource))(in)
	}
	out.Medium = api.StorageMedium(in.Medium)
	return nil
}

func Convert_v1beta3_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in *apiv1beta3.EmptyDirVolumeSource, out *api.EmptyDirVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in, out, s)
}

func autoConvert_v1beta3_ExecAction_To_api_ExecAction(in *apiv1beta3.ExecAction, out *api.ExecAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.ExecAction))(in)
	}
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	return nil
}

func Convert_v1beta3_ExecAction_To_api_ExecAction(in *apiv1beta3.ExecAction, out *api.ExecAction, s conversion.Scope) error {
	return autoConvert_v1beta3_ExecAction_To_api_ExecAction(in, out, s)
}

func autoConvert_v1beta3_FCVolumeSource_To_api_FCVolumeSource(in *apiv1beta3.FCVolumeSource, out *api.FCVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.FCVolumeSource))(in)
	}
	if in.TargetWWNs != nil {
		out.TargetWWNs = make([]string, len(in.TargetWWNs))
		for i := range in.TargetWWNs {
			out.TargetWWNs[i] = in.TargetWWNs[i]
		}
	} else {
		out.TargetWWNs = nil
	}
	if in.Lun != nil {
		out.Lun = new(int)
		*out.Lun = *in.Lun
	} else {
		out.Lun = nil
	}
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1beta3_FCVolumeSource_To_api_FCVolumeSource(in *apiv1beta3.FCVolumeSource, out *api.FCVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_FCVolumeSource_To_api_FCVolumeSource(in, out, s)
}

func autoConvert_v1beta3_FlockerVolumeSource_To_api_FlockerVolumeSource(in *apiv1beta3.FlockerVolumeSource, out *api.FlockerVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.FlockerVolumeSource))(in)
	}
	out.DatasetName = in.DatasetName
	return nil
}

func Convert_v1beta3_FlockerVolumeSource_To_api_FlockerVolumeSource(in *apiv1beta3.FlockerVolumeSource, out *api.FlockerVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_FlockerVolumeSource_To_api_FlockerVolumeSource(in, out, s)
}

func autoConvert_v1beta3_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in *apiv1beta3.GCEPersistentDiskVolumeSource, out *api.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.GCEPersistentDiskVolumeSource))(in)
	}
	out.PDName = in.PDName
	out.FSType = in.FSType
	out.Partition = in.Partition
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1beta3_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in *apiv1beta3.GCEPersistentDiskVolumeSource, out *api.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in, out, s)
}

func autoConvert_v1beta3_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in *apiv1beta3.GlusterfsVolumeSource, out *api.GlusterfsVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.GlusterfsVolumeSource))(in)
	}
	out.EndpointsName = in.EndpointsName
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1beta3_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in *apiv1beta3.GlusterfsVolumeSource, out *api.GlusterfsVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in, out, s)
}

func autoConvert_v1beta3_HostPathVolumeSource_To_api_HostPathVolumeSource(in *apiv1beta3.HostPathVolumeSource, out *api.HostPathVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.HostPathVolumeSource))(in)
	}
	out.Path = in.Path
	return nil
}

func Convert_v1beta3_HostPathVolumeSource_To_api_HostPathVolumeSource(in *apiv1beta3.HostPathVolumeSource, out *api.HostPathVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_HostPathVolumeSource_To_api_HostPathVolumeSource(in, out, s)
}

func autoConvert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in *apiv1beta3.LocalObjectReference, out *api.LocalObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.LocalObjectReference))(in)
	}
	out.Name = in.Name
	return nil
}

func Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in *apiv1beta3.LocalObjectReference, out *api.LocalObjectReference, s conversion.Scope) error {
	return autoConvert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in, out, s)
}

func autoConvert_v1beta3_NFSVolumeSource_To_api_NFSVolumeSource(in *apiv1beta3.NFSVolumeSource, out *api.NFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.NFSVolumeSource))(in)
	}
	out.Server = in.Server
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1beta3_NFSVolumeSource_To_api_NFSVolumeSource(in *apiv1beta3.NFSVolumeSource, out *api.NFSVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_NFSVolumeSource_To_api_NFSVolumeSource(in, out, s)
}

func autoConvert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector(in *apiv1beta3.ObjectFieldSelector, out *api.ObjectFieldSelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.ObjectFieldSelector))(in)
	}
	out.APIVersion = in.APIVersion
	out.FieldPath = in.FieldPath
	return nil
}

func Convert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector(in *apiv1beta3.ObjectFieldSelector, out *api.ObjectFieldSelector, s conversion.Scope) error {
	return autoConvert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector(in, out, s)
}

func autoConvert_v1beta3_ObjectMeta_To_api_ObjectMeta(in *apiv1beta3.ObjectMeta, out *api.ObjectMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.ObjectMeta))(in)
	}
	out.Name = in.Name
	out.GenerateName = in.GenerateName
	out.Namespace = in.Namespace
	out.SelfLink = in.SelfLink
	out.UID = in.UID
	out.ResourceVersion = in.ResourceVersion
	out.Generation = in.Generation
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.CreationTimestamp, &out.CreationTimestamp, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for unversioned.Time -> unversioned.Time
	if in.DeletionTimestamp != nil {
		out.DeletionTimestamp = new(unversioned.Time)
		if err := api.Convert_unversioned_Time_To_unversioned_Time(in.DeletionTimestamp, out.DeletionTimestamp, s); err != nil {
			return err
		}
	} else {
		out.DeletionTimestamp = nil
	}
	if in.DeletionGracePeriodSeconds != nil {
		out.DeletionGracePeriodSeconds = new(int64)
		*out.DeletionGracePeriodSeconds = *in.DeletionGracePeriodSeconds
	} else {
		out.DeletionGracePeriodSeconds = nil
	}
	if in.Labels != nil {
		out.Labels = make(map[string]string)
		for key, val := range in.Labels {
			out.Labels[key] = val
		}
	} else {
		out.Labels = nil
	}
	if in.Annotations != nil {
		out.Annotations = make(map[string]string)
		for key, val := range in.Annotations {
			out.Annotations[key] = val
		}
	} else {
		out.Annotations = nil
	}
	return nil
}

func Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(in *apiv1beta3.ObjectMeta, out *api.ObjectMeta, s conversion.Scope) error {
	return autoConvert_v1beta3_ObjectMeta_To_api_ObjectMeta(in, out, s)
}

func autoConvert_v1beta3_ObjectReference_To_api_ObjectReference(in *apiv1beta3.ObjectReference, out *api.ObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.ObjectReference))(in)
	}
	out.Kind = in.Kind
	out.Namespace = in.Namespace
	out.Name = in.Name
	out.UID = in.UID
	out.APIVersion = in.APIVersion
	out.ResourceVersion = in.ResourceVersion
	out.FieldPath = in.FieldPath
	return nil
}

func Convert_v1beta3_ObjectReference_To_api_ObjectReference(in *apiv1beta3.ObjectReference, out *api.ObjectReference, s conversion.Scope) error {
	return autoConvert_v1beta3_ObjectReference_To_api_ObjectReference(in, out, s)
}

func autoConvert_v1beta3_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in *apiv1beta3.PersistentVolumeClaimVolumeSource, out *api.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.PersistentVolumeClaimVolumeSource))(in)
	}
	out.ClaimName = in.ClaimName
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1beta3_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in *apiv1beta3.PersistentVolumeClaimVolumeSource, out *api.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in, out, s)
}

func autoConvert_v1beta3_PodSecurityContext_To_api_PodSecurityContext(in *apiv1beta3.PodSecurityContext, out *api.PodSecurityContext, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.PodSecurityContext))(in)
	}
	// unable to generate simple pointer conversion for v1beta3.SELinuxOptions -> api.SELinuxOptions
	if in.SELinuxOptions != nil {
		out.SELinuxOptions = new(api.SELinuxOptions)
		if err := Convert_v1beta3_SELinuxOptions_To_api_SELinuxOptions(in.SELinuxOptions, out.SELinuxOptions, s); err != nil {
			return err
		}
	} else {
		out.SELinuxOptions = nil
	}
	if in.RunAsUser != nil {
		out.RunAsUser = new(int64)
		*out.RunAsUser = *in.RunAsUser
	} else {
		out.RunAsUser = nil
	}
	if in.RunAsNonRoot != nil {
		out.RunAsNonRoot = new(bool)
		*out.RunAsNonRoot = *in.RunAsNonRoot
	} else {
		out.RunAsNonRoot = nil
	}
	if in.SupplementalGroups != nil {
		out.SupplementalGroups = make([]int64, len(in.SupplementalGroups))
		for i := range in.SupplementalGroups {
			out.SupplementalGroups[i] = in.SupplementalGroups[i]
		}
	} else {
		out.SupplementalGroups = nil
	}
	if in.FSGroup != nil {
		out.FSGroup = new(int64)
		*out.FSGroup = *in.FSGroup
	} else {
		out.FSGroup = nil
	}
	return nil
}

func autoConvert_v1beta3_PodSpec_To_api_PodSpec(in *apiv1beta3.PodSpec, out *api.PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]api.Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := Convert_v1beta3_Volume_To_api_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]api.Container, len(in.Containers))
		for i := range in.Containers {
			if err := s.Convert(&in.Containers[i], &out.Containers[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Containers = nil
	}
	out.RestartPolicy = api.RestartPolicy(in.RestartPolicy)
	if in.TerminationGracePeriodSeconds != nil {
		out.TerminationGracePeriodSeconds = new(int64)
		*out.TerminationGracePeriodSeconds = *in.TerminationGracePeriodSeconds
	} else {
		out.TerminationGracePeriodSeconds = nil
	}
	if in.ActiveDeadlineSeconds != nil {
		out.ActiveDeadlineSeconds = new(int64)
		*out.ActiveDeadlineSeconds = *in.ActiveDeadlineSeconds
	} else {
		out.ActiveDeadlineSeconds = nil
	}
	out.DNSPolicy = api.DNSPolicy(in.DNSPolicy)
	if in.NodeSelector != nil {
		out.NodeSelector = make(map[string]string)
		for key, val := range in.NodeSelector {
			out.NodeSelector[key] = val
		}
	} else {
		out.NodeSelector = nil
	}
	// in.ServiceAccount has no peer in out
	// in.Host has no peer in out
	// in.HostNetwork has no peer in out
	// in.HostPID has no peer in out
	// in.HostIPC has no peer in out
	// unable to generate simple pointer conversion for v1beta3.PodSecurityContext -> api.PodSecurityContext
	if in.SecurityContext != nil {
		if err := s.Convert(&in.SecurityContext, &out.SecurityContext, 0); err != nil {
			return err
		}
	} else {
		out.SecurityContext = nil
	}
	if in.ImagePullSecrets != nil {
		out.ImagePullSecrets = make([]api.LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}
	return nil
}

func autoConvert_v1beta3_PodTemplateSpec_To_api_PodTemplateSpec(in *apiv1beta3.PodTemplateSpec, out *api.PodTemplateSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.PodTemplateSpec))(in)
	}
	if err := Convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec, &out.Spec, 0); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_PodTemplateSpec_To_api_PodTemplateSpec(in *apiv1beta3.PodTemplateSpec, out *api.PodTemplateSpec, s conversion.Scope) error {
	return autoConvert_v1beta3_PodTemplateSpec_To_api_PodTemplateSpec(in, out, s)
}

func autoConvert_v1beta3_RBDVolumeSource_To_api_RBDVolumeSource(in *apiv1beta3.RBDVolumeSource, out *api.RBDVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.RBDVolumeSource))(in)
	}
	if in.CephMonitors != nil {
		out.CephMonitors = make([]string, len(in.CephMonitors))
		for i := range in.CephMonitors {
			out.CephMonitors[i] = in.CephMonitors[i]
		}
	} else {
		out.CephMonitors = nil
	}
	out.RBDImage = in.RBDImage
	out.FSType = in.FSType
	out.RBDPool = in.RBDPool
	out.RadosUser = in.RadosUser
	out.Keyring = in.Keyring
	// unable to generate simple pointer conversion for v1beta3.LocalObjectReference -> api.LocalObjectReference
	if in.SecretRef != nil {
		out.SecretRef = new(api.LocalObjectReference)
		if err := Convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1beta3_RBDVolumeSource_To_api_RBDVolumeSource(in *apiv1beta3.RBDVolumeSource, out *api.RBDVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_RBDVolumeSource_To_api_RBDVolumeSource(in, out, s)
}

func autoConvert_v1beta3_ResourceRequirements_To_api_ResourceRequirements(in *apiv1beta3.ResourceRequirements, out *api.ResourceRequirements, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.ResourceRequirements))(in)
	}
	if in.Limits != nil {
		out.Limits = make(api.ResourceList)
		for key, val := range in.Limits {
			newVal := resource.Quantity{}
			if err := api.Convert_resource_Quantity_To_resource_Quantity(&val, &newVal, s); err != nil {
				return err
			}
			out.Limits[api.ResourceName(key)] = newVal
		}
	} else {
		out.Limits = nil
	}
	if in.Requests != nil {
		out.Requests = make(api.ResourceList)
		for key, val := range in.Requests {
			newVal := resource.Quantity{}
			if err := api.Convert_resource_Quantity_To_resource_Quantity(&val, &newVal, s); err != nil {
				return err
			}
			out.Requests[api.ResourceName(key)] = newVal
		}
	} else {
		out.Requests = nil
	}
	return nil
}

func Convert_v1beta3_ResourceRequirements_To_api_ResourceRequirements(in *apiv1beta3.ResourceRequirements, out *api.ResourceRequirements, s conversion.Scope) error {
	return autoConvert_v1beta3_ResourceRequirements_To_api_ResourceRequirements(in, out, s)
}

func autoConvert_v1beta3_SELinuxOptions_To_api_SELinuxOptions(in *apiv1beta3.SELinuxOptions, out *api.SELinuxOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.SELinuxOptions))(in)
	}
	out.User = in.User
	out.Role = in.Role
	out.Type = in.Type
	out.Level = in.Level
	return nil
}

func Convert_v1beta3_SELinuxOptions_To_api_SELinuxOptions(in *apiv1beta3.SELinuxOptions, out *api.SELinuxOptions, s conversion.Scope) error {
	return autoConvert_v1beta3_SELinuxOptions_To_api_SELinuxOptions(in, out, s)
}

func autoConvert_v1beta3_SecretVolumeSource_To_api_SecretVolumeSource(in *apiv1beta3.SecretVolumeSource, out *api.SecretVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.SecretVolumeSource))(in)
	}
	out.SecretName = in.SecretName
	return nil
}

func Convert_v1beta3_SecretVolumeSource_To_api_SecretVolumeSource(in *apiv1beta3.SecretVolumeSource, out *api.SecretVolumeSource, s conversion.Scope) error {
	return autoConvert_v1beta3_SecretVolumeSource_To_api_SecretVolumeSource(in, out, s)
}

func autoConvert_v1beta3_TCPSocketAction_To_api_TCPSocketAction(in *apiv1beta3.TCPSocketAction, out *api.TCPSocketAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.TCPSocketAction))(in)
	}
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.Port, &out.Port, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_TCPSocketAction_To_api_TCPSocketAction(in *apiv1beta3.TCPSocketAction, out *api.TCPSocketAction, s conversion.Scope) error {
	return autoConvert_v1beta3_TCPSocketAction_To_api_TCPSocketAction(in, out, s)
}

func autoConvert_v1beta3_Volume_To_api_Volume(in *apiv1beta3.Volume, out *api.Volume, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.Volume))(in)
	}
	out.Name = in.Name
	if err := s.Convert(&in.VolumeSource, &out.VolumeSource, 0); err != nil {
		return err
	}
	return nil
}

func Convert_v1beta3_Volume_To_api_Volume(in *apiv1beta3.Volume, out *api.Volume, s conversion.Scope) error {
	return autoConvert_v1beta3_Volume_To_api_Volume(in, out, s)
}

func autoConvert_v1beta3_VolumeMount_To_api_VolumeMount(in *apiv1beta3.VolumeMount, out *api.VolumeMount, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.VolumeMount))(in)
	}
	out.Name = in.Name
	out.ReadOnly = in.ReadOnly
	out.MountPath = in.MountPath
	return nil
}

func Convert_v1beta3_VolumeMount_To_api_VolumeMount(in *apiv1beta3.VolumeMount, out *api.VolumeMount, s conversion.Scope) error {
	return autoConvert_v1beta3_VolumeMount_To_api_VolumeMount(in, out, s)
}

func autoConvert_v1beta3_VolumeSource_To_api_VolumeSource(in *apiv1beta3.VolumeSource, out *api.VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.VolumeSource))(in)
	}
	// unable to generate simple pointer conversion for v1beta3.HostPathVolumeSource -> api.HostPathVolumeSource
	if in.HostPath != nil {
		out.HostPath = new(api.HostPathVolumeSource)
		if err := Convert_v1beta3_HostPathVolumeSource_To_api_HostPathVolumeSource(in.HostPath, out.HostPath, s); err != nil {
			return err
		}
	} else {
		out.HostPath = nil
	}
	// unable to generate simple pointer conversion for v1beta3.EmptyDirVolumeSource -> api.EmptyDirVolumeSource
	if in.EmptyDir != nil {
		out.EmptyDir = new(api.EmptyDirVolumeSource)
		if err := Convert_v1beta3_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in.EmptyDir, out.EmptyDir, s); err != nil {
			return err
		}
	} else {
		out.EmptyDir = nil
	}
	// unable to generate simple pointer conversion for v1beta3.GCEPersistentDiskVolumeSource -> api.GCEPersistentDiskVolumeSource
	if in.GCEPersistentDisk != nil {
		out.GCEPersistentDisk = new(api.GCEPersistentDiskVolumeSource)
		if err := Convert_v1beta3_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in.GCEPersistentDisk, out.GCEPersistentDisk, s); err != nil {
			return err
		}
	} else {
		out.GCEPersistentDisk = nil
	}
	// unable to generate simple pointer conversion for v1beta3.AWSElasticBlockStoreVolumeSource -> api.AWSElasticBlockStoreVolumeSource
	if in.AWSElasticBlockStore != nil {
		out.AWSElasticBlockStore = new(api.AWSElasticBlockStoreVolumeSource)
		if err := Convert_v1beta3_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in.AWSElasticBlockStore, out.AWSElasticBlockStore, s); err != nil {
			return err
		}
	} else {
		out.AWSElasticBlockStore = nil
	}
	// unable to generate simple pointer conversion for v1beta3.GitRepoVolumeSource -> api.GitRepoVolumeSource
	if in.GitRepo != nil {
		if err := s.Convert(&in.GitRepo, &out.GitRepo, 0); err != nil {
			return err
		}
	} else {
		out.GitRepo = nil
	}
	// unable to generate simple pointer conversion for v1beta3.SecretVolumeSource -> api.SecretVolumeSource
	if in.Secret != nil {
		out.Secret = new(api.SecretVolumeSource)
		if err := Convert_v1beta3_SecretVolumeSource_To_api_SecretVolumeSource(in.Secret, out.Secret, s); err != nil {
			return err
		}
	} else {
		out.Secret = nil
	}
	// unable to generate simple pointer conversion for v1beta3.NFSVolumeSource -> api.NFSVolumeSource
	if in.NFS != nil {
		out.NFS = new(api.NFSVolumeSource)
		if err := Convert_v1beta3_NFSVolumeSource_To_api_NFSVolumeSource(in.NFS, out.NFS, s); err != nil {
			return err
		}
	} else {
		out.NFS = nil
	}
	// unable to generate simple pointer conversion for v1beta3.ISCSIVolumeSource -> api.ISCSIVolumeSource
	if in.ISCSI != nil {
		if err := s.Convert(&in.ISCSI, &out.ISCSI, 0); err != nil {
			return err
		}
	} else {
		out.ISCSI = nil
	}
	// unable to generate simple pointer conversion for v1beta3.GlusterfsVolumeSource -> api.GlusterfsVolumeSource
	if in.Glusterfs != nil {
		out.Glusterfs = new(api.GlusterfsVolumeSource)
		if err := Convert_v1beta3_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in.Glusterfs, out.Glusterfs, s); err != nil {
			return err
		}
	} else {
		out.Glusterfs = nil
	}
	// unable to generate simple pointer conversion for v1beta3.PersistentVolumeClaimVolumeSource -> api.PersistentVolumeClaimVolumeSource
	if in.PersistentVolumeClaim != nil {
		out.PersistentVolumeClaim = new(api.PersistentVolumeClaimVolumeSource)
		if err := Convert_v1beta3_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in.PersistentVolumeClaim, out.PersistentVolumeClaim, s); err != nil {
			return err
		}
	} else {
		out.PersistentVolumeClaim = nil
	}
	// unable to generate simple pointer conversion for v1beta3.RBDVolumeSource -> api.RBDVolumeSource
	if in.RBD != nil {
		out.RBD = new(api.RBDVolumeSource)
		if err := Convert_v1beta3_RBDVolumeSource_To_api_RBDVolumeSource(in.RBD, out.RBD, s); err != nil {
			return err
		}
	} else {
		out.RBD = nil
	}
	// unable to generate simple pointer conversion for v1beta3.CinderVolumeSource -> api.CinderVolumeSource
	if in.Cinder != nil {
		out.Cinder = new(api.CinderVolumeSource)
		if err := Convert_v1beta3_CinderVolumeSource_To_api_CinderVolumeSource(in.Cinder, out.Cinder, s); err != nil {
			return err
		}
	} else {
		out.Cinder = nil
	}
	// unable to generate simple pointer conversion for v1beta3.CephFSVolumeSource -> api.CephFSVolumeSource
	if in.CephFS != nil {
		if err := s.Convert(&in.CephFS, &out.CephFS, 0); err != nil {
			return err
		}
	} else {
		out.CephFS = nil
	}
	// unable to generate simple pointer conversion for v1beta3.FlockerVolumeSource -> api.FlockerVolumeSource
	if in.Flocker != nil {
		out.Flocker = new(api.FlockerVolumeSource)
		if err := Convert_v1beta3_FlockerVolumeSource_To_api_FlockerVolumeSource(in.Flocker, out.Flocker, s); err != nil {
			return err
		}
	} else {
		out.Flocker = nil
	}
	// unable to generate simple pointer conversion for v1beta3.DownwardAPIVolumeSource -> api.DownwardAPIVolumeSource
	if in.DownwardAPI != nil {
		out.DownwardAPI = new(api.DownwardAPIVolumeSource)
		if err := Convert_v1beta3_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in.DownwardAPI, out.DownwardAPI, s); err != nil {
			return err
		}
	} else {
		out.DownwardAPI = nil
	}
	// unable to generate simple pointer conversion for v1beta3.FCVolumeSource -> api.FCVolumeSource
	if in.FC != nil {
		out.FC = new(api.FCVolumeSource)
		if err := Convert_v1beta3_FCVolumeSource_To_api_FCVolumeSource(in.FC, out.FC, s); err != nil {
			return err
		}
	} else {
		out.FC = nil
	}
	// in.Metadata has no peer in out
	return nil
}

func init() {
	err := api.Scheme.AddGeneratedConversionFuncs(
		autoConvert_api_AWSElasticBlockStoreVolumeSource_To_v1beta3_AWSElasticBlockStoreVolumeSource,
		autoConvert_api_BinaryBuildRequestOptions_To_v1beta3_BinaryBuildRequestOptions,
		autoConvert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource,
		autoConvert_api_BuildConfigList_To_v1beta3_BuildConfigList,
		autoConvert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec,
		autoConvert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus,
		autoConvert_api_BuildConfig_To_v1beta3_BuildConfig,
		autoConvert_api_BuildList_To_v1beta3_BuildList,
		autoConvert_api_BuildLogOptions_To_v1beta3_BuildLogOptions,
		autoConvert_api_BuildLog_To_v1beta3_BuildLog,
		autoConvert_api_BuildOutput_To_v1beta3_BuildOutput,
		autoConvert_api_BuildPostCommitSpec_To_v1beta3_BuildPostCommitSpec,
		autoConvert_api_BuildSource_To_v1beta3_BuildSource,
		autoConvert_api_BuildSpec_To_v1beta3_BuildSpec,
		autoConvert_api_BuildStatus_To_v1beta3_BuildStatus,
		autoConvert_api_BuildStrategy_To_v1beta3_BuildStrategy,
		autoConvert_api_BuildTriggerPolicy_To_v1beta3_BuildTriggerPolicy,
		autoConvert_api_Build_To_v1beta3_Build,
		autoConvert_api_Capabilities_To_v1beta3_Capabilities,
		autoConvert_api_CinderVolumeSource_To_v1beta3_CinderVolumeSource,
		autoConvert_api_ClusterNetworkList_To_v1beta3_ClusterNetworkList,
		autoConvert_api_ClusterNetwork_To_v1beta3_ClusterNetwork,
		autoConvert_api_ClusterPolicyBindingList_To_v1beta3_ClusterPolicyBindingList,
		autoConvert_api_ClusterPolicyBinding_To_v1beta3_ClusterPolicyBinding,
		autoConvert_api_ClusterPolicyList_To_v1beta3_ClusterPolicyList,
		autoConvert_api_ClusterPolicy_To_v1beta3_ClusterPolicy,
		autoConvert_api_ClusterRoleBindingList_To_v1beta3_ClusterRoleBindingList,
		autoConvert_api_ClusterRoleBinding_To_v1beta3_ClusterRoleBinding,
		autoConvert_api_ClusterRoleList_To_v1beta3_ClusterRoleList,
		autoConvert_api_ClusterRole_To_v1beta3_ClusterRole,
		autoConvert_api_ContainerPort_To_v1beta3_ContainerPort,
		autoConvert_api_Container_To_v1beta3_Container,
		autoConvert_api_CustomBuildStrategy_To_v1beta3_CustomBuildStrategy,
		autoConvert_api_DeploymentCauseImageTrigger_To_v1beta3_DeploymentCauseImageTrigger,
		autoConvert_api_DeploymentCause_To_v1beta3_DeploymentCause,
		autoConvert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec,
		autoConvert_api_DeploymentConfigRollback_To_v1beta3_DeploymentConfigRollback,
		autoConvert_api_DeploymentDetails_To_v1beta3_DeploymentDetails,
		autoConvert_api_DeploymentLogOptions_To_v1beta3_DeploymentLogOptions,
		autoConvert_api_DeploymentLog_To_v1beta3_DeploymentLog,
		autoConvert_api_DeploymentTriggerImageChangeParams_To_v1beta3_DeploymentTriggerImageChangeParams,
		autoConvert_api_DeploymentTriggerPolicy_To_v1beta3_DeploymentTriggerPolicy,
		autoConvert_api_DockerBuildStrategy_To_v1beta3_DockerBuildStrategy,
		autoConvert_api_DownwardAPIVolumeFile_To_v1beta3_DownwardAPIVolumeFile,
		autoConvert_api_DownwardAPIVolumeSource_To_v1beta3_DownwardAPIVolumeSource,
		autoConvert_api_EmptyDirVolumeSource_To_v1beta3_EmptyDirVolumeSource,
		autoConvert_api_ExecAction_To_v1beta3_ExecAction,
		autoConvert_api_FCVolumeSource_To_v1beta3_FCVolumeSource,
		autoConvert_api_FlockerVolumeSource_To_v1beta3_FlockerVolumeSource,
		autoConvert_api_GCEPersistentDiskVolumeSource_To_v1beta3_GCEPersistentDiskVolumeSource,
		autoConvert_api_GitBuildSource_To_v1beta3_GitBuildSource,
		autoConvert_api_GitSourceRevision_To_v1beta3_GitSourceRevision,
		autoConvert_api_GlusterfsVolumeSource_To_v1beta3_GlusterfsVolumeSource,
		autoConvert_api_GroupList_To_v1beta3_GroupList,
		autoConvert_api_Group_To_v1beta3_Group,
		autoConvert_api_HostPathVolumeSource_To_v1beta3_HostPathVolumeSource,
		autoConvert_api_HostSubnetList_To_v1beta3_HostSubnetList,
		autoConvert_api_HostSubnet_To_v1beta3_HostSubnet,
		autoConvert_api_IdentityList_To_v1beta3_IdentityList,
		autoConvert_api_Identity_To_v1beta3_Identity,
		autoConvert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger,
		autoConvert_api_ImageList_To_v1beta3_ImageList,
		autoConvert_api_ImageSourcePath_To_v1beta3_ImageSourcePath,
		autoConvert_api_ImageSource_To_v1beta3_ImageSource,
		autoConvert_api_ImageStreamImage_To_v1beta3_ImageStreamImage,
		autoConvert_api_ImageStreamList_To_v1beta3_ImageStreamList,
		autoConvert_api_ImageStreamMapping_To_v1beta3_ImageStreamMapping,
		autoConvert_api_ImageStreamSpec_To_v1beta3_ImageStreamSpec,
		autoConvert_api_ImageStreamStatus_To_v1beta3_ImageStreamStatus,
		autoConvert_api_ImageStreamTagList_To_v1beta3_ImageStreamTagList,
		autoConvert_api_ImageStreamTag_To_v1beta3_ImageStreamTag,
		autoConvert_api_ImageStream_To_v1beta3_ImageStream,
		autoConvert_api_Image_To_v1beta3_Image,
		autoConvert_api_IsPersonalSubjectAccessReview_To_v1beta3_IsPersonalSubjectAccessReview,
		autoConvert_api_LocalObjectReference_To_v1beta3_LocalObjectReference,
		autoConvert_api_LocalResourceAccessReview_To_v1beta3_LocalResourceAccessReview,
		autoConvert_api_LocalSubjectAccessReview_To_v1beta3_LocalSubjectAccessReview,
		autoConvert_api_NFSVolumeSource_To_v1beta3_NFSVolumeSource,
		autoConvert_api_NetNamespaceList_To_v1beta3_NetNamespaceList,
		autoConvert_api_NetNamespace_To_v1beta3_NetNamespace,
		autoConvert_api_OAuthAccessTokenList_To_v1beta3_OAuthAccessTokenList,
		autoConvert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken,
		autoConvert_api_OAuthAuthorizeTokenList_To_v1beta3_OAuthAuthorizeTokenList,
		autoConvert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken,
		autoConvert_api_OAuthClientAuthorizationList_To_v1beta3_OAuthClientAuthorizationList,
		autoConvert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization,
		autoConvert_api_OAuthClientList_To_v1beta3_OAuthClientList,
		autoConvert_api_OAuthClient_To_v1beta3_OAuthClient,
		autoConvert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector,
		autoConvert_api_ObjectMeta_To_v1beta3_ObjectMeta,
		autoConvert_api_ObjectReference_To_v1beta3_ObjectReference,
		autoConvert_api_Parameter_To_v1beta3_Parameter,
		autoConvert_api_PersistentVolumeClaimVolumeSource_To_v1beta3_PersistentVolumeClaimVolumeSource,
		autoConvert_api_PodSecurityContext_To_v1beta3_PodSecurityContext,
		autoConvert_api_PodSpec_To_v1beta3_PodSpec,
		autoConvert_api_PodTemplateSpec_To_v1beta3_PodTemplateSpec,
		autoConvert_api_PolicyBindingList_To_v1beta3_PolicyBindingList,
		autoConvert_api_PolicyBinding_To_v1beta3_PolicyBinding,
		autoConvert_api_PolicyList_To_v1beta3_PolicyList,
		autoConvert_api_PolicyRule_To_v1beta3_PolicyRule,
		autoConvert_api_Policy_To_v1beta3_Policy,
		autoConvert_api_ProjectList_To_v1beta3_ProjectList,
		autoConvert_api_ProjectRequest_To_v1beta3_ProjectRequest,
		autoConvert_api_ProjectSpec_To_v1beta3_ProjectSpec,
		autoConvert_api_ProjectStatus_To_v1beta3_ProjectStatus,
		autoConvert_api_Project_To_v1beta3_Project,
		autoConvert_api_RBDVolumeSource_To_v1beta3_RBDVolumeSource,
		autoConvert_api_ResourceAccessReviewResponse_To_v1beta3_ResourceAccessReviewResponse,
		autoConvert_api_ResourceAccessReview_To_v1beta3_ResourceAccessReview,
		autoConvert_api_ResourceRequirements_To_v1beta3_ResourceRequirements,
		autoConvert_api_RoleBindingList_To_v1beta3_RoleBindingList,
		autoConvert_api_RoleBinding_To_v1beta3_RoleBinding,
		autoConvert_api_RoleList_To_v1beta3_RoleList,
		autoConvert_api_Role_To_v1beta3_Role,
		autoConvert_api_RollingDeploymentStrategyParams_To_v1beta3_RollingDeploymentStrategyParams,
		autoConvert_api_RouteIngressCondition_To_v1beta3_RouteIngressCondition,
		autoConvert_api_RouteIngress_To_v1beta3_RouteIngress,
		autoConvert_api_RouteList_To_v1beta3_RouteList,
		autoConvert_api_RoutePort_To_v1beta3_RoutePort,
		autoConvert_api_RouteSpec_To_v1beta3_RouteSpec,
		autoConvert_api_RouteStatus_To_v1beta3_RouteStatus,
		autoConvert_api_Route_To_v1beta3_Route,
		autoConvert_api_SELinuxOptions_To_v1beta3_SELinuxOptions,
		autoConvert_api_SecretBuildSource_To_v1beta3_SecretBuildSource,
		autoConvert_api_SecretSpec_To_v1beta3_SecretSpec,
		autoConvert_api_SecretVolumeSource_To_v1beta3_SecretVolumeSource,
		autoConvert_api_SourceBuildStrategy_To_v1beta3_SourceBuildStrategy,
		autoConvert_api_SourceControlUser_To_v1beta3_SourceControlUser,
		autoConvert_api_SourceRevision_To_v1beta3_SourceRevision,
		autoConvert_api_SubjectAccessReviewResponse_To_v1beta3_SubjectAccessReviewResponse,
		autoConvert_api_SubjectAccessReview_To_v1beta3_SubjectAccessReview,
		autoConvert_api_TCPSocketAction_To_v1beta3_TCPSocketAction,
		autoConvert_api_TLSConfig_To_v1beta3_TLSConfig,
		autoConvert_api_TagImageHook_To_v1beta3_TagImageHook,
		autoConvert_api_TemplateList_To_v1beta3_TemplateList,
		autoConvert_api_Template_To_v1beta3_Template,
		autoConvert_api_UserIdentityMapping_To_v1beta3_UserIdentityMapping,
		autoConvert_api_UserList_To_v1beta3_UserList,
		autoConvert_api_User_To_v1beta3_User,
		autoConvert_api_VolumeMount_To_v1beta3_VolumeMount,
		autoConvert_api_VolumeSource_To_v1beta3_VolumeSource,
		autoConvert_api_Volume_To_v1beta3_Volume,
		autoConvert_api_WebHookTrigger_To_v1beta3_WebHookTrigger,
		autoConvert_v1beta3_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource,
		autoConvert_v1beta3_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions,
		autoConvert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource,
		autoConvert_v1beta3_BuildConfigList_To_api_BuildConfigList,
		autoConvert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec,
		autoConvert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus,
		autoConvert_v1beta3_BuildConfig_To_api_BuildConfig,
		autoConvert_v1beta3_BuildList_To_api_BuildList,
		autoConvert_v1beta3_BuildLogOptions_To_api_BuildLogOptions,
		autoConvert_v1beta3_BuildLog_To_api_BuildLog,
		autoConvert_v1beta3_BuildOutput_To_api_BuildOutput,
		autoConvert_v1beta3_BuildPostCommitSpec_To_api_BuildPostCommitSpec,
		autoConvert_v1beta3_BuildSource_To_api_BuildSource,
		autoConvert_v1beta3_BuildSpec_To_api_BuildSpec,
		autoConvert_v1beta3_BuildStatus_To_api_BuildStatus,
		autoConvert_v1beta3_BuildStrategy_To_api_BuildStrategy,
		autoConvert_v1beta3_BuildTriggerPolicy_To_api_BuildTriggerPolicy,
		autoConvert_v1beta3_Build_To_api_Build,
		autoConvert_v1beta3_Capabilities_To_api_Capabilities,
		autoConvert_v1beta3_CinderVolumeSource_To_api_CinderVolumeSource,
		autoConvert_v1beta3_ClusterNetworkList_To_api_ClusterNetworkList,
		autoConvert_v1beta3_ClusterNetwork_To_api_ClusterNetwork,
		autoConvert_v1beta3_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList,
		autoConvert_v1beta3_ClusterPolicyBinding_To_api_ClusterPolicyBinding,
		autoConvert_v1beta3_ClusterPolicyList_To_api_ClusterPolicyList,
		autoConvert_v1beta3_ClusterPolicy_To_api_ClusterPolicy,
		autoConvert_v1beta3_ClusterRoleBindingList_To_api_ClusterRoleBindingList,
		autoConvert_v1beta3_ClusterRoleBinding_To_api_ClusterRoleBinding,
		autoConvert_v1beta3_ClusterRoleList_To_api_ClusterRoleList,
		autoConvert_v1beta3_ClusterRole_To_api_ClusterRole,
		autoConvert_v1beta3_ContainerPort_To_api_ContainerPort,
		autoConvert_v1beta3_Container_To_api_Container,
		autoConvert_v1beta3_CustomBuildStrategy_To_api_CustomBuildStrategy,
		autoConvert_v1beta3_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger,
		autoConvert_v1beta3_DeploymentCause_To_api_DeploymentCause,
		autoConvert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec,
		autoConvert_v1beta3_DeploymentConfigRollback_To_api_DeploymentConfigRollback,
		autoConvert_v1beta3_DeploymentConfigStatus_To_api_DeploymentConfigStatus,
		autoConvert_v1beta3_DeploymentDetails_To_api_DeploymentDetails,
		autoConvert_v1beta3_DeploymentLogOptions_To_api_DeploymentLogOptions,
		autoConvert_v1beta3_DeploymentLog_To_api_DeploymentLog,
		autoConvert_v1beta3_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams,
		autoConvert_v1beta3_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy,
		autoConvert_v1beta3_DockerBuildStrategy_To_api_DockerBuildStrategy,
		autoConvert_v1beta3_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile,
		autoConvert_v1beta3_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource,
		autoConvert_v1beta3_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource,
		autoConvert_v1beta3_ExecAction_To_api_ExecAction,
		autoConvert_v1beta3_FCVolumeSource_To_api_FCVolumeSource,
		autoConvert_v1beta3_FlockerVolumeSource_To_api_FlockerVolumeSource,
		autoConvert_v1beta3_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource,
		autoConvert_v1beta3_GitBuildSource_To_api_GitBuildSource,
		autoConvert_v1beta3_GitSourceRevision_To_api_GitSourceRevision,
		autoConvert_v1beta3_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource,
		autoConvert_v1beta3_GroupList_To_api_GroupList,
		autoConvert_v1beta3_Group_To_api_Group,
		autoConvert_v1beta3_HostPathVolumeSource_To_api_HostPathVolumeSource,
		autoConvert_v1beta3_HostSubnetList_To_api_HostSubnetList,
		autoConvert_v1beta3_HostSubnet_To_api_HostSubnet,
		autoConvert_v1beta3_IdentityList_To_api_IdentityList,
		autoConvert_v1beta3_Identity_To_api_Identity,
		autoConvert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger,
		autoConvert_v1beta3_ImageList_To_api_ImageList,
		autoConvert_v1beta3_ImageSourcePath_To_api_ImageSourcePath,
		autoConvert_v1beta3_ImageSource_To_api_ImageSource,
		autoConvert_v1beta3_ImageStreamImage_To_api_ImageStreamImage,
		autoConvert_v1beta3_ImageStreamList_To_api_ImageStreamList,
		autoConvert_v1beta3_ImageStreamMapping_To_api_ImageStreamMapping,
		autoConvert_v1beta3_ImageStreamSpec_To_api_ImageStreamSpec,
		autoConvert_v1beta3_ImageStreamStatus_To_api_ImageStreamStatus,
		autoConvert_v1beta3_ImageStreamTagList_To_api_ImageStreamTagList,
		autoConvert_v1beta3_ImageStreamTag_To_api_ImageStreamTag,
		autoConvert_v1beta3_ImageStream_To_api_ImageStream,
		autoConvert_v1beta3_Image_To_api_Image,
		autoConvert_v1beta3_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview,
		autoConvert_v1beta3_LocalObjectReference_To_api_LocalObjectReference,
		autoConvert_v1beta3_LocalResourceAccessReview_To_api_LocalResourceAccessReview,
		autoConvert_v1beta3_LocalSubjectAccessReview_To_api_LocalSubjectAccessReview,
		autoConvert_v1beta3_NFSVolumeSource_To_api_NFSVolumeSource,
		autoConvert_v1beta3_NetNamespaceList_To_api_NetNamespaceList,
		autoConvert_v1beta3_NetNamespace_To_api_NetNamespace,
		autoConvert_v1beta3_OAuthAccessTokenList_To_api_OAuthAccessTokenList,
		autoConvert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken,
		autoConvert_v1beta3_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList,
		autoConvert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken,
		autoConvert_v1beta3_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList,
		autoConvert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization,
		autoConvert_v1beta3_OAuthClientList_To_api_OAuthClientList,
		autoConvert_v1beta3_OAuthClient_To_api_OAuthClient,
		autoConvert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector,
		autoConvert_v1beta3_ObjectMeta_To_api_ObjectMeta,
		autoConvert_v1beta3_ObjectReference_To_api_ObjectReference,
		autoConvert_v1beta3_Parameter_To_api_Parameter,
		autoConvert_v1beta3_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource,
		autoConvert_v1beta3_PodSecurityContext_To_api_PodSecurityContext,
		autoConvert_v1beta3_PodSpec_To_api_PodSpec,
		autoConvert_v1beta3_PodTemplateSpec_To_api_PodTemplateSpec,
		autoConvert_v1beta3_PolicyBindingList_To_api_PolicyBindingList,
		autoConvert_v1beta3_PolicyBinding_To_api_PolicyBinding,
		autoConvert_v1beta3_PolicyList_To_api_PolicyList,
		autoConvert_v1beta3_PolicyRule_To_api_PolicyRule,
		autoConvert_v1beta3_Policy_To_api_Policy,
		autoConvert_v1beta3_ProjectList_To_api_ProjectList,
		autoConvert_v1beta3_ProjectRequest_To_api_ProjectRequest,
		autoConvert_v1beta3_ProjectSpec_To_api_ProjectSpec,
		autoConvert_v1beta3_ProjectStatus_To_api_ProjectStatus,
		autoConvert_v1beta3_Project_To_api_Project,
		autoConvert_v1beta3_RBDVolumeSource_To_api_RBDVolumeSource,
		autoConvert_v1beta3_ResourceAccessReviewResponse_To_api_ResourceAccessReviewResponse,
		autoConvert_v1beta3_ResourceAccessReview_To_api_ResourceAccessReview,
		autoConvert_v1beta3_ResourceRequirements_To_api_ResourceRequirements,
		autoConvert_v1beta3_RoleBindingList_To_api_RoleBindingList,
		autoConvert_v1beta3_RoleBinding_To_api_RoleBinding,
		autoConvert_v1beta3_RoleList_To_api_RoleList,
		autoConvert_v1beta3_Role_To_api_Role,
		autoConvert_v1beta3_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams,
		autoConvert_v1beta3_RouteIngressCondition_To_api_RouteIngressCondition,
		autoConvert_v1beta3_RouteIngress_To_api_RouteIngress,
		autoConvert_v1beta3_RouteList_To_api_RouteList,
		autoConvert_v1beta3_RoutePort_To_api_RoutePort,
		autoConvert_v1beta3_RouteSpec_To_api_RouteSpec,
		autoConvert_v1beta3_RouteStatus_To_api_RouteStatus,
		autoConvert_v1beta3_Route_To_api_Route,
		autoConvert_v1beta3_SELinuxOptions_To_api_SELinuxOptions,
		autoConvert_v1beta3_SecretBuildSource_To_api_SecretBuildSource,
		autoConvert_v1beta3_SecretSpec_To_api_SecretSpec,
		autoConvert_v1beta3_SecretVolumeSource_To_api_SecretVolumeSource,
		autoConvert_v1beta3_SourceBuildStrategy_To_api_SourceBuildStrategy,
		autoConvert_v1beta3_SourceControlUser_To_api_SourceControlUser,
		autoConvert_v1beta3_SourceRevision_To_api_SourceRevision,
		autoConvert_v1beta3_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse,
		autoConvert_v1beta3_SubjectAccessReview_To_api_SubjectAccessReview,
		autoConvert_v1beta3_TCPSocketAction_To_api_TCPSocketAction,
		autoConvert_v1beta3_TLSConfig_To_api_TLSConfig,
		autoConvert_v1beta3_TagImageHook_To_api_TagImageHook,
		autoConvert_v1beta3_TemplateList_To_api_TemplateList,
		autoConvert_v1beta3_Template_To_api_Template,
		autoConvert_v1beta3_UserIdentityMapping_To_api_UserIdentityMapping,
		autoConvert_v1beta3_UserList_To_api_UserList,
		autoConvert_v1beta3_User_To_api_User,
		autoConvert_v1beta3_VolumeMount_To_api_VolumeMount,
		autoConvert_v1beta3_VolumeSource_To_api_VolumeSource,
		autoConvert_v1beta3_Volume_To_api_Volume,
		autoConvert_v1beta3_WebHookTrigger_To_api_WebHookTrigger,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

// AUTO-GENERATED FUNCTIONS END HERE
