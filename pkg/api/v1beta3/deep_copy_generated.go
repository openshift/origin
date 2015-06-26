package v1beta3

// AUTO-GENERATED FUNCTIONS START HERE
import (
	api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	v1beta3 "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
	conversion "github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	runtime "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	util "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	apiv1beta3 "github.com/openshift/origin/pkg/authorization/api/v1beta3"
	buildapiv1beta3 "github.com/openshift/origin/pkg/build/api/v1beta3"
	deployapiv1beta3 "github.com/openshift/origin/pkg/deploy/api/v1beta3"
	imageapiv1beta3 "github.com/openshift/origin/pkg/image/api/v1beta3"
	oauthapiv1beta3 "github.com/openshift/origin/pkg/oauth/api/v1beta3"
	projectapiv1beta3 "github.com/openshift/origin/pkg/project/api/v1beta3"
	routeapiv1beta3 "github.com/openshift/origin/pkg/route/api/v1beta3"
	sdnapiv1beta3 "github.com/openshift/origin/pkg/sdn/api/v1beta3"
	templateapiv1beta3 "github.com/openshift/origin/pkg/template/api/v1beta3"
	userapiv1beta3 "github.com/openshift/origin/pkg/user/api/v1beta3"
)

func deepCopy_v1beta3_ClusterPolicy(in apiv1beta3.ClusterPolicy, out *apiv1beta3.ClusterPolicy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if in.Roles != nil {
		out.Roles = make([]apiv1beta3.NamedClusterRole, len(in.Roles))
		for i := range in.Roles {
			if err := deepCopy_v1beta3_NamedClusterRole(in.Roles[i], &out.Roles[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Roles = nil
	}
	return nil
}

func deepCopy_v1beta3_ClusterPolicyBinding(in apiv1beta3.ClusterPolicyBinding, out *apiv1beta3.ClusterPolicyBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if newVal, err := c.DeepCopy(in.PolicyRef); err != nil {
		return err
	} else {
		out.PolicyRef = newVal.(v1beta3.ObjectReference)
	}
	if in.RoleBindings != nil {
		out.RoleBindings = make([]apiv1beta3.NamedClusterRoleBinding, len(in.RoleBindings))
		for i := range in.RoleBindings {
			if err := deepCopy_v1beta3_NamedClusterRoleBinding(in.RoleBindings[i], &out.RoleBindings[i], c); err != nil {
				return err
			}
		}
	} else {
		out.RoleBindings = nil
	}
	return nil
}

func deepCopy_v1beta3_ClusterPolicyBindingList(in apiv1beta3.ClusterPolicyBindingList, out *apiv1beta3.ClusterPolicyBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.ClusterPolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_ClusterPolicyBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_ClusterPolicyList(in apiv1beta3.ClusterPolicyList, out *apiv1beta3.ClusterPolicyList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.ClusterPolicy, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_ClusterPolicy(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_ClusterRole(in apiv1beta3.ClusterRole, out *apiv1beta3.ClusterRole, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if in.Rules != nil {
		out.Rules = make([]apiv1beta3.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := deepCopy_v1beta3_PolicyRule(in.Rules[i], &out.Rules[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func deepCopy_v1beta3_ClusterRoleBinding(in apiv1beta3.ClusterRoleBinding, out *apiv1beta3.ClusterRoleBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if in.UserNames != nil {
		out.UserNames = make([]string, len(in.UserNames))
		for i := range in.UserNames {
			out.UserNames[i] = in.UserNames[i]
		}
	} else {
		out.UserNames = nil
	}
	if in.GroupNames != nil {
		out.GroupNames = make([]string, len(in.GroupNames))
		for i := range in.GroupNames {
			out.GroupNames[i] = in.GroupNames[i]
		}
	} else {
		out.GroupNames = nil
	}
	if newVal, err := c.DeepCopy(in.RoleRef); err != nil {
		return err
	} else {
		out.RoleRef = newVal.(v1beta3.ObjectReference)
	}
	return nil
}

func deepCopy_v1beta3_ClusterRoleBindingList(in apiv1beta3.ClusterRoleBindingList, out *apiv1beta3.ClusterRoleBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.ClusterRoleBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_ClusterRoleBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_ClusterRoleList(in apiv1beta3.ClusterRoleList, out *apiv1beta3.ClusterRoleList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_ClusterRole(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_IsPersonalSubjectAccessReview(in apiv1beta3.IsPersonalSubjectAccessReview, out *apiv1beta3.IsPersonalSubjectAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	return nil
}

func deepCopy_v1beta3_NamedClusterRole(in apiv1beta3.NamedClusterRole, out *apiv1beta3.NamedClusterRole, c *conversion.Cloner) error {
	out.Name = in.Name
	if err := deepCopy_v1beta3_ClusterRole(in.Role, &out.Role, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_NamedClusterRoleBinding(in apiv1beta3.NamedClusterRoleBinding, out *apiv1beta3.NamedClusterRoleBinding, c *conversion.Cloner) error {
	out.Name = in.Name
	if err := deepCopy_v1beta3_ClusterRoleBinding(in.RoleBinding, &out.RoleBinding, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_NamedRole(in apiv1beta3.NamedRole, out *apiv1beta3.NamedRole, c *conversion.Cloner) error {
	out.Name = in.Name
	if err := deepCopy_v1beta3_Role(in.Role, &out.Role, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_NamedRoleBinding(in apiv1beta3.NamedRoleBinding, out *apiv1beta3.NamedRoleBinding, c *conversion.Cloner) error {
	out.Name = in.Name
	if err := deepCopy_v1beta3_RoleBinding(in.RoleBinding, &out.RoleBinding, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_Policy(in apiv1beta3.Policy, out *apiv1beta3.Policy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if in.Roles != nil {
		out.Roles = make([]apiv1beta3.NamedRole, len(in.Roles))
		for i := range in.Roles {
			if err := deepCopy_v1beta3_NamedRole(in.Roles[i], &out.Roles[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Roles = nil
	}
	return nil
}

func deepCopy_v1beta3_PolicyBinding(in apiv1beta3.PolicyBinding, out *apiv1beta3.PolicyBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if newVal, err := c.DeepCopy(in.PolicyRef); err != nil {
		return err
	} else {
		out.PolicyRef = newVal.(v1beta3.ObjectReference)
	}
	if in.RoleBindings != nil {
		out.RoleBindings = make([]apiv1beta3.NamedRoleBinding, len(in.RoleBindings))
		for i := range in.RoleBindings {
			if err := deepCopy_v1beta3_NamedRoleBinding(in.RoleBindings[i], &out.RoleBindings[i], c); err != nil {
				return err
			}
		}
	} else {
		out.RoleBindings = nil
	}
	return nil
}

func deepCopy_v1beta3_PolicyBindingList(in apiv1beta3.PolicyBindingList, out *apiv1beta3.PolicyBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.PolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_PolicyBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_PolicyList(in apiv1beta3.PolicyList, out *apiv1beta3.PolicyList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.Policy, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_Policy(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_PolicyRule(in apiv1beta3.PolicyRule, out *apiv1beta3.PolicyRule, c *conversion.Cloner) error {
	if in.Verbs != nil {
		out.Verbs = make([]string, len(in.Verbs))
		for i := range in.Verbs {
			out.Verbs[i] = in.Verbs[i]
		}
	} else {
		out.Verbs = nil
	}
	if newVal, err := c.DeepCopy(in.AttributeRestrictions); err != nil {
		return err
	} else {
		out.AttributeRestrictions = newVal.(runtime.RawExtension)
	}
	if in.ResourceKinds != nil {
		out.ResourceKinds = make([]string, len(in.ResourceKinds))
		for i := range in.ResourceKinds {
			out.ResourceKinds[i] = in.ResourceKinds[i]
		}
	} else {
		out.ResourceKinds = nil
	}
	if in.Resources != nil {
		out.Resources = make([]string, len(in.Resources))
		for i := range in.Resources {
			out.Resources[i] = in.Resources[i]
		}
	} else {
		out.Resources = nil
	}
	if in.ResourceNames != nil {
		out.ResourceNames = make([]string, len(in.ResourceNames))
		for i := range in.ResourceNames {
			out.ResourceNames[i] = in.ResourceNames[i]
		}
	} else {
		out.ResourceNames = nil
	}
	if in.NonResourceURLsSlice != nil {
		out.NonResourceURLsSlice = make([]string, len(in.NonResourceURLsSlice))
		for i := range in.NonResourceURLsSlice {
			out.NonResourceURLsSlice[i] = in.NonResourceURLsSlice[i]
		}
	} else {
		out.NonResourceURLsSlice = nil
	}
	return nil
}

func deepCopy_v1beta3_ResourceAccessReview(in apiv1beta3.ResourceAccessReview, out *apiv1beta3.ResourceAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	out.Verb = in.Verb
	out.Resource = in.Resource
	if newVal, err := c.DeepCopy(in.Content); err != nil {
		return err
	} else {
		out.Content = newVal.(runtime.RawExtension)
	}
	out.ResourceName = in.ResourceName
	return nil
}

func deepCopy_v1beta3_ResourceAccessReviewResponse(in apiv1beta3.ResourceAccessReviewResponse, out *apiv1beta3.ResourceAccessReviewResponse, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	out.Namespace = in.Namespace
	if in.UsersSlice != nil {
		out.UsersSlice = make([]string, len(in.UsersSlice))
		for i := range in.UsersSlice {
			out.UsersSlice[i] = in.UsersSlice[i]
		}
	} else {
		out.UsersSlice = nil
	}
	if in.GroupsSlice != nil {
		out.GroupsSlice = make([]string, len(in.GroupsSlice))
		for i := range in.GroupsSlice {
			out.GroupsSlice[i] = in.GroupsSlice[i]
		}
	} else {
		out.GroupsSlice = nil
	}
	return nil
}

func deepCopy_v1beta3_Role(in apiv1beta3.Role, out *apiv1beta3.Role, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if in.Rules != nil {
		out.Rules = make([]apiv1beta3.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := deepCopy_v1beta3_PolicyRule(in.Rules[i], &out.Rules[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func deepCopy_v1beta3_RoleBinding(in apiv1beta3.RoleBinding, out *apiv1beta3.RoleBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if in.UserNames != nil {
		out.UserNames = make([]string, len(in.UserNames))
		for i := range in.UserNames {
			out.UserNames[i] = in.UserNames[i]
		}
	} else {
		out.UserNames = nil
	}
	if in.GroupNames != nil {
		out.GroupNames = make([]string, len(in.GroupNames))
		for i := range in.GroupNames {
			out.GroupNames[i] = in.GroupNames[i]
		}
	} else {
		out.GroupNames = nil
	}
	if newVal, err := c.DeepCopy(in.RoleRef); err != nil {
		return err
	} else {
		out.RoleRef = newVal.(v1beta3.ObjectReference)
	}
	return nil
}

func deepCopy_v1beta3_RoleBindingList(in apiv1beta3.RoleBindingList, out *apiv1beta3.RoleBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.RoleBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_RoleBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_RoleList(in apiv1beta3.RoleList, out *apiv1beta3.RoleList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1beta3.Role, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_Role(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_SubjectAccessReview(in apiv1beta3.SubjectAccessReview, out *apiv1beta3.SubjectAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	out.Verb = in.Verb
	out.Resource = in.Resource
	out.User = in.User
	if in.GroupsSlice != nil {
		out.GroupsSlice = make([]string, len(in.GroupsSlice))
		for i := range in.GroupsSlice {
			out.GroupsSlice[i] = in.GroupsSlice[i]
		}
	} else {
		out.GroupsSlice = nil
	}
	if newVal, err := c.DeepCopy(in.Content); err != nil {
		return err
	} else {
		out.Content = newVal.(runtime.RawExtension)
	}
	out.ResourceName = in.ResourceName
	return nil
}

func deepCopy_v1beta3_SubjectAccessReviewResponse(in apiv1beta3.SubjectAccessReviewResponse, out *apiv1beta3.SubjectAccessReviewResponse, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func deepCopy_v1beta3_Build(in buildapiv1beta3.Build, out *buildapiv1beta3.Build, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if err := deepCopy_v1beta3_BuildSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1beta3_BuildStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_BuildConfig(in buildapiv1beta3.BuildConfig, out *buildapiv1beta3.BuildConfig, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if err := deepCopy_v1beta3_BuildConfigSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1beta3_BuildConfigStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_BuildConfigList(in buildapiv1beta3.BuildConfigList, out *buildapiv1beta3.BuildConfigList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]buildapiv1beta3.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_BuildConfig(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_BuildConfigSpec(in buildapiv1beta3.BuildConfigSpec, out *buildapiv1beta3.BuildConfigSpec, c *conversion.Cloner) error {
	if in.Triggers != nil {
		out.Triggers = make([]buildapiv1beta3.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := deepCopy_v1beta3_BuildTriggerPolicy(in.Triggers[i], &out.Triggers[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := deepCopy_v1beta3_BuildSpec(in.BuildSpec, &out.BuildSpec, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_BuildConfigStatus(in buildapiv1beta3.BuildConfigStatus, out *buildapiv1beta3.BuildConfigStatus, c *conversion.Cloner) error {
	out.LastVersion = in.LastVersion
	return nil
}

func deepCopy_v1beta3_BuildList(in buildapiv1beta3.BuildList, out *buildapiv1beta3.BuildList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]buildapiv1beta3.Build, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_Build(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_BuildLog(in buildapiv1beta3.BuildLog, out *buildapiv1beta3.BuildLog, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	return nil
}

func deepCopy_v1beta3_BuildLogOptions(in buildapiv1beta3.BuildLogOptions, out *buildapiv1beta3.BuildLogOptions, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	out.Follow = in.Follow
	out.NoWait = in.NoWait
	return nil
}

func deepCopy_v1beta3_BuildOutput(in buildapiv1beta3.BuildOutput, out *buildapiv1beta3.BuildOutput, c *conversion.Cloner) error {
	if in.To != nil {
		if newVal, err := c.DeepCopy(in.To); err != nil {
			return err
		} else {
			out.To = newVal.(*v1beta3.ObjectReference)
		}
	} else {
		out.To = nil
	}
	if in.PushSecret != nil {
		if newVal, err := c.DeepCopy(in.PushSecret); err != nil {
			return err
		} else {
			out.PushSecret = newVal.(*v1beta3.LocalObjectReference)
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func deepCopy_v1beta3_BuildRequest(in buildapiv1beta3.BuildRequest, out *buildapiv1beta3.BuildRequest, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if in.Revision != nil {
		out.Revision = new(buildapiv1beta3.SourceRevision)
		if err := deepCopy_v1beta3_SourceRevision(*in.Revision, out.Revision, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if in.TriggeredByImage != nil {
		if newVal, err := c.DeepCopy(in.TriggeredByImage); err != nil {
			return err
		} else {
			out.TriggeredByImage = newVal.(*v1beta3.ObjectReference)
		}
	} else {
		out.TriggeredByImage = nil
	}
	return nil
}

func deepCopy_v1beta3_BuildSource(in buildapiv1beta3.BuildSource, out *buildapiv1beta3.BuildSource, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.Git != nil {
		out.Git = new(buildapiv1beta3.GitBuildSource)
		if err := deepCopy_v1beta3_GitBuildSource(*in.Git, out.Git, c); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	out.ContextDir = in.ContextDir
	if in.SourceSecret != nil {
		if newVal, err := c.DeepCopy(in.SourceSecret); err != nil {
			return err
		} else {
			out.SourceSecret = newVal.(*v1beta3.LocalObjectReference)
		}
	} else {
		out.SourceSecret = nil
	}
	return nil
}

func deepCopy_v1beta3_BuildSpec(in buildapiv1beta3.BuildSpec, out *buildapiv1beta3.BuildSpec, c *conversion.Cloner) error {
	out.ServiceAccount = in.ServiceAccount
	if err := deepCopy_v1beta3_BuildSource(in.Source, &out.Source, c); err != nil {
		return err
	}
	if in.Revision != nil {
		out.Revision = new(buildapiv1beta3.SourceRevision)
		if err := deepCopy_v1beta3_SourceRevision(*in.Revision, out.Revision, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := deepCopy_v1beta3_BuildStrategy(in.Strategy, &out.Strategy, c); err != nil {
		return err
	}
	if err := deepCopy_v1beta3_BuildOutput(in.Output, &out.Output, c); err != nil {
		return err
	}
	if newVal, err := c.DeepCopy(in.Resources); err != nil {
		return err
	} else {
		out.Resources = newVal.(v1beta3.ResourceRequirements)
	}
	return nil
}

func deepCopy_v1beta3_BuildStatus(in buildapiv1beta3.BuildStatus, out *buildapiv1beta3.BuildStatus, c *conversion.Cloner) error {
	out.Phase = in.Phase
	out.Cancelled = in.Cancelled
	out.Message = in.Message
	if in.StartTimestamp != nil {
		if newVal, err := c.DeepCopy(in.StartTimestamp); err != nil {
			return err
		} else {
			out.StartTimestamp = newVal.(*util.Time)
		}
	} else {
		out.StartTimestamp = nil
	}
	if in.CompletionTimestamp != nil {
		if newVal, err := c.DeepCopy(in.CompletionTimestamp); err != nil {
			return err
		} else {
			out.CompletionTimestamp = newVal.(*util.Time)
		}
	} else {
		out.CompletionTimestamp = nil
	}
	out.Duration = in.Duration
	if in.Config != nil {
		if newVal, err := c.DeepCopy(in.Config); err != nil {
			return err
		} else {
			out.Config = newVal.(*v1beta3.ObjectReference)
		}
	} else {
		out.Config = nil
	}
	return nil
}

func deepCopy_v1beta3_BuildStrategy(in buildapiv1beta3.BuildStrategy, out *buildapiv1beta3.BuildStrategy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.DockerStrategy != nil {
		out.DockerStrategy = new(buildapiv1beta3.DockerBuildStrategy)
		if err := deepCopy_v1beta3_DockerBuildStrategy(*in.DockerStrategy, out.DockerStrategy, c); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	if in.SourceStrategy != nil {
		out.SourceStrategy = new(buildapiv1beta3.SourceBuildStrategy)
		if err := deepCopy_v1beta3_SourceBuildStrategy(*in.SourceStrategy, out.SourceStrategy, c); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	if in.CustomStrategy != nil {
		out.CustomStrategy = new(buildapiv1beta3.CustomBuildStrategy)
		if err := deepCopy_v1beta3_CustomBuildStrategy(*in.CustomStrategy, out.CustomStrategy, c); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	return nil
}

func deepCopy_v1beta3_BuildTriggerPolicy(in buildapiv1beta3.BuildTriggerPolicy, out *buildapiv1beta3.BuildTriggerPolicy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(buildapiv1beta3.WebHookTrigger)
		if err := deepCopy_v1beta3_WebHookTrigger(*in.GitHubWebHook, out.GitHubWebHook, c); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(buildapiv1beta3.WebHookTrigger)
		if err := deepCopy_v1beta3_WebHookTrigger(*in.GenericWebHook, out.GenericWebHook, c); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	if in.ImageChange != nil {
		out.ImageChange = new(buildapiv1beta3.ImageChangeTrigger)
		if err := deepCopy_v1beta3_ImageChangeTrigger(*in.ImageChange, out.ImageChange, c); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func deepCopy_v1beta3_CustomBuildStrategy(in buildapiv1beta3.CustomBuildStrategy, out *buildapiv1beta3.CustomBuildStrategy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(v1beta3.ObjectReference)
	}
	if in.PullSecret != nil {
		if newVal, err := c.DeepCopy(in.PullSecret); err != nil {
			return err
		} else {
			out.PullSecret = newVal.(*v1beta3.LocalObjectReference)
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]v1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(v1beta3.EnvVar)
			}
		}
	} else {
		out.Env = nil
	}
	out.ExposeDockerSocket = in.ExposeDockerSocket
	return nil
}

func deepCopy_v1beta3_DockerBuildStrategy(in buildapiv1beta3.DockerBuildStrategy, out *buildapiv1beta3.DockerBuildStrategy, c *conversion.Cloner) error {
	if in.From != nil {
		if newVal, err := c.DeepCopy(in.From); err != nil {
			return err
		} else {
			out.From = newVal.(*v1beta3.ObjectReference)
		}
	} else {
		out.From = nil
	}
	if in.PullSecret != nil {
		if newVal, err := c.DeepCopy(in.PullSecret); err != nil {
			return err
		} else {
			out.PullSecret = newVal.(*v1beta3.LocalObjectReference)
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	return nil
}

func deepCopy_v1beta3_GitBuildSource(in buildapiv1beta3.GitBuildSource, out *buildapiv1beta3.GitBuildSource, c *conversion.Cloner) error {
	out.URI = in.URI
	out.Ref = in.Ref
	return nil
}

func deepCopy_v1beta3_GitSourceRevision(in buildapiv1beta3.GitSourceRevision, out *buildapiv1beta3.GitSourceRevision, c *conversion.Cloner) error {
	out.Commit = in.Commit
	if err := deepCopy_v1beta3_SourceControlUser(in.Author, &out.Author, c); err != nil {
		return err
	}
	if err := deepCopy_v1beta3_SourceControlUser(in.Committer, &out.Committer, c); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func deepCopy_v1beta3_ImageChangeTrigger(in buildapiv1beta3.ImageChangeTrigger, out *buildapiv1beta3.ImageChangeTrigger, c *conversion.Cloner) error {
	out.LastTriggeredImageID = in.LastTriggeredImageID
	return nil
}

func deepCopy_v1beta3_SourceBuildStrategy(in buildapiv1beta3.SourceBuildStrategy, out *buildapiv1beta3.SourceBuildStrategy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(v1beta3.ObjectReference)
	}
	if in.PullSecret != nil {
		if newVal, err := c.DeepCopy(in.PullSecret); err != nil {
			return err
		} else {
			out.PullSecret = newVal.(*v1beta3.LocalObjectReference)
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]v1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(v1beta3.EnvVar)
			}
		}
	} else {
		out.Env = nil
	}
	out.Scripts = in.Scripts
	out.Incremental = in.Incremental
	return nil
}

func deepCopy_v1beta3_SourceControlUser(in buildapiv1beta3.SourceControlUser, out *buildapiv1beta3.SourceControlUser, c *conversion.Cloner) error {
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func deepCopy_v1beta3_SourceRevision(in buildapiv1beta3.SourceRevision, out *buildapiv1beta3.SourceRevision, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.Git != nil {
		out.Git = new(buildapiv1beta3.GitSourceRevision)
		if err := deepCopy_v1beta3_GitSourceRevision(*in.Git, out.Git, c); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func deepCopy_v1beta3_WebHookTrigger(in buildapiv1beta3.WebHookTrigger, out *buildapiv1beta3.WebHookTrigger, c *conversion.Cloner) error {
	out.Secret = in.Secret
	return nil
}

func deepCopy_v1beta3_CustomDeploymentStrategyParams(in deployapiv1beta3.CustomDeploymentStrategyParams, out *deployapiv1beta3.CustomDeploymentStrategyParams, c *conversion.Cloner) error {
	out.Image = in.Image
	if in.Environment != nil {
		out.Environment = make([]v1beta3.EnvVar, len(in.Environment))
		for i := range in.Environment {
			if newVal, err := c.DeepCopy(in.Environment[i]); err != nil {
				return err
			} else {
				out.Environment[i] = newVal.(v1beta3.EnvVar)
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

func deepCopy_v1beta3_DeploymentCause(in deployapiv1beta3.DeploymentCause, out *deployapiv1beta3.DeploymentCause, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.ImageTrigger != nil {
		out.ImageTrigger = new(deployapiv1beta3.DeploymentCauseImageTrigger)
		if err := deepCopy_v1beta3_DeploymentCauseImageTrigger(*in.ImageTrigger, out.ImageTrigger, c); err != nil {
			return err
		}
	} else {
		out.ImageTrigger = nil
	}
	return nil
}

func deepCopy_v1beta3_DeploymentCauseImageTrigger(in deployapiv1beta3.DeploymentCauseImageTrigger, out *deployapiv1beta3.DeploymentCauseImageTrigger, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(v1beta3.ObjectReference)
	}
	return nil
}

func deepCopy_v1beta3_DeploymentConfig(in deployapiv1beta3.DeploymentConfig, out *deployapiv1beta3.DeploymentConfig, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if err := deepCopy_v1beta3_DeploymentConfigSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1beta3_DeploymentConfigStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_DeploymentConfigList(in deployapiv1beta3.DeploymentConfigList, out *deployapiv1beta3.DeploymentConfigList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]deployapiv1beta3.DeploymentConfig, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_DeploymentConfig(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_DeploymentConfigRollback(in deployapiv1beta3.DeploymentConfigRollback, out *deployapiv1beta3.DeploymentConfigRollback, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if err := deepCopy_v1beta3_DeploymentConfigRollbackSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_DeploymentConfigRollbackSpec(in deployapiv1beta3.DeploymentConfigRollbackSpec, out *deployapiv1beta3.DeploymentConfigRollbackSpec, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(v1beta3.ObjectReference)
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func deepCopy_v1beta3_DeploymentConfigSpec(in deployapiv1beta3.DeploymentConfigSpec, out *deployapiv1beta3.DeploymentConfigSpec, c *conversion.Cloner) error {
	if err := deepCopy_v1beta3_DeploymentStrategy(in.Strategy, &out.Strategy, c); err != nil {
		return err
	}
	if in.Triggers != nil {
		out.Triggers = make([]deployapiv1beta3.DeploymentTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := deepCopy_v1beta3_DeploymentTriggerPolicy(in.Triggers[i], &out.Triggers[i], c); err != nil {
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
	if in.TemplateRef != nil {
		if newVal, err := c.DeepCopy(in.TemplateRef); err != nil {
			return err
		} else {
			out.TemplateRef = newVal.(*v1beta3.ObjectReference)
		}
	} else {
		out.TemplateRef = nil
	}
	if in.Template != nil {
		if newVal, err := c.DeepCopy(in.Template); err != nil {
			return err
		} else {
			out.Template = newVal.(*v1beta3.PodTemplateSpec)
		}
	} else {
		out.Template = nil
	}
	return nil
}

func deepCopy_v1beta3_DeploymentConfigStatus(in deployapiv1beta3.DeploymentConfigStatus, out *deployapiv1beta3.DeploymentConfigStatus, c *conversion.Cloner) error {
	out.LatestVersion = in.LatestVersion
	if in.Details != nil {
		out.Details = new(deployapiv1beta3.DeploymentDetails)
		if err := deepCopy_v1beta3_DeploymentDetails(*in.Details, out.Details, c); err != nil {
			return err
		}
	} else {
		out.Details = nil
	}
	return nil
}

func deepCopy_v1beta3_DeploymentDetails(in deployapiv1beta3.DeploymentDetails, out *deployapiv1beta3.DeploymentDetails, c *conversion.Cloner) error {
	out.Message = in.Message
	if in.Causes != nil {
		out.Causes = make([]*deployapiv1beta3.DeploymentCause, len(in.Causes))
		for i := range in.Causes {
			if newVal, err := c.DeepCopy(in.Causes[i]); err != nil {
				return err
			} else {
				out.Causes[i] = newVal.(*deployapiv1beta3.DeploymentCause)
			}
		}
	} else {
		out.Causes = nil
	}
	return nil
}

func deepCopy_v1beta3_DeploymentStrategy(in deployapiv1beta3.DeploymentStrategy, out *deployapiv1beta3.DeploymentStrategy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.CustomParams != nil {
		out.CustomParams = new(deployapiv1beta3.CustomDeploymentStrategyParams)
		if err := deepCopy_v1beta3_CustomDeploymentStrategyParams(*in.CustomParams, out.CustomParams, c); err != nil {
			return err
		}
	} else {
		out.CustomParams = nil
	}
	if in.RecreateParams != nil {
		out.RecreateParams = new(deployapiv1beta3.RecreateDeploymentStrategyParams)
		if err := deepCopy_v1beta3_RecreateDeploymentStrategyParams(*in.RecreateParams, out.RecreateParams, c); err != nil {
			return err
		}
	} else {
		out.RecreateParams = nil
	}
	if in.RollingParams != nil {
		out.RollingParams = new(deployapiv1beta3.RollingDeploymentStrategyParams)
		if err := deepCopy_v1beta3_RollingDeploymentStrategyParams(*in.RollingParams, out.RollingParams, c); err != nil {
			return err
		}
	} else {
		out.RollingParams = nil
	}
	if newVal, err := c.DeepCopy(in.Resources); err != nil {
		return err
	} else {
		out.Resources = newVal.(v1beta3.ResourceRequirements)
	}
	return nil
}

func deepCopy_v1beta3_DeploymentTriggerImageChangeParams(in deployapiv1beta3.DeploymentTriggerImageChangeParams, out *deployapiv1beta3.DeploymentTriggerImageChangeParams, c *conversion.Cloner) error {
	out.Automatic = in.Automatic
	if in.ContainerNames != nil {
		out.ContainerNames = make([]string, len(in.ContainerNames))
		for i := range in.ContainerNames {
			out.ContainerNames[i] = in.ContainerNames[i]
		}
	} else {
		out.ContainerNames = nil
	}
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(v1beta3.ObjectReference)
	}
	out.LastTriggeredImage = in.LastTriggeredImage
	return nil
}

func deepCopy_v1beta3_DeploymentTriggerPolicy(in deployapiv1beta3.DeploymentTriggerPolicy, out *deployapiv1beta3.DeploymentTriggerPolicy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.ImageChangeParams != nil {
		out.ImageChangeParams = new(deployapiv1beta3.DeploymentTriggerImageChangeParams)
		if err := deepCopy_v1beta3_DeploymentTriggerImageChangeParams(*in.ImageChangeParams, out.ImageChangeParams, c); err != nil {
			return err
		}
	} else {
		out.ImageChangeParams = nil
	}
	return nil
}

func deepCopy_v1beta3_ExecNewPodHook(in deployapiv1beta3.ExecNewPodHook, out *deployapiv1beta3.ExecNewPodHook, c *conversion.Cloner) error {
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	if in.Env != nil {
		out.Env = make([]v1beta3.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(v1beta3.EnvVar)
			}
		}
	} else {
		out.Env = nil
	}
	out.ContainerName = in.ContainerName
	return nil
}

func deepCopy_v1beta3_LifecycleHook(in deployapiv1beta3.LifecycleHook, out *deployapiv1beta3.LifecycleHook, c *conversion.Cloner) error {
	out.FailurePolicy = in.FailurePolicy
	if in.ExecNewPod != nil {
		out.ExecNewPod = new(deployapiv1beta3.ExecNewPodHook)
		if err := deepCopy_v1beta3_ExecNewPodHook(*in.ExecNewPod, out.ExecNewPod, c); err != nil {
			return err
		}
	} else {
		out.ExecNewPod = nil
	}
	return nil
}

func deepCopy_v1beta3_RecreateDeploymentStrategyParams(in deployapiv1beta3.RecreateDeploymentStrategyParams, out *deployapiv1beta3.RecreateDeploymentStrategyParams, c *conversion.Cloner) error {
	if in.Pre != nil {
		out.Pre = new(deployapiv1beta3.LifecycleHook)
		if err := deepCopy_v1beta3_LifecycleHook(*in.Pre, out.Pre, c); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapiv1beta3.LifecycleHook)
		if err := deepCopy_v1beta3_LifecycleHook(*in.Post, out.Post, c); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func deepCopy_v1beta3_RollingDeploymentStrategyParams(in deployapiv1beta3.RollingDeploymentStrategyParams, out *deployapiv1beta3.RollingDeploymentStrategyParams, c *conversion.Cloner) error {
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
	if in.Pre != nil {
		out.Pre = new(deployapiv1beta3.LifecycleHook)
		if err := deepCopy_v1beta3_LifecycleHook(*in.Pre, out.Pre, c); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapiv1beta3.LifecycleHook)
		if err := deepCopy_v1beta3_LifecycleHook(*in.Post, out.Post, c); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func deepCopy_v1beta3_Image(in imageapiv1beta3.Image, out *imageapiv1beta3.Image, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	out.DockerImageReference = in.DockerImageReference
	if newVal, err := c.DeepCopy(in.DockerImageMetadata); err != nil {
		return err
	} else {
		out.DockerImageMetadata = newVal.(runtime.RawExtension)
	}
	out.DockerImageMetadataVersion = in.DockerImageMetadataVersion
	out.DockerImageManifest = in.DockerImageManifest
	return nil
}

func deepCopy_v1beta3_ImageList(in imageapiv1beta3.ImageList, out *imageapiv1beta3.ImageList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1beta3.Image, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_Image(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_ImageStream(in imageapiv1beta3.ImageStream, out *imageapiv1beta3.ImageStream, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if err := deepCopy_v1beta3_ImageStreamSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1beta3_ImageStreamStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_ImageStreamImage(in imageapiv1beta3.ImageStreamImage, out *imageapiv1beta3.ImageStreamImage, c *conversion.Cloner) error {
	if err := deepCopy_v1beta3_Image(in.Image, &out.Image, c); err != nil {
		return err
	}
	out.ImageName = in.ImageName
	return nil
}

func deepCopy_v1beta3_ImageStreamList(in imageapiv1beta3.ImageStreamList, out *imageapiv1beta3.ImageStreamList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1beta3.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_ImageStream(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_ImageStreamMapping(in imageapiv1beta3.ImageStreamMapping, out *imageapiv1beta3.ImageStreamMapping, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if err := deepCopy_v1beta3_Image(in.Image, &out.Image, c); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func deepCopy_v1beta3_ImageStreamSpec(in imageapiv1beta3.ImageStreamSpec, out *imageapiv1beta3.ImageStreamSpec, c *conversion.Cloner) error {
	out.DockerImageRepository = in.DockerImageRepository
	if in.Tags != nil {
		out.Tags = make([]imageapiv1beta3.NamedTagReference, len(in.Tags))
		for i := range in.Tags {
			if err := deepCopy_v1beta3_NamedTagReference(in.Tags[i], &out.Tags[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Tags = nil
	}
	return nil
}

func deepCopy_v1beta3_ImageStreamStatus(in imageapiv1beta3.ImageStreamStatus, out *imageapiv1beta3.ImageStreamStatus, c *conversion.Cloner) error {
	out.DockerImageRepository = in.DockerImageRepository
	if in.Tags != nil {
		out.Tags = make([]imageapiv1beta3.NamedTagEventList, len(in.Tags))
		for i := range in.Tags {
			if err := deepCopy_v1beta3_NamedTagEventList(in.Tags[i], &out.Tags[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Tags = nil
	}
	return nil
}

func deepCopy_v1beta3_ImageStreamTag(in imageapiv1beta3.ImageStreamTag, out *imageapiv1beta3.ImageStreamTag, c *conversion.Cloner) error {
	if err := deepCopy_v1beta3_Image(in.Image, &out.Image, c); err != nil {
		return err
	}
	out.ImageName = in.ImageName
	return nil
}

func deepCopy_v1beta3_NamedTagEventList(in imageapiv1beta3.NamedTagEventList, out *imageapiv1beta3.NamedTagEventList, c *conversion.Cloner) error {
	out.Tag = in.Tag
	if in.Items != nil {
		out.Items = make([]imageapiv1beta3.TagEvent, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_TagEvent(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_NamedTagReference(in imageapiv1beta3.NamedTagReference, out *imageapiv1beta3.NamedTagReference, c *conversion.Cloner) error {
	out.Name = in.Name
	if in.Annotations != nil {
		out.Annotations = make(map[string]string)
		for key, val := range in.Annotations {
			out.Annotations[key] = val
		}
	} else {
		out.Annotations = nil
	}
	if in.From != nil {
		if newVal, err := c.DeepCopy(in.From); err != nil {
			return err
		} else {
			out.From = newVal.(*v1beta3.ObjectReference)
		}
	} else {
		out.From = nil
	}
	return nil
}

func deepCopy_v1beta3_TagEvent(in imageapiv1beta3.TagEvent, out *imageapiv1beta3.TagEvent, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.Created); err != nil {
		return err
	} else {
		out.Created = newVal.(util.Time)
	}
	out.DockerImageReference = in.DockerImageReference
	out.Image = in.Image
	return nil
}

func deepCopy_v1beta3_OAuthAccessToken(in oauthapiv1beta3.OAuthAccessToken, out *oauthapiv1beta3.OAuthAccessToken, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
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

func deepCopy_v1beta3_OAuthAccessTokenList(in oauthapiv1beta3.OAuthAccessTokenList, out *oauthapiv1beta3.OAuthAccessTokenList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_OAuthAccessToken(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_OAuthAuthorizeToken(in oauthapiv1beta3.OAuthAuthorizeToken, out *oauthapiv1beta3.OAuthAuthorizeToken, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
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

func deepCopy_v1beta3_OAuthAuthorizeTokenList(in oauthapiv1beta3.OAuthAuthorizeTokenList, out *oauthapiv1beta3.OAuthAuthorizeTokenList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_OAuthAuthorizeToken(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_OAuthClient(in oauthapiv1beta3.OAuthClient, out *oauthapiv1beta3.OAuthClient, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
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

func deepCopy_v1beta3_OAuthClientAuthorization(in oauthapiv1beta3.OAuthClientAuthorization, out *oauthapiv1beta3.OAuthClientAuthorization, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
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

func deepCopy_v1beta3_OAuthClientAuthorizationList(in oauthapiv1beta3.OAuthClientAuthorizationList, out *oauthapiv1beta3.OAuthClientAuthorizationList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_OAuthClientAuthorization(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_OAuthClientList(in oauthapiv1beta3.OAuthClientList, out *oauthapiv1beta3.OAuthClientList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1beta3.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_OAuthClient(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_Project(in projectapiv1beta3.Project, out *projectapiv1beta3.Project, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if err := deepCopy_v1beta3_ProjectSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1beta3_ProjectStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_ProjectList(in projectapiv1beta3.ProjectList, out *projectapiv1beta3.ProjectList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]projectapiv1beta3.Project, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_Project(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_ProjectRequest(in projectapiv1beta3.ProjectRequest, out *projectapiv1beta3.ProjectRequest, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func deepCopy_v1beta3_ProjectSpec(in projectapiv1beta3.ProjectSpec, out *projectapiv1beta3.ProjectSpec, c *conversion.Cloner) error {
	if in.Finalizers != nil {
		out.Finalizers = make([]v1beta3.FinalizerName, len(in.Finalizers))
		for i := range in.Finalizers {
			out.Finalizers[i] = in.Finalizers[i]
		}
	} else {
		out.Finalizers = nil
	}
	return nil
}

func deepCopy_v1beta3_ProjectStatus(in projectapiv1beta3.ProjectStatus, out *projectapiv1beta3.ProjectStatus, c *conversion.Cloner) error {
	out.Phase = in.Phase
	return nil
}

func deepCopy_v1beta3_Route(in routeapiv1beta3.Route, out *routeapiv1beta3.Route, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if err := deepCopy_v1beta3_RouteSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1beta3_RouteStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1beta3_RouteList(in routeapiv1beta3.RouteList, out *routeapiv1beta3.RouteList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]routeapiv1beta3.Route, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_Route(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_RouteSpec(in routeapiv1beta3.RouteSpec, out *routeapiv1beta3.RouteSpec, c *conversion.Cloner) error {
	out.Host = in.Host
	out.Path = in.Path
	if newVal, err := c.DeepCopy(in.To); err != nil {
		return err
	} else {
		out.To = newVal.(v1beta3.ObjectReference)
	}
	if in.TLS != nil {
		out.TLS = new(routeapiv1beta3.TLSConfig)
		if err := deepCopy_v1beta3_TLSConfig(*in.TLS, out.TLS, c); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func deepCopy_v1beta3_RouteStatus(in routeapiv1beta3.RouteStatus, out *routeapiv1beta3.RouteStatus, c *conversion.Cloner) error {
	return nil
}

func deepCopy_v1beta3_TLSConfig(in routeapiv1beta3.TLSConfig, out *routeapiv1beta3.TLSConfig, c *conversion.Cloner) error {
	out.Termination = in.Termination
	out.Certificate = in.Certificate
	out.Key = in.Key
	out.CACertificate = in.CACertificate
	out.DestinationCACertificate = in.DestinationCACertificate
	return nil
}

func deepCopy_v1beta3_ClusterNetwork(in sdnapiv1beta3.ClusterNetwork, out *sdnapiv1beta3.ClusterNetwork, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	return nil
}

func deepCopy_v1beta3_ClusterNetworkList(in sdnapiv1beta3.ClusterNetworkList, out *sdnapiv1beta3.ClusterNetworkList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1beta3.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_ClusterNetwork(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_HostSubnet(in sdnapiv1beta3.HostSubnet, out *sdnapiv1beta3.HostSubnet, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func deepCopy_v1beta3_HostSubnetList(in sdnapiv1beta3.HostSubnetList, out *sdnapiv1beta3.HostSubnetList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1beta3.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_HostSubnet(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_Parameter(in templateapiv1beta3.Parameter, out *templateapiv1beta3.Parameter, c *conversion.Cloner) error {
	out.Name = in.Name
	out.Description = in.Description
	out.Value = in.Value
	out.Generate = in.Generate
	out.From = in.From
	return nil
}

func deepCopy_v1beta3_Template(in templateapiv1beta3.Template, out *templateapiv1beta3.Template, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if in.Objects != nil {
		out.Objects = make([]runtime.RawExtension, len(in.Objects))
		for i := range in.Objects {
			if newVal, err := c.DeepCopy(in.Objects[i]); err != nil {
				return err
			} else {
				out.Objects[i] = newVal.(runtime.RawExtension)
			}
		}
	} else {
		out.Objects = nil
	}
	if in.Parameters != nil {
		out.Parameters = make([]templateapiv1beta3.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := deepCopy_v1beta3_Parameter(in.Parameters[i], &out.Parameters[i], c); err != nil {
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

func deepCopy_v1beta3_TemplateList(in templateapiv1beta3.TemplateList, out *templateapiv1beta3.TemplateList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]templateapiv1beta3.Template, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_Template(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_Identity(in userapiv1beta3.Identity, out *userapiv1beta3.Identity, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if newVal, err := c.DeepCopy(in.User); err != nil {
		return err
	} else {
		out.User = newVal.(v1beta3.ObjectReference)
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

func deepCopy_v1beta3_IdentityList(in userapiv1beta3.IdentityList, out *userapiv1beta3.IdentityList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]userapiv1beta3.Identity, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_Identity(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1beta3_User(in userapiv1beta3.User, out *userapiv1beta3.User, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
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

func deepCopy_v1beta3_UserIdentityMapping(in userapiv1beta3.UserIdentityMapping, out *userapiv1beta3.UserIdentityMapping, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1beta3.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.Identity); err != nil {
		return err
	} else {
		out.Identity = newVal.(v1beta3.ObjectReference)
	}
	if newVal, err := c.DeepCopy(in.User); err != nil {
		return err
	} else {
		out.User = newVal.(v1beta3.ObjectReference)
	}
	return nil
}

func deepCopy_v1beta3_UserList(in userapiv1beta3.UserList, out *userapiv1beta3.UserList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1beta3.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1beta3.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]userapiv1beta3.User, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1beta3_User(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func init() {
	err := api.Scheme.AddGeneratedDeepCopyFuncs(
		deepCopy_v1beta3_ClusterPolicy,
		deepCopy_v1beta3_ClusterPolicyBinding,
		deepCopy_v1beta3_ClusterPolicyBindingList,
		deepCopy_v1beta3_ClusterPolicyList,
		deepCopy_v1beta3_ClusterRole,
		deepCopy_v1beta3_ClusterRoleBinding,
		deepCopy_v1beta3_ClusterRoleBindingList,
		deepCopy_v1beta3_ClusterRoleList,
		deepCopy_v1beta3_IsPersonalSubjectAccessReview,
		deepCopy_v1beta3_NamedClusterRole,
		deepCopy_v1beta3_NamedClusterRoleBinding,
		deepCopy_v1beta3_NamedRole,
		deepCopy_v1beta3_NamedRoleBinding,
		deepCopy_v1beta3_Policy,
		deepCopy_v1beta3_PolicyBinding,
		deepCopy_v1beta3_PolicyBindingList,
		deepCopy_v1beta3_PolicyList,
		deepCopy_v1beta3_PolicyRule,
		deepCopy_v1beta3_ResourceAccessReview,
		deepCopy_v1beta3_ResourceAccessReviewResponse,
		deepCopy_v1beta3_Role,
		deepCopy_v1beta3_RoleBinding,
		deepCopy_v1beta3_RoleBindingList,
		deepCopy_v1beta3_RoleList,
		deepCopy_v1beta3_SubjectAccessReview,
		deepCopy_v1beta3_SubjectAccessReviewResponse,
		deepCopy_v1beta3_Build,
		deepCopy_v1beta3_BuildConfig,
		deepCopy_v1beta3_BuildConfigList,
		deepCopy_v1beta3_BuildConfigSpec,
		deepCopy_v1beta3_BuildConfigStatus,
		deepCopy_v1beta3_BuildList,
		deepCopy_v1beta3_BuildLog,
		deepCopy_v1beta3_BuildLogOptions,
		deepCopy_v1beta3_BuildOutput,
		deepCopy_v1beta3_BuildRequest,
		deepCopy_v1beta3_BuildSource,
		deepCopy_v1beta3_BuildSpec,
		deepCopy_v1beta3_BuildStatus,
		deepCopy_v1beta3_BuildStrategy,
		deepCopy_v1beta3_BuildTriggerPolicy,
		deepCopy_v1beta3_CustomBuildStrategy,
		deepCopy_v1beta3_DockerBuildStrategy,
		deepCopy_v1beta3_GitBuildSource,
		deepCopy_v1beta3_GitSourceRevision,
		deepCopy_v1beta3_ImageChangeTrigger,
		deepCopy_v1beta3_SourceBuildStrategy,
		deepCopy_v1beta3_SourceControlUser,
		deepCopy_v1beta3_SourceRevision,
		deepCopy_v1beta3_WebHookTrigger,
		deepCopy_v1beta3_CustomDeploymentStrategyParams,
		deepCopy_v1beta3_DeploymentCause,
		deepCopy_v1beta3_DeploymentCauseImageTrigger,
		deepCopy_v1beta3_DeploymentConfig,
		deepCopy_v1beta3_DeploymentConfigList,
		deepCopy_v1beta3_DeploymentConfigRollback,
		deepCopy_v1beta3_DeploymentConfigRollbackSpec,
		deepCopy_v1beta3_DeploymentConfigSpec,
		deepCopy_v1beta3_DeploymentConfigStatus,
		deepCopy_v1beta3_DeploymentDetails,
		deepCopy_v1beta3_DeploymentStrategy,
		deepCopy_v1beta3_DeploymentTriggerImageChangeParams,
		deepCopy_v1beta3_DeploymentTriggerPolicy,
		deepCopy_v1beta3_ExecNewPodHook,
		deepCopy_v1beta3_LifecycleHook,
		deepCopy_v1beta3_RecreateDeploymentStrategyParams,
		deepCopy_v1beta3_RollingDeploymentStrategyParams,
		deepCopy_v1beta3_Image,
		deepCopy_v1beta3_ImageList,
		deepCopy_v1beta3_ImageStream,
		deepCopy_v1beta3_ImageStreamImage,
		deepCopy_v1beta3_ImageStreamList,
		deepCopy_v1beta3_ImageStreamMapping,
		deepCopy_v1beta3_ImageStreamSpec,
		deepCopy_v1beta3_ImageStreamStatus,
		deepCopy_v1beta3_ImageStreamTag,
		deepCopy_v1beta3_NamedTagEventList,
		deepCopy_v1beta3_NamedTagReference,
		deepCopy_v1beta3_TagEvent,
		deepCopy_v1beta3_OAuthAccessToken,
		deepCopy_v1beta3_OAuthAccessTokenList,
		deepCopy_v1beta3_OAuthAuthorizeToken,
		deepCopy_v1beta3_OAuthAuthorizeTokenList,
		deepCopy_v1beta3_OAuthClient,
		deepCopy_v1beta3_OAuthClientAuthorization,
		deepCopy_v1beta3_OAuthClientAuthorizationList,
		deepCopy_v1beta3_OAuthClientList,
		deepCopy_v1beta3_Project,
		deepCopy_v1beta3_ProjectList,
		deepCopy_v1beta3_ProjectRequest,
		deepCopy_v1beta3_ProjectSpec,
		deepCopy_v1beta3_ProjectStatus,
		deepCopy_v1beta3_Route,
		deepCopy_v1beta3_RouteList,
		deepCopy_v1beta3_RouteSpec,
		deepCopy_v1beta3_RouteStatus,
		deepCopy_v1beta3_TLSConfig,
		deepCopy_v1beta3_ClusterNetwork,
		deepCopy_v1beta3_ClusterNetworkList,
		deepCopy_v1beta3_HostSubnet,
		deepCopy_v1beta3_HostSubnetList,
		deepCopy_v1beta3_Parameter,
		deepCopy_v1beta3_Template,
		deepCopy_v1beta3_TemplateList,
		deepCopy_v1beta3_Identity,
		deepCopy_v1beta3_IdentityList,
		deepCopy_v1beta3_User,
		deepCopy_v1beta3_UserIdentityMapping,
		deepCopy_v1beta3_UserList,
	)
	if err != nil {
		// if one of the deep copy functions is malformed, detect it immediately.
		panic(err)
	}
}

// AUTO-GENERATED FUNCTIONS END HERE
