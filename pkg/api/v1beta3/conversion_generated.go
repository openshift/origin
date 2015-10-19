package v1beta3

// AUTO-GENERATED FUNCTIONS START HERE
import (
	api "github.com/openshift/origin/pkg/authorization/api"
	v1beta3 "github.com/openshift/origin/pkg/authorization/api/v1beta3"
	buildapi "github.com/openshift/origin/pkg/build/api"
	apiv1beta3 "github.com/openshift/origin/pkg/build/api/v1beta3"
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
	pkgapi "k8s.io/kubernetes/pkg/api"
	resource "k8s.io/kubernetes/pkg/api/resource"
	pkgapiv1beta3 "k8s.io/kubernetes/pkg/api/v1beta3"
	conversion "k8s.io/kubernetes/pkg/conversion"
	reflect "reflect"
)

func autoconvert_api_ClusterPolicy_To_v1beta3_ClusterPolicy(in *api.ClusterPolicy, out *v1beta3.ClusterPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterPolicy))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Roles, &out.Roles, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_ClusterPolicy_To_v1beta3_ClusterPolicy(in *api.ClusterPolicy, out *v1beta3.ClusterPolicy, s conversion.Scope) error {
	return autoconvert_api_ClusterPolicy_To_v1beta3_ClusterPolicy(in, out, s)
}

func autoconvert_api_ClusterPolicyBinding_To_v1beta3_ClusterPolicyBinding(in *api.ClusterPolicyBinding, out *v1beta3.ClusterPolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterPolicyBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ClusterPolicyBindingList_To_v1beta3_ClusterPolicyBindingList(in *api.ClusterPolicyBindingList, out *v1beta3.ClusterPolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterPolicyBindingList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.ClusterPolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ClusterPolicyBindingList_To_v1beta3_ClusterPolicyBindingList(in *api.ClusterPolicyBindingList, out *v1beta3.ClusterPolicyBindingList, s conversion.Scope) error {
	return autoconvert_api_ClusterPolicyBindingList_To_v1beta3_ClusterPolicyBindingList(in, out, s)
}

func autoconvert_api_ClusterPolicyList_To_v1beta3_ClusterPolicyList(in *api.ClusterPolicyList, out *v1beta3.ClusterPolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterPolicyList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.ClusterPolicy, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ClusterPolicyList_To_v1beta3_ClusterPolicyList(in *api.ClusterPolicyList, out *v1beta3.ClusterPolicyList, s conversion.Scope) error {
	return autoconvert_api_ClusterPolicyList_To_v1beta3_ClusterPolicyList(in, out, s)
}

func autoconvert_api_ClusterRole_To_v1beta3_ClusterRole(in *api.ClusterRole, out *v1beta3.ClusterRole, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterRole))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]v1beta3.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := s.Convert(&in.Rules[i], &out.Rules[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func convert_api_ClusterRole_To_v1beta3_ClusterRole(in *api.ClusterRole, out *v1beta3.ClusterRole, s conversion.Scope) error {
	return autoconvert_api_ClusterRole_To_v1beta3_ClusterRole(in, out, s)
}

func autoconvert_api_ClusterRoleBinding_To_v1beta3_ClusterRoleBinding(in *api.ClusterRoleBinding, out *v1beta3.ClusterRoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterRoleBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		out.Subjects = make([]pkgapiv1beta3.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ClusterRoleBindingList_To_v1beta3_ClusterRoleBindingList(in *api.ClusterRoleBindingList, out *v1beta3.ClusterRoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterRoleBindingList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.ClusterRoleBinding, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ClusterRoleBindingList_To_v1beta3_ClusterRoleBindingList(in *api.ClusterRoleBindingList, out *v1beta3.ClusterRoleBindingList, s conversion.Scope) error {
	return autoconvert_api_ClusterRoleBindingList_To_v1beta3_ClusterRoleBindingList(in, out, s)
}

func autoconvert_api_ClusterRoleList_To_v1beta3_ClusterRoleList(in *api.ClusterRoleList, out *v1beta3.ClusterRoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterRoleList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := convert_api_ClusterRole_To_v1beta3_ClusterRole(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ClusterRoleList_To_v1beta3_ClusterRoleList(in *api.ClusterRoleList, out *v1beta3.ClusterRoleList, s conversion.Scope) error {
	return autoconvert_api_ClusterRoleList_To_v1beta3_ClusterRoleList(in, out, s)
}

func autoconvert_api_IsPersonalSubjectAccessReview_To_v1beta3_IsPersonalSubjectAccessReview(in *api.IsPersonalSubjectAccessReview, out *v1beta3.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.IsPersonalSubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_IsPersonalSubjectAccessReview_To_v1beta3_IsPersonalSubjectAccessReview(in *api.IsPersonalSubjectAccessReview, out *v1beta3.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoconvert_api_IsPersonalSubjectAccessReview_To_v1beta3_IsPersonalSubjectAccessReview(in, out, s)
}

func autoconvert_api_LocalResourceAccessReview_To_v1beta3_LocalResourceAccessReview(in *api.LocalResourceAccessReview, out *v1beta3.LocalResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.LocalResourceAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.Action has no peer in out
	return nil
}

func autoconvert_api_LocalSubjectAccessReview_To_v1beta3_LocalSubjectAccessReview(in *api.LocalSubjectAccessReview, out *v1beta3.LocalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.LocalSubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.Action has no peer in out
	out.User = in.User
	// in.Groups has no peer in out
	return nil
}

func autoconvert_api_Policy_To_v1beta3_Policy(in *api.Policy, out *v1beta3.Policy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Policy))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Roles, &out.Roles, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_PolicyBinding_To_v1beta3_PolicyBinding(in *api.PolicyBinding, out *v1beta3.PolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PolicyBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_PolicyBindingList_To_v1beta3_PolicyBindingList(in *api.PolicyBindingList, out *v1beta3.PolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PolicyBindingList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.PolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_PolicyBindingList_To_v1beta3_PolicyBindingList(in *api.PolicyBindingList, out *v1beta3.PolicyBindingList, s conversion.Scope) error {
	return autoconvert_api_PolicyBindingList_To_v1beta3_PolicyBindingList(in, out, s)
}

func autoconvert_api_PolicyList_To_v1beta3_PolicyList(in *api.PolicyList, out *v1beta3.PolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PolicyList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.Policy, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_PolicyList_To_v1beta3_PolicyList(in *api.PolicyList, out *v1beta3.PolicyList, s conversion.Scope) error {
	return autoconvert_api_PolicyList_To_v1beta3_PolicyList(in, out, s)
}

func autoconvert_api_PolicyRule_To_v1beta3_PolicyRule(in *api.PolicyRule, out *v1beta3.PolicyRule, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PolicyRule))(in)
	}
	// in.Verbs has no peer in out
	if err := s.Convert(&in.AttributeRestrictions, &out.AttributeRestrictions, 0); err != nil {
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

func autoconvert_api_ResourceAccessReview_To_v1beta3_ResourceAccessReview(in *api.ResourceAccessReview, out *v1beta3.ResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ResourceAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.Action has no peer in out
	return nil
}

func autoconvert_api_ResourceAccessReviewResponse_To_v1beta3_ResourceAccessReviewResponse(in *api.ResourceAccessReviewResponse, out *v1beta3.ResourceAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ResourceAccessReviewResponse))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Namespace = in.Namespace
	// in.Users has no peer in out
	// in.Groups has no peer in out
	return nil
}

func autoconvert_api_Role_To_v1beta3_Role(in *api.Role, out *v1beta3.Role, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Role))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]v1beta3.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := s.Convert(&in.Rules[i], &out.Rules[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func convert_api_Role_To_v1beta3_Role(in *api.Role, out *v1beta3.Role, s conversion.Scope) error {
	return autoconvert_api_Role_To_v1beta3_Role(in, out, s)
}

func autoconvert_api_RoleBinding_To_v1beta3_RoleBinding(in *api.RoleBinding, out *v1beta3.RoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.RoleBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		out.Subjects = make([]pkgapiv1beta3.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_RoleBindingList_To_v1beta3_RoleBindingList(in *api.RoleBindingList, out *v1beta3.RoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.RoleBindingList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.RoleBinding, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_RoleBindingList_To_v1beta3_RoleBindingList(in *api.RoleBindingList, out *v1beta3.RoleBindingList, s conversion.Scope) error {
	return autoconvert_api_RoleBindingList_To_v1beta3_RoleBindingList(in, out, s)
}

func autoconvert_api_RoleList_To_v1beta3_RoleList(in *api.RoleList, out *v1beta3.RoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.RoleList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]v1beta3.Role, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Role_To_v1beta3_Role(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_RoleList_To_v1beta3_RoleList(in *api.RoleList, out *v1beta3.RoleList, s conversion.Scope) error {
	return autoconvert_api_RoleList_To_v1beta3_RoleList(in, out, s)
}

func autoconvert_api_SubjectAccessReview_To_v1beta3_SubjectAccessReview(in *api.SubjectAccessReview, out *v1beta3.SubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.SubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.Action has no peer in out
	out.User = in.User
	// in.Groups has no peer in out
	return nil
}

func autoconvert_api_SubjectAccessReviewResponse_To_v1beta3_SubjectAccessReviewResponse(in *api.SubjectAccessReviewResponse, out *v1beta3.SubjectAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.SubjectAccessReviewResponse))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func convert_api_SubjectAccessReviewResponse_To_v1beta3_SubjectAccessReviewResponse(in *api.SubjectAccessReviewResponse, out *v1beta3.SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoconvert_api_SubjectAccessReviewResponse_To_v1beta3_SubjectAccessReviewResponse(in, out, s)
}

func autoconvert_v1beta3_ClusterPolicy_To_api_ClusterPolicy(in *v1beta3.ClusterPolicy, out *api.ClusterPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ClusterPolicy))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Roles, &out.Roles, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_ClusterPolicy_To_api_ClusterPolicy(in *v1beta3.ClusterPolicy, out *api.ClusterPolicy, s conversion.Scope) error {
	return autoconvert_v1beta3_ClusterPolicy_To_api_ClusterPolicy(in, out, s)
}

func autoconvert_v1beta3_ClusterPolicyBinding_To_api_ClusterPolicyBinding(in *v1beta3.ClusterPolicyBinding, out *api.ClusterPolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ClusterPolicyBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1beta3_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *v1beta3.ClusterPolicyBindingList, out *api.ClusterPolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ClusterPolicyBindingList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]api.ClusterPolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *v1beta3.ClusterPolicyBindingList, out *api.ClusterPolicyBindingList, s conversion.Scope) error {
	return autoconvert_v1beta3_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in, out, s)
}

func autoconvert_v1beta3_ClusterPolicyList_To_api_ClusterPolicyList(in *v1beta3.ClusterPolicyList, out *api.ClusterPolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ClusterPolicyList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]api.ClusterPolicy, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_ClusterPolicy_To_api_ClusterPolicy(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_ClusterPolicyList_To_api_ClusterPolicyList(in *v1beta3.ClusterPolicyList, out *api.ClusterPolicyList, s conversion.Scope) error {
	return autoconvert_v1beta3_ClusterPolicyList_To_api_ClusterPolicyList(in, out, s)
}

func autoconvert_v1beta3_ClusterRole_To_api_ClusterRole(in *v1beta3.ClusterRole, out *api.ClusterRole, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ClusterRole))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]api.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := s.Convert(&in.Rules[i], &out.Rules[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func convert_v1beta3_ClusterRole_To_api_ClusterRole(in *v1beta3.ClusterRole, out *api.ClusterRole, s conversion.Scope) error {
	return autoconvert_v1beta3_ClusterRole_To_api_ClusterRole(in, out, s)
}

func autoconvert_v1beta3_ClusterRoleBinding_To_api_ClusterRoleBinding(in *v1beta3.ClusterRoleBinding, out *api.ClusterRoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ClusterRoleBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.UserNames has no peer in out
	// in.GroupNames has no peer in out
	if in.Subjects != nil {
		out.Subjects = make([]pkgapi.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1beta3_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *v1beta3.ClusterRoleBindingList, out *api.ClusterRoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ClusterRoleBindingList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]api.ClusterRoleBinding, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *v1beta3.ClusterRoleBindingList, out *api.ClusterRoleBindingList, s conversion.Scope) error {
	return autoconvert_v1beta3_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in, out, s)
}

func autoconvert_v1beta3_ClusterRoleList_To_api_ClusterRoleList(in *v1beta3.ClusterRoleList, out *api.ClusterRoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ClusterRoleList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]api.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_ClusterRole_To_api_ClusterRole(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_ClusterRoleList_To_api_ClusterRoleList(in *v1beta3.ClusterRoleList, out *api.ClusterRoleList, s conversion.Scope) error {
	return autoconvert_v1beta3_ClusterRoleList_To_api_ClusterRoleList(in, out, s)
}

func autoconvert_v1beta3_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *v1beta3.IsPersonalSubjectAccessReview, out *api.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.IsPersonalSubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *v1beta3.IsPersonalSubjectAccessReview, out *api.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoconvert_v1beta3_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in, out, s)
}

func autoconvert_v1beta3_LocalResourceAccessReview_To_api_LocalResourceAccessReview(in *v1beta3.LocalResourceAccessReview, out *api.LocalResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.LocalResourceAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.AuthorizationAttributes has no peer in out
	return nil
}

func autoconvert_v1beta3_LocalSubjectAccessReview_To_api_LocalSubjectAccessReview(in *v1beta3.LocalSubjectAccessReview, out *api.LocalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.LocalSubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.AuthorizationAttributes has no peer in out
	out.User = in.User
	// in.GroupsSlice has no peer in out
	return nil
}

func autoconvert_v1beta3_Policy_To_api_Policy(in *v1beta3.Policy, out *api.Policy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.Policy))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Roles, &out.Roles, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1beta3_PolicyBinding_To_api_PolicyBinding(in *v1beta3.PolicyBinding, out *api.PolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.PolicyBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1beta3_PolicyBindingList_To_api_PolicyBindingList(in *v1beta3.PolicyBindingList, out *api.PolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.PolicyBindingList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]api.PolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_PolicyBindingList_To_api_PolicyBindingList(in *v1beta3.PolicyBindingList, out *api.PolicyBindingList, s conversion.Scope) error {
	return autoconvert_v1beta3_PolicyBindingList_To_api_PolicyBindingList(in, out, s)
}

func autoconvert_v1beta3_PolicyList_To_api_PolicyList(in *v1beta3.PolicyList, out *api.PolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.PolicyList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]api.Policy, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_PolicyList_To_api_PolicyList(in *v1beta3.PolicyList, out *api.PolicyList, s conversion.Scope) error {
	return autoconvert_v1beta3_PolicyList_To_api_PolicyList(in, out, s)
}

func autoconvert_v1beta3_PolicyRule_To_api_PolicyRule(in *v1beta3.PolicyRule, out *api.PolicyRule, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.PolicyRule))(in)
	}
	// in.Verbs has no peer in out
	if err := s.Convert(&in.AttributeRestrictions, &out.AttributeRestrictions, 0); err != nil {
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

func autoconvert_v1beta3_ResourceAccessReview_To_api_ResourceAccessReview(in *v1beta3.ResourceAccessReview, out *api.ResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ResourceAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.AuthorizationAttributes has no peer in out
	return nil
}

func autoconvert_v1beta3_ResourceAccessReviewResponse_To_api_ResourceAccessReviewResponse(in *v1beta3.ResourceAccessReviewResponse, out *api.ResourceAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.ResourceAccessReviewResponse))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Namespace = in.Namespace
	// in.UsersSlice has no peer in out
	// in.GroupsSlice has no peer in out
	return nil
}

func autoconvert_v1beta3_Role_To_api_Role(in *v1beta3.Role, out *api.Role, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.Role))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]api.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := s.Convert(&in.Rules[i], &out.Rules[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func convert_v1beta3_Role_To_api_Role(in *v1beta3.Role, out *api.Role, s conversion.Scope) error {
	return autoconvert_v1beta3_Role_To_api_Role(in, out, s)
}

func autoconvert_v1beta3_RoleBinding_To_api_RoleBinding(in *v1beta3.RoleBinding, out *api.RoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.RoleBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.UserNames has no peer in out
	// in.GroupNames has no peer in out
	if in.Subjects != nil {
		out.Subjects = make([]pkgapi.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1beta3_RoleBindingList_To_api_RoleBindingList(in *v1beta3.RoleBindingList, out *api.RoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.RoleBindingList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]api.RoleBinding, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_RoleBindingList_To_api_RoleBindingList(in *v1beta3.RoleBindingList, out *api.RoleBindingList, s conversion.Scope) error {
	return autoconvert_v1beta3_RoleBindingList_To_api_RoleBindingList(in, out, s)
}

func autoconvert_v1beta3_RoleList_To_api_RoleList(in *v1beta3.RoleList, out *api.RoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.RoleList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]api.Role, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_Role_To_api_Role(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_RoleList_To_api_RoleList(in *v1beta3.RoleList, out *api.RoleList, s conversion.Scope) error {
	return autoconvert_v1beta3_RoleList_To_api_RoleList(in, out, s)
}

func autoconvert_v1beta3_SubjectAccessReview_To_api_SubjectAccessReview(in *v1beta3.SubjectAccessReview, out *api.SubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.SubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.AuthorizationAttributes has no peer in out
	out.User = in.User
	// in.GroupsSlice has no peer in out
	return nil
}

func autoconvert_v1beta3_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *v1beta3.SubjectAccessReviewResponse, out *api.SubjectAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1beta3.SubjectAccessReviewResponse))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func convert_v1beta3_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *v1beta3.SubjectAccessReviewResponse, out *api.SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoconvert_v1beta3_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in, out, s)
}

func autoconvert_api_BinaryBuildRequestOptions_To_v1beta3_BinaryBuildRequestOptions(in *buildapi.BinaryBuildRequestOptions, out *apiv1beta3.BinaryBuildRequestOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BinaryBuildRequestOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_BinaryBuildRequestOptions_To_v1beta3_BinaryBuildRequestOptions(in *buildapi.BinaryBuildRequestOptions, out *apiv1beta3.BinaryBuildRequestOptions, s conversion.Scope) error {
	return autoconvert_api_BinaryBuildRequestOptions_To_v1beta3_BinaryBuildRequestOptions(in, out, s)
}

func autoconvert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource(in *buildapi.BinaryBuildSource, out *apiv1beta3.BinaryBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BinaryBuildSource))(in)
	}
	out.AsFile = in.AsFile
	return nil
}

func convert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource(in *buildapi.BinaryBuildSource, out *apiv1beta3.BinaryBuildSource, s conversion.Scope) error {
	return autoconvert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource(in, out, s)
}

func autoconvert_api_Build_To_v1beta3_Build(in *buildapi.Build, out *apiv1beta3.Build, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.Build))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_BuildSpec_To_v1beta3_BuildSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_api_BuildStatus_To_v1beta3_BuildStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_api_Build_To_v1beta3_Build(in *buildapi.Build, out *apiv1beta3.Build, s conversion.Scope) error {
	return autoconvert_api_Build_To_v1beta3_Build(in, out, s)
}

func autoconvert_api_BuildConfig_To_v1beta3_BuildConfig(in *buildapi.BuildConfig, out *apiv1beta3.BuildConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfig))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_api_BuildConfig_To_v1beta3_BuildConfig(in *buildapi.BuildConfig, out *apiv1beta3.BuildConfig, s conversion.Scope) error {
	return autoconvert_api_BuildConfig_To_v1beta3_BuildConfig(in, out, s)
}

func autoconvert_api_BuildConfigList_To_v1beta3_BuildConfigList(in *buildapi.BuildConfigList, out *apiv1beta3.BuildConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := convert_api_BuildConfig_To_v1beta3_BuildConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_BuildConfigList_To_v1beta3_BuildConfigList(in *buildapi.BuildConfigList, out *apiv1beta3.BuildConfigList, s conversion.Scope) error {
	return autoconvert_api_BuildConfigList_To_v1beta3_BuildConfigList(in, out, s)
}

func autoconvert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec(in *buildapi.BuildConfigSpec, out *apiv1beta3.BuildConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigSpec))(in)
	}
	if in.Triggers != nil {
		out.Triggers = make([]apiv1beta3.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := s.Convert(&in.Triggers[i], &out.Triggers[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := convert_api_BuildSpec_To_v1beta3_BuildSpec(&in.BuildSpec, &out.BuildSpec, s); err != nil {
		return err
	}
	return nil
}

func convert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec(in *buildapi.BuildConfigSpec, out *apiv1beta3.BuildConfigSpec, s conversion.Scope) error {
	return autoconvert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec(in, out, s)
}

func autoconvert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus(in *buildapi.BuildConfigStatus, out *apiv1beta3.BuildConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigStatus))(in)
	}
	out.LastVersion = in.LastVersion
	return nil
}

func convert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus(in *buildapi.BuildConfigStatus, out *apiv1beta3.BuildConfigStatus, s conversion.Scope) error {
	return autoconvert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus(in, out, s)
}

func autoconvert_api_BuildList_To_v1beta3_BuildList(in *buildapi.BuildList, out *apiv1beta3.BuildList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.Build, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Build_To_v1beta3_Build(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_BuildList_To_v1beta3_BuildList(in *buildapi.BuildList, out *apiv1beta3.BuildList, s conversion.Scope) error {
	return autoconvert_api_BuildList_To_v1beta3_BuildList(in, out, s)
}

func autoconvert_api_BuildLog_To_v1beta3_BuildLog(in *buildapi.BuildLog, out *apiv1beta3.BuildLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildLog))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_BuildLog_To_v1beta3_BuildLog(in *buildapi.BuildLog, out *apiv1beta3.BuildLog, s conversion.Scope) error {
	return autoconvert_api_BuildLog_To_v1beta3_BuildLog(in, out, s)
}

func autoconvert_api_BuildLogOptions_To_v1beta3_BuildLogOptions(in *buildapi.BuildLogOptions, out *apiv1beta3.BuildLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildLogOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Follow = in.Follow
	out.NoWait = in.NoWait
	return nil
}

func convert_api_BuildLogOptions_To_v1beta3_BuildLogOptions(in *buildapi.BuildLogOptions, out *apiv1beta3.BuildLogOptions, s conversion.Scope) error {
	return autoconvert_api_BuildLogOptions_To_v1beta3_BuildLogOptions(in, out, s)
}

func autoconvert_api_BuildOutput_To_v1beta3_BuildOutput(in *buildapi.BuildOutput, out *apiv1beta3.BuildOutput, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildOutput))(in)
	}
	if in.To != nil {
		out.To = new(pkgapiv1beta3.ObjectReference)
		if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	if in.PushSecret != nil {
		out.PushSecret = new(pkgapiv1beta3.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.PushSecret, out.PushSecret, s); err != nil {
			return err
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func autoconvert_api_BuildRequest_To_v1beta3_BuildRequest(in *buildapi.BuildRequest, out *apiv1beta3.BuildRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildRequest))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Revision != nil {
		out.Revision = new(apiv1beta3.SourceRevision)
		if err := convert_api_SourceRevision_To_v1beta3_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if in.TriggeredByImage != nil {
		out.TriggeredByImage = new(pkgapiv1beta3.ObjectReference)
		if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(in.TriggeredByImage, out.TriggeredByImage, s); err != nil {
			return err
		}
	} else {
		out.TriggeredByImage = nil
	}
	if in.From != nil {
		out.From = new(pkgapiv1beta3.ObjectReference)
		if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.Binary != nil {
		out.Binary = new(apiv1beta3.BinaryBuildSource)
		if err := convert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
	return nil
}

func convert_api_BuildRequest_To_v1beta3_BuildRequest(in *buildapi.BuildRequest, out *apiv1beta3.BuildRequest, s conversion.Scope) error {
	return autoconvert_api_BuildRequest_To_v1beta3_BuildRequest(in, out, s)
}

func autoconvert_api_BuildSource_To_v1beta3_BuildSource(in *buildapi.BuildSource, out *apiv1beta3.BuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildSource))(in)
	}
	out.Type = apiv1beta3.BuildSourceType(in.Type)
	if in.Binary != nil {
		out.Binary = new(apiv1beta3.BinaryBuildSource)
		if err := convert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
	if in.Git != nil {
		out.Git = new(apiv1beta3.GitBuildSource)
		if err := convert_api_GitBuildSource_To_v1beta3_GitBuildSource(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	out.ContextDir = in.ContextDir
	if in.SourceSecret != nil {
		out.SourceSecret = new(pkgapiv1beta3.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.SourceSecret, out.SourceSecret, s); err != nil {
			return err
		}
	} else {
		out.SourceSecret = nil
	}
	return nil
}

func convert_api_BuildSource_To_v1beta3_BuildSource(in *buildapi.BuildSource, out *apiv1beta3.BuildSource, s conversion.Scope) error {
	return autoconvert_api_BuildSource_To_v1beta3_BuildSource(in, out, s)
}

func autoconvert_api_BuildSpec_To_v1beta3_BuildSpec(in *buildapi.BuildSpec, out *apiv1beta3.BuildSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildSpec))(in)
	}
	out.ServiceAccount = in.ServiceAccount
	if err := convert_api_BuildSource_To_v1beta3_BuildSource(&in.Source, &out.Source, s); err != nil {
		return err
	}
	if in.Revision != nil {
		out.Revision = new(apiv1beta3.SourceRevision)
		if err := convert_api_SourceRevision_To_v1beta3_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := convert_api_BuildStrategy_To_v1beta3_BuildStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Output, &out.Output, 0); err != nil {
		return err
	}
	if err := convert_api_ResourceRequirements_To_v1beta3_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
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

func convert_api_BuildSpec_To_v1beta3_BuildSpec(in *buildapi.BuildSpec, out *apiv1beta3.BuildSpec, s conversion.Scope) error {
	return autoconvert_api_BuildSpec_To_v1beta3_BuildSpec(in, out, s)
}

func autoconvert_api_BuildStatus_To_v1beta3_BuildStatus(in *buildapi.BuildStatus, out *apiv1beta3.BuildStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildStatus))(in)
	}
	out.Phase = apiv1beta3.BuildPhase(in.Phase)
	out.Cancelled = in.Cancelled
	out.Reason = apiv1beta3.StatusReason(in.Reason)
	out.Message = in.Message
	if in.StartTimestamp != nil {
		if err := s.Convert(&in.StartTimestamp, &out.StartTimestamp, 0); err != nil {
			return err
		}
	} else {
		out.StartTimestamp = nil
	}
	if in.CompletionTimestamp != nil {
		if err := s.Convert(&in.CompletionTimestamp, &out.CompletionTimestamp, 0); err != nil {
			return err
		}
	} else {
		out.CompletionTimestamp = nil
	}
	out.Duration = in.Duration
	out.OutputDockerImageReference = in.OutputDockerImageReference
	if in.Config != nil {
		out.Config = new(pkgapiv1beta3.ObjectReference)
		if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(in.Config, out.Config, s); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	return nil
}

func convert_api_BuildStatus_To_v1beta3_BuildStatus(in *buildapi.BuildStatus, out *apiv1beta3.BuildStatus, s conversion.Scope) error {
	return autoconvert_api_BuildStatus_To_v1beta3_BuildStatus(in, out, s)
}

func autoconvert_api_BuildStrategy_To_v1beta3_BuildStrategy(in *buildapi.BuildStrategy, out *apiv1beta3.BuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildStrategy))(in)
	}
	out.Type = apiv1beta3.BuildStrategyType(in.Type)
	if in.DockerStrategy != nil {
		if err := s.Convert(&in.DockerStrategy, &out.DockerStrategy, 0); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	if in.SourceStrategy != nil {
		if err := s.Convert(&in.SourceStrategy, &out.SourceStrategy, 0); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	if in.CustomStrategy != nil {
		if err := s.Convert(&in.CustomStrategy, &out.CustomStrategy, 0); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	return nil
}

func convert_api_BuildStrategy_To_v1beta3_BuildStrategy(in *buildapi.BuildStrategy, out *apiv1beta3.BuildStrategy, s conversion.Scope) error {
	return autoconvert_api_BuildStrategy_To_v1beta3_BuildStrategy(in, out, s)
}

func autoconvert_api_BuildTriggerPolicy_To_v1beta3_BuildTriggerPolicy(in *buildapi.BuildTriggerPolicy, out *apiv1beta3.BuildTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildTriggerPolicy))(in)
	}
	out.Type = apiv1beta3.BuildTriggerType(in.Type)
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(apiv1beta3.WebHookTrigger)
		if err := convert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in.GitHubWebHook, out.GitHubWebHook, s); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(apiv1beta3.WebHookTrigger)
		if err := convert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in.GenericWebHook, out.GenericWebHook, s); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	if in.ImageChange != nil {
		out.ImageChange = new(apiv1beta3.ImageChangeTrigger)
		if err := convert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger(in.ImageChange, out.ImageChange, s); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func autoconvert_api_CustomBuildStrategy_To_v1beta3_CustomBuildStrategy(in *buildapi.CustomBuildStrategy, out *apiv1beta3.CustomBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.CustomBuildStrategy))(in)
	}
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapiv1beta3.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapiv1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1beta3_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ExposeDockerSocket = in.ExposeDockerSocket
	out.ForcePull = in.ForcePull
	if in.Secrets != nil {
		out.Secrets = make([]apiv1beta3.SecretSpec, len(in.Secrets))
		for i := range in.Secrets {
			if err := convert_api_SecretSpec_To_v1beta3_SecretSpec(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func autoconvert_api_DockerBuildStrategy_To_v1beta3_DockerBuildStrategy(in *buildapi.DockerBuildStrategy, out *apiv1beta3.DockerBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.DockerBuildStrategy))(in)
	}
	if in.From != nil {
		out.From = new(pkgapiv1beta3.ObjectReference)
		if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapiv1beta3.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]pkgapiv1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1beta3_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ForcePull = in.ForcePull
	return nil
}

func autoconvert_api_GitBuildSource_To_v1beta3_GitBuildSource(in *buildapi.GitBuildSource, out *apiv1beta3.GitBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.GitBuildSource))(in)
	}
	out.URI = in.URI
	out.Ref = in.Ref
	out.HTTPProxy = in.HTTPProxy
	out.HTTPSProxy = in.HTTPSProxy
	return nil
}

func convert_api_GitBuildSource_To_v1beta3_GitBuildSource(in *buildapi.GitBuildSource, out *apiv1beta3.GitBuildSource, s conversion.Scope) error {
	return autoconvert_api_GitBuildSource_To_v1beta3_GitBuildSource(in, out, s)
}

func autoconvert_api_GitSourceRevision_To_v1beta3_GitSourceRevision(in *buildapi.GitSourceRevision, out *apiv1beta3.GitSourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.GitSourceRevision))(in)
	}
	out.Commit = in.Commit
	if err := convert_api_SourceControlUser_To_v1beta3_SourceControlUser(&in.Author, &out.Author, s); err != nil {
		return err
	}
	if err := convert_api_SourceControlUser_To_v1beta3_SourceControlUser(&in.Committer, &out.Committer, s); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func convert_api_GitSourceRevision_To_v1beta3_GitSourceRevision(in *buildapi.GitSourceRevision, out *apiv1beta3.GitSourceRevision, s conversion.Scope) error {
	return autoconvert_api_GitSourceRevision_To_v1beta3_GitSourceRevision(in, out, s)
}

func autoconvert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger(in *buildapi.ImageChangeTrigger, out *apiv1beta3.ImageChangeTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageChangeTrigger))(in)
	}
	out.LastTriggeredImageID = in.LastTriggeredImageID
	if in.From != nil {
		out.From = new(pkgapiv1beta3.ObjectReference)
		if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	return nil
}

func convert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger(in *buildapi.ImageChangeTrigger, out *apiv1beta3.ImageChangeTrigger, s conversion.Scope) error {
	return autoconvert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger(in, out, s)
}

func autoconvert_api_SecretSpec_To_v1beta3_SecretSpec(in *buildapi.SecretSpec, out *apiv1beta3.SecretSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SecretSpec))(in)
	}
	if err := convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(&in.SecretSource, &out.SecretSource, s); err != nil {
		return err
	}
	out.MountPath = in.MountPath
	return nil
}

func convert_api_SecretSpec_To_v1beta3_SecretSpec(in *buildapi.SecretSpec, out *apiv1beta3.SecretSpec, s conversion.Scope) error {
	return autoconvert_api_SecretSpec_To_v1beta3_SecretSpec(in, out, s)
}

func autoconvert_api_SourceBuildStrategy_To_v1beta3_SourceBuildStrategy(in *buildapi.SourceBuildStrategy, out *apiv1beta3.SourceBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceBuildStrategy))(in)
	}
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapiv1beta3.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapiv1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1beta3_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoconvert_api_SourceControlUser_To_v1beta3_SourceControlUser(in *buildapi.SourceControlUser, out *apiv1beta3.SourceControlUser, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceControlUser))(in)
	}
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func convert_api_SourceControlUser_To_v1beta3_SourceControlUser(in *buildapi.SourceControlUser, out *apiv1beta3.SourceControlUser, s conversion.Scope) error {
	return autoconvert_api_SourceControlUser_To_v1beta3_SourceControlUser(in, out, s)
}

func autoconvert_api_SourceRevision_To_v1beta3_SourceRevision(in *buildapi.SourceRevision, out *apiv1beta3.SourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceRevision))(in)
	}
	out.Type = apiv1beta3.BuildSourceType(in.Type)
	if in.Git != nil {
		out.Git = new(apiv1beta3.GitSourceRevision)
		if err := convert_api_GitSourceRevision_To_v1beta3_GitSourceRevision(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func convert_api_SourceRevision_To_v1beta3_SourceRevision(in *buildapi.SourceRevision, out *apiv1beta3.SourceRevision, s conversion.Scope) error {
	return autoconvert_api_SourceRevision_To_v1beta3_SourceRevision(in, out, s)
}

func autoconvert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in *buildapi.WebHookTrigger, out *apiv1beta3.WebHookTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.WebHookTrigger))(in)
	}
	out.Secret = in.Secret
	return nil
}

func convert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in *buildapi.WebHookTrigger, out *apiv1beta3.WebHookTrigger, s conversion.Scope) error {
	return autoconvert_api_WebHookTrigger_To_v1beta3_WebHookTrigger(in, out, s)
}

func autoconvert_v1beta3_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in *apiv1beta3.BinaryBuildRequestOptions, out *buildapi.BinaryBuildRequestOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BinaryBuildRequestOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1beta3_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in *apiv1beta3.BinaryBuildRequestOptions, out *buildapi.BinaryBuildRequestOptions, s conversion.Scope) error {
	return autoconvert_v1beta3_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in, out, s)
}

func autoconvert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource(in *apiv1beta3.BinaryBuildSource, out *buildapi.BinaryBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BinaryBuildSource))(in)
	}
	out.AsFile = in.AsFile
	return nil
}

func convert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource(in *apiv1beta3.BinaryBuildSource, out *buildapi.BinaryBuildSource, s conversion.Scope) error {
	return autoconvert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource(in, out, s)
}

func autoconvert_v1beta3_Build_To_api_Build(in *apiv1beta3.Build, out *buildapi.Build, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.Build))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1beta3_BuildSpec_To_api_BuildSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1beta3_BuildStatus_To_api_BuildStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_Build_To_api_Build(in *apiv1beta3.Build, out *buildapi.Build, s conversion.Scope) error {
	return autoconvert_v1beta3_Build_To_api_Build(in, out, s)
}

func autoconvert_v1beta3_BuildConfig_To_api_BuildConfig(in *apiv1beta3.BuildConfig, out *buildapi.BuildConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildConfig))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_BuildConfig_To_api_BuildConfig(in *apiv1beta3.BuildConfig, out *buildapi.BuildConfig, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildConfig_To_api_BuildConfig(in, out, s)
}

func autoconvert_v1beta3_BuildConfigList_To_api_BuildConfigList(in *apiv1beta3.BuildConfigList, out *buildapi.BuildConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildConfigList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]buildapi.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_BuildConfig_To_api_BuildConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_BuildConfigList_To_api_BuildConfigList(in *apiv1beta3.BuildConfigList, out *buildapi.BuildConfigList, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildConfigList_To_api_BuildConfigList(in, out, s)
}

func autoconvert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec(in *apiv1beta3.BuildConfigSpec, out *buildapi.BuildConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildConfigSpec))(in)
	}
	if in.Triggers != nil {
		out.Triggers = make([]buildapi.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := s.Convert(&in.Triggers[i], &out.Triggers[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := convert_v1beta3_BuildSpec_To_api_BuildSpec(&in.BuildSpec, &out.BuildSpec, s); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec(in *apiv1beta3.BuildConfigSpec, out *buildapi.BuildConfigSpec, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec(in, out, s)
}

func autoconvert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus(in *apiv1beta3.BuildConfigStatus, out *buildapi.BuildConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildConfigStatus))(in)
	}
	out.LastVersion = in.LastVersion
	return nil
}

func convert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus(in *apiv1beta3.BuildConfigStatus, out *buildapi.BuildConfigStatus, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus(in, out, s)
}

func autoconvert_v1beta3_BuildList_To_api_BuildList(in *apiv1beta3.BuildList, out *buildapi.BuildList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]buildapi.Build, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_Build_To_api_Build(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_BuildList_To_api_BuildList(in *apiv1beta3.BuildList, out *buildapi.BuildList, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildList_To_api_BuildList(in, out, s)
}

func autoconvert_v1beta3_BuildLog_To_api_BuildLog(in *apiv1beta3.BuildLog, out *buildapi.BuildLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildLog))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_BuildLog_To_api_BuildLog(in *apiv1beta3.BuildLog, out *buildapi.BuildLog, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildLog_To_api_BuildLog(in, out, s)
}

func autoconvert_v1beta3_BuildLogOptions_To_api_BuildLogOptions(in *apiv1beta3.BuildLogOptions, out *buildapi.BuildLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildLogOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Follow = in.Follow
	out.NoWait = in.NoWait
	return nil
}

func convert_v1beta3_BuildLogOptions_To_api_BuildLogOptions(in *apiv1beta3.BuildLogOptions, out *buildapi.BuildLogOptions, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildLogOptions_To_api_BuildLogOptions(in, out, s)
}

func autoconvert_v1beta3_BuildOutput_To_api_BuildOutput(in *apiv1beta3.BuildOutput, out *buildapi.BuildOutput, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildOutput))(in)
	}
	if in.To != nil {
		out.To = new(pkgapi.ObjectReference)
		if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	if in.PushSecret != nil {
		out.PushSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.PushSecret, out.PushSecret, s); err != nil {
			return err
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func autoconvert_v1beta3_BuildRequest_To_api_BuildRequest(in *apiv1beta3.BuildRequest, out *buildapi.BuildRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildRequest))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Revision != nil {
		out.Revision = new(buildapi.SourceRevision)
		if err := convert_v1beta3_SourceRevision_To_api_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if in.TriggeredByImage != nil {
		out.TriggeredByImage = new(pkgapi.ObjectReference)
		if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(in.TriggeredByImage, out.TriggeredByImage, s); err != nil {
			return err
		}
	} else {
		out.TriggeredByImage = nil
	}
	if in.From != nil {
		out.From = new(pkgapi.ObjectReference)
		if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.Binary != nil {
		out.Binary = new(buildapi.BinaryBuildSource)
		if err := convert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
	return nil
}

func convert_v1beta3_BuildRequest_To_api_BuildRequest(in *apiv1beta3.BuildRequest, out *buildapi.BuildRequest, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildRequest_To_api_BuildRequest(in, out, s)
}

func autoconvert_v1beta3_BuildSource_To_api_BuildSource(in *apiv1beta3.BuildSource, out *buildapi.BuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildSource))(in)
	}
	out.Type = buildapi.BuildSourceType(in.Type)
	if in.Binary != nil {
		out.Binary = new(buildapi.BinaryBuildSource)
		if err := convert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
	if in.Git != nil {
		out.Git = new(buildapi.GitBuildSource)
		if err := convert_v1beta3_GitBuildSource_To_api_GitBuildSource(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	out.ContextDir = in.ContextDir
	if in.SourceSecret != nil {
		out.SourceSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.SourceSecret, out.SourceSecret, s); err != nil {
			return err
		}
	} else {
		out.SourceSecret = nil
	}
	return nil
}

func convert_v1beta3_BuildSource_To_api_BuildSource(in *apiv1beta3.BuildSource, out *buildapi.BuildSource, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildSource_To_api_BuildSource(in, out, s)
}

func autoconvert_v1beta3_BuildSpec_To_api_BuildSpec(in *apiv1beta3.BuildSpec, out *buildapi.BuildSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildSpec))(in)
	}
	out.ServiceAccount = in.ServiceAccount
	if err := convert_v1beta3_BuildSource_To_api_BuildSource(&in.Source, &out.Source, s); err != nil {
		return err
	}
	if in.Revision != nil {
		out.Revision = new(buildapi.SourceRevision)
		if err := convert_v1beta3_SourceRevision_To_api_SourceRevision(in.Revision, out.Revision, s); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := convert_v1beta3_BuildStrategy_To_api_BuildStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Output, &out.Output, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ResourceRequirements_To_api_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
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

func convert_v1beta3_BuildSpec_To_api_BuildSpec(in *apiv1beta3.BuildSpec, out *buildapi.BuildSpec, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildSpec_To_api_BuildSpec(in, out, s)
}

func autoconvert_v1beta3_BuildStatus_To_api_BuildStatus(in *apiv1beta3.BuildStatus, out *buildapi.BuildStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildStatus))(in)
	}
	out.Phase = buildapi.BuildPhase(in.Phase)
	out.Cancelled = in.Cancelled
	out.Reason = buildapi.StatusReason(in.Reason)
	out.Message = in.Message
	if in.StartTimestamp != nil {
		if err := s.Convert(&in.StartTimestamp, &out.StartTimestamp, 0); err != nil {
			return err
		}
	} else {
		out.StartTimestamp = nil
	}
	if in.CompletionTimestamp != nil {
		if err := s.Convert(&in.CompletionTimestamp, &out.CompletionTimestamp, 0); err != nil {
			return err
		}
	} else {
		out.CompletionTimestamp = nil
	}
	out.Duration = in.Duration
	out.OutputDockerImageReference = in.OutputDockerImageReference
	if in.Config != nil {
		out.Config = new(pkgapi.ObjectReference)
		if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(in.Config, out.Config, s); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	return nil
}

func convert_v1beta3_BuildStatus_To_api_BuildStatus(in *apiv1beta3.BuildStatus, out *buildapi.BuildStatus, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildStatus_To_api_BuildStatus(in, out, s)
}

func autoconvert_v1beta3_BuildStrategy_To_api_BuildStrategy(in *apiv1beta3.BuildStrategy, out *buildapi.BuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildStrategy))(in)
	}
	out.Type = buildapi.BuildStrategyType(in.Type)
	if in.DockerStrategy != nil {
		if err := s.Convert(&in.DockerStrategy, &out.DockerStrategy, 0); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	if in.SourceStrategy != nil {
		if err := s.Convert(&in.SourceStrategy, &out.SourceStrategy, 0); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	if in.CustomStrategy != nil {
		if err := s.Convert(&in.CustomStrategy, &out.CustomStrategy, 0); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	return nil
}

func convert_v1beta3_BuildStrategy_To_api_BuildStrategy(in *apiv1beta3.BuildStrategy, out *buildapi.BuildStrategy, s conversion.Scope) error {
	return autoconvert_v1beta3_BuildStrategy_To_api_BuildStrategy(in, out, s)
}

func autoconvert_v1beta3_BuildTriggerPolicy_To_api_BuildTriggerPolicy(in *apiv1beta3.BuildTriggerPolicy, out *buildapi.BuildTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.BuildTriggerPolicy))(in)
	}
	out.Type = buildapi.BuildTriggerType(in.Type)
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(buildapi.WebHookTrigger)
		if err := convert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in.GitHubWebHook, out.GitHubWebHook, s); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(buildapi.WebHookTrigger)
		if err := convert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in.GenericWebHook, out.GenericWebHook, s); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	if in.ImageChange != nil {
		out.ImageChange = new(buildapi.ImageChangeTrigger)
		if err := convert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger(in.ImageChange, out.ImageChange, s); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func autoconvert_v1beta3_CustomBuildStrategy_To_api_CustomBuildStrategy(in *apiv1beta3.CustomBuildStrategy, out *buildapi.CustomBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.CustomBuildStrategy))(in)
	}
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1beta3_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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
			if err := convert_v1beta3_SecretSpec_To_api_SecretSpec(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func autoconvert_v1beta3_DockerBuildStrategy_To_api_DockerBuildStrategy(in *apiv1beta3.DockerBuildStrategy, out *buildapi.DockerBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.DockerBuildStrategy))(in)
	}
	if in.From != nil {
		out.From = new(pkgapi.ObjectReference)
		if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1beta3_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ForcePull = in.ForcePull
	return nil
}

func autoconvert_v1beta3_GitBuildSource_To_api_GitBuildSource(in *apiv1beta3.GitBuildSource, out *buildapi.GitBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.GitBuildSource))(in)
	}
	out.URI = in.URI
	out.Ref = in.Ref
	out.HTTPProxy = in.HTTPProxy
	out.HTTPSProxy = in.HTTPSProxy
	return nil
}

func convert_v1beta3_GitBuildSource_To_api_GitBuildSource(in *apiv1beta3.GitBuildSource, out *buildapi.GitBuildSource, s conversion.Scope) error {
	return autoconvert_v1beta3_GitBuildSource_To_api_GitBuildSource(in, out, s)
}

func autoconvert_v1beta3_GitSourceRevision_To_api_GitSourceRevision(in *apiv1beta3.GitSourceRevision, out *buildapi.GitSourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.GitSourceRevision))(in)
	}
	out.Commit = in.Commit
	if err := convert_v1beta3_SourceControlUser_To_api_SourceControlUser(&in.Author, &out.Author, s); err != nil {
		return err
	}
	if err := convert_v1beta3_SourceControlUser_To_api_SourceControlUser(&in.Committer, &out.Committer, s); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func convert_v1beta3_GitSourceRevision_To_api_GitSourceRevision(in *apiv1beta3.GitSourceRevision, out *buildapi.GitSourceRevision, s conversion.Scope) error {
	return autoconvert_v1beta3_GitSourceRevision_To_api_GitSourceRevision(in, out, s)
}

func autoconvert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger(in *apiv1beta3.ImageChangeTrigger, out *buildapi.ImageChangeTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.ImageChangeTrigger))(in)
	}
	out.LastTriggeredImageID = in.LastTriggeredImageID
	if in.From != nil {
		out.From = new(pkgapi.ObjectReference)
		if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	return nil
}

func convert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger(in *apiv1beta3.ImageChangeTrigger, out *buildapi.ImageChangeTrigger, s conversion.Scope) error {
	return autoconvert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger(in, out, s)
}

func autoconvert_v1beta3_SecretSpec_To_api_SecretSpec(in *apiv1beta3.SecretSpec, out *buildapi.SecretSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.SecretSpec))(in)
	}
	if err := convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(&in.SecretSource, &out.SecretSource, s); err != nil {
		return err
	}
	out.MountPath = in.MountPath
	return nil
}

func convert_v1beta3_SecretSpec_To_api_SecretSpec(in *apiv1beta3.SecretSpec, out *buildapi.SecretSpec, s conversion.Scope) error {
	return autoconvert_v1beta3_SecretSpec_To_api_SecretSpec(in, out, s)
}

func autoconvert_v1beta3_SourceBuildStrategy_To_api_SourceBuildStrategy(in *apiv1beta3.SourceBuildStrategy, out *buildapi.SourceBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.SourceBuildStrategy))(in)
	}
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1beta3_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoconvert_v1beta3_SourceControlUser_To_api_SourceControlUser(in *apiv1beta3.SourceControlUser, out *buildapi.SourceControlUser, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.SourceControlUser))(in)
	}
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func convert_v1beta3_SourceControlUser_To_api_SourceControlUser(in *apiv1beta3.SourceControlUser, out *buildapi.SourceControlUser, s conversion.Scope) error {
	return autoconvert_v1beta3_SourceControlUser_To_api_SourceControlUser(in, out, s)
}

func autoconvert_v1beta3_SourceRevision_To_api_SourceRevision(in *apiv1beta3.SourceRevision, out *buildapi.SourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.SourceRevision))(in)
	}
	out.Type = buildapi.BuildSourceType(in.Type)
	if in.Git != nil {
		out.Git = new(buildapi.GitSourceRevision)
		if err := convert_v1beta3_GitSourceRevision_To_api_GitSourceRevision(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func convert_v1beta3_SourceRevision_To_api_SourceRevision(in *apiv1beta3.SourceRevision, out *buildapi.SourceRevision, s conversion.Scope) error {
	return autoconvert_v1beta3_SourceRevision_To_api_SourceRevision(in, out, s)
}

func autoconvert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in *apiv1beta3.WebHookTrigger, out *buildapi.WebHookTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1beta3.WebHookTrigger))(in)
	}
	out.Secret = in.Secret
	return nil
}

func convert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in *apiv1beta3.WebHookTrigger, out *buildapi.WebHookTrigger, s conversion.Scope) error {
	return autoconvert_v1beta3_WebHookTrigger_To_api_WebHookTrigger(in, out, s)
}

func autoconvert_api_DeploymentConfig_To_v1beta3_DeploymentConfig(in *deployapi.DeploymentConfig, out *deployapiv1beta3.DeploymentConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfig))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.Triggers has no peer in out
	// in.Template has no peer in out
	// in.LatestVersion has no peer in out
	// in.Details has no peer in out
	return nil
}

func autoconvert_api_DeploymentConfigList_To_v1beta3_DeploymentConfigList(in *deployapi.DeploymentConfigList, out *deployapiv1beta3.DeploymentConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]deployapiv1beta3.DeploymentConfig, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_DeploymentConfigList_To_v1beta3_DeploymentConfigList(in *deployapi.DeploymentConfigList, out *deployapiv1beta3.DeploymentConfigList, s conversion.Scope) error {
	return autoconvert_api_DeploymentConfigList_To_v1beta3_DeploymentConfigList(in, out, s)
}

func autoconvert_api_DeploymentConfigRollback_To_v1beta3_DeploymentConfigRollback(in *deployapi.DeploymentConfigRollback, out *deployapiv1beta3.DeploymentConfigRollback, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigRollback))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func convert_api_DeploymentConfigRollback_To_v1beta3_DeploymentConfigRollback(in *deployapi.DeploymentConfigRollback, out *deployapiv1beta3.DeploymentConfigRollback, s conversion.Scope) error {
	return autoconvert_api_DeploymentConfigRollback_To_v1beta3_DeploymentConfigRollback(in, out, s)
}

func autoconvert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec(in *deployapi.DeploymentConfigRollbackSpec, out *deployapiv1beta3.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigRollbackSpec))(in)
	}
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func convert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec(in *deployapi.DeploymentConfigRollbackSpec, out *deployapiv1beta3.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	return autoconvert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec(in, out, s)
}

func autoconvert_api_DeploymentLog_To_v1beta3_DeploymentLog(in *deployapi.DeploymentLog, out *deployapiv1beta3.DeploymentLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentLog))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_DeploymentLog_To_v1beta3_DeploymentLog(in *deployapi.DeploymentLog, out *deployapiv1beta3.DeploymentLog, s conversion.Scope) error {
	return autoconvert_api_DeploymentLog_To_v1beta3_DeploymentLog(in, out, s)
}

func autoconvert_api_DeploymentLogOptions_To_v1beta3_DeploymentLogOptions(in *deployapi.DeploymentLogOptions, out *deployapiv1beta3.DeploymentLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentLogOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Follow = in.Follow
	out.NoWait = in.NoWait
	if in.Version != nil {
		out.Version = new(int)
		*out.Version = *in.Version
	} else {
		out.Version = nil
	}
	return nil
}

func convert_api_DeploymentLogOptions_To_v1beta3_DeploymentLogOptions(in *deployapi.DeploymentLogOptions, out *deployapiv1beta3.DeploymentLogOptions, s conversion.Scope) error {
	return autoconvert_api_DeploymentLogOptions_To_v1beta3_DeploymentLogOptions(in, out, s)
}

func autoconvert_v1beta3_DeploymentConfig_To_api_DeploymentConfig(in *deployapiv1beta3.DeploymentConfig, out *deployapi.DeploymentConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentConfig))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.Spec has no peer in out
	// in.Status has no peer in out
	return nil
}

func autoconvert_v1beta3_DeploymentConfigList_To_api_DeploymentConfigList(in *deployapiv1beta3.DeploymentConfigList, out *deployapi.DeploymentConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentConfigList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]deployapi.DeploymentConfig, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_DeploymentConfigList_To_api_DeploymentConfigList(in *deployapiv1beta3.DeploymentConfigList, out *deployapi.DeploymentConfigList, s conversion.Scope) error {
	return autoconvert_v1beta3_DeploymentConfigList_To_api_DeploymentConfigList(in, out, s)
}

func autoconvert_v1beta3_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in *deployapiv1beta3.DeploymentConfigRollback, out *deployapi.DeploymentConfigRollback, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentConfigRollback))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in *deployapiv1beta3.DeploymentConfigRollback, out *deployapi.DeploymentConfigRollback, s conversion.Scope) error {
	return autoconvert_v1beta3_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in, out, s)
}

func autoconvert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in *deployapiv1beta3.DeploymentConfigRollbackSpec, out *deployapi.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentConfigRollbackSpec))(in)
	}
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func convert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in *deployapiv1beta3.DeploymentConfigRollbackSpec, out *deployapi.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	return autoconvert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in, out, s)
}

func autoconvert_v1beta3_DeploymentLog_To_api_DeploymentLog(in *deployapiv1beta3.DeploymentLog, out *deployapi.DeploymentLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentLog))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_DeploymentLog_To_api_DeploymentLog(in *deployapiv1beta3.DeploymentLog, out *deployapi.DeploymentLog, s conversion.Scope) error {
	return autoconvert_v1beta3_DeploymentLog_To_api_DeploymentLog(in, out, s)
}

func autoconvert_v1beta3_DeploymentLogOptions_To_api_DeploymentLogOptions(in *deployapiv1beta3.DeploymentLogOptions, out *deployapi.DeploymentLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1beta3.DeploymentLogOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Follow = in.Follow
	out.NoWait = in.NoWait
	if in.Version != nil {
		out.Version = new(int)
		*out.Version = *in.Version
	} else {
		out.Version = nil
	}
	return nil
}

func convert_v1beta3_DeploymentLogOptions_To_api_DeploymentLogOptions(in *deployapiv1beta3.DeploymentLogOptions, out *deployapi.DeploymentLogOptions, s conversion.Scope) error {
	return autoconvert_v1beta3_DeploymentLogOptions_To_api_DeploymentLogOptions(in, out, s)
}

func autoconvert_api_Image_To_v1beta3_Image(in *imageapi.Image, out *imageapiv1beta3.Image, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.Image))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DockerImageReference = in.DockerImageReference
	if err := s.Convert(&in.DockerImageMetadata, &out.DockerImageMetadata, 0); err != nil {
		return err
	}
	out.DockerImageMetadataVersion = in.DockerImageMetadataVersion
	out.DockerImageManifest = in.DockerImageManifest
	return nil
}

func autoconvert_api_ImageList_To_v1beta3_ImageList(in *imageapi.ImageList, out *imageapiv1beta3.ImageList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1beta3.Image, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ImageList_To_v1beta3_ImageList(in *imageapi.ImageList, out *imageapiv1beta3.ImageList, s conversion.Scope) error {
	return autoconvert_api_ImageList_To_v1beta3_ImageList(in, out, s)
}

func autoconvert_api_ImageStream_To_v1beta3_ImageStream(in *imageapi.ImageStream, out *imageapiv1beta3.ImageStream, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStream))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec, &out.Spec, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ImageStreamImage_To_v1beta3_ImageStreamImage(in *imageapi.ImageStreamImage, out *imageapiv1beta3.ImageStreamImage, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamImage))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ImageStreamList_To_v1beta3_ImageStreamList(in *imageapi.ImageStreamList, out *imageapiv1beta3.ImageStreamList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1beta3.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ImageStreamList_To_v1beta3_ImageStreamList(in *imageapi.ImageStreamList, out *imageapiv1beta3.ImageStreamList, s conversion.Scope) error {
	return autoconvert_api_ImageStreamList_To_v1beta3_ImageStreamList(in, out, s)
}

func autoconvert_api_ImageStreamMapping_To_v1beta3_ImageStreamMapping(in *imageapi.ImageStreamMapping, out *imageapiv1beta3.ImageStreamMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamMapping))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.DockerImageRepository has no peer in out
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func autoconvert_api_ImageStreamSpec_To_v1beta3_ImageStreamSpec(in *imageapi.ImageStreamSpec, out *imageapiv1beta3.ImageStreamSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamSpec))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ImageStreamStatus_To_v1beta3_ImageStreamStatus(in *imageapi.ImageStreamStatus, out *imageapiv1beta3.ImageStreamStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamStatus))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ImageStreamTag_To_v1beta3_ImageStreamTag(in *imageapi.ImageStreamTag, out *imageapiv1beta3.ImageStreamTag, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamTag))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1beta3_Image_To_api_Image(in *imageapiv1beta3.Image, out *imageapi.Image, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.Image))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DockerImageReference = in.DockerImageReference
	if err := s.Convert(&in.DockerImageMetadata, &out.DockerImageMetadata, 0); err != nil {
		return err
	}
	out.DockerImageMetadataVersion = in.DockerImageMetadataVersion
	out.DockerImageManifest = in.DockerImageManifest
	return nil
}

func autoconvert_v1beta3_ImageList_To_api_ImageList(in *imageapiv1beta3.ImageList, out *imageapi.ImageList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapi.Image, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_ImageList_To_api_ImageList(in *imageapiv1beta3.ImageList, out *imageapi.ImageList, s conversion.Scope) error {
	return autoconvert_v1beta3_ImageList_To_api_ImageList(in, out, s)
}

func autoconvert_v1beta3_ImageStream_To_api_ImageStream(in *imageapiv1beta3.ImageStream, out *imageapi.ImageStream, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStream))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec, &out.Spec, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Status, &out.Status, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1beta3_ImageStreamImage_To_api_ImageStreamImage(in *imageapiv1beta3.ImageStreamImage, out *imageapi.ImageStreamImage, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamImage))(in)
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	// in.ImageName has no peer in out
	return nil
}

func autoconvert_v1beta3_ImageStreamList_To_api_ImageStreamList(in *imageapiv1beta3.ImageStreamList, out *imageapi.ImageStreamList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapi.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_ImageStreamList_To_api_ImageStreamList(in *imageapiv1beta3.ImageStreamList, out *imageapi.ImageStreamList, s conversion.Scope) error {
	return autoconvert_v1beta3_ImageStreamList_To_api_ImageStreamList(in, out, s)
}

func autoconvert_v1beta3_ImageStreamMapping_To_api_ImageStreamMapping(in *imageapiv1beta3.ImageStreamMapping, out *imageapi.ImageStreamMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamMapping))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func autoconvert_v1beta3_ImageStreamSpec_To_api_ImageStreamSpec(in *imageapiv1beta3.ImageStreamSpec, out *imageapi.ImageStreamSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamSpec))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1beta3_ImageStreamStatus_To_api_ImageStreamStatus(in *imageapiv1beta3.ImageStreamStatus, out *imageapi.ImageStreamStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamStatus))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1beta3_ImageStreamTag_To_api_ImageStreamTag(in *imageapiv1beta3.ImageStreamTag, out *imageapi.ImageStreamTag, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1beta3.ImageStreamTag))(in)
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	// in.ImageName has no peer in out
	return nil
}

func autoconvert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken(in *oauthapi.OAuthAccessToken, out *oauthapiv1beta3.OAuthAccessToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAccessToken))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken(in *oauthapi.OAuthAccessToken, out *oauthapiv1beta3.OAuthAccessToken, s conversion.Scope) error {
	return autoconvert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken(in, out, s)
}

func autoconvert_api_OAuthAccessTokenList_To_v1beta3_OAuthAccessTokenList(in *oauthapi.OAuthAccessTokenList, out *oauthapiv1beta3.OAuthAccessTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAccessTokenList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := convert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_OAuthAccessTokenList_To_v1beta3_OAuthAccessTokenList(in *oauthapi.OAuthAccessTokenList, out *oauthapiv1beta3.OAuthAccessTokenList, s conversion.Scope) error {
	return autoconvert_api_OAuthAccessTokenList_To_v1beta3_OAuthAccessTokenList(in, out, s)
}

func autoconvert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken(in *oauthapi.OAuthAuthorizeToken, out *oauthapiv1beta3.OAuthAuthorizeToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAuthorizeToken))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken(in *oauthapi.OAuthAuthorizeToken, out *oauthapiv1beta3.OAuthAuthorizeToken, s conversion.Scope) error {
	return autoconvert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken(in, out, s)
}

func autoconvert_api_OAuthAuthorizeTokenList_To_v1beta3_OAuthAuthorizeTokenList(in *oauthapi.OAuthAuthorizeTokenList, out *oauthapiv1beta3.OAuthAuthorizeTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAuthorizeTokenList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := convert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_OAuthAuthorizeTokenList_To_v1beta3_OAuthAuthorizeTokenList(in *oauthapi.OAuthAuthorizeTokenList, out *oauthapiv1beta3.OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoconvert_api_OAuthAuthorizeTokenList_To_v1beta3_OAuthAuthorizeTokenList(in, out, s)
}

func autoconvert_api_OAuthClient_To_v1beta3_OAuthClient(in *oauthapi.OAuthClient, out *oauthapiv1beta3.OAuthClient, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClient))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_OAuthClient_To_v1beta3_OAuthClient(in *oauthapi.OAuthClient, out *oauthapiv1beta3.OAuthClient, s conversion.Scope) error {
	return autoconvert_api_OAuthClient_To_v1beta3_OAuthClient(in, out, s)
}

func autoconvert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization(in *oauthapi.OAuthClientAuthorization, out *oauthapiv1beta3.OAuthClientAuthorization, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientAuthorization))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization(in *oauthapi.OAuthClientAuthorization, out *oauthapiv1beta3.OAuthClientAuthorization, s conversion.Scope) error {
	return autoconvert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization(in, out, s)
}

func autoconvert_api_OAuthClientAuthorizationList_To_v1beta3_OAuthClientAuthorizationList(in *oauthapi.OAuthClientAuthorizationList, out *oauthapiv1beta3.OAuthClientAuthorizationList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientAuthorizationList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := convert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_OAuthClientAuthorizationList_To_v1beta3_OAuthClientAuthorizationList(in *oauthapi.OAuthClientAuthorizationList, out *oauthapiv1beta3.OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoconvert_api_OAuthClientAuthorizationList_To_v1beta3_OAuthClientAuthorizationList(in, out, s)
}

func autoconvert_api_OAuthClientList_To_v1beta3_OAuthClientList(in *oauthapi.OAuthClientList, out *oauthapiv1beta3.OAuthClientList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := convert_api_OAuthClient_To_v1beta3_OAuthClient(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_OAuthClientList_To_v1beta3_OAuthClientList(in *oauthapi.OAuthClientList, out *oauthapiv1beta3.OAuthClientList, s conversion.Scope) error {
	return autoconvert_api_OAuthClientList_To_v1beta3_OAuthClientList(in, out, s)
}

func autoconvert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken(in *oauthapiv1beta3.OAuthAccessToken, out *oauthapi.OAuthAccessToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthAccessToken))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken(in *oauthapiv1beta3.OAuthAccessToken, out *oauthapi.OAuthAccessToken, s conversion.Scope) error {
	return autoconvert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken(in, out, s)
}

func autoconvert_v1beta3_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *oauthapiv1beta3.OAuthAccessTokenList, out *oauthapi.OAuthAccessTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthAccessTokenList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *oauthapiv1beta3.OAuthAccessTokenList, out *oauthapi.OAuthAccessTokenList, s conversion.Scope) error {
	return autoconvert_v1beta3_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in, out, s)
}

func autoconvert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *oauthapiv1beta3.OAuthAuthorizeToken, out *oauthapi.OAuthAuthorizeToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthAuthorizeToken))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *oauthapiv1beta3.OAuthAuthorizeToken, out *oauthapi.OAuthAuthorizeToken, s conversion.Scope) error {
	return autoconvert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in, out, s)
}

func autoconvert_v1beta3_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *oauthapiv1beta3.OAuthAuthorizeTokenList, out *oauthapi.OAuthAuthorizeTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthAuthorizeTokenList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *oauthapiv1beta3.OAuthAuthorizeTokenList, out *oauthapi.OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoconvert_v1beta3_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in, out, s)
}

func autoconvert_v1beta3_OAuthClient_To_api_OAuthClient(in *oauthapiv1beta3.OAuthClient, out *oauthapi.OAuthClient, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthClient))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1beta3_OAuthClient_To_api_OAuthClient(in *oauthapiv1beta3.OAuthClient, out *oauthapi.OAuthClient, s conversion.Scope) error {
	return autoconvert_v1beta3_OAuthClient_To_api_OAuthClient(in, out, s)
}

func autoconvert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *oauthapiv1beta3.OAuthClientAuthorization, out *oauthapi.OAuthClientAuthorization, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthClientAuthorization))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *oauthapiv1beta3.OAuthClientAuthorization, out *oauthapi.OAuthClientAuthorization, s conversion.Scope) error {
	return autoconvert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in, out, s)
}

func autoconvert_v1beta3_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *oauthapiv1beta3.OAuthClientAuthorizationList, out *oauthapi.OAuthClientAuthorizationList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthClientAuthorizationList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *oauthapiv1beta3.OAuthClientAuthorizationList, out *oauthapi.OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoconvert_v1beta3_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in, out, s)
}

func autoconvert_v1beta3_OAuthClientList_To_api_OAuthClientList(in *oauthapiv1beta3.OAuthClientList, out *oauthapi.OAuthClientList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1beta3.OAuthClientList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_OAuthClient_To_api_OAuthClient(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_OAuthClientList_To_api_OAuthClientList(in *oauthapiv1beta3.OAuthClientList, out *oauthapi.OAuthClientList, s conversion.Scope) error {
	return autoconvert_v1beta3_OAuthClientList_To_api_OAuthClientList(in, out, s)
}

func autoconvert_api_Project_To_v1beta3_Project(in *projectapi.Project, out *projectapiv1beta3.Project, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.Project))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_ProjectSpec_To_v1beta3_ProjectSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_api_ProjectStatus_To_v1beta3_ProjectStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_api_Project_To_v1beta3_Project(in *projectapi.Project, out *projectapiv1beta3.Project, s conversion.Scope) error {
	return autoconvert_api_Project_To_v1beta3_Project(in, out, s)
}

func autoconvert_api_ProjectList_To_v1beta3_ProjectList(in *projectapi.ProjectList, out *projectapiv1beta3.ProjectList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]projectapiv1beta3.Project, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Project_To_v1beta3_Project(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ProjectList_To_v1beta3_ProjectList(in *projectapi.ProjectList, out *projectapiv1beta3.ProjectList, s conversion.Scope) error {
	return autoconvert_api_ProjectList_To_v1beta3_ProjectList(in, out, s)
}

func autoconvert_api_ProjectRequest_To_v1beta3_ProjectRequest(in *projectapi.ProjectRequest, out *projectapiv1beta3.ProjectRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectRequest))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func convert_api_ProjectRequest_To_v1beta3_ProjectRequest(in *projectapi.ProjectRequest, out *projectapiv1beta3.ProjectRequest, s conversion.Scope) error {
	return autoconvert_api_ProjectRequest_To_v1beta3_ProjectRequest(in, out, s)
}

func autoconvert_api_ProjectSpec_To_v1beta3_ProjectSpec(in *projectapi.ProjectSpec, out *projectapiv1beta3.ProjectSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectSpec))(in)
	}
	if in.Finalizers != nil {
		out.Finalizers = make([]pkgapiv1beta3.FinalizerName, len(in.Finalizers))
		for i := range in.Finalizers {
			out.Finalizers[i] = pkgapiv1beta3.FinalizerName(in.Finalizers[i])
		}
	} else {
		out.Finalizers = nil
	}
	return nil
}

func convert_api_ProjectSpec_To_v1beta3_ProjectSpec(in *projectapi.ProjectSpec, out *projectapiv1beta3.ProjectSpec, s conversion.Scope) error {
	return autoconvert_api_ProjectSpec_To_v1beta3_ProjectSpec(in, out, s)
}

func autoconvert_api_ProjectStatus_To_v1beta3_ProjectStatus(in *projectapi.ProjectStatus, out *projectapiv1beta3.ProjectStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectStatus))(in)
	}
	out.Phase = pkgapiv1beta3.NamespacePhase(in.Phase)
	return nil
}

func convert_api_ProjectStatus_To_v1beta3_ProjectStatus(in *projectapi.ProjectStatus, out *projectapiv1beta3.ProjectStatus, s conversion.Scope) error {
	return autoconvert_api_ProjectStatus_To_v1beta3_ProjectStatus(in, out, s)
}

func autoconvert_v1beta3_Project_To_api_Project(in *projectapiv1beta3.Project, out *projectapi.Project, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.Project))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1beta3_ProjectSpec_To_api_ProjectSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1beta3_ProjectStatus_To_api_ProjectStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_Project_To_api_Project(in *projectapiv1beta3.Project, out *projectapi.Project, s conversion.Scope) error {
	return autoconvert_v1beta3_Project_To_api_Project(in, out, s)
}

func autoconvert_v1beta3_ProjectList_To_api_ProjectList(in *projectapiv1beta3.ProjectList, out *projectapi.ProjectList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.ProjectList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]projectapi.Project, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_Project_To_api_Project(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_ProjectList_To_api_ProjectList(in *projectapiv1beta3.ProjectList, out *projectapi.ProjectList, s conversion.Scope) error {
	return autoconvert_v1beta3_ProjectList_To_api_ProjectList(in, out, s)
}

func autoconvert_v1beta3_ProjectRequest_To_api_ProjectRequest(in *projectapiv1beta3.ProjectRequest, out *projectapi.ProjectRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.ProjectRequest))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func convert_v1beta3_ProjectRequest_To_api_ProjectRequest(in *projectapiv1beta3.ProjectRequest, out *projectapi.ProjectRequest, s conversion.Scope) error {
	return autoconvert_v1beta3_ProjectRequest_To_api_ProjectRequest(in, out, s)
}

func autoconvert_v1beta3_ProjectSpec_To_api_ProjectSpec(in *projectapiv1beta3.ProjectSpec, out *projectapi.ProjectSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.ProjectSpec))(in)
	}
	if in.Finalizers != nil {
		out.Finalizers = make([]pkgapi.FinalizerName, len(in.Finalizers))
		for i := range in.Finalizers {
			out.Finalizers[i] = pkgapi.FinalizerName(in.Finalizers[i])
		}
	} else {
		out.Finalizers = nil
	}
	return nil
}

func convert_v1beta3_ProjectSpec_To_api_ProjectSpec(in *projectapiv1beta3.ProjectSpec, out *projectapi.ProjectSpec, s conversion.Scope) error {
	return autoconvert_v1beta3_ProjectSpec_To_api_ProjectSpec(in, out, s)
}

func autoconvert_v1beta3_ProjectStatus_To_api_ProjectStatus(in *projectapiv1beta3.ProjectStatus, out *projectapi.ProjectStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1beta3.ProjectStatus))(in)
	}
	out.Phase = pkgapi.NamespacePhase(in.Phase)
	return nil
}

func convert_v1beta3_ProjectStatus_To_api_ProjectStatus(in *projectapiv1beta3.ProjectStatus, out *projectapi.ProjectStatus, s conversion.Scope) error {
	return autoconvert_v1beta3_ProjectStatus_To_api_ProjectStatus(in, out, s)
}

func autoconvert_api_Route_To_v1beta3_Route(in *routeapi.Route, out *routeapiv1beta3.Route, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.Route))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_RouteSpec_To_v1beta3_RouteSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_api_RouteStatus_To_v1beta3_RouteStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_api_Route_To_v1beta3_Route(in *routeapi.Route, out *routeapiv1beta3.Route, s conversion.Scope) error {
	return autoconvert_api_Route_To_v1beta3_Route(in, out, s)
}

func autoconvert_api_RouteList_To_v1beta3_RouteList(in *routeapi.RouteList, out *routeapiv1beta3.RouteList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]routeapiv1beta3.Route, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Route_To_v1beta3_Route(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_RouteList_To_v1beta3_RouteList(in *routeapi.RouteList, out *routeapiv1beta3.RouteList, s conversion.Scope) error {
	return autoconvert_api_RouteList_To_v1beta3_RouteList(in, out, s)
}

func autoconvert_api_RoutePort_To_v1beta3_RoutePort(in *routeapi.RoutePort, out *routeapiv1beta3.RoutePort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RoutePort))(in)
	}
	if err := s.Convert(&in.TargetPort, &out.TargetPort, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_RoutePort_To_v1beta3_RoutePort(in *routeapi.RoutePort, out *routeapiv1beta3.RoutePort, s conversion.Scope) error {
	return autoconvert_api_RoutePort_To_v1beta3_RoutePort(in, out, s)
}

func autoconvert_api_RouteSpec_To_v1beta3_RouteSpec(in *routeapi.RouteSpec, out *routeapiv1beta3.RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	if in.Port != nil {
		out.Port = new(routeapiv1beta3.RoutePort)
		if err := convert_api_RoutePort_To_v1beta3_RoutePort(in.Port, out.Port, s); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	if in.TLS != nil {
		out.TLS = new(routeapiv1beta3.TLSConfig)
		if err := convert_api_TLSConfig_To_v1beta3_TLSConfig(in.TLS, out.TLS, s); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func convert_api_RouteSpec_To_v1beta3_RouteSpec(in *routeapi.RouteSpec, out *routeapiv1beta3.RouteSpec, s conversion.Scope) error {
	return autoconvert_api_RouteSpec_To_v1beta3_RouteSpec(in, out, s)
}

func autoconvert_api_RouteStatus_To_v1beta3_RouteStatus(in *routeapi.RouteStatus, out *routeapiv1beta3.RouteStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteStatus))(in)
	}
	return nil
}

func convert_api_RouteStatus_To_v1beta3_RouteStatus(in *routeapi.RouteStatus, out *routeapiv1beta3.RouteStatus, s conversion.Scope) error {
	return autoconvert_api_RouteStatus_To_v1beta3_RouteStatus(in, out, s)
}

func autoconvert_api_TLSConfig_To_v1beta3_TLSConfig(in *routeapi.TLSConfig, out *routeapiv1beta3.TLSConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.TLSConfig))(in)
	}
	out.Termination = routeapiv1beta3.TLSTerminationType(in.Termination)
	out.Certificate = in.Certificate
	out.Key = in.Key
	out.CACertificate = in.CACertificate
	out.DestinationCACertificate = in.DestinationCACertificate
	out.Insecure = routeapiv1beta3.TLSInsecureType(in.Insecure)
	return nil
}

func convert_api_TLSConfig_To_v1beta3_TLSConfig(in *routeapi.TLSConfig, out *routeapiv1beta3.TLSConfig, s conversion.Scope) error {
	return autoconvert_api_TLSConfig_To_v1beta3_TLSConfig(in, out, s)
}

func autoconvert_v1beta3_Route_To_api_Route(in *routeapiv1beta3.Route, out *routeapi.Route, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.Route))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1beta3_RouteSpec_To_api_RouteSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1beta3_RouteStatus_To_api_RouteStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_Route_To_api_Route(in *routeapiv1beta3.Route, out *routeapi.Route, s conversion.Scope) error {
	return autoconvert_v1beta3_Route_To_api_Route(in, out, s)
}

func autoconvert_v1beta3_RouteList_To_api_RouteList(in *routeapiv1beta3.RouteList, out *routeapi.RouteList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RouteList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]routeapi.Route, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_Route_To_api_Route(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_RouteList_To_api_RouteList(in *routeapiv1beta3.RouteList, out *routeapi.RouteList, s conversion.Scope) error {
	return autoconvert_v1beta3_RouteList_To_api_RouteList(in, out, s)
}

func autoconvert_v1beta3_RoutePort_To_api_RoutePort(in *routeapiv1beta3.RoutePort, out *routeapi.RoutePort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RoutePort))(in)
	}
	if err := s.Convert(&in.TargetPort, &out.TargetPort, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_RoutePort_To_api_RoutePort(in *routeapiv1beta3.RoutePort, out *routeapi.RoutePort, s conversion.Scope) error {
	return autoconvert_v1beta3_RoutePort_To_api_RoutePort(in, out, s)
}

func autoconvert_v1beta3_RouteSpec_To_api_RouteSpec(in *routeapiv1beta3.RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	if in.Port != nil {
		out.Port = new(routeapi.RoutePort)
		if err := convert_v1beta3_RoutePort_To_api_RoutePort(in.Port, out.Port, s); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	if in.TLS != nil {
		out.TLS = new(routeapi.TLSConfig)
		if err := convert_v1beta3_TLSConfig_To_api_TLSConfig(in.TLS, out.TLS, s); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func convert_v1beta3_RouteSpec_To_api_RouteSpec(in *routeapiv1beta3.RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	return autoconvert_v1beta3_RouteSpec_To_api_RouteSpec(in, out, s)
}

func autoconvert_v1beta3_RouteStatus_To_api_RouteStatus(in *routeapiv1beta3.RouteStatus, out *routeapi.RouteStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.RouteStatus))(in)
	}
	return nil
}

func convert_v1beta3_RouteStatus_To_api_RouteStatus(in *routeapiv1beta3.RouteStatus, out *routeapi.RouteStatus, s conversion.Scope) error {
	return autoconvert_v1beta3_RouteStatus_To_api_RouteStatus(in, out, s)
}

func autoconvert_v1beta3_TLSConfig_To_api_TLSConfig(in *routeapiv1beta3.TLSConfig, out *routeapi.TLSConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1beta3.TLSConfig))(in)
	}
	out.Termination = routeapi.TLSTerminationType(in.Termination)
	out.Certificate = in.Certificate
	out.Key = in.Key
	out.CACertificate = in.CACertificate
	out.DestinationCACertificate = in.DestinationCACertificate
	out.Insecure = routeapi.TLSInsecureType(in.Insecure)
	return nil
}

func convert_v1beta3_TLSConfig_To_api_TLSConfig(in *routeapiv1beta3.TLSConfig, out *routeapi.TLSConfig, s conversion.Scope) error {
	return autoconvert_v1beta3_TLSConfig_To_api_TLSConfig(in, out, s)
}

func autoconvert_api_ClusterNetwork_To_v1beta3_ClusterNetwork(in *sdnapi.ClusterNetwork, out *sdnapiv1beta3.ClusterNetwork, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.ClusterNetwork))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func convert_api_ClusterNetwork_To_v1beta3_ClusterNetwork(in *sdnapi.ClusterNetwork, out *sdnapiv1beta3.ClusterNetwork, s conversion.Scope) error {
	return autoconvert_api_ClusterNetwork_To_v1beta3_ClusterNetwork(in, out, s)
}

func autoconvert_api_ClusterNetworkList_To_v1beta3_ClusterNetworkList(in *sdnapi.ClusterNetworkList, out *sdnapiv1beta3.ClusterNetworkList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.ClusterNetworkList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1beta3.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := convert_api_ClusterNetwork_To_v1beta3_ClusterNetwork(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ClusterNetworkList_To_v1beta3_ClusterNetworkList(in *sdnapi.ClusterNetworkList, out *sdnapiv1beta3.ClusterNetworkList, s conversion.Scope) error {
	return autoconvert_api_ClusterNetworkList_To_v1beta3_ClusterNetworkList(in, out, s)
}

func autoconvert_api_HostSubnet_To_v1beta3_HostSubnet(in *sdnapi.HostSubnet, out *sdnapiv1beta3.HostSubnet, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.HostSubnet))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func convert_api_HostSubnet_To_v1beta3_HostSubnet(in *sdnapi.HostSubnet, out *sdnapiv1beta3.HostSubnet, s conversion.Scope) error {
	return autoconvert_api_HostSubnet_To_v1beta3_HostSubnet(in, out, s)
}

func autoconvert_api_HostSubnetList_To_v1beta3_HostSubnetList(in *sdnapi.HostSubnetList, out *sdnapiv1beta3.HostSubnetList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.HostSubnetList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1beta3.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := convert_api_HostSubnet_To_v1beta3_HostSubnet(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_HostSubnetList_To_v1beta3_HostSubnetList(in *sdnapi.HostSubnetList, out *sdnapiv1beta3.HostSubnetList, s conversion.Scope) error {
	return autoconvert_api_HostSubnetList_To_v1beta3_HostSubnetList(in, out, s)
}

func autoconvert_api_NetNamespace_To_v1beta3_NetNamespace(in *sdnapi.NetNamespace, out *sdnapiv1beta3.NetNamespace, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.NetNamespace))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func convert_api_NetNamespace_To_v1beta3_NetNamespace(in *sdnapi.NetNamespace, out *sdnapiv1beta3.NetNamespace, s conversion.Scope) error {
	return autoconvert_api_NetNamespace_To_v1beta3_NetNamespace(in, out, s)
}

func autoconvert_api_NetNamespaceList_To_v1beta3_NetNamespaceList(in *sdnapi.NetNamespaceList, out *sdnapiv1beta3.NetNamespaceList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.NetNamespaceList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1beta3.NetNamespace, len(in.Items))
		for i := range in.Items {
			if err := convert_api_NetNamespace_To_v1beta3_NetNamespace(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_NetNamespaceList_To_v1beta3_NetNamespaceList(in *sdnapi.NetNamespaceList, out *sdnapiv1beta3.NetNamespaceList, s conversion.Scope) error {
	return autoconvert_api_NetNamespaceList_To_v1beta3_NetNamespaceList(in, out, s)
}

func autoconvert_v1beta3_ClusterNetwork_To_api_ClusterNetwork(in *sdnapiv1beta3.ClusterNetwork, out *sdnapi.ClusterNetwork, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.ClusterNetwork))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func convert_v1beta3_ClusterNetwork_To_api_ClusterNetwork(in *sdnapiv1beta3.ClusterNetwork, out *sdnapi.ClusterNetwork, s conversion.Scope) error {
	return autoconvert_v1beta3_ClusterNetwork_To_api_ClusterNetwork(in, out, s)
}

func autoconvert_v1beta3_ClusterNetworkList_To_api_ClusterNetworkList(in *sdnapiv1beta3.ClusterNetworkList, out *sdnapi.ClusterNetworkList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.ClusterNetworkList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_ClusterNetwork_To_api_ClusterNetwork(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_ClusterNetworkList_To_api_ClusterNetworkList(in *sdnapiv1beta3.ClusterNetworkList, out *sdnapi.ClusterNetworkList, s conversion.Scope) error {
	return autoconvert_v1beta3_ClusterNetworkList_To_api_ClusterNetworkList(in, out, s)
}

func autoconvert_v1beta3_HostSubnet_To_api_HostSubnet(in *sdnapiv1beta3.HostSubnet, out *sdnapi.HostSubnet, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.HostSubnet))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func convert_v1beta3_HostSubnet_To_api_HostSubnet(in *sdnapiv1beta3.HostSubnet, out *sdnapi.HostSubnet, s conversion.Scope) error {
	return autoconvert_v1beta3_HostSubnet_To_api_HostSubnet(in, out, s)
}

func autoconvert_v1beta3_HostSubnetList_To_api_HostSubnetList(in *sdnapiv1beta3.HostSubnetList, out *sdnapi.HostSubnetList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.HostSubnetList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_HostSubnet_To_api_HostSubnet(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_HostSubnetList_To_api_HostSubnetList(in *sdnapiv1beta3.HostSubnetList, out *sdnapi.HostSubnetList, s conversion.Scope) error {
	return autoconvert_v1beta3_HostSubnetList_To_api_HostSubnetList(in, out, s)
}

func autoconvert_v1beta3_NetNamespace_To_api_NetNamespace(in *sdnapiv1beta3.NetNamespace, out *sdnapi.NetNamespace, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.NetNamespace))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func convert_v1beta3_NetNamespace_To_api_NetNamespace(in *sdnapiv1beta3.NetNamespace, out *sdnapi.NetNamespace, s conversion.Scope) error {
	return autoconvert_v1beta3_NetNamespace_To_api_NetNamespace(in, out, s)
}

func autoconvert_v1beta3_NetNamespaceList_To_api_NetNamespaceList(in *sdnapiv1beta3.NetNamespaceList, out *sdnapi.NetNamespaceList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1beta3.NetNamespaceList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.NetNamespace, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_NetNamespace_To_api_NetNamespace(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_NetNamespaceList_To_api_NetNamespaceList(in *sdnapiv1beta3.NetNamespaceList, out *sdnapi.NetNamespaceList, s conversion.Scope) error {
	return autoconvert_v1beta3_NetNamespaceList_To_api_NetNamespaceList(in, out, s)
}

func autoconvert_api_Parameter_To_v1beta3_Parameter(in *templateapi.Parameter, out *templateapiv1beta3.Parameter, s conversion.Scope) error {
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

func convert_api_Parameter_To_v1beta3_Parameter(in *templateapi.Parameter, out *templateapiv1beta3.Parameter, s conversion.Scope) error {
	return autoconvert_api_Parameter_To_v1beta3_Parameter(in, out, s)
}

func autoconvert_api_Template_To_v1beta3_Template(in *templateapi.Template, out *templateapiv1beta3.Template, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapi.Template))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Parameters != nil {
		out.Parameters = make([]templateapiv1beta3.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := convert_api_Parameter_To_v1beta3_Parameter(&in.Parameters[i], &out.Parameters[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Parameters = nil
	}
	if err := s.Convert(&in.Objects, &out.Objects, 0); err != nil {
		return err
	}
	// in.ObjectLabels has no peer in out
	return nil
}

func autoconvert_api_TemplateList_To_v1beta3_TemplateList(in *templateapi.TemplateList, out *templateapiv1beta3.TemplateList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapi.TemplateList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]templateapiv1beta3.Template, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_TemplateList_To_v1beta3_TemplateList(in *templateapi.TemplateList, out *templateapiv1beta3.TemplateList, s conversion.Scope) error {
	return autoconvert_api_TemplateList_To_v1beta3_TemplateList(in, out, s)
}

func autoconvert_v1beta3_Parameter_To_api_Parameter(in *templateapiv1beta3.Parameter, out *templateapi.Parameter, s conversion.Scope) error {
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

func convert_v1beta3_Parameter_To_api_Parameter(in *templateapiv1beta3.Parameter, out *templateapi.Parameter, s conversion.Scope) error {
	return autoconvert_v1beta3_Parameter_To_api_Parameter(in, out, s)
}

func autoconvert_v1beta3_Template_To_api_Template(in *templateapiv1beta3.Template, out *templateapi.Template, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1beta3.Template))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Objects, &out.Objects, 0); err != nil {
		return err
	}
	if in.Parameters != nil {
		out.Parameters = make([]templateapi.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := convert_v1beta3_Parameter_To_api_Parameter(&in.Parameters[i], &out.Parameters[i], s); err != nil {
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

func autoconvert_v1beta3_TemplateList_To_api_TemplateList(in *templateapiv1beta3.TemplateList, out *templateapi.TemplateList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1beta3.TemplateList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]templateapi.Template, len(in.Items))
		for i := range in.Items {
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_TemplateList_To_api_TemplateList(in *templateapiv1beta3.TemplateList, out *templateapi.TemplateList, s conversion.Scope) error {
	return autoconvert_v1beta3_TemplateList_To_api_TemplateList(in, out, s)
}

func autoconvert_api_Group_To_v1beta3_Group(in *userapi.Group, out *userapiv1beta3.Group, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.Group))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_Group_To_v1beta3_Group(in *userapi.Group, out *userapiv1beta3.Group, s conversion.Scope) error {
	return autoconvert_api_Group_To_v1beta3_Group(in, out, s)
}

func autoconvert_api_GroupList_To_v1beta3_GroupList(in *userapi.GroupList, out *userapiv1beta3.GroupList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.GroupList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapiv1beta3.Group, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Group_To_v1beta3_Group(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_GroupList_To_v1beta3_GroupList(in *userapi.GroupList, out *userapiv1beta3.GroupList, s conversion.Scope) error {
	return autoconvert_api_GroupList_To_v1beta3_GroupList(in, out, s)
}

func autoconvert_api_Identity_To_v1beta3_Identity(in *userapi.Identity, out *userapiv1beta3.Identity, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.Identity))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.User, &out.User, s); err != nil {
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

func convert_api_Identity_To_v1beta3_Identity(in *userapi.Identity, out *userapiv1beta3.Identity, s conversion.Scope) error {
	return autoconvert_api_Identity_To_v1beta3_Identity(in, out, s)
}

func autoconvert_api_IdentityList_To_v1beta3_IdentityList(in *userapi.IdentityList, out *userapiv1beta3.IdentityList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.IdentityList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapiv1beta3.Identity, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Identity_To_v1beta3_Identity(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_IdentityList_To_v1beta3_IdentityList(in *userapi.IdentityList, out *userapiv1beta3.IdentityList, s conversion.Scope) error {
	return autoconvert_api_IdentityList_To_v1beta3_IdentityList(in, out, s)
}

func autoconvert_api_User_To_v1beta3_User(in *userapi.User, out *userapiv1beta3.User, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.User))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_User_To_v1beta3_User(in *userapi.User, out *userapiv1beta3.User, s conversion.Scope) error {
	return autoconvert_api_User_To_v1beta3_User(in, out, s)
}

func autoconvert_api_UserIdentityMapping_To_v1beta3_UserIdentityMapping(in *userapi.UserIdentityMapping, out *userapiv1beta3.UserIdentityMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.UserIdentityMapping))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1beta3_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := convert_api_ObjectReference_To_v1beta3_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func convert_api_UserIdentityMapping_To_v1beta3_UserIdentityMapping(in *userapi.UserIdentityMapping, out *userapiv1beta3.UserIdentityMapping, s conversion.Scope) error {
	return autoconvert_api_UserIdentityMapping_To_v1beta3_UserIdentityMapping(in, out, s)
}

func autoconvert_api_UserList_To_v1beta3_UserList(in *userapi.UserList, out *userapiv1beta3.UserList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.UserList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapiv1beta3.User, len(in.Items))
		for i := range in.Items {
			if err := convert_api_User_To_v1beta3_User(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_UserList_To_v1beta3_UserList(in *userapi.UserList, out *userapiv1beta3.UserList, s conversion.Scope) error {
	return autoconvert_api_UserList_To_v1beta3_UserList(in, out, s)
}

func autoconvert_v1beta3_Group_To_api_Group(in *userapiv1beta3.Group, out *userapi.Group, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.Group))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1beta3_Group_To_api_Group(in *userapiv1beta3.Group, out *userapi.Group, s conversion.Scope) error {
	return autoconvert_v1beta3_Group_To_api_Group(in, out, s)
}

func autoconvert_v1beta3_GroupList_To_api_GroupList(in *userapiv1beta3.GroupList, out *userapi.GroupList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.GroupList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapi.Group, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_Group_To_api_Group(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_GroupList_To_api_GroupList(in *userapiv1beta3.GroupList, out *userapi.GroupList, s conversion.Scope) error {
	return autoconvert_v1beta3_GroupList_To_api_GroupList(in, out, s)
}

func autoconvert_v1beta3_Identity_To_api_Identity(in *userapiv1beta3.Identity, out *userapi.Identity, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.Identity))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
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

func convert_v1beta3_Identity_To_api_Identity(in *userapiv1beta3.Identity, out *userapi.Identity, s conversion.Scope) error {
	return autoconvert_v1beta3_Identity_To_api_Identity(in, out, s)
}

func autoconvert_v1beta3_IdentityList_To_api_IdentityList(in *userapiv1beta3.IdentityList, out *userapi.IdentityList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.IdentityList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapi.Identity, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_Identity_To_api_Identity(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_IdentityList_To_api_IdentityList(in *userapiv1beta3.IdentityList, out *userapi.IdentityList, s conversion.Scope) error {
	return autoconvert_v1beta3_IdentityList_To_api_IdentityList(in, out, s)
}

func autoconvert_v1beta3_User_To_api_User(in *userapiv1beta3.User, out *userapi.User, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.User))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1beta3_User_To_api_User(in *userapiv1beta3.User, out *userapi.User, s conversion.Scope) error {
	return autoconvert_v1beta3_User_To_api_User(in, out, s)
}

func autoconvert_v1beta3_UserIdentityMapping_To_api_UserIdentityMapping(in *userapiv1beta3.UserIdentityMapping, out *userapi.UserIdentityMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.UserIdentityMapping))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := convert_v1beta3_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func convert_v1beta3_UserIdentityMapping_To_api_UserIdentityMapping(in *userapiv1beta3.UserIdentityMapping, out *userapi.UserIdentityMapping, s conversion.Scope) error {
	return autoconvert_v1beta3_UserIdentityMapping_To_api_UserIdentityMapping(in, out, s)
}

func autoconvert_v1beta3_UserList_To_api_UserList(in *userapiv1beta3.UserList, out *userapi.UserList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1beta3.UserList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]userapi.User, len(in.Items))
		for i := range in.Items {
			if err := convert_v1beta3_User_To_api_User(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1beta3_UserList_To_api_UserList(in *userapiv1beta3.UserList, out *userapi.UserList, s conversion.Scope) error {
	return autoconvert_v1beta3_UserList_To_api_UserList(in, out, s)
}

func autoconvert_api_EnvVar_To_v1beta3_EnvVar(in *pkgapi.EnvVar, out *pkgapiv1beta3.EnvVar, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.EnvVar))(in)
	}
	out.Name = in.Name
	out.Value = in.Value
	if in.ValueFrom != nil {
		out.ValueFrom = new(pkgapiv1beta3.EnvVarSource)
		if err := convert_api_EnvVarSource_To_v1beta3_EnvVarSource(in.ValueFrom, out.ValueFrom, s); err != nil {
			return err
		}
	} else {
		out.ValueFrom = nil
	}
	return nil
}

func convert_api_EnvVar_To_v1beta3_EnvVar(in *pkgapi.EnvVar, out *pkgapiv1beta3.EnvVar, s conversion.Scope) error {
	return autoconvert_api_EnvVar_To_v1beta3_EnvVar(in, out, s)
}

func autoconvert_api_EnvVarSource_To_v1beta3_EnvVarSource(in *pkgapi.EnvVarSource, out *pkgapiv1beta3.EnvVarSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.EnvVarSource))(in)
	}
	if in.FieldRef != nil {
		out.FieldRef = new(pkgapiv1beta3.ObjectFieldSelector)
		if err := convert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector(in.FieldRef, out.FieldRef, s); err != nil {
			return err
		}
	} else {
		out.FieldRef = nil
	}
	return nil
}

func convert_api_EnvVarSource_To_v1beta3_EnvVarSource(in *pkgapi.EnvVarSource, out *pkgapiv1beta3.EnvVarSource, s conversion.Scope) error {
	return autoconvert_api_EnvVarSource_To_v1beta3_EnvVarSource(in, out, s)
}

func autoconvert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in *pkgapi.LocalObjectReference, out *pkgapiv1beta3.LocalObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.LocalObjectReference))(in)
	}
	out.Name = in.Name
	return nil
}

func convert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in *pkgapi.LocalObjectReference, out *pkgapiv1beta3.LocalObjectReference, s conversion.Scope) error {
	return autoconvert_api_LocalObjectReference_To_v1beta3_LocalObjectReference(in, out, s)
}

func autoconvert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector(in *pkgapi.ObjectFieldSelector, out *pkgapiv1beta3.ObjectFieldSelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.ObjectFieldSelector))(in)
	}
	out.APIVersion = in.APIVersion
	out.FieldPath = in.FieldPath
	return nil
}

func convert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector(in *pkgapi.ObjectFieldSelector, out *pkgapiv1beta3.ObjectFieldSelector, s conversion.Scope) error {
	return autoconvert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector(in, out, s)
}

func autoconvert_api_ObjectMeta_To_v1beta3_ObjectMeta(in *pkgapi.ObjectMeta, out *pkgapiv1beta3.ObjectMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.ObjectMeta))(in)
	}
	out.Name = in.Name
	out.GenerateName = in.GenerateName
	out.Namespace = in.Namespace
	out.SelfLink = in.SelfLink
	out.UID = in.UID
	out.ResourceVersion = in.ResourceVersion
	out.Generation = in.Generation
	if err := s.Convert(&in.CreationTimestamp, &out.CreationTimestamp, 0); err != nil {
		return err
	}
	if in.DeletionTimestamp != nil {
		if err := s.Convert(&in.DeletionTimestamp, &out.DeletionTimestamp, 0); err != nil {
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

func convert_api_ObjectMeta_To_v1beta3_ObjectMeta(in *pkgapi.ObjectMeta, out *pkgapiv1beta3.ObjectMeta, s conversion.Scope) error {
	return autoconvert_api_ObjectMeta_To_v1beta3_ObjectMeta(in, out, s)
}

func autoconvert_api_ObjectReference_To_v1beta3_ObjectReference(in *pkgapi.ObjectReference, out *pkgapiv1beta3.ObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.ObjectReference))(in)
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

func convert_api_ObjectReference_To_v1beta3_ObjectReference(in *pkgapi.ObjectReference, out *pkgapiv1beta3.ObjectReference, s conversion.Scope) error {
	return autoconvert_api_ObjectReference_To_v1beta3_ObjectReference(in, out, s)
}

func autoconvert_api_ResourceRequirements_To_v1beta3_ResourceRequirements(in *pkgapi.ResourceRequirements, out *pkgapiv1beta3.ResourceRequirements, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.ResourceRequirements))(in)
	}
	if in.Limits != nil {
		out.Limits = make(pkgapiv1beta3.ResourceList)
		for key, val := range in.Limits {
			newVal := resource.Quantity{}
			if err := s.Convert(&val, &newVal, 0); err != nil {
				return err
			}
			out.Limits[pkgapiv1beta3.ResourceName(key)] = newVal
		}
	} else {
		out.Limits = nil
	}
	if in.Requests != nil {
		out.Requests = make(pkgapiv1beta3.ResourceList)
		for key, val := range in.Requests {
			newVal := resource.Quantity{}
			if err := s.Convert(&val, &newVal, 0); err != nil {
				return err
			}
			out.Requests[pkgapiv1beta3.ResourceName(key)] = newVal
		}
	} else {
		out.Requests = nil
	}
	return nil
}

func convert_api_ResourceRequirements_To_v1beta3_ResourceRequirements(in *pkgapi.ResourceRequirements, out *pkgapiv1beta3.ResourceRequirements, s conversion.Scope) error {
	return autoconvert_api_ResourceRequirements_To_v1beta3_ResourceRequirements(in, out, s)
}

func autoconvert_v1beta3_EnvVar_To_api_EnvVar(in *pkgapiv1beta3.EnvVar, out *pkgapi.EnvVar, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1beta3.EnvVar))(in)
	}
	out.Name = in.Name
	out.Value = in.Value
	if in.ValueFrom != nil {
		out.ValueFrom = new(pkgapi.EnvVarSource)
		if err := convert_v1beta3_EnvVarSource_To_api_EnvVarSource(in.ValueFrom, out.ValueFrom, s); err != nil {
			return err
		}
	} else {
		out.ValueFrom = nil
	}
	return nil
}

func convert_v1beta3_EnvVar_To_api_EnvVar(in *pkgapiv1beta3.EnvVar, out *pkgapi.EnvVar, s conversion.Scope) error {
	return autoconvert_v1beta3_EnvVar_To_api_EnvVar(in, out, s)
}

func autoconvert_v1beta3_EnvVarSource_To_api_EnvVarSource(in *pkgapiv1beta3.EnvVarSource, out *pkgapi.EnvVarSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1beta3.EnvVarSource))(in)
	}
	if in.FieldRef != nil {
		out.FieldRef = new(pkgapi.ObjectFieldSelector)
		if err := convert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector(in.FieldRef, out.FieldRef, s); err != nil {
			return err
		}
	} else {
		out.FieldRef = nil
	}
	return nil
}

func convert_v1beta3_EnvVarSource_To_api_EnvVarSource(in *pkgapiv1beta3.EnvVarSource, out *pkgapi.EnvVarSource, s conversion.Scope) error {
	return autoconvert_v1beta3_EnvVarSource_To_api_EnvVarSource(in, out, s)
}

func autoconvert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in *pkgapiv1beta3.LocalObjectReference, out *pkgapi.LocalObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1beta3.LocalObjectReference))(in)
	}
	out.Name = in.Name
	return nil
}

func convert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in *pkgapiv1beta3.LocalObjectReference, out *pkgapi.LocalObjectReference, s conversion.Scope) error {
	return autoconvert_v1beta3_LocalObjectReference_To_api_LocalObjectReference(in, out, s)
}

func autoconvert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector(in *pkgapiv1beta3.ObjectFieldSelector, out *pkgapi.ObjectFieldSelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1beta3.ObjectFieldSelector))(in)
	}
	out.APIVersion = in.APIVersion
	out.FieldPath = in.FieldPath
	return nil
}

func convert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector(in *pkgapiv1beta3.ObjectFieldSelector, out *pkgapi.ObjectFieldSelector, s conversion.Scope) error {
	return autoconvert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector(in, out, s)
}

func autoconvert_v1beta3_ObjectMeta_To_api_ObjectMeta(in *pkgapiv1beta3.ObjectMeta, out *pkgapi.ObjectMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1beta3.ObjectMeta))(in)
	}
	out.Name = in.Name
	out.GenerateName = in.GenerateName
	out.Namespace = in.Namespace
	out.SelfLink = in.SelfLink
	out.UID = in.UID
	out.ResourceVersion = in.ResourceVersion
	out.Generation = in.Generation
	if err := s.Convert(&in.CreationTimestamp, &out.CreationTimestamp, 0); err != nil {
		return err
	}
	if in.DeletionTimestamp != nil {
		if err := s.Convert(&in.DeletionTimestamp, &out.DeletionTimestamp, 0); err != nil {
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

func convert_v1beta3_ObjectMeta_To_api_ObjectMeta(in *pkgapiv1beta3.ObjectMeta, out *pkgapi.ObjectMeta, s conversion.Scope) error {
	return autoconvert_v1beta3_ObjectMeta_To_api_ObjectMeta(in, out, s)
}

func autoconvert_v1beta3_ObjectReference_To_api_ObjectReference(in *pkgapiv1beta3.ObjectReference, out *pkgapi.ObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1beta3.ObjectReference))(in)
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

func convert_v1beta3_ObjectReference_To_api_ObjectReference(in *pkgapiv1beta3.ObjectReference, out *pkgapi.ObjectReference, s conversion.Scope) error {
	return autoconvert_v1beta3_ObjectReference_To_api_ObjectReference(in, out, s)
}

func autoconvert_v1beta3_ResourceRequirements_To_api_ResourceRequirements(in *pkgapiv1beta3.ResourceRequirements, out *pkgapi.ResourceRequirements, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1beta3.ResourceRequirements))(in)
	}
	if in.Limits != nil {
		out.Limits = make(pkgapi.ResourceList)
		for key, val := range in.Limits {
			newVal := resource.Quantity{}
			if err := s.Convert(&val, &newVal, 0); err != nil {
				return err
			}
			out.Limits[pkgapi.ResourceName(key)] = newVal
		}
	} else {
		out.Limits = nil
	}
	if in.Requests != nil {
		out.Requests = make(pkgapi.ResourceList)
		for key, val := range in.Requests {
			newVal := resource.Quantity{}
			if err := s.Convert(&val, &newVal, 0); err != nil {
				return err
			}
			out.Requests[pkgapi.ResourceName(key)] = newVal
		}
	} else {
		out.Requests = nil
	}
	return nil
}

func convert_v1beta3_ResourceRequirements_To_api_ResourceRequirements(in *pkgapiv1beta3.ResourceRequirements, out *pkgapi.ResourceRequirements, s conversion.Scope) error {
	return autoconvert_v1beta3_ResourceRequirements_To_api_ResourceRequirements(in, out, s)
}

func init() {
	err := pkgapi.Scheme.AddGeneratedConversionFuncs(
		autoconvert_api_BinaryBuildRequestOptions_To_v1beta3_BinaryBuildRequestOptions,
		autoconvert_api_BinaryBuildSource_To_v1beta3_BinaryBuildSource,
		autoconvert_api_BuildConfigList_To_v1beta3_BuildConfigList,
		autoconvert_api_BuildConfigSpec_To_v1beta3_BuildConfigSpec,
		autoconvert_api_BuildConfigStatus_To_v1beta3_BuildConfigStatus,
		autoconvert_api_BuildConfig_To_v1beta3_BuildConfig,
		autoconvert_api_BuildList_To_v1beta3_BuildList,
		autoconvert_api_BuildLogOptions_To_v1beta3_BuildLogOptions,
		autoconvert_api_BuildLog_To_v1beta3_BuildLog,
		autoconvert_api_BuildOutput_To_v1beta3_BuildOutput,
		autoconvert_api_BuildRequest_To_v1beta3_BuildRequest,
		autoconvert_api_BuildSource_To_v1beta3_BuildSource,
		autoconvert_api_BuildSpec_To_v1beta3_BuildSpec,
		autoconvert_api_BuildStatus_To_v1beta3_BuildStatus,
		autoconvert_api_BuildStrategy_To_v1beta3_BuildStrategy,
		autoconvert_api_BuildTriggerPolicy_To_v1beta3_BuildTriggerPolicy,
		autoconvert_api_Build_To_v1beta3_Build,
		autoconvert_api_ClusterNetworkList_To_v1beta3_ClusterNetworkList,
		autoconvert_api_ClusterNetwork_To_v1beta3_ClusterNetwork,
		autoconvert_api_ClusterPolicyBindingList_To_v1beta3_ClusterPolicyBindingList,
		autoconvert_api_ClusterPolicyBinding_To_v1beta3_ClusterPolicyBinding,
		autoconvert_api_ClusterPolicyList_To_v1beta3_ClusterPolicyList,
		autoconvert_api_ClusterPolicy_To_v1beta3_ClusterPolicy,
		autoconvert_api_ClusterRoleBindingList_To_v1beta3_ClusterRoleBindingList,
		autoconvert_api_ClusterRoleBinding_To_v1beta3_ClusterRoleBinding,
		autoconvert_api_ClusterRoleList_To_v1beta3_ClusterRoleList,
		autoconvert_api_ClusterRole_To_v1beta3_ClusterRole,
		autoconvert_api_CustomBuildStrategy_To_v1beta3_CustomBuildStrategy,
		autoconvert_api_DeploymentConfigList_To_v1beta3_DeploymentConfigList,
		autoconvert_api_DeploymentConfigRollbackSpec_To_v1beta3_DeploymentConfigRollbackSpec,
		autoconvert_api_DeploymentConfigRollback_To_v1beta3_DeploymentConfigRollback,
		autoconvert_api_DeploymentConfig_To_v1beta3_DeploymentConfig,
		autoconvert_api_DeploymentLogOptions_To_v1beta3_DeploymentLogOptions,
		autoconvert_api_DeploymentLog_To_v1beta3_DeploymentLog,
		autoconvert_api_DockerBuildStrategy_To_v1beta3_DockerBuildStrategy,
		autoconvert_api_EnvVarSource_To_v1beta3_EnvVarSource,
		autoconvert_api_EnvVar_To_v1beta3_EnvVar,
		autoconvert_api_GitBuildSource_To_v1beta3_GitBuildSource,
		autoconvert_api_GitSourceRevision_To_v1beta3_GitSourceRevision,
		autoconvert_api_GroupList_To_v1beta3_GroupList,
		autoconvert_api_Group_To_v1beta3_Group,
		autoconvert_api_HostSubnetList_To_v1beta3_HostSubnetList,
		autoconvert_api_HostSubnet_To_v1beta3_HostSubnet,
		autoconvert_api_IdentityList_To_v1beta3_IdentityList,
		autoconvert_api_Identity_To_v1beta3_Identity,
		autoconvert_api_ImageChangeTrigger_To_v1beta3_ImageChangeTrigger,
		autoconvert_api_ImageList_To_v1beta3_ImageList,
		autoconvert_api_ImageStreamImage_To_v1beta3_ImageStreamImage,
		autoconvert_api_ImageStreamList_To_v1beta3_ImageStreamList,
		autoconvert_api_ImageStreamMapping_To_v1beta3_ImageStreamMapping,
		autoconvert_api_ImageStreamSpec_To_v1beta3_ImageStreamSpec,
		autoconvert_api_ImageStreamStatus_To_v1beta3_ImageStreamStatus,
		autoconvert_api_ImageStreamTag_To_v1beta3_ImageStreamTag,
		autoconvert_api_ImageStream_To_v1beta3_ImageStream,
		autoconvert_api_Image_To_v1beta3_Image,
		autoconvert_api_IsPersonalSubjectAccessReview_To_v1beta3_IsPersonalSubjectAccessReview,
		autoconvert_api_LocalObjectReference_To_v1beta3_LocalObjectReference,
		autoconvert_api_LocalResourceAccessReview_To_v1beta3_LocalResourceAccessReview,
		autoconvert_api_LocalSubjectAccessReview_To_v1beta3_LocalSubjectAccessReview,
		autoconvert_api_NetNamespaceList_To_v1beta3_NetNamespaceList,
		autoconvert_api_NetNamespace_To_v1beta3_NetNamespace,
		autoconvert_api_OAuthAccessTokenList_To_v1beta3_OAuthAccessTokenList,
		autoconvert_api_OAuthAccessToken_To_v1beta3_OAuthAccessToken,
		autoconvert_api_OAuthAuthorizeTokenList_To_v1beta3_OAuthAuthorizeTokenList,
		autoconvert_api_OAuthAuthorizeToken_To_v1beta3_OAuthAuthorizeToken,
		autoconvert_api_OAuthClientAuthorizationList_To_v1beta3_OAuthClientAuthorizationList,
		autoconvert_api_OAuthClientAuthorization_To_v1beta3_OAuthClientAuthorization,
		autoconvert_api_OAuthClientList_To_v1beta3_OAuthClientList,
		autoconvert_api_OAuthClient_To_v1beta3_OAuthClient,
		autoconvert_api_ObjectFieldSelector_To_v1beta3_ObjectFieldSelector,
		autoconvert_api_ObjectMeta_To_v1beta3_ObjectMeta,
		autoconvert_api_ObjectReference_To_v1beta3_ObjectReference,
		autoconvert_api_Parameter_To_v1beta3_Parameter,
		autoconvert_api_PolicyBindingList_To_v1beta3_PolicyBindingList,
		autoconvert_api_PolicyBinding_To_v1beta3_PolicyBinding,
		autoconvert_api_PolicyList_To_v1beta3_PolicyList,
		autoconvert_api_PolicyRule_To_v1beta3_PolicyRule,
		autoconvert_api_Policy_To_v1beta3_Policy,
		autoconvert_api_ProjectList_To_v1beta3_ProjectList,
		autoconvert_api_ProjectRequest_To_v1beta3_ProjectRequest,
		autoconvert_api_ProjectSpec_To_v1beta3_ProjectSpec,
		autoconvert_api_ProjectStatus_To_v1beta3_ProjectStatus,
		autoconvert_api_Project_To_v1beta3_Project,
		autoconvert_api_ResourceAccessReviewResponse_To_v1beta3_ResourceAccessReviewResponse,
		autoconvert_api_ResourceAccessReview_To_v1beta3_ResourceAccessReview,
		autoconvert_api_ResourceRequirements_To_v1beta3_ResourceRequirements,
		autoconvert_api_RoleBindingList_To_v1beta3_RoleBindingList,
		autoconvert_api_RoleBinding_To_v1beta3_RoleBinding,
		autoconvert_api_RoleList_To_v1beta3_RoleList,
		autoconvert_api_Role_To_v1beta3_Role,
		autoconvert_api_RouteList_To_v1beta3_RouteList,
		autoconvert_api_RoutePort_To_v1beta3_RoutePort,
		autoconvert_api_RouteSpec_To_v1beta3_RouteSpec,
		autoconvert_api_RouteStatus_To_v1beta3_RouteStatus,
		autoconvert_api_Route_To_v1beta3_Route,
		autoconvert_api_SecretSpec_To_v1beta3_SecretSpec,
		autoconvert_api_SourceBuildStrategy_To_v1beta3_SourceBuildStrategy,
		autoconvert_api_SourceControlUser_To_v1beta3_SourceControlUser,
		autoconvert_api_SourceRevision_To_v1beta3_SourceRevision,
		autoconvert_api_SubjectAccessReviewResponse_To_v1beta3_SubjectAccessReviewResponse,
		autoconvert_api_SubjectAccessReview_To_v1beta3_SubjectAccessReview,
		autoconvert_api_TLSConfig_To_v1beta3_TLSConfig,
		autoconvert_api_TemplateList_To_v1beta3_TemplateList,
		autoconvert_api_Template_To_v1beta3_Template,
		autoconvert_api_UserIdentityMapping_To_v1beta3_UserIdentityMapping,
		autoconvert_api_UserList_To_v1beta3_UserList,
		autoconvert_api_User_To_v1beta3_User,
		autoconvert_api_WebHookTrigger_To_v1beta3_WebHookTrigger,
		autoconvert_v1beta3_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions,
		autoconvert_v1beta3_BinaryBuildSource_To_api_BinaryBuildSource,
		autoconvert_v1beta3_BuildConfigList_To_api_BuildConfigList,
		autoconvert_v1beta3_BuildConfigSpec_To_api_BuildConfigSpec,
		autoconvert_v1beta3_BuildConfigStatus_To_api_BuildConfigStatus,
		autoconvert_v1beta3_BuildConfig_To_api_BuildConfig,
		autoconvert_v1beta3_BuildList_To_api_BuildList,
		autoconvert_v1beta3_BuildLogOptions_To_api_BuildLogOptions,
		autoconvert_v1beta3_BuildLog_To_api_BuildLog,
		autoconvert_v1beta3_BuildOutput_To_api_BuildOutput,
		autoconvert_v1beta3_BuildRequest_To_api_BuildRequest,
		autoconvert_v1beta3_BuildSource_To_api_BuildSource,
		autoconvert_v1beta3_BuildSpec_To_api_BuildSpec,
		autoconvert_v1beta3_BuildStatus_To_api_BuildStatus,
		autoconvert_v1beta3_BuildStrategy_To_api_BuildStrategy,
		autoconvert_v1beta3_BuildTriggerPolicy_To_api_BuildTriggerPolicy,
		autoconvert_v1beta3_Build_To_api_Build,
		autoconvert_v1beta3_ClusterNetworkList_To_api_ClusterNetworkList,
		autoconvert_v1beta3_ClusterNetwork_To_api_ClusterNetwork,
		autoconvert_v1beta3_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList,
		autoconvert_v1beta3_ClusterPolicyBinding_To_api_ClusterPolicyBinding,
		autoconvert_v1beta3_ClusterPolicyList_To_api_ClusterPolicyList,
		autoconvert_v1beta3_ClusterPolicy_To_api_ClusterPolicy,
		autoconvert_v1beta3_ClusterRoleBindingList_To_api_ClusterRoleBindingList,
		autoconvert_v1beta3_ClusterRoleBinding_To_api_ClusterRoleBinding,
		autoconvert_v1beta3_ClusterRoleList_To_api_ClusterRoleList,
		autoconvert_v1beta3_ClusterRole_To_api_ClusterRole,
		autoconvert_v1beta3_CustomBuildStrategy_To_api_CustomBuildStrategy,
		autoconvert_v1beta3_DeploymentConfigList_To_api_DeploymentConfigList,
		autoconvert_v1beta3_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec,
		autoconvert_v1beta3_DeploymentConfigRollback_To_api_DeploymentConfigRollback,
		autoconvert_v1beta3_DeploymentConfig_To_api_DeploymentConfig,
		autoconvert_v1beta3_DeploymentLogOptions_To_api_DeploymentLogOptions,
		autoconvert_v1beta3_DeploymentLog_To_api_DeploymentLog,
		autoconvert_v1beta3_DockerBuildStrategy_To_api_DockerBuildStrategy,
		autoconvert_v1beta3_EnvVarSource_To_api_EnvVarSource,
		autoconvert_v1beta3_EnvVar_To_api_EnvVar,
		autoconvert_v1beta3_GitBuildSource_To_api_GitBuildSource,
		autoconvert_v1beta3_GitSourceRevision_To_api_GitSourceRevision,
		autoconvert_v1beta3_GroupList_To_api_GroupList,
		autoconvert_v1beta3_Group_To_api_Group,
		autoconvert_v1beta3_HostSubnetList_To_api_HostSubnetList,
		autoconvert_v1beta3_HostSubnet_To_api_HostSubnet,
		autoconvert_v1beta3_IdentityList_To_api_IdentityList,
		autoconvert_v1beta3_Identity_To_api_Identity,
		autoconvert_v1beta3_ImageChangeTrigger_To_api_ImageChangeTrigger,
		autoconvert_v1beta3_ImageList_To_api_ImageList,
		autoconvert_v1beta3_ImageStreamImage_To_api_ImageStreamImage,
		autoconvert_v1beta3_ImageStreamList_To_api_ImageStreamList,
		autoconvert_v1beta3_ImageStreamMapping_To_api_ImageStreamMapping,
		autoconvert_v1beta3_ImageStreamSpec_To_api_ImageStreamSpec,
		autoconvert_v1beta3_ImageStreamStatus_To_api_ImageStreamStatus,
		autoconvert_v1beta3_ImageStreamTag_To_api_ImageStreamTag,
		autoconvert_v1beta3_ImageStream_To_api_ImageStream,
		autoconvert_v1beta3_Image_To_api_Image,
		autoconvert_v1beta3_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview,
		autoconvert_v1beta3_LocalObjectReference_To_api_LocalObjectReference,
		autoconvert_v1beta3_LocalResourceAccessReview_To_api_LocalResourceAccessReview,
		autoconvert_v1beta3_LocalSubjectAccessReview_To_api_LocalSubjectAccessReview,
		autoconvert_v1beta3_NetNamespaceList_To_api_NetNamespaceList,
		autoconvert_v1beta3_NetNamespace_To_api_NetNamespace,
		autoconvert_v1beta3_OAuthAccessTokenList_To_api_OAuthAccessTokenList,
		autoconvert_v1beta3_OAuthAccessToken_To_api_OAuthAccessToken,
		autoconvert_v1beta3_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList,
		autoconvert_v1beta3_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken,
		autoconvert_v1beta3_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList,
		autoconvert_v1beta3_OAuthClientAuthorization_To_api_OAuthClientAuthorization,
		autoconvert_v1beta3_OAuthClientList_To_api_OAuthClientList,
		autoconvert_v1beta3_OAuthClient_To_api_OAuthClient,
		autoconvert_v1beta3_ObjectFieldSelector_To_api_ObjectFieldSelector,
		autoconvert_v1beta3_ObjectMeta_To_api_ObjectMeta,
		autoconvert_v1beta3_ObjectReference_To_api_ObjectReference,
		autoconvert_v1beta3_Parameter_To_api_Parameter,
		autoconvert_v1beta3_PolicyBindingList_To_api_PolicyBindingList,
		autoconvert_v1beta3_PolicyBinding_To_api_PolicyBinding,
		autoconvert_v1beta3_PolicyList_To_api_PolicyList,
		autoconvert_v1beta3_PolicyRule_To_api_PolicyRule,
		autoconvert_v1beta3_Policy_To_api_Policy,
		autoconvert_v1beta3_ProjectList_To_api_ProjectList,
		autoconvert_v1beta3_ProjectRequest_To_api_ProjectRequest,
		autoconvert_v1beta3_ProjectSpec_To_api_ProjectSpec,
		autoconvert_v1beta3_ProjectStatus_To_api_ProjectStatus,
		autoconvert_v1beta3_Project_To_api_Project,
		autoconvert_v1beta3_ResourceAccessReviewResponse_To_api_ResourceAccessReviewResponse,
		autoconvert_v1beta3_ResourceAccessReview_To_api_ResourceAccessReview,
		autoconvert_v1beta3_ResourceRequirements_To_api_ResourceRequirements,
		autoconvert_v1beta3_RoleBindingList_To_api_RoleBindingList,
		autoconvert_v1beta3_RoleBinding_To_api_RoleBinding,
		autoconvert_v1beta3_RoleList_To_api_RoleList,
		autoconvert_v1beta3_Role_To_api_Role,
		autoconvert_v1beta3_RouteList_To_api_RouteList,
		autoconvert_v1beta3_RoutePort_To_api_RoutePort,
		autoconvert_v1beta3_RouteSpec_To_api_RouteSpec,
		autoconvert_v1beta3_RouteStatus_To_api_RouteStatus,
		autoconvert_v1beta3_Route_To_api_Route,
		autoconvert_v1beta3_SecretSpec_To_api_SecretSpec,
		autoconvert_v1beta3_SourceBuildStrategy_To_api_SourceBuildStrategy,
		autoconvert_v1beta3_SourceControlUser_To_api_SourceControlUser,
		autoconvert_v1beta3_SourceRevision_To_api_SourceRevision,
		autoconvert_v1beta3_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse,
		autoconvert_v1beta3_SubjectAccessReview_To_api_SubjectAccessReview,
		autoconvert_v1beta3_TLSConfig_To_api_TLSConfig,
		autoconvert_v1beta3_TemplateList_To_api_TemplateList,
		autoconvert_v1beta3_Template_To_api_Template,
		autoconvert_v1beta3_UserIdentityMapping_To_api_UserIdentityMapping,
		autoconvert_v1beta3_UserList_To_api_UserList,
		autoconvert_v1beta3_User_To_api_User,
		autoconvert_v1beta3_WebHookTrigger_To_api_WebHookTrigger,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

// AUTO-GENERATED FUNCTIONS END HERE
