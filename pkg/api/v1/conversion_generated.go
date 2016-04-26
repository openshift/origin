package v1

// AUTO-GENERATED FUNCTIONS START HERE
import (
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/api/v1"
	buildapi "github.com/openshift/origin/pkg/build/api"
	v1 "github.com/openshift/origin/pkg/build/api/v1"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imageapiv1 "github.com/openshift/origin/pkg/image/api/v1"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/api/v1"
	projectapi "github.com/openshift/origin/pkg/project/api"
	projectapiv1 "github.com/openshift/origin/pkg/project/api/v1"
	routeapi "github.com/openshift/origin/pkg/route/api"
	routeapiv1 "github.com/openshift/origin/pkg/route/api/v1"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	sdnapiv1 "github.com/openshift/origin/pkg/sdn/api/v1"
	templateapi "github.com/openshift/origin/pkg/template/api"
	templateapiv1 "github.com/openshift/origin/pkg/template/api/v1"
	userapi "github.com/openshift/origin/pkg/user/api"
	userapiv1 "github.com/openshift/origin/pkg/user/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	resource "k8s.io/kubernetes/pkg/api/resource"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	apiv1 "k8s.io/kubernetes/pkg/api/v1"
	batchv1 "k8s.io/kubernetes/pkg/apis/batch/v1"
	conversion "k8s.io/kubernetes/pkg/conversion"
	runtime "k8s.io/kubernetes/pkg/runtime"
	reflect "reflect"
)

func autoConvert_api_ClusterPolicy_To_v1_ClusterPolicy(in *authorizationapi.ClusterPolicy, out *authorizationapiv1.ClusterPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterPolicy))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := authorizationapiv1.Convert_api_ClusterRoleArray_to_v1_NamedClusterRoleArray(&in.Roles, &out.Roles, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ClusterPolicyBinding_To_v1_ClusterPolicyBinding(in *authorizationapi.ClusterPolicyBinding, out *authorizationapiv1.ClusterPolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterPolicyBinding))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := authorizationapiv1.Convert_api_ClusterRoleBindingArray_to_v1_NamedClusterRoleBindingArray(&in.RoleBindings, &out.RoleBindings, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList(in *authorizationapi.ClusterPolicyBindingList, out *authorizationapiv1.ClusterPolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterPolicyBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1.ClusterPolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_api_ClusterPolicyBinding_To_v1_ClusterPolicyBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList(in *authorizationapi.ClusterPolicyBindingList, out *authorizationapiv1.ClusterPolicyBindingList, s conversion.Scope) error {
	return autoConvert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList(in, out, s)
}

func autoConvert_api_ClusterPolicyList_To_v1_ClusterPolicyList(in *authorizationapi.ClusterPolicyList, out *authorizationapiv1.ClusterPolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterPolicyList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1.ClusterPolicy, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_api_ClusterPolicy_To_v1_ClusterPolicy(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterPolicyList_To_v1_ClusterPolicyList(in *authorizationapi.ClusterPolicyList, out *authorizationapiv1.ClusterPolicyList, s conversion.Scope) error {
	return autoConvert_api_ClusterPolicyList_To_v1_ClusterPolicyList(in, out, s)
}

func autoConvert_api_ClusterRole_To_v1_ClusterRole(in *authorizationapi.ClusterRole, out *authorizationapiv1.ClusterRole, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterRole))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]authorizationapiv1.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := authorizationapiv1.Convert_api_PolicyRule_To_v1_PolicyRule(&in.Rules[i], &out.Rules[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_api_ClusterRole_To_v1_ClusterRole(in *authorizationapi.ClusterRole, out *authorizationapiv1.ClusterRole, s conversion.Scope) error {
	return autoConvert_api_ClusterRole_To_v1_ClusterRole(in, out, s)
}

func autoConvert_api_ClusterRoleBinding_To_v1_ClusterRoleBinding(in *authorizationapi.ClusterRoleBinding, out *authorizationapiv1.ClusterRoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterRoleBinding))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		out.Subjects = make([]apiv1.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList(in *authorizationapi.ClusterRoleBindingList, out *authorizationapiv1.ClusterRoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterRoleBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1.ClusterRoleBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_api_ClusterRoleBinding_To_v1_ClusterRoleBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList(in *authorizationapi.ClusterRoleBindingList, out *authorizationapiv1.ClusterRoleBindingList, s conversion.Scope) error {
	return autoConvert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList(in, out, s)
}

func autoConvert_api_ClusterRoleList_To_v1_ClusterRoleList(in *authorizationapi.ClusterRoleList, out *authorizationapiv1.ClusterRoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ClusterRoleList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_ClusterRole_To_v1_ClusterRole(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterRoleList_To_v1_ClusterRoleList(in *authorizationapi.ClusterRoleList, out *authorizationapiv1.ClusterRoleList, s conversion.Scope) error {
	return autoConvert_api_ClusterRoleList_To_v1_ClusterRoleList(in, out, s)
}

func autoConvert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(in *authorizationapi.IsPersonalSubjectAccessReview, out *authorizationapiv1.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.IsPersonalSubjectAccessReview))(in)
	}
	return nil
}

func Convert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(in *authorizationapi.IsPersonalSubjectAccessReview, out *authorizationapiv1.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoConvert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(in, out, s)
}

func autoConvert_api_LocalResourceAccessReview_To_v1_LocalResourceAccessReview(in *authorizationapi.LocalResourceAccessReview, out *authorizationapiv1.LocalResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.LocalResourceAccessReview))(in)
	}
	// in.Action has no peer in out
	return nil
}

func autoConvert_api_LocalSubjectAccessReview_To_v1_LocalSubjectAccessReview(in *authorizationapi.LocalSubjectAccessReview, out *authorizationapiv1.LocalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.LocalSubjectAccessReview))(in)
	}
	// in.Action has no peer in out
	out.User = in.User
	// in.Groups has no peer in out
	return nil
}

func autoConvert_api_Policy_To_v1_Policy(in *authorizationapi.Policy, out *authorizationapiv1.Policy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.Policy))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := authorizationapiv1.Convert_api_NamedRoleArray_to_v1_RoleArray(&in.Roles, &out.Roles, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_PolicyBinding_To_v1_PolicyBinding(in *authorizationapi.PolicyBinding, out *authorizationapiv1.PolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.PolicyBinding))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := authorizationapiv1.Convert_api_RoleBindingArray_to_v1_NamedRoleBindingArray(&in.RoleBindings, &out.RoleBindings, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_PolicyBindingList_To_v1_PolicyBindingList(in *authorizationapi.PolicyBindingList, out *authorizationapiv1.PolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.PolicyBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1.PolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_api_PolicyBinding_To_v1_PolicyBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_PolicyBindingList_To_v1_PolicyBindingList(in *authorizationapi.PolicyBindingList, out *authorizationapiv1.PolicyBindingList, s conversion.Scope) error {
	return autoConvert_api_PolicyBindingList_To_v1_PolicyBindingList(in, out, s)
}

func autoConvert_api_PolicyList_To_v1_PolicyList(in *authorizationapi.PolicyList, out *authorizationapiv1.PolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.PolicyList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1.Policy, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_api_Policy_To_v1_Policy(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_PolicyList_To_v1_PolicyList(in *authorizationapi.PolicyList, out *authorizationapiv1.PolicyList, s conversion.Scope) error {
	return autoConvert_api_PolicyList_To_v1_PolicyList(in, out, s)
}

func autoConvert_api_PolicyRule_To_v1_PolicyRule(in *authorizationapi.PolicyRule, out *authorizationapiv1.PolicyRule, s conversion.Scope) error {
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

func autoConvert_api_ResourceAccessReview_To_v1_ResourceAccessReview(in *authorizationapi.ResourceAccessReview, out *authorizationapiv1.ResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ResourceAccessReview))(in)
	}
	// in.Action has no peer in out
	return nil
}

func autoConvert_api_ResourceAccessReviewResponse_To_v1_ResourceAccessReviewResponse(in *authorizationapi.ResourceAccessReviewResponse, out *authorizationapiv1.ResourceAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.ResourceAccessReviewResponse))(in)
	}
	out.Namespace = in.Namespace
	// in.Users has no peer in out
	// in.Groups has no peer in out
	return nil
}

func autoConvert_api_Role_To_v1_Role(in *authorizationapi.Role, out *authorizationapiv1.Role, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.Role))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]authorizationapiv1.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := authorizationapiv1.Convert_api_PolicyRule_To_v1_PolicyRule(&in.Rules[i], &out.Rules[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_api_Role_To_v1_Role(in *authorizationapi.Role, out *authorizationapiv1.Role, s conversion.Scope) error {
	return autoConvert_api_Role_To_v1_Role(in, out, s)
}

func autoConvert_api_RoleBinding_To_v1_RoleBinding(in *authorizationapi.RoleBinding, out *authorizationapiv1.RoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.RoleBinding))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		out.Subjects = make([]apiv1.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_RoleBindingList_To_v1_RoleBindingList(in *authorizationapi.RoleBindingList, out *authorizationapiv1.RoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.RoleBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1.RoleBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_api_RoleBinding_To_v1_RoleBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_RoleBindingList_To_v1_RoleBindingList(in *authorizationapi.RoleBindingList, out *authorizationapiv1.RoleBindingList, s conversion.Scope) error {
	return autoConvert_api_RoleBindingList_To_v1_RoleBindingList(in, out, s)
}

func autoConvert_api_RoleList_To_v1_RoleList(in *authorizationapi.RoleList, out *authorizationapiv1.RoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.RoleList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapiv1.Role, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Role_To_v1_Role(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_RoleList_To_v1_RoleList(in *authorizationapi.RoleList, out *authorizationapiv1.RoleList, s conversion.Scope) error {
	return autoConvert_api_RoleList_To_v1_RoleList(in, out, s)
}

func autoConvert_api_SubjectAccessReview_To_v1_SubjectAccessReview(in *authorizationapi.SubjectAccessReview, out *authorizationapiv1.SubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.SubjectAccessReview))(in)
	}
	// in.Action has no peer in out
	out.User = in.User
	// in.Groups has no peer in out
	return nil
}

func autoConvert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse(in *authorizationapi.SubjectAccessReviewResponse, out *authorizationapiv1.SubjectAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapi.SubjectAccessReviewResponse))(in)
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func Convert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse(in *authorizationapi.SubjectAccessReviewResponse, out *authorizationapiv1.SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoConvert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse(in, out, s)
}

func autoConvert_v1_ClusterPolicy_To_api_ClusterPolicy(in *authorizationapiv1.ClusterPolicy, out *authorizationapi.ClusterPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ClusterPolicy))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := authorizationapiv1.Convert_v1_NamedClusterRoleArray_to_api_ClusterRoleArray(&in.Roles, &out.Roles, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_ClusterPolicyBinding_To_api_ClusterPolicyBinding(in *authorizationapiv1.ClusterPolicyBinding, out *authorizationapi.ClusterPolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ClusterPolicyBinding))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := authorizationapiv1.Convert_v1_NamedClusterRoleBindingArray_to_ClusterRoleBindingArray(&in.RoleBindings, &out.RoleBindings, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *authorizationapiv1.ClusterPolicyBindingList, out *authorizationapi.ClusterPolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ClusterPolicyBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.ClusterPolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_v1_ClusterPolicyBinding_To_api_ClusterPolicyBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *authorizationapiv1.ClusterPolicyBindingList, out *authorizationapi.ClusterPolicyBindingList, s conversion.Scope) error {
	return autoConvert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in, out, s)
}

func autoConvert_v1_ClusterPolicyList_To_api_ClusterPolicyList(in *authorizationapiv1.ClusterPolicyList, out *authorizationapi.ClusterPolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ClusterPolicyList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.ClusterPolicy, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_v1_ClusterPolicy_To_api_ClusterPolicy(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterPolicyList_To_api_ClusterPolicyList(in *authorizationapiv1.ClusterPolicyList, out *authorizationapi.ClusterPolicyList, s conversion.Scope) error {
	return autoConvert_v1_ClusterPolicyList_To_api_ClusterPolicyList(in, out, s)
}

func autoConvert_v1_ClusterRole_To_api_ClusterRole(in *authorizationapiv1.ClusterRole, out *authorizationapi.ClusterRole, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ClusterRole))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]authorizationapi.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := authorizationapiv1.Convert_v1_PolicyRule_To_api_PolicyRule(&in.Rules[i], &out.Rules[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_v1_ClusterRole_To_api_ClusterRole(in *authorizationapiv1.ClusterRole, out *authorizationapi.ClusterRole, s conversion.Scope) error {
	return autoConvert_v1_ClusterRole_To_api_ClusterRole(in, out, s)
}

func autoConvert_v1_ClusterRoleBinding_To_api_ClusterRoleBinding(in *authorizationapiv1.ClusterRoleBinding, out *authorizationapi.ClusterRoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ClusterRoleBinding))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.UserNames has no peer in out
	// in.GroupNames has no peer in out
	if in.Subjects != nil {
		out.Subjects = make([]api.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *authorizationapiv1.ClusterRoleBindingList, out *authorizationapi.ClusterRoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ClusterRoleBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.ClusterRoleBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_v1_ClusterRoleBinding_To_api_ClusterRoleBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *authorizationapiv1.ClusterRoleBindingList, out *authorizationapi.ClusterRoleBindingList, s conversion.Scope) error {
	return autoConvert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in, out, s)
}

func autoConvert_v1_ClusterRoleList_To_api_ClusterRoleList(in *authorizationapiv1.ClusterRoleList, out *authorizationapi.ClusterRoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ClusterRoleList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_ClusterRole_To_api_ClusterRole(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterRoleList_To_api_ClusterRoleList(in *authorizationapiv1.ClusterRoleList, out *authorizationapi.ClusterRoleList, s conversion.Scope) error {
	return autoConvert_v1_ClusterRoleList_To_api_ClusterRoleList(in, out, s)
}

func autoConvert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *authorizationapiv1.IsPersonalSubjectAccessReview, out *authorizationapi.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.IsPersonalSubjectAccessReview))(in)
	}
	return nil
}

func Convert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *authorizationapiv1.IsPersonalSubjectAccessReview, out *authorizationapi.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoConvert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in, out, s)
}

func autoConvert_v1_LocalResourceAccessReview_To_api_LocalResourceAccessReview(in *authorizationapiv1.LocalResourceAccessReview, out *authorizationapi.LocalResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.LocalResourceAccessReview))(in)
	}
	// in.AuthorizationAttributes has no peer in out
	return nil
}

func autoConvert_v1_LocalSubjectAccessReview_To_api_LocalSubjectAccessReview(in *authorizationapiv1.LocalSubjectAccessReview, out *authorizationapi.LocalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.LocalSubjectAccessReview))(in)
	}
	// in.AuthorizationAttributes has no peer in out
	out.User = in.User
	// in.GroupsSlice has no peer in out
	return nil
}

func autoConvert_v1_Policy_To_api_Policy(in *authorizationapiv1.Policy, out *authorizationapi.Policy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.Policy))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := authorizationapiv1.Convert_v1_NamedRoleArray_to_api_RoleArray(&in.Roles, &out.Roles, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_PolicyBinding_To_api_PolicyBinding(in *authorizationapiv1.PolicyBinding, out *authorizationapi.PolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.PolicyBinding))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastModified, &out.LastModified, s); err != nil {
		return err
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := authorizationapiv1.Convert_v1_NamedRoleBindingArray_to_api_RoleBindingArray(&in.RoleBindings, &out.RoleBindings, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_PolicyBindingList_To_api_PolicyBindingList(in *authorizationapiv1.PolicyBindingList, out *authorizationapi.PolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.PolicyBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.PolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_v1_PolicyBinding_To_api_PolicyBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_PolicyBindingList_To_api_PolicyBindingList(in *authorizationapiv1.PolicyBindingList, out *authorizationapi.PolicyBindingList, s conversion.Scope) error {
	return autoConvert_v1_PolicyBindingList_To_api_PolicyBindingList(in, out, s)
}

func autoConvert_v1_PolicyList_To_api_PolicyList(in *authorizationapiv1.PolicyList, out *authorizationapi.PolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.PolicyList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.Policy, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_v1_Policy_To_api_Policy(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_PolicyList_To_api_PolicyList(in *authorizationapiv1.PolicyList, out *authorizationapi.PolicyList, s conversion.Scope) error {
	return autoConvert_v1_PolicyList_To_api_PolicyList(in, out, s)
}

func autoConvert_v1_PolicyRule_To_api_PolicyRule(in *authorizationapiv1.PolicyRule, out *authorizationapi.PolicyRule, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.PolicyRule))(in)
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
	// in.Resources has no peer in out
	// in.ResourceNames has no peer in out
	// in.NonResourceURLsSlice has no peer in out
	return nil
}

func autoConvert_v1_ResourceAccessReview_To_api_ResourceAccessReview(in *authorizationapiv1.ResourceAccessReview, out *authorizationapi.ResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ResourceAccessReview))(in)
	}
	// in.AuthorizationAttributes has no peer in out
	return nil
}

func autoConvert_v1_ResourceAccessReviewResponse_To_api_ResourceAccessReviewResponse(in *authorizationapiv1.ResourceAccessReviewResponse, out *authorizationapi.ResourceAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.ResourceAccessReviewResponse))(in)
	}
	out.Namespace = in.Namespace
	// in.UsersSlice has no peer in out
	// in.GroupsSlice has no peer in out
	return nil
}

func autoConvert_v1_Role_To_api_Role(in *authorizationapiv1.Role, out *authorizationapi.Role, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.Role))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]authorizationapi.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := authorizationapiv1.Convert_v1_PolicyRule_To_api_PolicyRule(&in.Rules[i], &out.Rules[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func Convert_v1_Role_To_api_Role(in *authorizationapiv1.Role, out *authorizationapi.Role, s conversion.Scope) error {
	return autoConvert_v1_Role_To_api_Role(in, out, s)
}

func autoConvert_v1_RoleBinding_To_api_RoleBinding(in *authorizationapiv1.RoleBinding, out *authorizationapi.RoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.RoleBinding))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.UserNames has no peer in out
	// in.GroupNames has no peer in out
	if in.Subjects != nil {
		out.Subjects = make([]api.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_RoleBindingList_To_api_RoleBindingList(in *authorizationapiv1.RoleBindingList, out *authorizationapi.RoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.RoleBindingList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.RoleBinding, len(in.Items))
		for i := range in.Items {
			if err := authorizationapiv1.Convert_v1_RoleBinding_To_api_RoleBinding(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_RoleBindingList_To_api_RoleBindingList(in *authorizationapiv1.RoleBindingList, out *authorizationapi.RoleBindingList, s conversion.Scope) error {
	return autoConvert_v1_RoleBindingList_To_api_RoleBindingList(in, out, s)
}

func autoConvert_v1_RoleList_To_api_RoleList(in *authorizationapiv1.RoleList, out *authorizationapi.RoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.RoleList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]authorizationapi.Role, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_Role_To_api_Role(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_RoleList_To_api_RoleList(in *authorizationapiv1.RoleList, out *authorizationapi.RoleList, s conversion.Scope) error {
	return autoConvert_v1_RoleList_To_api_RoleList(in, out, s)
}

func autoConvert_v1_SubjectAccessReview_To_api_SubjectAccessReview(in *authorizationapiv1.SubjectAccessReview, out *authorizationapi.SubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.SubjectAccessReview))(in)
	}
	// in.AuthorizationAttributes has no peer in out
	out.User = in.User
	// in.GroupsSlice has no peer in out
	return nil
}

func autoConvert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *authorizationapiv1.SubjectAccessReviewResponse, out *authorizationapi.SubjectAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*authorizationapiv1.SubjectAccessReviewResponse))(in)
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func Convert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *authorizationapiv1.SubjectAccessReviewResponse, out *authorizationapi.SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoConvert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in, out, s)
}

func autoConvert_api_BinaryBuildRequestOptions_To_v1_BinaryBuildRequestOptions(in *buildapi.BinaryBuildRequestOptions, out *v1.BinaryBuildRequestOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BinaryBuildRequestOptions))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_api_BinaryBuildRequestOptions_To_v1_BinaryBuildRequestOptions(in *buildapi.BinaryBuildRequestOptions, out *v1.BinaryBuildRequestOptions, s conversion.Scope) error {
	return autoConvert_api_BinaryBuildRequestOptions_To_v1_BinaryBuildRequestOptions(in, out, s)
}

func autoConvert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in *buildapi.BinaryBuildSource, out *v1.BinaryBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BinaryBuildSource))(in)
	}
	out.AsFile = in.AsFile
	return nil
}

func Convert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in *buildapi.BinaryBuildSource, out *v1.BinaryBuildSource, s conversion.Scope) error {
	return autoConvert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in, out, s)
}

func autoConvert_api_Build_To_v1_Build(in *buildapi.Build, out *v1.Build, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.Build))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_BuildSpec_To_v1_BuildSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_BuildStatus_To_v1_BuildStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_Build_To_v1_Build(in *buildapi.Build, out *v1.Build, s conversion.Scope) error {
	return autoConvert_api_Build_To_v1_Build(in, out, s)
}

func autoConvert_api_BuildConfig_To_v1_BuildConfig(in *buildapi.BuildConfig, out *v1.BuildConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfig))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_BuildConfigSpec_To_v1_BuildConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_BuildConfigStatus_To_v1_BuildConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_BuildConfigList_To_v1_BuildConfigList(in *buildapi.BuildConfigList, out *v1.BuildConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := v1.Convert_api_BuildConfig_To_v1_BuildConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_BuildConfigList_To_v1_BuildConfigList(in *buildapi.BuildConfigList, out *v1.BuildConfigList, s conversion.Scope) error {
	return autoConvert_api_BuildConfigList_To_v1_BuildConfigList(in, out, s)
}

func autoConvert_api_BuildConfigSpec_To_v1_BuildConfigSpec(in *buildapi.BuildConfigSpec, out *v1.BuildConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigSpec))(in)
	}
	if in.Triggers != nil {
		out.Triggers = make([]v1.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := v1.Convert_api_BuildTriggerPolicy_To_v1_BuildTriggerPolicy(&in.Triggers[i], &out.Triggers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := Convert_api_BuildSpec_To_v1_BuildSpec(&in.BuildSpec, &out.BuildSpec, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_BuildConfigSpec_To_v1_BuildConfigSpec(in *buildapi.BuildConfigSpec, out *v1.BuildConfigSpec, s conversion.Scope) error {
	return autoConvert_api_BuildConfigSpec_To_v1_BuildConfigSpec(in, out, s)
}

func autoConvert_api_BuildConfigStatus_To_v1_BuildConfigStatus(in *buildapi.BuildConfigStatus, out *v1.BuildConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigStatus))(in)
	}
	out.LastVersion = in.LastVersion
	return nil
}

func Convert_api_BuildConfigStatus_To_v1_BuildConfigStatus(in *buildapi.BuildConfigStatus, out *v1.BuildConfigStatus, s conversion.Scope) error {
	return autoConvert_api_BuildConfigStatus_To_v1_BuildConfigStatus(in, out, s)
}

func autoConvert_api_BuildList_To_v1_BuildList(in *buildapi.BuildList, out *v1.BuildList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1.Build, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Build_To_v1_Build(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_BuildList_To_v1_BuildList(in *buildapi.BuildList, out *v1.BuildList, s conversion.Scope) error {
	return autoConvert_api_BuildList_To_v1_BuildList(in, out, s)
}

func autoConvert_api_BuildLog_To_v1_BuildLog(in *buildapi.BuildLog, out *v1.BuildLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildLog))(in)
	}
	return nil
}

func Convert_api_BuildLog_To_v1_BuildLog(in *buildapi.BuildLog, out *v1.BuildLog, s conversion.Scope) error {
	return autoConvert_api_BuildLog_To_v1_BuildLog(in, out, s)
}

func autoConvert_api_BuildLogOptions_To_v1_BuildLogOptions(in *buildapi.BuildLogOptions, out *v1.BuildLogOptions, s conversion.Scope) error {
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

func Convert_api_BuildLogOptions_To_v1_BuildLogOptions(in *buildapi.BuildLogOptions, out *v1.BuildLogOptions, s conversion.Scope) error {
	return autoConvert_api_BuildLogOptions_To_v1_BuildLogOptions(in, out, s)
}

func autoConvert_api_BuildOutput_To_v1_BuildOutput(in *buildapi.BuildOutput, out *v1.BuildOutput, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildOutput))(in)
	}
	// unable to generate simple pointer conversion for api.ObjectReference -> v1.ObjectReference
	if in.To != nil {
		out.To = new(apiv1.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1_ObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.PushSecret != nil {
		out.PushSecret = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PushSecret, out.PushSecret, s); err != nil {
			return err
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func autoConvert_api_BuildPostCommitSpec_To_v1_BuildPostCommitSpec(in *buildapi.BuildPostCommitSpec, out *v1.BuildPostCommitSpec, s conversion.Scope) error {
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

func Convert_api_BuildPostCommitSpec_To_v1_BuildPostCommitSpec(in *buildapi.BuildPostCommitSpec, out *v1.BuildPostCommitSpec, s conversion.Scope) error {
	return autoConvert_api_BuildPostCommitSpec_To_v1_BuildPostCommitSpec(in, out, s)
}

func autoConvert_api_BuildRequest_To_v1_BuildRequest(in *buildapi.BuildRequest, out *v1.BuildRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildRequest))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.SourceRevision -> v1.SourceRevision
	if in.Revision != nil {
		out.Revision = new(v1.SourceRevision)
		if err := v1.Convert_api_SourceRevision_To_v1_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	// unable to generate simple pointer conversion for api.ObjectReference -> v1.ObjectReference
	if in.TriggeredByImage != nil {
		out.TriggeredByImage = new(apiv1.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1_ObjectReference(in.TriggeredByImage, out.TriggeredByImage, s); err != nil {
			return err
		}
	} else {
		out.TriggeredByImage = nil
	}
	// unable to generate simple pointer conversion for api.ObjectReference -> v1.ObjectReference
	if in.From != nil {
		out.From = new(apiv1.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	// unable to generate simple pointer conversion for api.BinaryBuildSource -> v1.BinaryBuildSource
	if in.Binary != nil {
		out.Binary = new(v1.BinaryBuildSource)
		if err := Convert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
			return err
		}
	} else {
		out.Binary = nil
	}
	if in.LastVersion != nil {
		out.LastVersion = new(int)
		*out.LastVersion = *in.LastVersion
	} else {
		out.LastVersion = nil
	}
	if in.Env != nil {
		out.Env = make([]apiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	return nil
}

func Convert_api_BuildRequest_To_v1_BuildRequest(in *buildapi.BuildRequest, out *v1.BuildRequest, s conversion.Scope) error {
	return autoConvert_api_BuildRequest_To_v1_BuildRequest(in, out, s)
}

func autoConvert_api_BuildSource_To_v1_BuildSource(in *buildapi.BuildSource, out *v1.BuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildSource))(in)
	}
	// unable to generate simple pointer conversion for api.BinaryBuildSource -> v1.BinaryBuildSource
	if in.Binary != nil {
		out.Binary = new(v1.BinaryBuildSource)
		if err := Convert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
	// unable to generate simple pointer conversion for api.GitBuildSource -> v1.GitBuildSource
	if in.Git != nil {
		out.Git = new(v1.GitBuildSource)
		if err := Convert_api_GitBuildSource_To_v1_GitBuildSource(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	if in.Images != nil {
		out.Images = make([]v1.ImageSource, len(in.Images))
		for i := range in.Images {
			if err := Convert_api_ImageSource_To_v1_ImageSource(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	out.ContextDir = in.ContextDir
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.SourceSecret != nil {
		out.SourceSecret = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.SourceSecret, out.SourceSecret, s); err != nil {
			return err
		}
	} else {
		out.SourceSecret = nil
	}
	if in.Secrets != nil {
		out.Secrets = make([]v1.SecretBuildSource, len(in.Secrets))
		for i := range in.Secrets {
			if err := Convert_api_SecretBuildSource_To_v1_SecretBuildSource(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func autoConvert_api_BuildSpec_To_v1_BuildSpec(in *buildapi.BuildSpec, out *v1.BuildSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildSpec))(in)
	}
	out.ServiceAccount = in.ServiceAccount
	if err := v1.Convert_api_BuildSource_To_v1_BuildSource(&in.Source, &out.Source, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.SourceRevision -> v1.SourceRevision
	if in.Revision != nil {
		out.Revision = new(v1.SourceRevision)
		if err := v1.Convert_api_SourceRevision_To_v1_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := v1.Convert_api_BuildStrategy_To_v1_BuildStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if err := v1.Convert_api_BuildOutput_To_v1_BuildOutput(&in.Output, &out.Output, s); err != nil {
		return err
	}
	if err := Convert_api_ResourceRequirements_To_v1_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if err := Convert_api_BuildPostCommitSpec_To_v1_BuildPostCommitSpec(&in.PostCommit, &out.PostCommit, s); err != nil {
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

func Convert_api_BuildSpec_To_v1_BuildSpec(in *buildapi.BuildSpec, out *v1.BuildSpec, s conversion.Scope) error {
	return autoConvert_api_BuildSpec_To_v1_BuildSpec(in, out, s)
}

func autoConvert_api_BuildStatus_To_v1_BuildStatus(in *buildapi.BuildStatus, out *v1.BuildStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildStatus))(in)
	}
	out.Phase = v1.BuildPhase(in.Phase)
	out.Cancelled = in.Cancelled
	out.Reason = v1.StatusReason(in.Reason)
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
	// unable to generate simple pointer conversion for api.ObjectReference -> v1.ObjectReference
	if in.Config != nil {
		out.Config = new(apiv1.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1_ObjectReference(in.Config, out.Config, s); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	return nil
}

func Convert_api_BuildStatus_To_v1_BuildStatus(in *buildapi.BuildStatus, out *v1.BuildStatus, s conversion.Scope) error {
	return autoConvert_api_BuildStatus_To_v1_BuildStatus(in, out, s)
}

func autoConvert_api_BuildStrategy_To_v1_BuildStrategy(in *buildapi.BuildStrategy, out *v1.BuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildStrategy))(in)
	}
	// unable to generate simple pointer conversion for api.DockerBuildStrategy -> v1.DockerBuildStrategy
	if in.DockerStrategy != nil {
		out.DockerStrategy = new(v1.DockerBuildStrategy)
		if err := v1.Convert_api_DockerBuildStrategy_To_v1_DockerBuildStrategy(in.DockerStrategy, out.DockerStrategy, s); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	// unable to generate simple pointer conversion for api.SourceBuildStrategy -> v1.SourceBuildStrategy
	if in.SourceStrategy != nil {
		out.SourceStrategy = new(v1.SourceBuildStrategy)
		if err := v1.Convert_api_SourceBuildStrategy_To_v1_SourceBuildStrategy(in.SourceStrategy, out.SourceStrategy, s); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	// unable to generate simple pointer conversion for api.CustomBuildStrategy -> v1.CustomBuildStrategy
	if in.CustomStrategy != nil {
		out.CustomStrategy = new(v1.CustomBuildStrategy)
		if err := v1.Convert_api_CustomBuildStrategy_To_v1_CustomBuildStrategy(in.CustomStrategy, out.CustomStrategy, s); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	return nil
}

func autoConvert_api_BuildTriggerPolicy_To_v1_BuildTriggerPolicy(in *buildapi.BuildTriggerPolicy, out *v1.BuildTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildTriggerPolicy))(in)
	}
	out.Type = v1.BuildTriggerType(in.Type)
	// unable to generate simple pointer conversion for api.WebHookTrigger -> v1.WebHookTrigger
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(v1.WebHookTrigger)
		if err := Convert_api_WebHookTrigger_To_v1_WebHookTrigger(in.GitHubWebHook, out.GitHubWebHook, s); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	// unable to generate simple pointer conversion for api.WebHookTrigger -> v1.WebHookTrigger
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(v1.WebHookTrigger)
		if err := Convert_api_WebHookTrigger_To_v1_WebHookTrigger(in.GenericWebHook, out.GenericWebHook, s); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	// unable to generate simple pointer conversion for api.ImageChangeTrigger -> v1.ImageChangeTrigger
	if in.ImageChange != nil {
		out.ImageChange = new(v1.ImageChangeTrigger)
		if err := Convert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger(in.ImageChange, out.ImageChange, s); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func autoConvert_api_CustomBuildStrategy_To_v1_CustomBuildStrategy(in *buildapi.CustomBuildStrategy, out *v1.CustomBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.CustomBuildStrategy))(in)
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]apiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ExposeDockerSocket = in.ExposeDockerSocket
	out.ForcePull = in.ForcePull
	if in.Secrets != nil {
		out.Secrets = make([]v1.SecretSpec, len(in.Secrets))
		for i := range in.Secrets {
			if err := Convert_api_SecretSpec_To_v1_SecretSpec(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	out.BuildAPIVersion = in.BuildAPIVersion
	return nil
}

func autoConvert_api_DockerBuildStrategy_To_v1_DockerBuildStrategy(in *buildapi.DockerBuildStrategy, out *v1.DockerBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.DockerBuildStrategy))(in)
	}
	// unable to generate simple pointer conversion for api.ObjectReference -> v1.ObjectReference
	if in.From != nil {
		out.From = new(apiv1.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]apiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoConvert_api_GitBuildSource_To_v1_GitBuildSource(in *buildapi.GitBuildSource, out *v1.GitBuildSource, s conversion.Scope) error {
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

func Convert_api_GitBuildSource_To_v1_GitBuildSource(in *buildapi.GitBuildSource, out *v1.GitBuildSource, s conversion.Scope) error {
	return autoConvert_api_GitBuildSource_To_v1_GitBuildSource(in, out, s)
}

func autoConvert_api_GitSourceRevision_To_v1_GitSourceRevision(in *buildapi.GitSourceRevision, out *v1.GitSourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.GitSourceRevision))(in)
	}
	out.Commit = in.Commit
	if err := Convert_api_SourceControlUser_To_v1_SourceControlUser(&in.Author, &out.Author, s); err != nil {
		return err
	}
	if err := Convert_api_SourceControlUser_To_v1_SourceControlUser(&in.Committer, &out.Committer, s); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func Convert_api_GitSourceRevision_To_v1_GitSourceRevision(in *buildapi.GitSourceRevision, out *v1.GitSourceRevision, s conversion.Scope) error {
	return autoConvert_api_GitSourceRevision_To_v1_GitSourceRevision(in, out, s)
}

func autoConvert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger(in *buildapi.ImageChangeTrigger, out *v1.ImageChangeTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageChangeTrigger))(in)
	}
	out.LastTriggeredImageID = in.LastTriggeredImageID
	// unable to generate simple pointer conversion for api.ObjectReference -> v1.ObjectReference
	if in.From != nil {
		out.From = new(apiv1.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	return nil
}

func Convert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger(in *buildapi.ImageChangeTrigger, out *v1.ImageChangeTrigger, s conversion.Scope) error {
	return autoConvert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger(in, out, s)
}

func autoConvert_api_ImageSource_To_v1_ImageSource(in *buildapi.ImageSource, out *v1.ImageSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageSource))(in)
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.Paths != nil {
		out.Paths = make([]v1.ImageSourcePath, len(in.Paths))
		for i := range in.Paths {
			if err := Convert_api_ImageSourcePath_To_v1_ImageSourcePath(&in.Paths[i], &out.Paths[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Paths = nil
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	return nil
}

func Convert_api_ImageSource_To_v1_ImageSource(in *buildapi.ImageSource, out *v1.ImageSource, s conversion.Scope) error {
	return autoConvert_api_ImageSource_To_v1_ImageSource(in, out, s)
}

func autoConvert_api_ImageSourcePath_To_v1_ImageSourcePath(in *buildapi.ImageSourcePath, out *v1.ImageSourcePath, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageSourcePath))(in)
	}
	out.SourcePath = in.SourcePath
	out.DestinationDir = in.DestinationDir
	return nil
}

func Convert_api_ImageSourcePath_To_v1_ImageSourcePath(in *buildapi.ImageSourcePath, out *v1.ImageSourcePath, s conversion.Scope) error {
	return autoConvert_api_ImageSourcePath_To_v1_ImageSourcePath(in, out, s)
}

func autoConvert_api_SecretBuildSource_To_v1_SecretBuildSource(in *buildapi.SecretBuildSource, out *v1.SecretBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SecretBuildSource))(in)
	}
	if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(&in.Secret, &out.Secret, s); err != nil {
		return err
	}
	out.DestinationDir = in.DestinationDir
	return nil
}

func Convert_api_SecretBuildSource_To_v1_SecretBuildSource(in *buildapi.SecretBuildSource, out *v1.SecretBuildSource, s conversion.Scope) error {
	return autoConvert_api_SecretBuildSource_To_v1_SecretBuildSource(in, out, s)
}

func autoConvert_api_SecretSpec_To_v1_SecretSpec(in *buildapi.SecretSpec, out *v1.SecretSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SecretSpec))(in)
	}
	if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(&in.SecretSource, &out.SecretSource, s); err != nil {
		return err
	}
	out.MountPath = in.MountPath
	return nil
}

func Convert_api_SecretSpec_To_v1_SecretSpec(in *buildapi.SecretSpec, out *v1.SecretSpec, s conversion.Scope) error {
	return autoConvert_api_SecretSpec_To_v1_SecretSpec(in, out, s)
}

func autoConvert_api_SourceBuildStrategy_To_v1_SourceBuildStrategy(in *buildapi.SourceBuildStrategy, out *v1.SourceBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceBuildStrategy))(in)
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]apiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoConvert_api_SourceControlUser_To_v1_SourceControlUser(in *buildapi.SourceControlUser, out *v1.SourceControlUser, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceControlUser))(in)
	}
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func Convert_api_SourceControlUser_To_v1_SourceControlUser(in *buildapi.SourceControlUser, out *v1.SourceControlUser, s conversion.Scope) error {
	return autoConvert_api_SourceControlUser_To_v1_SourceControlUser(in, out, s)
}

func autoConvert_api_SourceRevision_To_v1_SourceRevision(in *buildapi.SourceRevision, out *v1.SourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceRevision))(in)
	}
	// unable to generate simple pointer conversion for api.GitSourceRevision -> v1.GitSourceRevision
	if in.Git != nil {
		out.Git = new(v1.GitSourceRevision)
		if err := Convert_api_GitSourceRevision_To_v1_GitSourceRevision(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func autoConvert_api_WebHookTrigger_To_v1_WebHookTrigger(in *buildapi.WebHookTrigger, out *v1.WebHookTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.WebHookTrigger))(in)
	}
	out.Secret = in.Secret
	return nil
}

func Convert_api_WebHookTrigger_To_v1_WebHookTrigger(in *buildapi.WebHookTrigger, out *v1.WebHookTrigger, s conversion.Scope) error {
	return autoConvert_api_WebHookTrigger_To_v1_WebHookTrigger(in, out, s)
}

func autoConvert_v1_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in *v1.BinaryBuildRequestOptions, out *buildapi.BinaryBuildRequestOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BinaryBuildRequestOptions))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_v1_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in *v1.BinaryBuildRequestOptions, out *buildapi.BinaryBuildRequestOptions, s conversion.Scope) error {
	return autoConvert_v1_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in, out, s)
}

func autoConvert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in *v1.BinaryBuildSource, out *buildapi.BinaryBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BinaryBuildSource))(in)
	}
	out.AsFile = in.AsFile
	return nil
}

func Convert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in *v1.BinaryBuildSource, out *buildapi.BinaryBuildSource, s conversion.Scope) error {
	return autoConvert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in, out, s)
}

func autoConvert_v1_Build_To_api_Build(in *v1.Build, out *buildapi.Build, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.Build))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_BuildSpec_To_api_BuildSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1_BuildStatus_To_api_BuildStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_Build_To_api_Build(in *v1.Build, out *buildapi.Build, s conversion.Scope) error {
	return autoConvert_v1_Build_To_api_Build(in, out, s)
}

func autoConvert_v1_BuildConfig_To_api_BuildConfig(in *v1.BuildConfig, out *buildapi.BuildConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildConfig))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_BuildConfigSpec_To_api_BuildConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1_BuildConfigStatus_To_api_BuildConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_BuildConfigList_To_api_BuildConfigList(in *v1.BuildConfigList, out *buildapi.BuildConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildConfigList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]buildapi.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := v1.Convert_v1_BuildConfig_To_api_BuildConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_BuildConfigList_To_api_BuildConfigList(in *v1.BuildConfigList, out *buildapi.BuildConfigList, s conversion.Scope) error {
	return autoConvert_v1_BuildConfigList_To_api_BuildConfigList(in, out, s)
}

func autoConvert_v1_BuildConfigSpec_To_api_BuildConfigSpec(in *v1.BuildConfigSpec, out *buildapi.BuildConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildConfigSpec))(in)
	}
	if in.Triggers != nil {
		out.Triggers = make([]buildapi.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := v1.Convert_v1_BuildTriggerPolicy_To_api_BuildTriggerPolicy(&in.Triggers[i], &out.Triggers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := Convert_v1_BuildSpec_To_api_BuildSpec(&in.BuildSpec, &out.BuildSpec, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_BuildConfigSpec_To_api_BuildConfigSpec(in *v1.BuildConfigSpec, out *buildapi.BuildConfigSpec, s conversion.Scope) error {
	return autoConvert_v1_BuildConfigSpec_To_api_BuildConfigSpec(in, out, s)
}

func autoConvert_v1_BuildConfigStatus_To_api_BuildConfigStatus(in *v1.BuildConfigStatus, out *buildapi.BuildConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildConfigStatus))(in)
	}
	out.LastVersion = in.LastVersion
	return nil
}

func Convert_v1_BuildConfigStatus_To_api_BuildConfigStatus(in *v1.BuildConfigStatus, out *buildapi.BuildConfigStatus, s conversion.Scope) error {
	return autoConvert_v1_BuildConfigStatus_To_api_BuildConfigStatus(in, out, s)
}

func autoConvert_v1_BuildList_To_api_BuildList(in *v1.BuildList, out *buildapi.BuildList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]buildapi.Build, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_Build_To_api_Build(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_BuildList_To_api_BuildList(in *v1.BuildList, out *buildapi.BuildList, s conversion.Scope) error {
	return autoConvert_v1_BuildList_To_api_BuildList(in, out, s)
}

func autoConvert_v1_BuildLog_To_api_BuildLog(in *v1.BuildLog, out *buildapi.BuildLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildLog))(in)
	}
	return nil
}

func Convert_v1_BuildLog_To_api_BuildLog(in *v1.BuildLog, out *buildapi.BuildLog, s conversion.Scope) error {
	return autoConvert_v1_BuildLog_To_api_BuildLog(in, out, s)
}

func autoConvert_v1_BuildLogOptions_To_api_BuildLogOptions(in *v1.BuildLogOptions, out *buildapi.BuildLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildLogOptions))(in)
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

func Convert_v1_BuildLogOptions_To_api_BuildLogOptions(in *v1.BuildLogOptions, out *buildapi.BuildLogOptions, s conversion.Scope) error {
	return autoConvert_v1_BuildLogOptions_To_api_BuildLogOptions(in, out, s)
}

func autoConvert_v1_BuildOutput_To_api_BuildOutput(in *v1.BuildOutput, out *buildapi.BuildOutput, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildOutput))(in)
	}
	// unable to generate simple pointer conversion for v1.ObjectReference -> api.ObjectReference
	if in.To != nil {
		out.To = new(api.ObjectReference)
		if err := Convert_v1_ObjectReference_To_api_ObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.PushSecret != nil {
		out.PushSecret = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PushSecret, out.PushSecret, s); err != nil {
			return err
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func autoConvert_v1_BuildPostCommitSpec_To_api_BuildPostCommitSpec(in *v1.BuildPostCommitSpec, out *buildapi.BuildPostCommitSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildPostCommitSpec))(in)
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

func Convert_v1_BuildPostCommitSpec_To_api_BuildPostCommitSpec(in *v1.BuildPostCommitSpec, out *buildapi.BuildPostCommitSpec, s conversion.Scope) error {
	return autoConvert_v1_BuildPostCommitSpec_To_api_BuildPostCommitSpec(in, out, s)
}

func autoConvert_v1_BuildRequest_To_api_BuildRequest(in *v1.BuildRequest, out *buildapi.BuildRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildRequest))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1.SourceRevision -> api.SourceRevision
	if in.Revision != nil {
		out.Revision = new(buildapi.SourceRevision)
		if err := v1.Convert_v1_SourceRevision_To_api_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	// unable to generate simple pointer conversion for v1.ObjectReference -> api.ObjectReference
	if in.TriggeredByImage != nil {
		out.TriggeredByImage = new(api.ObjectReference)
		if err := Convert_v1_ObjectReference_To_api_ObjectReference(in.TriggeredByImage, out.TriggeredByImage, s); err != nil {
			return err
		}
	} else {
		out.TriggeredByImage = nil
	}
	// unable to generate simple pointer conversion for v1.ObjectReference -> api.ObjectReference
	if in.From != nil {
		out.From = new(api.ObjectReference)
		if err := Convert_v1_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	// unable to generate simple pointer conversion for v1.BinaryBuildSource -> api.BinaryBuildSource
	if in.Binary != nil {
		out.Binary = new(buildapi.BinaryBuildSource)
		if err := Convert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
			return err
		}
	} else {
		out.Binary = nil
	}
	if in.LastVersion != nil {
		out.LastVersion = new(int)
		*out.LastVersion = *in.LastVersion
	} else {
		out.LastVersion = nil
	}
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	return nil
}

func Convert_v1_BuildRequest_To_api_BuildRequest(in *v1.BuildRequest, out *buildapi.BuildRequest, s conversion.Scope) error {
	return autoConvert_v1_BuildRequest_To_api_BuildRequest(in, out, s)
}

func autoConvert_v1_BuildSource_To_api_BuildSource(in *v1.BuildSource, out *buildapi.BuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildSource))(in)
	}
	// in.Type has no peer in out
	// unable to generate simple pointer conversion for v1.BinaryBuildSource -> api.BinaryBuildSource
	if in.Binary != nil {
		out.Binary = new(buildapi.BinaryBuildSource)
		if err := Convert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
	// unable to generate simple pointer conversion for v1.GitBuildSource -> api.GitBuildSource
	if in.Git != nil {
		out.Git = new(buildapi.GitBuildSource)
		if err := Convert_v1_GitBuildSource_To_api_GitBuildSource(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	if in.Images != nil {
		out.Images = make([]buildapi.ImageSource, len(in.Images))
		for i := range in.Images {
			if err := Convert_v1_ImageSource_To_api_ImageSource(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	out.ContextDir = in.ContextDir
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.SourceSecret != nil {
		out.SourceSecret = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.SourceSecret, out.SourceSecret, s); err != nil {
			return err
		}
	} else {
		out.SourceSecret = nil
	}
	if in.Secrets != nil {
		out.Secrets = make([]buildapi.SecretBuildSource, len(in.Secrets))
		for i := range in.Secrets {
			if err := Convert_v1_SecretBuildSource_To_api_SecretBuildSource(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func autoConvert_v1_BuildSpec_To_api_BuildSpec(in *v1.BuildSpec, out *buildapi.BuildSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildSpec))(in)
	}
	out.ServiceAccount = in.ServiceAccount
	if err := v1.Convert_v1_BuildSource_To_api_BuildSource(&in.Source, &out.Source, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1.SourceRevision -> api.SourceRevision
	if in.Revision != nil {
		out.Revision = new(buildapi.SourceRevision)
		if err := v1.Convert_v1_SourceRevision_To_api_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := v1.Convert_v1_BuildStrategy_To_api_BuildStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if err := v1.Convert_v1_BuildOutput_To_api_BuildOutput(&in.Output, &out.Output, s); err != nil {
		return err
	}
	if err := Convert_v1_ResourceRequirements_To_api_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if err := Convert_v1_BuildPostCommitSpec_To_api_BuildPostCommitSpec(&in.PostCommit, &out.PostCommit, s); err != nil {
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

func Convert_v1_BuildSpec_To_api_BuildSpec(in *v1.BuildSpec, out *buildapi.BuildSpec, s conversion.Scope) error {
	return autoConvert_v1_BuildSpec_To_api_BuildSpec(in, out, s)
}

func autoConvert_v1_BuildStatus_To_api_BuildStatus(in *v1.BuildStatus, out *buildapi.BuildStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildStatus))(in)
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
	// unable to generate simple pointer conversion for v1.ObjectReference -> api.ObjectReference
	if in.Config != nil {
		out.Config = new(api.ObjectReference)
		if err := Convert_v1_ObjectReference_To_api_ObjectReference(in.Config, out.Config, s); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	return nil
}

func Convert_v1_BuildStatus_To_api_BuildStatus(in *v1.BuildStatus, out *buildapi.BuildStatus, s conversion.Scope) error {
	return autoConvert_v1_BuildStatus_To_api_BuildStatus(in, out, s)
}

func autoConvert_v1_BuildStrategy_To_api_BuildStrategy(in *v1.BuildStrategy, out *buildapi.BuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildStrategy))(in)
	}
	// in.Type has no peer in out
	// unable to generate simple pointer conversion for v1.DockerBuildStrategy -> api.DockerBuildStrategy
	if in.DockerStrategy != nil {
		out.DockerStrategy = new(buildapi.DockerBuildStrategy)
		if err := v1.Convert_v1_DockerBuildStrategy_To_api_DockerBuildStrategy(in.DockerStrategy, out.DockerStrategy, s); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	// unable to generate simple pointer conversion for v1.SourceBuildStrategy -> api.SourceBuildStrategy
	if in.SourceStrategy != nil {
		out.SourceStrategy = new(buildapi.SourceBuildStrategy)
		if err := v1.Convert_v1_SourceBuildStrategy_To_api_SourceBuildStrategy(in.SourceStrategy, out.SourceStrategy, s); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	// unable to generate simple pointer conversion for v1.CustomBuildStrategy -> api.CustomBuildStrategy
	if in.CustomStrategy != nil {
		out.CustomStrategy = new(buildapi.CustomBuildStrategy)
		if err := v1.Convert_v1_CustomBuildStrategy_To_api_CustomBuildStrategy(in.CustomStrategy, out.CustomStrategy, s); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	return nil
}

func autoConvert_v1_BuildTriggerPolicy_To_api_BuildTriggerPolicy(in *v1.BuildTriggerPolicy, out *buildapi.BuildTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.BuildTriggerPolicy))(in)
	}
	out.Type = buildapi.BuildTriggerType(in.Type)
	// unable to generate simple pointer conversion for v1.WebHookTrigger -> api.WebHookTrigger
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(buildapi.WebHookTrigger)
		if err := Convert_v1_WebHookTrigger_To_api_WebHookTrigger(in.GitHubWebHook, out.GitHubWebHook, s); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	// unable to generate simple pointer conversion for v1.WebHookTrigger -> api.WebHookTrigger
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(buildapi.WebHookTrigger)
		if err := Convert_v1_WebHookTrigger_To_api_WebHookTrigger(in.GenericWebHook, out.GenericWebHook, s); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	// unable to generate simple pointer conversion for v1.ImageChangeTrigger -> api.ImageChangeTrigger
	if in.ImageChange != nil {
		out.ImageChange = new(buildapi.ImageChangeTrigger)
		if err := Convert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger(in.ImageChange, out.ImageChange, s); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func autoConvert_v1_CustomBuildStrategy_To_api_CustomBuildStrategy(in *v1.CustomBuildStrategy, out *buildapi.CustomBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.CustomBuildStrategy))(in)
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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
			if err := Convert_v1_SecretSpec_To_api_SecretSpec(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	out.BuildAPIVersion = in.BuildAPIVersion
	return nil
}

func autoConvert_v1_DockerBuildStrategy_To_api_DockerBuildStrategy(in *v1.DockerBuildStrategy, out *buildapi.DockerBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.DockerBuildStrategy))(in)
	}
	// unable to generate simple pointer conversion for v1.ObjectReference -> api.ObjectReference
	if in.From != nil {
		out.From = new(api.ObjectReference)
		if err := Convert_v1_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoConvert_v1_GitBuildSource_To_api_GitBuildSource(in *v1.GitBuildSource, out *buildapi.GitBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.GitBuildSource))(in)
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

func Convert_v1_GitBuildSource_To_api_GitBuildSource(in *v1.GitBuildSource, out *buildapi.GitBuildSource, s conversion.Scope) error {
	return autoConvert_v1_GitBuildSource_To_api_GitBuildSource(in, out, s)
}

func autoConvert_v1_GitSourceRevision_To_api_GitSourceRevision(in *v1.GitSourceRevision, out *buildapi.GitSourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.GitSourceRevision))(in)
	}
	out.Commit = in.Commit
	if err := Convert_v1_SourceControlUser_To_api_SourceControlUser(&in.Author, &out.Author, s); err != nil {
		return err
	}
	if err := Convert_v1_SourceControlUser_To_api_SourceControlUser(&in.Committer, &out.Committer, s); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func Convert_v1_GitSourceRevision_To_api_GitSourceRevision(in *v1.GitSourceRevision, out *buildapi.GitSourceRevision, s conversion.Scope) error {
	return autoConvert_v1_GitSourceRevision_To_api_GitSourceRevision(in, out, s)
}

func autoConvert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger(in *v1.ImageChangeTrigger, out *buildapi.ImageChangeTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ImageChangeTrigger))(in)
	}
	out.LastTriggeredImageID = in.LastTriggeredImageID
	// unable to generate simple pointer conversion for v1.ObjectReference -> api.ObjectReference
	if in.From != nil {
		out.From = new(api.ObjectReference)
		if err := Convert_v1_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	return nil
}

func Convert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger(in *v1.ImageChangeTrigger, out *buildapi.ImageChangeTrigger, s conversion.Scope) error {
	return autoConvert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger(in, out, s)
}

func autoConvert_v1_ImageSource_To_api_ImageSource(in *v1.ImageSource, out *buildapi.ImageSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ImageSource))(in)
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.Paths != nil {
		out.Paths = make([]buildapi.ImageSourcePath, len(in.Paths))
		for i := range in.Paths {
			if err := Convert_v1_ImageSourcePath_To_api_ImageSourcePath(&in.Paths[i], &out.Paths[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Paths = nil
	}
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	return nil
}

func Convert_v1_ImageSource_To_api_ImageSource(in *v1.ImageSource, out *buildapi.ImageSource, s conversion.Scope) error {
	return autoConvert_v1_ImageSource_To_api_ImageSource(in, out, s)
}

func autoConvert_v1_ImageSourcePath_To_api_ImageSourcePath(in *v1.ImageSourcePath, out *buildapi.ImageSourcePath, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ImageSourcePath))(in)
	}
	out.SourcePath = in.SourcePath
	out.DestinationDir = in.DestinationDir
	return nil
}

func Convert_v1_ImageSourcePath_To_api_ImageSourcePath(in *v1.ImageSourcePath, out *buildapi.ImageSourcePath, s conversion.Scope) error {
	return autoConvert_v1_ImageSourcePath_To_api_ImageSourcePath(in, out, s)
}

func autoConvert_v1_SecretBuildSource_To_api_SecretBuildSource(in *v1.SecretBuildSource, out *buildapi.SecretBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.SecretBuildSource))(in)
	}
	if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(&in.Secret, &out.Secret, s); err != nil {
		return err
	}
	out.DestinationDir = in.DestinationDir
	return nil
}

func Convert_v1_SecretBuildSource_To_api_SecretBuildSource(in *v1.SecretBuildSource, out *buildapi.SecretBuildSource, s conversion.Scope) error {
	return autoConvert_v1_SecretBuildSource_To_api_SecretBuildSource(in, out, s)
}

func autoConvert_v1_SecretSpec_To_api_SecretSpec(in *v1.SecretSpec, out *buildapi.SecretSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.SecretSpec))(in)
	}
	if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(&in.SecretSource, &out.SecretSource, s); err != nil {
		return err
	}
	out.MountPath = in.MountPath
	return nil
}

func Convert_v1_SecretSpec_To_api_SecretSpec(in *v1.SecretSpec, out *buildapi.SecretSpec, s conversion.Scope) error {
	return autoConvert_v1_SecretSpec_To_api_SecretSpec(in, out, s)
}

func autoConvert_v1_SourceBuildStrategy_To_api_SourceBuildStrategy(in *v1.SourceBuildStrategy, out *buildapi.SourceBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.SourceBuildStrategy))(in)
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.PullSecret != nil {
		out.PullSecret = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoConvert_v1_SourceControlUser_To_api_SourceControlUser(in *v1.SourceControlUser, out *buildapi.SourceControlUser, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.SourceControlUser))(in)
	}
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func Convert_v1_SourceControlUser_To_api_SourceControlUser(in *v1.SourceControlUser, out *buildapi.SourceControlUser, s conversion.Scope) error {
	return autoConvert_v1_SourceControlUser_To_api_SourceControlUser(in, out, s)
}

func autoConvert_v1_SourceRevision_To_api_SourceRevision(in *v1.SourceRevision, out *buildapi.SourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.SourceRevision))(in)
	}
	// in.Type has no peer in out
	// unable to generate simple pointer conversion for v1.GitSourceRevision -> api.GitSourceRevision
	if in.Git != nil {
		out.Git = new(buildapi.GitSourceRevision)
		if err := Convert_v1_GitSourceRevision_To_api_GitSourceRevision(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func autoConvert_v1_WebHookTrigger_To_api_WebHookTrigger(in *v1.WebHookTrigger, out *buildapi.WebHookTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.WebHookTrigger))(in)
	}
	out.Secret = in.Secret
	return nil
}

func Convert_v1_WebHookTrigger_To_api_WebHookTrigger(in *v1.WebHookTrigger, out *buildapi.WebHookTrigger, s conversion.Scope) error {
	return autoConvert_v1_WebHookTrigger_To_api_WebHookTrigger(in, out, s)
}

func autoConvert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams(in *deployapi.CustomDeploymentStrategyParams, out *deployapiv1.CustomDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.CustomDeploymentStrategyParams))(in)
	}
	out.Image = in.Image
	if in.Environment != nil {
		out.Environment = make([]apiv1.EnvVar, len(in.Environment))
		for i := range in.Environment {
			if err := Convert_api_EnvVar_To_v1_EnvVar(&in.Environment[i], &out.Environment[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Environment = nil
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

func Convert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams(in *deployapi.CustomDeploymentStrategyParams, out *deployapiv1.CustomDeploymentStrategyParams, s conversion.Scope) error {
	return autoConvert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams(in, out, s)
}

func autoConvert_api_DeploymentCause_To_v1_DeploymentCause(in *deployapi.DeploymentCause, out *deployapiv1.DeploymentCause, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentCause))(in)
	}
	out.Type = deployapiv1.DeploymentTriggerType(in.Type)
	// unable to generate simple pointer conversion for api.DeploymentCauseImageTrigger -> v1.DeploymentCauseImageTrigger
	if in.ImageTrigger != nil {
		out.ImageTrigger = new(deployapiv1.DeploymentCauseImageTrigger)
		if err := Convert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger(in.ImageTrigger, out.ImageTrigger, s); err != nil {
			return err
		}
	} else {
		out.ImageTrigger = nil
	}
	return nil
}

func Convert_api_DeploymentCause_To_v1_DeploymentCause(in *deployapi.DeploymentCause, out *deployapiv1.DeploymentCause, s conversion.Scope) error {
	return autoConvert_api_DeploymentCause_To_v1_DeploymentCause(in, out, s)
}

func autoConvert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger(in *deployapi.DeploymentCauseImageTrigger, out *deployapiv1.DeploymentCauseImageTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentCauseImageTrigger))(in)
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger(in *deployapi.DeploymentCauseImageTrigger, out *deployapiv1.DeploymentCauseImageTrigger, s conversion.Scope) error {
	return autoConvert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger(in, out, s)
}

func autoConvert_api_DeploymentConfig_To_v1_DeploymentConfig(in *deployapi.DeploymentConfig, out *deployapiv1.DeploymentConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfig))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_DeploymentConfig_To_v1_DeploymentConfig(in *deployapi.DeploymentConfig, out *deployapiv1.DeploymentConfig, s conversion.Scope) error {
	return autoConvert_api_DeploymentConfig_To_v1_DeploymentConfig(in, out, s)
}

func autoConvert_api_DeploymentConfigList_To_v1_DeploymentConfigList(in *deployapi.DeploymentConfigList, out *deployapiv1.DeploymentConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]deployapiv1.DeploymentConfig, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_DeploymentConfig_To_v1_DeploymentConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_DeploymentConfigList_To_v1_DeploymentConfigList(in *deployapi.DeploymentConfigList, out *deployapiv1.DeploymentConfigList, s conversion.Scope) error {
	return autoConvert_api_DeploymentConfigList_To_v1_DeploymentConfigList(in, out, s)
}

func autoConvert_api_DeploymentConfigRollback_To_v1_DeploymentConfigRollback(in *deployapi.DeploymentConfigRollback, out *deployapiv1.DeploymentConfigRollback, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigRollback))(in)
	}
	if err := Convert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_DeploymentConfigRollback_To_v1_DeploymentConfigRollback(in *deployapi.DeploymentConfigRollback, out *deployapiv1.DeploymentConfigRollback, s conversion.Scope) error {
	return autoConvert_api_DeploymentConfigRollback_To_v1_DeploymentConfigRollback(in, out, s)
}

func autoConvert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec(in *deployapi.DeploymentConfigRollbackSpec, out *deployapiv1.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigRollbackSpec))(in)
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func Convert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec(in *deployapi.DeploymentConfigRollbackSpec, out *deployapiv1.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	return autoConvert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec(in, out, s)
}

func autoConvert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec(in *deployapi.DeploymentConfigSpec, out *deployapiv1.DeploymentConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigSpec))(in)
	}
	if err := Convert_api_DeploymentStrategy_To_v1_DeploymentStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if in.Triggers != nil {
		out.Triggers = make([]deployapiv1.DeploymentTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := Convert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy(&in.Triggers[i], &out.Triggers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	out.Replicas = in.Replicas
	out.Test = in.Test
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	// unable to generate simple pointer conversion for api.PodTemplateSpec -> v1.PodTemplateSpec
	if in.Template != nil {
		out.Template = new(apiv1.PodTemplateSpec)
		if err := Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

func Convert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec(in *deployapi.DeploymentConfigSpec, out *deployapiv1.DeploymentConfigSpec, s conversion.Scope) error {
	return autoConvert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec(in, out, s)
}

func autoConvert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus(in *deployapi.DeploymentConfigStatus, out *deployapiv1.DeploymentConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigStatus))(in)
	}
	out.LatestVersion = in.LatestVersion
	// unable to generate simple pointer conversion for api.DeploymentDetails -> v1.DeploymentDetails
	if in.Details != nil {
		out.Details = new(deployapiv1.DeploymentDetails)
		if err := Convert_api_DeploymentDetails_To_v1_DeploymentDetails(in.Details, out.Details, s); err != nil {
			return err
		}
	} else {
		out.Details = nil
	}
	return nil
}

func Convert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus(in *deployapi.DeploymentConfigStatus, out *deployapiv1.DeploymentConfigStatus, s conversion.Scope) error {
	return autoConvert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus(in, out, s)
}

func autoConvert_api_DeploymentDetails_To_v1_DeploymentDetails(in *deployapi.DeploymentDetails, out *deployapiv1.DeploymentDetails, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentDetails))(in)
	}
	out.Message = in.Message
	if in.Causes != nil {
		out.Causes = make([]deployapiv1.DeploymentCause, len(in.Causes))
		for i := range in.Causes {
			if err := Convert_api_DeploymentCause_To_v1_DeploymentCause(&in.Causes[i], &out.Causes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Causes = nil
	}
	return nil
}

func Convert_api_DeploymentDetails_To_v1_DeploymentDetails(in *deployapi.DeploymentDetails, out *deployapiv1.DeploymentDetails, s conversion.Scope) error {
	return autoConvert_api_DeploymentDetails_To_v1_DeploymentDetails(in, out, s)
}

func autoConvert_api_DeploymentLog_To_v1_DeploymentLog(in *deployapi.DeploymentLog, out *deployapiv1.DeploymentLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentLog))(in)
	}
	return nil
}

func Convert_api_DeploymentLog_To_v1_DeploymentLog(in *deployapi.DeploymentLog, out *deployapiv1.DeploymentLog, s conversion.Scope) error {
	return autoConvert_api_DeploymentLog_To_v1_DeploymentLog(in, out, s)
}

func autoConvert_api_DeploymentLogOptions_To_v1_DeploymentLogOptions(in *deployapi.DeploymentLogOptions, out *deployapiv1.DeploymentLogOptions, s conversion.Scope) error {
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

func Convert_api_DeploymentLogOptions_To_v1_DeploymentLogOptions(in *deployapi.DeploymentLogOptions, out *deployapiv1.DeploymentLogOptions, s conversion.Scope) error {
	return autoConvert_api_DeploymentLogOptions_To_v1_DeploymentLogOptions(in, out, s)
}

func autoConvert_api_DeploymentStrategy_To_v1_DeploymentStrategy(in *deployapi.DeploymentStrategy, out *deployapiv1.DeploymentStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentStrategy))(in)
	}
	out.Type = deployapiv1.DeploymentStrategyType(in.Type)
	// unable to generate simple pointer conversion for api.CustomDeploymentStrategyParams -> v1.CustomDeploymentStrategyParams
	if in.CustomParams != nil {
		out.CustomParams = new(deployapiv1.CustomDeploymentStrategyParams)
		if err := Convert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams(in.CustomParams, out.CustomParams, s); err != nil {
			return err
		}
	} else {
		out.CustomParams = nil
	}
	// unable to generate simple pointer conversion for api.RecreateDeploymentStrategyParams -> v1.RecreateDeploymentStrategyParams
	if in.RecreateParams != nil {
		out.RecreateParams = new(deployapiv1.RecreateDeploymentStrategyParams)
		if err := Convert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams(in.RecreateParams, out.RecreateParams, s); err != nil {
			return err
		}
	} else {
		out.RecreateParams = nil
	}
	// unable to generate simple pointer conversion for api.RollingDeploymentStrategyParams -> v1.RollingDeploymentStrategyParams
	if in.RollingParams != nil {
		out.RollingParams = new(deployapiv1.RollingDeploymentStrategyParams)
		if err := deployapiv1.Convert_api_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams(in.RollingParams, out.RollingParams, s); err != nil {
			return err
		}
	} else {
		out.RollingParams = nil
	}
	if err := Convert_api_ResourceRequirements_To_v1_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
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

func Convert_api_DeploymentStrategy_To_v1_DeploymentStrategy(in *deployapi.DeploymentStrategy, out *deployapiv1.DeploymentStrategy, s conversion.Scope) error {
	return autoConvert_api_DeploymentStrategy_To_v1_DeploymentStrategy(in, out, s)
}

func autoConvert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams(in *deployapi.DeploymentTriggerImageChangeParams, out *deployapiv1.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
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
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.LastTriggeredImage = in.LastTriggeredImage
	return nil
}

func autoConvert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy(in *deployapi.DeploymentTriggerPolicy, out *deployapiv1.DeploymentTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentTriggerPolicy))(in)
	}
	out.Type = deployapiv1.DeploymentTriggerType(in.Type)
	// unable to generate simple pointer conversion for api.DeploymentTriggerImageChangeParams -> v1.DeploymentTriggerImageChangeParams
	if in.ImageChangeParams != nil {
		out.ImageChangeParams = new(deployapiv1.DeploymentTriggerImageChangeParams)
		if err := deployapiv1.Convert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams(in.ImageChangeParams, out.ImageChangeParams, s); err != nil {
			return err
		}
	} else {
		out.ImageChangeParams = nil
	}
	return nil
}

func Convert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy(in *deployapi.DeploymentTriggerPolicy, out *deployapiv1.DeploymentTriggerPolicy, s conversion.Scope) error {
	return autoConvert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy(in, out, s)
}

func autoConvert_api_ExecNewPodHook_To_v1_ExecNewPodHook(in *deployapi.ExecNewPodHook, out *deployapiv1.ExecNewPodHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.ExecNewPodHook))(in)
	}
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	if in.Env != nil {
		out.Env = make([]apiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ContainerName = in.ContainerName
	if in.Volumes != nil {
		out.Volumes = make([]string, len(in.Volumes))
		for i := range in.Volumes {
			out.Volumes[i] = in.Volumes[i]
		}
	} else {
		out.Volumes = nil
	}
	return nil
}

func Convert_api_ExecNewPodHook_To_v1_ExecNewPodHook(in *deployapi.ExecNewPodHook, out *deployapiv1.ExecNewPodHook, s conversion.Scope) error {
	return autoConvert_api_ExecNewPodHook_To_v1_ExecNewPodHook(in, out, s)
}

func autoConvert_api_LifecycleHook_To_v1_LifecycleHook(in *deployapi.LifecycleHook, out *deployapiv1.LifecycleHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.LifecycleHook))(in)
	}
	out.FailurePolicy = deployapiv1.LifecycleHookFailurePolicy(in.FailurePolicy)
	// unable to generate simple pointer conversion for api.ExecNewPodHook -> v1.ExecNewPodHook
	if in.ExecNewPod != nil {
		out.ExecNewPod = new(deployapiv1.ExecNewPodHook)
		if err := Convert_api_ExecNewPodHook_To_v1_ExecNewPodHook(in.ExecNewPod, out.ExecNewPod, s); err != nil {
			return err
		}
	} else {
		out.ExecNewPod = nil
	}
	if in.TagImages != nil {
		out.TagImages = make([]deployapiv1.TagImageHook, len(in.TagImages))
		for i := range in.TagImages {
			if err := Convert_api_TagImageHook_To_v1_TagImageHook(&in.TagImages[i], &out.TagImages[i], s); err != nil {
				return err
			}
		}
	} else {
		out.TagImages = nil
	}
	return nil
}

func Convert_api_LifecycleHook_To_v1_LifecycleHook(in *deployapi.LifecycleHook, out *deployapiv1.LifecycleHook, s conversion.Scope) error {
	return autoConvert_api_LifecycleHook_To_v1_LifecycleHook(in, out, s)
}

func autoConvert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams(in *deployapi.RecreateDeploymentStrategyParams, out *deployapiv1.RecreateDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.RecreateDeploymentStrategyParams))(in)
	}
	if in.TimeoutSeconds != nil {
		out.TimeoutSeconds = new(int64)
		*out.TimeoutSeconds = *in.TimeoutSeconds
	} else {
		out.TimeoutSeconds = nil
	}
	// unable to generate simple pointer conversion for api.LifecycleHook -> v1.LifecycleHook
	if in.Pre != nil {
		out.Pre = new(deployapiv1.LifecycleHook)
		if err := Convert_api_LifecycleHook_To_v1_LifecycleHook(in.Pre, out.Pre, s); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	// unable to generate simple pointer conversion for api.LifecycleHook -> v1.LifecycleHook
	if in.Mid != nil {
		out.Mid = new(deployapiv1.LifecycleHook)
		if err := Convert_api_LifecycleHook_To_v1_LifecycleHook(in.Mid, out.Mid, s); err != nil {
			return err
		}
	} else {
		out.Mid = nil
	}
	// unable to generate simple pointer conversion for api.LifecycleHook -> v1.LifecycleHook
	if in.Post != nil {
		out.Post = new(deployapiv1.LifecycleHook)
		if err := Convert_api_LifecycleHook_To_v1_LifecycleHook(in.Post, out.Post, s); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func Convert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams(in *deployapi.RecreateDeploymentStrategyParams, out *deployapiv1.RecreateDeploymentStrategyParams, s conversion.Scope) error {
	return autoConvert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams(in, out, s)
}

func autoConvert_api_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams(in *deployapi.RollingDeploymentStrategyParams, out *deployapiv1.RollingDeploymentStrategyParams, s conversion.Scope) error {
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
	// unable to generate simple pointer conversion for api.LifecycleHook -> v1.LifecycleHook
	if in.Pre != nil {
		out.Pre = new(deployapiv1.LifecycleHook)
		if err := Convert_api_LifecycleHook_To_v1_LifecycleHook(in.Pre, out.Pre, s); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	// unable to generate simple pointer conversion for api.LifecycleHook -> v1.LifecycleHook
	if in.Post != nil {
		out.Post = new(deployapiv1.LifecycleHook)
		if err := Convert_api_LifecycleHook_To_v1_LifecycleHook(in.Post, out.Post, s); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func autoConvert_api_TagImageHook_To_v1_TagImageHook(in *deployapi.TagImageHook, out *deployapiv1.TagImageHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.TagImageHook))(in)
	}
	out.ContainerName = in.ContainerName
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_TagImageHook_To_v1_TagImageHook(in *deployapi.TagImageHook, out *deployapiv1.TagImageHook, s conversion.Scope) error {
	return autoConvert_api_TagImageHook_To_v1_TagImageHook(in, out, s)
}

func autoConvert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams(in *deployapiv1.CustomDeploymentStrategyParams, out *deployapi.CustomDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.CustomDeploymentStrategyParams))(in)
	}
	out.Image = in.Image
	if in.Environment != nil {
		out.Environment = make([]api.EnvVar, len(in.Environment))
		for i := range in.Environment {
			if err := Convert_v1_EnvVar_To_api_EnvVar(&in.Environment[i], &out.Environment[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Environment = nil
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

func Convert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams(in *deployapiv1.CustomDeploymentStrategyParams, out *deployapi.CustomDeploymentStrategyParams, s conversion.Scope) error {
	return autoConvert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams(in, out, s)
}

func autoConvert_v1_DeploymentCause_To_api_DeploymentCause(in *deployapiv1.DeploymentCause, out *deployapi.DeploymentCause, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentCause))(in)
	}
	out.Type = deployapi.DeploymentTriggerType(in.Type)
	// unable to generate simple pointer conversion for v1.DeploymentCauseImageTrigger -> api.DeploymentCauseImageTrigger
	if in.ImageTrigger != nil {
		out.ImageTrigger = new(deployapi.DeploymentCauseImageTrigger)
		if err := Convert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in.ImageTrigger, out.ImageTrigger, s); err != nil {
			return err
		}
	} else {
		out.ImageTrigger = nil
	}
	return nil
}

func Convert_v1_DeploymentCause_To_api_DeploymentCause(in *deployapiv1.DeploymentCause, out *deployapi.DeploymentCause, s conversion.Scope) error {
	return autoConvert_v1_DeploymentCause_To_api_DeploymentCause(in, out, s)
}

func autoConvert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in *deployapiv1.DeploymentCauseImageTrigger, out *deployapi.DeploymentCauseImageTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentCauseImageTrigger))(in)
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in *deployapiv1.DeploymentCauseImageTrigger, out *deployapi.DeploymentCauseImageTrigger, s conversion.Scope) error {
	return autoConvert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in, out, s)
}

func autoConvert_v1_DeploymentConfig_To_api_DeploymentConfig(in *deployapiv1.DeploymentConfig, out *deployapi.DeploymentConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfig))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_DeploymentConfig_To_api_DeploymentConfig(in *deployapiv1.DeploymentConfig, out *deployapi.DeploymentConfig, s conversion.Scope) error {
	return autoConvert_v1_DeploymentConfig_To_api_DeploymentConfig(in, out, s)
}

func autoConvert_v1_DeploymentConfigList_To_api_DeploymentConfigList(in *deployapiv1.DeploymentConfigList, out *deployapi.DeploymentConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]deployapi.DeploymentConfig, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_DeploymentConfig_To_api_DeploymentConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_DeploymentConfigList_To_api_DeploymentConfigList(in *deployapiv1.DeploymentConfigList, out *deployapi.DeploymentConfigList, s conversion.Scope) error {
	return autoConvert_v1_DeploymentConfigList_To_api_DeploymentConfigList(in, out, s)
}

func autoConvert_v1_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in *deployapiv1.DeploymentConfigRollback, out *deployapi.DeploymentConfigRollback, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigRollback))(in)
	}
	if err := Convert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in *deployapiv1.DeploymentConfigRollback, out *deployapi.DeploymentConfigRollback, s conversion.Scope) error {
	return autoConvert_v1_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in, out, s)
}

func autoConvert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in *deployapiv1.DeploymentConfigRollbackSpec, out *deployapi.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigRollbackSpec))(in)
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func Convert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in *deployapiv1.DeploymentConfigRollbackSpec, out *deployapi.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	return autoConvert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in, out, s)
}

func autoConvert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec(in *deployapiv1.DeploymentConfigSpec, out *deployapi.DeploymentConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigSpec))(in)
	}
	if err := Convert_v1_DeploymentStrategy_To_api_DeploymentStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if in.Triggers != nil {
		out.Triggers = make([]deployapi.DeploymentTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := Convert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(&in.Triggers[i], &out.Triggers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	out.Replicas = in.Replicas
	out.Test = in.Test
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	// unable to generate simple pointer conversion for v1.PodTemplateSpec -> api.PodTemplateSpec
	if in.Template != nil {
		out.Template = new(api.PodTemplateSpec)
		if err := Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

func Convert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec(in *deployapiv1.DeploymentConfigSpec, out *deployapi.DeploymentConfigSpec, s conversion.Scope) error {
	return autoConvert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec(in, out, s)
}

func autoConvert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus(in *deployapiv1.DeploymentConfigStatus, out *deployapi.DeploymentConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigStatus))(in)
	}
	out.LatestVersion = in.LatestVersion
	// unable to generate simple pointer conversion for v1.DeploymentDetails -> api.DeploymentDetails
	if in.Details != nil {
		out.Details = new(deployapi.DeploymentDetails)
		if err := Convert_v1_DeploymentDetails_To_api_DeploymentDetails(in.Details, out.Details, s); err != nil {
			return err
		}
	} else {
		out.Details = nil
	}
	return nil
}

func Convert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus(in *deployapiv1.DeploymentConfigStatus, out *deployapi.DeploymentConfigStatus, s conversion.Scope) error {
	return autoConvert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus(in, out, s)
}

func autoConvert_v1_DeploymentDetails_To_api_DeploymentDetails(in *deployapiv1.DeploymentDetails, out *deployapi.DeploymentDetails, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentDetails))(in)
	}
	out.Message = in.Message
	if in.Causes != nil {
		out.Causes = make([]deployapi.DeploymentCause, len(in.Causes))
		for i := range in.Causes {
			if err := Convert_v1_DeploymentCause_To_api_DeploymentCause(&in.Causes[i], &out.Causes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Causes = nil
	}
	return nil
}

func Convert_v1_DeploymentDetails_To_api_DeploymentDetails(in *deployapiv1.DeploymentDetails, out *deployapi.DeploymentDetails, s conversion.Scope) error {
	return autoConvert_v1_DeploymentDetails_To_api_DeploymentDetails(in, out, s)
}

func autoConvert_v1_DeploymentLog_To_api_DeploymentLog(in *deployapiv1.DeploymentLog, out *deployapi.DeploymentLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentLog))(in)
	}
	return nil
}

func Convert_v1_DeploymentLog_To_api_DeploymentLog(in *deployapiv1.DeploymentLog, out *deployapi.DeploymentLog, s conversion.Scope) error {
	return autoConvert_v1_DeploymentLog_To_api_DeploymentLog(in, out, s)
}

func autoConvert_v1_DeploymentLogOptions_To_api_DeploymentLogOptions(in *deployapiv1.DeploymentLogOptions, out *deployapi.DeploymentLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentLogOptions))(in)
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

func Convert_v1_DeploymentLogOptions_To_api_DeploymentLogOptions(in *deployapiv1.DeploymentLogOptions, out *deployapi.DeploymentLogOptions, s conversion.Scope) error {
	return autoConvert_v1_DeploymentLogOptions_To_api_DeploymentLogOptions(in, out, s)
}

func autoConvert_v1_DeploymentStrategy_To_api_DeploymentStrategy(in *deployapiv1.DeploymentStrategy, out *deployapi.DeploymentStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentStrategy))(in)
	}
	out.Type = deployapi.DeploymentStrategyType(in.Type)
	// unable to generate simple pointer conversion for v1.CustomDeploymentStrategyParams -> api.CustomDeploymentStrategyParams
	if in.CustomParams != nil {
		out.CustomParams = new(deployapi.CustomDeploymentStrategyParams)
		if err := Convert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams(in.CustomParams, out.CustomParams, s); err != nil {
			return err
		}
	} else {
		out.CustomParams = nil
	}
	// unable to generate simple pointer conversion for v1.RecreateDeploymentStrategyParams -> api.RecreateDeploymentStrategyParams
	if in.RecreateParams != nil {
		out.RecreateParams = new(deployapi.RecreateDeploymentStrategyParams)
		if err := Convert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams(in.RecreateParams, out.RecreateParams, s); err != nil {
			return err
		}
	} else {
		out.RecreateParams = nil
	}
	// unable to generate simple pointer conversion for v1.RollingDeploymentStrategyParams -> api.RollingDeploymentStrategyParams
	if in.RollingParams != nil {
		out.RollingParams = new(deployapi.RollingDeploymentStrategyParams)
		if err := deployapiv1.Convert_v1_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams(in.RollingParams, out.RollingParams, s); err != nil {
			return err
		}
	} else {
		out.RollingParams = nil
	}
	if err := Convert_v1_ResourceRequirements_To_api_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
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

func Convert_v1_DeploymentStrategy_To_api_DeploymentStrategy(in *deployapiv1.DeploymentStrategy, out *deployapi.DeploymentStrategy, s conversion.Scope) error {
	return autoConvert_v1_DeploymentStrategy_To_api_DeploymentStrategy(in, out, s)
}

func autoConvert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams(in *deployapiv1.DeploymentTriggerImageChangeParams, out *deployapi.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentTriggerImageChangeParams))(in)
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
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.LastTriggeredImage = in.LastTriggeredImage
	return nil
}

func autoConvert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(in *deployapiv1.DeploymentTriggerPolicy, out *deployapi.DeploymentTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentTriggerPolicy))(in)
	}
	out.Type = deployapi.DeploymentTriggerType(in.Type)
	// unable to generate simple pointer conversion for v1.DeploymentTriggerImageChangeParams -> api.DeploymentTriggerImageChangeParams
	if in.ImageChangeParams != nil {
		out.ImageChangeParams = new(deployapi.DeploymentTriggerImageChangeParams)
		if err := deployapiv1.Convert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams(in.ImageChangeParams, out.ImageChangeParams, s); err != nil {
			return err
		}
	} else {
		out.ImageChangeParams = nil
	}
	return nil
}

func Convert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(in *deployapiv1.DeploymentTriggerPolicy, out *deployapi.DeploymentTriggerPolicy, s conversion.Scope) error {
	return autoConvert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(in, out, s)
}

func autoConvert_v1_ExecNewPodHook_To_api_ExecNewPodHook(in *deployapiv1.ExecNewPodHook, out *deployapi.ExecNewPodHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.ExecNewPodHook))(in)
	}
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ContainerName = in.ContainerName
	if in.Volumes != nil {
		out.Volumes = make([]string, len(in.Volumes))
		for i := range in.Volumes {
			out.Volumes[i] = in.Volumes[i]
		}
	} else {
		out.Volumes = nil
	}
	return nil
}

func Convert_v1_ExecNewPodHook_To_api_ExecNewPodHook(in *deployapiv1.ExecNewPodHook, out *deployapi.ExecNewPodHook, s conversion.Scope) error {
	return autoConvert_v1_ExecNewPodHook_To_api_ExecNewPodHook(in, out, s)
}

func autoConvert_v1_LifecycleHook_To_api_LifecycleHook(in *deployapiv1.LifecycleHook, out *deployapi.LifecycleHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.LifecycleHook))(in)
	}
	out.FailurePolicy = deployapi.LifecycleHookFailurePolicy(in.FailurePolicy)
	// unable to generate simple pointer conversion for v1.ExecNewPodHook -> api.ExecNewPodHook
	if in.ExecNewPod != nil {
		out.ExecNewPod = new(deployapi.ExecNewPodHook)
		if err := Convert_v1_ExecNewPodHook_To_api_ExecNewPodHook(in.ExecNewPod, out.ExecNewPod, s); err != nil {
			return err
		}
	} else {
		out.ExecNewPod = nil
	}
	if in.TagImages != nil {
		out.TagImages = make([]deployapi.TagImageHook, len(in.TagImages))
		for i := range in.TagImages {
			if err := Convert_v1_TagImageHook_To_api_TagImageHook(&in.TagImages[i], &out.TagImages[i], s); err != nil {
				return err
			}
		}
	} else {
		out.TagImages = nil
	}
	return nil
}

func Convert_v1_LifecycleHook_To_api_LifecycleHook(in *deployapiv1.LifecycleHook, out *deployapi.LifecycleHook, s conversion.Scope) error {
	return autoConvert_v1_LifecycleHook_To_api_LifecycleHook(in, out, s)
}

func autoConvert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams(in *deployapiv1.RecreateDeploymentStrategyParams, out *deployapi.RecreateDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.RecreateDeploymentStrategyParams))(in)
	}
	if in.TimeoutSeconds != nil {
		out.TimeoutSeconds = new(int64)
		*out.TimeoutSeconds = *in.TimeoutSeconds
	} else {
		out.TimeoutSeconds = nil
	}
	// unable to generate simple pointer conversion for v1.LifecycleHook -> api.LifecycleHook
	if in.Pre != nil {
		out.Pre = new(deployapi.LifecycleHook)
		if err := Convert_v1_LifecycleHook_To_api_LifecycleHook(in.Pre, out.Pre, s); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	// unable to generate simple pointer conversion for v1.LifecycleHook -> api.LifecycleHook
	if in.Mid != nil {
		out.Mid = new(deployapi.LifecycleHook)
		if err := Convert_v1_LifecycleHook_To_api_LifecycleHook(in.Mid, out.Mid, s); err != nil {
			return err
		}
	} else {
		out.Mid = nil
	}
	// unable to generate simple pointer conversion for v1.LifecycleHook -> api.LifecycleHook
	if in.Post != nil {
		out.Post = new(deployapi.LifecycleHook)
		if err := Convert_v1_LifecycleHook_To_api_LifecycleHook(in.Post, out.Post, s); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func Convert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams(in *deployapiv1.RecreateDeploymentStrategyParams, out *deployapi.RecreateDeploymentStrategyParams, s conversion.Scope) error {
	return autoConvert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams(in, out, s)
}

func autoConvert_v1_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams(in *deployapiv1.RollingDeploymentStrategyParams, out *deployapi.RollingDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.RollingDeploymentStrategyParams))(in)
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
	// unable to generate simple pointer conversion for v1.LifecycleHook -> api.LifecycleHook
	if in.Pre != nil {
		out.Pre = new(deployapi.LifecycleHook)
		if err := Convert_v1_LifecycleHook_To_api_LifecycleHook(in.Pre, out.Pre, s); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	// unable to generate simple pointer conversion for v1.LifecycleHook -> api.LifecycleHook
	if in.Post != nil {
		out.Post = new(deployapi.LifecycleHook)
		if err := Convert_v1_LifecycleHook_To_api_LifecycleHook(in.Post, out.Post, s); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func autoConvert_v1_TagImageHook_To_api_TagImageHook(in *deployapiv1.TagImageHook, out *deployapi.TagImageHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.TagImageHook))(in)
	}
	out.ContainerName = in.ContainerName
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_TagImageHook_To_api_TagImageHook(in *deployapiv1.TagImageHook, out *deployapi.TagImageHook, s conversion.Scope) error {
	return autoConvert_v1_TagImageHook_To_api_TagImageHook(in, out, s)
}

func autoConvert_api_Image_To_v1_Image(in *imageapi.Image, out *imageapiv1.Image, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.Image))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DockerImageReference = in.DockerImageReference
	if err := s.Convert(&in.DockerImageMetadata, &out.DockerImageMetadata, 0); err != nil {
		return err
	}
	out.DockerImageMetadataVersion = in.DockerImageMetadataVersion
	out.DockerImageManifest = in.DockerImageManifest
	if in.DockerImageLayers != nil {
		out.DockerImageLayers = make([]imageapiv1.ImageLayer, len(in.DockerImageLayers))
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

func autoConvert_api_ImageImportSpec_To_v1_ImageImportSpec(in *imageapi.ImageImportSpec, out *imageapiv1.ImageImportSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageImportSpec))(in)
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.To != nil {
		out.To = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	if err := Convert_api_TagImportPolicy_To_v1_TagImportPolicy(&in.ImportPolicy, &out.ImportPolicy, s); err != nil {
		return err
	}
	out.IncludeManifest = in.IncludeManifest
	return nil
}

func Convert_api_ImageImportSpec_To_v1_ImageImportSpec(in *imageapi.ImageImportSpec, out *imageapiv1.ImageImportSpec, s conversion.Scope) error {
	return autoConvert_api_ImageImportSpec_To_v1_ImageImportSpec(in, out, s)
}

func autoConvert_api_ImageImportStatus_To_v1_ImageImportStatus(in *imageapi.ImageImportStatus, out *imageapiv1.ImageImportStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageImportStatus))(in)
	}
	out.Tag = in.Tag
	if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.Image -> v1.Image
	if in.Image != nil {
		out.Image = new(imageapiv1.Image)
		if err := imageapiv1.Convert_api_Image_To_v1_Image(in.Image, out.Image, s); err != nil {
			return err
		}
	} else {
		out.Image = nil
	}
	return nil
}

func Convert_api_ImageImportStatus_To_v1_ImageImportStatus(in *imageapi.ImageImportStatus, out *imageapiv1.ImageImportStatus, s conversion.Scope) error {
	return autoConvert_api_ImageImportStatus_To_v1_ImageImportStatus(in, out, s)
}

func autoConvert_api_ImageList_To_v1_ImageList(in *imageapi.ImageList, out *imageapiv1.ImageList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1.Image, len(in.Items))
		for i := range in.Items {
			if err := imageapiv1.Convert_api_Image_To_v1_Image(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ImageList_To_v1_ImageList(in *imageapi.ImageList, out *imageapiv1.ImageList, s conversion.Scope) error {
	return autoConvert_api_ImageList_To_v1_ImageList(in, out, s)
}

func autoConvert_api_ImageStream_To_v1_ImageStream(in *imageapi.ImageStream, out *imageapiv1.ImageStream, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStream))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := imageapiv1.Convert_api_ImageStreamSpec_To_v1_ImageStreamSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := imageapiv1.Convert_api_ImageStreamStatus_To_v1_ImageStreamStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_ImageStream_To_v1_ImageStream(in *imageapi.ImageStream, out *imageapiv1.ImageStream, s conversion.Scope) error {
	return autoConvert_api_ImageStream_To_v1_ImageStream(in, out, s)
}

func autoConvert_api_ImageStreamImage_To_v1_ImageStreamImage(in *imageapi.ImageStreamImage, out *imageapiv1.ImageStreamImage, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamImage))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := imageapiv1.Convert_api_Image_To_v1_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_ImageStreamImage_To_v1_ImageStreamImage(in *imageapi.ImageStreamImage, out *imageapiv1.ImageStreamImage, s conversion.Scope) error {
	return autoConvert_api_ImageStreamImage_To_v1_ImageStreamImage(in, out, s)
}

func autoConvert_api_ImageStreamImport_To_v1_ImageStreamImport(in *imageapi.ImageStreamImport, out *imageapiv1.ImageStreamImport, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamImport))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_ImageStreamImportSpec_To_v1_ImageStreamImportSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_ImageStreamImportStatus_To_v1_ImageStreamImportStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_ImageStreamImport_To_v1_ImageStreamImport(in *imageapi.ImageStreamImport, out *imageapiv1.ImageStreamImport, s conversion.Scope) error {
	return autoConvert_api_ImageStreamImport_To_v1_ImageStreamImport(in, out, s)
}

func autoConvert_api_ImageStreamImportSpec_To_v1_ImageStreamImportSpec(in *imageapi.ImageStreamImportSpec, out *imageapiv1.ImageStreamImportSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamImportSpec))(in)
	}
	out.Import = in.Import
	// unable to generate simple pointer conversion for api.RepositoryImportSpec -> v1.RepositoryImportSpec
	if in.Repository != nil {
		out.Repository = new(imageapiv1.RepositoryImportSpec)
		if err := Convert_api_RepositoryImportSpec_To_v1_RepositoryImportSpec(in.Repository, out.Repository, s); err != nil {
			return err
		}
	} else {
		out.Repository = nil
	}
	if in.Images != nil {
		out.Images = make([]imageapiv1.ImageImportSpec, len(in.Images))
		for i := range in.Images {
			if err := Convert_api_ImageImportSpec_To_v1_ImageImportSpec(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	return nil
}

func Convert_api_ImageStreamImportSpec_To_v1_ImageStreamImportSpec(in *imageapi.ImageStreamImportSpec, out *imageapiv1.ImageStreamImportSpec, s conversion.Scope) error {
	return autoConvert_api_ImageStreamImportSpec_To_v1_ImageStreamImportSpec(in, out, s)
}

func autoConvert_api_ImageStreamImportStatus_To_v1_ImageStreamImportStatus(in *imageapi.ImageStreamImportStatus, out *imageapiv1.ImageStreamImportStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamImportStatus))(in)
	}
	// unable to generate simple pointer conversion for api.ImageStream -> v1.ImageStream
	if in.Import != nil {
		out.Import = new(imageapiv1.ImageStream)
		if err := Convert_api_ImageStream_To_v1_ImageStream(in.Import, out.Import, s); err != nil {
			return err
		}
	} else {
		out.Import = nil
	}
	// unable to generate simple pointer conversion for api.RepositoryImportStatus -> v1.RepositoryImportStatus
	if in.Repository != nil {
		out.Repository = new(imageapiv1.RepositoryImportStatus)
		if err := Convert_api_RepositoryImportStatus_To_v1_RepositoryImportStatus(in.Repository, out.Repository, s); err != nil {
			return err
		}
	} else {
		out.Repository = nil
	}
	if in.Images != nil {
		out.Images = make([]imageapiv1.ImageImportStatus, len(in.Images))
		for i := range in.Images {
			if err := Convert_api_ImageImportStatus_To_v1_ImageImportStatus(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	return nil
}

func Convert_api_ImageStreamImportStatus_To_v1_ImageStreamImportStatus(in *imageapi.ImageStreamImportStatus, out *imageapiv1.ImageStreamImportStatus, s conversion.Scope) error {
	return autoConvert_api_ImageStreamImportStatus_To_v1_ImageStreamImportStatus(in, out, s)
}

func autoConvert_api_ImageStreamList_To_v1_ImageStreamList(in *imageapi.ImageStreamList, out *imageapiv1.ImageStreamList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_ImageStream_To_v1_ImageStream(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ImageStreamList_To_v1_ImageStreamList(in *imageapi.ImageStreamList, out *imageapiv1.ImageStreamList, s conversion.Scope) error {
	return autoConvert_api_ImageStreamList_To_v1_ImageStreamList(in, out, s)
}

func autoConvert_api_ImageStreamMapping_To_v1_ImageStreamMapping(in *imageapi.ImageStreamMapping, out *imageapiv1.ImageStreamMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamMapping))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.DockerImageRepository has no peer in out
	if err := imageapiv1.Convert_api_Image_To_v1_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func autoConvert_api_ImageStreamSpec_To_v1_ImageStreamSpec(in *imageapi.ImageStreamSpec, out *imageapiv1.ImageStreamSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamSpec))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := imageapiv1.Convert_api_TagReferenceMap_to_v1_TagReferenceArray(&in.Tags, &out.Tags, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ImageStreamStatus_To_v1_ImageStreamStatus(in *imageapi.ImageStreamStatus, out *imageapiv1.ImageStreamStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamStatus))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := imageapiv1.Convert_api_TagEventListArray_to_v1_NamedTagEventListArray(&in.Tags, &out.Tags, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_api_ImageStreamTag_To_v1_ImageStreamTag(in *imageapi.ImageStreamTag, out *imageapiv1.ImageStreamTag, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamTag))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.TagReference -> v1.TagReference
	if in.Tag != nil {
		out.Tag = new(imageapiv1.TagReference)
		if err := Convert_api_TagReference_To_v1_TagReference(in.Tag, out.Tag, s); err != nil {
			return err
		}
	} else {
		out.Tag = nil
	}
	out.Generation = in.Generation
	if in.Conditions != nil {
		out.Conditions = make([]imageapiv1.TagEventCondition, len(in.Conditions))
		for i := range in.Conditions {
			if err := Convert_api_TagEventCondition_To_v1_TagEventCondition(&in.Conditions[i], &out.Conditions[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Conditions = nil
	}
	if err := imageapiv1.Convert_api_Image_To_v1_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_ImageStreamTag_To_v1_ImageStreamTag(in *imageapi.ImageStreamTag, out *imageapiv1.ImageStreamTag, s conversion.Scope) error {
	return autoConvert_api_ImageStreamTag_To_v1_ImageStreamTag(in, out, s)
}

func autoConvert_api_ImageStreamTagList_To_v1_ImageStreamTagList(in *imageapi.ImageStreamTagList, out *imageapiv1.ImageStreamTagList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamTagList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1.ImageStreamTag, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_ImageStreamTag_To_v1_ImageStreamTag(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ImageStreamTagList_To_v1_ImageStreamTagList(in *imageapi.ImageStreamTagList, out *imageapiv1.ImageStreamTagList, s conversion.Scope) error {
	return autoConvert_api_ImageStreamTagList_To_v1_ImageStreamTagList(in, out, s)
}

func autoConvert_api_RepositoryImportSpec_To_v1_RepositoryImportSpec(in *imageapi.RepositoryImportSpec, out *imageapiv1.RepositoryImportSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.RepositoryImportSpec))(in)
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if err := Convert_api_TagImportPolicy_To_v1_TagImportPolicy(&in.ImportPolicy, &out.ImportPolicy, s); err != nil {
		return err
	}
	out.IncludeManifest = in.IncludeManifest
	return nil
}

func Convert_api_RepositoryImportSpec_To_v1_RepositoryImportSpec(in *imageapi.RepositoryImportSpec, out *imageapiv1.RepositoryImportSpec, s conversion.Scope) error {
	return autoConvert_api_RepositoryImportSpec_To_v1_RepositoryImportSpec(in, out, s)
}

func autoConvert_api_RepositoryImportStatus_To_v1_RepositoryImportStatus(in *imageapi.RepositoryImportStatus, out *imageapiv1.RepositoryImportStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.RepositoryImportStatus))(in)
	}
	if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
		return err
	}
	if in.Images != nil {
		out.Images = make([]imageapiv1.ImageImportStatus, len(in.Images))
		for i := range in.Images {
			if err := Convert_api_ImageImportStatus_To_v1_ImageImportStatus(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	if in.AdditionalTags != nil {
		out.AdditionalTags = make([]string, len(in.AdditionalTags))
		for i := range in.AdditionalTags {
			out.AdditionalTags[i] = in.AdditionalTags[i]
		}
	} else {
		out.AdditionalTags = nil
	}
	return nil
}

func Convert_api_RepositoryImportStatus_To_v1_RepositoryImportStatus(in *imageapi.RepositoryImportStatus, out *imageapiv1.RepositoryImportStatus, s conversion.Scope) error {
	return autoConvert_api_RepositoryImportStatus_To_v1_RepositoryImportStatus(in, out, s)
}

func autoConvert_api_TagEventCondition_To_v1_TagEventCondition(in *imageapi.TagEventCondition, out *imageapiv1.TagEventCondition, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.TagEventCondition))(in)
	}
	out.Type = imageapiv1.TagEventConditionType(in.Type)
	out.Status = apiv1.ConditionStatus(in.Status)
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastTransitionTime, &out.LastTransitionTime, s); err != nil {
		return err
	}
	out.Reason = in.Reason
	out.Message = in.Message
	out.Generation = in.Generation
	return nil
}

func Convert_api_TagEventCondition_To_v1_TagEventCondition(in *imageapi.TagEventCondition, out *imageapiv1.TagEventCondition, s conversion.Scope) error {
	return autoConvert_api_TagEventCondition_To_v1_TagEventCondition(in, out, s)
}

func autoConvert_api_TagImportPolicy_To_v1_TagImportPolicy(in *imageapi.TagImportPolicy, out *imageapiv1.TagImportPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.TagImportPolicy))(in)
	}
	out.Insecure = in.Insecure
	out.Scheduled = in.Scheduled
	return nil
}

func Convert_api_TagImportPolicy_To_v1_TagImportPolicy(in *imageapi.TagImportPolicy, out *imageapiv1.TagImportPolicy, s conversion.Scope) error {
	return autoConvert_api_TagImportPolicy_To_v1_TagImportPolicy(in, out, s)
}

func autoConvert_api_TagReference_To_v1_TagReference(in *imageapi.TagReference, out *imageapiv1.TagReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.TagReference))(in)
	}
	out.Name = in.Name
	if in.Annotations != nil {
		out.Annotations = make(map[string]string)
		for key, val := range in.Annotations {
			out.Annotations[key] = val
		}
	} else {
		out.Annotations = nil
	}
	// unable to generate simple pointer conversion for api.ObjectReference -> v1.ObjectReference
	if in.From != nil {
		out.From = new(apiv1.ObjectReference)
		if err := Convert_api_ObjectReference_To_v1_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	out.Reference = in.Reference
	if in.Generation != nil {
		out.Generation = new(int64)
		*out.Generation = *in.Generation
	} else {
		out.Generation = nil
	}
	if err := Convert_api_TagImportPolicy_To_v1_TagImportPolicy(&in.ImportPolicy, &out.ImportPolicy, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_TagReference_To_v1_TagReference(in *imageapi.TagReference, out *imageapiv1.TagReference, s conversion.Scope) error {
	return autoConvert_api_TagReference_To_v1_TagReference(in, out, s)
}

func autoConvert_v1_Image_To_api_Image(in *imageapiv1.Image, out *imageapi.Image, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.Image))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func autoConvert_v1_ImageImportSpec_To_api_ImageImportSpec(in *imageapiv1.ImageImportSpec, out *imageapi.ImageImportSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageImportSpec))(in)
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.To != nil {
		out.To = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	if err := Convert_v1_TagImportPolicy_To_api_TagImportPolicy(&in.ImportPolicy, &out.ImportPolicy, s); err != nil {
		return err
	}
	out.IncludeManifest = in.IncludeManifest
	return nil
}

func Convert_v1_ImageImportSpec_To_api_ImageImportSpec(in *imageapiv1.ImageImportSpec, out *imageapi.ImageImportSpec, s conversion.Scope) error {
	return autoConvert_v1_ImageImportSpec_To_api_ImageImportSpec(in, out, s)
}

func autoConvert_v1_ImageImportStatus_To_api_ImageImportStatus(in *imageapiv1.ImageImportStatus, out *imageapi.ImageImportStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageImportStatus))(in)
	}
	if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1.Image -> api.Image
	if in.Image != nil {
		out.Image = new(imageapi.Image)
		if err := imageapiv1.Convert_v1_Image_To_api_Image(in.Image, out.Image, s); err != nil {
			return err
		}
	} else {
		out.Image = nil
	}
	out.Tag = in.Tag
	return nil
}

func Convert_v1_ImageImportStatus_To_api_ImageImportStatus(in *imageapiv1.ImageImportStatus, out *imageapi.ImageImportStatus, s conversion.Scope) error {
	return autoConvert_v1_ImageImportStatus_To_api_ImageImportStatus(in, out, s)
}

func autoConvert_v1_ImageList_To_api_ImageList(in *imageapiv1.ImageList, out *imageapi.ImageList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapi.Image, len(in.Items))
		for i := range in.Items {
			if err := imageapiv1.Convert_v1_Image_To_api_Image(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ImageList_To_api_ImageList(in *imageapiv1.ImageList, out *imageapi.ImageList, s conversion.Scope) error {
	return autoConvert_v1_ImageList_To_api_ImageList(in, out, s)
}

func autoConvert_v1_ImageStream_To_api_ImageStream(in *imageapiv1.ImageStream, out *imageapi.ImageStream, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStream))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := imageapiv1.Convert_v1_ImageStreamSpec_To_api_ImageStreamSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := imageapiv1.Convert_v1_ImageStreamStatus_To_api_ImageStreamStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_ImageStream_To_api_ImageStream(in *imageapiv1.ImageStream, out *imageapi.ImageStream, s conversion.Scope) error {
	return autoConvert_v1_ImageStream_To_api_ImageStream(in, out, s)
}

func autoConvert_v1_ImageStreamImage_To_api_ImageStreamImage(in *imageapiv1.ImageStreamImage, out *imageapi.ImageStreamImage, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamImage))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := imageapiv1.Convert_v1_Image_To_api_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_ImageStreamImage_To_api_ImageStreamImage(in *imageapiv1.ImageStreamImage, out *imageapi.ImageStreamImage, s conversion.Scope) error {
	return autoConvert_v1_ImageStreamImage_To_api_ImageStreamImage(in, out, s)
}

func autoConvert_v1_ImageStreamImport_To_api_ImageStreamImport(in *imageapiv1.ImageStreamImport, out *imageapi.ImageStreamImport, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamImport))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_ImageStreamImportSpec_To_api_ImageStreamImportSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1_ImageStreamImportStatus_To_api_ImageStreamImportStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_ImageStreamImport_To_api_ImageStreamImport(in *imageapiv1.ImageStreamImport, out *imageapi.ImageStreamImport, s conversion.Scope) error {
	return autoConvert_v1_ImageStreamImport_To_api_ImageStreamImport(in, out, s)
}

func autoConvert_v1_ImageStreamImportSpec_To_api_ImageStreamImportSpec(in *imageapiv1.ImageStreamImportSpec, out *imageapi.ImageStreamImportSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamImportSpec))(in)
	}
	out.Import = in.Import
	// unable to generate simple pointer conversion for v1.RepositoryImportSpec -> api.RepositoryImportSpec
	if in.Repository != nil {
		out.Repository = new(imageapi.RepositoryImportSpec)
		if err := Convert_v1_RepositoryImportSpec_To_api_RepositoryImportSpec(in.Repository, out.Repository, s); err != nil {
			return err
		}
	} else {
		out.Repository = nil
	}
	if in.Images != nil {
		out.Images = make([]imageapi.ImageImportSpec, len(in.Images))
		for i := range in.Images {
			if err := Convert_v1_ImageImportSpec_To_api_ImageImportSpec(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	return nil
}

func Convert_v1_ImageStreamImportSpec_To_api_ImageStreamImportSpec(in *imageapiv1.ImageStreamImportSpec, out *imageapi.ImageStreamImportSpec, s conversion.Scope) error {
	return autoConvert_v1_ImageStreamImportSpec_To_api_ImageStreamImportSpec(in, out, s)
}

func autoConvert_v1_ImageStreamImportStatus_To_api_ImageStreamImportStatus(in *imageapiv1.ImageStreamImportStatus, out *imageapi.ImageStreamImportStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamImportStatus))(in)
	}
	// unable to generate simple pointer conversion for v1.ImageStream -> api.ImageStream
	if in.Import != nil {
		out.Import = new(imageapi.ImageStream)
		if err := Convert_v1_ImageStream_To_api_ImageStream(in.Import, out.Import, s); err != nil {
			return err
		}
	} else {
		out.Import = nil
	}
	// unable to generate simple pointer conversion for v1.RepositoryImportStatus -> api.RepositoryImportStatus
	if in.Repository != nil {
		out.Repository = new(imageapi.RepositoryImportStatus)
		if err := Convert_v1_RepositoryImportStatus_To_api_RepositoryImportStatus(in.Repository, out.Repository, s); err != nil {
			return err
		}
	} else {
		out.Repository = nil
	}
	if in.Images != nil {
		out.Images = make([]imageapi.ImageImportStatus, len(in.Images))
		for i := range in.Images {
			if err := Convert_v1_ImageImportStatus_To_api_ImageImportStatus(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	return nil
}

func Convert_v1_ImageStreamImportStatus_To_api_ImageStreamImportStatus(in *imageapiv1.ImageStreamImportStatus, out *imageapi.ImageStreamImportStatus, s conversion.Scope) error {
	return autoConvert_v1_ImageStreamImportStatus_To_api_ImageStreamImportStatus(in, out, s)
}

func autoConvert_v1_ImageStreamList_To_api_ImageStreamList(in *imageapiv1.ImageStreamList, out *imageapi.ImageStreamList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapi.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_ImageStream_To_api_ImageStream(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ImageStreamList_To_api_ImageStreamList(in *imageapiv1.ImageStreamList, out *imageapi.ImageStreamList, s conversion.Scope) error {
	return autoConvert_v1_ImageStreamList_To_api_ImageStreamList(in, out, s)
}

func autoConvert_v1_ImageStreamMapping_To_api_ImageStreamMapping(in *imageapiv1.ImageStreamMapping, out *imageapi.ImageStreamMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamMapping))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := imageapiv1.Convert_v1_Image_To_api_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func autoConvert_v1_ImageStreamSpec_To_api_ImageStreamSpec(in *imageapiv1.ImageStreamSpec, out *imageapi.ImageStreamSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamSpec))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := imageapiv1.Convert_v1_TagReferenceArray_to_api_TagReferenceMap(&in.Tags, &out.Tags, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_ImageStreamStatus_To_api_ImageStreamStatus(in *imageapiv1.ImageStreamStatus, out *imageapi.ImageStreamStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamStatus))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := imageapiv1.Convert_v1_NamedTagEventListArray_to_api_TagEventListArray(&in.Tags, &out.Tags, s); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1_ImageStreamTag_To_api_ImageStreamTag(in *imageapiv1.ImageStreamTag, out *imageapi.ImageStreamTag, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamTag))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1.TagReference -> api.TagReference
	if in.Tag != nil {
		out.Tag = new(imageapi.TagReference)
		if err := Convert_v1_TagReference_To_api_TagReference(in.Tag, out.Tag, s); err != nil {
			return err
		}
	} else {
		out.Tag = nil
	}
	out.Generation = in.Generation
	if in.Conditions != nil {
		out.Conditions = make([]imageapi.TagEventCondition, len(in.Conditions))
		for i := range in.Conditions {
			if err := Convert_v1_TagEventCondition_To_api_TagEventCondition(&in.Conditions[i], &out.Conditions[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Conditions = nil
	}
	if err := imageapiv1.Convert_v1_Image_To_api_Image(&in.Image, &out.Image, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_ImageStreamTag_To_api_ImageStreamTag(in *imageapiv1.ImageStreamTag, out *imageapi.ImageStreamTag, s conversion.Scope) error {
	return autoConvert_v1_ImageStreamTag_To_api_ImageStreamTag(in, out, s)
}

func autoConvert_v1_ImageStreamTagList_To_api_ImageStreamTagList(in *imageapiv1.ImageStreamTagList, out *imageapi.ImageStreamTagList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamTagList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapi.ImageStreamTag, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_ImageStreamTag_To_api_ImageStreamTag(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ImageStreamTagList_To_api_ImageStreamTagList(in *imageapiv1.ImageStreamTagList, out *imageapi.ImageStreamTagList, s conversion.Scope) error {
	return autoConvert_v1_ImageStreamTagList_To_api_ImageStreamTagList(in, out, s)
}

func autoConvert_v1_RepositoryImportSpec_To_api_RepositoryImportSpec(in *imageapiv1.RepositoryImportSpec, out *imageapi.RepositoryImportSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.RepositoryImportSpec))(in)
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if err := Convert_v1_TagImportPolicy_To_api_TagImportPolicy(&in.ImportPolicy, &out.ImportPolicy, s); err != nil {
		return err
	}
	out.IncludeManifest = in.IncludeManifest
	return nil
}

func Convert_v1_RepositoryImportSpec_To_api_RepositoryImportSpec(in *imageapiv1.RepositoryImportSpec, out *imageapi.RepositoryImportSpec, s conversion.Scope) error {
	return autoConvert_v1_RepositoryImportSpec_To_api_RepositoryImportSpec(in, out, s)
}

func autoConvert_v1_RepositoryImportStatus_To_api_RepositoryImportStatus(in *imageapiv1.RepositoryImportStatus, out *imageapi.RepositoryImportStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.RepositoryImportStatus))(in)
	}
	if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
		return err
	}
	if in.Images != nil {
		out.Images = make([]imageapi.ImageImportStatus, len(in.Images))
		for i := range in.Images {
			if err := Convert_v1_ImageImportStatus_To_api_ImageImportStatus(&in.Images[i], &out.Images[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Images = nil
	}
	if in.AdditionalTags != nil {
		out.AdditionalTags = make([]string, len(in.AdditionalTags))
		for i := range in.AdditionalTags {
			out.AdditionalTags[i] = in.AdditionalTags[i]
		}
	} else {
		out.AdditionalTags = nil
	}
	return nil
}

func Convert_v1_RepositoryImportStatus_To_api_RepositoryImportStatus(in *imageapiv1.RepositoryImportStatus, out *imageapi.RepositoryImportStatus, s conversion.Scope) error {
	return autoConvert_v1_RepositoryImportStatus_To_api_RepositoryImportStatus(in, out, s)
}

func autoConvert_v1_TagEventCondition_To_api_TagEventCondition(in *imageapiv1.TagEventCondition, out *imageapi.TagEventCondition, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.TagEventCondition))(in)
	}
	out.Type = imageapi.TagEventConditionType(in.Type)
	out.Status = api.ConditionStatus(in.Status)
	if err := api.Convert_unversioned_Time_To_unversioned_Time(&in.LastTransitionTime, &out.LastTransitionTime, s); err != nil {
		return err
	}
	out.Reason = in.Reason
	out.Message = in.Message
	out.Generation = in.Generation
	return nil
}

func Convert_v1_TagEventCondition_To_api_TagEventCondition(in *imageapiv1.TagEventCondition, out *imageapi.TagEventCondition, s conversion.Scope) error {
	return autoConvert_v1_TagEventCondition_To_api_TagEventCondition(in, out, s)
}

func autoConvert_v1_TagImportPolicy_To_api_TagImportPolicy(in *imageapiv1.TagImportPolicy, out *imageapi.TagImportPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.TagImportPolicy))(in)
	}
	out.Insecure = in.Insecure
	out.Scheduled = in.Scheduled
	return nil
}

func Convert_v1_TagImportPolicy_To_api_TagImportPolicy(in *imageapiv1.TagImportPolicy, out *imageapi.TagImportPolicy, s conversion.Scope) error {
	return autoConvert_v1_TagImportPolicy_To_api_TagImportPolicy(in, out, s)
}

func autoConvert_v1_TagReference_To_api_TagReference(in *imageapiv1.TagReference, out *imageapi.TagReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.TagReference))(in)
	}
	out.Name = in.Name
	if in.Annotations != nil {
		out.Annotations = make(map[string]string)
		for key, val := range in.Annotations {
			out.Annotations[key] = val
		}
	} else {
		out.Annotations = nil
	}
	// unable to generate simple pointer conversion for v1.ObjectReference -> api.ObjectReference
	if in.From != nil {
		out.From = new(api.ObjectReference)
		if err := Convert_v1_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	out.Reference = in.Reference
	if in.Generation != nil {
		out.Generation = new(int64)
		*out.Generation = *in.Generation
	} else {
		out.Generation = nil
	}
	if err := Convert_v1_TagImportPolicy_To_api_TagImportPolicy(&in.ImportPolicy, &out.ImportPolicy, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_TagReference_To_api_TagReference(in *imageapiv1.TagReference, out *imageapi.TagReference, s conversion.Scope) error {
	return autoConvert_v1_TagReference_To_api_TagReference(in, out, s)
}

func autoConvert_api_OAuthAccessToken_To_v1_OAuthAccessToken(in *oauthapi.OAuthAccessToken, out *oauthapiv1.OAuthAccessToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAccessToken))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_api_OAuthAccessToken_To_v1_OAuthAccessToken(in *oauthapi.OAuthAccessToken, out *oauthapiv1.OAuthAccessToken, s conversion.Scope) error {
	return autoConvert_api_OAuthAccessToken_To_v1_OAuthAccessToken(in, out, s)
}

func autoConvert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList(in *oauthapi.OAuthAccessTokenList, out *oauthapiv1.OAuthAccessTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAccessTokenList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_OAuthAccessToken_To_v1_OAuthAccessToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList(in *oauthapi.OAuthAccessTokenList, out *oauthapiv1.OAuthAccessTokenList, s conversion.Scope) error {
	return autoConvert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList(in, out, s)
}

func autoConvert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(in *oauthapi.OAuthAuthorizeToken, out *oauthapiv1.OAuthAuthorizeToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAuthorizeToken))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(in *oauthapi.OAuthAuthorizeToken, out *oauthapiv1.OAuthAuthorizeToken, s conversion.Scope) error {
	return autoConvert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(in, out, s)
}

func autoConvert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList(in *oauthapi.OAuthAuthorizeTokenList, out *oauthapiv1.OAuthAuthorizeTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAuthorizeTokenList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList(in *oauthapi.OAuthAuthorizeTokenList, out *oauthapiv1.OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoConvert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList(in, out, s)
}

func autoConvert_api_OAuthClient_To_v1_OAuthClient(in *oauthapi.OAuthClient, out *oauthapiv1.OAuthClient, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClient))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_api_OAuthClient_To_v1_OAuthClient(in *oauthapi.OAuthClient, out *oauthapiv1.OAuthClient, s conversion.Scope) error {
	return autoConvert_api_OAuthClient_To_v1_OAuthClient(in, out, s)
}

func autoConvert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(in *oauthapi.OAuthClientAuthorization, out *oauthapiv1.OAuthClientAuthorization, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientAuthorization))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(in *oauthapi.OAuthClientAuthorization, out *oauthapiv1.OAuthClientAuthorization, s conversion.Scope) error {
	return autoConvert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(in, out, s)
}

func autoConvert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList(in *oauthapi.OAuthClientAuthorizationList, out *oauthapiv1.OAuthClientAuthorizationList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientAuthorizationList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList(in *oauthapi.OAuthClientAuthorizationList, out *oauthapiv1.OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoConvert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList(in, out, s)
}

func autoConvert_api_OAuthClientList_To_v1_OAuthClientList(in *oauthapi.OAuthClientList, out *oauthapiv1.OAuthClientList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_OAuthClient_To_v1_OAuthClient(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_OAuthClientList_To_v1_OAuthClientList(in *oauthapi.OAuthClientList, out *oauthapiv1.OAuthClientList, s conversion.Scope) error {
	return autoConvert_api_OAuthClientList_To_v1_OAuthClientList(in, out, s)
}

func autoConvert_v1_OAuthAccessToken_To_api_OAuthAccessToken(in *oauthapiv1.OAuthAccessToken, out *oauthapi.OAuthAccessToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthAccessToken))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_v1_OAuthAccessToken_To_api_OAuthAccessToken(in *oauthapiv1.OAuthAccessToken, out *oauthapi.OAuthAccessToken, s conversion.Scope) error {
	return autoConvert_v1_OAuthAccessToken_To_api_OAuthAccessToken(in, out, s)
}

func autoConvert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *oauthapiv1.OAuthAccessTokenList, out *oauthapi.OAuthAccessTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthAccessTokenList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_OAuthAccessToken_To_api_OAuthAccessToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *oauthapiv1.OAuthAccessTokenList, out *oauthapi.OAuthAccessTokenList, s conversion.Scope) error {
	return autoConvert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in, out, s)
}

func autoConvert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *oauthapiv1.OAuthAuthorizeToken, out *oauthapi.OAuthAuthorizeToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthAuthorizeToken))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *oauthapiv1.OAuthAuthorizeToken, out *oauthapi.OAuthAuthorizeToken, s conversion.Scope) error {
	return autoConvert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in, out, s)
}

func autoConvert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *oauthapiv1.OAuthAuthorizeTokenList, out *oauthapi.OAuthAuthorizeTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthAuthorizeTokenList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *oauthapiv1.OAuthAuthorizeTokenList, out *oauthapi.OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoConvert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in, out, s)
}

func autoConvert_v1_OAuthClient_To_api_OAuthClient(in *oauthapiv1.OAuthClient, out *oauthapi.OAuthClient, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthClient))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_v1_OAuthClient_To_api_OAuthClient(in *oauthapiv1.OAuthClient, out *oauthapi.OAuthClient, s conversion.Scope) error {
	return autoConvert_v1_OAuthClient_To_api_OAuthClient(in, out, s)
}

func autoConvert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *oauthapiv1.OAuthClientAuthorization, out *oauthapi.OAuthClientAuthorization, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthClientAuthorization))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *oauthapiv1.OAuthClientAuthorization, out *oauthapi.OAuthClientAuthorization, s conversion.Scope) error {
	return autoConvert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in, out, s)
}

func autoConvert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *oauthapiv1.OAuthClientAuthorizationList, out *oauthapi.OAuthClientAuthorizationList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthClientAuthorizationList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *oauthapiv1.OAuthClientAuthorizationList, out *oauthapi.OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoConvert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in, out, s)
}

func autoConvert_v1_OAuthClientList_To_api_OAuthClientList(in *oauthapiv1.OAuthClientList, out *oauthapi.OAuthClientList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthClientList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_OAuthClient_To_api_OAuthClient(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_OAuthClientList_To_api_OAuthClientList(in *oauthapiv1.OAuthClientList, out *oauthapi.OAuthClientList, s conversion.Scope) error {
	return autoConvert_v1_OAuthClientList_To_api_OAuthClientList(in, out, s)
}

func autoConvert_api_Project_To_v1_Project(in *projectapi.Project, out *projectapiv1.Project, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.Project))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_ProjectSpec_To_v1_ProjectSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_ProjectStatus_To_v1_ProjectStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_Project_To_v1_Project(in *projectapi.Project, out *projectapiv1.Project, s conversion.Scope) error {
	return autoConvert_api_Project_To_v1_Project(in, out, s)
}

func autoConvert_api_ProjectList_To_v1_ProjectList(in *projectapi.ProjectList, out *projectapiv1.ProjectList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]projectapiv1.Project, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Project_To_v1_Project(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ProjectList_To_v1_ProjectList(in *projectapi.ProjectList, out *projectapiv1.ProjectList, s conversion.Scope) error {
	return autoConvert_api_ProjectList_To_v1_ProjectList(in, out, s)
}

func autoConvert_api_ProjectRequest_To_v1_ProjectRequest(in *projectapi.ProjectRequest, out *projectapiv1.ProjectRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectRequest))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func Convert_api_ProjectRequest_To_v1_ProjectRequest(in *projectapi.ProjectRequest, out *projectapiv1.ProjectRequest, s conversion.Scope) error {
	return autoConvert_api_ProjectRequest_To_v1_ProjectRequest(in, out, s)
}

func autoConvert_api_ProjectSpec_To_v1_ProjectSpec(in *projectapi.ProjectSpec, out *projectapiv1.ProjectSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectSpec))(in)
	}
	if in.Finalizers != nil {
		out.Finalizers = make([]apiv1.FinalizerName, len(in.Finalizers))
		for i := range in.Finalizers {
			out.Finalizers[i] = apiv1.FinalizerName(in.Finalizers[i])
		}
	} else {
		out.Finalizers = nil
	}
	return nil
}

func Convert_api_ProjectSpec_To_v1_ProjectSpec(in *projectapi.ProjectSpec, out *projectapiv1.ProjectSpec, s conversion.Scope) error {
	return autoConvert_api_ProjectSpec_To_v1_ProjectSpec(in, out, s)
}

func autoConvert_api_ProjectStatus_To_v1_ProjectStatus(in *projectapi.ProjectStatus, out *projectapiv1.ProjectStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectStatus))(in)
	}
	out.Phase = apiv1.NamespacePhase(in.Phase)
	return nil
}

func Convert_api_ProjectStatus_To_v1_ProjectStatus(in *projectapi.ProjectStatus, out *projectapiv1.ProjectStatus, s conversion.Scope) error {
	return autoConvert_api_ProjectStatus_To_v1_ProjectStatus(in, out, s)
}

func autoConvert_v1_Project_To_api_Project(in *projectapiv1.Project, out *projectapi.Project, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.Project))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_ProjectSpec_To_api_ProjectSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1_ProjectStatus_To_api_ProjectStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_Project_To_api_Project(in *projectapiv1.Project, out *projectapi.Project, s conversion.Scope) error {
	return autoConvert_v1_Project_To_api_Project(in, out, s)
}

func autoConvert_v1_ProjectList_To_api_ProjectList(in *projectapiv1.ProjectList, out *projectapi.ProjectList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.ProjectList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]projectapi.Project, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_Project_To_api_Project(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ProjectList_To_api_ProjectList(in *projectapiv1.ProjectList, out *projectapi.ProjectList, s conversion.Scope) error {
	return autoConvert_v1_ProjectList_To_api_ProjectList(in, out, s)
}

func autoConvert_v1_ProjectRequest_To_api_ProjectRequest(in *projectapiv1.ProjectRequest, out *projectapi.ProjectRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.ProjectRequest))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func Convert_v1_ProjectRequest_To_api_ProjectRequest(in *projectapiv1.ProjectRequest, out *projectapi.ProjectRequest, s conversion.Scope) error {
	return autoConvert_v1_ProjectRequest_To_api_ProjectRequest(in, out, s)
}

func autoConvert_v1_ProjectSpec_To_api_ProjectSpec(in *projectapiv1.ProjectSpec, out *projectapi.ProjectSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.ProjectSpec))(in)
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

func Convert_v1_ProjectSpec_To_api_ProjectSpec(in *projectapiv1.ProjectSpec, out *projectapi.ProjectSpec, s conversion.Scope) error {
	return autoConvert_v1_ProjectSpec_To_api_ProjectSpec(in, out, s)
}

func autoConvert_v1_ProjectStatus_To_api_ProjectStatus(in *projectapiv1.ProjectStatus, out *projectapi.ProjectStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.ProjectStatus))(in)
	}
	out.Phase = api.NamespacePhase(in.Phase)
	return nil
}

func Convert_v1_ProjectStatus_To_api_ProjectStatus(in *projectapiv1.ProjectStatus, out *projectapi.ProjectStatus, s conversion.Scope) error {
	return autoConvert_v1_ProjectStatus_To_api_ProjectStatus(in, out, s)
}

func autoConvert_api_Route_To_v1_Route(in *routeapi.Route, out *routeapiv1.Route, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.Route))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_RouteSpec_To_v1_RouteSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_api_RouteStatus_To_v1_RouteStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_Route_To_v1_Route(in *routeapi.Route, out *routeapiv1.Route, s conversion.Scope) error {
	return autoConvert_api_Route_To_v1_Route(in, out, s)
}

func autoConvert_api_RouteIngress_To_v1_RouteIngress(in *routeapi.RouteIngress, out *routeapiv1.RouteIngress, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteIngress))(in)
	}
	out.Host = in.Host
	out.RouterName = in.RouterName
	if in.Conditions != nil {
		out.Conditions = make([]routeapiv1.RouteIngressCondition, len(in.Conditions))
		for i := range in.Conditions {
			if err := Convert_api_RouteIngressCondition_To_v1_RouteIngressCondition(&in.Conditions[i], &out.Conditions[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Conditions = nil
	}
	return nil
}

func Convert_api_RouteIngress_To_v1_RouteIngress(in *routeapi.RouteIngress, out *routeapiv1.RouteIngress, s conversion.Scope) error {
	return autoConvert_api_RouteIngress_To_v1_RouteIngress(in, out, s)
}

func autoConvert_api_RouteIngressCondition_To_v1_RouteIngressCondition(in *routeapi.RouteIngressCondition, out *routeapiv1.RouteIngressCondition, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteIngressCondition))(in)
	}
	out.Type = routeapiv1.RouteIngressConditionType(in.Type)
	out.Status = apiv1.ConditionStatus(in.Status)
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

func Convert_api_RouteIngressCondition_To_v1_RouteIngressCondition(in *routeapi.RouteIngressCondition, out *routeapiv1.RouteIngressCondition, s conversion.Scope) error {
	return autoConvert_api_RouteIngressCondition_To_v1_RouteIngressCondition(in, out, s)
}

func autoConvert_api_RouteList_To_v1_RouteList(in *routeapi.RouteList, out *routeapiv1.RouteList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]routeapiv1.Route, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Route_To_v1_Route(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_RouteList_To_v1_RouteList(in *routeapi.RouteList, out *routeapiv1.RouteList, s conversion.Scope) error {
	return autoConvert_api_RouteList_To_v1_RouteList(in, out, s)
}

func autoConvert_api_RoutePort_To_v1_RoutePort(in *routeapi.RoutePort, out *routeapiv1.RoutePort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RoutePort))(in)
	}
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.TargetPort, &out.TargetPort, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_RoutePort_To_v1_RoutePort(in *routeapi.RoutePort, out *routeapiv1.RoutePort, s conversion.Scope) error {
	return autoConvert_api_RoutePort_To_v1_RoutePort(in, out, s)
}

func autoConvert_api_RouteSpec_To_v1_RouteSpec(in *routeapi.RouteSpec, out *routeapiv1.RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for api.RoutePort -> v1.RoutePort
	if in.Port != nil {
		out.Port = new(routeapiv1.RoutePort)
		if err := Convert_api_RoutePort_To_v1_RoutePort(in.Port, out.Port, s); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	// unable to generate simple pointer conversion for api.TLSConfig -> v1.TLSConfig
	if in.TLS != nil {
		out.TLS = new(routeapiv1.TLSConfig)
		if err := Convert_api_TLSConfig_To_v1_TLSConfig(in.TLS, out.TLS, s); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func Convert_api_RouteSpec_To_v1_RouteSpec(in *routeapi.RouteSpec, out *routeapiv1.RouteSpec, s conversion.Scope) error {
	return autoConvert_api_RouteSpec_To_v1_RouteSpec(in, out, s)
}

func autoConvert_api_RouteStatus_To_v1_RouteStatus(in *routeapi.RouteStatus, out *routeapiv1.RouteStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteStatus))(in)
	}
	if in.Ingress != nil {
		out.Ingress = make([]routeapiv1.RouteIngress, len(in.Ingress))
		for i := range in.Ingress {
			if err := Convert_api_RouteIngress_To_v1_RouteIngress(&in.Ingress[i], &out.Ingress[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ingress = nil
	}
	return nil
}

func Convert_api_RouteStatus_To_v1_RouteStatus(in *routeapi.RouteStatus, out *routeapiv1.RouteStatus, s conversion.Scope) error {
	return autoConvert_api_RouteStatus_To_v1_RouteStatus(in, out, s)
}

func autoConvert_api_TLSConfig_To_v1_TLSConfig(in *routeapi.TLSConfig, out *routeapiv1.TLSConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.TLSConfig))(in)
	}
	out.Termination = routeapiv1.TLSTerminationType(in.Termination)
	out.Certificate = in.Certificate
	out.Key = in.Key
	out.CACertificate = in.CACertificate
	out.DestinationCACertificate = in.DestinationCACertificate
	out.InsecureEdgeTerminationPolicy = routeapiv1.InsecureEdgeTerminationPolicyType(in.InsecureEdgeTerminationPolicy)
	return nil
}

func Convert_api_TLSConfig_To_v1_TLSConfig(in *routeapi.TLSConfig, out *routeapiv1.TLSConfig, s conversion.Scope) error {
	return autoConvert_api_TLSConfig_To_v1_TLSConfig(in, out, s)
}

func autoConvert_v1_Route_To_api_Route(in *routeapiv1.Route, out *routeapi.Route, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.Route))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_RouteSpec_To_api_RouteSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1_RouteStatus_To_api_RouteStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_Route_To_api_Route(in *routeapiv1.Route, out *routeapi.Route, s conversion.Scope) error {
	return autoConvert_v1_Route_To_api_Route(in, out, s)
}

func autoConvert_v1_RouteIngress_To_api_RouteIngress(in *routeapiv1.RouteIngress, out *routeapi.RouteIngress, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RouteIngress))(in)
	}
	out.Host = in.Host
	out.RouterName = in.RouterName
	if in.Conditions != nil {
		out.Conditions = make([]routeapi.RouteIngressCondition, len(in.Conditions))
		for i := range in.Conditions {
			if err := Convert_v1_RouteIngressCondition_To_api_RouteIngressCondition(&in.Conditions[i], &out.Conditions[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Conditions = nil
	}
	return nil
}

func Convert_v1_RouteIngress_To_api_RouteIngress(in *routeapiv1.RouteIngress, out *routeapi.RouteIngress, s conversion.Scope) error {
	return autoConvert_v1_RouteIngress_To_api_RouteIngress(in, out, s)
}

func autoConvert_v1_RouteIngressCondition_To_api_RouteIngressCondition(in *routeapiv1.RouteIngressCondition, out *routeapi.RouteIngressCondition, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RouteIngressCondition))(in)
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

func Convert_v1_RouteIngressCondition_To_api_RouteIngressCondition(in *routeapiv1.RouteIngressCondition, out *routeapi.RouteIngressCondition, s conversion.Scope) error {
	return autoConvert_v1_RouteIngressCondition_To_api_RouteIngressCondition(in, out, s)
}

func autoConvert_v1_RouteList_To_api_RouteList(in *routeapiv1.RouteList, out *routeapi.RouteList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RouteList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]routeapi.Route, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_Route_To_api_Route(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_RouteList_To_api_RouteList(in *routeapiv1.RouteList, out *routeapi.RouteList, s conversion.Scope) error {
	return autoConvert_v1_RouteList_To_api_RouteList(in, out, s)
}

func autoConvert_v1_RoutePort_To_api_RoutePort(in *routeapiv1.RoutePort, out *routeapi.RoutePort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RoutePort))(in)
	}
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.TargetPort, &out.TargetPort, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_RoutePort_To_api_RoutePort(in *routeapiv1.RoutePort, out *routeapi.RoutePort, s conversion.Scope) error {
	return autoConvert_v1_RoutePort_To_api_RoutePort(in, out, s)
}

func autoConvert_v1_RouteSpec_To_api_RouteSpec(in *routeapiv1.RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	// unable to generate simple pointer conversion for v1.RoutePort -> api.RoutePort
	if in.Port != nil {
		out.Port = new(routeapi.RoutePort)
		if err := Convert_v1_RoutePort_To_api_RoutePort(in.Port, out.Port, s); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	// unable to generate simple pointer conversion for v1.TLSConfig -> api.TLSConfig
	if in.TLS != nil {
		out.TLS = new(routeapi.TLSConfig)
		if err := Convert_v1_TLSConfig_To_api_TLSConfig(in.TLS, out.TLS, s); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func Convert_v1_RouteSpec_To_api_RouteSpec(in *routeapiv1.RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	return autoConvert_v1_RouteSpec_To_api_RouteSpec(in, out, s)
}

func autoConvert_v1_RouteStatus_To_api_RouteStatus(in *routeapiv1.RouteStatus, out *routeapi.RouteStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RouteStatus))(in)
	}
	if in.Ingress != nil {
		out.Ingress = make([]routeapi.RouteIngress, len(in.Ingress))
		for i := range in.Ingress {
			if err := Convert_v1_RouteIngress_To_api_RouteIngress(&in.Ingress[i], &out.Ingress[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ingress = nil
	}
	return nil
}

func Convert_v1_RouteStatus_To_api_RouteStatus(in *routeapiv1.RouteStatus, out *routeapi.RouteStatus, s conversion.Scope) error {
	return autoConvert_v1_RouteStatus_To_api_RouteStatus(in, out, s)
}

func autoConvert_v1_TLSConfig_To_api_TLSConfig(in *routeapiv1.TLSConfig, out *routeapi.TLSConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.TLSConfig))(in)
	}
	out.Termination = routeapi.TLSTerminationType(in.Termination)
	out.Certificate = in.Certificate
	out.Key = in.Key
	out.CACertificate = in.CACertificate
	out.DestinationCACertificate = in.DestinationCACertificate
	out.InsecureEdgeTerminationPolicy = routeapi.InsecureEdgeTerminationPolicyType(in.InsecureEdgeTerminationPolicy)
	return nil
}

func Convert_v1_TLSConfig_To_api_TLSConfig(in *routeapiv1.TLSConfig, out *routeapi.TLSConfig, s conversion.Scope) error {
	return autoConvert_v1_TLSConfig_To_api_TLSConfig(in, out, s)
}

func autoConvert_api_ClusterNetwork_To_v1_ClusterNetwork(in *sdnapi.ClusterNetwork, out *sdnapiv1.ClusterNetwork, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.ClusterNetwork))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func Convert_api_ClusterNetwork_To_v1_ClusterNetwork(in *sdnapi.ClusterNetwork, out *sdnapiv1.ClusterNetwork, s conversion.Scope) error {
	return autoConvert_api_ClusterNetwork_To_v1_ClusterNetwork(in, out, s)
}

func autoConvert_api_ClusterNetworkList_To_v1_ClusterNetworkList(in *sdnapi.ClusterNetworkList, out *sdnapiv1.ClusterNetworkList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.ClusterNetworkList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_ClusterNetwork_To_v1_ClusterNetwork(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ClusterNetworkList_To_v1_ClusterNetworkList(in *sdnapi.ClusterNetworkList, out *sdnapiv1.ClusterNetworkList, s conversion.Scope) error {
	return autoConvert_api_ClusterNetworkList_To_v1_ClusterNetworkList(in, out, s)
}

func autoConvert_api_HostSubnet_To_v1_HostSubnet(in *sdnapi.HostSubnet, out *sdnapiv1.HostSubnet, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.HostSubnet))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func Convert_api_HostSubnet_To_v1_HostSubnet(in *sdnapi.HostSubnet, out *sdnapiv1.HostSubnet, s conversion.Scope) error {
	return autoConvert_api_HostSubnet_To_v1_HostSubnet(in, out, s)
}

func autoConvert_api_HostSubnetList_To_v1_HostSubnetList(in *sdnapi.HostSubnetList, out *sdnapiv1.HostSubnetList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.HostSubnetList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_HostSubnet_To_v1_HostSubnet(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_HostSubnetList_To_v1_HostSubnetList(in *sdnapi.HostSubnetList, out *sdnapiv1.HostSubnetList, s conversion.Scope) error {
	return autoConvert_api_HostSubnetList_To_v1_HostSubnetList(in, out, s)
}

func autoConvert_api_NetNamespace_To_v1_NetNamespace(in *sdnapi.NetNamespace, out *sdnapiv1.NetNamespace, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.NetNamespace))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func Convert_api_NetNamespace_To_v1_NetNamespace(in *sdnapi.NetNamespace, out *sdnapiv1.NetNamespace, s conversion.Scope) error {
	return autoConvert_api_NetNamespace_To_v1_NetNamespace(in, out, s)
}

func autoConvert_api_NetNamespaceList_To_v1_NetNamespaceList(in *sdnapi.NetNamespaceList, out *sdnapiv1.NetNamespaceList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.NetNamespaceList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1.NetNamespace, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_NetNamespace_To_v1_NetNamespace(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_NetNamespaceList_To_v1_NetNamespaceList(in *sdnapi.NetNamespaceList, out *sdnapiv1.NetNamespaceList, s conversion.Scope) error {
	return autoConvert_api_NetNamespaceList_To_v1_NetNamespaceList(in, out, s)
}

func autoConvert_v1_ClusterNetwork_To_api_ClusterNetwork(in *sdnapiv1.ClusterNetwork, out *sdnapi.ClusterNetwork, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.ClusterNetwork))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func Convert_v1_ClusterNetwork_To_api_ClusterNetwork(in *sdnapiv1.ClusterNetwork, out *sdnapi.ClusterNetwork, s conversion.Scope) error {
	return autoConvert_v1_ClusterNetwork_To_api_ClusterNetwork(in, out, s)
}

func autoConvert_v1_ClusterNetworkList_To_api_ClusterNetworkList(in *sdnapiv1.ClusterNetworkList, out *sdnapi.ClusterNetworkList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.ClusterNetworkList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_ClusterNetwork_To_api_ClusterNetwork(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ClusterNetworkList_To_api_ClusterNetworkList(in *sdnapiv1.ClusterNetworkList, out *sdnapi.ClusterNetworkList, s conversion.Scope) error {
	return autoConvert_v1_ClusterNetworkList_To_api_ClusterNetworkList(in, out, s)
}

func autoConvert_v1_HostSubnet_To_api_HostSubnet(in *sdnapiv1.HostSubnet, out *sdnapi.HostSubnet, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.HostSubnet))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func Convert_v1_HostSubnet_To_api_HostSubnet(in *sdnapiv1.HostSubnet, out *sdnapi.HostSubnet, s conversion.Scope) error {
	return autoConvert_v1_HostSubnet_To_api_HostSubnet(in, out, s)
}

func autoConvert_v1_HostSubnetList_To_api_HostSubnetList(in *sdnapiv1.HostSubnetList, out *sdnapi.HostSubnetList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.HostSubnetList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_HostSubnet_To_api_HostSubnet(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_HostSubnetList_To_api_HostSubnetList(in *sdnapiv1.HostSubnetList, out *sdnapi.HostSubnetList, s conversion.Scope) error {
	return autoConvert_v1_HostSubnetList_To_api_HostSubnetList(in, out, s)
}

func autoConvert_v1_NetNamespace_To_api_NetNamespace(in *sdnapiv1.NetNamespace, out *sdnapi.NetNamespace, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.NetNamespace))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func Convert_v1_NetNamespace_To_api_NetNamespace(in *sdnapiv1.NetNamespace, out *sdnapi.NetNamespace, s conversion.Scope) error {
	return autoConvert_v1_NetNamespace_To_api_NetNamespace(in, out, s)
}

func autoConvert_v1_NetNamespaceList_To_api_NetNamespaceList(in *sdnapiv1.NetNamespaceList, out *sdnapi.NetNamespaceList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.NetNamespaceList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.NetNamespace, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_NetNamespace_To_api_NetNamespace(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_NetNamespaceList_To_api_NetNamespaceList(in *sdnapiv1.NetNamespaceList, out *sdnapi.NetNamespaceList, s conversion.Scope) error {
	return autoConvert_v1_NetNamespaceList_To_api_NetNamespaceList(in, out, s)
}

func autoConvert_api_Parameter_To_v1_Parameter(in *templateapi.Parameter, out *templateapiv1.Parameter, s conversion.Scope) error {
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

func Convert_api_Parameter_To_v1_Parameter(in *templateapi.Parameter, out *templateapiv1.Parameter, s conversion.Scope) error {
	return autoConvert_api_Parameter_To_v1_Parameter(in, out, s)
}

func autoConvert_api_Template_To_v1_Template(in *templateapi.Template, out *templateapiv1.Template, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapi.Template))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Parameters != nil {
		out.Parameters = make([]templateapiv1.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := Convert_api_Parameter_To_v1_Parameter(&in.Parameters[i], &out.Parameters[i], s); err != nil {
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

func autoConvert_api_TemplateList_To_v1_TemplateList(in *templateapi.TemplateList, out *templateapiv1.TemplateList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapi.TemplateList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]templateapiv1.Template, len(in.Items))
		for i := range in.Items {
			if err := templateapiv1.Convert_api_Template_To_v1_Template(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_TemplateList_To_v1_TemplateList(in *templateapi.TemplateList, out *templateapiv1.TemplateList, s conversion.Scope) error {
	return autoConvert_api_TemplateList_To_v1_TemplateList(in, out, s)
}

func autoConvert_v1_Parameter_To_api_Parameter(in *templateapiv1.Parameter, out *templateapi.Parameter, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1.Parameter))(in)
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

func Convert_v1_Parameter_To_api_Parameter(in *templateapiv1.Parameter, out *templateapi.Parameter, s conversion.Scope) error {
	return autoConvert_v1_Parameter_To_api_Parameter(in, out, s)
}

func autoConvert_v1_Template_To_api_Template(in *templateapiv1.Template, out *templateapi.Template, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1.Template))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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
			if err := Convert_v1_Parameter_To_api_Parameter(&in.Parameters[i], &out.Parameters[i], s); err != nil {
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

func autoConvert_v1_TemplateList_To_api_TemplateList(in *templateapiv1.TemplateList, out *templateapi.TemplateList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1.TemplateList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]templateapi.Template, len(in.Items))
		for i := range in.Items {
			if err := templateapiv1.Convert_v1_Template_To_api_Template(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_TemplateList_To_api_TemplateList(in *templateapiv1.TemplateList, out *templateapi.TemplateList, s conversion.Scope) error {
	return autoConvert_v1_TemplateList_To_api_TemplateList(in, out, s)
}

func autoConvert_api_Group_To_v1_Group(in *userapi.Group, out *userapiv1.Group, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.Group))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_api_Group_To_v1_Group(in *userapi.Group, out *userapiv1.Group, s conversion.Scope) error {
	return autoConvert_api_Group_To_v1_Group(in, out, s)
}

func autoConvert_api_GroupList_To_v1_GroupList(in *userapi.GroupList, out *userapiv1.GroupList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.GroupList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapiv1.Group, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Group_To_v1_Group(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_GroupList_To_v1_GroupList(in *userapi.GroupList, out *userapiv1.GroupList, s conversion.Scope) error {
	return autoConvert_api_GroupList_To_v1_GroupList(in, out, s)
}

func autoConvert_api_Identity_To_v1_Identity(in *userapi.Identity, out *userapiv1.Identity, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.Identity))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.User, &out.User, s); err != nil {
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

func Convert_api_Identity_To_v1_Identity(in *userapi.Identity, out *userapiv1.Identity, s conversion.Scope) error {
	return autoConvert_api_Identity_To_v1_Identity(in, out, s)
}

func autoConvert_api_IdentityList_To_v1_IdentityList(in *userapi.IdentityList, out *userapiv1.IdentityList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.IdentityList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapiv1.Identity, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_Identity_To_v1_Identity(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_IdentityList_To_v1_IdentityList(in *userapi.IdentityList, out *userapiv1.IdentityList, s conversion.Scope) error {
	return autoConvert_api_IdentityList_To_v1_IdentityList(in, out, s)
}

func autoConvert_api_User_To_v1_User(in *userapi.User, out *userapiv1.User, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.User))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_api_User_To_v1_User(in *userapi.User, out *userapiv1.User, s conversion.Scope) error {
	return autoConvert_api_User_To_v1_User(in, out, s)
}

func autoConvert_api_UserIdentityMapping_To_v1_UserIdentityMapping(in *userapi.UserIdentityMapping, out *userapiv1.UserIdentityMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.UserIdentityMapping))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := Convert_api_ObjectReference_To_v1_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_UserIdentityMapping_To_v1_UserIdentityMapping(in *userapi.UserIdentityMapping, out *userapiv1.UserIdentityMapping, s conversion.Scope) error {
	return autoConvert_api_UserIdentityMapping_To_v1_UserIdentityMapping(in, out, s)
}

func autoConvert_api_UserList_To_v1_UserList(in *userapi.UserList, out *userapiv1.UserList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.UserList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapiv1.User, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_User_To_v1_User(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_UserList_To_v1_UserList(in *userapi.UserList, out *userapiv1.UserList, s conversion.Scope) error {
	return autoConvert_api_UserList_To_v1_UserList(in, out, s)
}

func autoConvert_v1_Group_To_api_Group(in *userapiv1.Group, out *userapi.Group, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.Group))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_v1_Group_To_api_Group(in *userapiv1.Group, out *userapi.Group, s conversion.Scope) error {
	return autoConvert_v1_Group_To_api_Group(in, out, s)
}

func autoConvert_v1_GroupList_To_api_GroupList(in *userapiv1.GroupList, out *userapi.GroupList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.GroupList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapi.Group, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_Group_To_api_Group(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_GroupList_To_api_GroupList(in *userapiv1.GroupList, out *userapi.GroupList, s conversion.Scope) error {
	return autoConvert_v1_GroupList_To_api_GroupList(in, out, s)
}

func autoConvert_v1_Identity_To_api_Identity(in *userapiv1.Identity, out *userapi.Identity, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.Identity))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
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

func Convert_v1_Identity_To_api_Identity(in *userapiv1.Identity, out *userapi.Identity, s conversion.Scope) error {
	return autoConvert_v1_Identity_To_api_Identity(in, out, s)
}

func autoConvert_v1_IdentityList_To_api_IdentityList(in *userapiv1.IdentityList, out *userapi.IdentityList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.IdentityList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapi.Identity, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_Identity_To_api_Identity(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_IdentityList_To_api_IdentityList(in *userapiv1.IdentityList, out *userapi.IdentityList, s conversion.Scope) error {
	return autoConvert_v1_IdentityList_To_api_IdentityList(in, out, s)
}

func autoConvert_v1_User_To_api_User(in *userapiv1.User, out *userapi.User, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.User))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func Convert_v1_User_To_api_User(in *userapiv1.User, out *userapi.User, s conversion.Scope) error {
	return autoConvert_v1_User_To_api_User(in, out, s)
}

func autoConvert_v1_UserIdentityMapping_To_api_UserIdentityMapping(in *userapiv1.UserIdentityMapping, out *userapi.UserIdentityMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.UserIdentityMapping))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := Convert_v1_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_UserIdentityMapping_To_api_UserIdentityMapping(in *userapiv1.UserIdentityMapping, out *userapi.UserIdentityMapping, s conversion.Scope) error {
	return autoConvert_v1_UserIdentityMapping_To_api_UserIdentityMapping(in, out, s)
}

func autoConvert_v1_UserList_To_api_UserList(in *userapiv1.UserList, out *userapi.UserList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.UserList))(in)
	}
	if err := api.Convert_unversioned_ListMeta_To_unversioned_ListMeta(&in.ListMeta, &out.ListMeta, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapi.User, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_User_To_api_User(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_UserList_To_api_UserList(in *userapiv1.UserList, out *userapi.UserList, s conversion.Scope) error {
	return autoConvert_v1_UserList_To_api_UserList(in, out, s)
}

func autoConvert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource(in *api.AWSElasticBlockStoreVolumeSource, out *apiv1.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.AWSElasticBlockStoreVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.Partition = int32(in.Partition)
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource(in *api.AWSElasticBlockStoreVolumeSource, out *apiv1.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	return autoConvert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource(in, out, s)
}

func autoConvert_api_AzureFileVolumeSource_To_v1_AzureFileVolumeSource(in *api.AzureFileVolumeSource, out *apiv1.AzureFileVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.AzureFileVolumeSource))(in)
	}
	out.SecretName = in.SecretName
	out.ShareName = in.ShareName
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_AzureFileVolumeSource_To_v1_AzureFileVolumeSource(in *api.AzureFileVolumeSource, out *apiv1.AzureFileVolumeSource, s conversion.Scope) error {
	return autoConvert_api_AzureFileVolumeSource_To_v1_AzureFileVolumeSource(in, out, s)
}

func autoConvert_api_Capabilities_To_v1_Capabilities(in *api.Capabilities, out *apiv1.Capabilities, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Capabilities))(in)
	}
	if in.Add != nil {
		out.Add = make([]apiv1.Capability, len(in.Add))
		for i := range in.Add {
			out.Add[i] = apiv1.Capability(in.Add[i])
		}
	} else {
		out.Add = nil
	}
	if in.Drop != nil {
		out.Drop = make([]apiv1.Capability, len(in.Drop))
		for i := range in.Drop {
			out.Drop[i] = apiv1.Capability(in.Drop[i])
		}
	} else {
		out.Drop = nil
	}
	return nil
}

func Convert_api_Capabilities_To_v1_Capabilities(in *api.Capabilities, out *apiv1.Capabilities, s conversion.Scope) error {
	return autoConvert_api_Capabilities_To_v1_Capabilities(in, out, s)
}

func autoConvert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource(in *api.CephFSVolumeSource, out *apiv1.CephFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.CephFSVolumeSource))(in)
	}
	if in.Monitors != nil {
		out.Monitors = make([]string, len(in.Monitors))
		for i := range in.Monitors {
			out.Monitors[i] = in.Monitors[i]
		}
	} else {
		out.Monitors = nil
	}
	out.Path = in.Path
	out.User = in.User
	out.SecretFile = in.SecretFile
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.SecretRef != nil {
		out.SecretRef = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource(in *api.CephFSVolumeSource, out *apiv1.CephFSVolumeSource, s conversion.Scope) error {
	return autoConvert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource(in, out, s)
}

func autoConvert_api_CinderVolumeSource_To_v1_CinderVolumeSource(in *api.CinderVolumeSource, out *apiv1.CinderVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.CinderVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_CinderVolumeSource_To_v1_CinderVolumeSource(in *api.CinderVolumeSource, out *apiv1.CinderVolumeSource, s conversion.Scope) error {
	return autoConvert_api_CinderVolumeSource_To_v1_CinderVolumeSource(in, out, s)
}

func autoConvert_api_ConfigMapKeySelector_To_v1_ConfigMapKeySelector(in *api.ConfigMapKeySelector, out *apiv1.ConfigMapKeySelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ConfigMapKeySelector))(in)
	}
	if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(&in.LocalObjectReference, &out.LocalObjectReference, s); err != nil {
		return err
	}
	out.Key = in.Key
	return nil
}

func Convert_api_ConfigMapKeySelector_To_v1_ConfigMapKeySelector(in *api.ConfigMapKeySelector, out *apiv1.ConfigMapKeySelector, s conversion.Scope) error {
	return autoConvert_api_ConfigMapKeySelector_To_v1_ConfigMapKeySelector(in, out, s)
}

func autoConvert_api_ConfigMapVolumeSource_To_v1_ConfigMapVolumeSource(in *api.ConfigMapVolumeSource, out *apiv1.ConfigMapVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ConfigMapVolumeSource))(in)
	}
	if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(&in.LocalObjectReference, &out.LocalObjectReference, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]apiv1.KeyToPath, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_KeyToPath_To_v1_KeyToPath(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_ConfigMapVolumeSource_To_v1_ConfigMapVolumeSource(in *api.ConfigMapVolumeSource, out *apiv1.ConfigMapVolumeSource, s conversion.Scope) error {
	return autoConvert_api_ConfigMapVolumeSource_To_v1_ConfigMapVolumeSource(in, out, s)
}

func autoConvert_api_Container_To_v1_Container(in *api.Container, out *apiv1.Container, s conversion.Scope) error {
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
		out.Ports = make([]apiv1.ContainerPort, len(in.Ports))
		for i := range in.Ports {
			if err := Convert_api_ContainerPort_To_v1_ContainerPort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ports = nil
	}
	if in.Env != nil {
		out.Env = make([]apiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	if err := Convert_api_ResourceRequirements_To_v1_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if in.VolumeMounts != nil {
		out.VolumeMounts = make([]apiv1.VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			if err := Convert_api_VolumeMount_To_v1_VolumeMount(&in.VolumeMounts[i], &out.VolumeMounts[i], s); err != nil {
				return err
			}
		}
	} else {
		out.VolumeMounts = nil
	}
	// unable to generate simple pointer conversion for api.Probe -> v1.Probe
	if in.LivenessProbe != nil {
		out.LivenessProbe = new(apiv1.Probe)
		if err := Convert_api_Probe_To_v1_Probe(in.LivenessProbe, out.LivenessProbe, s); err != nil {
			return err
		}
	} else {
		out.LivenessProbe = nil
	}
	// unable to generate simple pointer conversion for api.Probe -> v1.Probe
	if in.ReadinessProbe != nil {
		out.ReadinessProbe = new(apiv1.Probe)
		if err := Convert_api_Probe_To_v1_Probe(in.ReadinessProbe, out.ReadinessProbe, s); err != nil {
			return err
		}
	} else {
		out.ReadinessProbe = nil
	}
	// unable to generate simple pointer conversion for api.Lifecycle -> v1.Lifecycle
	if in.Lifecycle != nil {
		out.Lifecycle = new(apiv1.Lifecycle)
		if err := Convert_api_Lifecycle_To_v1_Lifecycle(in.Lifecycle, out.Lifecycle, s); err != nil {
			return err
		}
	} else {
		out.Lifecycle = nil
	}
	out.TerminationMessagePath = in.TerminationMessagePath
	out.ImagePullPolicy = apiv1.PullPolicy(in.ImagePullPolicy)
	// unable to generate simple pointer conversion for api.SecurityContext -> v1.SecurityContext
	if in.SecurityContext != nil {
		out.SecurityContext = new(apiv1.SecurityContext)
		if err := Convert_api_SecurityContext_To_v1_SecurityContext(in.SecurityContext, out.SecurityContext, s); err != nil {
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

func Convert_api_Container_To_v1_Container(in *api.Container, out *apiv1.Container, s conversion.Scope) error {
	return autoConvert_api_Container_To_v1_Container(in, out, s)
}

func autoConvert_api_ContainerPort_To_v1_ContainerPort(in *api.ContainerPort, out *apiv1.ContainerPort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ContainerPort))(in)
	}
	out.Name = in.Name
	out.HostPort = int32(in.HostPort)
	out.ContainerPort = int32(in.ContainerPort)
	out.Protocol = apiv1.Protocol(in.Protocol)
	out.HostIP = in.HostIP
	return nil
}

func Convert_api_ContainerPort_To_v1_ContainerPort(in *api.ContainerPort, out *apiv1.ContainerPort, s conversion.Scope) error {
	return autoConvert_api_ContainerPort_To_v1_ContainerPort(in, out, s)
}

func autoConvert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile(in *api.DownwardAPIVolumeFile, out *apiv1.DownwardAPIVolumeFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.DownwardAPIVolumeFile))(in)
	}
	out.Path = in.Path
	if err := Convert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile(in *api.DownwardAPIVolumeFile, out *apiv1.DownwardAPIVolumeFile, s conversion.Scope) error {
	return autoConvert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile(in, out, s)
}

func autoConvert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource(in *api.DownwardAPIVolumeSource, out *apiv1.DownwardAPIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.DownwardAPIVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]apiv1.DownwardAPIVolumeFile, len(in.Items))
		for i := range in.Items {
			if err := Convert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource(in *api.DownwardAPIVolumeSource, out *apiv1.DownwardAPIVolumeSource, s conversion.Scope) error {
	return autoConvert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource(in, out, s)
}

func autoConvert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource(in *api.EmptyDirVolumeSource, out *apiv1.EmptyDirVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.EmptyDirVolumeSource))(in)
	}
	out.Medium = apiv1.StorageMedium(in.Medium)
	return nil
}

func Convert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource(in *api.EmptyDirVolumeSource, out *apiv1.EmptyDirVolumeSource, s conversion.Scope) error {
	return autoConvert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource(in, out, s)
}

func autoConvert_api_EnvVar_To_v1_EnvVar(in *api.EnvVar, out *apiv1.EnvVar, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.EnvVar))(in)
	}
	out.Name = in.Name
	out.Value = in.Value
	// unable to generate simple pointer conversion for api.EnvVarSource -> v1.EnvVarSource
	if in.ValueFrom != nil {
		out.ValueFrom = new(apiv1.EnvVarSource)
		if err := Convert_api_EnvVarSource_To_v1_EnvVarSource(in.ValueFrom, out.ValueFrom, s); err != nil {
			return err
		}
	} else {
		out.ValueFrom = nil
	}
	return nil
}

func Convert_api_EnvVar_To_v1_EnvVar(in *api.EnvVar, out *apiv1.EnvVar, s conversion.Scope) error {
	return autoConvert_api_EnvVar_To_v1_EnvVar(in, out, s)
}

func autoConvert_api_EnvVarSource_To_v1_EnvVarSource(in *api.EnvVarSource, out *apiv1.EnvVarSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.EnvVarSource))(in)
	}
	// unable to generate simple pointer conversion for api.ObjectFieldSelector -> v1.ObjectFieldSelector
	if in.FieldRef != nil {
		out.FieldRef = new(apiv1.ObjectFieldSelector)
		if err := Convert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(in.FieldRef, out.FieldRef, s); err != nil {
			return err
		}
	} else {
		out.FieldRef = nil
	}
	// unable to generate simple pointer conversion for api.ConfigMapKeySelector -> v1.ConfigMapKeySelector
	if in.ConfigMapKeyRef != nil {
		out.ConfigMapKeyRef = new(apiv1.ConfigMapKeySelector)
		if err := Convert_api_ConfigMapKeySelector_To_v1_ConfigMapKeySelector(in.ConfigMapKeyRef, out.ConfigMapKeyRef, s); err != nil {
			return err
		}
	} else {
		out.ConfigMapKeyRef = nil
	}
	// unable to generate simple pointer conversion for api.SecretKeySelector -> v1.SecretKeySelector
	if in.SecretKeyRef != nil {
		out.SecretKeyRef = new(apiv1.SecretKeySelector)
		if err := Convert_api_SecretKeySelector_To_v1_SecretKeySelector(in.SecretKeyRef, out.SecretKeyRef, s); err != nil {
			return err
		}
	} else {
		out.SecretKeyRef = nil
	}
	return nil
}

func Convert_api_EnvVarSource_To_v1_EnvVarSource(in *api.EnvVarSource, out *apiv1.EnvVarSource, s conversion.Scope) error {
	return autoConvert_api_EnvVarSource_To_v1_EnvVarSource(in, out, s)
}

func autoConvert_api_ExecAction_To_v1_ExecAction(in *api.ExecAction, out *apiv1.ExecAction, s conversion.Scope) error {
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

func Convert_api_ExecAction_To_v1_ExecAction(in *api.ExecAction, out *apiv1.ExecAction, s conversion.Scope) error {
	return autoConvert_api_ExecAction_To_v1_ExecAction(in, out, s)
}

func autoConvert_api_FCVolumeSource_To_v1_FCVolumeSource(in *api.FCVolumeSource, out *apiv1.FCVolumeSource, s conversion.Scope) error {
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
		out.Lun = new(int32)
		*out.Lun = int32(*in.Lun)
	} else {
		out.Lun = nil
	}
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_FCVolumeSource_To_v1_FCVolumeSource(in *api.FCVolumeSource, out *apiv1.FCVolumeSource, s conversion.Scope) error {
	return autoConvert_api_FCVolumeSource_To_v1_FCVolumeSource(in, out, s)
}

func autoConvert_api_FlexVolumeSource_To_v1_FlexVolumeSource(in *api.FlexVolumeSource, out *apiv1.FlexVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.FlexVolumeSource))(in)
	}
	out.Driver = in.Driver
	out.FSType = in.FSType
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.SecretRef != nil {
		out.SecretRef = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	if in.Options != nil {
		out.Options = make(map[string]string)
		for key, val := range in.Options {
			out.Options[key] = val
		}
	} else {
		out.Options = nil
	}
	return nil
}

func Convert_api_FlexVolumeSource_To_v1_FlexVolumeSource(in *api.FlexVolumeSource, out *apiv1.FlexVolumeSource, s conversion.Scope) error {
	return autoConvert_api_FlexVolumeSource_To_v1_FlexVolumeSource(in, out, s)
}

func autoConvert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource(in *api.FlockerVolumeSource, out *apiv1.FlockerVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.FlockerVolumeSource))(in)
	}
	out.DatasetName = in.DatasetName
	return nil
}

func Convert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource(in *api.FlockerVolumeSource, out *apiv1.FlockerVolumeSource, s conversion.Scope) error {
	return autoConvert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource(in, out, s)
}

func autoConvert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource(in *api.GCEPersistentDiskVolumeSource, out *apiv1.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.GCEPersistentDiskVolumeSource))(in)
	}
	out.PDName = in.PDName
	out.FSType = in.FSType
	out.Partition = int32(in.Partition)
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource(in *api.GCEPersistentDiskVolumeSource, out *apiv1.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	return autoConvert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource(in, out, s)
}

func autoConvert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource(in *api.GitRepoVolumeSource, out *apiv1.GitRepoVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.GitRepoVolumeSource))(in)
	}
	out.Repository = in.Repository
	out.Revision = in.Revision
	out.Directory = in.Directory
	return nil
}

func Convert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource(in *api.GitRepoVolumeSource, out *apiv1.GitRepoVolumeSource, s conversion.Scope) error {
	return autoConvert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource(in, out, s)
}

func autoConvert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource(in *api.GlusterfsVolumeSource, out *apiv1.GlusterfsVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.GlusterfsVolumeSource))(in)
	}
	out.EndpointsName = in.EndpointsName
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource(in *api.GlusterfsVolumeSource, out *apiv1.GlusterfsVolumeSource, s conversion.Scope) error {
	return autoConvert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource(in, out, s)
}

func autoConvert_api_HTTPGetAction_To_v1_HTTPGetAction(in *api.HTTPGetAction, out *apiv1.HTTPGetAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.HTTPGetAction))(in)
	}
	out.Path = in.Path
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.Port, &out.Port, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.Scheme = apiv1.URIScheme(in.Scheme)
	if in.HTTPHeaders != nil {
		out.HTTPHeaders = make([]apiv1.HTTPHeader, len(in.HTTPHeaders))
		for i := range in.HTTPHeaders {
			if err := Convert_api_HTTPHeader_To_v1_HTTPHeader(&in.HTTPHeaders[i], &out.HTTPHeaders[i], s); err != nil {
				return err
			}
		}
	} else {
		out.HTTPHeaders = nil
	}
	return nil
}

func Convert_api_HTTPGetAction_To_v1_HTTPGetAction(in *api.HTTPGetAction, out *apiv1.HTTPGetAction, s conversion.Scope) error {
	return autoConvert_api_HTTPGetAction_To_v1_HTTPGetAction(in, out, s)
}

func autoConvert_api_HTTPHeader_To_v1_HTTPHeader(in *api.HTTPHeader, out *apiv1.HTTPHeader, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.HTTPHeader))(in)
	}
	out.Name = in.Name
	out.Value = in.Value
	return nil
}

func Convert_api_HTTPHeader_To_v1_HTTPHeader(in *api.HTTPHeader, out *apiv1.HTTPHeader, s conversion.Scope) error {
	return autoConvert_api_HTTPHeader_To_v1_HTTPHeader(in, out, s)
}

func autoConvert_api_Handler_To_v1_Handler(in *api.Handler, out *apiv1.Handler, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Handler))(in)
	}
	// unable to generate simple pointer conversion for api.ExecAction -> v1.ExecAction
	if in.Exec != nil {
		out.Exec = new(apiv1.ExecAction)
		if err := Convert_api_ExecAction_To_v1_ExecAction(in.Exec, out.Exec, s); err != nil {
			return err
		}
	} else {
		out.Exec = nil
	}
	// unable to generate simple pointer conversion for api.HTTPGetAction -> v1.HTTPGetAction
	if in.HTTPGet != nil {
		out.HTTPGet = new(apiv1.HTTPGetAction)
		if err := Convert_api_HTTPGetAction_To_v1_HTTPGetAction(in.HTTPGet, out.HTTPGet, s); err != nil {
			return err
		}
	} else {
		out.HTTPGet = nil
	}
	// unable to generate simple pointer conversion for api.TCPSocketAction -> v1.TCPSocketAction
	if in.TCPSocket != nil {
		out.TCPSocket = new(apiv1.TCPSocketAction)
		if err := Convert_api_TCPSocketAction_To_v1_TCPSocketAction(in.TCPSocket, out.TCPSocket, s); err != nil {
			return err
		}
	} else {
		out.TCPSocket = nil
	}
	return nil
}

func Convert_api_Handler_To_v1_Handler(in *api.Handler, out *apiv1.Handler, s conversion.Scope) error {
	return autoConvert_api_Handler_To_v1_Handler(in, out, s)
}

func autoConvert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource(in *api.HostPathVolumeSource, out *apiv1.HostPathVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.HostPathVolumeSource))(in)
	}
	out.Path = in.Path
	return nil
}

func Convert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource(in *api.HostPathVolumeSource, out *apiv1.HostPathVolumeSource, s conversion.Scope) error {
	return autoConvert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource(in, out, s)
}

func autoConvert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource(in *api.ISCSIVolumeSource, out *apiv1.ISCSIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ISCSIVolumeSource))(in)
	}
	out.TargetPortal = in.TargetPortal
	out.IQN = in.IQN
	out.Lun = int32(in.Lun)
	out.ISCSIInterface = in.ISCSIInterface
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource(in *api.ISCSIVolumeSource, out *apiv1.ISCSIVolumeSource, s conversion.Scope) error {
	return autoConvert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource(in, out, s)
}

func autoConvert_api_KeyToPath_To_v1_KeyToPath(in *api.KeyToPath, out *apiv1.KeyToPath, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.KeyToPath))(in)
	}
	out.Key = in.Key
	out.Path = in.Path
	return nil
}

func Convert_api_KeyToPath_To_v1_KeyToPath(in *api.KeyToPath, out *apiv1.KeyToPath, s conversion.Scope) error {
	return autoConvert_api_KeyToPath_To_v1_KeyToPath(in, out, s)
}

func autoConvert_api_Lifecycle_To_v1_Lifecycle(in *api.Lifecycle, out *apiv1.Lifecycle, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Lifecycle))(in)
	}
	// unable to generate simple pointer conversion for api.Handler -> v1.Handler
	if in.PostStart != nil {
		out.PostStart = new(apiv1.Handler)
		if err := Convert_api_Handler_To_v1_Handler(in.PostStart, out.PostStart, s); err != nil {
			return err
		}
	} else {
		out.PostStart = nil
	}
	// unable to generate simple pointer conversion for api.Handler -> v1.Handler
	if in.PreStop != nil {
		out.PreStop = new(apiv1.Handler)
		if err := Convert_api_Handler_To_v1_Handler(in.PreStop, out.PreStop, s); err != nil {
			return err
		}
	} else {
		out.PreStop = nil
	}
	return nil
}

func Convert_api_Lifecycle_To_v1_Lifecycle(in *api.Lifecycle, out *apiv1.Lifecycle, s conversion.Scope) error {
	return autoConvert_api_Lifecycle_To_v1_Lifecycle(in, out, s)
}

func autoConvert_api_LocalObjectReference_To_v1_LocalObjectReference(in *api.LocalObjectReference, out *apiv1.LocalObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.LocalObjectReference))(in)
	}
	out.Name = in.Name
	return nil
}

func Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in *api.LocalObjectReference, out *apiv1.LocalObjectReference, s conversion.Scope) error {
	return autoConvert_api_LocalObjectReference_To_v1_LocalObjectReference(in, out, s)
}

func autoConvert_api_NFSVolumeSource_To_v1_NFSVolumeSource(in *api.NFSVolumeSource, out *apiv1.NFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.NFSVolumeSource))(in)
	}
	out.Server = in.Server
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_NFSVolumeSource_To_v1_NFSVolumeSource(in *api.NFSVolumeSource, out *apiv1.NFSVolumeSource, s conversion.Scope) error {
	return autoConvert_api_NFSVolumeSource_To_v1_NFSVolumeSource(in, out, s)
}

func autoConvert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(in *api.ObjectFieldSelector, out *apiv1.ObjectFieldSelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ObjectFieldSelector))(in)
	}
	out.APIVersion = in.APIVersion
	out.FieldPath = in.FieldPath
	return nil
}

func Convert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(in *api.ObjectFieldSelector, out *apiv1.ObjectFieldSelector, s conversion.Scope) error {
	return autoConvert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(in, out, s)
}

func autoConvert_api_ObjectMeta_To_v1_ObjectMeta(in *api.ObjectMeta, out *apiv1.ObjectMeta, s conversion.Scope) error {
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

func Convert_api_ObjectMeta_To_v1_ObjectMeta(in *api.ObjectMeta, out *apiv1.ObjectMeta, s conversion.Scope) error {
	return autoConvert_api_ObjectMeta_To_v1_ObjectMeta(in, out, s)
}

func autoConvert_api_ObjectReference_To_v1_ObjectReference(in *api.ObjectReference, out *apiv1.ObjectReference, s conversion.Scope) error {
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

func Convert_api_ObjectReference_To_v1_ObjectReference(in *api.ObjectReference, out *apiv1.ObjectReference, s conversion.Scope) error {
	return autoConvert_api_ObjectReference_To_v1_ObjectReference(in, out, s)
}

func autoConvert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource(in *api.PersistentVolumeClaimVolumeSource, out *apiv1.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PersistentVolumeClaimVolumeSource))(in)
	}
	out.ClaimName = in.ClaimName
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource(in *api.PersistentVolumeClaimVolumeSource, out *apiv1.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	return autoConvert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource(in, out, s)
}

func autoConvert_api_PodSpec_To_v1_PodSpec(in *api.PodSpec, out *apiv1.PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]apiv1.Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := Convert_api_Volume_To_v1_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]apiv1.Container, len(in.Containers))
		for i := range in.Containers {
			if err := Convert_api_Container_To_v1_Container(&in.Containers[i], &out.Containers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Containers = nil
	}
	out.RestartPolicy = apiv1.RestartPolicy(in.RestartPolicy)
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
	out.DNSPolicy = apiv1.DNSPolicy(in.DNSPolicy)
	if in.NodeSelector != nil {
		out.NodeSelector = make(map[string]string)
		for key, val := range in.NodeSelector {
			out.NodeSelector[key] = val
		}
	} else {
		out.NodeSelector = nil
	}
	out.ServiceAccountName = in.ServiceAccountName
	out.NodeName = in.NodeName
	// unable to generate simple pointer conversion for api.PodSecurityContext -> v1.PodSecurityContext
	if in.SecurityContext != nil {
		if err := s.Convert(&in.SecurityContext, &out.SecurityContext, 0); err != nil {
			return err
		}
	} else {
		out.SecurityContext = nil
	}
	if in.ImagePullSecrets != nil {
		out.ImagePullSecrets = make([]apiv1.LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}
	return nil
}

func autoConvert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in *api.PodTemplateSpec, out *apiv1.PodTemplateSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PodTemplateSpec))(in)
	}
	if err := Convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := batchv1.Convert_api_PodSpec_To_v1_PodSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in *api.PodTemplateSpec, out *apiv1.PodTemplateSpec, s conversion.Scope) error {
	return autoConvert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in, out, s)
}

func autoConvert_api_Probe_To_v1_Probe(in *api.Probe, out *apiv1.Probe, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Probe))(in)
	}
	if err := Convert_api_Handler_To_v1_Handler(&in.Handler, &out.Handler, s); err != nil {
		return err
	}
	out.InitialDelaySeconds = int32(in.InitialDelaySeconds)
	out.TimeoutSeconds = int32(in.TimeoutSeconds)
	out.PeriodSeconds = int32(in.PeriodSeconds)
	out.SuccessThreshold = int32(in.SuccessThreshold)
	out.FailureThreshold = int32(in.FailureThreshold)
	return nil
}

func Convert_api_Probe_To_v1_Probe(in *api.Probe, out *apiv1.Probe, s conversion.Scope) error {
	return autoConvert_api_Probe_To_v1_Probe(in, out, s)
}

func autoConvert_api_RBDVolumeSource_To_v1_RBDVolumeSource(in *api.RBDVolumeSource, out *apiv1.RBDVolumeSource, s conversion.Scope) error {
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
	// unable to generate simple pointer conversion for api.LocalObjectReference -> v1.LocalObjectReference
	if in.SecretRef != nil {
		out.SecretRef = new(apiv1.LocalObjectReference)
		if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_api_RBDVolumeSource_To_v1_RBDVolumeSource(in *api.RBDVolumeSource, out *apiv1.RBDVolumeSource, s conversion.Scope) error {
	return autoConvert_api_RBDVolumeSource_To_v1_RBDVolumeSource(in, out, s)
}

func autoConvert_api_ResourceRequirements_To_v1_ResourceRequirements(in *api.ResourceRequirements, out *apiv1.ResourceRequirements, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ResourceRequirements))(in)
	}
	if in.Limits != nil {
		out.Limits = make(apiv1.ResourceList)
		for key, val := range in.Limits {
			newVal := resource.Quantity{}
			if err := api.Convert_resource_Quantity_To_resource_Quantity(&val, &newVal, s); err != nil {
				return err
			}
			out.Limits[apiv1.ResourceName(key)] = newVal
		}
	} else {
		out.Limits = nil
	}
	if in.Requests != nil {
		out.Requests = make(apiv1.ResourceList)
		for key, val := range in.Requests {
			newVal := resource.Quantity{}
			if err := api.Convert_resource_Quantity_To_resource_Quantity(&val, &newVal, s); err != nil {
				return err
			}
			out.Requests[apiv1.ResourceName(key)] = newVal
		}
	} else {
		out.Requests = nil
	}
	return nil
}

func Convert_api_ResourceRequirements_To_v1_ResourceRequirements(in *api.ResourceRequirements, out *apiv1.ResourceRequirements, s conversion.Scope) error {
	return autoConvert_api_ResourceRequirements_To_v1_ResourceRequirements(in, out, s)
}

func autoConvert_api_SELinuxOptions_To_v1_SELinuxOptions(in *api.SELinuxOptions, out *apiv1.SELinuxOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.SELinuxOptions))(in)
	}
	out.User = in.User
	out.Role = in.Role
	out.Type = in.Type
	out.Level = in.Level
	return nil
}

func Convert_api_SELinuxOptions_To_v1_SELinuxOptions(in *api.SELinuxOptions, out *apiv1.SELinuxOptions, s conversion.Scope) error {
	return autoConvert_api_SELinuxOptions_To_v1_SELinuxOptions(in, out, s)
}

func autoConvert_api_SecretKeySelector_To_v1_SecretKeySelector(in *api.SecretKeySelector, out *apiv1.SecretKeySelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.SecretKeySelector))(in)
	}
	if err := Convert_api_LocalObjectReference_To_v1_LocalObjectReference(&in.LocalObjectReference, &out.LocalObjectReference, s); err != nil {
		return err
	}
	out.Key = in.Key
	return nil
}

func Convert_api_SecretKeySelector_To_v1_SecretKeySelector(in *api.SecretKeySelector, out *apiv1.SecretKeySelector, s conversion.Scope) error {
	return autoConvert_api_SecretKeySelector_To_v1_SecretKeySelector(in, out, s)
}

func autoConvert_api_SecretVolumeSource_To_v1_SecretVolumeSource(in *api.SecretVolumeSource, out *apiv1.SecretVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.SecretVolumeSource))(in)
	}
	out.SecretName = in.SecretName
	return nil
}

func Convert_api_SecretVolumeSource_To_v1_SecretVolumeSource(in *api.SecretVolumeSource, out *apiv1.SecretVolumeSource, s conversion.Scope) error {
	return autoConvert_api_SecretVolumeSource_To_v1_SecretVolumeSource(in, out, s)
}

func autoConvert_api_SecurityContext_To_v1_SecurityContext(in *api.SecurityContext, out *apiv1.SecurityContext, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.SecurityContext))(in)
	}
	// unable to generate simple pointer conversion for api.Capabilities -> v1.Capabilities
	if in.Capabilities != nil {
		out.Capabilities = new(apiv1.Capabilities)
		if err := Convert_api_Capabilities_To_v1_Capabilities(in.Capabilities, out.Capabilities, s); err != nil {
			return err
		}
	} else {
		out.Capabilities = nil
	}
	if in.Privileged != nil {
		out.Privileged = new(bool)
		*out.Privileged = *in.Privileged
	} else {
		out.Privileged = nil
	}
	// unable to generate simple pointer conversion for api.SELinuxOptions -> v1.SELinuxOptions
	if in.SELinuxOptions != nil {
		out.SELinuxOptions = new(apiv1.SELinuxOptions)
		if err := Convert_api_SELinuxOptions_To_v1_SELinuxOptions(in.SELinuxOptions, out.SELinuxOptions, s); err != nil {
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
	if in.ReadOnlyRootFilesystem != nil {
		out.ReadOnlyRootFilesystem = new(bool)
		*out.ReadOnlyRootFilesystem = *in.ReadOnlyRootFilesystem
	} else {
		out.ReadOnlyRootFilesystem = nil
	}
	return nil
}

func Convert_api_SecurityContext_To_v1_SecurityContext(in *api.SecurityContext, out *apiv1.SecurityContext, s conversion.Scope) error {
	return autoConvert_api_SecurityContext_To_v1_SecurityContext(in, out, s)
}

func autoConvert_api_TCPSocketAction_To_v1_TCPSocketAction(in *api.TCPSocketAction, out *apiv1.TCPSocketAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.TCPSocketAction))(in)
	}
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.Port, &out.Port, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_TCPSocketAction_To_v1_TCPSocketAction(in *api.TCPSocketAction, out *apiv1.TCPSocketAction, s conversion.Scope) error {
	return autoConvert_api_TCPSocketAction_To_v1_TCPSocketAction(in, out, s)
}

func autoConvert_api_Volume_To_v1_Volume(in *api.Volume, out *apiv1.Volume, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Volume))(in)
	}
	out.Name = in.Name
	if err := apiv1.Convert_api_VolumeSource_To_v1_VolumeSource(&in.VolumeSource, &out.VolumeSource, s); err != nil {
		return err
	}
	return nil
}

func Convert_api_Volume_To_v1_Volume(in *api.Volume, out *apiv1.Volume, s conversion.Scope) error {
	return autoConvert_api_Volume_To_v1_Volume(in, out, s)
}

func autoConvert_api_VolumeMount_To_v1_VolumeMount(in *api.VolumeMount, out *apiv1.VolumeMount, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.VolumeMount))(in)
	}
	out.Name = in.Name
	out.ReadOnly = in.ReadOnly
	out.MountPath = in.MountPath
	return nil
}

func Convert_api_VolumeMount_To_v1_VolumeMount(in *api.VolumeMount, out *apiv1.VolumeMount, s conversion.Scope) error {
	return autoConvert_api_VolumeMount_To_v1_VolumeMount(in, out, s)
}

func autoConvert_api_VolumeSource_To_v1_VolumeSource(in *api.VolumeSource, out *apiv1.VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.VolumeSource))(in)
	}
	// unable to generate simple pointer conversion for api.HostPathVolumeSource -> v1.HostPathVolumeSource
	if in.HostPath != nil {
		out.HostPath = new(apiv1.HostPathVolumeSource)
		if err := Convert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource(in.HostPath, out.HostPath, s); err != nil {
			return err
		}
	} else {
		out.HostPath = nil
	}
	// unable to generate simple pointer conversion for api.EmptyDirVolumeSource -> v1.EmptyDirVolumeSource
	if in.EmptyDir != nil {
		out.EmptyDir = new(apiv1.EmptyDirVolumeSource)
		if err := Convert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource(in.EmptyDir, out.EmptyDir, s); err != nil {
			return err
		}
	} else {
		out.EmptyDir = nil
	}
	// unable to generate simple pointer conversion for api.GCEPersistentDiskVolumeSource -> v1.GCEPersistentDiskVolumeSource
	if in.GCEPersistentDisk != nil {
		out.GCEPersistentDisk = new(apiv1.GCEPersistentDiskVolumeSource)
		if err := Convert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource(in.GCEPersistentDisk, out.GCEPersistentDisk, s); err != nil {
			return err
		}
	} else {
		out.GCEPersistentDisk = nil
	}
	// unable to generate simple pointer conversion for api.AWSElasticBlockStoreVolumeSource -> v1.AWSElasticBlockStoreVolumeSource
	if in.AWSElasticBlockStore != nil {
		out.AWSElasticBlockStore = new(apiv1.AWSElasticBlockStoreVolumeSource)
		if err := Convert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource(in.AWSElasticBlockStore, out.AWSElasticBlockStore, s); err != nil {
			return err
		}
	} else {
		out.AWSElasticBlockStore = nil
	}
	// unable to generate simple pointer conversion for api.GitRepoVolumeSource -> v1.GitRepoVolumeSource
	if in.GitRepo != nil {
		out.GitRepo = new(apiv1.GitRepoVolumeSource)
		if err := Convert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource(in.GitRepo, out.GitRepo, s); err != nil {
			return err
		}
	} else {
		out.GitRepo = nil
	}
	// unable to generate simple pointer conversion for api.SecretVolumeSource -> v1.SecretVolumeSource
	if in.Secret != nil {
		out.Secret = new(apiv1.SecretVolumeSource)
		if err := Convert_api_SecretVolumeSource_To_v1_SecretVolumeSource(in.Secret, out.Secret, s); err != nil {
			return err
		}
	} else {
		out.Secret = nil
	}
	// unable to generate simple pointer conversion for api.NFSVolumeSource -> v1.NFSVolumeSource
	if in.NFS != nil {
		out.NFS = new(apiv1.NFSVolumeSource)
		if err := Convert_api_NFSVolumeSource_To_v1_NFSVolumeSource(in.NFS, out.NFS, s); err != nil {
			return err
		}
	} else {
		out.NFS = nil
	}
	// unable to generate simple pointer conversion for api.ISCSIVolumeSource -> v1.ISCSIVolumeSource
	if in.ISCSI != nil {
		out.ISCSI = new(apiv1.ISCSIVolumeSource)
		if err := Convert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource(in.ISCSI, out.ISCSI, s); err != nil {
			return err
		}
	} else {
		out.ISCSI = nil
	}
	// unable to generate simple pointer conversion for api.GlusterfsVolumeSource -> v1.GlusterfsVolumeSource
	if in.Glusterfs != nil {
		out.Glusterfs = new(apiv1.GlusterfsVolumeSource)
		if err := Convert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource(in.Glusterfs, out.Glusterfs, s); err != nil {
			return err
		}
	} else {
		out.Glusterfs = nil
	}
	// unable to generate simple pointer conversion for api.PersistentVolumeClaimVolumeSource -> v1.PersistentVolumeClaimVolumeSource
	if in.PersistentVolumeClaim != nil {
		out.PersistentVolumeClaim = new(apiv1.PersistentVolumeClaimVolumeSource)
		if err := Convert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource(in.PersistentVolumeClaim, out.PersistentVolumeClaim, s); err != nil {
			return err
		}
	} else {
		out.PersistentVolumeClaim = nil
	}
	// unable to generate simple pointer conversion for api.RBDVolumeSource -> v1.RBDVolumeSource
	if in.RBD != nil {
		out.RBD = new(apiv1.RBDVolumeSource)
		if err := Convert_api_RBDVolumeSource_To_v1_RBDVolumeSource(in.RBD, out.RBD, s); err != nil {
			return err
		}
	} else {
		out.RBD = nil
	}
	// unable to generate simple pointer conversion for api.FlexVolumeSource -> v1.FlexVolumeSource
	if in.FlexVolume != nil {
		out.FlexVolume = new(apiv1.FlexVolumeSource)
		if err := Convert_api_FlexVolumeSource_To_v1_FlexVolumeSource(in.FlexVolume, out.FlexVolume, s); err != nil {
			return err
		}
	} else {
		out.FlexVolume = nil
	}
	// unable to generate simple pointer conversion for api.CinderVolumeSource -> v1.CinderVolumeSource
	if in.Cinder != nil {
		out.Cinder = new(apiv1.CinderVolumeSource)
		if err := Convert_api_CinderVolumeSource_To_v1_CinderVolumeSource(in.Cinder, out.Cinder, s); err != nil {
			return err
		}
	} else {
		out.Cinder = nil
	}
	// unable to generate simple pointer conversion for api.CephFSVolumeSource -> v1.CephFSVolumeSource
	if in.CephFS != nil {
		out.CephFS = new(apiv1.CephFSVolumeSource)
		if err := Convert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource(in.CephFS, out.CephFS, s); err != nil {
			return err
		}
	} else {
		out.CephFS = nil
	}
	// unable to generate simple pointer conversion for api.FlockerVolumeSource -> v1.FlockerVolumeSource
	if in.Flocker != nil {
		out.Flocker = new(apiv1.FlockerVolumeSource)
		if err := Convert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource(in.Flocker, out.Flocker, s); err != nil {
			return err
		}
	} else {
		out.Flocker = nil
	}
	// unable to generate simple pointer conversion for api.DownwardAPIVolumeSource -> v1.DownwardAPIVolumeSource
	if in.DownwardAPI != nil {
		out.DownwardAPI = new(apiv1.DownwardAPIVolumeSource)
		if err := Convert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource(in.DownwardAPI, out.DownwardAPI, s); err != nil {
			return err
		}
	} else {
		out.DownwardAPI = nil
	}
	// unable to generate simple pointer conversion for api.FCVolumeSource -> v1.FCVolumeSource
	if in.FC != nil {
		out.FC = new(apiv1.FCVolumeSource)
		if err := Convert_api_FCVolumeSource_To_v1_FCVolumeSource(in.FC, out.FC, s); err != nil {
			return err
		}
	} else {
		out.FC = nil
	}
	// unable to generate simple pointer conversion for api.AzureFileVolumeSource -> v1.AzureFileVolumeSource
	if in.AzureFile != nil {
		out.AzureFile = new(apiv1.AzureFileVolumeSource)
		if err := Convert_api_AzureFileVolumeSource_To_v1_AzureFileVolumeSource(in.AzureFile, out.AzureFile, s); err != nil {
			return err
		}
	} else {
		out.AzureFile = nil
	}
	// unable to generate simple pointer conversion for api.ConfigMapVolumeSource -> v1.ConfigMapVolumeSource
	if in.ConfigMap != nil {
		out.ConfigMap = new(apiv1.ConfigMapVolumeSource)
		if err := Convert_api_ConfigMapVolumeSource_To_v1_ConfigMapVolumeSource(in.ConfigMap, out.ConfigMap, s); err != nil {
			return err
		}
	} else {
		out.ConfigMap = nil
	}
	return nil
}

func autoConvert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in *apiv1.AWSElasticBlockStoreVolumeSource, out *api.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.AWSElasticBlockStoreVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.Partition = int(in.Partition)
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in *apiv1.AWSElasticBlockStoreVolumeSource, out *api.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in, out, s)
}

func autoConvert_v1_AzureFileVolumeSource_To_api_AzureFileVolumeSource(in *apiv1.AzureFileVolumeSource, out *api.AzureFileVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.AzureFileVolumeSource))(in)
	}
	out.SecretName = in.SecretName
	out.ShareName = in.ShareName
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_AzureFileVolumeSource_To_api_AzureFileVolumeSource(in *apiv1.AzureFileVolumeSource, out *api.AzureFileVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_AzureFileVolumeSource_To_api_AzureFileVolumeSource(in, out, s)
}

func autoConvert_v1_Capabilities_To_api_Capabilities(in *apiv1.Capabilities, out *api.Capabilities, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.Capabilities))(in)
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

func Convert_v1_Capabilities_To_api_Capabilities(in *apiv1.Capabilities, out *api.Capabilities, s conversion.Scope) error {
	return autoConvert_v1_Capabilities_To_api_Capabilities(in, out, s)
}

func autoConvert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource(in *apiv1.CephFSVolumeSource, out *api.CephFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.CephFSVolumeSource))(in)
	}
	if in.Monitors != nil {
		out.Monitors = make([]string, len(in.Monitors))
		for i := range in.Monitors {
			out.Monitors[i] = in.Monitors[i]
		}
	} else {
		out.Monitors = nil
	}
	out.Path = in.Path
	out.User = in.User
	out.SecretFile = in.SecretFile
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.SecretRef != nil {
		out.SecretRef = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource(in *apiv1.CephFSVolumeSource, out *api.CephFSVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource(in, out, s)
}

func autoConvert_v1_CinderVolumeSource_To_api_CinderVolumeSource(in *apiv1.CinderVolumeSource, out *api.CinderVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.CinderVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_CinderVolumeSource_To_api_CinderVolumeSource(in *apiv1.CinderVolumeSource, out *api.CinderVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_CinderVolumeSource_To_api_CinderVolumeSource(in, out, s)
}

func autoConvert_v1_ConfigMapKeySelector_To_api_ConfigMapKeySelector(in *apiv1.ConfigMapKeySelector, out *api.ConfigMapKeySelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ConfigMapKeySelector))(in)
	}
	if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(&in.LocalObjectReference, &out.LocalObjectReference, s); err != nil {
		return err
	}
	out.Key = in.Key
	return nil
}

func Convert_v1_ConfigMapKeySelector_To_api_ConfigMapKeySelector(in *apiv1.ConfigMapKeySelector, out *api.ConfigMapKeySelector, s conversion.Scope) error {
	return autoConvert_v1_ConfigMapKeySelector_To_api_ConfigMapKeySelector(in, out, s)
}

func autoConvert_v1_ConfigMapVolumeSource_To_api_ConfigMapVolumeSource(in *apiv1.ConfigMapVolumeSource, out *api.ConfigMapVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ConfigMapVolumeSource))(in)
	}
	if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(&in.LocalObjectReference, &out.LocalObjectReference, s); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]api.KeyToPath, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_KeyToPath_To_api_KeyToPath(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_ConfigMapVolumeSource_To_api_ConfigMapVolumeSource(in *apiv1.ConfigMapVolumeSource, out *api.ConfigMapVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_ConfigMapVolumeSource_To_api_ConfigMapVolumeSource(in, out, s)
}

func autoConvert_v1_Container_To_api_Container(in *apiv1.Container, out *api.Container, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.Container))(in)
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
			if err := Convert_v1_ContainerPort_To_api_ContainerPort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ports = nil
	}
	if in.Env != nil {
		out.Env = make([]api.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := Convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	if err := Convert_v1_ResourceRequirements_To_api_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if in.VolumeMounts != nil {
		out.VolumeMounts = make([]api.VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			if err := Convert_v1_VolumeMount_To_api_VolumeMount(&in.VolumeMounts[i], &out.VolumeMounts[i], s); err != nil {
				return err
			}
		}
	} else {
		out.VolumeMounts = nil
	}
	// unable to generate simple pointer conversion for v1.Probe -> api.Probe
	if in.LivenessProbe != nil {
		out.LivenessProbe = new(api.Probe)
		if err := Convert_v1_Probe_To_api_Probe(in.LivenessProbe, out.LivenessProbe, s); err != nil {
			return err
		}
	} else {
		out.LivenessProbe = nil
	}
	// unable to generate simple pointer conversion for v1.Probe -> api.Probe
	if in.ReadinessProbe != nil {
		out.ReadinessProbe = new(api.Probe)
		if err := Convert_v1_Probe_To_api_Probe(in.ReadinessProbe, out.ReadinessProbe, s); err != nil {
			return err
		}
	} else {
		out.ReadinessProbe = nil
	}
	// unable to generate simple pointer conversion for v1.Lifecycle -> api.Lifecycle
	if in.Lifecycle != nil {
		out.Lifecycle = new(api.Lifecycle)
		if err := Convert_v1_Lifecycle_To_api_Lifecycle(in.Lifecycle, out.Lifecycle, s); err != nil {
			return err
		}
	} else {
		out.Lifecycle = nil
	}
	out.TerminationMessagePath = in.TerminationMessagePath
	out.ImagePullPolicy = api.PullPolicy(in.ImagePullPolicy)
	// unable to generate simple pointer conversion for v1.SecurityContext -> api.SecurityContext
	if in.SecurityContext != nil {
		out.SecurityContext = new(api.SecurityContext)
		if err := Convert_v1_SecurityContext_To_api_SecurityContext(in.SecurityContext, out.SecurityContext, s); err != nil {
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

func Convert_v1_Container_To_api_Container(in *apiv1.Container, out *api.Container, s conversion.Scope) error {
	return autoConvert_v1_Container_To_api_Container(in, out, s)
}

func autoConvert_v1_ContainerPort_To_api_ContainerPort(in *apiv1.ContainerPort, out *api.ContainerPort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ContainerPort))(in)
	}
	out.Name = in.Name
	out.HostPort = int(in.HostPort)
	out.ContainerPort = int(in.ContainerPort)
	out.Protocol = api.Protocol(in.Protocol)
	out.HostIP = in.HostIP
	return nil
}

func Convert_v1_ContainerPort_To_api_ContainerPort(in *apiv1.ContainerPort, out *api.ContainerPort, s conversion.Scope) error {
	return autoConvert_v1_ContainerPort_To_api_ContainerPort(in, out, s)
}

func autoConvert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(in *apiv1.DownwardAPIVolumeFile, out *api.DownwardAPIVolumeFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.DownwardAPIVolumeFile))(in)
	}
	out.Path = in.Path
	if err := Convert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(in *apiv1.DownwardAPIVolumeFile, out *api.DownwardAPIVolumeFile, s conversion.Scope) error {
	return autoConvert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(in, out, s)
}

func autoConvert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in *apiv1.DownwardAPIVolumeSource, out *api.DownwardAPIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.DownwardAPIVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]api.DownwardAPIVolumeFile, len(in.Items))
		for i := range in.Items {
			if err := Convert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func Convert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in *apiv1.DownwardAPIVolumeSource, out *api.DownwardAPIVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in, out, s)
}

func autoConvert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in *apiv1.EmptyDirVolumeSource, out *api.EmptyDirVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.EmptyDirVolumeSource))(in)
	}
	out.Medium = api.StorageMedium(in.Medium)
	return nil
}

func Convert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in *apiv1.EmptyDirVolumeSource, out *api.EmptyDirVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in, out, s)
}

func autoConvert_v1_EnvVar_To_api_EnvVar(in *apiv1.EnvVar, out *api.EnvVar, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.EnvVar))(in)
	}
	out.Name = in.Name
	out.Value = in.Value
	// unable to generate simple pointer conversion for v1.EnvVarSource -> api.EnvVarSource
	if in.ValueFrom != nil {
		out.ValueFrom = new(api.EnvVarSource)
		if err := Convert_v1_EnvVarSource_To_api_EnvVarSource(in.ValueFrom, out.ValueFrom, s); err != nil {
			return err
		}
	} else {
		out.ValueFrom = nil
	}
	return nil
}

func Convert_v1_EnvVar_To_api_EnvVar(in *apiv1.EnvVar, out *api.EnvVar, s conversion.Scope) error {
	return autoConvert_v1_EnvVar_To_api_EnvVar(in, out, s)
}

func autoConvert_v1_EnvVarSource_To_api_EnvVarSource(in *apiv1.EnvVarSource, out *api.EnvVarSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.EnvVarSource))(in)
	}
	// unable to generate simple pointer conversion for v1.ObjectFieldSelector -> api.ObjectFieldSelector
	if in.FieldRef != nil {
		out.FieldRef = new(api.ObjectFieldSelector)
		if err := Convert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(in.FieldRef, out.FieldRef, s); err != nil {
			return err
		}
	} else {
		out.FieldRef = nil
	}
	// unable to generate simple pointer conversion for v1.ConfigMapKeySelector -> api.ConfigMapKeySelector
	if in.ConfigMapKeyRef != nil {
		out.ConfigMapKeyRef = new(api.ConfigMapKeySelector)
		if err := Convert_v1_ConfigMapKeySelector_To_api_ConfigMapKeySelector(in.ConfigMapKeyRef, out.ConfigMapKeyRef, s); err != nil {
			return err
		}
	} else {
		out.ConfigMapKeyRef = nil
	}
	// unable to generate simple pointer conversion for v1.SecretKeySelector -> api.SecretKeySelector
	if in.SecretKeyRef != nil {
		out.SecretKeyRef = new(api.SecretKeySelector)
		if err := Convert_v1_SecretKeySelector_To_api_SecretKeySelector(in.SecretKeyRef, out.SecretKeyRef, s); err != nil {
			return err
		}
	} else {
		out.SecretKeyRef = nil
	}
	return nil
}

func Convert_v1_EnvVarSource_To_api_EnvVarSource(in *apiv1.EnvVarSource, out *api.EnvVarSource, s conversion.Scope) error {
	return autoConvert_v1_EnvVarSource_To_api_EnvVarSource(in, out, s)
}

func autoConvert_v1_ExecAction_To_api_ExecAction(in *apiv1.ExecAction, out *api.ExecAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ExecAction))(in)
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

func Convert_v1_ExecAction_To_api_ExecAction(in *apiv1.ExecAction, out *api.ExecAction, s conversion.Scope) error {
	return autoConvert_v1_ExecAction_To_api_ExecAction(in, out, s)
}

func autoConvert_v1_FCVolumeSource_To_api_FCVolumeSource(in *apiv1.FCVolumeSource, out *api.FCVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.FCVolumeSource))(in)
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
		*out.Lun = int(*in.Lun)
	} else {
		out.Lun = nil
	}
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_FCVolumeSource_To_api_FCVolumeSource(in *apiv1.FCVolumeSource, out *api.FCVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_FCVolumeSource_To_api_FCVolumeSource(in, out, s)
}

func autoConvert_v1_FlexVolumeSource_To_api_FlexVolumeSource(in *apiv1.FlexVolumeSource, out *api.FlexVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.FlexVolumeSource))(in)
	}
	out.Driver = in.Driver
	out.FSType = in.FSType
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.SecretRef != nil {
		out.SecretRef = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	if in.Options != nil {
		out.Options = make(map[string]string)
		for key, val := range in.Options {
			out.Options[key] = val
		}
	} else {
		out.Options = nil
	}
	return nil
}

func Convert_v1_FlexVolumeSource_To_api_FlexVolumeSource(in *apiv1.FlexVolumeSource, out *api.FlexVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_FlexVolumeSource_To_api_FlexVolumeSource(in, out, s)
}

func autoConvert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource(in *apiv1.FlockerVolumeSource, out *api.FlockerVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.FlockerVolumeSource))(in)
	}
	out.DatasetName = in.DatasetName
	return nil
}

func Convert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource(in *apiv1.FlockerVolumeSource, out *api.FlockerVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource(in, out, s)
}

func autoConvert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in *apiv1.GCEPersistentDiskVolumeSource, out *api.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.GCEPersistentDiskVolumeSource))(in)
	}
	out.PDName = in.PDName
	out.FSType = in.FSType
	out.Partition = int(in.Partition)
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in *apiv1.GCEPersistentDiskVolumeSource, out *api.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in, out, s)
}

func autoConvert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource(in *apiv1.GitRepoVolumeSource, out *api.GitRepoVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.GitRepoVolumeSource))(in)
	}
	out.Repository = in.Repository
	out.Revision = in.Revision
	out.Directory = in.Directory
	return nil
}

func Convert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource(in *apiv1.GitRepoVolumeSource, out *api.GitRepoVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource(in, out, s)
}

func autoConvert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in *apiv1.GlusterfsVolumeSource, out *api.GlusterfsVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.GlusterfsVolumeSource))(in)
	}
	out.EndpointsName = in.EndpointsName
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in *apiv1.GlusterfsVolumeSource, out *api.GlusterfsVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in, out, s)
}

func autoConvert_v1_HTTPGetAction_To_api_HTTPGetAction(in *apiv1.HTTPGetAction, out *api.HTTPGetAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.HTTPGetAction))(in)
	}
	out.Path = in.Path
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.Port, &out.Port, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.Scheme = api.URIScheme(in.Scheme)
	if in.HTTPHeaders != nil {
		out.HTTPHeaders = make([]api.HTTPHeader, len(in.HTTPHeaders))
		for i := range in.HTTPHeaders {
			if err := Convert_v1_HTTPHeader_To_api_HTTPHeader(&in.HTTPHeaders[i], &out.HTTPHeaders[i], s); err != nil {
				return err
			}
		}
	} else {
		out.HTTPHeaders = nil
	}
	return nil
}

func Convert_v1_HTTPGetAction_To_api_HTTPGetAction(in *apiv1.HTTPGetAction, out *api.HTTPGetAction, s conversion.Scope) error {
	return autoConvert_v1_HTTPGetAction_To_api_HTTPGetAction(in, out, s)
}

func autoConvert_v1_HTTPHeader_To_api_HTTPHeader(in *apiv1.HTTPHeader, out *api.HTTPHeader, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.HTTPHeader))(in)
	}
	out.Name = in.Name
	out.Value = in.Value
	return nil
}

func Convert_v1_HTTPHeader_To_api_HTTPHeader(in *apiv1.HTTPHeader, out *api.HTTPHeader, s conversion.Scope) error {
	return autoConvert_v1_HTTPHeader_To_api_HTTPHeader(in, out, s)
}

func autoConvert_v1_Handler_To_api_Handler(in *apiv1.Handler, out *api.Handler, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.Handler))(in)
	}
	// unable to generate simple pointer conversion for v1.ExecAction -> api.ExecAction
	if in.Exec != nil {
		out.Exec = new(api.ExecAction)
		if err := Convert_v1_ExecAction_To_api_ExecAction(in.Exec, out.Exec, s); err != nil {
			return err
		}
	} else {
		out.Exec = nil
	}
	// unable to generate simple pointer conversion for v1.HTTPGetAction -> api.HTTPGetAction
	if in.HTTPGet != nil {
		out.HTTPGet = new(api.HTTPGetAction)
		if err := Convert_v1_HTTPGetAction_To_api_HTTPGetAction(in.HTTPGet, out.HTTPGet, s); err != nil {
			return err
		}
	} else {
		out.HTTPGet = nil
	}
	// unable to generate simple pointer conversion for v1.TCPSocketAction -> api.TCPSocketAction
	if in.TCPSocket != nil {
		out.TCPSocket = new(api.TCPSocketAction)
		if err := Convert_v1_TCPSocketAction_To_api_TCPSocketAction(in.TCPSocket, out.TCPSocket, s); err != nil {
			return err
		}
	} else {
		out.TCPSocket = nil
	}
	return nil
}

func Convert_v1_Handler_To_api_Handler(in *apiv1.Handler, out *api.Handler, s conversion.Scope) error {
	return autoConvert_v1_Handler_To_api_Handler(in, out, s)
}

func autoConvert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource(in *apiv1.HostPathVolumeSource, out *api.HostPathVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.HostPathVolumeSource))(in)
	}
	out.Path = in.Path
	return nil
}

func Convert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource(in *apiv1.HostPathVolumeSource, out *api.HostPathVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource(in, out, s)
}

func autoConvert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource(in *apiv1.ISCSIVolumeSource, out *api.ISCSIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ISCSIVolumeSource))(in)
	}
	out.TargetPortal = in.TargetPortal
	out.IQN = in.IQN
	out.Lun = int(in.Lun)
	out.ISCSIInterface = in.ISCSIInterface
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource(in *apiv1.ISCSIVolumeSource, out *api.ISCSIVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource(in, out, s)
}

func autoConvert_v1_KeyToPath_To_api_KeyToPath(in *apiv1.KeyToPath, out *api.KeyToPath, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.KeyToPath))(in)
	}
	out.Key = in.Key
	out.Path = in.Path
	return nil
}

func Convert_v1_KeyToPath_To_api_KeyToPath(in *apiv1.KeyToPath, out *api.KeyToPath, s conversion.Scope) error {
	return autoConvert_v1_KeyToPath_To_api_KeyToPath(in, out, s)
}

func autoConvert_v1_Lifecycle_To_api_Lifecycle(in *apiv1.Lifecycle, out *api.Lifecycle, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.Lifecycle))(in)
	}
	// unable to generate simple pointer conversion for v1.Handler -> api.Handler
	if in.PostStart != nil {
		out.PostStart = new(api.Handler)
		if err := Convert_v1_Handler_To_api_Handler(in.PostStart, out.PostStart, s); err != nil {
			return err
		}
	} else {
		out.PostStart = nil
	}
	// unable to generate simple pointer conversion for v1.Handler -> api.Handler
	if in.PreStop != nil {
		out.PreStop = new(api.Handler)
		if err := Convert_v1_Handler_To_api_Handler(in.PreStop, out.PreStop, s); err != nil {
			return err
		}
	} else {
		out.PreStop = nil
	}
	return nil
}

func Convert_v1_Lifecycle_To_api_Lifecycle(in *apiv1.Lifecycle, out *api.Lifecycle, s conversion.Scope) error {
	return autoConvert_v1_Lifecycle_To_api_Lifecycle(in, out, s)
}

func autoConvert_v1_LocalObjectReference_To_api_LocalObjectReference(in *apiv1.LocalObjectReference, out *api.LocalObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.LocalObjectReference))(in)
	}
	out.Name = in.Name
	return nil
}

func Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in *apiv1.LocalObjectReference, out *api.LocalObjectReference, s conversion.Scope) error {
	return autoConvert_v1_LocalObjectReference_To_api_LocalObjectReference(in, out, s)
}

func autoConvert_v1_NFSVolumeSource_To_api_NFSVolumeSource(in *apiv1.NFSVolumeSource, out *api.NFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.NFSVolumeSource))(in)
	}
	out.Server = in.Server
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_NFSVolumeSource_To_api_NFSVolumeSource(in *apiv1.NFSVolumeSource, out *api.NFSVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_NFSVolumeSource_To_api_NFSVolumeSource(in, out, s)
}

func autoConvert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(in *apiv1.ObjectFieldSelector, out *api.ObjectFieldSelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ObjectFieldSelector))(in)
	}
	out.APIVersion = in.APIVersion
	out.FieldPath = in.FieldPath
	return nil
}

func Convert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(in *apiv1.ObjectFieldSelector, out *api.ObjectFieldSelector, s conversion.Scope) error {
	return autoConvert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(in, out, s)
}

func autoConvert_v1_ObjectMeta_To_api_ObjectMeta(in *apiv1.ObjectMeta, out *api.ObjectMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ObjectMeta))(in)
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

func Convert_v1_ObjectMeta_To_api_ObjectMeta(in *apiv1.ObjectMeta, out *api.ObjectMeta, s conversion.Scope) error {
	return autoConvert_v1_ObjectMeta_To_api_ObjectMeta(in, out, s)
}

func autoConvert_v1_ObjectReference_To_api_ObjectReference(in *apiv1.ObjectReference, out *api.ObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ObjectReference))(in)
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

func Convert_v1_ObjectReference_To_api_ObjectReference(in *apiv1.ObjectReference, out *api.ObjectReference, s conversion.Scope) error {
	return autoConvert_v1_ObjectReference_To_api_ObjectReference(in, out, s)
}

func autoConvert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in *apiv1.PersistentVolumeClaimVolumeSource, out *api.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.PersistentVolumeClaimVolumeSource))(in)
	}
	out.ClaimName = in.ClaimName
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in *apiv1.PersistentVolumeClaimVolumeSource, out *api.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in, out, s)
}

func autoConvert_v1_PodSpec_To_api_PodSpec(in *apiv1.PodSpec, out *api.PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]api.Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := Convert_v1_Volume_To_api_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]api.Container, len(in.Containers))
		for i := range in.Containers {
			if err := Convert_v1_Container_To_api_Container(&in.Containers[i], &out.Containers[i], s); err != nil {
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
	// in.DeprecatedHost has no peer in out
	out.ServiceAccountName = in.ServiceAccountName
	// in.DeprecatedServiceAccount has no peer in out
	out.NodeName = in.NodeName
	// in.HostNetwork has no peer in out
	// in.HostPID has no peer in out
	// in.HostIPC has no peer in out
	// unable to generate simple pointer conversion for v1.PodSecurityContext -> api.PodSecurityContext
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
			if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}
	return nil
}

func autoConvert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in *apiv1.PodTemplateSpec, out *api.PodTemplateSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.PodTemplateSpec))(in)
	}
	if err := Convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := batchv1.Convert_v1_PodSpec_To_api_PodSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in *apiv1.PodTemplateSpec, out *api.PodTemplateSpec, s conversion.Scope) error {
	return autoConvert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in, out, s)
}

func autoConvert_v1_Probe_To_api_Probe(in *apiv1.Probe, out *api.Probe, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.Probe))(in)
	}
	if err := Convert_v1_Handler_To_api_Handler(&in.Handler, &out.Handler, s); err != nil {
		return err
	}
	out.InitialDelaySeconds = int(in.InitialDelaySeconds)
	out.TimeoutSeconds = int(in.TimeoutSeconds)
	out.PeriodSeconds = int(in.PeriodSeconds)
	out.SuccessThreshold = int(in.SuccessThreshold)
	out.FailureThreshold = int(in.FailureThreshold)
	return nil
}

func Convert_v1_Probe_To_api_Probe(in *apiv1.Probe, out *api.Probe, s conversion.Scope) error {
	return autoConvert_v1_Probe_To_api_Probe(in, out, s)
}

func autoConvert_v1_RBDVolumeSource_To_api_RBDVolumeSource(in *apiv1.RBDVolumeSource, out *api.RBDVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.RBDVolumeSource))(in)
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
	// unable to generate simple pointer conversion for v1.LocalObjectReference -> api.LocalObjectReference
	if in.SecretRef != nil {
		out.SecretRef = new(api.LocalObjectReference)
		if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func Convert_v1_RBDVolumeSource_To_api_RBDVolumeSource(in *apiv1.RBDVolumeSource, out *api.RBDVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_RBDVolumeSource_To_api_RBDVolumeSource(in, out, s)
}

func autoConvert_v1_ResourceRequirements_To_api_ResourceRequirements(in *apiv1.ResourceRequirements, out *api.ResourceRequirements, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ResourceRequirements))(in)
	}
	if err := s.Convert(&in.Limits, &out.Limits, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Requests, &out.Requests, 0); err != nil {
		return err
	}
	return nil
}

func Convert_v1_ResourceRequirements_To_api_ResourceRequirements(in *apiv1.ResourceRequirements, out *api.ResourceRequirements, s conversion.Scope) error {
	return autoConvert_v1_ResourceRequirements_To_api_ResourceRequirements(in, out, s)
}

func autoConvert_v1_SELinuxOptions_To_api_SELinuxOptions(in *apiv1.SELinuxOptions, out *api.SELinuxOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.SELinuxOptions))(in)
	}
	out.User = in.User
	out.Role = in.Role
	out.Type = in.Type
	out.Level = in.Level
	return nil
}

func Convert_v1_SELinuxOptions_To_api_SELinuxOptions(in *apiv1.SELinuxOptions, out *api.SELinuxOptions, s conversion.Scope) error {
	return autoConvert_v1_SELinuxOptions_To_api_SELinuxOptions(in, out, s)
}

func autoConvert_v1_SecretKeySelector_To_api_SecretKeySelector(in *apiv1.SecretKeySelector, out *api.SecretKeySelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.SecretKeySelector))(in)
	}
	if err := Convert_v1_LocalObjectReference_To_api_LocalObjectReference(&in.LocalObjectReference, &out.LocalObjectReference, s); err != nil {
		return err
	}
	out.Key = in.Key
	return nil
}

func Convert_v1_SecretKeySelector_To_api_SecretKeySelector(in *apiv1.SecretKeySelector, out *api.SecretKeySelector, s conversion.Scope) error {
	return autoConvert_v1_SecretKeySelector_To_api_SecretKeySelector(in, out, s)
}

func autoConvert_v1_SecretVolumeSource_To_api_SecretVolumeSource(in *apiv1.SecretVolumeSource, out *api.SecretVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.SecretVolumeSource))(in)
	}
	out.SecretName = in.SecretName
	return nil
}

func Convert_v1_SecretVolumeSource_To_api_SecretVolumeSource(in *apiv1.SecretVolumeSource, out *api.SecretVolumeSource, s conversion.Scope) error {
	return autoConvert_v1_SecretVolumeSource_To_api_SecretVolumeSource(in, out, s)
}

func autoConvert_v1_SecurityContext_To_api_SecurityContext(in *apiv1.SecurityContext, out *api.SecurityContext, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.SecurityContext))(in)
	}
	// unable to generate simple pointer conversion for v1.Capabilities -> api.Capabilities
	if in.Capabilities != nil {
		out.Capabilities = new(api.Capabilities)
		if err := Convert_v1_Capabilities_To_api_Capabilities(in.Capabilities, out.Capabilities, s); err != nil {
			return err
		}
	} else {
		out.Capabilities = nil
	}
	if in.Privileged != nil {
		out.Privileged = new(bool)
		*out.Privileged = *in.Privileged
	} else {
		out.Privileged = nil
	}
	// unable to generate simple pointer conversion for v1.SELinuxOptions -> api.SELinuxOptions
	if in.SELinuxOptions != nil {
		out.SELinuxOptions = new(api.SELinuxOptions)
		if err := Convert_v1_SELinuxOptions_To_api_SELinuxOptions(in.SELinuxOptions, out.SELinuxOptions, s); err != nil {
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
	if in.ReadOnlyRootFilesystem != nil {
		out.ReadOnlyRootFilesystem = new(bool)
		*out.ReadOnlyRootFilesystem = *in.ReadOnlyRootFilesystem
	} else {
		out.ReadOnlyRootFilesystem = nil
	}
	return nil
}

func Convert_v1_SecurityContext_To_api_SecurityContext(in *apiv1.SecurityContext, out *api.SecurityContext, s conversion.Scope) error {
	return autoConvert_v1_SecurityContext_To_api_SecurityContext(in, out, s)
}

func autoConvert_v1_TCPSocketAction_To_api_TCPSocketAction(in *apiv1.TCPSocketAction, out *api.TCPSocketAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.TCPSocketAction))(in)
	}
	if err := api.Convert_intstr_IntOrString_To_intstr_IntOrString(&in.Port, &out.Port, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_TCPSocketAction_To_api_TCPSocketAction(in *apiv1.TCPSocketAction, out *api.TCPSocketAction, s conversion.Scope) error {
	return autoConvert_v1_TCPSocketAction_To_api_TCPSocketAction(in, out, s)
}

func autoConvert_v1_Volume_To_api_Volume(in *apiv1.Volume, out *api.Volume, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.Volume))(in)
	}
	out.Name = in.Name
	if err := apiv1.Convert_v1_VolumeSource_To_api_VolumeSource(&in.VolumeSource, &out.VolumeSource, s); err != nil {
		return err
	}
	return nil
}

func Convert_v1_Volume_To_api_Volume(in *apiv1.Volume, out *api.Volume, s conversion.Scope) error {
	return autoConvert_v1_Volume_To_api_Volume(in, out, s)
}

func autoConvert_v1_VolumeMount_To_api_VolumeMount(in *apiv1.VolumeMount, out *api.VolumeMount, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.VolumeMount))(in)
	}
	out.Name = in.Name
	out.ReadOnly = in.ReadOnly
	out.MountPath = in.MountPath
	return nil
}

func Convert_v1_VolumeMount_To_api_VolumeMount(in *apiv1.VolumeMount, out *api.VolumeMount, s conversion.Scope) error {
	return autoConvert_v1_VolumeMount_To_api_VolumeMount(in, out, s)
}

func autoConvert_v1_VolumeSource_To_api_VolumeSource(in *apiv1.VolumeSource, out *api.VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.VolumeSource))(in)
	}
	// unable to generate simple pointer conversion for v1.HostPathVolumeSource -> api.HostPathVolumeSource
	if in.HostPath != nil {
		out.HostPath = new(api.HostPathVolumeSource)
		if err := Convert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource(in.HostPath, out.HostPath, s); err != nil {
			return err
		}
	} else {
		out.HostPath = nil
	}
	// unable to generate simple pointer conversion for v1.EmptyDirVolumeSource -> api.EmptyDirVolumeSource
	if in.EmptyDir != nil {
		out.EmptyDir = new(api.EmptyDirVolumeSource)
		if err := Convert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in.EmptyDir, out.EmptyDir, s); err != nil {
			return err
		}
	} else {
		out.EmptyDir = nil
	}
	// unable to generate simple pointer conversion for v1.GCEPersistentDiskVolumeSource -> api.GCEPersistentDiskVolumeSource
	if in.GCEPersistentDisk != nil {
		out.GCEPersistentDisk = new(api.GCEPersistentDiskVolumeSource)
		if err := Convert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in.GCEPersistentDisk, out.GCEPersistentDisk, s); err != nil {
			return err
		}
	} else {
		out.GCEPersistentDisk = nil
	}
	// unable to generate simple pointer conversion for v1.AWSElasticBlockStoreVolumeSource -> api.AWSElasticBlockStoreVolumeSource
	if in.AWSElasticBlockStore != nil {
		out.AWSElasticBlockStore = new(api.AWSElasticBlockStoreVolumeSource)
		if err := Convert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in.AWSElasticBlockStore, out.AWSElasticBlockStore, s); err != nil {
			return err
		}
	} else {
		out.AWSElasticBlockStore = nil
	}
	// unable to generate simple pointer conversion for v1.GitRepoVolumeSource -> api.GitRepoVolumeSource
	if in.GitRepo != nil {
		out.GitRepo = new(api.GitRepoVolumeSource)
		if err := Convert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource(in.GitRepo, out.GitRepo, s); err != nil {
			return err
		}
	} else {
		out.GitRepo = nil
	}
	// unable to generate simple pointer conversion for v1.SecretVolumeSource -> api.SecretVolumeSource
	if in.Secret != nil {
		out.Secret = new(api.SecretVolumeSource)
		if err := Convert_v1_SecretVolumeSource_To_api_SecretVolumeSource(in.Secret, out.Secret, s); err != nil {
			return err
		}
	} else {
		out.Secret = nil
	}
	// unable to generate simple pointer conversion for v1.NFSVolumeSource -> api.NFSVolumeSource
	if in.NFS != nil {
		out.NFS = new(api.NFSVolumeSource)
		if err := Convert_v1_NFSVolumeSource_To_api_NFSVolumeSource(in.NFS, out.NFS, s); err != nil {
			return err
		}
	} else {
		out.NFS = nil
	}
	// unable to generate simple pointer conversion for v1.ISCSIVolumeSource -> api.ISCSIVolumeSource
	if in.ISCSI != nil {
		out.ISCSI = new(api.ISCSIVolumeSource)
		if err := Convert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource(in.ISCSI, out.ISCSI, s); err != nil {
			return err
		}
	} else {
		out.ISCSI = nil
	}
	// unable to generate simple pointer conversion for v1.GlusterfsVolumeSource -> api.GlusterfsVolumeSource
	if in.Glusterfs != nil {
		out.Glusterfs = new(api.GlusterfsVolumeSource)
		if err := Convert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in.Glusterfs, out.Glusterfs, s); err != nil {
			return err
		}
	} else {
		out.Glusterfs = nil
	}
	// unable to generate simple pointer conversion for v1.PersistentVolumeClaimVolumeSource -> api.PersistentVolumeClaimVolumeSource
	if in.PersistentVolumeClaim != nil {
		out.PersistentVolumeClaim = new(api.PersistentVolumeClaimVolumeSource)
		if err := Convert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in.PersistentVolumeClaim, out.PersistentVolumeClaim, s); err != nil {
			return err
		}
	} else {
		out.PersistentVolumeClaim = nil
	}
	// unable to generate simple pointer conversion for v1.RBDVolumeSource -> api.RBDVolumeSource
	if in.RBD != nil {
		out.RBD = new(api.RBDVolumeSource)
		if err := Convert_v1_RBDVolumeSource_To_api_RBDVolumeSource(in.RBD, out.RBD, s); err != nil {
			return err
		}
	} else {
		out.RBD = nil
	}
	// unable to generate simple pointer conversion for v1.FlexVolumeSource -> api.FlexVolumeSource
	if in.FlexVolume != nil {
		out.FlexVolume = new(api.FlexVolumeSource)
		if err := Convert_v1_FlexVolumeSource_To_api_FlexVolumeSource(in.FlexVolume, out.FlexVolume, s); err != nil {
			return err
		}
	} else {
		out.FlexVolume = nil
	}
	// unable to generate simple pointer conversion for v1.CinderVolumeSource -> api.CinderVolumeSource
	if in.Cinder != nil {
		out.Cinder = new(api.CinderVolumeSource)
		if err := Convert_v1_CinderVolumeSource_To_api_CinderVolumeSource(in.Cinder, out.Cinder, s); err != nil {
			return err
		}
	} else {
		out.Cinder = nil
	}
	// unable to generate simple pointer conversion for v1.CephFSVolumeSource -> api.CephFSVolumeSource
	if in.CephFS != nil {
		out.CephFS = new(api.CephFSVolumeSource)
		if err := Convert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource(in.CephFS, out.CephFS, s); err != nil {
			return err
		}
	} else {
		out.CephFS = nil
	}
	// unable to generate simple pointer conversion for v1.FlockerVolumeSource -> api.FlockerVolumeSource
	if in.Flocker != nil {
		out.Flocker = new(api.FlockerVolumeSource)
		if err := Convert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource(in.Flocker, out.Flocker, s); err != nil {
			return err
		}
	} else {
		out.Flocker = nil
	}
	// unable to generate simple pointer conversion for v1.DownwardAPIVolumeSource -> api.DownwardAPIVolumeSource
	if in.DownwardAPI != nil {
		out.DownwardAPI = new(api.DownwardAPIVolumeSource)
		if err := Convert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in.DownwardAPI, out.DownwardAPI, s); err != nil {
			return err
		}
	} else {
		out.DownwardAPI = nil
	}
	// unable to generate simple pointer conversion for v1.FCVolumeSource -> api.FCVolumeSource
	if in.FC != nil {
		out.FC = new(api.FCVolumeSource)
		if err := Convert_v1_FCVolumeSource_To_api_FCVolumeSource(in.FC, out.FC, s); err != nil {
			return err
		}
	} else {
		out.FC = nil
	}
	// unable to generate simple pointer conversion for v1.AzureFileVolumeSource -> api.AzureFileVolumeSource
	if in.AzureFile != nil {
		out.AzureFile = new(api.AzureFileVolumeSource)
		if err := Convert_v1_AzureFileVolumeSource_To_api_AzureFileVolumeSource(in.AzureFile, out.AzureFile, s); err != nil {
			return err
		}
	} else {
		out.AzureFile = nil
	}
	// unable to generate simple pointer conversion for v1.ConfigMapVolumeSource -> api.ConfigMapVolumeSource
	if in.ConfigMap != nil {
		out.ConfigMap = new(api.ConfigMapVolumeSource)
		if err := Convert_v1_ConfigMapVolumeSource_To_api_ConfigMapVolumeSource(in.ConfigMap, out.ConfigMap, s); err != nil {
			return err
		}
	} else {
		out.ConfigMap = nil
	}
	// in.Metadata has no peer in out
	return nil
}

func init() {
	err := api.Scheme.AddGeneratedConversionFuncs(
		autoConvert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource,
		autoConvert_api_AzureFileVolumeSource_To_v1_AzureFileVolumeSource,
		autoConvert_api_BinaryBuildRequestOptions_To_v1_BinaryBuildRequestOptions,
		autoConvert_api_BinaryBuildSource_To_v1_BinaryBuildSource,
		autoConvert_api_BuildConfigList_To_v1_BuildConfigList,
		autoConvert_api_BuildConfigSpec_To_v1_BuildConfigSpec,
		autoConvert_api_BuildConfigStatus_To_v1_BuildConfigStatus,
		autoConvert_api_BuildConfig_To_v1_BuildConfig,
		autoConvert_api_BuildList_To_v1_BuildList,
		autoConvert_api_BuildLogOptions_To_v1_BuildLogOptions,
		autoConvert_api_BuildLog_To_v1_BuildLog,
		autoConvert_api_BuildOutput_To_v1_BuildOutput,
		autoConvert_api_BuildPostCommitSpec_To_v1_BuildPostCommitSpec,
		autoConvert_api_BuildRequest_To_v1_BuildRequest,
		autoConvert_api_BuildSource_To_v1_BuildSource,
		autoConvert_api_BuildSpec_To_v1_BuildSpec,
		autoConvert_api_BuildStatus_To_v1_BuildStatus,
		autoConvert_api_BuildStrategy_To_v1_BuildStrategy,
		autoConvert_api_BuildTriggerPolicy_To_v1_BuildTriggerPolicy,
		autoConvert_api_Build_To_v1_Build,
		autoConvert_api_Capabilities_To_v1_Capabilities,
		autoConvert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource,
		autoConvert_api_CinderVolumeSource_To_v1_CinderVolumeSource,
		autoConvert_api_ClusterNetworkList_To_v1_ClusterNetworkList,
		autoConvert_api_ClusterNetwork_To_v1_ClusterNetwork,
		autoConvert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList,
		autoConvert_api_ClusterPolicyBinding_To_v1_ClusterPolicyBinding,
		autoConvert_api_ClusterPolicyList_To_v1_ClusterPolicyList,
		autoConvert_api_ClusterPolicy_To_v1_ClusterPolicy,
		autoConvert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList,
		autoConvert_api_ClusterRoleBinding_To_v1_ClusterRoleBinding,
		autoConvert_api_ClusterRoleList_To_v1_ClusterRoleList,
		autoConvert_api_ClusterRole_To_v1_ClusterRole,
		autoConvert_api_ConfigMapKeySelector_To_v1_ConfigMapKeySelector,
		autoConvert_api_ConfigMapVolumeSource_To_v1_ConfigMapVolumeSource,
		autoConvert_api_ContainerPort_To_v1_ContainerPort,
		autoConvert_api_Container_To_v1_Container,
		autoConvert_api_CustomBuildStrategy_To_v1_CustomBuildStrategy,
		autoConvert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams,
		autoConvert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger,
		autoConvert_api_DeploymentCause_To_v1_DeploymentCause,
		autoConvert_api_DeploymentConfigList_To_v1_DeploymentConfigList,
		autoConvert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec,
		autoConvert_api_DeploymentConfigRollback_To_v1_DeploymentConfigRollback,
		autoConvert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec,
		autoConvert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus,
		autoConvert_api_DeploymentConfig_To_v1_DeploymentConfig,
		autoConvert_api_DeploymentDetails_To_v1_DeploymentDetails,
		autoConvert_api_DeploymentLogOptions_To_v1_DeploymentLogOptions,
		autoConvert_api_DeploymentLog_To_v1_DeploymentLog,
		autoConvert_api_DeploymentStrategy_To_v1_DeploymentStrategy,
		autoConvert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams,
		autoConvert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy,
		autoConvert_api_DockerBuildStrategy_To_v1_DockerBuildStrategy,
		autoConvert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile,
		autoConvert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource,
		autoConvert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource,
		autoConvert_api_EnvVarSource_To_v1_EnvVarSource,
		autoConvert_api_EnvVar_To_v1_EnvVar,
		autoConvert_api_ExecAction_To_v1_ExecAction,
		autoConvert_api_ExecNewPodHook_To_v1_ExecNewPodHook,
		autoConvert_api_FCVolumeSource_To_v1_FCVolumeSource,
		autoConvert_api_FlexVolumeSource_To_v1_FlexVolumeSource,
		autoConvert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource,
		autoConvert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource,
		autoConvert_api_GitBuildSource_To_v1_GitBuildSource,
		autoConvert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource,
		autoConvert_api_GitSourceRevision_To_v1_GitSourceRevision,
		autoConvert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource,
		autoConvert_api_GroupList_To_v1_GroupList,
		autoConvert_api_Group_To_v1_Group,
		autoConvert_api_HTTPGetAction_To_v1_HTTPGetAction,
		autoConvert_api_HTTPHeader_To_v1_HTTPHeader,
		autoConvert_api_Handler_To_v1_Handler,
		autoConvert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource,
		autoConvert_api_HostSubnetList_To_v1_HostSubnetList,
		autoConvert_api_HostSubnet_To_v1_HostSubnet,
		autoConvert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource,
		autoConvert_api_IdentityList_To_v1_IdentityList,
		autoConvert_api_Identity_To_v1_Identity,
		autoConvert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger,
		autoConvert_api_ImageImportSpec_To_v1_ImageImportSpec,
		autoConvert_api_ImageImportStatus_To_v1_ImageImportStatus,
		autoConvert_api_ImageList_To_v1_ImageList,
		autoConvert_api_ImageSourcePath_To_v1_ImageSourcePath,
		autoConvert_api_ImageSource_To_v1_ImageSource,
		autoConvert_api_ImageStreamImage_To_v1_ImageStreamImage,
		autoConvert_api_ImageStreamImportSpec_To_v1_ImageStreamImportSpec,
		autoConvert_api_ImageStreamImportStatus_To_v1_ImageStreamImportStatus,
		autoConvert_api_ImageStreamImport_To_v1_ImageStreamImport,
		autoConvert_api_ImageStreamList_To_v1_ImageStreamList,
		autoConvert_api_ImageStreamMapping_To_v1_ImageStreamMapping,
		autoConvert_api_ImageStreamSpec_To_v1_ImageStreamSpec,
		autoConvert_api_ImageStreamStatus_To_v1_ImageStreamStatus,
		autoConvert_api_ImageStreamTagList_To_v1_ImageStreamTagList,
		autoConvert_api_ImageStreamTag_To_v1_ImageStreamTag,
		autoConvert_api_ImageStream_To_v1_ImageStream,
		autoConvert_api_Image_To_v1_Image,
		autoConvert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview,
		autoConvert_api_KeyToPath_To_v1_KeyToPath,
		autoConvert_api_LifecycleHook_To_v1_LifecycleHook,
		autoConvert_api_Lifecycle_To_v1_Lifecycle,
		autoConvert_api_LocalObjectReference_To_v1_LocalObjectReference,
		autoConvert_api_LocalResourceAccessReview_To_v1_LocalResourceAccessReview,
		autoConvert_api_LocalSubjectAccessReview_To_v1_LocalSubjectAccessReview,
		autoConvert_api_NFSVolumeSource_To_v1_NFSVolumeSource,
		autoConvert_api_NetNamespaceList_To_v1_NetNamespaceList,
		autoConvert_api_NetNamespace_To_v1_NetNamespace,
		autoConvert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList,
		autoConvert_api_OAuthAccessToken_To_v1_OAuthAccessToken,
		autoConvert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList,
		autoConvert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken,
		autoConvert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList,
		autoConvert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization,
		autoConvert_api_OAuthClientList_To_v1_OAuthClientList,
		autoConvert_api_OAuthClient_To_v1_OAuthClient,
		autoConvert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector,
		autoConvert_api_ObjectMeta_To_v1_ObjectMeta,
		autoConvert_api_ObjectReference_To_v1_ObjectReference,
		autoConvert_api_Parameter_To_v1_Parameter,
		autoConvert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource,
		autoConvert_api_PodSpec_To_v1_PodSpec,
		autoConvert_api_PodTemplateSpec_To_v1_PodTemplateSpec,
		autoConvert_api_PolicyBindingList_To_v1_PolicyBindingList,
		autoConvert_api_PolicyBinding_To_v1_PolicyBinding,
		autoConvert_api_PolicyList_To_v1_PolicyList,
		autoConvert_api_PolicyRule_To_v1_PolicyRule,
		autoConvert_api_Policy_To_v1_Policy,
		autoConvert_api_Probe_To_v1_Probe,
		autoConvert_api_ProjectList_To_v1_ProjectList,
		autoConvert_api_ProjectRequest_To_v1_ProjectRequest,
		autoConvert_api_ProjectSpec_To_v1_ProjectSpec,
		autoConvert_api_ProjectStatus_To_v1_ProjectStatus,
		autoConvert_api_Project_To_v1_Project,
		autoConvert_api_RBDVolumeSource_To_v1_RBDVolumeSource,
		autoConvert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams,
		autoConvert_api_RepositoryImportSpec_To_v1_RepositoryImportSpec,
		autoConvert_api_RepositoryImportStatus_To_v1_RepositoryImportStatus,
		autoConvert_api_ResourceAccessReviewResponse_To_v1_ResourceAccessReviewResponse,
		autoConvert_api_ResourceAccessReview_To_v1_ResourceAccessReview,
		autoConvert_api_ResourceRequirements_To_v1_ResourceRequirements,
		autoConvert_api_RoleBindingList_To_v1_RoleBindingList,
		autoConvert_api_RoleBinding_To_v1_RoleBinding,
		autoConvert_api_RoleList_To_v1_RoleList,
		autoConvert_api_Role_To_v1_Role,
		autoConvert_api_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams,
		autoConvert_api_RouteIngressCondition_To_v1_RouteIngressCondition,
		autoConvert_api_RouteIngress_To_v1_RouteIngress,
		autoConvert_api_RouteList_To_v1_RouteList,
		autoConvert_api_RoutePort_To_v1_RoutePort,
		autoConvert_api_RouteSpec_To_v1_RouteSpec,
		autoConvert_api_RouteStatus_To_v1_RouteStatus,
		autoConvert_api_Route_To_v1_Route,
		autoConvert_api_SELinuxOptions_To_v1_SELinuxOptions,
		autoConvert_api_SecretBuildSource_To_v1_SecretBuildSource,
		autoConvert_api_SecretKeySelector_To_v1_SecretKeySelector,
		autoConvert_api_SecretSpec_To_v1_SecretSpec,
		autoConvert_api_SecretVolumeSource_To_v1_SecretVolumeSource,
		autoConvert_api_SecurityContext_To_v1_SecurityContext,
		autoConvert_api_SourceBuildStrategy_To_v1_SourceBuildStrategy,
		autoConvert_api_SourceControlUser_To_v1_SourceControlUser,
		autoConvert_api_SourceRevision_To_v1_SourceRevision,
		autoConvert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse,
		autoConvert_api_SubjectAccessReview_To_v1_SubjectAccessReview,
		autoConvert_api_TCPSocketAction_To_v1_TCPSocketAction,
		autoConvert_api_TLSConfig_To_v1_TLSConfig,
		autoConvert_api_TagEventCondition_To_v1_TagEventCondition,
		autoConvert_api_TagImageHook_To_v1_TagImageHook,
		autoConvert_api_TagImportPolicy_To_v1_TagImportPolicy,
		autoConvert_api_TagReference_To_v1_TagReference,
		autoConvert_api_TemplateList_To_v1_TemplateList,
		autoConvert_api_Template_To_v1_Template,
		autoConvert_api_UserIdentityMapping_To_v1_UserIdentityMapping,
		autoConvert_api_UserList_To_v1_UserList,
		autoConvert_api_User_To_v1_User,
		autoConvert_api_VolumeMount_To_v1_VolumeMount,
		autoConvert_api_VolumeSource_To_v1_VolumeSource,
		autoConvert_api_Volume_To_v1_Volume,
		autoConvert_api_WebHookTrigger_To_v1_WebHookTrigger,
		autoConvert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource,
		autoConvert_v1_AzureFileVolumeSource_To_api_AzureFileVolumeSource,
		autoConvert_v1_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions,
		autoConvert_v1_BinaryBuildSource_To_api_BinaryBuildSource,
		autoConvert_v1_BuildConfigList_To_api_BuildConfigList,
		autoConvert_v1_BuildConfigSpec_To_api_BuildConfigSpec,
		autoConvert_v1_BuildConfigStatus_To_api_BuildConfigStatus,
		autoConvert_v1_BuildConfig_To_api_BuildConfig,
		autoConvert_v1_BuildList_To_api_BuildList,
		autoConvert_v1_BuildLogOptions_To_api_BuildLogOptions,
		autoConvert_v1_BuildLog_To_api_BuildLog,
		autoConvert_v1_BuildOutput_To_api_BuildOutput,
		autoConvert_v1_BuildPostCommitSpec_To_api_BuildPostCommitSpec,
		autoConvert_v1_BuildRequest_To_api_BuildRequest,
		autoConvert_v1_BuildSource_To_api_BuildSource,
		autoConvert_v1_BuildSpec_To_api_BuildSpec,
		autoConvert_v1_BuildStatus_To_api_BuildStatus,
		autoConvert_v1_BuildStrategy_To_api_BuildStrategy,
		autoConvert_v1_BuildTriggerPolicy_To_api_BuildTriggerPolicy,
		autoConvert_v1_Build_To_api_Build,
		autoConvert_v1_Capabilities_To_api_Capabilities,
		autoConvert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource,
		autoConvert_v1_CinderVolumeSource_To_api_CinderVolumeSource,
		autoConvert_v1_ClusterNetworkList_To_api_ClusterNetworkList,
		autoConvert_v1_ClusterNetwork_To_api_ClusterNetwork,
		autoConvert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList,
		autoConvert_v1_ClusterPolicyBinding_To_api_ClusterPolicyBinding,
		autoConvert_v1_ClusterPolicyList_To_api_ClusterPolicyList,
		autoConvert_v1_ClusterPolicy_To_api_ClusterPolicy,
		autoConvert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList,
		autoConvert_v1_ClusterRoleBinding_To_api_ClusterRoleBinding,
		autoConvert_v1_ClusterRoleList_To_api_ClusterRoleList,
		autoConvert_v1_ClusterRole_To_api_ClusterRole,
		autoConvert_v1_ConfigMapKeySelector_To_api_ConfigMapKeySelector,
		autoConvert_v1_ConfigMapVolumeSource_To_api_ConfigMapVolumeSource,
		autoConvert_v1_ContainerPort_To_api_ContainerPort,
		autoConvert_v1_Container_To_api_Container,
		autoConvert_v1_CustomBuildStrategy_To_api_CustomBuildStrategy,
		autoConvert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams,
		autoConvert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger,
		autoConvert_v1_DeploymentCause_To_api_DeploymentCause,
		autoConvert_v1_DeploymentConfigList_To_api_DeploymentConfigList,
		autoConvert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec,
		autoConvert_v1_DeploymentConfigRollback_To_api_DeploymentConfigRollback,
		autoConvert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec,
		autoConvert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus,
		autoConvert_v1_DeploymentConfig_To_api_DeploymentConfig,
		autoConvert_v1_DeploymentDetails_To_api_DeploymentDetails,
		autoConvert_v1_DeploymentLogOptions_To_api_DeploymentLogOptions,
		autoConvert_v1_DeploymentLog_To_api_DeploymentLog,
		autoConvert_v1_DeploymentStrategy_To_api_DeploymentStrategy,
		autoConvert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams,
		autoConvert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy,
		autoConvert_v1_DockerBuildStrategy_To_api_DockerBuildStrategy,
		autoConvert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile,
		autoConvert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource,
		autoConvert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource,
		autoConvert_v1_EnvVarSource_To_api_EnvVarSource,
		autoConvert_v1_EnvVar_To_api_EnvVar,
		autoConvert_v1_ExecAction_To_api_ExecAction,
		autoConvert_v1_ExecNewPodHook_To_api_ExecNewPodHook,
		autoConvert_v1_FCVolumeSource_To_api_FCVolumeSource,
		autoConvert_v1_FlexVolumeSource_To_api_FlexVolumeSource,
		autoConvert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource,
		autoConvert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource,
		autoConvert_v1_GitBuildSource_To_api_GitBuildSource,
		autoConvert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource,
		autoConvert_v1_GitSourceRevision_To_api_GitSourceRevision,
		autoConvert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource,
		autoConvert_v1_GroupList_To_api_GroupList,
		autoConvert_v1_Group_To_api_Group,
		autoConvert_v1_HTTPGetAction_To_api_HTTPGetAction,
		autoConvert_v1_HTTPHeader_To_api_HTTPHeader,
		autoConvert_v1_Handler_To_api_Handler,
		autoConvert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource,
		autoConvert_v1_HostSubnetList_To_api_HostSubnetList,
		autoConvert_v1_HostSubnet_To_api_HostSubnet,
		autoConvert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource,
		autoConvert_v1_IdentityList_To_api_IdentityList,
		autoConvert_v1_Identity_To_api_Identity,
		autoConvert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger,
		autoConvert_v1_ImageImportSpec_To_api_ImageImportSpec,
		autoConvert_v1_ImageImportStatus_To_api_ImageImportStatus,
		autoConvert_v1_ImageList_To_api_ImageList,
		autoConvert_v1_ImageSourcePath_To_api_ImageSourcePath,
		autoConvert_v1_ImageSource_To_api_ImageSource,
		autoConvert_v1_ImageStreamImage_To_api_ImageStreamImage,
		autoConvert_v1_ImageStreamImportSpec_To_api_ImageStreamImportSpec,
		autoConvert_v1_ImageStreamImportStatus_To_api_ImageStreamImportStatus,
		autoConvert_v1_ImageStreamImport_To_api_ImageStreamImport,
		autoConvert_v1_ImageStreamList_To_api_ImageStreamList,
		autoConvert_v1_ImageStreamMapping_To_api_ImageStreamMapping,
		autoConvert_v1_ImageStreamSpec_To_api_ImageStreamSpec,
		autoConvert_v1_ImageStreamStatus_To_api_ImageStreamStatus,
		autoConvert_v1_ImageStreamTagList_To_api_ImageStreamTagList,
		autoConvert_v1_ImageStreamTag_To_api_ImageStreamTag,
		autoConvert_v1_ImageStream_To_api_ImageStream,
		autoConvert_v1_Image_To_api_Image,
		autoConvert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview,
		autoConvert_v1_KeyToPath_To_api_KeyToPath,
		autoConvert_v1_LifecycleHook_To_api_LifecycleHook,
		autoConvert_v1_Lifecycle_To_api_Lifecycle,
		autoConvert_v1_LocalObjectReference_To_api_LocalObjectReference,
		autoConvert_v1_LocalResourceAccessReview_To_api_LocalResourceAccessReview,
		autoConvert_v1_LocalSubjectAccessReview_To_api_LocalSubjectAccessReview,
		autoConvert_v1_NFSVolumeSource_To_api_NFSVolumeSource,
		autoConvert_v1_NetNamespaceList_To_api_NetNamespaceList,
		autoConvert_v1_NetNamespace_To_api_NetNamespace,
		autoConvert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList,
		autoConvert_v1_OAuthAccessToken_To_api_OAuthAccessToken,
		autoConvert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList,
		autoConvert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken,
		autoConvert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList,
		autoConvert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization,
		autoConvert_v1_OAuthClientList_To_api_OAuthClientList,
		autoConvert_v1_OAuthClient_To_api_OAuthClient,
		autoConvert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector,
		autoConvert_v1_ObjectMeta_To_api_ObjectMeta,
		autoConvert_v1_ObjectReference_To_api_ObjectReference,
		autoConvert_v1_Parameter_To_api_Parameter,
		autoConvert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource,
		autoConvert_v1_PodSpec_To_api_PodSpec,
		autoConvert_v1_PodTemplateSpec_To_api_PodTemplateSpec,
		autoConvert_v1_PolicyBindingList_To_api_PolicyBindingList,
		autoConvert_v1_PolicyBinding_To_api_PolicyBinding,
		autoConvert_v1_PolicyList_To_api_PolicyList,
		autoConvert_v1_PolicyRule_To_api_PolicyRule,
		autoConvert_v1_Policy_To_api_Policy,
		autoConvert_v1_Probe_To_api_Probe,
		autoConvert_v1_ProjectList_To_api_ProjectList,
		autoConvert_v1_ProjectRequest_To_api_ProjectRequest,
		autoConvert_v1_ProjectSpec_To_api_ProjectSpec,
		autoConvert_v1_ProjectStatus_To_api_ProjectStatus,
		autoConvert_v1_Project_To_api_Project,
		autoConvert_v1_RBDVolumeSource_To_api_RBDVolumeSource,
		autoConvert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams,
		autoConvert_v1_RepositoryImportSpec_To_api_RepositoryImportSpec,
		autoConvert_v1_RepositoryImportStatus_To_api_RepositoryImportStatus,
		autoConvert_v1_ResourceAccessReviewResponse_To_api_ResourceAccessReviewResponse,
		autoConvert_v1_ResourceAccessReview_To_api_ResourceAccessReview,
		autoConvert_v1_ResourceRequirements_To_api_ResourceRequirements,
		autoConvert_v1_RoleBindingList_To_api_RoleBindingList,
		autoConvert_v1_RoleBinding_To_api_RoleBinding,
		autoConvert_v1_RoleList_To_api_RoleList,
		autoConvert_v1_Role_To_api_Role,
		autoConvert_v1_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams,
		autoConvert_v1_RouteIngressCondition_To_api_RouteIngressCondition,
		autoConvert_v1_RouteIngress_To_api_RouteIngress,
		autoConvert_v1_RouteList_To_api_RouteList,
		autoConvert_v1_RoutePort_To_api_RoutePort,
		autoConvert_v1_RouteSpec_To_api_RouteSpec,
		autoConvert_v1_RouteStatus_To_api_RouteStatus,
		autoConvert_v1_Route_To_api_Route,
		autoConvert_v1_SELinuxOptions_To_api_SELinuxOptions,
		autoConvert_v1_SecretBuildSource_To_api_SecretBuildSource,
		autoConvert_v1_SecretKeySelector_To_api_SecretKeySelector,
		autoConvert_v1_SecretSpec_To_api_SecretSpec,
		autoConvert_v1_SecretVolumeSource_To_api_SecretVolumeSource,
		autoConvert_v1_SecurityContext_To_api_SecurityContext,
		autoConvert_v1_SourceBuildStrategy_To_api_SourceBuildStrategy,
		autoConvert_v1_SourceControlUser_To_api_SourceControlUser,
		autoConvert_v1_SourceRevision_To_api_SourceRevision,
		autoConvert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse,
		autoConvert_v1_SubjectAccessReview_To_api_SubjectAccessReview,
		autoConvert_v1_TCPSocketAction_To_api_TCPSocketAction,
		autoConvert_v1_TLSConfig_To_api_TLSConfig,
		autoConvert_v1_TagEventCondition_To_api_TagEventCondition,
		autoConvert_v1_TagImageHook_To_api_TagImageHook,
		autoConvert_v1_TagImportPolicy_To_api_TagImportPolicy,
		autoConvert_v1_TagReference_To_api_TagReference,
		autoConvert_v1_TemplateList_To_api_TemplateList,
		autoConvert_v1_Template_To_api_Template,
		autoConvert_v1_UserIdentityMapping_To_api_UserIdentityMapping,
		autoConvert_v1_UserList_To_api_UserList,
		autoConvert_v1_User_To_api_User,
		autoConvert_v1_VolumeMount_To_api_VolumeMount,
		autoConvert_v1_VolumeSource_To_api_VolumeSource,
		autoConvert_v1_Volume_To_api_Volume,
		autoConvert_v1_WebHookTrigger_To_api_WebHookTrigger,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

// AUTO-GENERATED FUNCTIONS END HERE
