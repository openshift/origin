package v1

// AUTO-GENERATED FUNCTIONS START HERE
import (
	api "github.com/openshift/origin/pkg/authorization/api"
	v1 "github.com/openshift/origin/pkg/authorization/api/v1"
	buildapi "github.com/openshift/origin/pkg/build/api"
	apiv1 "github.com/openshift/origin/pkg/build/api/v1"
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
	pkgapi "k8s.io/kubernetes/pkg/api"
	resource "k8s.io/kubernetes/pkg/api/resource"
	pkgapiv1 "k8s.io/kubernetes/pkg/api/v1"
	conversion "k8s.io/kubernetes/pkg/conversion"
	reflect "reflect"
)

func autoconvert_api_ClusterPolicy_To_v1_ClusterPolicy(in *api.ClusterPolicy, out *v1.ClusterPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterPolicy))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func autoconvert_api_ClusterPolicyBinding_To_v1_ClusterPolicyBinding(in *api.ClusterPolicyBinding, out *v1.ClusterPolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterPolicyBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList(in *api.ClusterPolicyBindingList, out *v1.ClusterPolicyBindingList, s conversion.Scope) error {
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
		out.Items = make([]v1.ClusterPolicyBinding, len(in.Items))
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

func convert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList(in *api.ClusterPolicyBindingList, out *v1.ClusterPolicyBindingList, s conversion.Scope) error {
	return autoconvert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList(in, out, s)
}

func autoconvert_api_ClusterPolicyList_To_v1_ClusterPolicyList(in *api.ClusterPolicyList, out *v1.ClusterPolicyList, s conversion.Scope) error {
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
		out.Items = make([]v1.ClusterPolicy, len(in.Items))
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

func convert_api_ClusterPolicyList_To_v1_ClusterPolicyList(in *api.ClusterPolicyList, out *v1.ClusterPolicyList, s conversion.Scope) error {
	return autoconvert_api_ClusterPolicyList_To_v1_ClusterPolicyList(in, out, s)
}

func autoconvert_api_ClusterRole_To_v1_ClusterRole(in *api.ClusterRole, out *v1.ClusterRole, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterRole))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]v1.PolicyRule, len(in.Rules))
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

func convert_api_ClusterRole_To_v1_ClusterRole(in *api.ClusterRole, out *v1.ClusterRole, s conversion.Scope) error {
	return autoconvert_api_ClusterRole_To_v1_ClusterRole(in, out, s)
}

func autoconvert_api_ClusterRoleBinding_To_v1_ClusterRoleBinding(in *api.ClusterRoleBinding, out *v1.ClusterRoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ClusterRoleBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		out.Subjects = make([]pkgapiv1.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList(in *api.ClusterRoleBindingList, out *v1.ClusterRoleBindingList, s conversion.Scope) error {
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
		out.Items = make([]v1.ClusterRoleBinding, len(in.Items))
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

func convert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList(in *api.ClusterRoleBindingList, out *v1.ClusterRoleBindingList, s conversion.Scope) error {
	return autoconvert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList(in, out, s)
}

func autoconvert_api_ClusterRoleList_To_v1_ClusterRoleList(in *api.ClusterRoleList, out *v1.ClusterRoleList, s conversion.Scope) error {
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
		out.Items = make([]v1.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := convert_api_ClusterRole_To_v1_ClusterRole(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ClusterRoleList_To_v1_ClusterRoleList(in *api.ClusterRoleList, out *v1.ClusterRoleList, s conversion.Scope) error {
	return autoconvert_api_ClusterRoleList_To_v1_ClusterRoleList(in, out, s)
}

func autoconvert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(in *api.IsPersonalSubjectAccessReview, out *v1.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.IsPersonalSubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(in *api.IsPersonalSubjectAccessReview, out *v1.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoconvert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview(in, out, s)
}

func autoconvert_api_LocalResourceAccessReview_To_v1_LocalResourceAccessReview(in *api.LocalResourceAccessReview, out *v1.LocalResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.LocalResourceAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.Action has no peer in out
	return nil
}

func autoconvert_api_LocalSubjectAccessReview_To_v1_LocalSubjectAccessReview(in *api.LocalSubjectAccessReview, out *v1.LocalSubjectAccessReview, s conversion.Scope) error {
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

func autoconvert_api_Policy_To_v1_Policy(in *api.Policy, out *v1.Policy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Policy))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func autoconvert_api_PolicyBinding_To_v1_PolicyBinding(in *api.PolicyBinding, out *v1.PolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.PolicyBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_PolicyBindingList_To_v1_PolicyBindingList(in *api.PolicyBindingList, out *v1.PolicyBindingList, s conversion.Scope) error {
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
		out.Items = make([]v1.PolicyBinding, len(in.Items))
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

func convert_api_PolicyBindingList_To_v1_PolicyBindingList(in *api.PolicyBindingList, out *v1.PolicyBindingList, s conversion.Scope) error {
	return autoconvert_api_PolicyBindingList_To_v1_PolicyBindingList(in, out, s)
}

func autoconvert_api_PolicyList_To_v1_PolicyList(in *api.PolicyList, out *v1.PolicyList, s conversion.Scope) error {
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
		out.Items = make([]v1.Policy, len(in.Items))
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

func convert_api_PolicyList_To_v1_PolicyList(in *api.PolicyList, out *v1.PolicyList, s conversion.Scope) error {
	return autoconvert_api_PolicyList_To_v1_PolicyList(in, out, s)
}

func autoconvert_api_PolicyRule_To_v1_PolicyRule(in *api.PolicyRule, out *v1.PolicyRule, s conversion.Scope) error {
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

func autoconvert_api_ResourceAccessReview_To_v1_ResourceAccessReview(in *api.ResourceAccessReview, out *v1.ResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.ResourceAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.Action has no peer in out
	return nil
}

func autoconvert_api_ResourceAccessReviewResponse_To_v1_ResourceAccessReviewResponse(in *api.ResourceAccessReviewResponse, out *v1.ResourceAccessReviewResponse, s conversion.Scope) error {
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

func autoconvert_api_Role_To_v1_Role(in *api.Role, out *v1.Role, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.Role))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Rules != nil {
		out.Rules = make([]v1.PolicyRule, len(in.Rules))
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

func convert_api_Role_To_v1_Role(in *api.Role, out *v1.Role, s conversion.Scope) error {
	return autoconvert_api_Role_To_v1_Role(in, out, s)
}

func autoconvert_api_RoleBinding_To_v1_RoleBinding(in *api.RoleBinding, out *v1.RoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*api.RoleBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Subjects != nil {
		out.Subjects = make([]pkgapiv1.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_RoleBindingList_To_v1_RoleBindingList(in *api.RoleBindingList, out *v1.RoleBindingList, s conversion.Scope) error {
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
		out.Items = make([]v1.RoleBinding, len(in.Items))
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

func convert_api_RoleBindingList_To_v1_RoleBindingList(in *api.RoleBindingList, out *v1.RoleBindingList, s conversion.Scope) error {
	return autoconvert_api_RoleBindingList_To_v1_RoleBindingList(in, out, s)
}

func autoconvert_api_RoleList_To_v1_RoleList(in *api.RoleList, out *v1.RoleList, s conversion.Scope) error {
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
		out.Items = make([]v1.Role, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Role_To_v1_Role(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_RoleList_To_v1_RoleList(in *api.RoleList, out *v1.RoleList, s conversion.Scope) error {
	return autoconvert_api_RoleList_To_v1_RoleList(in, out, s)
}

func autoconvert_api_SubjectAccessReview_To_v1_SubjectAccessReview(in *api.SubjectAccessReview, out *v1.SubjectAccessReview, s conversion.Scope) error {
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

func autoconvert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse(in *api.SubjectAccessReviewResponse, out *v1.SubjectAccessReviewResponse, s conversion.Scope) error {
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

func convert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse(in *api.SubjectAccessReviewResponse, out *v1.SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoconvert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse(in, out, s)
}

func autoconvert_v1_ClusterPolicy_To_api_ClusterPolicy(in *v1.ClusterPolicy, out *api.ClusterPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ClusterPolicy))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func autoconvert_v1_ClusterPolicyBinding_To_api_ClusterPolicyBinding(in *v1.ClusterPolicyBinding, out *api.ClusterPolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ClusterPolicyBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *v1.ClusterPolicyBindingList, out *api.ClusterPolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ClusterPolicyBindingList))(in)
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

func convert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in *v1.ClusterPolicyBindingList, out *api.ClusterPolicyBindingList, s conversion.Scope) error {
	return autoconvert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList(in, out, s)
}

func autoconvert_v1_ClusterPolicyList_To_api_ClusterPolicyList(in *v1.ClusterPolicyList, out *api.ClusterPolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ClusterPolicyList))(in)
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
			if err := s.Convert(&in.Items[i], &out.Items[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_ClusterPolicyList_To_api_ClusterPolicyList(in *v1.ClusterPolicyList, out *api.ClusterPolicyList, s conversion.Scope) error {
	return autoconvert_v1_ClusterPolicyList_To_api_ClusterPolicyList(in, out, s)
}

func autoconvert_v1_ClusterRole_To_api_ClusterRole(in *v1.ClusterRole, out *api.ClusterRole, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ClusterRole))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_ClusterRole_To_api_ClusterRole(in *v1.ClusterRole, out *api.ClusterRole, s conversion.Scope) error {
	return autoconvert_v1_ClusterRole_To_api_ClusterRole(in, out, s)
}

func autoconvert_v1_ClusterRoleBinding_To_api_ClusterRoleBinding(in *v1.ClusterRoleBinding, out *api.ClusterRoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ClusterRoleBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.UserNames has no peer in out
	// in.GroupNames has no peer in out
	if in.Subjects != nil {
		out.Subjects = make([]pkgapi.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *v1.ClusterRoleBindingList, out *api.ClusterRoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ClusterRoleBindingList))(in)
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

func convert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in *v1.ClusterRoleBindingList, out *api.ClusterRoleBindingList, s conversion.Scope) error {
	return autoconvert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList(in, out, s)
}

func autoconvert_v1_ClusterRoleList_To_api_ClusterRoleList(in *v1.ClusterRoleList, out *api.ClusterRoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ClusterRoleList))(in)
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
			if err := convert_v1_ClusterRole_To_api_ClusterRole(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_ClusterRoleList_To_api_ClusterRoleList(in *v1.ClusterRoleList, out *api.ClusterRoleList, s conversion.Scope) error {
	return autoconvert_v1_ClusterRoleList_To_api_ClusterRoleList(in, out, s)
}

func autoconvert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *v1.IsPersonalSubjectAccessReview, out *api.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.IsPersonalSubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in *v1.IsPersonalSubjectAccessReview, out *api.IsPersonalSubjectAccessReview, s conversion.Scope) error {
	return autoconvert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview(in, out, s)
}

func autoconvert_v1_LocalResourceAccessReview_To_api_LocalResourceAccessReview(in *v1.LocalResourceAccessReview, out *api.LocalResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.LocalResourceAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.AuthorizationAttributes has no peer in out
	return nil
}

func autoconvert_v1_LocalSubjectAccessReview_To_api_LocalSubjectAccessReview(in *v1.LocalSubjectAccessReview, out *api.LocalSubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.LocalSubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.AuthorizationAttributes has no peer in out
	out.User = in.User
	// in.GroupsSlice has no peer in out
	return nil
}

func autoconvert_v1_Policy_To_api_Policy(in *v1.Policy, out *api.Policy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.Policy))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func autoconvert_v1_PolicyBinding_To_api_PolicyBinding(in *v1.PolicyBinding, out *api.PolicyBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.PolicyBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.LastModified, &out.LastModified, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.PolicyRef, &out.PolicyRef, s); err != nil {
		return err
	}
	if err := s.Convert(&in.RoleBindings, &out.RoleBindings, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1_PolicyBindingList_To_api_PolicyBindingList(in *v1.PolicyBindingList, out *api.PolicyBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.PolicyBindingList))(in)
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

func convert_v1_PolicyBindingList_To_api_PolicyBindingList(in *v1.PolicyBindingList, out *api.PolicyBindingList, s conversion.Scope) error {
	return autoconvert_v1_PolicyBindingList_To_api_PolicyBindingList(in, out, s)
}

func autoconvert_v1_PolicyList_To_api_PolicyList(in *v1.PolicyList, out *api.PolicyList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.PolicyList))(in)
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

func convert_v1_PolicyList_To_api_PolicyList(in *v1.PolicyList, out *api.PolicyList, s conversion.Scope) error {
	return autoconvert_v1_PolicyList_To_api_PolicyList(in, out, s)
}

func autoconvert_v1_PolicyRule_To_api_PolicyRule(in *v1.PolicyRule, out *api.PolicyRule, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.PolicyRule))(in)
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
	// in.NonResourceURLsSlice has no peer in out
	return nil
}

func autoconvert_v1_ResourceAccessReview_To_api_ResourceAccessReview(in *v1.ResourceAccessReview, out *api.ResourceAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ResourceAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.AuthorizationAttributes has no peer in out
	return nil
}

func autoconvert_v1_ResourceAccessReviewResponse_To_api_ResourceAccessReviewResponse(in *v1.ResourceAccessReviewResponse, out *api.ResourceAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.ResourceAccessReviewResponse))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Namespace = in.Namespace
	// in.UsersSlice has no peer in out
	// in.GroupsSlice has no peer in out
	return nil
}

func autoconvert_v1_Role_To_api_Role(in *v1.Role, out *api.Role, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.Role))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_Role_To_api_Role(in *v1.Role, out *api.Role, s conversion.Scope) error {
	return autoconvert_v1_Role_To_api_Role(in, out, s)
}

func autoconvert_v1_RoleBinding_To_api_RoleBinding(in *v1.RoleBinding, out *api.RoleBinding, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.RoleBinding))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.UserNames has no peer in out
	// in.GroupNames has no peer in out
	if in.Subjects != nil {
		out.Subjects = make([]pkgapi.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.Subjects[i], &out.Subjects[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Subjects = nil
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.RoleRef, &out.RoleRef, s); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1_RoleBindingList_To_api_RoleBindingList(in *v1.RoleBindingList, out *api.RoleBindingList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.RoleBindingList))(in)
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

func convert_v1_RoleBindingList_To_api_RoleBindingList(in *v1.RoleBindingList, out *api.RoleBindingList, s conversion.Scope) error {
	return autoconvert_v1_RoleBindingList_To_api_RoleBindingList(in, out, s)
}

func autoconvert_v1_RoleList_To_api_RoleList(in *v1.RoleList, out *api.RoleList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.RoleList))(in)
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
			if err := convert_v1_Role_To_api_Role(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_RoleList_To_api_RoleList(in *v1.RoleList, out *api.RoleList, s conversion.Scope) error {
	return autoconvert_v1_RoleList_To_api_RoleList(in, out, s)
}

func autoconvert_v1_SubjectAccessReview_To_api_SubjectAccessReview(in *v1.SubjectAccessReview, out *api.SubjectAccessReview, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.SubjectAccessReview))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	// in.AuthorizationAttributes has no peer in out
	out.User = in.User
	// in.GroupsSlice has no peer in out
	return nil
}

func autoconvert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *v1.SubjectAccessReviewResponse, out *api.SubjectAccessReviewResponse, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*v1.SubjectAccessReviewResponse))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func convert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in *v1.SubjectAccessReviewResponse, out *api.SubjectAccessReviewResponse, s conversion.Scope) error {
	return autoconvert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse(in, out, s)
}

func autoconvert_api_BinaryBuildRequestOptions_To_v1_BinaryBuildRequestOptions(in *buildapi.BinaryBuildRequestOptions, out *apiv1.BinaryBuildRequestOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BinaryBuildRequestOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_BinaryBuildRequestOptions_To_v1_BinaryBuildRequestOptions(in *buildapi.BinaryBuildRequestOptions, out *apiv1.BinaryBuildRequestOptions, s conversion.Scope) error {
	return autoconvert_api_BinaryBuildRequestOptions_To_v1_BinaryBuildRequestOptions(in, out, s)
}

func autoconvert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in *buildapi.BinaryBuildSource, out *apiv1.BinaryBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BinaryBuildSource))(in)
	}
	out.AsFile = in.AsFile
	return nil
}

func convert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in *buildapi.BinaryBuildSource, out *apiv1.BinaryBuildSource, s conversion.Scope) error {
	return autoconvert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in, out, s)
}

func autoconvert_api_Build_To_v1_Build(in *buildapi.Build, out *apiv1.Build, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.Build))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_BuildSpec_To_v1_BuildSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_api_BuildStatus_To_v1_BuildStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_api_Build_To_v1_Build(in *buildapi.Build, out *apiv1.Build, s conversion.Scope) error {
	return autoconvert_api_Build_To_v1_Build(in, out, s)
}

func autoconvert_api_BuildConfig_To_v1_BuildConfig(in *buildapi.BuildConfig, out *apiv1.BuildConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfig))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_BuildConfigSpec_To_v1_BuildConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_api_BuildConfigStatus_To_v1_BuildConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_api_BuildConfig_To_v1_BuildConfig(in *buildapi.BuildConfig, out *apiv1.BuildConfig, s conversion.Scope) error {
	return autoconvert_api_BuildConfig_To_v1_BuildConfig(in, out, s)
}

func autoconvert_api_BuildConfigList_To_v1_BuildConfigList(in *buildapi.BuildConfigList, out *apiv1.BuildConfigList, s conversion.Scope) error {
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
		out.Items = make([]apiv1.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := convert_api_BuildConfig_To_v1_BuildConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_BuildConfigList_To_v1_BuildConfigList(in *buildapi.BuildConfigList, out *apiv1.BuildConfigList, s conversion.Scope) error {
	return autoconvert_api_BuildConfigList_To_v1_BuildConfigList(in, out, s)
}

func autoconvert_api_BuildConfigSpec_To_v1_BuildConfigSpec(in *buildapi.BuildConfigSpec, out *apiv1.BuildConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigSpec))(in)
	}
	if in.Triggers != nil {
		out.Triggers = make([]apiv1.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := s.Convert(&in.Triggers[i], &out.Triggers[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := convert_api_BuildSpec_To_v1_BuildSpec(&in.BuildSpec, &out.BuildSpec, s); err != nil {
		return err
	}
	return nil
}

func convert_api_BuildConfigSpec_To_v1_BuildConfigSpec(in *buildapi.BuildConfigSpec, out *apiv1.BuildConfigSpec, s conversion.Scope) error {
	return autoconvert_api_BuildConfigSpec_To_v1_BuildConfigSpec(in, out, s)
}

func autoconvert_api_BuildConfigStatus_To_v1_BuildConfigStatus(in *buildapi.BuildConfigStatus, out *apiv1.BuildConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildConfigStatus))(in)
	}
	out.LastVersion = in.LastVersion
	return nil
}

func convert_api_BuildConfigStatus_To_v1_BuildConfigStatus(in *buildapi.BuildConfigStatus, out *apiv1.BuildConfigStatus, s conversion.Scope) error {
	return autoconvert_api_BuildConfigStatus_To_v1_BuildConfigStatus(in, out, s)
}

func autoconvert_api_BuildList_To_v1_BuildList(in *buildapi.BuildList, out *apiv1.BuildList, s conversion.Scope) error {
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
		out.Items = make([]apiv1.Build, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Build_To_v1_Build(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_BuildList_To_v1_BuildList(in *buildapi.BuildList, out *apiv1.BuildList, s conversion.Scope) error {
	return autoconvert_api_BuildList_To_v1_BuildList(in, out, s)
}

func autoconvert_api_BuildLog_To_v1_BuildLog(in *buildapi.BuildLog, out *apiv1.BuildLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildLog))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_BuildLog_To_v1_BuildLog(in *buildapi.BuildLog, out *apiv1.BuildLog, s conversion.Scope) error {
	return autoconvert_api_BuildLog_To_v1_BuildLog(in, out, s)
}

func autoconvert_api_BuildLogOptions_To_v1_BuildLogOptions(in *buildapi.BuildLogOptions, out *apiv1.BuildLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildLogOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
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
	if in.SinceTime != nil {
		if err := s.Convert(&in.SinceTime, &out.SinceTime, 0); err != nil {
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

func convert_api_BuildLogOptions_To_v1_BuildLogOptions(in *buildapi.BuildLogOptions, out *apiv1.BuildLogOptions, s conversion.Scope) error {
	return autoconvert_api_BuildLogOptions_To_v1_BuildLogOptions(in, out, s)
}

func autoconvert_api_BuildOutput_To_v1_BuildOutput(in *buildapi.BuildOutput, out *apiv1.BuildOutput, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildOutput))(in)
	}
	if in.To != nil {
		out.To = new(pkgapiv1.ObjectReference)
		if err := convert_api_ObjectReference_To_v1_ObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	if in.PushSecret != nil {
		out.PushSecret = new(pkgapiv1.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PushSecret, out.PushSecret, s); err != nil {
			return err
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func autoconvert_api_BuildRequest_To_v1_BuildRequest(in *buildapi.BuildRequest, out *apiv1.BuildRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildRequest))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Revision != nil {
		if err := s.Convert(&in.Revision, &out.Revision, 0); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if in.TriggeredByImage != nil {
		out.TriggeredByImage = new(pkgapiv1.ObjectReference)
		if err := convert_api_ObjectReference_To_v1_ObjectReference(in.TriggeredByImage, out.TriggeredByImage, s); err != nil {
			return err
		}
	} else {
		out.TriggeredByImage = nil
	}
	if in.From != nil {
		out.From = new(pkgapiv1.ObjectReference)
		if err := convert_api_ObjectReference_To_v1_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.Binary != nil {
		out.Binary = new(apiv1.BinaryBuildSource)
		if err := convert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
		out.Env = make([]pkgapiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	return nil
}

func convert_api_BuildRequest_To_v1_BuildRequest(in *buildapi.BuildRequest, out *apiv1.BuildRequest, s conversion.Scope) error {
	return autoconvert_api_BuildRequest_To_v1_BuildRequest(in, out, s)
}

func autoconvert_api_BuildSource_To_v1_BuildSource(in *buildapi.BuildSource, out *apiv1.BuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildSource))(in)
	}
	if in.Binary != nil {
		out.Binary = new(apiv1.BinaryBuildSource)
		if err := convert_api_BinaryBuildSource_To_v1_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
		out.Git = new(apiv1.GitBuildSource)
		if err := convert_api_GitBuildSource_To_v1_GitBuildSource(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	if in.Image != nil {
		out.Image = new(apiv1.ImageSource)
		if err := convert_api_ImageSource_To_v1_ImageSource(in.Image, out.Image, s); err != nil {
			return err
		}
	} else {
		out.Image = nil
	}
	out.ContextDir = in.ContextDir
	if in.SourceSecret != nil {
		out.SourceSecret = new(pkgapiv1.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.SourceSecret, out.SourceSecret, s); err != nil {
			return err
		}
	} else {
		out.SourceSecret = nil
	}
	return nil
}

func autoconvert_api_BuildSpec_To_v1_BuildSpec(in *buildapi.BuildSpec, out *apiv1.BuildSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildSpec))(in)
	}
	out.ServiceAccount = in.ServiceAccount
	if err := s.Convert(&in.Source, &out.Source, 0); err != nil {
		return err
	}
	if in.Revision != nil {
		if err := s.Convert(&in.Revision, &out.Revision, 0); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := s.Convert(&in.Strategy, &out.Strategy, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Output, &out.Output, 0); err != nil {
		return err
	}
	if err := convert_api_ResourceRequirements_To_v1_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
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

func convert_api_BuildSpec_To_v1_BuildSpec(in *buildapi.BuildSpec, out *apiv1.BuildSpec, s conversion.Scope) error {
	return autoconvert_api_BuildSpec_To_v1_BuildSpec(in, out, s)
}

func autoconvert_api_BuildStatus_To_v1_BuildStatus(in *buildapi.BuildStatus, out *apiv1.BuildStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildStatus))(in)
	}
	out.Phase = apiv1.BuildPhase(in.Phase)
	out.Cancelled = in.Cancelled
	out.Reason = apiv1.StatusReason(in.Reason)
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
		out.Config = new(pkgapiv1.ObjectReference)
		if err := convert_api_ObjectReference_To_v1_ObjectReference(in.Config, out.Config, s); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	return nil
}

func convert_api_BuildStatus_To_v1_BuildStatus(in *buildapi.BuildStatus, out *apiv1.BuildStatus, s conversion.Scope) error {
	return autoconvert_api_BuildStatus_To_v1_BuildStatus(in, out, s)
}

func autoconvert_api_BuildStrategy_To_v1_BuildStrategy(in *buildapi.BuildStrategy, out *apiv1.BuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildStrategy))(in)
	}
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

func autoconvert_api_BuildTriggerPolicy_To_v1_BuildTriggerPolicy(in *buildapi.BuildTriggerPolicy, out *apiv1.BuildTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.BuildTriggerPolicy))(in)
	}
	out.Type = apiv1.BuildTriggerType(in.Type)
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(apiv1.WebHookTrigger)
		if err := convert_api_WebHookTrigger_To_v1_WebHookTrigger(in.GitHubWebHook, out.GitHubWebHook, s); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(apiv1.WebHookTrigger)
		if err := convert_api_WebHookTrigger_To_v1_WebHookTrigger(in.GenericWebHook, out.GenericWebHook, s); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	if in.ImageChange != nil {
		out.ImageChange = new(apiv1.ImageChangeTrigger)
		if err := convert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger(in.ImageChange, out.ImageChange, s); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func autoconvert_api_CustomBuildStrategy_To_v1_CustomBuildStrategy(in *buildapi.CustomBuildStrategy, out *apiv1.CustomBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.CustomBuildStrategy))(in)
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapiv1.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	out.ExposeDockerSocket = in.ExposeDockerSocket
	out.ForcePull = in.ForcePull
	if in.Secrets != nil {
		out.Secrets = make([]apiv1.SecretSpec, len(in.Secrets))
		for i := range in.Secrets {
			if err := convert_api_SecretSpec_To_v1_SecretSpec(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func autoconvert_api_DockerBuildStrategy_To_v1_DockerBuildStrategy(in *buildapi.DockerBuildStrategy, out *apiv1.DockerBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.DockerBuildStrategy))(in)
	}
	if in.From != nil {
		out.From = new(pkgapiv1.ObjectReference)
		if err := convert_api_ObjectReference_To_v1_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapiv1.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]pkgapiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoconvert_api_GitBuildSource_To_v1_GitBuildSource(in *buildapi.GitBuildSource, out *apiv1.GitBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.GitBuildSource))(in)
	}
	out.URI = in.URI
	out.Ref = in.Ref
	out.HTTPProxy = in.HTTPProxy
	out.HTTPSProxy = in.HTTPSProxy
	return nil
}

func convert_api_GitBuildSource_To_v1_GitBuildSource(in *buildapi.GitBuildSource, out *apiv1.GitBuildSource, s conversion.Scope) error {
	return autoconvert_api_GitBuildSource_To_v1_GitBuildSource(in, out, s)
}

func autoconvert_api_GitSourceRevision_To_v1_GitSourceRevision(in *buildapi.GitSourceRevision, out *apiv1.GitSourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.GitSourceRevision))(in)
	}
	out.Commit = in.Commit
	if err := convert_api_SourceControlUser_To_v1_SourceControlUser(&in.Author, &out.Author, s); err != nil {
		return err
	}
	if err := convert_api_SourceControlUser_To_v1_SourceControlUser(&in.Committer, &out.Committer, s); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func convert_api_GitSourceRevision_To_v1_GitSourceRevision(in *buildapi.GitSourceRevision, out *apiv1.GitSourceRevision, s conversion.Scope) error {
	return autoconvert_api_GitSourceRevision_To_v1_GitSourceRevision(in, out, s)
}

func autoconvert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger(in *buildapi.ImageChangeTrigger, out *apiv1.ImageChangeTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageChangeTrigger))(in)
	}
	out.LastTriggeredImageID = in.LastTriggeredImageID
	if in.From != nil {
		out.From = new(pkgapiv1.ObjectReference)
		if err := convert_api_ObjectReference_To_v1_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	return nil
}

func convert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger(in *buildapi.ImageChangeTrigger, out *apiv1.ImageChangeTrigger, s conversion.Scope) error {
	return autoconvert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger(in, out, s)
}

func autoconvert_api_ImageSource_To_v1_ImageSource(in *buildapi.ImageSource, out *apiv1.ImageSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageSource))(in)
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.Paths != nil {
		out.Paths = make([]apiv1.ImageSourcePath, len(in.Paths))
		for i := range in.Paths {
			if err := convert_api_ImageSourcePath_To_v1_ImageSourcePath(&in.Paths[i], &out.Paths[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Paths = nil
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapiv1.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	return nil
}

func convert_api_ImageSource_To_v1_ImageSource(in *buildapi.ImageSource, out *apiv1.ImageSource, s conversion.Scope) error {
	return autoconvert_api_ImageSource_To_v1_ImageSource(in, out, s)
}

func autoconvert_api_ImageSourcePath_To_v1_ImageSourcePath(in *buildapi.ImageSourcePath, out *apiv1.ImageSourcePath, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.ImageSourcePath))(in)
	}
	out.SourcePath = in.SourcePath
	out.DestinationDir = in.DestinationDir
	return nil
}

func convert_api_ImageSourcePath_To_v1_ImageSourcePath(in *buildapi.ImageSourcePath, out *apiv1.ImageSourcePath, s conversion.Scope) error {
	return autoconvert_api_ImageSourcePath_To_v1_ImageSourcePath(in, out, s)
}

func autoconvert_api_SecretSpec_To_v1_SecretSpec(in *buildapi.SecretSpec, out *apiv1.SecretSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SecretSpec))(in)
	}
	if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(&in.SecretSource, &out.SecretSource, s); err != nil {
		return err
	}
	out.MountPath = in.MountPath
	return nil
}

func convert_api_SecretSpec_To_v1_SecretSpec(in *buildapi.SecretSpec, out *apiv1.SecretSpec, s conversion.Scope) error {
	return autoconvert_api_SecretSpec_To_v1_SecretSpec(in, out, s)
}

func autoconvert_api_SourceBuildStrategy_To_v1_SourceBuildStrategy(in *buildapi.SourceBuildStrategy, out *apiv1.SourceBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceBuildStrategy))(in)
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapiv1.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoconvert_api_SourceControlUser_To_v1_SourceControlUser(in *buildapi.SourceControlUser, out *apiv1.SourceControlUser, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceControlUser))(in)
	}
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func convert_api_SourceControlUser_To_v1_SourceControlUser(in *buildapi.SourceControlUser, out *apiv1.SourceControlUser, s conversion.Scope) error {
	return autoconvert_api_SourceControlUser_To_v1_SourceControlUser(in, out, s)
}

func autoconvert_api_SourceRevision_To_v1_SourceRevision(in *buildapi.SourceRevision, out *apiv1.SourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.SourceRevision))(in)
	}
	if in.Git != nil {
		out.Git = new(apiv1.GitSourceRevision)
		if err := convert_api_GitSourceRevision_To_v1_GitSourceRevision(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func autoconvert_api_WebHookTrigger_To_v1_WebHookTrigger(in *buildapi.WebHookTrigger, out *apiv1.WebHookTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*buildapi.WebHookTrigger))(in)
	}
	out.Secret = in.Secret
	return nil
}

func convert_api_WebHookTrigger_To_v1_WebHookTrigger(in *buildapi.WebHookTrigger, out *apiv1.WebHookTrigger, s conversion.Scope) error {
	return autoconvert_api_WebHookTrigger_To_v1_WebHookTrigger(in, out, s)
}

func autoconvert_v1_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in *apiv1.BinaryBuildRequestOptions, out *buildapi.BinaryBuildRequestOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BinaryBuildRequestOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in *apiv1.BinaryBuildRequestOptions, out *buildapi.BinaryBuildRequestOptions, s conversion.Scope) error {
	return autoconvert_v1_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions(in, out, s)
}

func autoconvert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in *apiv1.BinaryBuildSource, out *buildapi.BinaryBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BinaryBuildSource))(in)
	}
	out.AsFile = in.AsFile
	return nil
}

func convert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in *apiv1.BinaryBuildSource, out *buildapi.BinaryBuildSource, s conversion.Scope) error {
	return autoconvert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in, out, s)
}

func autoconvert_v1_Build_To_api_Build(in *apiv1.Build, out *buildapi.Build, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.Build))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1_BuildSpec_To_api_BuildSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1_BuildStatus_To_api_BuildStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_Build_To_api_Build(in *apiv1.Build, out *buildapi.Build, s conversion.Scope) error {
	return autoconvert_v1_Build_To_api_Build(in, out, s)
}

func autoconvert_v1_BuildConfig_To_api_BuildConfig(in *apiv1.BuildConfig, out *buildapi.BuildConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildConfig))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1_BuildConfigSpec_To_api_BuildConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1_BuildConfigStatus_To_api_BuildConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_BuildConfig_To_api_BuildConfig(in *apiv1.BuildConfig, out *buildapi.BuildConfig, s conversion.Scope) error {
	return autoconvert_v1_BuildConfig_To_api_BuildConfig(in, out, s)
}

func autoconvert_v1_BuildConfigList_To_api_BuildConfigList(in *apiv1.BuildConfigList, out *buildapi.BuildConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildConfigList))(in)
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
			if err := convert_v1_BuildConfig_To_api_BuildConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_BuildConfigList_To_api_BuildConfigList(in *apiv1.BuildConfigList, out *buildapi.BuildConfigList, s conversion.Scope) error {
	return autoconvert_v1_BuildConfigList_To_api_BuildConfigList(in, out, s)
}

func autoconvert_v1_BuildConfigSpec_To_api_BuildConfigSpec(in *apiv1.BuildConfigSpec, out *buildapi.BuildConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildConfigSpec))(in)
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
	if err := convert_v1_BuildSpec_To_api_BuildSpec(&in.BuildSpec, &out.BuildSpec, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_BuildConfigSpec_To_api_BuildConfigSpec(in *apiv1.BuildConfigSpec, out *buildapi.BuildConfigSpec, s conversion.Scope) error {
	return autoconvert_v1_BuildConfigSpec_To_api_BuildConfigSpec(in, out, s)
}

func autoconvert_v1_BuildConfigStatus_To_api_BuildConfigStatus(in *apiv1.BuildConfigStatus, out *buildapi.BuildConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildConfigStatus))(in)
	}
	out.LastVersion = in.LastVersion
	return nil
}

func convert_v1_BuildConfigStatus_To_api_BuildConfigStatus(in *apiv1.BuildConfigStatus, out *buildapi.BuildConfigStatus, s conversion.Scope) error {
	return autoconvert_v1_BuildConfigStatus_To_api_BuildConfigStatus(in, out, s)
}

func autoconvert_v1_BuildList_To_api_BuildList(in *apiv1.BuildList, out *buildapi.BuildList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildList))(in)
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
			if err := convert_v1_Build_To_api_Build(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_BuildList_To_api_BuildList(in *apiv1.BuildList, out *buildapi.BuildList, s conversion.Scope) error {
	return autoconvert_v1_BuildList_To_api_BuildList(in, out, s)
}

func autoconvert_v1_BuildLog_To_api_BuildLog(in *apiv1.BuildLog, out *buildapi.BuildLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildLog))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_BuildLog_To_api_BuildLog(in *apiv1.BuildLog, out *buildapi.BuildLog, s conversion.Scope) error {
	return autoconvert_v1_BuildLog_To_api_BuildLog(in, out, s)
}

func autoconvert_v1_BuildLogOptions_To_api_BuildLogOptions(in *apiv1.BuildLogOptions, out *buildapi.BuildLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildLogOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
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
	if in.SinceTime != nil {
		if err := s.Convert(&in.SinceTime, &out.SinceTime, 0); err != nil {
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

func convert_v1_BuildLogOptions_To_api_BuildLogOptions(in *apiv1.BuildLogOptions, out *buildapi.BuildLogOptions, s conversion.Scope) error {
	return autoconvert_v1_BuildLogOptions_To_api_BuildLogOptions(in, out, s)
}

func autoconvert_v1_BuildOutput_To_api_BuildOutput(in *apiv1.BuildOutput, out *buildapi.BuildOutput, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildOutput))(in)
	}
	if in.To != nil {
		out.To = new(pkgapi.ObjectReference)
		if err := convert_v1_ObjectReference_To_api_ObjectReference(in.To, out.To, s); err != nil {
			return err
		}
	} else {
		out.To = nil
	}
	if in.PushSecret != nil {
		out.PushSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PushSecret, out.PushSecret, s); err != nil {
			return err
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func autoconvert_v1_BuildRequest_To_api_BuildRequest(in *apiv1.BuildRequest, out *buildapi.BuildRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildRequest))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Revision != nil {
		if err := s.Convert(&in.Revision, &out.Revision, 0); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if in.TriggeredByImage != nil {
		out.TriggeredByImage = new(pkgapi.ObjectReference)
		if err := convert_v1_ObjectReference_To_api_ObjectReference(in.TriggeredByImage, out.TriggeredByImage, s); err != nil {
			return err
		}
	} else {
		out.TriggeredByImage = nil
	}
	if in.From != nil {
		out.From = new(pkgapi.ObjectReference)
		if err := convert_v1_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.Binary != nil {
		out.Binary = new(buildapi.BinaryBuildSource)
		if err := convert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	return nil
}

func convert_v1_BuildRequest_To_api_BuildRequest(in *apiv1.BuildRequest, out *buildapi.BuildRequest, s conversion.Scope) error {
	return autoconvert_v1_BuildRequest_To_api_BuildRequest(in, out, s)
}

func autoconvert_v1_BuildSource_To_api_BuildSource(in *apiv1.BuildSource, out *buildapi.BuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildSource))(in)
	}
	// in.Type has no peer in out
	if in.Binary != nil {
		out.Binary = new(buildapi.BinaryBuildSource)
		if err := convert_v1_BinaryBuildSource_To_api_BinaryBuildSource(in.Binary, out.Binary, s); err != nil {
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
		if err := convert_v1_GitBuildSource_To_api_GitBuildSource(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	if in.Image != nil {
		out.Image = new(buildapi.ImageSource)
		if err := convert_v1_ImageSource_To_api_ImageSource(in.Image, out.Image, s); err != nil {
			return err
		}
	} else {
		out.Image = nil
	}
	out.ContextDir = in.ContextDir
	if in.SourceSecret != nil {
		out.SourceSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.SourceSecret, out.SourceSecret, s); err != nil {
			return err
		}
	} else {
		out.SourceSecret = nil
	}
	return nil
}

func autoconvert_v1_BuildSpec_To_api_BuildSpec(in *apiv1.BuildSpec, out *buildapi.BuildSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildSpec))(in)
	}
	out.ServiceAccount = in.ServiceAccount
	if err := s.Convert(&in.Source, &out.Source, 0); err != nil {
		return err
	}
	if in.Revision != nil {
		if err := s.Convert(&in.Revision, &out.Revision, 0); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := s.Convert(&in.Strategy, &out.Strategy, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.Output, &out.Output, 0); err != nil {
		return err
	}
	if err := convert_v1_ResourceRequirements_To_api_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
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

func convert_v1_BuildSpec_To_api_BuildSpec(in *apiv1.BuildSpec, out *buildapi.BuildSpec, s conversion.Scope) error {
	return autoconvert_v1_BuildSpec_To_api_BuildSpec(in, out, s)
}

func autoconvert_v1_BuildStatus_To_api_BuildStatus(in *apiv1.BuildStatus, out *buildapi.BuildStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildStatus))(in)
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
		if err := convert_v1_ObjectReference_To_api_ObjectReference(in.Config, out.Config, s); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	return nil
}

func convert_v1_BuildStatus_To_api_BuildStatus(in *apiv1.BuildStatus, out *buildapi.BuildStatus, s conversion.Scope) error {
	return autoconvert_v1_BuildStatus_To_api_BuildStatus(in, out, s)
}

func autoconvert_v1_BuildStrategy_To_api_BuildStrategy(in *apiv1.BuildStrategy, out *buildapi.BuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildStrategy))(in)
	}
	// in.Type has no peer in out
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

func autoconvert_v1_BuildTriggerPolicy_To_api_BuildTriggerPolicy(in *apiv1.BuildTriggerPolicy, out *buildapi.BuildTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.BuildTriggerPolicy))(in)
	}
	out.Type = buildapi.BuildTriggerType(in.Type)
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(buildapi.WebHookTrigger)
		if err := convert_v1_WebHookTrigger_To_api_WebHookTrigger(in.GitHubWebHook, out.GitHubWebHook, s); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(buildapi.WebHookTrigger)
		if err := convert_v1_WebHookTrigger_To_api_WebHookTrigger(in.GenericWebHook, out.GenericWebHook, s); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	if in.ImageChange != nil {
		out.ImageChange = new(buildapi.ImageChangeTrigger)
		if err := convert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger(in.ImageChange, out.ImageChange, s); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func autoconvert_v1_CustomBuildStrategy_To_api_CustomBuildStrategy(in *apiv1.CustomBuildStrategy, out *buildapi.CustomBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.CustomBuildStrategy))(in)
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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
			if err := convert_v1_SecretSpec_To_api_SecretSpec(&in.Secrets[i], &out.Secrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func autoconvert_v1_DockerBuildStrategy_To_api_DockerBuildStrategy(in *apiv1.DockerBuildStrategy, out *buildapi.DockerBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.DockerBuildStrategy))(in)
	}
	if in.From != nil {
		out.From = new(pkgapi.ObjectReference)
		if err := convert_v1_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoconvert_v1_GitBuildSource_To_api_GitBuildSource(in *apiv1.GitBuildSource, out *buildapi.GitBuildSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.GitBuildSource))(in)
	}
	out.URI = in.URI
	out.Ref = in.Ref
	out.HTTPProxy = in.HTTPProxy
	out.HTTPSProxy = in.HTTPSProxy
	return nil
}

func convert_v1_GitBuildSource_To_api_GitBuildSource(in *apiv1.GitBuildSource, out *buildapi.GitBuildSource, s conversion.Scope) error {
	return autoconvert_v1_GitBuildSource_To_api_GitBuildSource(in, out, s)
}

func autoconvert_v1_GitSourceRevision_To_api_GitSourceRevision(in *apiv1.GitSourceRevision, out *buildapi.GitSourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.GitSourceRevision))(in)
	}
	out.Commit = in.Commit
	if err := convert_v1_SourceControlUser_To_api_SourceControlUser(&in.Author, &out.Author, s); err != nil {
		return err
	}
	if err := convert_v1_SourceControlUser_To_api_SourceControlUser(&in.Committer, &out.Committer, s); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func convert_v1_GitSourceRevision_To_api_GitSourceRevision(in *apiv1.GitSourceRevision, out *buildapi.GitSourceRevision, s conversion.Scope) error {
	return autoconvert_v1_GitSourceRevision_To_api_GitSourceRevision(in, out, s)
}

func autoconvert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger(in *apiv1.ImageChangeTrigger, out *buildapi.ImageChangeTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ImageChangeTrigger))(in)
	}
	out.LastTriggeredImageID = in.LastTriggeredImageID
	if in.From != nil {
		out.From = new(pkgapi.ObjectReference)
		if err := convert_v1_ObjectReference_To_api_ObjectReference(in.From, out.From, s); err != nil {
			return err
		}
	} else {
		out.From = nil
	}
	return nil
}

func convert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger(in *apiv1.ImageChangeTrigger, out *buildapi.ImageChangeTrigger, s conversion.Scope) error {
	return autoconvert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger(in, out, s)
}

func autoconvert_v1_ImageSource_To_api_ImageSource(in *apiv1.ImageSource, out *buildapi.ImageSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ImageSource))(in)
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.Paths != nil {
		out.Paths = make([]buildapi.ImageSourcePath, len(in.Paths))
		for i := range in.Paths {
			if err := convert_v1_ImageSourcePath_To_api_ImageSourcePath(&in.Paths[i], &out.Paths[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Paths = nil
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	return nil
}

func convert_v1_ImageSource_To_api_ImageSource(in *apiv1.ImageSource, out *buildapi.ImageSource, s conversion.Scope) error {
	return autoconvert_v1_ImageSource_To_api_ImageSource(in, out, s)
}

func autoconvert_v1_ImageSourcePath_To_api_ImageSourcePath(in *apiv1.ImageSourcePath, out *buildapi.ImageSourcePath, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.ImageSourcePath))(in)
	}
	out.SourcePath = in.SourcePath
	out.DestinationDir = in.DestinationDir
	return nil
}

func convert_v1_ImageSourcePath_To_api_ImageSourcePath(in *apiv1.ImageSourcePath, out *buildapi.ImageSourcePath, s conversion.Scope) error {
	return autoconvert_v1_ImageSourcePath_To_api_ImageSourcePath(in, out, s)
}

func autoconvert_v1_SecretSpec_To_api_SecretSpec(in *apiv1.SecretSpec, out *buildapi.SecretSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.SecretSpec))(in)
	}
	if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(&in.SecretSource, &out.SecretSource, s); err != nil {
		return err
	}
	out.MountPath = in.MountPath
	return nil
}

func convert_v1_SecretSpec_To_api_SecretSpec(in *apiv1.SecretSpec, out *buildapi.SecretSpec, s conversion.Scope) error {
	return autoconvert_v1_SecretSpec_To_api_SecretSpec(in, out, s)
}

func autoconvert_v1_SourceBuildStrategy_To_api_SourceBuildStrategy(in *apiv1.SourceBuildStrategy, out *buildapi.SourceBuildStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.SourceBuildStrategy))(in)
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	if in.PullSecret != nil {
		out.PullSecret = new(pkgapi.LocalObjectReference)
		if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.PullSecret, out.PullSecret, s); err != nil {
			return err
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func autoconvert_v1_SourceControlUser_To_api_SourceControlUser(in *apiv1.SourceControlUser, out *buildapi.SourceControlUser, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.SourceControlUser))(in)
	}
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func convert_v1_SourceControlUser_To_api_SourceControlUser(in *apiv1.SourceControlUser, out *buildapi.SourceControlUser, s conversion.Scope) error {
	return autoconvert_v1_SourceControlUser_To_api_SourceControlUser(in, out, s)
}

func autoconvert_v1_SourceRevision_To_api_SourceRevision(in *apiv1.SourceRevision, out *buildapi.SourceRevision, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.SourceRevision))(in)
	}
	// in.Type has no peer in out
	if in.Git != nil {
		out.Git = new(buildapi.GitSourceRevision)
		if err := convert_v1_GitSourceRevision_To_api_GitSourceRevision(in.Git, out.Git, s); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func autoconvert_v1_WebHookTrigger_To_api_WebHookTrigger(in *apiv1.WebHookTrigger, out *buildapi.WebHookTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*apiv1.WebHookTrigger))(in)
	}
	out.Secret = in.Secret
	return nil
}

func convert_v1_WebHookTrigger_To_api_WebHookTrigger(in *apiv1.WebHookTrigger, out *buildapi.WebHookTrigger, s conversion.Scope) error {
	return autoconvert_v1_WebHookTrigger_To_api_WebHookTrigger(in, out, s)
}

func autoconvert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams(in *deployapi.CustomDeploymentStrategyParams, out *deployapiv1.CustomDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.CustomDeploymentStrategyParams))(in)
	}
	out.Image = in.Image
	if in.Environment != nil {
		out.Environment = make([]pkgapiv1.EnvVar, len(in.Environment))
		for i := range in.Environment {
			if err := convert_api_EnvVar_To_v1_EnvVar(&in.Environment[i], &out.Environment[i], s); err != nil {
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

func convert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams(in *deployapi.CustomDeploymentStrategyParams, out *deployapiv1.CustomDeploymentStrategyParams, s conversion.Scope) error {
	return autoconvert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams(in, out, s)
}

func autoconvert_api_DeploymentCause_To_v1_DeploymentCause(in *deployapi.DeploymentCause, out *deployapiv1.DeploymentCause, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentCause))(in)
	}
	out.Type = deployapiv1.DeploymentTriggerType(in.Type)
	if in.ImageTrigger != nil {
		out.ImageTrigger = new(deployapiv1.DeploymentCauseImageTrigger)
		if err := convert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger(in.ImageTrigger, out.ImageTrigger, s); err != nil {
			return err
		}
	} else {
		out.ImageTrigger = nil
	}
	return nil
}

func convert_api_DeploymentCause_To_v1_DeploymentCause(in *deployapi.DeploymentCause, out *deployapiv1.DeploymentCause, s conversion.Scope) error {
	return autoconvert_api_DeploymentCause_To_v1_DeploymentCause(in, out, s)
}

func autoconvert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger(in *deployapi.DeploymentCauseImageTrigger, out *deployapiv1.DeploymentCauseImageTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentCauseImageTrigger))(in)
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	return nil
}

func convert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger(in *deployapi.DeploymentCauseImageTrigger, out *deployapiv1.DeploymentCauseImageTrigger, s conversion.Scope) error {
	return autoconvert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger(in, out, s)
}

func autoconvert_api_DeploymentConfig_To_v1_DeploymentConfig(in *deployapi.DeploymentConfig, out *deployapiv1.DeploymentConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfig))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_api_DeploymentConfig_To_v1_DeploymentConfig(in *deployapi.DeploymentConfig, out *deployapiv1.DeploymentConfig, s conversion.Scope) error {
	return autoconvert_api_DeploymentConfig_To_v1_DeploymentConfig(in, out, s)
}

func autoconvert_api_DeploymentConfigList_To_v1_DeploymentConfigList(in *deployapi.DeploymentConfigList, out *deployapiv1.DeploymentConfigList, s conversion.Scope) error {
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
		out.Items = make([]deployapiv1.DeploymentConfig, len(in.Items))
		for i := range in.Items {
			if err := convert_api_DeploymentConfig_To_v1_DeploymentConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_DeploymentConfigList_To_v1_DeploymentConfigList(in *deployapi.DeploymentConfigList, out *deployapiv1.DeploymentConfigList, s conversion.Scope) error {
	return autoconvert_api_DeploymentConfigList_To_v1_DeploymentConfigList(in, out, s)
}

func autoconvert_api_DeploymentConfigRollback_To_v1_DeploymentConfigRollback(in *deployapi.DeploymentConfigRollback, out *deployapiv1.DeploymentConfigRollback, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigRollback))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func convert_api_DeploymentConfigRollback_To_v1_DeploymentConfigRollback(in *deployapi.DeploymentConfigRollback, out *deployapiv1.DeploymentConfigRollback, s conversion.Scope) error {
	return autoconvert_api_DeploymentConfigRollback_To_v1_DeploymentConfigRollback(in, out, s)
}

func autoconvert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec(in *deployapi.DeploymentConfigRollbackSpec, out *deployapiv1.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigRollbackSpec))(in)
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func convert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec(in *deployapi.DeploymentConfigRollbackSpec, out *deployapiv1.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	return autoconvert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec(in, out, s)
}

func autoconvert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec(in *deployapi.DeploymentConfigSpec, out *deployapiv1.DeploymentConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigSpec))(in)
	}
	if err := convert_api_DeploymentStrategy_To_v1_DeploymentStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if in.Triggers != nil {
		out.Triggers = make([]deployapiv1.DeploymentTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := convert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy(&in.Triggers[i], &out.Triggers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	out.Replicas = in.Replicas
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	if in.Template != nil {
		out.Template = new(pkgapiv1.PodTemplateSpec)
		if err := convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

func convert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec(in *deployapi.DeploymentConfigSpec, out *deployapiv1.DeploymentConfigSpec, s conversion.Scope) error {
	return autoconvert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec(in, out, s)
}

func autoconvert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus(in *deployapi.DeploymentConfigStatus, out *deployapiv1.DeploymentConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentConfigStatus))(in)
	}
	out.LatestVersion = in.LatestVersion
	if in.Details != nil {
		out.Details = new(deployapiv1.DeploymentDetails)
		if err := convert_api_DeploymentDetails_To_v1_DeploymentDetails(in.Details, out.Details, s); err != nil {
			return err
		}
	} else {
		out.Details = nil
	}
	return nil
}

func convert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus(in *deployapi.DeploymentConfigStatus, out *deployapiv1.DeploymentConfigStatus, s conversion.Scope) error {
	return autoconvert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus(in, out, s)
}

func autoconvert_api_DeploymentDetails_To_v1_DeploymentDetails(in *deployapi.DeploymentDetails, out *deployapiv1.DeploymentDetails, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentDetails))(in)
	}
	out.Message = in.Message
	if in.Causes != nil {
		out.Causes = make([]*deployapiv1.DeploymentCause, len(in.Causes))
		for i := range in.Causes {
			if err := s.Convert(&in.Causes[i], &out.Causes[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Causes = nil
	}
	return nil
}

func convert_api_DeploymentDetails_To_v1_DeploymentDetails(in *deployapi.DeploymentDetails, out *deployapiv1.DeploymentDetails, s conversion.Scope) error {
	return autoconvert_api_DeploymentDetails_To_v1_DeploymentDetails(in, out, s)
}

func autoconvert_api_DeploymentLog_To_v1_DeploymentLog(in *deployapi.DeploymentLog, out *deployapiv1.DeploymentLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentLog))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_DeploymentLog_To_v1_DeploymentLog(in *deployapi.DeploymentLog, out *deployapiv1.DeploymentLog, s conversion.Scope) error {
	return autoconvert_api_DeploymentLog_To_v1_DeploymentLog(in, out, s)
}

func autoconvert_api_DeploymentLogOptions_To_v1_DeploymentLogOptions(in *deployapi.DeploymentLogOptions, out *deployapiv1.DeploymentLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentLogOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
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
	if in.SinceTime != nil {
		if err := s.Convert(&in.SinceTime, &out.SinceTime, 0); err != nil {
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

func convert_api_DeploymentLogOptions_To_v1_DeploymentLogOptions(in *deployapi.DeploymentLogOptions, out *deployapiv1.DeploymentLogOptions, s conversion.Scope) error {
	return autoconvert_api_DeploymentLogOptions_To_v1_DeploymentLogOptions(in, out, s)
}

func autoconvert_api_DeploymentStrategy_To_v1_DeploymentStrategy(in *deployapi.DeploymentStrategy, out *deployapiv1.DeploymentStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentStrategy))(in)
	}
	out.Type = deployapiv1.DeploymentStrategyType(in.Type)
	if in.CustomParams != nil {
		out.CustomParams = new(deployapiv1.CustomDeploymentStrategyParams)
		if err := convert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams(in.CustomParams, out.CustomParams, s); err != nil {
			return err
		}
	} else {
		out.CustomParams = nil
	}
	if in.RecreateParams != nil {
		out.RecreateParams = new(deployapiv1.RecreateDeploymentStrategyParams)
		if err := convert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams(in.RecreateParams, out.RecreateParams, s); err != nil {
			return err
		}
	} else {
		out.RecreateParams = nil
	}
	if in.RollingParams != nil {
		if err := s.Convert(&in.RollingParams, &out.RollingParams, 0); err != nil {
			return err
		}
	} else {
		out.RollingParams = nil
	}
	if err := convert_api_ResourceRequirements_To_v1_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
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

func convert_api_DeploymentStrategy_To_v1_DeploymentStrategy(in *deployapi.DeploymentStrategy, out *deployapiv1.DeploymentStrategy, s conversion.Scope) error {
	return autoconvert_api_DeploymentStrategy_To_v1_DeploymentStrategy(in, out, s)
}

func autoconvert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams(in *deployapi.DeploymentTriggerImageChangeParams, out *deployapiv1.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
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
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.LastTriggeredImage = in.LastTriggeredImage
	return nil
}

func autoconvert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy(in *deployapi.DeploymentTriggerPolicy, out *deployapiv1.DeploymentTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.DeploymentTriggerPolicy))(in)
	}
	out.Type = deployapiv1.DeploymentTriggerType(in.Type)
	if in.ImageChangeParams != nil {
		if err := s.Convert(&in.ImageChangeParams, &out.ImageChangeParams, 0); err != nil {
			return err
		}
	} else {
		out.ImageChangeParams = nil
	}
	return nil
}

func convert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy(in *deployapi.DeploymentTriggerPolicy, out *deployapiv1.DeploymentTriggerPolicy, s conversion.Scope) error {
	return autoconvert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy(in, out, s)
}

func autoconvert_api_ExecNewPodHook_To_v1_ExecNewPodHook(in *deployapi.ExecNewPodHook, out *deployapiv1.ExecNewPodHook, s conversion.Scope) error {
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
		out.Env = make([]pkgapiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func convert_api_ExecNewPodHook_To_v1_ExecNewPodHook(in *deployapi.ExecNewPodHook, out *deployapiv1.ExecNewPodHook, s conversion.Scope) error {
	return autoconvert_api_ExecNewPodHook_To_v1_ExecNewPodHook(in, out, s)
}

func autoconvert_api_LifecycleHook_To_v1_LifecycleHook(in *deployapi.LifecycleHook, out *deployapiv1.LifecycleHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.LifecycleHook))(in)
	}
	out.FailurePolicy = deployapiv1.LifecycleHookFailurePolicy(in.FailurePolicy)
	if in.ExecNewPod != nil {
		out.ExecNewPod = new(deployapiv1.ExecNewPodHook)
		if err := convert_api_ExecNewPodHook_To_v1_ExecNewPodHook(in.ExecNewPod, out.ExecNewPod, s); err != nil {
			return err
		}
	} else {
		out.ExecNewPod = nil
	}
	return nil
}

func convert_api_LifecycleHook_To_v1_LifecycleHook(in *deployapi.LifecycleHook, out *deployapiv1.LifecycleHook, s conversion.Scope) error {
	return autoconvert_api_LifecycleHook_To_v1_LifecycleHook(in, out, s)
}

func autoconvert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams(in *deployapi.RecreateDeploymentStrategyParams, out *deployapiv1.RecreateDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapi.RecreateDeploymentStrategyParams))(in)
	}
	if in.Pre != nil {
		out.Pre = new(deployapiv1.LifecycleHook)
		if err := convert_api_LifecycleHook_To_v1_LifecycleHook(in.Pre, out.Pre, s); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapiv1.LifecycleHook)
		if err := convert_api_LifecycleHook_To_v1_LifecycleHook(in.Post, out.Post, s); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func convert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams(in *deployapi.RecreateDeploymentStrategyParams, out *deployapiv1.RecreateDeploymentStrategyParams, s conversion.Scope) error {
	return autoconvert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams(in, out, s)
}

func autoconvert_api_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams(in *deployapi.RollingDeploymentStrategyParams, out *deployapiv1.RollingDeploymentStrategyParams, s conversion.Scope) error {
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
	if in.Pre != nil {
		out.Pre = new(deployapiv1.LifecycleHook)
		if err := convert_api_LifecycleHook_To_v1_LifecycleHook(in.Pre, out.Pre, s); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapiv1.LifecycleHook)
		if err := convert_api_LifecycleHook_To_v1_LifecycleHook(in.Post, out.Post, s); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func autoconvert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams(in *deployapiv1.CustomDeploymentStrategyParams, out *deployapi.CustomDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.CustomDeploymentStrategyParams))(in)
	}
	out.Image = in.Image
	if in.Environment != nil {
		out.Environment = make([]pkgapi.EnvVar, len(in.Environment))
		for i := range in.Environment {
			if err := convert_v1_EnvVar_To_api_EnvVar(&in.Environment[i], &out.Environment[i], s); err != nil {
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

func convert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams(in *deployapiv1.CustomDeploymentStrategyParams, out *deployapi.CustomDeploymentStrategyParams, s conversion.Scope) error {
	return autoconvert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams(in, out, s)
}

func autoconvert_v1_DeploymentCause_To_api_DeploymentCause(in *deployapiv1.DeploymentCause, out *deployapi.DeploymentCause, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentCause))(in)
	}
	out.Type = deployapi.DeploymentTriggerType(in.Type)
	if in.ImageTrigger != nil {
		out.ImageTrigger = new(deployapi.DeploymentCauseImageTrigger)
		if err := convert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in.ImageTrigger, out.ImageTrigger, s); err != nil {
			return err
		}
	} else {
		out.ImageTrigger = nil
	}
	return nil
}

func convert_v1_DeploymentCause_To_api_DeploymentCause(in *deployapiv1.DeploymentCause, out *deployapi.DeploymentCause, s conversion.Scope) error {
	return autoconvert_v1_DeploymentCause_To_api_DeploymentCause(in, out, s)
}

func autoconvert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in *deployapiv1.DeploymentCauseImageTrigger, out *deployapi.DeploymentCauseImageTrigger, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentCauseImageTrigger))(in)
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in *deployapiv1.DeploymentCauseImageTrigger, out *deployapi.DeploymentCauseImageTrigger, s conversion.Scope) error {
	return autoconvert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger(in, out, s)
}

func autoconvert_v1_DeploymentConfig_To_api_DeploymentConfig(in *deployapiv1.DeploymentConfig, out *deployapi.DeploymentConfig, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfig))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_DeploymentConfig_To_api_DeploymentConfig(in *deployapiv1.DeploymentConfig, out *deployapi.DeploymentConfig, s conversion.Scope) error {
	return autoconvert_v1_DeploymentConfig_To_api_DeploymentConfig(in, out, s)
}

func autoconvert_v1_DeploymentConfigList_To_api_DeploymentConfigList(in *deployapiv1.DeploymentConfigList, out *deployapi.DeploymentConfigList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigList))(in)
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
			if err := convert_v1_DeploymentConfig_To_api_DeploymentConfig(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_DeploymentConfigList_To_api_DeploymentConfigList(in *deployapiv1.DeploymentConfigList, out *deployapi.DeploymentConfigList, s conversion.Scope) error {
	return autoconvert_v1_DeploymentConfigList_To_api_DeploymentConfigList(in, out, s)
}

func autoconvert_v1_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in *deployapiv1.DeploymentConfigRollback, out *deployapi.DeploymentConfigRollback, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigRollback))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in *deployapiv1.DeploymentConfigRollback, out *deployapi.DeploymentConfigRollback, s conversion.Scope) error {
	return autoconvert_v1_DeploymentConfigRollback_To_api_DeploymentConfigRollback(in, out, s)
}

func autoconvert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in *deployapiv1.DeploymentConfigRollbackSpec, out *deployapi.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigRollbackSpec))(in)
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func convert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in *deployapiv1.DeploymentConfigRollbackSpec, out *deployapi.DeploymentConfigRollbackSpec, s conversion.Scope) error {
	return autoconvert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec(in, out, s)
}

func autoconvert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec(in *deployapiv1.DeploymentConfigSpec, out *deployapi.DeploymentConfigSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigSpec))(in)
	}
	if err := convert_v1_DeploymentStrategy_To_api_DeploymentStrategy(&in.Strategy, &out.Strategy, s); err != nil {
		return err
	}
	if in.Triggers != nil {
		out.Triggers = make([]deployapi.DeploymentTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := convert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(&in.Triggers[i], &out.Triggers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	out.Replicas = in.Replicas
	if in.Selector != nil {
		out.Selector = make(map[string]string)
		for key, val := range in.Selector {
			out.Selector[key] = val
		}
	} else {
		out.Selector = nil
	}
	if in.Template != nil {
		out.Template = new(pkgapi.PodTemplateSpec)
		if err := convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in.Template, out.Template, s); err != nil {
			return err
		}
	} else {
		out.Template = nil
	}
	return nil
}

func convert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec(in *deployapiv1.DeploymentConfigSpec, out *deployapi.DeploymentConfigSpec, s conversion.Scope) error {
	return autoconvert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec(in, out, s)
}

func autoconvert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus(in *deployapiv1.DeploymentConfigStatus, out *deployapi.DeploymentConfigStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentConfigStatus))(in)
	}
	out.LatestVersion = in.LatestVersion
	if in.Details != nil {
		out.Details = new(deployapi.DeploymentDetails)
		if err := convert_v1_DeploymentDetails_To_api_DeploymentDetails(in.Details, out.Details, s); err != nil {
			return err
		}
	} else {
		out.Details = nil
	}
	return nil
}

func convert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus(in *deployapiv1.DeploymentConfigStatus, out *deployapi.DeploymentConfigStatus, s conversion.Scope) error {
	return autoconvert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus(in, out, s)
}

func autoconvert_v1_DeploymentDetails_To_api_DeploymentDetails(in *deployapiv1.DeploymentDetails, out *deployapi.DeploymentDetails, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentDetails))(in)
	}
	out.Message = in.Message
	if in.Causes != nil {
		out.Causes = make([]*deployapi.DeploymentCause, len(in.Causes))
		for i := range in.Causes {
			if err := s.Convert(&in.Causes[i], &out.Causes[i], 0); err != nil {
				return err
			}
		}
	} else {
		out.Causes = nil
	}
	return nil
}

func convert_v1_DeploymentDetails_To_api_DeploymentDetails(in *deployapiv1.DeploymentDetails, out *deployapi.DeploymentDetails, s conversion.Scope) error {
	return autoconvert_v1_DeploymentDetails_To_api_DeploymentDetails(in, out, s)
}

func autoconvert_v1_DeploymentLog_To_api_DeploymentLog(in *deployapiv1.DeploymentLog, out *deployapi.DeploymentLog, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentLog))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_DeploymentLog_To_api_DeploymentLog(in *deployapiv1.DeploymentLog, out *deployapi.DeploymentLog, s conversion.Scope) error {
	return autoconvert_v1_DeploymentLog_To_api_DeploymentLog(in, out, s)
}

func autoconvert_v1_DeploymentLogOptions_To_api_DeploymentLogOptions(in *deployapiv1.DeploymentLogOptions, out *deployapi.DeploymentLogOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentLogOptions))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
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
	if in.SinceTime != nil {
		if err := s.Convert(&in.SinceTime, &out.SinceTime, 0); err != nil {
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

func convert_v1_DeploymentLogOptions_To_api_DeploymentLogOptions(in *deployapiv1.DeploymentLogOptions, out *deployapi.DeploymentLogOptions, s conversion.Scope) error {
	return autoconvert_v1_DeploymentLogOptions_To_api_DeploymentLogOptions(in, out, s)
}

func autoconvert_v1_DeploymentStrategy_To_api_DeploymentStrategy(in *deployapiv1.DeploymentStrategy, out *deployapi.DeploymentStrategy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentStrategy))(in)
	}
	out.Type = deployapi.DeploymentStrategyType(in.Type)
	if in.CustomParams != nil {
		out.CustomParams = new(deployapi.CustomDeploymentStrategyParams)
		if err := convert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams(in.CustomParams, out.CustomParams, s); err != nil {
			return err
		}
	} else {
		out.CustomParams = nil
	}
	if in.RecreateParams != nil {
		out.RecreateParams = new(deployapi.RecreateDeploymentStrategyParams)
		if err := convert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams(in.RecreateParams, out.RecreateParams, s); err != nil {
			return err
		}
	} else {
		out.RecreateParams = nil
	}
	if in.RollingParams != nil {
		if err := s.Convert(&in.RollingParams, &out.RollingParams, 0); err != nil {
			return err
		}
	} else {
		out.RollingParams = nil
	}
	if err := convert_v1_ResourceRequirements_To_api_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
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

func convert_v1_DeploymentStrategy_To_api_DeploymentStrategy(in *deployapiv1.DeploymentStrategy, out *deployapi.DeploymentStrategy, s conversion.Scope) error {
	return autoconvert_v1_DeploymentStrategy_To_api_DeploymentStrategy(in, out, s)
}

func autoconvert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams(in *deployapiv1.DeploymentTriggerImageChangeParams, out *deployapi.DeploymentTriggerImageChangeParams, s conversion.Scope) error {
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
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.From, &out.From, s); err != nil {
		return err
	}
	out.LastTriggeredImage = in.LastTriggeredImage
	return nil
}

func autoconvert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(in *deployapiv1.DeploymentTriggerPolicy, out *deployapi.DeploymentTriggerPolicy, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.DeploymentTriggerPolicy))(in)
	}
	out.Type = deployapi.DeploymentTriggerType(in.Type)
	if in.ImageChangeParams != nil {
		if err := s.Convert(&in.ImageChangeParams, &out.ImageChangeParams, 0); err != nil {
			return err
		}
	} else {
		out.ImageChangeParams = nil
	}
	return nil
}

func convert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(in *deployapiv1.DeploymentTriggerPolicy, out *deployapi.DeploymentTriggerPolicy, s conversion.Scope) error {
	return autoconvert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy(in, out, s)
}

func autoconvert_v1_ExecNewPodHook_To_api_ExecNewPodHook(in *deployapiv1.ExecNewPodHook, out *deployapi.ExecNewPodHook, s conversion.Scope) error {
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
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
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

func convert_v1_ExecNewPodHook_To_api_ExecNewPodHook(in *deployapiv1.ExecNewPodHook, out *deployapi.ExecNewPodHook, s conversion.Scope) error {
	return autoconvert_v1_ExecNewPodHook_To_api_ExecNewPodHook(in, out, s)
}

func autoconvert_v1_LifecycleHook_To_api_LifecycleHook(in *deployapiv1.LifecycleHook, out *deployapi.LifecycleHook, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.LifecycleHook))(in)
	}
	out.FailurePolicy = deployapi.LifecycleHookFailurePolicy(in.FailurePolicy)
	if in.ExecNewPod != nil {
		out.ExecNewPod = new(deployapi.ExecNewPodHook)
		if err := convert_v1_ExecNewPodHook_To_api_ExecNewPodHook(in.ExecNewPod, out.ExecNewPod, s); err != nil {
			return err
		}
	} else {
		out.ExecNewPod = nil
	}
	return nil
}

func convert_v1_LifecycleHook_To_api_LifecycleHook(in *deployapiv1.LifecycleHook, out *deployapi.LifecycleHook, s conversion.Scope) error {
	return autoconvert_v1_LifecycleHook_To_api_LifecycleHook(in, out, s)
}

func autoconvert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams(in *deployapiv1.RecreateDeploymentStrategyParams, out *deployapi.RecreateDeploymentStrategyParams, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*deployapiv1.RecreateDeploymentStrategyParams))(in)
	}
	if in.Pre != nil {
		out.Pre = new(deployapi.LifecycleHook)
		if err := convert_v1_LifecycleHook_To_api_LifecycleHook(in.Pre, out.Pre, s); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapi.LifecycleHook)
		if err := convert_v1_LifecycleHook_To_api_LifecycleHook(in.Post, out.Post, s); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func convert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams(in *deployapiv1.RecreateDeploymentStrategyParams, out *deployapi.RecreateDeploymentStrategyParams, s conversion.Scope) error {
	return autoconvert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams(in, out, s)
}

func autoconvert_v1_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams(in *deployapiv1.RollingDeploymentStrategyParams, out *deployapi.RollingDeploymentStrategyParams, s conversion.Scope) error {
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
	if in.Pre != nil {
		out.Pre = new(deployapi.LifecycleHook)
		if err := convert_v1_LifecycleHook_To_api_LifecycleHook(in.Pre, out.Pre, s); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapi.LifecycleHook)
		if err := convert_v1_LifecycleHook_To_api_LifecycleHook(in.Post, out.Post, s); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func autoconvert_api_Image_To_v1_Image(in *imageapi.Image, out *imageapiv1.Image, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.Image))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func autoconvert_api_ImageList_To_v1_ImageList(in *imageapi.ImageList, out *imageapiv1.ImageList, s conversion.Scope) error {
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
		out.Items = make([]imageapiv1.Image, len(in.Items))
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

func convert_api_ImageList_To_v1_ImageList(in *imageapi.ImageList, out *imageapiv1.ImageList, s conversion.Scope) error {
	return autoconvert_api_ImageList_To_v1_ImageList(in, out, s)
}

func autoconvert_api_ImageStream_To_v1_ImageStream(in *imageapi.ImageStream, out *imageapiv1.ImageStream, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStream))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_ImageStream_To_v1_ImageStream(in *imageapi.ImageStream, out *imageapiv1.ImageStream, s conversion.Scope) error {
	return autoconvert_api_ImageStream_To_v1_ImageStream(in, out, s)
}

func autoconvert_api_ImageStreamImage_To_v1_ImageStreamImage(in *imageapi.ImageStreamImage, out *imageapiv1.ImageStreamImage, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamImage))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_ImageStreamImage_To_v1_ImageStreamImage(in *imageapi.ImageStreamImage, out *imageapiv1.ImageStreamImage, s conversion.Scope) error {
	return autoconvert_api_ImageStreamImage_To_v1_ImageStreamImage(in, out, s)
}

func autoconvert_api_ImageStreamList_To_v1_ImageStreamList(in *imageapi.ImageStreamList, out *imageapiv1.ImageStreamList, s conversion.Scope) error {
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
		out.Items = make([]imageapiv1.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := convert_api_ImageStream_To_v1_ImageStream(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ImageStreamList_To_v1_ImageStreamList(in *imageapi.ImageStreamList, out *imageapiv1.ImageStreamList, s conversion.Scope) error {
	return autoconvert_api_ImageStreamList_To_v1_ImageStreamList(in, out, s)
}

func autoconvert_api_ImageStreamMapping_To_v1_ImageStreamMapping(in *imageapi.ImageStreamMapping, out *imageapiv1.ImageStreamMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamMapping))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	// in.DockerImageRepository has no peer in out
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func autoconvert_api_ImageStreamSpec_To_v1_ImageStreamSpec(in *imageapi.ImageStreamSpec, out *imageapiv1.ImageStreamSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamSpec))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ImageStreamStatus_To_v1_ImageStreamStatus(in *imageapi.ImageStreamStatus, out *imageapiv1.ImageStreamStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamStatus))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_api_ImageStreamTag_To_v1_ImageStreamTag(in *imageapi.ImageStreamTag, out *imageapiv1.ImageStreamTag, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamTag))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_ImageStreamTag_To_v1_ImageStreamTag(in *imageapi.ImageStreamTag, out *imageapiv1.ImageStreamTag, s conversion.Scope) error {
	return autoconvert_api_ImageStreamTag_To_v1_ImageStreamTag(in, out, s)
}

func autoconvert_api_ImageStreamTagList_To_v1_ImageStreamTagList(in *imageapi.ImageStreamTagList, out *imageapiv1.ImageStreamTagList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapi.ImageStreamTagList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1.ImageStreamTag, len(in.Items))
		for i := range in.Items {
			if err := convert_api_ImageStreamTag_To_v1_ImageStreamTag(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ImageStreamTagList_To_v1_ImageStreamTagList(in *imageapi.ImageStreamTagList, out *imageapiv1.ImageStreamTagList, s conversion.Scope) error {
	return autoconvert_api_ImageStreamTagList_To_v1_ImageStreamTagList(in, out, s)
}

func autoconvert_v1_Image_To_api_Image(in *imageapiv1.Image, out *imageapi.Image, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.Image))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func autoconvert_v1_ImageList_To_api_ImageList(in *imageapiv1.ImageList, out *imageapi.ImageList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageList))(in)
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

func convert_v1_ImageList_To_api_ImageList(in *imageapiv1.ImageList, out *imageapi.ImageList, s conversion.Scope) error {
	return autoconvert_v1_ImageList_To_api_ImageList(in, out, s)
}

func autoconvert_v1_ImageStream_To_api_ImageStream(in *imageapiv1.ImageStream, out *imageapi.ImageStream, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStream))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_ImageStream_To_api_ImageStream(in *imageapiv1.ImageStream, out *imageapi.ImageStream, s conversion.Scope) error {
	return autoconvert_v1_ImageStream_To_api_ImageStream(in, out, s)
}

func autoconvert_v1_ImageStreamImage_To_api_ImageStreamImage(in *imageapiv1.ImageStreamImage, out *imageapi.ImageStreamImage, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamImage))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_ImageStreamImage_To_api_ImageStreamImage(in *imageapiv1.ImageStreamImage, out *imageapi.ImageStreamImage, s conversion.Scope) error {
	return autoconvert_v1_ImageStreamImage_To_api_ImageStreamImage(in, out, s)
}

func autoconvert_v1_ImageStreamList_To_api_ImageStreamList(in *imageapiv1.ImageStreamList, out *imageapi.ImageStreamList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamList))(in)
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
			if err := convert_v1_ImageStream_To_api_ImageStream(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_ImageStreamList_To_api_ImageStreamList(in *imageapiv1.ImageStreamList, out *imageapi.ImageStreamList, s conversion.Scope) error {
	return autoconvert_v1_ImageStreamList_To_api_ImageStreamList(in, out, s)
}

func autoconvert_v1_ImageStreamMapping_To_api_ImageStreamMapping(in *imageapiv1.ImageStreamMapping, out *imageapi.ImageStreamMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamMapping))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func autoconvert_v1_ImageStreamSpec_To_api_ImageStreamSpec(in *imageapiv1.ImageStreamSpec, out *imageapi.ImageStreamSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamSpec))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1_ImageStreamStatus_To_api_ImageStreamStatus(in *imageapiv1.ImageStreamStatus, out *imageapi.ImageStreamStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamStatus))(in)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := s.Convert(&in.Tags, &out.Tags, 0); err != nil {
		return err
	}
	return nil
}

func autoconvert_v1_ImageStreamTag_To_api_ImageStreamTag(in *imageapiv1.ImageStreamTag, out *imageapi.ImageStreamTag, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamTag))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Image, &out.Image, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_ImageStreamTag_To_api_ImageStreamTag(in *imageapiv1.ImageStreamTag, out *imageapi.ImageStreamTag, s conversion.Scope) error {
	return autoconvert_v1_ImageStreamTag_To_api_ImageStreamTag(in, out, s)
}

func autoconvert_v1_ImageStreamTagList_To_api_ImageStreamTagList(in *imageapiv1.ImageStreamTagList, out *imageapi.ImageStreamTagList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*imageapiv1.ImageStreamTagList))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := s.Convert(&in.ListMeta, &out.ListMeta, 0); err != nil {
		return err
	}
	if in.Items != nil {
		out.Items = make([]imageapi.ImageStreamTag, len(in.Items))
		for i := range in.Items {
			if err := convert_v1_ImageStreamTag_To_api_ImageStreamTag(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_ImageStreamTagList_To_api_ImageStreamTagList(in *imageapiv1.ImageStreamTagList, out *imageapi.ImageStreamTagList, s conversion.Scope) error {
	return autoconvert_v1_ImageStreamTagList_To_api_ImageStreamTagList(in, out, s)
}

func autoconvert_api_OAuthAccessToken_To_v1_OAuthAccessToken(in *oauthapi.OAuthAccessToken, out *oauthapiv1.OAuthAccessToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAccessToken))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_OAuthAccessToken_To_v1_OAuthAccessToken(in *oauthapi.OAuthAccessToken, out *oauthapiv1.OAuthAccessToken, s conversion.Scope) error {
	return autoconvert_api_OAuthAccessToken_To_v1_OAuthAccessToken(in, out, s)
}

func autoconvert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList(in *oauthapi.OAuthAccessTokenList, out *oauthapiv1.OAuthAccessTokenList, s conversion.Scope) error {
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
		out.Items = make([]oauthapiv1.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := convert_api_OAuthAccessToken_To_v1_OAuthAccessToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList(in *oauthapi.OAuthAccessTokenList, out *oauthapiv1.OAuthAccessTokenList, s conversion.Scope) error {
	return autoconvert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList(in, out, s)
}

func autoconvert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(in *oauthapi.OAuthAuthorizeToken, out *oauthapiv1.OAuthAuthorizeToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthAuthorizeToken))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(in *oauthapi.OAuthAuthorizeToken, out *oauthapiv1.OAuthAuthorizeToken, s conversion.Scope) error {
	return autoconvert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(in, out, s)
}

func autoconvert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList(in *oauthapi.OAuthAuthorizeTokenList, out *oauthapiv1.OAuthAuthorizeTokenList, s conversion.Scope) error {
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
		out.Items = make([]oauthapiv1.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := convert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList(in *oauthapi.OAuthAuthorizeTokenList, out *oauthapiv1.OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoconvert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList(in, out, s)
}

func autoconvert_api_OAuthClient_To_v1_OAuthClient(in *oauthapi.OAuthClient, out *oauthapiv1.OAuthClient, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClient))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_OAuthClient_To_v1_OAuthClient(in *oauthapi.OAuthClient, out *oauthapiv1.OAuthClient, s conversion.Scope) error {
	return autoconvert_api_OAuthClient_To_v1_OAuthClient(in, out, s)
}

func autoconvert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(in *oauthapi.OAuthClientAuthorization, out *oauthapiv1.OAuthClientAuthorization, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapi.OAuthClientAuthorization))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(in *oauthapi.OAuthClientAuthorization, out *oauthapiv1.OAuthClientAuthorization, s conversion.Scope) error {
	return autoconvert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(in, out, s)
}

func autoconvert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList(in *oauthapi.OAuthClientAuthorizationList, out *oauthapiv1.OAuthClientAuthorizationList, s conversion.Scope) error {
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
		out.Items = make([]oauthapiv1.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := convert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList(in *oauthapi.OAuthClientAuthorizationList, out *oauthapiv1.OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoconvert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList(in, out, s)
}

func autoconvert_api_OAuthClientList_To_v1_OAuthClientList(in *oauthapi.OAuthClientList, out *oauthapiv1.OAuthClientList, s conversion.Scope) error {
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
		out.Items = make([]oauthapiv1.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := convert_api_OAuthClient_To_v1_OAuthClient(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_OAuthClientList_To_v1_OAuthClientList(in *oauthapi.OAuthClientList, out *oauthapiv1.OAuthClientList, s conversion.Scope) error {
	return autoconvert_api_OAuthClientList_To_v1_OAuthClientList(in, out, s)
}

func autoconvert_v1_OAuthAccessToken_To_api_OAuthAccessToken(in *oauthapiv1.OAuthAccessToken, out *oauthapi.OAuthAccessToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthAccessToken))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_OAuthAccessToken_To_api_OAuthAccessToken(in *oauthapiv1.OAuthAccessToken, out *oauthapi.OAuthAccessToken, s conversion.Scope) error {
	return autoconvert_v1_OAuthAccessToken_To_api_OAuthAccessToken(in, out, s)
}

func autoconvert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *oauthapiv1.OAuthAccessTokenList, out *oauthapi.OAuthAccessTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthAccessTokenList))(in)
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
			if err := convert_v1_OAuthAccessToken_To_api_OAuthAccessToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in *oauthapiv1.OAuthAccessTokenList, out *oauthapi.OAuthAccessTokenList, s conversion.Scope) error {
	return autoconvert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList(in, out, s)
}

func autoconvert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *oauthapiv1.OAuthAuthorizeToken, out *oauthapi.OAuthAuthorizeToken, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthAuthorizeToken))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in *oauthapiv1.OAuthAuthorizeToken, out *oauthapi.OAuthAuthorizeToken, s conversion.Scope) error {
	return autoconvert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(in, out, s)
}

func autoconvert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *oauthapiv1.OAuthAuthorizeTokenList, out *oauthapi.OAuthAuthorizeTokenList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthAuthorizeTokenList))(in)
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
			if err := convert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in *oauthapiv1.OAuthAuthorizeTokenList, out *oauthapi.OAuthAuthorizeTokenList, s conversion.Scope) error {
	return autoconvert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList(in, out, s)
}

func autoconvert_v1_OAuthClient_To_api_OAuthClient(in *oauthapiv1.OAuthClient, out *oauthapi.OAuthClient, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthClient))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_OAuthClient_To_api_OAuthClient(in *oauthapiv1.OAuthClient, out *oauthapi.OAuthClient, s conversion.Scope) error {
	return autoconvert_v1_OAuthClient_To_api_OAuthClient(in, out, s)
}

func autoconvert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *oauthapiv1.OAuthClientAuthorization, out *oauthapi.OAuthClientAuthorization, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthClientAuthorization))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in *oauthapiv1.OAuthClientAuthorization, out *oauthapi.OAuthClientAuthorization, s conversion.Scope) error {
	return autoconvert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(in, out, s)
}

func autoconvert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *oauthapiv1.OAuthClientAuthorizationList, out *oauthapi.OAuthClientAuthorizationList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthClientAuthorizationList))(in)
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
			if err := convert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in *oauthapiv1.OAuthClientAuthorizationList, out *oauthapi.OAuthClientAuthorizationList, s conversion.Scope) error {
	return autoconvert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList(in, out, s)
}

func autoconvert_v1_OAuthClientList_To_api_OAuthClientList(in *oauthapiv1.OAuthClientList, out *oauthapi.OAuthClientList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*oauthapiv1.OAuthClientList))(in)
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
			if err := convert_v1_OAuthClient_To_api_OAuthClient(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_OAuthClientList_To_api_OAuthClientList(in *oauthapiv1.OAuthClientList, out *oauthapi.OAuthClientList, s conversion.Scope) error {
	return autoconvert_v1_OAuthClientList_To_api_OAuthClientList(in, out, s)
}

func autoconvert_api_Project_To_v1_Project(in *projectapi.Project, out *projectapiv1.Project, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.Project))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_ProjectSpec_To_v1_ProjectSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_api_ProjectStatus_To_v1_ProjectStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_api_Project_To_v1_Project(in *projectapi.Project, out *projectapiv1.Project, s conversion.Scope) error {
	return autoconvert_api_Project_To_v1_Project(in, out, s)
}

func autoconvert_api_ProjectList_To_v1_ProjectList(in *projectapi.ProjectList, out *projectapiv1.ProjectList, s conversion.Scope) error {
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
		out.Items = make([]projectapiv1.Project, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Project_To_v1_Project(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ProjectList_To_v1_ProjectList(in *projectapi.ProjectList, out *projectapiv1.ProjectList, s conversion.Scope) error {
	return autoconvert_api_ProjectList_To_v1_ProjectList(in, out, s)
}

func autoconvert_api_ProjectRequest_To_v1_ProjectRequest(in *projectapi.ProjectRequest, out *projectapiv1.ProjectRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectRequest))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func convert_api_ProjectRequest_To_v1_ProjectRequest(in *projectapi.ProjectRequest, out *projectapiv1.ProjectRequest, s conversion.Scope) error {
	return autoconvert_api_ProjectRequest_To_v1_ProjectRequest(in, out, s)
}

func autoconvert_api_ProjectSpec_To_v1_ProjectSpec(in *projectapi.ProjectSpec, out *projectapiv1.ProjectSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectSpec))(in)
	}
	if in.Finalizers != nil {
		out.Finalizers = make([]pkgapiv1.FinalizerName, len(in.Finalizers))
		for i := range in.Finalizers {
			out.Finalizers[i] = pkgapiv1.FinalizerName(in.Finalizers[i])
		}
	} else {
		out.Finalizers = nil
	}
	return nil
}

func convert_api_ProjectSpec_To_v1_ProjectSpec(in *projectapi.ProjectSpec, out *projectapiv1.ProjectSpec, s conversion.Scope) error {
	return autoconvert_api_ProjectSpec_To_v1_ProjectSpec(in, out, s)
}

func autoconvert_api_ProjectStatus_To_v1_ProjectStatus(in *projectapi.ProjectStatus, out *projectapiv1.ProjectStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapi.ProjectStatus))(in)
	}
	out.Phase = pkgapiv1.NamespacePhase(in.Phase)
	return nil
}

func convert_api_ProjectStatus_To_v1_ProjectStatus(in *projectapi.ProjectStatus, out *projectapiv1.ProjectStatus, s conversion.Scope) error {
	return autoconvert_api_ProjectStatus_To_v1_ProjectStatus(in, out, s)
}

func autoconvert_v1_Project_To_api_Project(in *projectapiv1.Project, out *projectapi.Project, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.Project))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1_ProjectSpec_To_api_ProjectSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1_ProjectStatus_To_api_ProjectStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_Project_To_api_Project(in *projectapiv1.Project, out *projectapi.Project, s conversion.Scope) error {
	return autoconvert_v1_Project_To_api_Project(in, out, s)
}

func autoconvert_v1_ProjectList_To_api_ProjectList(in *projectapiv1.ProjectList, out *projectapi.ProjectList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.ProjectList))(in)
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
			if err := convert_v1_Project_To_api_Project(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_ProjectList_To_api_ProjectList(in *projectapiv1.ProjectList, out *projectapi.ProjectList, s conversion.Scope) error {
	return autoconvert_v1_ProjectList_To_api_ProjectList(in, out, s)
}

func autoconvert_v1_ProjectRequest_To_api_ProjectRequest(in *projectapiv1.ProjectRequest, out *projectapi.ProjectRequest, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.ProjectRequest))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func convert_v1_ProjectRequest_To_api_ProjectRequest(in *projectapiv1.ProjectRequest, out *projectapi.ProjectRequest, s conversion.Scope) error {
	return autoconvert_v1_ProjectRequest_To_api_ProjectRequest(in, out, s)
}

func autoconvert_v1_ProjectSpec_To_api_ProjectSpec(in *projectapiv1.ProjectSpec, out *projectapi.ProjectSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.ProjectSpec))(in)
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

func convert_v1_ProjectSpec_To_api_ProjectSpec(in *projectapiv1.ProjectSpec, out *projectapi.ProjectSpec, s conversion.Scope) error {
	return autoconvert_v1_ProjectSpec_To_api_ProjectSpec(in, out, s)
}

func autoconvert_v1_ProjectStatus_To_api_ProjectStatus(in *projectapiv1.ProjectStatus, out *projectapi.ProjectStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*projectapiv1.ProjectStatus))(in)
	}
	out.Phase = pkgapi.NamespacePhase(in.Phase)
	return nil
}

func convert_v1_ProjectStatus_To_api_ProjectStatus(in *projectapiv1.ProjectStatus, out *projectapi.ProjectStatus, s conversion.Scope) error {
	return autoconvert_v1_ProjectStatus_To_api_ProjectStatus(in, out, s)
}

func autoconvert_api_Route_To_v1_Route(in *routeapi.Route, out *routeapiv1.Route, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.Route))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_RouteSpec_To_v1_RouteSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_api_RouteStatus_To_v1_RouteStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_api_Route_To_v1_Route(in *routeapi.Route, out *routeapiv1.Route, s conversion.Scope) error {
	return autoconvert_api_Route_To_v1_Route(in, out, s)
}

func autoconvert_api_RouteList_To_v1_RouteList(in *routeapi.RouteList, out *routeapiv1.RouteList, s conversion.Scope) error {
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
		out.Items = make([]routeapiv1.Route, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Route_To_v1_Route(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_RouteList_To_v1_RouteList(in *routeapi.RouteList, out *routeapiv1.RouteList, s conversion.Scope) error {
	return autoconvert_api_RouteList_To_v1_RouteList(in, out, s)
}

func autoconvert_api_RoutePort_To_v1_RoutePort(in *routeapi.RoutePort, out *routeapiv1.RoutePort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RoutePort))(in)
	}
	if err := s.Convert(&in.TargetPort, &out.TargetPort, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_RoutePort_To_v1_RoutePort(in *routeapi.RoutePort, out *routeapiv1.RoutePort, s conversion.Scope) error {
	return autoconvert_api_RoutePort_To_v1_RoutePort(in, out, s)
}

func autoconvert_api_RouteSpec_To_v1_RouteSpec(in *routeapi.RouteSpec, out *routeapiv1.RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	if in.Port != nil {
		out.Port = new(routeapiv1.RoutePort)
		if err := convert_api_RoutePort_To_v1_RoutePort(in.Port, out.Port, s); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	if in.TLS != nil {
		out.TLS = new(routeapiv1.TLSConfig)
		if err := convert_api_TLSConfig_To_v1_TLSConfig(in.TLS, out.TLS, s); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func convert_api_RouteSpec_To_v1_RouteSpec(in *routeapi.RouteSpec, out *routeapiv1.RouteSpec, s conversion.Scope) error {
	return autoconvert_api_RouteSpec_To_v1_RouteSpec(in, out, s)
}

func autoconvert_api_RouteStatus_To_v1_RouteStatus(in *routeapi.RouteStatus, out *routeapiv1.RouteStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapi.RouteStatus))(in)
	}
	return nil
}

func convert_api_RouteStatus_To_v1_RouteStatus(in *routeapi.RouteStatus, out *routeapiv1.RouteStatus, s conversion.Scope) error {
	return autoconvert_api_RouteStatus_To_v1_RouteStatus(in, out, s)
}

func autoconvert_api_TLSConfig_To_v1_TLSConfig(in *routeapi.TLSConfig, out *routeapiv1.TLSConfig, s conversion.Scope) error {
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

func convert_api_TLSConfig_To_v1_TLSConfig(in *routeapi.TLSConfig, out *routeapiv1.TLSConfig, s conversion.Scope) error {
	return autoconvert_api_TLSConfig_To_v1_TLSConfig(in, out, s)
}

func autoconvert_v1_Route_To_api_Route(in *routeapiv1.Route, out *routeapi.Route, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.Route))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1_RouteSpec_To_api_RouteSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := convert_v1_RouteStatus_To_api_RouteStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_Route_To_api_Route(in *routeapiv1.Route, out *routeapi.Route, s conversion.Scope) error {
	return autoconvert_v1_Route_To_api_Route(in, out, s)
}

func autoconvert_v1_RouteList_To_api_RouteList(in *routeapiv1.RouteList, out *routeapi.RouteList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RouteList))(in)
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
			if err := convert_v1_Route_To_api_Route(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_RouteList_To_api_RouteList(in *routeapiv1.RouteList, out *routeapi.RouteList, s conversion.Scope) error {
	return autoconvert_v1_RouteList_To_api_RouteList(in, out, s)
}

func autoconvert_v1_RoutePort_To_api_RoutePort(in *routeapiv1.RoutePort, out *routeapi.RoutePort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RoutePort))(in)
	}
	if err := s.Convert(&in.TargetPort, &out.TargetPort, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_RoutePort_To_api_RoutePort(in *routeapiv1.RoutePort, out *routeapi.RoutePort, s conversion.Scope) error {
	return autoconvert_v1_RoutePort_To_api_RoutePort(in, out, s)
}

func autoconvert_v1_RouteSpec_To_api_RouteSpec(in *routeapiv1.RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RouteSpec))(in)
	}
	out.Host = in.Host
	out.Path = in.Path
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.To, &out.To, s); err != nil {
		return err
	}
	if in.Port != nil {
		out.Port = new(routeapi.RoutePort)
		if err := convert_v1_RoutePort_To_api_RoutePort(in.Port, out.Port, s); err != nil {
			return err
		}
	} else {
		out.Port = nil
	}
	if in.TLS != nil {
		out.TLS = new(routeapi.TLSConfig)
		if err := convert_v1_TLSConfig_To_api_TLSConfig(in.TLS, out.TLS, s); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func convert_v1_RouteSpec_To_api_RouteSpec(in *routeapiv1.RouteSpec, out *routeapi.RouteSpec, s conversion.Scope) error {
	return autoconvert_v1_RouteSpec_To_api_RouteSpec(in, out, s)
}

func autoconvert_v1_RouteStatus_To_api_RouteStatus(in *routeapiv1.RouteStatus, out *routeapi.RouteStatus, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*routeapiv1.RouteStatus))(in)
	}
	return nil
}

func convert_v1_RouteStatus_To_api_RouteStatus(in *routeapiv1.RouteStatus, out *routeapi.RouteStatus, s conversion.Scope) error {
	return autoconvert_v1_RouteStatus_To_api_RouteStatus(in, out, s)
}

func autoconvert_v1_TLSConfig_To_api_TLSConfig(in *routeapiv1.TLSConfig, out *routeapi.TLSConfig, s conversion.Scope) error {
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

func convert_v1_TLSConfig_To_api_TLSConfig(in *routeapiv1.TLSConfig, out *routeapi.TLSConfig, s conversion.Scope) error {
	return autoconvert_v1_TLSConfig_To_api_TLSConfig(in, out, s)
}

func autoconvert_api_ClusterNetwork_To_v1_ClusterNetwork(in *sdnapi.ClusterNetwork, out *sdnapiv1.ClusterNetwork, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.ClusterNetwork))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func convert_api_ClusterNetwork_To_v1_ClusterNetwork(in *sdnapi.ClusterNetwork, out *sdnapiv1.ClusterNetwork, s conversion.Scope) error {
	return autoconvert_api_ClusterNetwork_To_v1_ClusterNetwork(in, out, s)
}

func autoconvert_api_ClusterNetworkList_To_v1_ClusterNetworkList(in *sdnapi.ClusterNetworkList, out *sdnapiv1.ClusterNetworkList, s conversion.Scope) error {
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
		out.Items = make([]sdnapiv1.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := convert_api_ClusterNetwork_To_v1_ClusterNetwork(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_ClusterNetworkList_To_v1_ClusterNetworkList(in *sdnapi.ClusterNetworkList, out *sdnapiv1.ClusterNetworkList, s conversion.Scope) error {
	return autoconvert_api_ClusterNetworkList_To_v1_ClusterNetworkList(in, out, s)
}

func autoconvert_api_HostSubnet_To_v1_HostSubnet(in *sdnapi.HostSubnet, out *sdnapiv1.HostSubnet, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.HostSubnet))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func convert_api_HostSubnet_To_v1_HostSubnet(in *sdnapi.HostSubnet, out *sdnapiv1.HostSubnet, s conversion.Scope) error {
	return autoconvert_api_HostSubnet_To_v1_HostSubnet(in, out, s)
}

func autoconvert_api_HostSubnetList_To_v1_HostSubnetList(in *sdnapi.HostSubnetList, out *sdnapiv1.HostSubnetList, s conversion.Scope) error {
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
		out.Items = make([]sdnapiv1.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := convert_api_HostSubnet_To_v1_HostSubnet(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_HostSubnetList_To_v1_HostSubnetList(in *sdnapi.HostSubnetList, out *sdnapiv1.HostSubnetList, s conversion.Scope) error {
	return autoconvert_api_HostSubnetList_To_v1_HostSubnetList(in, out, s)
}

func autoconvert_api_NetNamespace_To_v1_NetNamespace(in *sdnapi.NetNamespace, out *sdnapiv1.NetNamespace, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapi.NetNamespace))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func convert_api_NetNamespace_To_v1_NetNamespace(in *sdnapi.NetNamespace, out *sdnapiv1.NetNamespace, s conversion.Scope) error {
	return autoconvert_api_NetNamespace_To_v1_NetNamespace(in, out, s)
}

func autoconvert_api_NetNamespaceList_To_v1_NetNamespaceList(in *sdnapi.NetNamespaceList, out *sdnapiv1.NetNamespaceList, s conversion.Scope) error {
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
		out.Items = make([]sdnapiv1.NetNamespace, len(in.Items))
		for i := range in.Items {
			if err := convert_api_NetNamespace_To_v1_NetNamespace(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_NetNamespaceList_To_v1_NetNamespaceList(in *sdnapi.NetNamespaceList, out *sdnapiv1.NetNamespaceList, s conversion.Scope) error {
	return autoconvert_api_NetNamespaceList_To_v1_NetNamespaceList(in, out, s)
}

func autoconvert_v1_ClusterNetwork_To_api_ClusterNetwork(in *sdnapiv1.ClusterNetwork, out *sdnapi.ClusterNetwork, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.ClusterNetwork))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func convert_v1_ClusterNetwork_To_api_ClusterNetwork(in *sdnapiv1.ClusterNetwork, out *sdnapi.ClusterNetwork, s conversion.Scope) error {
	return autoconvert_v1_ClusterNetwork_To_api_ClusterNetwork(in, out, s)
}

func autoconvert_v1_ClusterNetworkList_To_api_ClusterNetworkList(in *sdnapiv1.ClusterNetworkList, out *sdnapi.ClusterNetworkList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.ClusterNetworkList))(in)
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
			if err := convert_v1_ClusterNetwork_To_api_ClusterNetwork(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_ClusterNetworkList_To_api_ClusterNetworkList(in *sdnapiv1.ClusterNetworkList, out *sdnapi.ClusterNetworkList, s conversion.Scope) error {
	return autoconvert_v1_ClusterNetworkList_To_api_ClusterNetworkList(in, out, s)
}

func autoconvert_v1_HostSubnet_To_api_HostSubnet(in *sdnapiv1.HostSubnet, out *sdnapi.HostSubnet, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.HostSubnet))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func convert_v1_HostSubnet_To_api_HostSubnet(in *sdnapiv1.HostSubnet, out *sdnapi.HostSubnet, s conversion.Scope) error {
	return autoconvert_v1_HostSubnet_To_api_HostSubnet(in, out, s)
}

func autoconvert_v1_HostSubnetList_To_api_HostSubnetList(in *sdnapiv1.HostSubnetList, out *sdnapi.HostSubnetList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.HostSubnetList))(in)
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
			if err := convert_v1_HostSubnet_To_api_HostSubnet(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_HostSubnetList_To_api_HostSubnetList(in *sdnapiv1.HostSubnetList, out *sdnapi.HostSubnetList, s conversion.Scope) error {
	return autoconvert_v1_HostSubnetList_To_api_HostSubnetList(in, out, s)
}

func autoconvert_v1_NetNamespace_To_api_NetNamespace(in *sdnapiv1.NetNamespace, out *sdnapi.NetNamespace, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.NetNamespace))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func convert_v1_NetNamespace_To_api_NetNamespace(in *sdnapiv1.NetNamespace, out *sdnapi.NetNamespace, s conversion.Scope) error {
	return autoconvert_v1_NetNamespace_To_api_NetNamespace(in, out, s)
}

func autoconvert_v1_NetNamespaceList_To_api_NetNamespaceList(in *sdnapiv1.NetNamespaceList, out *sdnapi.NetNamespaceList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*sdnapiv1.NetNamespaceList))(in)
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
			if err := convert_v1_NetNamespace_To_api_NetNamespace(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_NetNamespaceList_To_api_NetNamespaceList(in *sdnapiv1.NetNamespaceList, out *sdnapi.NetNamespaceList, s conversion.Scope) error {
	return autoconvert_v1_NetNamespaceList_To_api_NetNamespaceList(in, out, s)
}

func autoconvert_api_Parameter_To_v1_Parameter(in *templateapi.Parameter, out *templateapiv1.Parameter, s conversion.Scope) error {
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

func convert_api_Parameter_To_v1_Parameter(in *templateapi.Parameter, out *templateapiv1.Parameter, s conversion.Scope) error {
	return autoconvert_api_Parameter_To_v1_Parameter(in, out, s)
}

func autoconvert_api_Template_To_v1_Template(in *templateapi.Template, out *templateapiv1.Template, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapi.Template))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if in.Parameters != nil {
		out.Parameters = make([]templateapiv1.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := convert_api_Parameter_To_v1_Parameter(&in.Parameters[i], &out.Parameters[i], s); err != nil {
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

func autoconvert_api_TemplateList_To_v1_TemplateList(in *templateapi.TemplateList, out *templateapiv1.TemplateList, s conversion.Scope) error {
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
		out.Items = make([]templateapiv1.Template, len(in.Items))
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

func convert_api_TemplateList_To_v1_TemplateList(in *templateapi.TemplateList, out *templateapiv1.TemplateList, s conversion.Scope) error {
	return autoconvert_api_TemplateList_To_v1_TemplateList(in, out, s)
}

func autoconvert_v1_Parameter_To_api_Parameter(in *templateapiv1.Parameter, out *templateapi.Parameter, s conversion.Scope) error {
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

func convert_v1_Parameter_To_api_Parameter(in *templateapiv1.Parameter, out *templateapi.Parameter, s conversion.Scope) error {
	return autoconvert_v1_Parameter_To_api_Parameter(in, out, s)
}

func autoconvert_v1_Template_To_api_Template(in *templateapiv1.Template, out *templateapi.Template, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1.Template))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Objects, &out.Objects, 0); err != nil {
		return err
	}
	if in.Parameters != nil {
		out.Parameters = make([]templateapi.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := convert_v1_Parameter_To_api_Parameter(&in.Parameters[i], &out.Parameters[i], s); err != nil {
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

func autoconvert_v1_TemplateList_To_api_TemplateList(in *templateapiv1.TemplateList, out *templateapi.TemplateList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*templateapiv1.TemplateList))(in)
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

func convert_v1_TemplateList_To_api_TemplateList(in *templateapiv1.TemplateList, out *templateapi.TemplateList, s conversion.Scope) error {
	return autoconvert_v1_TemplateList_To_api_TemplateList(in, out, s)
}

func autoconvert_api_Group_To_v1_Group(in *userapi.Group, out *userapiv1.Group, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.Group))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_Group_To_v1_Group(in *userapi.Group, out *userapiv1.Group, s conversion.Scope) error {
	return autoconvert_api_Group_To_v1_Group(in, out, s)
}

func autoconvert_api_GroupList_To_v1_GroupList(in *userapi.GroupList, out *userapiv1.GroupList, s conversion.Scope) error {
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
		out.Items = make([]userapiv1.Group, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Group_To_v1_Group(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_GroupList_To_v1_GroupList(in *userapi.GroupList, out *userapiv1.GroupList, s conversion.Scope) error {
	return autoconvert_api_GroupList_To_v1_GroupList(in, out, s)
}

func autoconvert_api_Identity_To_v1_Identity(in *userapi.Identity, out *userapiv1.Identity, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.Identity))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.User, &out.User, s); err != nil {
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

func convert_api_Identity_To_v1_Identity(in *userapi.Identity, out *userapiv1.Identity, s conversion.Scope) error {
	return autoconvert_api_Identity_To_v1_Identity(in, out, s)
}

func autoconvert_api_IdentityList_To_v1_IdentityList(in *userapi.IdentityList, out *userapiv1.IdentityList, s conversion.Scope) error {
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
		out.Items = make([]userapiv1.Identity, len(in.Items))
		for i := range in.Items {
			if err := convert_api_Identity_To_v1_Identity(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_IdentityList_To_v1_IdentityList(in *userapi.IdentityList, out *userapiv1.IdentityList, s conversion.Scope) error {
	return autoconvert_api_IdentityList_To_v1_IdentityList(in, out, s)
}

func autoconvert_api_User_To_v1_User(in *userapi.User, out *userapiv1.User, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.User))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_api_User_To_v1_User(in *userapi.User, out *userapiv1.User, s conversion.Scope) error {
	return autoconvert_api_User_To_v1_User(in, out, s)
}

func autoconvert_api_UserIdentityMapping_To_v1_UserIdentityMapping(in *userapi.UserIdentityMapping, out *userapiv1.UserIdentityMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapi.UserIdentityMapping))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := convert_api_ObjectReference_To_v1_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func convert_api_UserIdentityMapping_To_v1_UserIdentityMapping(in *userapi.UserIdentityMapping, out *userapiv1.UserIdentityMapping, s conversion.Scope) error {
	return autoconvert_api_UserIdentityMapping_To_v1_UserIdentityMapping(in, out, s)
}

func autoconvert_api_UserList_To_v1_UserList(in *userapi.UserList, out *userapiv1.UserList, s conversion.Scope) error {
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
		out.Items = make([]userapiv1.User, len(in.Items))
		for i := range in.Items {
			if err := convert_api_User_To_v1_User(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_UserList_To_v1_UserList(in *userapi.UserList, out *userapiv1.UserList, s conversion.Scope) error {
	return autoconvert_api_UserList_To_v1_UserList(in, out, s)
}

func autoconvert_v1_Group_To_api_Group(in *userapiv1.Group, out *userapi.Group, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.Group))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_Group_To_api_Group(in *userapiv1.Group, out *userapi.Group, s conversion.Scope) error {
	return autoconvert_v1_Group_To_api_Group(in, out, s)
}

func autoconvert_v1_GroupList_To_api_GroupList(in *userapiv1.GroupList, out *userapi.GroupList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.GroupList))(in)
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
			if err := convert_v1_Group_To_api_Group(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_GroupList_To_api_GroupList(in *userapiv1.GroupList, out *userapi.GroupList, s conversion.Scope) error {
	return autoconvert_v1_GroupList_To_api_GroupList(in, out, s)
}

func autoconvert_v1_Identity_To_api_Identity(in *userapiv1.Identity, out *userapi.Identity, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.Identity))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
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

func convert_v1_Identity_To_api_Identity(in *userapiv1.Identity, out *userapi.Identity, s conversion.Scope) error {
	return autoconvert_v1_Identity_To_api_Identity(in, out, s)
}

func autoconvert_v1_IdentityList_To_api_IdentityList(in *userapiv1.IdentityList, out *userapi.IdentityList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.IdentityList))(in)
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
			if err := convert_v1_Identity_To_api_Identity(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_IdentityList_To_api_IdentityList(in *userapiv1.IdentityList, out *userapi.IdentityList, s conversion.Scope) error {
	return autoconvert_v1_IdentityList_To_api_IdentityList(in, out, s)
}

func autoconvert_v1_User_To_api_User(in *userapiv1.User, out *userapi.User, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.User))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
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

func convert_v1_User_To_api_User(in *userapiv1.User, out *userapi.User, s conversion.Scope) error {
	return autoconvert_v1_User_To_api_User(in, out, s)
}

func autoconvert_v1_UserIdentityMapping_To_api_UserIdentityMapping(in *userapiv1.UserIdentityMapping, out *userapi.UserIdentityMapping, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.UserIdentityMapping))(in)
	}
	if err := s.Convert(&in.TypeMeta, &out.TypeMeta, 0); err != nil {
		return err
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.Identity, &out.Identity, s); err != nil {
		return err
	}
	if err := convert_v1_ObjectReference_To_api_ObjectReference(&in.User, &out.User, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_UserIdentityMapping_To_api_UserIdentityMapping(in *userapiv1.UserIdentityMapping, out *userapi.UserIdentityMapping, s conversion.Scope) error {
	return autoconvert_v1_UserIdentityMapping_To_api_UserIdentityMapping(in, out, s)
}

func autoconvert_v1_UserList_To_api_UserList(in *userapiv1.UserList, out *userapi.UserList, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*userapiv1.UserList))(in)
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
			if err := convert_v1_User_To_api_User(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_UserList_To_api_UserList(in *userapiv1.UserList, out *userapi.UserList, s conversion.Scope) error {
	return autoconvert_v1_UserList_To_api_UserList(in, out, s)
}

func autoconvert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource(in *pkgapi.AWSElasticBlockStoreVolumeSource, out *pkgapiv1.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.AWSElasticBlockStoreVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.Partition = in.Partition
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource(in *pkgapi.AWSElasticBlockStoreVolumeSource, out *pkgapiv1.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	return autoconvert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource(in, out, s)
}

func autoconvert_api_Capabilities_To_v1_Capabilities(in *pkgapi.Capabilities, out *pkgapiv1.Capabilities, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.Capabilities))(in)
	}
	if in.Add != nil {
		out.Add = make([]pkgapiv1.Capability, len(in.Add))
		for i := range in.Add {
			out.Add[i] = pkgapiv1.Capability(in.Add[i])
		}
	} else {
		out.Add = nil
	}
	if in.Drop != nil {
		out.Drop = make([]pkgapiv1.Capability, len(in.Drop))
		for i := range in.Drop {
			out.Drop[i] = pkgapiv1.Capability(in.Drop[i])
		}
	} else {
		out.Drop = nil
	}
	return nil
}

func convert_api_Capabilities_To_v1_Capabilities(in *pkgapi.Capabilities, out *pkgapiv1.Capabilities, s conversion.Scope) error {
	return autoconvert_api_Capabilities_To_v1_Capabilities(in, out, s)
}

func autoconvert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource(in *pkgapi.CephFSVolumeSource, out *pkgapiv1.CephFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.CephFSVolumeSource))(in)
	}
	if in.Monitors != nil {
		out.Monitors = make([]string, len(in.Monitors))
		for i := range in.Monitors {
			out.Monitors[i] = in.Monitors[i]
		}
	} else {
		out.Monitors = nil
	}
	out.User = in.User
	out.SecretFile = in.SecretFile
	if in.SecretRef != nil {
		out.SecretRef = new(pkgapiv1.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource(in *pkgapi.CephFSVolumeSource, out *pkgapiv1.CephFSVolumeSource, s conversion.Scope) error {
	return autoconvert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource(in, out, s)
}

func autoconvert_api_CinderVolumeSource_To_v1_CinderVolumeSource(in *pkgapi.CinderVolumeSource, out *pkgapiv1.CinderVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.CinderVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_api_CinderVolumeSource_To_v1_CinderVolumeSource(in *pkgapi.CinderVolumeSource, out *pkgapiv1.CinderVolumeSource, s conversion.Scope) error {
	return autoconvert_api_CinderVolumeSource_To_v1_CinderVolumeSource(in, out, s)
}

func autoconvert_api_Container_To_v1_Container(in *pkgapi.Container, out *pkgapiv1.Container, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.Container))(in)
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
		out.Ports = make([]pkgapiv1.ContainerPort, len(in.Ports))
		for i := range in.Ports {
			if err := convert_api_ContainerPort_To_v1_ContainerPort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ports = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapiv1.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_api_EnvVar_To_v1_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	if err := convert_api_ResourceRequirements_To_v1_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if in.VolumeMounts != nil {
		out.VolumeMounts = make([]pkgapiv1.VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			if err := convert_api_VolumeMount_To_v1_VolumeMount(&in.VolumeMounts[i], &out.VolumeMounts[i], s); err != nil {
				return err
			}
		}
	} else {
		out.VolumeMounts = nil
	}
	if in.LivenessProbe != nil {
		out.LivenessProbe = new(pkgapiv1.Probe)
		if err := convert_api_Probe_To_v1_Probe(in.LivenessProbe, out.LivenessProbe, s); err != nil {
			return err
		}
	} else {
		out.LivenessProbe = nil
	}
	if in.ReadinessProbe != nil {
		out.ReadinessProbe = new(pkgapiv1.Probe)
		if err := convert_api_Probe_To_v1_Probe(in.ReadinessProbe, out.ReadinessProbe, s); err != nil {
			return err
		}
	} else {
		out.ReadinessProbe = nil
	}
	if in.Lifecycle != nil {
		out.Lifecycle = new(pkgapiv1.Lifecycle)
		if err := convert_api_Lifecycle_To_v1_Lifecycle(in.Lifecycle, out.Lifecycle, s); err != nil {
			return err
		}
	} else {
		out.Lifecycle = nil
	}
	out.TerminationMessagePath = in.TerminationMessagePath
	out.ImagePullPolicy = pkgapiv1.PullPolicy(in.ImagePullPolicy)
	if in.SecurityContext != nil {
		out.SecurityContext = new(pkgapiv1.SecurityContext)
		if err := convert_api_SecurityContext_To_v1_SecurityContext(in.SecurityContext, out.SecurityContext, s); err != nil {
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

func convert_api_Container_To_v1_Container(in *pkgapi.Container, out *pkgapiv1.Container, s conversion.Scope) error {
	return autoconvert_api_Container_To_v1_Container(in, out, s)
}

func autoconvert_api_ContainerPort_To_v1_ContainerPort(in *pkgapi.ContainerPort, out *pkgapiv1.ContainerPort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.ContainerPort))(in)
	}
	out.Name = in.Name
	out.HostPort = in.HostPort
	out.ContainerPort = in.ContainerPort
	out.Protocol = pkgapiv1.Protocol(in.Protocol)
	out.HostIP = in.HostIP
	return nil
}

func convert_api_ContainerPort_To_v1_ContainerPort(in *pkgapi.ContainerPort, out *pkgapiv1.ContainerPort, s conversion.Scope) error {
	return autoconvert_api_ContainerPort_To_v1_ContainerPort(in, out, s)
}

func autoconvert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile(in *pkgapi.DownwardAPIVolumeFile, out *pkgapiv1.DownwardAPIVolumeFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.DownwardAPIVolumeFile))(in)
	}
	out.Path = in.Path
	if err := convert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}

func convert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile(in *pkgapi.DownwardAPIVolumeFile, out *pkgapiv1.DownwardAPIVolumeFile, s conversion.Scope) error {
	return autoconvert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile(in, out, s)
}

func autoconvert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource(in *pkgapi.DownwardAPIVolumeSource, out *pkgapiv1.DownwardAPIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.DownwardAPIVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]pkgapiv1.DownwardAPIVolumeFile, len(in.Items))
		for i := range in.Items {
			if err := convert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource(in *pkgapi.DownwardAPIVolumeSource, out *pkgapiv1.DownwardAPIVolumeSource, s conversion.Scope) error {
	return autoconvert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource(in, out, s)
}

func autoconvert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource(in *pkgapi.EmptyDirVolumeSource, out *pkgapiv1.EmptyDirVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.EmptyDirVolumeSource))(in)
	}
	out.Medium = pkgapiv1.StorageMedium(in.Medium)
	return nil
}

func convert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource(in *pkgapi.EmptyDirVolumeSource, out *pkgapiv1.EmptyDirVolumeSource, s conversion.Scope) error {
	return autoconvert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource(in, out, s)
}

func autoconvert_api_EnvVar_To_v1_EnvVar(in *pkgapi.EnvVar, out *pkgapiv1.EnvVar, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.EnvVar))(in)
	}
	out.Name = in.Name
	out.Value = in.Value
	if in.ValueFrom != nil {
		out.ValueFrom = new(pkgapiv1.EnvVarSource)
		if err := convert_api_EnvVarSource_To_v1_EnvVarSource(in.ValueFrom, out.ValueFrom, s); err != nil {
			return err
		}
	} else {
		out.ValueFrom = nil
	}
	return nil
}

func convert_api_EnvVar_To_v1_EnvVar(in *pkgapi.EnvVar, out *pkgapiv1.EnvVar, s conversion.Scope) error {
	return autoconvert_api_EnvVar_To_v1_EnvVar(in, out, s)
}

func autoconvert_api_EnvVarSource_To_v1_EnvVarSource(in *pkgapi.EnvVarSource, out *pkgapiv1.EnvVarSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.EnvVarSource))(in)
	}
	if in.FieldRef != nil {
		out.FieldRef = new(pkgapiv1.ObjectFieldSelector)
		if err := convert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(in.FieldRef, out.FieldRef, s); err != nil {
			return err
		}
	} else {
		out.FieldRef = nil
	}
	return nil
}

func convert_api_EnvVarSource_To_v1_EnvVarSource(in *pkgapi.EnvVarSource, out *pkgapiv1.EnvVarSource, s conversion.Scope) error {
	return autoconvert_api_EnvVarSource_To_v1_EnvVarSource(in, out, s)
}

func autoconvert_api_ExecAction_To_v1_ExecAction(in *pkgapi.ExecAction, out *pkgapiv1.ExecAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.ExecAction))(in)
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

func convert_api_ExecAction_To_v1_ExecAction(in *pkgapi.ExecAction, out *pkgapiv1.ExecAction, s conversion.Scope) error {
	return autoconvert_api_ExecAction_To_v1_ExecAction(in, out, s)
}

func autoconvert_api_FCVolumeSource_To_v1_FCVolumeSource(in *pkgapi.FCVolumeSource, out *pkgapiv1.FCVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.FCVolumeSource))(in)
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

func convert_api_FCVolumeSource_To_v1_FCVolumeSource(in *pkgapi.FCVolumeSource, out *pkgapiv1.FCVolumeSource, s conversion.Scope) error {
	return autoconvert_api_FCVolumeSource_To_v1_FCVolumeSource(in, out, s)
}

func autoconvert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource(in *pkgapi.FlockerVolumeSource, out *pkgapiv1.FlockerVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.FlockerVolumeSource))(in)
	}
	out.DatasetName = in.DatasetName
	return nil
}

func convert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource(in *pkgapi.FlockerVolumeSource, out *pkgapiv1.FlockerVolumeSource, s conversion.Scope) error {
	return autoconvert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource(in, out, s)
}

func autoconvert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource(in *pkgapi.GCEPersistentDiskVolumeSource, out *pkgapiv1.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.GCEPersistentDiskVolumeSource))(in)
	}
	out.PDName = in.PDName
	out.FSType = in.FSType
	out.Partition = in.Partition
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource(in *pkgapi.GCEPersistentDiskVolumeSource, out *pkgapiv1.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	return autoconvert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource(in, out, s)
}

func autoconvert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource(in *pkgapi.GitRepoVolumeSource, out *pkgapiv1.GitRepoVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.GitRepoVolumeSource))(in)
	}
	out.Repository = in.Repository
	out.Revision = in.Revision
	return nil
}

func convert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource(in *pkgapi.GitRepoVolumeSource, out *pkgapiv1.GitRepoVolumeSource, s conversion.Scope) error {
	return autoconvert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource(in, out, s)
}

func autoconvert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource(in *pkgapi.GlusterfsVolumeSource, out *pkgapiv1.GlusterfsVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.GlusterfsVolumeSource))(in)
	}
	out.EndpointsName = in.EndpointsName
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource(in *pkgapi.GlusterfsVolumeSource, out *pkgapiv1.GlusterfsVolumeSource, s conversion.Scope) error {
	return autoconvert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource(in, out, s)
}

func autoconvert_api_HTTPGetAction_To_v1_HTTPGetAction(in *pkgapi.HTTPGetAction, out *pkgapiv1.HTTPGetAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.HTTPGetAction))(in)
	}
	out.Path = in.Path
	if err := s.Convert(&in.Port, &out.Port, 0); err != nil {
		return err
	}
	out.Host = in.Host
	out.Scheme = pkgapiv1.URIScheme(in.Scheme)
	return nil
}

func convert_api_HTTPGetAction_To_v1_HTTPGetAction(in *pkgapi.HTTPGetAction, out *pkgapiv1.HTTPGetAction, s conversion.Scope) error {
	return autoconvert_api_HTTPGetAction_To_v1_HTTPGetAction(in, out, s)
}

func autoconvert_api_Handler_To_v1_Handler(in *pkgapi.Handler, out *pkgapiv1.Handler, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.Handler))(in)
	}
	if in.Exec != nil {
		out.Exec = new(pkgapiv1.ExecAction)
		if err := convert_api_ExecAction_To_v1_ExecAction(in.Exec, out.Exec, s); err != nil {
			return err
		}
	} else {
		out.Exec = nil
	}
	if in.HTTPGet != nil {
		out.HTTPGet = new(pkgapiv1.HTTPGetAction)
		if err := convert_api_HTTPGetAction_To_v1_HTTPGetAction(in.HTTPGet, out.HTTPGet, s); err != nil {
			return err
		}
	} else {
		out.HTTPGet = nil
	}
	if in.TCPSocket != nil {
		out.TCPSocket = new(pkgapiv1.TCPSocketAction)
		if err := convert_api_TCPSocketAction_To_v1_TCPSocketAction(in.TCPSocket, out.TCPSocket, s); err != nil {
			return err
		}
	} else {
		out.TCPSocket = nil
	}
	return nil
}

func convert_api_Handler_To_v1_Handler(in *pkgapi.Handler, out *pkgapiv1.Handler, s conversion.Scope) error {
	return autoconvert_api_Handler_To_v1_Handler(in, out, s)
}

func autoconvert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource(in *pkgapi.HostPathVolumeSource, out *pkgapiv1.HostPathVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.HostPathVolumeSource))(in)
	}
	out.Path = in.Path
	return nil
}

func convert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource(in *pkgapi.HostPathVolumeSource, out *pkgapiv1.HostPathVolumeSource, s conversion.Scope) error {
	return autoconvert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource(in, out, s)
}

func autoconvert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource(in *pkgapi.ISCSIVolumeSource, out *pkgapiv1.ISCSIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.ISCSIVolumeSource))(in)
	}
	out.TargetPortal = in.TargetPortal
	out.IQN = in.IQN
	out.Lun = in.Lun
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource(in *pkgapi.ISCSIVolumeSource, out *pkgapiv1.ISCSIVolumeSource, s conversion.Scope) error {
	return autoconvert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource(in, out, s)
}

func autoconvert_api_Lifecycle_To_v1_Lifecycle(in *pkgapi.Lifecycle, out *pkgapiv1.Lifecycle, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.Lifecycle))(in)
	}
	if in.PostStart != nil {
		out.PostStart = new(pkgapiv1.Handler)
		if err := convert_api_Handler_To_v1_Handler(in.PostStart, out.PostStart, s); err != nil {
			return err
		}
	} else {
		out.PostStart = nil
	}
	if in.PreStop != nil {
		out.PreStop = new(pkgapiv1.Handler)
		if err := convert_api_Handler_To_v1_Handler(in.PreStop, out.PreStop, s); err != nil {
			return err
		}
	} else {
		out.PreStop = nil
	}
	return nil
}

func convert_api_Lifecycle_To_v1_Lifecycle(in *pkgapi.Lifecycle, out *pkgapiv1.Lifecycle, s conversion.Scope) error {
	return autoconvert_api_Lifecycle_To_v1_Lifecycle(in, out, s)
}

func autoconvert_api_LocalObjectReference_To_v1_LocalObjectReference(in *pkgapi.LocalObjectReference, out *pkgapiv1.LocalObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.LocalObjectReference))(in)
	}
	out.Name = in.Name
	return nil
}

func convert_api_LocalObjectReference_To_v1_LocalObjectReference(in *pkgapi.LocalObjectReference, out *pkgapiv1.LocalObjectReference, s conversion.Scope) error {
	return autoconvert_api_LocalObjectReference_To_v1_LocalObjectReference(in, out, s)
}

func autoconvert_api_NFSVolumeSource_To_v1_NFSVolumeSource(in *pkgapi.NFSVolumeSource, out *pkgapiv1.NFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.NFSVolumeSource))(in)
	}
	out.Server = in.Server
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_api_NFSVolumeSource_To_v1_NFSVolumeSource(in *pkgapi.NFSVolumeSource, out *pkgapiv1.NFSVolumeSource, s conversion.Scope) error {
	return autoconvert_api_NFSVolumeSource_To_v1_NFSVolumeSource(in, out, s)
}

func autoconvert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(in *pkgapi.ObjectFieldSelector, out *pkgapiv1.ObjectFieldSelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.ObjectFieldSelector))(in)
	}
	out.APIVersion = in.APIVersion
	out.FieldPath = in.FieldPath
	return nil
}

func convert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(in *pkgapi.ObjectFieldSelector, out *pkgapiv1.ObjectFieldSelector, s conversion.Scope) error {
	return autoconvert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector(in, out, s)
}

func autoconvert_api_ObjectMeta_To_v1_ObjectMeta(in *pkgapi.ObjectMeta, out *pkgapiv1.ObjectMeta, s conversion.Scope) error {
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

func convert_api_ObjectMeta_To_v1_ObjectMeta(in *pkgapi.ObjectMeta, out *pkgapiv1.ObjectMeta, s conversion.Scope) error {
	return autoconvert_api_ObjectMeta_To_v1_ObjectMeta(in, out, s)
}

func autoconvert_api_ObjectReference_To_v1_ObjectReference(in *pkgapi.ObjectReference, out *pkgapiv1.ObjectReference, s conversion.Scope) error {
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

func convert_api_ObjectReference_To_v1_ObjectReference(in *pkgapi.ObjectReference, out *pkgapiv1.ObjectReference, s conversion.Scope) error {
	return autoconvert_api_ObjectReference_To_v1_ObjectReference(in, out, s)
}

func autoconvert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource(in *pkgapi.PersistentVolumeClaimVolumeSource, out *pkgapiv1.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.PersistentVolumeClaimVolumeSource))(in)
	}
	out.ClaimName = in.ClaimName
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource(in *pkgapi.PersistentVolumeClaimVolumeSource, out *pkgapiv1.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	return autoconvert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource(in, out, s)
}

func autoconvert_api_PodSpec_To_v1_PodSpec(in *pkgapi.PodSpec, out *pkgapiv1.PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]pkgapiv1.Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := convert_api_Volume_To_v1_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]pkgapiv1.Container, len(in.Containers))
		for i := range in.Containers {
			if err := convert_api_Container_To_v1_Container(&in.Containers[i], &out.Containers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Containers = nil
	}
	out.RestartPolicy = pkgapiv1.RestartPolicy(in.RestartPolicy)
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
	out.DNSPolicy = pkgapiv1.DNSPolicy(in.DNSPolicy)
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
	if in.SecurityContext != nil {
		if err := s.Convert(&in.SecurityContext, &out.SecurityContext, 0); err != nil {
			return err
		}
	} else {
		out.SecurityContext = nil
	}
	if in.ImagePullSecrets != nil {
		out.ImagePullSecrets = make([]pkgapiv1.LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}
	return nil
}

func autoconvert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in *pkgapi.PodTemplateSpec, out *pkgapiv1.PodTemplateSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.PodTemplateSpec))(in)
	}
	if err := convert_api_ObjectMeta_To_v1_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec, &out.Spec, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in *pkgapi.PodTemplateSpec, out *pkgapiv1.PodTemplateSpec, s conversion.Scope) error {
	return autoconvert_api_PodTemplateSpec_To_v1_PodTemplateSpec(in, out, s)
}

func autoconvert_api_Probe_To_v1_Probe(in *pkgapi.Probe, out *pkgapiv1.Probe, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.Probe))(in)
	}
	if err := convert_api_Handler_To_v1_Handler(&in.Handler, &out.Handler, s); err != nil {
		return err
	}
	out.InitialDelaySeconds = in.InitialDelaySeconds
	out.TimeoutSeconds = in.TimeoutSeconds
	return nil
}

func convert_api_Probe_To_v1_Probe(in *pkgapi.Probe, out *pkgapiv1.Probe, s conversion.Scope) error {
	return autoconvert_api_Probe_To_v1_Probe(in, out, s)
}

func autoconvert_api_RBDVolumeSource_To_v1_RBDVolumeSource(in *pkgapi.RBDVolumeSource, out *pkgapiv1.RBDVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.RBDVolumeSource))(in)
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
	if in.SecretRef != nil {
		out.SecretRef = new(pkgapiv1.LocalObjectReference)
		if err := convert_api_LocalObjectReference_To_v1_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_api_RBDVolumeSource_To_v1_RBDVolumeSource(in *pkgapi.RBDVolumeSource, out *pkgapiv1.RBDVolumeSource, s conversion.Scope) error {
	return autoconvert_api_RBDVolumeSource_To_v1_RBDVolumeSource(in, out, s)
}

func autoconvert_api_ResourceRequirements_To_v1_ResourceRequirements(in *pkgapi.ResourceRequirements, out *pkgapiv1.ResourceRequirements, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.ResourceRequirements))(in)
	}
	if in.Limits != nil {
		out.Limits = make(pkgapiv1.ResourceList)
		for key, val := range in.Limits {
			newVal := resource.Quantity{}
			if err := s.Convert(&val, &newVal, 0); err != nil {
				return err
			}
			out.Limits[pkgapiv1.ResourceName(key)] = newVal
		}
	} else {
		out.Limits = nil
	}
	if in.Requests != nil {
		out.Requests = make(pkgapiv1.ResourceList)
		for key, val := range in.Requests {
			newVal := resource.Quantity{}
			if err := s.Convert(&val, &newVal, 0); err != nil {
				return err
			}
			out.Requests[pkgapiv1.ResourceName(key)] = newVal
		}
	} else {
		out.Requests = nil
	}
	return nil
}

func convert_api_ResourceRequirements_To_v1_ResourceRequirements(in *pkgapi.ResourceRequirements, out *pkgapiv1.ResourceRequirements, s conversion.Scope) error {
	return autoconvert_api_ResourceRequirements_To_v1_ResourceRequirements(in, out, s)
}

func autoconvert_api_SELinuxOptions_To_v1_SELinuxOptions(in *pkgapi.SELinuxOptions, out *pkgapiv1.SELinuxOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.SELinuxOptions))(in)
	}
	out.User = in.User
	out.Role = in.Role
	out.Type = in.Type
	out.Level = in.Level
	return nil
}

func convert_api_SELinuxOptions_To_v1_SELinuxOptions(in *pkgapi.SELinuxOptions, out *pkgapiv1.SELinuxOptions, s conversion.Scope) error {
	return autoconvert_api_SELinuxOptions_To_v1_SELinuxOptions(in, out, s)
}

func autoconvert_api_SecretVolumeSource_To_v1_SecretVolumeSource(in *pkgapi.SecretVolumeSource, out *pkgapiv1.SecretVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.SecretVolumeSource))(in)
	}
	out.SecretName = in.SecretName
	return nil
}

func convert_api_SecretVolumeSource_To_v1_SecretVolumeSource(in *pkgapi.SecretVolumeSource, out *pkgapiv1.SecretVolumeSource, s conversion.Scope) error {
	return autoconvert_api_SecretVolumeSource_To_v1_SecretVolumeSource(in, out, s)
}

func autoconvert_api_SecurityContext_To_v1_SecurityContext(in *pkgapi.SecurityContext, out *pkgapiv1.SecurityContext, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.SecurityContext))(in)
	}
	if in.Capabilities != nil {
		out.Capabilities = new(pkgapiv1.Capabilities)
		if err := convert_api_Capabilities_To_v1_Capabilities(in.Capabilities, out.Capabilities, s); err != nil {
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
	if in.SELinuxOptions != nil {
		out.SELinuxOptions = new(pkgapiv1.SELinuxOptions)
		if err := convert_api_SELinuxOptions_To_v1_SELinuxOptions(in.SELinuxOptions, out.SELinuxOptions, s); err != nil {
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
	return nil
}

func convert_api_SecurityContext_To_v1_SecurityContext(in *pkgapi.SecurityContext, out *pkgapiv1.SecurityContext, s conversion.Scope) error {
	return autoconvert_api_SecurityContext_To_v1_SecurityContext(in, out, s)
}

func autoconvert_api_TCPSocketAction_To_v1_TCPSocketAction(in *pkgapi.TCPSocketAction, out *pkgapiv1.TCPSocketAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.TCPSocketAction))(in)
	}
	if err := s.Convert(&in.Port, &out.Port, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_TCPSocketAction_To_v1_TCPSocketAction(in *pkgapi.TCPSocketAction, out *pkgapiv1.TCPSocketAction, s conversion.Scope) error {
	return autoconvert_api_TCPSocketAction_To_v1_TCPSocketAction(in, out, s)
}

func autoconvert_api_Volume_To_v1_Volume(in *pkgapi.Volume, out *pkgapiv1.Volume, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.Volume))(in)
	}
	out.Name = in.Name
	if err := s.Convert(&in.VolumeSource, &out.VolumeSource, 0); err != nil {
		return err
	}
	return nil
}

func convert_api_Volume_To_v1_Volume(in *pkgapi.Volume, out *pkgapiv1.Volume, s conversion.Scope) error {
	return autoconvert_api_Volume_To_v1_Volume(in, out, s)
}

func autoconvert_api_VolumeMount_To_v1_VolumeMount(in *pkgapi.VolumeMount, out *pkgapiv1.VolumeMount, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.VolumeMount))(in)
	}
	out.Name = in.Name
	out.ReadOnly = in.ReadOnly
	out.MountPath = in.MountPath
	return nil
}

func convert_api_VolumeMount_To_v1_VolumeMount(in *pkgapi.VolumeMount, out *pkgapiv1.VolumeMount, s conversion.Scope) error {
	return autoconvert_api_VolumeMount_To_v1_VolumeMount(in, out, s)
}

func autoconvert_api_VolumeSource_To_v1_VolumeSource(in *pkgapi.VolumeSource, out *pkgapiv1.VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapi.VolumeSource))(in)
	}
	if in.HostPath != nil {
		out.HostPath = new(pkgapiv1.HostPathVolumeSource)
		if err := convert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource(in.HostPath, out.HostPath, s); err != nil {
			return err
		}
	} else {
		out.HostPath = nil
	}
	if in.EmptyDir != nil {
		out.EmptyDir = new(pkgapiv1.EmptyDirVolumeSource)
		if err := convert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource(in.EmptyDir, out.EmptyDir, s); err != nil {
			return err
		}
	} else {
		out.EmptyDir = nil
	}
	if in.GCEPersistentDisk != nil {
		out.GCEPersistentDisk = new(pkgapiv1.GCEPersistentDiskVolumeSource)
		if err := convert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource(in.GCEPersistentDisk, out.GCEPersistentDisk, s); err != nil {
			return err
		}
	} else {
		out.GCEPersistentDisk = nil
	}
	if in.AWSElasticBlockStore != nil {
		out.AWSElasticBlockStore = new(pkgapiv1.AWSElasticBlockStoreVolumeSource)
		if err := convert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource(in.AWSElasticBlockStore, out.AWSElasticBlockStore, s); err != nil {
			return err
		}
	} else {
		out.AWSElasticBlockStore = nil
	}
	if in.GitRepo != nil {
		out.GitRepo = new(pkgapiv1.GitRepoVolumeSource)
		if err := convert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource(in.GitRepo, out.GitRepo, s); err != nil {
			return err
		}
	} else {
		out.GitRepo = nil
	}
	if in.Secret != nil {
		out.Secret = new(pkgapiv1.SecretVolumeSource)
		if err := convert_api_SecretVolumeSource_To_v1_SecretVolumeSource(in.Secret, out.Secret, s); err != nil {
			return err
		}
	} else {
		out.Secret = nil
	}
	if in.NFS != nil {
		out.NFS = new(pkgapiv1.NFSVolumeSource)
		if err := convert_api_NFSVolumeSource_To_v1_NFSVolumeSource(in.NFS, out.NFS, s); err != nil {
			return err
		}
	} else {
		out.NFS = nil
	}
	if in.ISCSI != nil {
		out.ISCSI = new(pkgapiv1.ISCSIVolumeSource)
		if err := convert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource(in.ISCSI, out.ISCSI, s); err != nil {
			return err
		}
	} else {
		out.ISCSI = nil
	}
	if in.Glusterfs != nil {
		out.Glusterfs = new(pkgapiv1.GlusterfsVolumeSource)
		if err := convert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource(in.Glusterfs, out.Glusterfs, s); err != nil {
			return err
		}
	} else {
		out.Glusterfs = nil
	}
	if in.PersistentVolumeClaim != nil {
		out.PersistentVolumeClaim = new(pkgapiv1.PersistentVolumeClaimVolumeSource)
		if err := convert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource(in.PersistentVolumeClaim, out.PersistentVolumeClaim, s); err != nil {
			return err
		}
	} else {
		out.PersistentVolumeClaim = nil
	}
	if in.RBD != nil {
		out.RBD = new(pkgapiv1.RBDVolumeSource)
		if err := convert_api_RBDVolumeSource_To_v1_RBDVolumeSource(in.RBD, out.RBD, s); err != nil {
			return err
		}
	} else {
		out.RBD = nil
	}
	if in.Cinder != nil {
		out.Cinder = new(pkgapiv1.CinderVolumeSource)
		if err := convert_api_CinderVolumeSource_To_v1_CinderVolumeSource(in.Cinder, out.Cinder, s); err != nil {
			return err
		}
	} else {
		out.Cinder = nil
	}
	if in.CephFS != nil {
		out.CephFS = new(pkgapiv1.CephFSVolumeSource)
		if err := convert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource(in.CephFS, out.CephFS, s); err != nil {
			return err
		}
	} else {
		out.CephFS = nil
	}
	if in.Flocker != nil {
		out.Flocker = new(pkgapiv1.FlockerVolumeSource)
		if err := convert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource(in.Flocker, out.Flocker, s); err != nil {
			return err
		}
	} else {
		out.Flocker = nil
	}
	if in.DownwardAPI != nil {
		out.DownwardAPI = new(pkgapiv1.DownwardAPIVolumeSource)
		if err := convert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource(in.DownwardAPI, out.DownwardAPI, s); err != nil {
			return err
		}
	} else {
		out.DownwardAPI = nil
	}
	if in.FC != nil {
		out.FC = new(pkgapiv1.FCVolumeSource)
		if err := convert_api_FCVolumeSource_To_v1_FCVolumeSource(in.FC, out.FC, s); err != nil {
			return err
		}
	} else {
		out.FC = nil
	}
	return nil
}

func autoconvert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in *pkgapiv1.AWSElasticBlockStoreVolumeSource, out *pkgapi.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.AWSElasticBlockStoreVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.Partition = in.Partition
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in *pkgapiv1.AWSElasticBlockStoreVolumeSource, out *pkgapi.AWSElasticBlockStoreVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in, out, s)
}

func autoconvert_v1_Capabilities_To_api_Capabilities(in *pkgapiv1.Capabilities, out *pkgapi.Capabilities, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.Capabilities))(in)
	}
	if in.Add != nil {
		out.Add = make([]pkgapi.Capability, len(in.Add))
		for i := range in.Add {
			out.Add[i] = pkgapi.Capability(in.Add[i])
		}
	} else {
		out.Add = nil
	}
	if in.Drop != nil {
		out.Drop = make([]pkgapi.Capability, len(in.Drop))
		for i := range in.Drop {
			out.Drop[i] = pkgapi.Capability(in.Drop[i])
		}
	} else {
		out.Drop = nil
	}
	return nil
}

func convert_v1_Capabilities_To_api_Capabilities(in *pkgapiv1.Capabilities, out *pkgapi.Capabilities, s conversion.Scope) error {
	return autoconvert_v1_Capabilities_To_api_Capabilities(in, out, s)
}

func autoconvert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource(in *pkgapiv1.CephFSVolumeSource, out *pkgapi.CephFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.CephFSVolumeSource))(in)
	}
	if in.Monitors != nil {
		out.Monitors = make([]string, len(in.Monitors))
		for i := range in.Monitors {
			out.Monitors[i] = in.Monitors[i]
		}
	} else {
		out.Monitors = nil
	}
	out.User = in.User
	out.SecretFile = in.SecretFile
	if in.SecretRef != nil {
		out.SecretRef = new(pkgapi.LocalObjectReference)
		if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource(in *pkgapiv1.CephFSVolumeSource, out *pkgapi.CephFSVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource(in, out, s)
}

func autoconvert_v1_CinderVolumeSource_To_api_CinderVolumeSource(in *pkgapiv1.CinderVolumeSource, out *pkgapi.CinderVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.CinderVolumeSource))(in)
	}
	out.VolumeID = in.VolumeID
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_v1_CinderVolumeSource_To_api_CinderVolumeSource(in *pkgapiv1.CinderVolumeSource, out *pkgapi.CinderVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_CinderVolumeSource_To_api_CinderVolumeSource(in, out, s)
}

func autoconvert_v1_Container_To_api_Container(in *pkgapiv1.Container, out *pkgapi.Container, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.Container))(in)
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
		out.Ports = make([]pkgapi.ContainerPort, len(in.Ports))
		for i := range in.Ports {
			if err := convert_v1_ContainerPort_To_api_ContainerPort(&in.Ports[i], &out.Ports[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Ports = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if err := convert_v1_EnvVar_To_api_EnvVar(&in.Env[i], &out.Env[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Env = nil
	}
	if err := convert_v1_ResourceRequirements_To_api_ResourceRequirements(&in.Resources, &out.Resources, s); err != nil {
		return err
	}
	if in.VolumeMounts != nil {
		out.VolumeMounts = make([]pkgapi.VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			if err := convert_v1_VolumeMount_To_api_VolumeMount(&in.VolumeMounts[i], &out.VolumeMounts[i], s); err != nil {
				return err
			}
		}
	} else {
		out.VolumeMounts = nil
	}
	if in.LivenessProbe != nil {
		out.LivenessProbe = new(pkgapi.Probe)
		if err := convert_v1_Probe_To_api_Probe(in.LivenessProbe, out.LivenessProbe, s); err != nil {
			return err
		}
	} else {
		out.LivenessProbe = nil
	}
	if in.ReadinessProbe != nil {
		out.ReadinessProbe = new(pkgapi.Probe)
		if err := convert_v1_Probe_To_api_Probe(in.ReadinessProbe, out.ReadinessProbe, s); err != nil {
			return err
		}
	} else {
		out.ReadinessProbe = nil
	}
	if in.Lifecycle != nil {
		out.Lifecycle = new(pkgapi.Lifecycle)
		if err := convert_v1_Lifecycle_To_api_Lifecycle(in.Lifecycle, out.Lifecycle, s); err != nil {
			return err
		}
	} else {
		out.Lifecycle = nil
	}
	out.TerminationMessagePath = in.TerminationMessagePath
	out.ImagePullPolicy = pkgapi.PullPolicy(in.ImagePullPolicy)
	if in.SecurityContext != nil {
		out.SecurityContext = new(pkgapi.SecurityContext)
		if err := convert_v1_SecurityContext_To_api_SecurityContext(in.SecurityContext, out.SecurityContext, s); err != nil {
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

func convert_v1_Container_To_api_Container(in *pkgapiv1.Container, out *pkgapi.Container, s conversion.Scope) error {
	return autoconvert_v1_Container_To_api_Container(in, out, s)
}

func autoconvert_v1_ContainerPort_To_api_ContainerPort(in *pkgapiv1.ContainerPort, out *pkgapi.ContainerPort, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.ContainerPort))(in)
	}
	out.Name = in.Name
	out.HostPort = in.HostPort
	out.ContainerPort = in.ContainerPort
	out.Protocol = pkgapi.Protocol(in.Protocol)
	out.HostIP = in.HostIP
	return nil
}

func convert_v1_ContainerPort_To_api_ContainerPort(in *pkgapiv1.ContainerPort, out *pkgapi.ContainerPort, s conversion.Scope) error {
	return autoconvert_v1_ContainerPort_To_api_ContainerPort(in, out, s)
}

func autoconvert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(in *pkgapiv1.DownwardAPIVolumeFile, out *pkgapi.DownwardAPIVolumeFile, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.DownwardAPIVolumeFile))(in)
	}
	out.Path = in.Path
	if err := convert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(&in.FieldRef, &out.FieldRef, s); err != nil {
		return err
	}
	return nil
}

func convert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(in *pkgapiv1.DownwardAPIVolumeFile, out *pkgapi.DownwardAPIVolumeFile, s conversion.Scope) error {
	return autoconvert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(in, out, s)
}

func autoconvert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in *pkgapiv1.DownwardAPIVolumeSource, out *pkgapi.DownwardAPIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.DownwardAPIVolumeSource))(in)
	}
	if in.Items != nil {
		out.Items = make([]pkgapi.DownwardAPIVolumeFile, len(in.Items))
		for i := range in.Items {
			if err := convert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile(&in.Items[i], &out.Items[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func convert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in *pkgapiv1.DownwardAPIVolumeSource, out *pkgapi.DownwardAPIVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in, out, s)
}

func autoconvert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in *pkgapiv1.EmptyDirVolumeSource, out *pkgapi.EmptyDirVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.EmptyDirVolumeSource))(in)
	}
	out.Medium = pkgapi.StorageMedium(in.Medium)
	return nil
}

func convert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in *pkgapiv1.EmptyDirVolumeSource, out *pkgapi.EmptyDirVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in, out, s)
}

func autoconvert_v1_EnvVar_To_api_EnvVar(in *pkgapiv1.EnvVar, out *pkgapi.EnvVar, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.EnvVar))(in)
	}
	out.Name = in.Name
	out.Value = in.Value
	if in.ValueFrom != nil {
		out.ValueFrom = new(pkgapi.EnvVarSource)
		if err := convert_v1_EnvVarSource_To_api_EnvVarSource(in.ValueFrom, out.ValueFrom, s); err != nil {
			return err
		}
	} else {
		out.ValueFrom = nil
	}
	return nil
}

func convert_v1_EnvVar_To_api_EnvVar(in *pkgapiv1.EnvVar, out *pkgapi.EnvVar, s conversion.Scope) error {
	return autoconvert_v1_EnvVar_To_api_EnvVar(in, out, s)
}

func autoconvert_v1_EnvVarSource_To_api_EnvVarSource(in *pkgapiv1.EnvVarSource, out *pkgapi.EnvVarSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.EnvVarSource))(in)
	}
	if in.FieldRef != nil {
		out.FieldRef = new(pkgapi.ObjectFieldSelector)
		if err := convert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(in.FieldRef, out.FieldRef, s); err != nil {
			return err
		}
	} else {
		out.FieldRef = nil
	}
	return nil
}

func convert_v1_EnvVarSource_To_api_EnvVarSource(in *pkgapiv1.EnvVarSource, out *pkgapi.EnvVarSource, s conversion.Scope) error {
	return autoconvert_v1_EnvVarSource_To_api_EnvVarSource(in, out, s)
}

func autoconvert_v1_ExecAction_To_api_ExecAction(in *pkgapiv1.ExecAction, out *pkgapi.ExecAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.ExecAction))(in)
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

func convert_v1_ExecAction_To_api_ExecAction(in *pkgapiv1.ExecAction, out *pkgapi.ExecAction, s conversion.Scope) error {
	return autoconvert_v1_ExecAction_To_api_ExecAction(in, out, s)
}

func autoconvert_v1_FCVolumeSource_To_api_FCVolumeSource(in *pkgapiv1.FCVolumeSource, out *pkgapi.FCVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.FCVolumeSource))(in)
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

func convert_v1_FCVolumeSource_To_api_FCVolumeSource(in *pkgapiv1.FCVolumeSource, out *pkgapi.FCVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_FCVolumeSource_To_api_FCVolumeSource(in, out, s)
}

func autoconvert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource(in *pkgapiv1.FlockerVolumeSource, out *pkgapi.FlockerVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.FlockerVolumeSource))(in)
	}
	out.DatasetName = in.DatasetName
	return nil
}

func convert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource(in *pkgapiv1.FlockerVolumeSource, out *pkgapi.FlockerVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource(in, out, s)
}

func autoconvert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in *pkgapiv1.GCEPersistentDiskVolumeSource, out *pkgapi.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.GCEPersistentDiskVolumeSource))(in)
	}
	out.PDName = in.PDName
	out.FSType = in.FSType
	out.Partition = in.Partition
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in *pkgapiv1.GCEPersistentDiskVolumeSource, out *pkgapi.GCEPersistentDiskVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in, out, s)
}

func autoconvert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource(in *pkgapiv1.GitRepoVolumeSource, out *pkgapi.GitRepoVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.GitRepoVolumeSource))(in)
	}
	out.Repository = in.Repository
	out.Revision = in.Revision
	return nil
}

func convert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource(in *pkgapiv1.GitRepoVolumeSource, out *pkgapi.GitRepoVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource(in, out, s)
}

func autoconvert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in *pkgapiv1.GlusterfsVolumeSource, out *pkgapi.GlusterfsVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.GlusterfsVolumeSource))(in)
	}
	out.EndpointsName = in.EndpointsName
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in *pkgapiv1.GlusterfsVolumeSource, out *pkgapi.GlusterfsVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in, out, s)
}

func autoconvert_v1_HTTPGetAction_To_api_HTTPGetAction(in *pkgapiv1.HTTPGetAction, out *pkgapi.HTTPGetAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.HTTPGetAction))(in)
	}
	out.Path = in.Path
	if err := s.Convert(&in.Port, &out.Port, 0); err != nil {
		return err
	}
	out.Host = in.Host
	out.Scheme = pkgapi.URIScheme(in.Scheme)
	return nil
}

func convert_v1_HTTPGetAction_To_api_HTTPGetAction(in *pkgapiv1.HTTPGetAction, out *pkgapi.HTTPGetAction, s conversion.Scope) error {
	return autoconvert_v1_HTTPGetAction_To_api_HTTPGetAction(in, out, s)
}

func autoconvert_v1_Handler_To_api_Handler(in *pkgapiv1.Handler, out *pkgapi.Handler, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.Handler))(in)
	}
	if in.Exec != nil {
		out.Exec = new(pkgapi.ExecAction)
		if err := convert_v1_ExecAction_To_api_ExecAction(in.Exec, out.Exec, s); err != nil {
			return err
		}
	} else {
		out.Exec = nil
	}
	if in.HTTPGet != nil {
		out.HTTPGet = new(pkgapi.HTTPGetAction)
		if err := convert_v1_HTTPGetAction_To_api_HTTPGetAction(in.HTTPGet, out.HTTPGet, s); err != nil {
			return err
		}
	} else {
		out.HTTPGet = nil
	}
	if in.TCPSocket != nil {
		out.TCPSocket = new(pkgapi.TCPSocketAction)
		if err := convert_v1_TCPSocketAction_To_api_TCPSocketAction(in.TCPSocket, out.TCPSocket, s); err != nil {
			return err
		}
	} else {
		out.TCPSocket = nil
	}
	return nil
}

func convert_v1_Handler_To_api_Handler(in *pkgapiv1.Handler, out *pkgapi.Handler, s conversion.Scope) error {
	return autoconvert_v1_Handler_To_api_Handler(in, out, s)
}

func autoconvert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource(in *pkgapiv1.HostPathVolumeSource, out *pkgapi.HostPathVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.HostPathVolumeSource))(in)
	}
	out.Path = in.Path
	return nil
}

func convert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource(in *pkgapiv1.HostPathVolumeSource, out *pkgapi.HostPathVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource(in, out, s)
}

func autoconvert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource(in *pkgapiv1.ISCSIVolumeSource, out *pkgapi.ISCSIVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.ISCSIVolumeSource))(in)
	}
	out.TargetPortal = in.TargetPortal
	out.IQN = in.IQN
	out.Lun = in.Lun
	out.FSType = in.FSType
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource(in *pkgapiv1.ISCSIVolumeSource, out *pkgapi.ISCSIVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource(in, out, s)
}

func autoconvert_v1_Lifecycle_To_api_Lifecycle(in *pkgapiv1.Lifecycle, out *pkgapi.Lifecycle, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.Lifecycle))(in)
	}
	if in.PostStart != nil {
		out.PostStart = new(pkgapi.Handler)
		if err := convert_v1_Handler_To_api_Handler(in.PostStart, out.PostStart, s); err != nil {
			return err
		}
	} else {
		out.PostStart = nil
	}
	if in.PreStop != nil {
		out.PreStop = new(pkgapi.Handler)
		if err := convert_v1_Handler_To_api_Handler(in.PreStop, out.PreStop, s); err != nil {
			return err
		}
	} else {
		out.PreStop = nil
	}
	return nil
}

func convert_v1_Lifecycle_To_api_Lifecycle(in *pkgapiv1.Lifecycle, out *pkgapi.Lifecycle, s conversion.Scope) error {
	return autoconvert_v1_Lifecycle_To_api_Lifecycle(in, out, s)
}

func autoconvert_v1_LocalObjectReference_To_api_LocalObjectReference(in *pkgapiv1.LocalObjectReference, out *pkgapi.LocalObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.LocalObjectReference))(in)
	}
	out.Name = in.Name
	return nil
}

func convert_v1_LocalObjectReference_To_api_LocalObjectReference(in *pkgapiv1.LocalObjectReference, out *pkgapi.LocalObjectReference, s conversion.Scope) error {
	return autoconvert_v1_LocalObjectReference_To_api_LocalObjectReference(in, out, s)
}

func autoconvert_v1_NFSVolumeSource_To_api_NFSVolumeSource(in *pkgapiv1.NFSVolumeSource, out *pkgapi.NFSVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.NFSVolumeSource))(in)
	}
	out.Server = in.Server
	out.Path = in.Path
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_v1_NFSVolumeSource_To_api_NFSVolumeSource(in *pkgapiv1.NFSVolumeSource, out *pkgapi.NFSVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_NFSVolumeSource_To_api_NFSVolumeSource(in, out, s)
}

func autoconvert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(in *pkgapiv1.ObjectFieldSelector, out *pkgapi.ObjectFieldSelector, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.ObjectFieldSelector))(in)
	}
	out.APIVersion = in.APIVersion
	out.FieldPath = in.FieldPath
	return nil
}

func convert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(in *pkgapiv1.ObjectFieldSelector, out *pkgapi.ObjectFieldSelector, s conversion.Scope) error {
	return autoconvert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector(in, out, s)
}

func autoconvert_v1_ObjectMeta_To_api_ObjectMeta(in *pkgapiv1.ObjectMeta, out *pkgapi.ObjectMeta, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.ObjectMeta))(in)
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

func convert_v1_ObjectMeta_To_api_ObjectMeta(in *pkgapiv1.ObjectMeta, out *pkgapi.ObjectMeta, s conversion.Scope) error {
	return autoconvert_v1_ObjectMeta_To_api_ObjectMeta(in, out, s)
}

func autoconvert_v1_ObjectReference_To_api_ObjectReference(in *pkgapiv1.ObjectReference, out *pkgapi.ObjectReference, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.ObjectReference))(in)
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

func convert_v1_ObjectReference_To_api_ObjectReference(in *pkgapiv1.ObjectReference, out *pkgapi.ObjectReference, s conversion.Scope) error {
	return autoconvert_v1_ObjectReference_To_api_ObjectReference(in, out, s)
}

func autoconvert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in *pkgapiv1.PersistentVolumeClaimVolumeSource, out *pkgapi.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.PersistentVolumeClaimVolumeSource))(in)
	}
	out.ClaimName = in.ClaimName
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in *pkgapiv1.PersistentVolumeClaimVolumeSource, out *pkgapi.PersistentVolumeClaimVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in, out, s)
}

func autoconvert_v1_PodSpec_To_api_PodSpec(in *pkgapiv1.PodSpec, out *pkgapi.PodSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.PodSpec))(in)
	}
	if in.Volumes != nil {
		out.Volumes = make([]pkgapi.Volume, len(in.Volumes))
		for i := range in.Volumes {
			if err := convert_v1_Volume_To_api_Volume(&in.Volumes[i], &out.Volumes[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Volumes = nil
	}
	if in.Containers != nil {
		out.Containers = make([]pkgapi.Container, len(in.Containers))
		for i := range in.Containers {
			if err := convert_v1_Container_To_api_Container(&in.Containers[i], &out.Containers[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Containers = nil
	}
	out.RestartPolicy = pkgapi.RestartPolicy(in.RestartPolicy)
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
	out.DNSPolicy = pkgapi.DNSPolicy(in.DNSPolicy)
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
	if in.SecurityContext != nil {
		if err := s.Convert(&in.SecurityContext, &out.SecurityContext, 0); err != nil {
			return err
		}
	} else {
		out.SecurityContext = nil
	}
	if in.ImagePullSecrets != nil {
		out.ImagePullSecrets = make([]pkgapi.LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(&in.ImagePullSecrets[i], &out.ImagePullSecrets[i], s); err != nil {
				return err
			}
		}
	} else {
		out.ImagePullSecrets = nil
	}
	return nil
}

func autoconvert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in *pkgapiv1.PodTemplateSpec, out *pkgapi.PodTemplateSpec, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.PodTemplateSpec))(in)
	}
	if err := convert_v1_ObjectMeta_To_api_ObjectMeta(&in.ObjectMeta, &out.ObjectMeta, s); err != nil {
		return err
	}
	if err := s.Convert(&in.Spec, &out.Spec, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in *pkgapiv1.PodTemplateSpec, out *pkgapi.PodTemplateSpec, s conversion.Scope) error {
	return autoconvert_v1_PodTemplateSpec_To_api_PodTemplateSpec(in, out, s)
}

func autoconvert_v1_Probe_To_api_Probe(in *pkgapiv1.Probe, out *pkgapi.Probe, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.Probe))(in)
	}
	if err := convert_v1_Handler_To_api_Handler(&in.Handler, &out.Handler, s); err != nil {
		return err
	}
	out.InitialDelaySeconds = in.InitialDelaySeconds
	out.TimeoutSeconds = in.TimeoutSeconds
	return nil
}

func convert_v1_Probe_To_api_Probe(in *pkgapiv1.Probe, out *pkgapi.Probe, s conversion.Scope) error {
	return autoconvert_v1_Probe_To_api_Probe(in, out, s)
}

func autoconvert_v1_RBDVolumeSource_To_api_RBDVolumeSource(in *pkgapiv1.RBDVolumeSource, out *pkgapi.RBDVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.RBDVolumeSource))(in)
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
	if in.SecretRef != nil {
		out.SecretRef = new(pkgapi.LocalObjectReference)
		if err := convert_v1_LocalObjectReference_To_api_LocalObjectReference(in.SecretRef, out.SecretRef, s); err != nil {
			return err
		}
	} else {
		out.SecretRef = nil
	}
	out.ReadOnly = in.ReadOnly
	return nil
}

func convert_v1_RBDVolumeSource_To_api_RBDVolumeSource(in *pkgapiv1.RBDVolumeSource, out *pkgapi.RBDVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_RBDVolumeSource_To_api_RBDVolumeSource(in, out, s)
}

func autoconvert_v1_ResourceRequirements_To_api_ResourceRequirements(in *pkgapiv1.ResourceRequirements, out *pkgapi.ResourceRequirements, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.ResourceRequirements))(in)
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

func convert_v1_ResourceRequirements_To_api_ResourceRequirements(in *pkgapiv1.ResourceRequirements, out *pkgapi.ResourceRequirements, s conversion.Scope) error {
	return autoconvert_v1_ResourceRequirements_To_api_ResourceRequirements(in, out, s)
}

func autoconvert_v1_SELinuxOptions_To_api_SELinuxOptions(in *pkgapiv1.SELinuxOptions, out *pkgapi.SELinuxOptions, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.SELinuxOptions))(in)
	}
	out.User = in.User
	out.Role = in.Role
	out.Type = in.Type
	out.Level = in.Level
	return nil
}

func convert_v1_SELinuxOptions_To_api_SELinuxOptions(in *pkgapiv1.SELinuxOptions, out *pkgapi.SELinuxOptions, s conversion.Scope) error {
	return autoconvert_v1_SELinuxOptions_To_api_SELinuxOptions(in, out, s)
}

func autoconvert_v1_SecretVolumeSource_To_api_SecretVolumeSource(in *pkgapiv1.SecretVolumeSource, out *pkgapi.SecretVolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.SecretVolumeSource))(in)
	}
	out.SecretName = in.SecretName
	return nil
}

func convert_v1_SecretVolumeSource_To_api_SecretVolumeSource(in *pkgapiv1.SecretVolumeSource, out *pkgapi.SecretVolumeSource, s conversion.Scope) error {
	return autoconvert_v1_SecretVolumeSource_To_api_SecretVolumeSource(in, out, s)
}

func autoconvert_v1_SecurityContext_To_api_SecurityContext(in *pkgapiv1.SecurityContext, out *pkgapi.SecurityContext, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.SecurityContext))(in)
	}
	if in.Capabilities != nil {
		out.Capabilities = new(pkgapi.Capabilities)
		if err := convert_v1_Capabilities_To_api_Capabilities(in.Capabilities, out.Capabilities, s); err != nil {
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
	if in.SELinuxOptions != nil {
		out.SELinuxOptions = new(pkgapi.SELinuxOptions)
		if err := convert_v1_SELinuxOptions_To_api_SELinuxOptions(in.SELinuxOptions, out.SELinuxOptions, s); err != nil {
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
	return nil
}

func convert_v1_SecurityContext_To_api_SecurityContext(in *pkgapiv1.SecurityContext, out *pkgapi.SecurityContext, s conversion.Scope) error {
	return autoconvert_v1_SecurityContext_To_api_SecurityContext(in, out, s)
}

func autoconvert_v1_TCPSocketAction_To_api_TCPSocketAction(in *pkgapiv1.TCPSocketAction, out *pkgapi.TCPSocketAction, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.TCPSocketAction))(in)
	}
	if err := s.Convert(&in.Port, &out.Port, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_TCPSocketAction_To_api_TCPSocketAction(in *pkgapiv1.TCPSocketAction, out *pkgapi.TCPSocketAction, s conversion.Scope) error {
	return autoconvert_v1_TCPSocketAction_To_api_TCPSocketAction(in, out, s)
}

func autoconvert_v1_Volume_To_api_Volume(in *pkgapiv1.Volume, out *pkgapi.Volume, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.Volume))(in)
	}
	out.Name = in.Name
	if err := s.Convert(&in.VolumeSource, &out.VolumeSource, 0); err != nil {
		return err
	}
	return nil
}

func convert_v1_Volume_To_api_Volume(in *pkgapiv1.Volume, out *pkgapi.Volume, s conversion.Scope) error {
	return autoconvert_v1_Volume_To_api_Volume(in, out, s)
}

func autoconvert_v1_VolumeMount_To_api_VolumeMount(in *pkgapiv1.VolumeMount, out *pkgapi.VolumeMount, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.VolumeMount))(in)
	}
	out.Name = in.Name
	out.ReadOnly = in.ReadOnly
	out.MountPath = in.MountPath
	return nil
}

func convert_v1_VolumeMount_To_api_VolumeMount(in *pkgapiv1.VolumeMount, out *pkgapi.VolumeMount, s conversion.Scope) error {
	return autoconvert_v1_VolumeMount_To_api_VolumeMount(in, out, s)
}

func autoconvert_v1_VolumeSource_To_api_VolumeSource(in *pkgapiv1.VolumeSource, out *pkgapi.VolumeSource, s conversion.Scope) error {
	if defaulting, found := s.DefaultingInterface(reflect.TypeOf(*in)); found {
		defaulting.(func(*pkgapiv1.VolumeSource))(in)
	}
	if in.HostPath != nil {
		out.HostPath = new(pkgapi.HostPathVolumeSource)
		if err := convert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource(in.HostPath, out.HostPath, s); err != nil {
			return err
		}
	} else {
		out.HostPath = nil
	}
	if in.EmptyDir != nil {
		out.EmptyDir = new(pkgapi.EmptyDirVolumeSource)
		if err := convert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource(in.EmptyDir, out.EmptyDir, s); err != nil {
			return err
		}
	} else {
		out.EmptyDir = nil
	}
	if in.GCEPersistentDisk != nil {
		out.GCEPersistentDisk = new(pkgapi.GCEPersistentDiskVolumeSource)
		if err := convert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource(in.GCEPersistentDisk, out.GCEPersistentDisk, s); err != nil {
			return err
		}
	} else {
		out.GCEPersistentDisk = nil
	}
	if in.AWSElasticBlockStore != nil {
		out.AWSElasticBlockStore = new(pkgapi.AWSElasticBlockStoreVolumeSource)
		if err := convert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource(in.AWSElasticBlockStore, out.AWSElasticBlockStore, s); err != nil {
			return err
		}
	} else {
		out.AWSElasticBlockStore = nil
	}
	if in.GitRepo != nil {
		out.GitRepo = new(pkgapi.GitRepoVolumeSource)
		if err := convert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource(in.GitRepo, out.GitRepo, s); err != nil {
			return err
		}
	} else {
		out.GitRepo = nil
	}
	if in.Secret != nil {
		out.Secret = new(pkgapi.SecretVolumeSource)
		if err := convert_v1_SecretVolumeSource_To_api_SecretVolumeSource(in.Secret, out.Secret, s); err != nil {
			return err
		}
	} else {
		out.Secret = nil
	}
	if in.NFS != nil {
		out.NFS = new(pkgapi.NFSVolumeSource)
		if err := convert_v1_NFSVolumeSource_To_api_NFSVolumeSource(in.NFS, out.NFS, s); err != nil {
			return err
		}
	} else {
		out.NFS = nil
	}
	if in.ISCSI != nil {
		out.ISCSI = new(pkgapi.ISCSIVolumeSource)
		if err := convert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource(in.ISCSI, out.ISCSI, s); err != nil {
			return err
		}
	} else {
		out.ISCSI = nil
	}
	if in.Glusterfs != nil {
		out.Glusterfs = new(pkgapi.GlusterfsVolumeSource)
		if err := convert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource(in.Glusterfs, out.Glusterfs, s); err != nil {
			return err
		}
	} else {
		out.Glusterfs = nil
	}
	if in.PersistentVolumeClaim != nil {
		out.PersistentVolumeClaim = new(pkgapi.PersistentVolumeClaimVolumeSource)
		if err := convert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource(in.PersistentVolumeClaim, out.PersistentVolumeClaim, s); err != nil {
			return err
		}
	} else {
		out.PersistentVolumeClaim = nil
	}
	if in.RBD != nil {
		out.RBD = new(pkgapi.RBDVolumeSource)
		if err := convert_v1_RBDVolumeSource_To_api_RBDVolumeSource(in.RBD, out.RBD, s); err != nil {
			return err
		}
	} else {
		out.RBD = nil
	}
	if in.Cinder != nil {
		out.Cinder = new(pkgapi.CinderVolumeSource)
		if err := convert_v1_CinderVolumeSource_To_api_CinderVolumeSource(in.Cinder, out.Cinder, s); err != nil {
			return err
		}
	} else {
		out.Cinder = nil
	}
	if in.CephFS != nil {
		out.CephFS = new(pkgapi.CephFSVolumeSource)
		if err := convert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource(in.CephFS, out.CephFS, s); err != nil {
			return err
		}
	} else {
		out.CephFS = nil
	}
	if in.Flocker != nil {
		out.Flocker = new(pkgapi.FlockerVolumeSource)
		if err := convert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource(in.Flocker, out.Flocker, s); err != nil {
			return err
		}
	} else {
		out.Flocker = nil
	}
	if in.DownwardAPI != nil {
		out.DownwardAPI = new(pkgapi.DownwardAPIVolumeSource)
		if err := convert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource(in.DownwardAPI, out.DownwardAPI, s); err != nil {
			return err
		}
	} else {
		out.DownwardAPI = nil
	}
	if in.FC != nil {
		out.FC = new(pkgapi.FCVolumeSource)
		if err := convert_v1_FCVolumeSource_To_api_FCVolumeSource(in.FC, out.FC, s); err != nil {
			return err
		}
	} else {
		out.FC = nil
	}
	// in.Metadata has no peer in out
	return nil
}

func init() {
	err := pkgapi.Scheme.AddGeneratedConversionFuncs(
		autoconvert_api_AWSElasticBlockStoreVolumeSource_To_v1_AWSElasticBlockStoreVolumeSource,
		autoconvert_api_BinaryBuildRequestOptions_To_v1_BinaryBuildRequestOptions,
		autoconvert_api_BinaryBuildSource_To_v1_BinaryBuildSource,
		autoconvert_api_BuildConfigList_To_v1_BuildConfigList,
		autoconvert_api_BuildConfigSpec_To_v1_BuildConfigSpec,
		autoconvert_api_BuildConfigStatus_To_v1_BuildConfigStatus,
		autoconvert_api_BuildConfig_To_v1_BuildConfig,
		autoconvert_api_BuildList_To_v1_BuildList,
		autoconvert_api_BuildLogOptions_To_v1_BuildLogOptions,
		autoconvert_api_BuildLog_To_v1_BuildLog,
		autoconvert_api_BuildOutput_To_v1_BuildOutput,
		autoconvert_api_BuildRequest_To_v1_BuildRequest,
		autoconvert_api_BuildSource_To_v1_BuildSource,
		autoconvert_api_BuildSpec_To_v1_BuildSpec,
		autoconvert_api_BuildStatus_To_v1_BuildStatus,
		autoconvert_api_BuildStrategy_To_v1_BuildStrategy,
		autoconvert_api_BuildTriggerPolicy_To_v1_BuildTriggerPolicy,
		autoconvert_api_Build_To_v1_Build,
		autoconvert_api_Capabilities_To_v1_Capabilities,
		autoconvert_api_CephFSVolumeSource_To_v1_CephFSVolumeSource,
		autoconvert_api_CinderVolumeSource_To_v1_CinderVolumeSource,
		autoconvert_api_ClusterNetworkList_To_v1_ClusterNetworkList,
		autoconvert_api_ClusterNetwork_To_v1_ClusterNetwork,
		autoconvert_api_ClusterPolicyBindingList_To_v1_ClusterPolicyBindingList,
		autoconvert_api_ClusterPolicyBinding_To_v1_ClusterPolicyBinding,
		autoconvert_api_ClusterPolicyList_To_v1_ClusterPolicyList,
		autoconvert_api_ClusterPolicy_To_v1_ClusterPolicy,
		autoconvert_api_ClusterRoleBindingList_To_v1_ClusterRoleBindingList,
		autoconvert_api_ClusterRoleBinding_To_v1_ClusterRoleBinding,
		autoconvert_api_ClusterRoleList_To_v1_ClusterRoleList,
		autoconvert_api_ClusterRole_To_v1_ClusterRole,
		autoconvert_api_ContainerPort_To_v1_ContainerPort,
		autoconvert_api_Container_To_v1_Container,
		autoconvert_api_CustomBuildStrategy_To_v1_CustomBuildStrategy,
		autoconvert_api_CustomDeploymentStrategyParams_To_v1_CustomDeploymentStrategyParams,
		autoconvert_api_DeploymentCauseImageTrigger_To_v1_DeploymentCauseImageTrigger,
		autoconvert_api_DeploymentCause_To_v1_DeploymentCause,
		autoconvert_api_DeploymentConfigList_To_v1_DeploymentConfigList,
		autoconvert_api_DeploymentConfigRollbackSpec_To_v1_DeploymentConfigRollbackSpec,
		autoconvert_api_DeploymentConfigRollback_To_v1_DeploymentConfigRollback,
		autoconvert_api_DeploymentConfigSpec_To_v1_DeploymentConfigSpec,
		autoconvert_api_DeploymentConfigStatus_To_v1_DeploymentConfigStatus,
		autoconvert_api_DeploymentConfig_To_v1_DeploymentConfig,
		autoconvert_api_DeploymentDetails_To_v1_DeploymentDetails,
		autoconvert_api_DeploymentLogOptions_To_v1_DeploymentLogOptions,
		autoconvert_api_DeploymentLog_To_v1_DeploymentLog,
		autoconvert_api_DeploymentStrategy_To_v1_DeploymentStrategy,
		autoconvert_api_DeploymentTriggerImageChangeParams_To_v1_DeploymentTriggerImageChangeParams,
		autoconvert_api_DeploymentTriggerPolicy_To_v1_DeploymentTriggerPolicy,
		autoconvert_api_DockerBuildStrategy_To_v1_DockerBuildStrategy,
		autoconvert_api_DownwardAPIVolumeFile_To_v1_DownwardAPIVolumeFile,
		autoconvert_api_DownwardAPIVolumeSource_To_v1_DownwardAPIVolumeSource,
		autoconvert_api_EmptyDirVolumeSource_To_v1_EmptyDirVolumeSource,
		autoconvert_api_EnvVarSource_To_v1_EnvVarSource,
		autoconvert_api_EnvVar_To_v1_EnvVar,
		autoconvert_api_ExecAction_To_v1_ExecAction,
		autoconvert_api_ExecNewPodHook_To_v1_ExecNewPodHook,
		autoconvert_api_FCVolumeSource_To_v1_FCVolumeSource,
		autoconvert_api_FlockerVolumeSource_To_v1_FlockerVolumeSource,
		autoconvert_api_GCEPersistentDiskVolumeSource_To_v1_GCEPersistentDiskVolumeSource,
		autoconvert_api_GitBuildSource_To_v1_GitBuildSource,
		autoconvert_api_GitRepoVolumeSource_To_v1_GitRepoVolumeSource,
		autoconvert_api_GitSourceRevision_To_v1_GitSourceRevision,
		autoconvert_api_GlusterfsVolumeSource_To_v1_GlusterfsVolumeSource,
		autoconvert_api_GroupList_To_v1_GroupList,
		autoconvert_api_Group_To_v1_Group,
		autoconvert_api_HTTPGetAction_To_v1_HTTPGetAction,
		autoconvert_api_Handler_To_v1_Handler,
		autoconvert_api_HostPathVolumeSource_To_v1_HostPathVolumeSource,
		autoconvert_api_HostSubnetList_To_v1_HostSubnetList,
		autoconvert_api_HostSubnet_To_v1_HostSubnet,
		autoconvert_api_ISCSIVolumeSource_To_v1_ISCSIVolumeSource,
		autoconvert_api_IdentityList_To_v1_IdentityList,
		autoconvert_api_Identity_To_v1_Identity,
		autoconvert_api_ImageChangeTrigger_To_v1_ImageChangeTrigger,
		autoconvert_api_ImageList_To_v1_ImageList,
		autoconvert_api_ImageSourcePath_To_v1_ImageSourcePath,
		autoconvert_api_ImageSource_To_v1_ImageSource,
		autoconvert_api_ImageStreamImage_To_v1_ImageStreamImage,
		autoconvert_api_ImageStreamList_To_v1_ImageStreamList,
		autoconvert_api_ImageStreamMapping_To_v1_ImageStreamMapping,
		autoconvert_api_ImageStreamSpec_To_v1_ImageStreamSpec,
		autoconvert_api_ImageStreamStatus_To_v1_ImageStreamStatus,
		autoconvert_api_ImageStreamTagList_To_v1_ImageStreamTagList,
		autoconvert_api_ImageStreamTag_To_v1_ImageStreamTag,
		autoconvert_api_ImageStream_To_v1_ImageStream,
		autoconvert_api_Image_To_v1_Image,
		autoconvert_api_IsPersonalSubjectAccessReview_To_v1_IsPersonalSubjectAccessReview,
		autoconvert_api_LifecycleHook_To_v1_LifecycleHook,
		autoconvert_api_Lifecycle_To_v1_Lifecycle,
		autoconvert_api_LocalObjectReference_To_v1_LocalObjectReference,
		autoconvert_api_LocalResourceAccessReview_To_v1_LocalResourceAccessReview,
		autoconvert_api_LocalSubjectAccessReview_To_v1_LocalSubjectAccessReview,
		autoconvert_api_NFSVolumeSource_To_v1_NFSVolumeSource,
		autoconvert_api_NetNamespaceList_To_v1_NetNamespaceList,
		autoconvert_api_NetNamespace_To_v1_NetNamespace,
		autoconvert_api_OAuthAccessTokenList_To_v1_OAuthAccessTokenList,
		autoconvert_api_OAuthAccessToken_To_v1_OAuthAccessToken,
		autoconvert_api_OAuthAuthorizeTokenList_To_v1_OAuthAuthorizeTokenList,
		autoconvert_api_OAuthAuthorizeToken_To_v1_OAuthAuthorizeToken,
		autoconvert_api_OAuthClientAuthorizationList_To_v1_OAuthClientAuthorizationList,
		autoconvert_api_OAuthClientAuthorization_To_v1_OAuthClientAuthorization,
		autoconvert_api_OAuthClientList_To_v1_OAuthClientList,
		autoconvert_api_OAuthClient_To_v1_OAuthClient,
		autoconvert_api_ObjectFieldSelector_To_v1_ObjectFieldSelector,
		autoconvert_api_ObjectMeta_To_v1_ObjectMeta,
		autoconvert_api_ObjectReference_To_v1_ObjectReference,
		autoconvert_api_Parameter_To_v1_Parameter,
		autoconvert_api_PersistentVolumeClaimVolumeSource_To_v1_PersistentVolumeClaimVolumeSource,
		autoconvert_api_PodSpec_To_v1_PodSpec,
		autoconvert_api_PodTemplateSpec_To_v1_PodTemplateSpec,
		autoconvert_api_PolicyBindingList_To_v1_PolicyBindingList,
		autoconvert_api_PolicyBinding_To_v1_PolicyBinding,
		autoconvert_api_PolicyList_To_v1_PolicyList,
		autoconvert_api_PolicyRule_To_v1_PolicyRule,
		autoconvert_api_Policy_To_v1_Policy,
		autoconvert_api_Probe_To_v1_Probe,
		autoconvert_api_ProjectList_To_v1_ProjectList,
		autoconvert_api_ProjectRequest_To_v1_ProjectRequest,
		autoconvert_api_ProjectSpec_To_v1_ProjectSpec,
		autoconvert_api_ProjectStatus_To_v1_ProjectStatus,
		autoconvert_api_Project_To_v1_Project,
		autoconvert_api_RBDVolumeSource_To_v1_RBDVolumeSource,
		autoconvert_api_RecreateDeploymentStrategyParams_To_v1_RecreateDeploymentStrategyParams,
		autoconvert_api_ResourceAccessReviewResponse_To_v1_ResourceAccessReviewResponse,
		autoconvert_api_ResourceAccessReview_To_v1_ResourceAccessReview,
		autoconvert_api_ResourceRequirements_To_v1_ResourceRequirements,
		autoconvert_api_RoleBindingList_To_v1_RoleBindingList,
		autoconvert_api_RoleBinding_To_v1_RoleBinding,
		autoconvert_api_RoleList_To_v1_RoleList,
		autoconvert_api_Role_To_v1_Role,
		autoconvert_api_RollingDeploymentStrategyParams_To_v1_RollingDeploymentStrategyParams,
		autoconvert_api_RouteList_To_v1_RouteList,
		autoconvert_api_RoutePort_To_v1_RoutePort,
		autoconvert_api_RouteSpec_To_v1_RouteSpec,
		autoconvert_api_RouteStatus_To_v1_RouteStatus,
		autoconvert_api_Route_To_v1_Route,
		autoconvert_api_SELinuxOptions_To_v1_SELinuxOptions,
		autoconvert_api_SecretSpec_To_v1_SecretSpec,
		autoconvert_api_SecretVolumeSource_To_v1_SecretVolumeSource,
		autoconvert_api_SecurityContext_To_v1_SecurityContext,
		autoconvert_api_SourceBuildStrategy_To_v1_SourceBuildStrategy,
		autoconvert_api_SourceControlUser_To_v1_SourceControlUser,
		autoconvert_api_SourceRevision_To_v1_SourceRevision,
		autoconvert_api_SubjectAccessReviewResponse_To_v1_SubjectAccessReviewResponse,
		autoconvert_api_SubjectAccessReview_To_v1_SubjectAccessReview,
		autoconvert_api_TCPSocketAction_To_v1_TCPSocketAction,
		autoconvert_api_TLSConfig_To_v1_TLSConfig,
		autoconvert_api_TemplateList_To_v1_TemplateList,
		autoconvert_api_Template_To_v1_Template,
		autoconvert_api_UserIdentityMapping_To_v1_UserIdentityMapping,
		autoconvert_api_UserList_To_v1_UserList,
		autoconvert_api_User_To_v1_User,
		autoconvert_api_VolumeMount_To_v1_VolumeMount,
		autoconvert_api_VolumeSource_To_v1_VolumeSource,
		autoconvert_api_Volume_To_v1_Volume,
		autoconvert_api_WebHookTrigger_To_v1_WebHookTrigger,
		autoconvert_v1_AWSElasticBlockStoreVolumeSource_To_api_AWSElasticBlockStoreVolumeSource,
		autoconvert_v1_BinaryBuildRequestOptions_To_api_BinaryBuildRequestOptions,
		autoconvert_v1_BinaryBuildSource_To_api_BinaryBuildSource,
		autoconvert_v1_BuildConfigList_To_api_BuildConfigList,
		autoconvert_v1_BuildConfigSpec_To_api_BuildConfigSpec,
		autoconvert_v1_BuildConfigStatus_To_api_BuildConfigStatus,
		autoconvert_v1_BuildConfig_To_api_BuildConfig,
		autoconvert_v1_BuildList_To_api_BuildList,
		autoconvert_v1_BuildLogOptions_To_api_BuildLogOptions,
		autoconvert_v1_BuildLog_To_api_BuildLog,
		autoconvert_v1_BuildOutput_To_api_BuildOutput,
		autoconvert_v1_BuildRequest_To_api_BuildRequest,
		autoconvert_v1_BuildSource_To_api_BuildSource,
		autoconvert_v1_BuildSpec_To_api_BuildSpec,
		autoconvert_v1_BuildStatus_To_api_BuildStatus,
		autoconvert_v1_BuildStrategy_To_api_BuildStrategy,
		autoconvert_v1_BuildTriggerPolicy_To_api_BuildTriggerPolicy,
		autoconvert_v1_Build_To_api_Build,
		autoconvert_v1_Capabilities_To_api_Capabilities,
		autoconvert_v1_CephFSVolumeSource_To_api_CephFSVolumeSource,
		autoconvert_v1_CinderVolumeSource_To_api_CinderVolumeSource,
		autoconvert_v1_ClusterNetworkList_To_api_ClusterNetworkList,
		autoconvert_v1_ClusterNetwork_To_api_ClusterNetwork,
		autoconvert_v1_ClusterPolicyBindingList_To_api_ClusterPolicyBindingList,
		autoconvert_v1_ClusterPolicyBinding_To_api_ClusterPolicyBinding,
		autoconvert_v1_ClusterPolicyList_To_api_ClusterPolicyList,
		autoconvert_v1_ClusterPolicy_To_api_ClusterPolicy,
		autoconvert_v1_ClusterRoleBindingList_To_api_ClusterRoleBindingList,
		autoconvert_v1_ClusterRoleBinding_To_api_ClusterRoleBinding,
		autoconvert_v1_ClusterRoleList_To_api_ClusterRoleList,
		autoconvert_v1_ClusterRole_To_api_ClusterRole,
		autoconvert_v1_ContainerPort_To_api_ContainerPort,
		autoconvert_v1_Container_To_api_Container,
		autoconvert_v1_CustomBuildStrategy_To_api_CustomBuildStrategy,
		autoconvert_v1_CustomDeploymentStrategyParams_To_api_CustomDeploymentStrategyParams,
		autoconvert_v1_DeploymentCauseImageTrigger_To_api_DeploymentCauseImageTrigger,
		autoconvert_v1_DeploymentCause_To_api_DeploymentCause,
		autoconvert_v1_DeploymentConfigList_To_api_DeploymentConfigList,
		autoconvert_v1_DeploymentConfigRollbackSpec_To_api_DeploymentConfigRollbackSpec,
		autoconvert_v1_DeploymentConfigRollback_To_api_DeploymentConfigRollback,
		autoconvert_v1_DeploymentConfigSpec_To_api_DeploymentConfigSpec,
		autoconvert_v1_DeploymentConfigStatus_To_api_DeploymentConfigStatus,
		autoconvert_v1_DeploymentConfig_To_api_DeploymentConfig,
		autoconvert_v1_DeploymentDetails_To_api_DeploymentDetails,
		autoconvert_v1_DeploymentLogOptions_To_api_DeploymentLogOptions,
		autoconvert_v1_DeploymentLog_To_api_DeploymentLog,
		autoconvert_v1_DeploymentStrategy_To_api_DeploymentStrategy,
		autoconvert_v1_DeploymentTriggerImageChangeParams_To_api_DeploymentTriggerImageChangeParams,
		autoconvert_v1_DeploymentTriggerPolicy_To_api_DeploymentTriggerPolicy,
		autoconvert_v1_DockerBuildStrategy_To_api_DockerBuildStrategy,
		autoconvert_v1_DownwardAPIVolumeFile_To_api_DownwardAPIVolumeFile,
		autoconvert_v1_DownwardAPIVolumeSource_To_api_DownwardAPIVolumeSource,
		autoconvert_v1_EmptyDirVolumeSource_To_api_EmptyDirVolumeSource,
		autoconvert_v1_EnvVarSource_To_api_EnvVarSource,
		autoconvert_v1_EnvVar_To_api_EnvVar,
		autoconvert_v1_ExecAction_To_api_ExecAction,
		autoconvert_v1_ExecNewPodHook_To_api_ExecNewPodHook,
		autoconvert_v1_FCVolumeSource_To_api_FCVolumeSource,
		autoconvert_v1_FlockerVolumeSource_To_api_FlockerVolumeSource,
		autoconvert_v1_GCEPersistentDiskVolumeSource_To_api_GCEPersistentDiskVolumeSource,
		autoconvert_v1_GitBuildSource_To_api_GitBuildSource,
		autoconvert_v1_GitRepoVolumeSource_To_api_GitRepoVolumeSource,
		autoconvert_v1_GitSourceRevision_To_api_GitSourceRevision,
		autoconvert_v1_GlusterfsVolumeSource_To_api_GlusterfsVolumeSource,
		autoconvert_v1_GroupList_To_api_GroupList,
		autoconvert_v1_Group_To_api_Group,
		autoconvert_v1_HTTPGetAction_To_api_HTTPGetAction,
		autoconvert_v1_Handler_To_api_Handler,
		autoconvert_v1_HostPathVolumeSource_To_api_HostPathVolumeSource,
		autoconvert_v1_HostSubnetList_To_api_HostSubnetList,
		autoconvert_v1_HostSubnet_To_api_HostSubnet,
		autoconvert_v1_ISCSIVolumeSource_To_api_ISCSIVolumeSource,
		autoconvert_v1_IdentityList_To_api_IdentityList,
		autoconvert_v1_Identity_To_api_Identity,
		autoconvert_v1_ImageChangeTrigger_To_api_ImageChangeTrigger,
		autoconvert_v1_ImageList_To_api_ImageList,
		autoconvert_v1_ImageSourcePath_To_api_ImageSourcePath,
		autoconvert_v1_ImageSource_To_api_ImageSource,
		autoconvert_v1_ImageStreamImage_To_api_ImageStreamImage,
		autoconvert_v1_ImageStreamList_To_api_ImageStreamList,
		autoconvert_v1_ImageStreamMapping_To_api_ImageStreamMapping,
		autoconvert_v1_ImageStreamSpec_To_api_ImageStreamSpec,
		autoconvert_v1_ImageStreamStatus_To_api_ImageStreamStatus,
		autoconvert_v1_ImageStreamTagList_To_api_ImageStreamTagList,
		autoconvert_v1_ImageStreamTag_To_api_ImageStreamTag,
		autoconvert_v1_ImageStream_To_api_ImageStream,
		autoconvert_v1_Image_To_api_Image,
		autoconvert_v1_IsPersonalSubjectAccessReview_To_api_IsPersonalSubjectAccessReview,
		autoconvert_v1_LifecycleHook_To_api_LifecycleHook,
		autoconvert_v1_Lifecycle_To_api_Lifecycle,
		autoconvert_v1_LocalObjectReference_To_api_LocalObjectReference,
		autoconvert_v1_LocalResourceAccessReview_To_api_LocalResourceAccessReview,
		autoconvert_v1_LocalSubjectAccessReview_To_api_LocalSubjectAccessReview,
		autoconvert_v1_NFSVolumeSource_To_api_NFSVolumeSource,
		autoconvert_v1_NetNamespaceList_To_api_NetNamespaceList,
		autoconvert_v1_NetNamespace_To_api_NetNamespace,
		autoconvert_v1_OAuthAccessTokenList_To_api_OAuthAccessTokenList,
		autoconvert_v1_OAuthAccessToken_To_api_OAuthAccessToken,
		autoconvert_v1_OAuthAuthorizeTokenList_To_api_OAuthAuthorizeTokenList,
		autoconvert_v1_OAuthAuthorizeToken_To_api_OAuthAuthorizeToken,
		autoconvert_v1_OAuthClientAuthorizationList_To_api_OAuthClientAuthorizationList,
		autoconvert_v1_OAuthClientAuthorization_To_api_OAuthClientAuthorization,
		autoconvert_v1_OAuthClientList_To_api_OAuthClientList,
		autoconvert_v1_OAuthClient_To_api_OAuthClient,
		autoconvert_v1_ObjectFieldSelector_To_api_ObjectFieldSelector,
		autoconvert_v1_ObjectMeta_To_api_ObjectMeta,
		autoconvert_v1_ObjectReference_To_api_ObjectReference,
		autoconvert_v1_Parameter_To_api_Parameter,
		autoconvert_v1_PersistentVolumeClaimVolumeSource_To_api_PersistentVolumeClaimVolumeSource,
		autoconvert_v1_PodSpec_To_api_PodSpec,
		autoconvert_v1_PodTemplateSpec_To_api_PodTemplateSpec,
		autoconvert_v1_PolicyBindingList_To_api_PolicyBindingList,
		autoconvert_v1_PolicyBinding_To_api_PolicyBinding,
		autoconvert_v1_PolicyList_To_api_PolicyList,
		autoconvert_v1_PolicyRule_To_api_PolicyRule,
		autoconvert_v1_Policy_To_api_Policy,
		autoconvert_v1_Probe_To_api_Probe,
		autoconvert_v1_ProjectList_To_api_ProjectList,
		autoconvert_v1_ProjectRequest_To_api_ProjectRequest,
		autoconvert_v1_ProjectSpec_To_api_ProjectSpec,
		autoconvert_v1_ProjectStatus_To_api_ProjectStatus,
		autoconvert_v1_Project_To_api_Project,
		autoconvert_v1_RBDVolumeSource_To_api_RBDVolumeSource,
		autoconvert_v1_RecreateDeploymentStrategyParams_To_api_RecreateDeploymentStrategyParams,
		autoconvert_v1_ResourceAccessReviewResponse_To_api_ResourceAccessReviewResponse,
		autoconvert_v1_ResourceAccessReview_To_api_ResourceAccessReview,
		autoconvert_v1_ResourceRequirements_To_api_ResourceRequirements,
		autoconvert_v1_RoleBindingList_To_api_RoleBindingList,
		autoconvert_v1_RoleBinding_To_api_RoleBinding,
		autoconvert_v1_RoleList_To_api_RoleList,
		autoconvert_v1_Role_To_api_Role,
		autoconvert_v1_RollingDeploymentStrategyParams_To_api_RollingDeploymentStrategyParams,
		autoconvert_v1_RouteList_To_api_RouteList,
		autoconvert_v1_RoutePort_To_api_RoutePort,
		autoconvert_v1_RouteSpec_To_api_RouteSpec,
		autoconvert_v1_RouteStatus_To_api_RouteStatus,
		autoconvert_v1_Route_To_api_Route,
		autoconvert_v1_SELinuxOptions_To_api_SELinuxOptions,
		autoconvert_v1_SecretSpec_To_api_SecretSpec,
		autoconvert_v1_SecretVolumeSource_To_api_SecretVolumeSource,
		autoconvert_v1_SecurityContext_To_api_SecurityContext,
		autoconvert_v1_SourceBuildStrategy_To_api_SourceBuildStrategy,
		autoconvert_v1_SourceControlUser_To_api_SourceControlUser,
		autoconvert_v1_SourceRevision_To_api_SourceRevision,
		autoconvert_v1_SubjectAccessReviewResponse_To_api_SubjectAccessReviewResponse,
		autoconvert_v1_SubjectAccessReview_To_api_SubjectAccessReview,
		autoconvert_v1_TCPSocketAction_To_api_TCPSocketAction,
		autoconvert_v1_TLSConfig_To_api_TLSConfig,
		autoconvert_v1_TemplateList_To_api_TemplateList,
		autoconvert_v1_Template_To_api_Template,
		autoconvert_v1_UserIdentityMapping_To_api_UserIdentityMapping,
		autoconvert_v1_UserList_To_api_UserList,
		autoconvert_v1_User_To_api_User,
		autoconvert_v1_VolumeMount_To_api_VolumeMount,
		autoconvert_v1_VolumeSource_To_api_VolumeSource,
		autoconvert_v1_Volume_To_api_Volume,
		autoconvert_v1_WebHookTrigger_To_api_WebHookTrigger,
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}

// AUTO-GENERATED FUNCTIONS END HERE
