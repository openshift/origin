package v1

// AUTO-GENERATED FUNCTIONS START HERE
import (
	api "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	v1 "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"
	conversion "github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	runtime "github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	util "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	apiv1 "github.com/openshift/origin/pkg/authorization/api/v1"
	buildapiv1 "github.com/openshift/origin/pkg/build/api/v1"
	deployapiv1 "github.com/openshift/origin/pkg/deploy/api/v1"
	imageapiv1 "github.com/openshift/origin/pkg/image/api/v1"
	oauthapiv1 "github.com/openshift/origin/pkg/oauth/api/v1"
	projectapiv1 "github.com/openshift/origin/pkg/project/api/v1"
	routeapiv1 "github.com/openshift/origin/pkg/route/api/v1"
	sdnapiv1 "github.com/openshift/origin/pkg/sdn/api/v1"
	templateapiv1 "github.com/openshift/origin/pkg/template/api/v1"
	userapiv1 "github.com/openshift/origin/pkg/user/api/v1"
)

func deepCopy_v1_ClusterPolicy(in apiv1.ClusterPolicy, out *apiv1.ClusterPolicy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if in.Roles != nil {
		out.Roles = make([]apiv1.NamedClusterRole, len(in.Roles))
		for i := range in.Roles {
			if err := deepCopy_v1_NamedClusterRole(in.Roles[i], &out.Roles[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Roles = nil
	}
	return nil
}

func deepCopy_v1_ClusterPolicyBinding(in apiv1.ClusterPolicyBinding, out *apiv1.ClusterPolicyBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if newVal, err := c.DeepCopy(in.PolicyRef); err != nil {
		return err
	} else {
		out.PolicyRef = newVal.(v1.ObjectReference)
	}
	if in.RoleBindings != nil {
		out.RoleBindings = make([]apiv1.NamedClusterRoleBinding, len(in.RoleBindings))
		for i := range in.RoleBindings {
			if err := deepCopy_v1_NamedClusterRoleBinding(in.RoleBindings[i], &out.RoleBindings[i], c); err != nil {
				return err
			}
		}
	} else {
		out.RoleBindings = nil
	}
	return nil
}

func deepCopy_v1_ClusterPolicyBindingList(in apiv1.ClusterPolicyBindingList, out *apiv1.ClusterPolicyBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1.ClusterPolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_ClusterPolicyBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_ClusterPolicyList(in apiv1.ClusterPolicyList, out *apiv1.ClusterPolicyList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1.ClusterPolicy, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_ClusterPolicy(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_ClusterRole(in apiv1.ClusterRole, out *apiv1.ClusterRole, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if in.Rules != nil {
		out.Rules = make([]apiv1.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := deepCopy_v1_PolicyRule(in.Rules[i], &out.Rules[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func deepCopy_v1_ClusterRoleBinding(in apiv1.ClusterRoleBinding, out *apiv1.ClusterRoleBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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
		out.RoleRef = newVal.(v1.ObjectReference)
	}
	return nil
}

func deepCopy_v1_ClusterRoleBindingList(in apiv1.ClusterRoleBindingList, out *apiv1.ClusterRoleBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1.ClusterRoleBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_ClusterRoleBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_ClusterRoleList(in apiv1.ClusterRoleList, out *apiv1.ClusterRoleList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_ClusterRole(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_IsPersonalSubjectAccessReview(in apiv1.IsPersonalSubjectAccessReview, out *apiv1.IsPersonalSubjectAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	return nil
}

func deepCopy_v1_NamedClusterRole(in apiv1.NamedClusterRole, out *apiv1.NamedClusterRole, c *conversion.Cloner) error {
	out.Name = in.Name
	if err := deepCopy_v1_ClusterRole(in.Role, &out.Role, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_NamedClusterRoleBinding(in apiv1.NamedClusterRoleBinding, out *apiv1.NamedClusterRoleBinding, c *conversion.Cloner) error {
	out.Name = in.Name
	if err := deepCopy_v1_ClusterRoleBinding(in.RoleBinding, &out.RoleBinding, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_NamedRole(in apiv1.NamedRole, out *apiv1.NamedRole, c *conversion.Cloner) error {
	out.Name = in.Name
	if err := deepCopy_v1_Role(in.Role, &out.Role, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_NamedRoleBinding(in apiv1.NamedRoleBinding, out *apiv1.NamedRoleBinding, c *conversion.Cloner) error {
	out.Name = in.Name
	if err := deepCopy_v1_RoleBinding(in.RoleBinding, &out.RoleBinding, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_Policy(in apiv1.Policy, out *apiv1.Policy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if in.Roles != nil {
		out.Roles = make([]apiv1.NamedRole, len(in.Roles))
		for i := range in.Roles {
			if err := deepCopy_v1_NamedRole(in.Roles[i], &out.Roles[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Roles = nil
	}
	return nil
}

func deepCopy_v1_PolicyBinding(in apiv1.PolicyBinding, out *apiv1.PolicyBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if newVal, err := c.DeepCopy(in.PolicyRef); err != nil {
		return err
	} else {
		out.PolicyRef = newVal.(v1.ObjectReference)
	}
	if in.RoleBindings != nil {
		out.RoleBindings = make([]apiv1.NamedRoleBinding, len(in.RoleBindings))
		for i := range in.RoleBindings {
			if err := deepCopy_v1_NamedRoleBinding(in.RoleBindings[i], &out.RoleBindings[i], c); err != nil {
				return err
			}
		}
	} else {
		out.RoleBindings = nil
	}
	return nil
}

func deepCopy_v1_PolicyBindingList(in apiv1.PolicyBindingList, out *apiv1.PolicyBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1.PolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_PolicyBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_PolicyList(in apiv1.PolicyList, out *apiv1.PolicyList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1.Policy, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_Policy(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_PolicyRule(in apiv1.PolicyRule, out *apiv1.PolicyRule, c *conversion.Cloner) error {
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

func deepCopy_v1_ResourceAccessReview(in apiv1.ResourceAccessReview, out *apiv1.ResourceAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
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

func deepCopy_v1_ResourceAccessReviewResponse(in apiv1.ResourceAccessReviewResponse, out *apiv1.ResourceAccessReviewResponse, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
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

func deepCopy_v1_Role(in apiv1.Role, out *apiv1.Role, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if in.Rules != nil {
		out.Rules = make([]apiv1.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := deepCopy_v1_PolicyRule(in.Rules[i], &out.Rules[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func deepCopy_v1_RoleBinding(in apiv1.RoleBinding, out *apiv1.RoleBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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
		out.RoleRef = newVal.(v1.ObjectReference)
	}
	return nil
}

func deepCopy_v1_RoleBindingList(in apiv1.RoleBindingList, out *apiv1.RoleBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1.RoleBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_RoleBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_RoleList(in apiv1.RoleList, out *apiv1.RoleList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]apiv1.Role, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_Role(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_SubjectAccessReview(in apiv1.SubjectAccessReview, out *apiv1.SubjectAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
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

func deepCopy_v1_SubjectAccessReviewResponse(in apiv1.SubjectAccessReviewResponse, out *apiv1.SubjectAccessReviewResponse, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func deepCopy_v1_Build(in buildapiv1.Build, out *buildapiv1.Build, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if err := deepCopy_v1_BuildSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1_BuildStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_BuildConfig(in buildapiv1.BuildConfig, out *buildapiv1.BuildConfig, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if err := deepCopy_v1_BuildConfigSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1_BuildConfigStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_BuildConfigList(in buildapiv1.BuildConfigList, out *buildapiv1.BuildConfigList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]buildapiv1.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_BuildConfig(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_BuildConfigSpec(in buildapiv1.BuildConfigSpec, out *buildapiv1.BuildConfigSpec, c *conversion.Cloner) error {
	if in.Triggers != nil {
		out.Triggers = make([]buildapiv1.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := deepCopy_v1_BuildTriggerPolicy(in.Triggers[i], &out.Triggers[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := deepCopy_v1_BuildSpec(in.BuildSpec, &out.BuildSpec, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_BuildConfigStatus(in buildapiv1.BuildConfigStatus, out *buildapiv1.BuildConfigStatus, c *conversion.Cloner) error {
	out.LastVersion = in.LastVersion
	return nil
}

func deepCopy_v1_BuildList(in buildapiv1.BuildList, out *buildapiv1.BuildList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]buildapiv1.Build, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_Build(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_BuildLog(in buildapiv1.BuildLog, out *buildapiv1.BuildLog, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	return nil
}

func deepCopy_v1_BuildLogOptions(in buildapiv1.BuildLogOptions, out *buildapiv1.BuildLogOptions, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	out.Follow = in.Follow
	out.NoWait = in.NoWait
	return nil
}

func deepCopy_v1_BuildOutput(in buildapiv1.BuildOutput, out *buildapiv1.BuildOutput, c *conversion.Cloner) error {
	if in.To != nil {
		if newVal, err := c.DeepCopy(in.To); err != nil {
			return err
		} else {
			out.To = newVal.(*v1.ObjectReference)
		}
	} else {
		out.To = nil
	}
	if in.PushSecret != nil {
		if newVal, err := c.DeepCopy(in.PushSecret); err != nil {
			return err
		} else {
			out.PushSecret = newVal.(*v1.LocalObjectReference)
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func deepCopy_v1_BuildRequest(in buildapiv1.BuildRequest, out *buildapiv1.BuildRequest, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if in.Revision != nil {
		out.Revision = new(buildapiv1.SourceRevision)
		if err := deepCopy_v1_SourceRevision(*in.Revision, out.Revision, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if in.TriggeredByImage != nil {
		if newVal, err := c.DeepCopy(in.TriggeredByImage); err != nil {
			return err
		} else {
			out.TriggeredByImage = newVal.(*v1.ObjectReference)
		}
	} else {
		out.TriggeredByImage = nil
	}
	return nil
}

func deepCopy_v1_BuildSource(in buildapiv1.BuildSource, out *buildapiv1.BuildSource, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.Git != nil {
		out.Git = new(buildapiv1.GitBuildSource)
		if err := deepCopy_v1_GitBuildSource(*in.Git, out.Git, c); err != nil {
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
			out.SourceSecret = newVal.(*v1.LocalObjectReference)
		}
	} else {
		out.SourceSecret = nil
	}
	return nil
}

func deepCopy_v1_BuildSpec(in buildapiv1.BuildSpec, out *buildapiv1.BuildSpec, c *conversion.Cloner) error {
	out.ServiceAccount = in.ServiceAccount
	if err := deepCopy_v1_BuildSource(in.Source, &out.Source, c); err != nil {
		return err
	}
	if in.Revision != nil {
		out.Revision = new(buildapiv1.SourceRevision)
		if err := deepCopy_v1_SourceRevision(*in.Revision, out.Revision, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := deepCopy_v1_BuildStrategy(in.Strategy, &out.Strategy, c); err != nil {
		return err
	}
	if err := deepCopy_v1_BuildOutput(in.Output, &out.Output, c); err != nil {
		return err
	}
	if newVal, err := c.DeepCopy(in.Resources); err != nil {
		return err
	} else {
		out.Resources = newVal.(v1.ResourceRequirements)
	}
	return nil
}

func deepCopy_v1_BuildStatus(in buildapiv1.BuildStatus, out *buildapiv1.BuildStatus, c *conversion.Cloner) error {
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
			out.Config = newVal.(*v1.ObjectReference)
		}
	} else {
		out.Config = nil
	}
	return nil
}

func deepCopy_v1_BuildStrategy(in buildapiv1.BuildStrategy, out *buildapiv1.BuildStrategy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.DockerStrategy != nil {
		out.DockerStrategy = new(buildapiv1.DockerBuildStrategy)
		if err := deepCopy_v1_DockerBuildStrategy(*in.DockerStrategy, out.DockerStrategy, c); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	if in.SourceStrategy != nil {
		out.SourceStrategy = new(buildapiv1.SourceBuildStrategy)
		if err := deepCopy_v1_SourceBuildStrategy(*in.SourceStrategy, out.SourceStrategy, c); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	if in.CustomStrategy != nil {
		out.CustomStrategy = new(buildapiv1.CustomBuildStrategy)
		if err := deepCopy_v1_CustomBuildStrategy(*in.CustomStrategy, out.CustomStrategy, c); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	return nil
}

func deepCopy_v1_BuildTriggerPolicy(in buildapiv1.BuildTriggerPolicy, out *buildapiv1.BuildTriggerPolicy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(buildapiv1.WebHookTrigger)
		if err := deepCopy_v1_WebHookTrigger(*in.GitHubWebHook, out.GitHubWebHook, c); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(buildapiv1.WebHookTrigger)
		if err := deepCopy_v1_WebHookTrigger(*in.GenericWebHook, out.GenericWebHook, c); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	if in.ImageChange != nil {
		out.ImageChange = new(buildapiv1.ImageChangeTrigger)
		if err := deepCopy_v1_ImageChangeTrigger(*in.ImageChange, out.ImageChange, c); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func deepCopy_v1_CustomBuildStrategy(in buildapiv1.CustomBuildStrategy, out *buildapiv1.CustomBuildStrategy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(v1.ObjectReference)
	}
	if in.PullSecret != nil {
		if newVal, err := c.DeepCopy(in.PullSecret); err != nil {
			return err
		} else {
			out.PullSecret = newVal.(*v1.LocalObjectReference)
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]v1.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(v1.EnvVar)
			}
		}
	} else {
		out.Env = nil
	}
	out.ExposeDockerSocket = in.ExposeDockerSocket
	return nil
}

func deepCopy_v1_DockerBuildStrategy(in buildapiv1.DockerBuildStrategy, out *buildapiv1.DockerBuildStrategy, c *conversion.Cloner) error {
	if in.From != nil {
		if newVal, err := c.DeepCopy(in.From); err != nil {
			return err
		} else {
			out.From = newVal.(*v1.ObjectReference)
		}
	} else {
		out.From = nil
	}
	if in.PullSecret != nil {
		if newVal, err := c.DeepCopy(in.PullSecret); err != nil {
			return err
		} else {
			out.PullSecret = newVal.(*v1.LocalObjectReference)
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]v1.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(v1.EnvVar)
			}
		}
	} else {
		out.Env = nil
	}
	out.ForcePull = in.ForcePull
	return nil
}

func deepCopy_v1_GitBuildSource(in buildapiv1.GitBuildSource, out *buildapiv1.GitBuildSource, c *conversion.Cloner) error {
	out.URI = in.URI
	out.Ref = in.Ref
	out.HTTPProxy = in.HTTPProxy
	out.HTTPSProxy = in.HTTPSProxy
	return nil
}

func deepCopy_v1_GitSourceRevision(in buildapiv1.GitSourceRevision, out *buildapiv1.GitSourceRevision, c *conversion.Cloner) error {
	out.Commit = in.Commit
	if err := deepCopy_v1_SourceControlUser(in.Author, &out.Author, c); err != nil {
		return err
	}
	if err := deepCopy_v1_SourceControlUser(in.Committer, &out.Committer, c); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func deepCopy_v1_ImageChangeTrigger(in buildapiv1.ImageChangeTrigger, out *buildapiv1.ImageChangeTrigger, c *conversion.Cloner) error {
	out.LastTriggeredImageID = in.LastTriggeredImageID
	return nil
}

func deepCopy_v1_SourceBuildStrategy(in buildapiv1.SourceBuildStrategy, out *buildapiv1.SourceBuildStrategy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(v1.ObjectReference)
	}
	if in.PullSecret != nil {
		if newVal, err := c.DeepCopy(in.PullSecret); err != nil {
			return err
		} else {
			out.PullSecret = newVal.(*v1.LocalObjectReference)
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]v1.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(v1.EnvVar)
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

func deepCopy_v1_SourceControlUser(in buildapiv1.SourceControlUser, out *buildapiv1.SourceControlUser, c *conversion.Cloner) error {
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func deepCopy_v1_SourceRevision(in buildapiv1.SourceRevision, out *buildapiv1.SourceRevision, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.Git != nil {
		out.Git = new(buildapiv1.GitSourceRevision)
		if err := deepCopy_v1_GitSourceRevision(*in.Git, out.Git, c); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func deepCopy_v1_WebHookTrigger(in buildapiv1.WebHookTrigger, out *buildapiv1.WebHookTrigger, c *conversion.Cloner) error {
	out.Secret = in.Secret
	return nil
}

func deepCopy_v1_CustomDeploymentStrategyParams(in deployapiv1.CustomDeploymentStrategyParams, out *deployapiv1.CustomDeploymentStrategyParams, c *conversion.Cloner) error {
	out.Image = in.Image
	if in.Environment != nil {
		out.Environment = make([]v1.EnvVar, len(in.Environment))
		for i := range in.Environment {
			if newVal, err := c.DeepCopy(in.Environment[i]); err != nil {
				return err
			} else {
				out.Environment[i] = newVal.(v1.EnvVar)
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

func deepCopy_v1_DeploymentCause(in deployapiv1.DeploymentCause, out *deployapiv1.DeploymentCause, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.ImageTrigger != nil {
		out.ImageTrigger = new(deployapiv1.DeploymentCauseImageTrigger)
		if err := deepCopy_v1_DeploymentCauseImageTrigger(*in.ImageTrigger, out.ImageTrigger, c); err != nil {
			return err
		}
	} else {
		out.ImageTrigger = nil
	}
	return nil
}

func deepCopy_v1_DeploymentCauseImageTrigger(in deployapiv1.DeploymentCauseImageTrigger, out *deployapiv1.DeploymentCauseImageTrigger, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(v1.ObjectReference)
	}
	return nil
}

func deepCopy_v1_DeploymentConfig(in deployapiv1.DeploymentConfig, out *deployapiv1.DeploymentConfig, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if err := deepCopy_v1_DeploymentConfigSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1_DeploymentConfigStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_DeploymentConfigList(in deployapiv1.DeploymentConfigList, out *deployapiv1.DeploymentConfigList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]deployapiv1.DeploymentConfig, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_DeploymentConfig(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_DeploymentConfigRollback(in deployapiv1.DeploymentConfigRollback, out *deployapiv1.DeploymentConfigRollback, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if err := deepCopy_v1_DeploymentConfigRollbackSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_DeploymentConfigRollbackSpec(in deployapiv1.DeploymentConfigRollbackSpec, out *deployapiv1.DeploymentConfigRollbackSpec, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(v1.ObjectReference)
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func deepCopy_v1_DeploymentConfigSpec(in deployapiv1.DeploymentConfigSpec, out *deployapiv1.DeploymentConfigSpec, c *conversion.Cloner) error {
	if err := deepCopy_v1_DeploymentStrategy(in.Strategy, &out.Strategy, c); err != nil {
		return err
	}
	if in.Triggers != nil {
		out.Triggers = make([]deployapiv1.DeploymentTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := deepCopy_v1_DeploymentTriggerPolicy(in.Triggers[i], &out.Triggers[i], c); err != nil {
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
		if newVal, err := c.DeepCopy(in.Template); err != nil {
			return err
		} else {
			out.Template = newVal.(*v1.PodTemplateSpec)
		}
	} else {
		out.Template = nil
	}
	return nil
}

func deepCopy_v1_DeploymentConfigStatus(in deployapiv1.DeploymentConfigStatus, out *deployapiv1.DeploymentConfigStatus, c *conversion.Cloner) error {
	out.LatestVersion = in.LatestVersion
	if in.Details != nil {
		out.Details = new(deployapiv1.DeploymentDetails)
		if err := deepCopy_v1_DeploymentDetails(*in.Details, out.Details, c); err != nil {
			return err
		}
	} else {
		out.Details = nil
	}
	return nil
}

func deepCopy_v1_DeploymentDetails(in deployapiv1.DeploymentDetails, out *deployapiv1.DeploymentDetails, c *conversion.Cloner) error {
	out.Message = in.Message
	if in.Causes != nil {
		out.Causes = make([]*deployapiv1.DeploymentCause, len(in.Causes))
		for i := range in.Causes {
			if newVal, err := c.DeepCopy(in.Causes[i]); err != nil {
				return err
			} else {
				out.Causes[i] = newVal.(*deployapiv1.DeploymentCause)
			}
		}
	} else {
		out.Causes = nil
	}
	return nil
}

func deepCopy_v1_DeploymentStrategy(in deployapiv1.DeploymentStrategy, out *deployapiv1.DeploymentStrategy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.CustomParams != nil {
		out.CustomParams = new(deployapiv1.CustomDeploymentStrategyParams)
		if err := deepCopy_v1_CustomDeploymentStrategyParams(*in.CustomParams, out.CustomParams, c); err != nil {
			return err
		}
	} else {
		out.CustomParams = nil
	}
	if in.RecreateParams != nil {
		out.RecreateParams = new(deployapiv1.RecreateDeploymentStrategyParams)
		if err := deepCopy_v1_RecreateDeploymentStrategyParams(*in.RecreateParams, out.RecreateParams, c); err != nil {
			return err
		}
	} else {
		out.RecreateParams = nil
	}
	if in.RollingParams != nil {
		out.RollingParams = new(deployapiv1.RollingDeploymentStrategyParams)
		if err := deepCopy_v1_RollingDeploymentStrategyParams(*in.RollingParams, out.RollingParams, c); err != nil {
			return err
		}
	} else {
		out.RollingParams = nil
	}
	if newVal, err := c.DeepCopy(in.Resources); err != nil {
		return err
	} else {
		out.Resources = newVal.(v1.ResourceRequirements)
	}
	return nil
}

func deepCopy_v1_DeploymentTriggerImageChangeParams(in deployapiv1.DeploymentTriggerImageChangeParams, out *deployapiv1.DeploymentTriggerImageChangeParams, c *conversion.Cloner) error {
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
		out.From = newVal.(v1.ObjectReference)
	}
	out.LastTriggeredImage = in.LastTriggeredImage
	return nil
}

func deepCopy_v1_DeploymentTriggerPolicy(in deployapiv1.DeploymentTriggerPolicy, out *deployapiv1.DeploymentTriggerPolicy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.ImageChangeParams != nil {
		out.ImageChangeParams = new(deployapiv1.DeploymentTriggerImageChangeParams)
		if err := deepCopy_v1_DeploymentTriggerImageChangeParams(*in.ImageChangeParams, out.ImageChangeParams, c); err != nil {
			return err
		}
	} else {
		out.ImageChangeParams = nil
	}
	return nil
}

func deepCopy_v1_ExecNewPodHook(in deployapiv1.ExecNewPodHook, out *deployapiv1.ExecNewPodHook, c *conversion.Cloner) error {
	if in.Command != nil {
		out.Command = make([]string, len(in.Command))
		for i := range in.Command {
			out.Command[i] = in.Command[i]
		}
	} else {
		out.Command = nil
	}
	if in.Env != nil {
		out.Env = make([]v1.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(v1.EnvVar)
			}
		}
	} else {
		out.Env = nil
	}
	out.ContainerName = in.ContainerName
	return nil
}

func deepCopy_v1_LifecycleHook(in deployapiv1.LifecycleHook, out *deployapiv1.LifecycleHook, c *conversion.Cloner) error {
	out.FailurePolicy = in.FailurePolicy
	if in.ExecNewPod != nil {
		out.ExecNewPod = new(deployapiv1.ExecNewPodHook)
		if err := deepCopy_v1_ExecNewPodHook(*in.ExecNewPod, out.ExecNewPod, c); err != nil {
			return err
		}
	} else {
		out.ExecNewPod = nil
	}
	return nil
}

func deepCopy_v1_RecreateDeploymentStrategyParams(in deployapiv1.RecreateDeploymentStrategyParams, out *deployapiv1.RecreateDeploymentStrategyParams, c *conversion.Cloner) error {
	if in.Pre != nil {
		out.Pre = new(deployapiv1.LifecycleHook)
		if err := deepCopy_v1_LifecycleHook(*in.Pre, out.Pre, c); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapiv1.LifecycleHook)
		if err := deepCopy_v1_LifecycleHook(*in.Post, out.Post, c); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func deepCopy_v1_RollingDeploymentStrategyParams(in deployapiv1.RollingDeploymentStrategyParams, out *deployapiv1.RollingDeploymentStrategyParams, c *conversion.Cloner) error {
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
	if in.UpdatePercent != nil {
		out.UpdatePercent = new(int)
		*out.UpdatePercent = *in.UpdatePercent
	} else {
		out.UpdatePercent = nil
	}
	if in.Pre != nil {
		out.Pre = new(deployapiv1.LifecycleHook)
		if err := deepCopy_v1_LifecycleHook(*in.Pre, out.Pre, c); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapiv1.LifecycleHook)
		if err := deepCopy_v1_LifecycleHook(*in.Post, out.Post, c); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func deepCopy_v1_Image(in imageapiv1.Image, out *imageapiv1.Image, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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

func deepCopy_v1_ImageList(in imageapiv1.ImageList, out *imageapiv1.ImageList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1.Image, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_Image(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_ImageStream(in imageapiv1.ImageStream, out *imageapiv1.ImageStream, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if err := deepCopy_v1_ImageStreamSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1_ImageStreamStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_ImageStreamImage(in imageapiv1.ImageStreamImage, out *imageapiv1.ImageStreamImage, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if err := deepCopy_v1_Image(in.Image, &out.Image, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_ImageStreamList(in imageapiv1.ImageStreamList, out *imageapiv1.ImageStreamList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]imageapiv1.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_ImageStream(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_ImageStreamMapping(in imageapiv1.ImageStreamMapping, out *imageapiv1.ImageStreamMapping, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if err := deepCopy_v1_Image(in.Image, &out.Image, c); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func deepCopy_v1_ImageStreamSpec(in imageapiv1.ImageStreamSpec, out *imageapiv1.ImageStreamSpec, c *conversion.Cloner) error {
	out.DockerImageRepository = in.DockerImageRepository
	if in.Tags != nil {
		out.Tags = make([]imageapiv1.NamedTagReference, len(in.Tags))
		for i := range in.Tags {
			if err := deepCopy_v1_NamedTagReference(in.Tags[i], &out.Tags[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Tags = nil
	}
	return nil
}

func deepCopy_v1_ImageStreamStatus(in imageapiv1.ImageStreamStatus, out *imageapiv1.ImageStreamStatus, c *conversion.Cloner) error {
	out.DockerImageRepository = in.DockerImageRepository
	if in.Tags != nil {
		out.Tags = make([]imageapiv1.NamedTagEventList, len(in.Tags))
		for i := range in.Tags {
			if err := deepCopy_v1_NamedTagEventList(in.Tags[i], &out.Tags[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Tags = nil
	}
	return nil
}

func deepCopy_v1_ImageStreamTag(in imageapiv1.ImageStreamTag, out *imageapiv1.ImageStreamTag, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if err := deepCopy_v1_Image(in.Image, &out.Image, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_NamedTagEventList(in imageapiv1.NamedTagEventList, out *imageapiv1.NamedTagEventList, c *conversion.Cloner) error {
	out.Tag = in.Tag
	if in.Items != nil {
		out.Items = make([]imageapiv1.TagEvent, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_TagEvent(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_NamedTagReference(in imageapiv1.NamedTagReference, out *imageapiv1.NamedTagReference, c *conversion.Cloner) error {
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
			out.From = newVal.(*v1.ObjectReference)
		}
	} else {
		out.From = nil
	}
	return nil
}

func deepCopy_v1_TagEvent(in imageapiv1.TagEvent, out *imageapiv1.TagEvent, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.Created); err != nil {
		return err
	} else {
		out.Created = newVal.(util.Time)
	}
	out.DockerImageReference = in.DockerImageReference
	out.Image = in.Image
	return nil
}

func deepCopy_v1_OAuthAccessToken(in oauthapiv1.OAuthAccessToken, out *oauthapiv1.OAuthAccessToken, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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

func deepCopy_v1_OAuthAccessTokenList(in oauthapiv1.OAuthAccessTokenList, out *oauthapiv1.OAuthAccessTokenList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_OAuthAccessToken(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_OAuthAuthorizeToken(in oauthapiv1.OAuthAuthorizeToken, out *oauthapiv1.OAuthAuthorizeToken, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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

func deepCopy_v1_OAuthAuthorizeTokenList(in oauthapiv1.OAuthAuthorizeTokenList, out *oauthapiv1.OAuthAuthorizeTokenList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_OAuthAuthorizeToken(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_OAuthClient(in oauthapiv1.OAuthClient, out *oauthapiv1.OAuthClient, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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

func deepCopy_v1_OAuthClientAuthorization(in oauthapiv1.OAuthClientAuthorization, out *oauthapiv1.OAuthClientAuthorization, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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

func deepCopy_v1_OAuthClientAuthorizationList(in oauthapiv1.OAuthClientAuthorizationList, out *oauthapiv1.OAuthClientAuthorizationList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_OAuthClientAuthorization(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_OAuthClientList(in oauthapiv1.OAuthClientList, out *oauthapiv1.OAuthClientList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapiv1.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_OAuthClient(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_Project(in projectapiv1.Project, out *projectapiv1.Project, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if err := deepCopy_v1_ProjectSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1_ProjectStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_ProjectList(in projectapiv1.ProjectList, out *projectapiv1.ProjectList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]projectapiv1.Project, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_Project(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_ProjectRequest(in projectapiv1.ProjectRequest, out *projectapiv1.ProjectRequest, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func deepCopy_v1_ProjectSpec(in projectapiv1.ProjectSpec, out *projectapiv1.ProjectSpec, c *conversion.Cloner) error {
	if in.Finalizers != nil {
		out.Finalizers = make([]v1.FinalizerName, len(in.Finalizers))
		for i := range in.Finalizers {
			out.Finalizers[i] = in.Finalizers[i]
		}
	} else {
		out.Finalizers = nil
	}
	return nil
}

func deepCopy_v1_ProjectStatus(in projectapiv1.ProjectStatus, out *projectapiv1.ProjectStatus, c *conversion.Cloner) error {
	out.Phase = in.Phase
	return nil
}

func deepCopy_v1_Route(in routeapiv1.Route, out *routeapiv1.Route, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if err := deepCopy_v1_RouteSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_v1_RouteStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_v1_RouteList(in routeapiv1.RouteList, out *routeapiv1.RouteList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]routeapiv1.Route, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_Route(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_RouteSpec(in routeapiv1.RouteSpec, out *routeapiv1.RouteSpec, c *conversion.Cloner) error {
	out.Host = in.Host
	out.Path = in.Path
	if newVal, err := c.DeepCopy(in.To); err != nil {
		return err
	} else {
		out.To = newVal.(v1.ObjectReference)
	}
	if in.TLS != nil {
		out.TLS = new(routeapiv1.TLSConfig)
		if err := deepCopy_v1_TLSConfig(*in.TLS, out.TLS, c); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func deepCopy_v1_RouteStatus(in routeapiv1.RouteStatus, out *routeapiv1.RouteStatus, c *conversion.Cloner) error {
	return nil
}

func deepCopy_v1_TLSConfig(in routeapiv1.TLSConfig, out *routeapiv1.TLSConfig, c *conversion.Cloner) error {
	out.Termination = in.Termination
	out.Certificate = in.Certificate
	out.Key = in.Key
	out.CACertificate = in.CACertificate
	out.DestinationCACertificate = in.DestinationCACertificate
	return nil
}

func deepCopy_v1_ClusterNetwork(in sdnapiv1.ClusterNetwork, out *sdnapiv1.ClusterNetwork, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	return nil
}

func deepCopy_v1_ClusterNetworkList(in sdnapiv1.ClusterNetworkList, out *sdnapiv1.ClusterNetworkList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_ClusterNetwork(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_HostSubnet(in sdnapiv1.HostSubnet, out *sdnapiv1.HostSubnet, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func deepCopy_v1_HostSubnetList(in sdnapiv1.HostSubnetList, out *sdnapiv1.HostSubnetList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]sdnapiv1.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_HostSubnet(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_Parameter(in templateapiv1.Parameter, out *templateapiv1.Parameter, c *conversion.Cloner) error {
	out.Name = in.Name
	out.Description = in.Description
	out.Value = in.Value
	out.Generate = in.Generate
	out.From = in.From
	return nil
}

func deepCopy_v1_Template(in templateapiv1.Template, out *templateapiv1.Template, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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
		out.Parameters = make([]templateapiv1.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := deepCopy_v1_Parameter(in.Parameters[i], &out.Parameters[i], c); err != nil {
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

func deepCopy_v1_TemplateList(in templateapiv1.TemplateList, out *templateapiv1.TemplateList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]templateapiv1.Template, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_Template(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_Group(in userapiv1.Group, out *userapiv1.Group, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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

func deepCopy_v1_GroupList(in userapiv1.GroupList, out *userapiv1.GroupList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]userapiv1.Group, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_Group(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_Identity(in userapiv1.Identity, out *userapiv1.Identity, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if newVal, err := c.DeepCopy(in.User); err != nil {
		return err
	} else {
		out.User = newVal.(v1.ObjectReference)
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

func deepCopy_v1_IdentityList(in userapiv1.IdentityList, out *userapiv1.IdentityList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]userapiv1.Identity, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_Identity(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_v1_User(in userapiv1.User, out *userapiv1.User, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
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

func deepCopy_v1_UserIdentityMapping(in userapiv1.UserIdentityMapping, out *userapiv1.UserIdentityMapping, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(v1.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.Identity); err != nil {
		return err
	} else {
		out.Identity = newVal.(v1.ObjectReference)
	}
	if newVal, err := c.DeepCopy(in.User); err != nil {
		return err
	} else {
		out.User = newVal.(v1.ObjectReference)
	}
	return nil
}

func deepCopy_v1_UserList(in userapiv1.UserList, out *userapiv1.UserList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(v1.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(v1.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]userapiv1.User, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_v1_User(in.Items[i], &out.Items[i], c); err != nil {
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
		deepCopy_v1_ClusterPolicy,
		deepCopy_v1_ClusterPolicyBinding,
		deepCopy_v1_ClusterPolicyBindingList,
		deepCopy_v1_ClusterPolicyList,
		deepCopy_v1_ClusterRole,
		deepCopy_v1_ClusterRoleBinding,
		deepCopy_v1_ClusterRoleBindingList,
		deepCopy_v1_ClusterRoleList,
		deepCopy_v1_IsPersonalSubjectAccessReview,
		deepCopy_v1_NamedClusterRole,
		deepCopy_v1_NamedClusterRoleBinding,
		deepCopy_v1_NamedRole,
		deepCopy_v1_NamedRoleBinding,
		deepCopy_v1_Policy,
		deepCopy_v1_PolicyBinding,
		deepCopy_v1_PolicyBindingList,
		deepCopy_v1_PolicyList,
		deepCopy_v1_PolicyRule,
		deepCopy_v1_ResourceAccessReview,
		deepCopy_v1_ResourceAccessReviewResponse,
		deepCopy_v1_Role,
		deepCopy_v1_RoleBinding,
		deepCopy_v1_RoleBindingList,
		deepCopy_v1_RoleList,
		deepCopy_v1_SubjectAccessReview,
		deepCopy_v1_SubjectAccessReviewResponse,
		deepCopy_v1_Build,
		deepCopy_v1_BuildConfig,
		deepCopy_v1_BuildConfigList,
		deepCopy_v1_BuildConfigSpec,
		deepCopy_v1_BuildConfigStatus,
		deepCopy_v1_BuildList,
		deepCopy_v1_BuildLog,
		deepCopy_v1_BuildLogOptions,
		deepCopy_v1_BuildOutput,
		deepCopy_v1_BuildRequest,
		deepCopy_v1_BuildSource,
		deepCopy_v1_BuildSpec,
		deepCopy_v1_BuildStatus,
		deepCopy_v1_BuildStrategy,
		deepCopy_v1_BuildTriggerPolicy,
		deepCopy_v1_CustomBuildStrategy,
		deepCopy_v1_DockerBuildStrategy,
		deepCopy_v1_GitBuildSource,
		deepCopy_v1_GitSourceRevision,
		deepCopy_v1_ImageChangeTrigger,
		deepCopy_v1_SourceBuildStrategy,
		deepCopy_v1_SourceControlUser,
		deepCopy_v1_SourceRevision,
		deepCopy_v1_WebHookTrigger,
		deepCopy_v1_CustomDeploymentStrategyParams,
		deepCopy_v1_DeploymentCause,
		deepCopy_v1_DeploymentCauseImageTrigger,
		deepCopy_v1_DeploymentConfig,
		deepCopy_v1_DeploymentConfigList,
		deepCopy_v1_DeploymentConfigRollback,
		deepCopy_v1_DeploymentConfigRollbackSpec,
		deepCopy_v1_DeploymentConfigSpec,
		deepCopy_v1_DeploymentConfigStatus,
		deepCopy_v1_DeploymentDetails,
		deepCopy_v1_DeploymentStrategy,
		deepCopy_v1_DeploymentTriggerImageChangeParams,
		deepCopy_v1_DeploymentTriggerPolicy,
		deepCopy_v1_ExecNewPodHook,
		deepCopy_v1_LifecycleHook,
		deepCopy_v1_RecreateDeploymentStrategyParams,
		deepCopy_v1_RollingDeploymentStrategyParams,
		deepCopy_v1_Image,
		deepCopy_v1_ImageList,
		deepCopy_v1_ImageStream,
		deepCopy_v1_ImageStreamImage,
		deepCopy_v1_ImageStreamList,
		deepCopy_v1_ImageStreamMapping,
		deepCopy_v1_ImageStreamSpec,
		deepCopy_v1_ImageStreamStatus,
		deepCopy_v1_ImageStreamTag,
		deepCopy_v1_NamedTagEventList,
		deepCopy_v1_NamedTagReference,
		deepCopy_v1_TagEvent,
		deepCopy_v1_OAuthAccessToken,
		deepCopy_v1_OAuthAccessTokenList,
		deepCopy_v1_OAuthAuthorizeToken,
		deepCopy_v1_OAuthAuthorizeTokenList,
		deepCopy_v1_OAuthClient,
		deepCopy_v1_OAuthClientAuthorization,
		deepCopy_v1_OAuthClientAuthorizationList,
		deepCopy_v1_OAuthClientList,
		deepCopy_v1_Project,
		deepCopy_v1_ProjectList,
		deepCopy_v1_ProjectRequest,
		deepCopy_v1_ProjectSpec,
		deepCopy_v1_ProjectStatus,
		deepCopy_v1_Route,
		deepCopy_v1_RouteList,
		deepCopy_v1_RouteSpec,
		deepCopy_v1_RouteStatus,
		deepCopy_v1_TLSConfig,
		deepCopy_v1_ClusterNetwork,
		deepCopy_v1_ClusterNetworkList,
		deepCopy_v1_HostSubnet,
		deepCopy_v1_HostSubnetList,
		deepCopy_v1_Parameter,
		deepCopy_v1_Template,
		deepCopy_v1_TemplateList,
		deepCopy_v1_Group,
		deepCopy_v1_GroupList,
		deepCopy_v1_Identity,
		deepCopy_v1_IdentityList,
		deepCopy_v1_User,
		deepCopy_v1_UserIdentityMapping,
		deepCopy_v1_UserList,
	)
	if err != nil {
		// if one of the deep copy functions is malformed, detect it immediately.
		panic(err)
	}
}

// AUTO-GENERATED FUNCTIONS END HERE
