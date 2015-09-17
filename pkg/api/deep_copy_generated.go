package api

// AUTO-GENERATED FUNCTIONS START HERE
import (
	api "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
	sdnapi "github.com/openshift/origin/pkg/sdn/api"
	templateapi "github.com/openshift/origin/pkg/template/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	pkgapi "k8s.io/kubernetes/pkg/api"
	conversion "k8s.io/kubernetes/pkg/conversion"
	runtime "k8s.io/kubernetes/pkg/runtime"
	util "k8s.io/kubernetes/pkg/util"
)

func deepCopy_api_AuthorizationAttributes(in api.AuthorizationAttributes, out *api.AuthorizationAttributes, c *conversion.Cloner) error {
	out.Namespace = in.Namespace
	out.Verb = in.Verb
	out.Resource = in.Resource
	out.ResourceName = in.ResourceName
	if newVal, err := c.DeepCopy(in.Content); err != nil {
		return err
	} else {
		out.Content = newVal.(runtime.EmbeddedObject)
	}
	return nil
}

func deepCopy_api_ClusterPolicy(in api.ClusterPolicy, out *api.ClusterPolicy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if in.Roles != nil {
		out.Roles = make(map[string]*api.ClusterRole)
		for key, val := range in.Roles {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.Roles[key] = newVal.(*api.ClusterRole)
			}
		}
	} else {
		out.Roles = nil
	}
	return nil
}

func deepCopy_api_ClusterPolicyBinding(in api.ClusterPolicyBinding, out *api.ClusterPolicyBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if newVal, err := c.DeepCopy(in.PolicyRef); err != nil {
		return err
	} else {
		out.PolicyRef = newVal.(pkgapi.ObjectReference)
	}
	if in.RoleBindings != nil {
		out.RoleBindings = make(map[string]*api.ClusterRoleBinding)
		for key, val := range in.RoleBindings {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.RoleBindings[key] = newVal.(*api.ClusterRoleBinding)
			}
		}
	} else {
		out.RoleBindings = nil
	}
	return nil
}

func deepCopy_api_ClusterPolicyBindingList(in api.ClusterPolicyBindingList, out *api.ClusterPolicyBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]api.ClusterPolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_ClusterPolicyBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_ClusterPolicyList(in api.ClusterPolicyList, out *api.ClusterPolicyList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]api.ClusterPolicy, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_ClusterPolicy(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_ClusterRole(in api.ClusterRole, out *api.ClusterRole, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if in.Rules != nil {
		out.Rules = make([]api.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := deepCopy_api_PolicyRule(in.Rules[i], &out.Rules[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func deepCopy_api_ClusterRoleBinding(in api.ClusterRoleBinding, out *api.ClusterRoleBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if in.Subjects != nil {
		out.Subjects = make([]pkgapi.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if newVal, err := c.DeepCopy(in.Subjects[i]); err != nil {
				return err
			} else {
				out.Subjects[i] = newVal.(pkgapi.ObjectReference)
			}
		}
	} else {
		out.Subjects = nil
	}
	if newVal, err := c.DeepCopy(in.RoleRef); err != nil {
		return err
	} else {
		out.RoleRef = newVal.(pkgapi.ObjectReference)
	}
	return nil
}

func deepCopy_api_ClusterRoleBindingList(in api.ClusterRoleBindingList, out *api.ClusterRoleBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]api.ClusterRoleBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_ClusterRoleBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_ClusterRoleList(in api.ClusterRoleList, out *api.ClusterRoleList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]api.ClusterRole, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_ClusterRole(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_IsPersonalSubjectAccessReview(in api.IsPersonalSubjectAccessReview, out *api.IsPersonalSubjectAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	return nil
}

func deepCopy_api_LocalResourceAccessReview(in api.LocalResourceAccessReview, out *api.LocalResourceAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if err := deepCopy_api_AuthorizationAttributes(in.Action, &out.Action, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_LocalSubjectAccessReview(in api.LocalSubjectAccessReview, out *api.LocalSubjectAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if err := deepCopy_api_AuthorizationAttributes(in.Action, &out.Action, c); err != nil {
		return err
	}
	out.User = in.User
	if in.Groups != nil {
		out.Groups = make(util.StringSet)
		for key, val := range in.Groups {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.Groups[key] = newVal.(util.Empty)
			}
		}
	} else {
		out.Groups = nil
	}
	return nil
}

func deepCopy_api_Policy(in api.Policy, out *api.Policy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if in.Roles != nil {
		out.Roles = make(map[string]*api.Role)
		for key, val := range in.Roles {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.Roles[key] = newVal.(*api.Role)
			}
		}
	} else {
		out.Roles = nil
	}
	return nil
}

func deepCopy_api_PolicyBinding(in api.PolicyBinding, out *api.PolicyBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.LastModified); err != nil {
		return err
	} else {
		out.LastModified = newVal.(util.Time)
	}
	if newVal, err := c.DeepCopy(in.PolicyRef); err != nil {
		return err
	} else {
		out.PolicyRef = newVal.(pkgapi.ObjectReference)
	}
	if in.RoleBindings != nil {
		out.RoleBindings = make(map[string]*api.RoleBinding)
		for key, val := range in.RoleBindings {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.RoleBindings[key] = newVal.(*api.RoleBinding)
			}
		}
	} else {
		out.RoleBindings = nil
	}
	return nil
}

func deepCopy_api_PolicyBindingList(in api.PolicyBindingList, out *api.PolicyBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]api.PolicyBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_PolicyBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_PolicyList(in api.PolicyList, out *api.PolicyList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]api.Policy, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_Policy(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_PolicyRule(in api.PolicyRule, out *api.PolicyRule, c *conversion.Cloner) error {
	if in.Verbs != nil {
		out.Verbs = make(util.StringSet)
		for key, val := range in.Verbs {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.Verbs[key] = newVal.(util.Empty)
			}
		}
	} else {
		out.Verbs = nil
	}
	if newVal, err := c.DeepCopy(in.AttributeRestrictions); err != nil {
		return err
	} else {
		out.AttributeRestrictions = newVal.(runtime.EmbeddedObject)
	}
	if in.Resources != nil {
		out.Resources = make(util.StringSet)
		for key, val := range in.Resources {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.Resources[key] = newVal.(util.Empty)
			}
		}
	} else {
		out.Resources = nil
	}
	if in.ResourceNames != nil {
		out.ResourceNames = make(util.StringSet)
		for key, val := range in.ResourceNames {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.ResourceNames[key] = newVal.(util.Empty)
			}
		}
	} else {
		out.ResourceNames = nil
	}
	if in.NonResourceURLs != nil {
		out.NonResourceURLs = make(util.StringSet)
		for key, val := range in.NonResourceURLs {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.NonResourceURLs[key] = newVal.(util.Empty)
			}
		}
	} else {
		out.NonResourceURLs = nil
	}
	return nil
}

func deepCopy_api_ResourceAccessReview(in api.ResourceAccessReview, out *api.ResourceAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if err := deepCopy_api_AuthorizationAttributes(in.Action, &out.Action, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_ResourceAccessReviewResponse(in api.ResourceAccessReviewResponse, out *api.ResourceAccessReviewResponse, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	out.Namespace = in.Namespace
	if in.Users != nil {
		out.Users = make(util.StringSet)
		for key, val := range in.Users {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.Users[key] = newVal.(util.Empty)
			}
		}
	} else {
		out.Users = nil
	}
	if in.Groups != nil {
		out.Groups = make(util.StringSet)
		for key, val := range in.Groups {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.Groups[key] = newVal.(util.Empty)
			}
		}
	} else {
		out.Groups = nil
	}
	return nil
}

func deepCopy_api_Role(in api.Role, out *api.Role, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if in.Rules != nil {
		out.Rules = make([]api.PolicyRule, len(in.Rules))
		for i := range in.Rules {
			if err := deepCopy_api_PolicyRule(in.Rules[i], &out.Rules[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Rules = nil
	}
	return nil
}

func deepCopy_api_RoleBinding(in api.RoleBinding, out *api.RoleBinding, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if in.Subjects != nil {
		out.Subjects = make([]pkgapi.ObjectReference, len(in.Subjects))
		for i := range in.Subjects {
			if newVal, err := c.DeepCopy(in.Subjects[i]); err != nil {
				return err
			} else {
				out.Subjects[i] = newVal.(pkgapi.ObjectReference)
			}
		}
	} else {
		out.Subjects = nil
	}
	if newVal, err := c.DeepCopy(in.RoleRef); err != nil {
		return err
	} else {
		out.RoleRef = newVal.(pkgapi.ObjectReference)
	}
	return nil
}

func deepCopy_api_RoleBindingList(in api.RoleBindingList, out *api.RoleBindingList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]api.RoleBinding, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_RoleBinding(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_RoleList(in api.RoleList, out *api.RoleList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]api.Role, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_Role(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_SubjectAccessReview(in api.SubjectAccessReview, out *api.SubjectAccessReview, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if err := deepCopy_api_AuthorizationAttributes(in.Action, &out.Action, c); err != nil {
		return err
	}
	out.User = in.User
	if in.Groups != nil {
		out.Groups = make(util.StringSet)
		for key, val := range in.Groups {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.Groups[key] = newVal.(util.Empty)
			}
		}
	} else {
		out.Groups = nil
	}
	return nil
}

func deepCopy_api_SubjectAccessReviewResponse(in api.SubjectAccessReviewResponse, out *api.SubjectAccessReviewResponse, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	out.Namespace = in.Namespace
	out.Allowed = in.Allowed
	out.Reason = in.Reason
	return nil
}

func deepCopy_api_Build(in buildapi.Build, out *buildapi.Build, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if err := deepCopy_api_BuildSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_api_BuildStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_BuildConfig(in buildapi.BuildConfig, out *buildapi.BuildConfig, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if err := deepCopy_api_BuildConfigSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_api_BuildConfigStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_BuildConfigList(in buildapi.BuildConfigList, out *buildapi.BuildConfigList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]buildapi.BuildConfig, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_BuildConfig(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_BuildConfigSpec(in buildapi.BuildConfigSpec, out *buildapi.BuildConfigSpec, c *conversion.Cloner) error {
	if in.Triggers != nil {
		out.Triggers = make([]buildapi.BuildTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := deepCopy_api_BuildTriggerPolicy(in.Triggers[i], &out.Triggers[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := deepCopy_api_BuildSpec(in.BuildSpec, &out.BuildSpec, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_BuildConfigStatus(in buildapi.BuildConfigStatus, out *buildapi.BuildConfigStatus, c *conversion.Cloner) error {
	out.LastVersion = in.LastVersion
	return nil
}

func deepCopy_api_BuildList(in buildapi.BuildList, out *buildapi.BuildList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]buildapi.Build, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_Build(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_BuildLog(in buildapi.BuildLog, out *buildapi.BuildLog, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	return nil
}

func deepCopy_api_BuildLogOptions(in buildapi.BuildLogOptions, out *buildapi.BuildLogOptions, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	out.Follow = in.Follow
	out.NoWait = in.NoWait
	return nil
}

func deepCopy_api_BuildOutput(in buildapi.BuildOutput, out *buildapi.BuildOutput, c *conversion.Cloner) error {
	if in.To != nil {
		if newVal, err := c.DeepCopy(in.To); err != nil {
			return err
		} else {
			out.To = newVal.(*pkgapi.ObjectReference)
		}
	} else {
		out.To = nil
	}
	if in.PushSecret != nil {
		if newVal, err := c.DeepCopy(in.PushSecret); err != nil {
			return err
		} else {
			out.PushSecret = newVal.(*pkgapi.LocalObjectReference)
		}
	} else {
		out.PushSecret = nil
	}
	return nil
}

func deepCopy_api_BuildRequest(in buildapi.BuildRequest, out *buildapi.BuildRequest, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if in.Revision != nil {
		out.Revision = new(buildapi.SourceRevision)
		if err := deepCopy_api_SourceRevision(*in.Revision, out.Revision, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if in.TriggeredByImage != nil {
		if newVal, err := c.DeepCopy(in.TriggeredByImage); err != nil {
			return err
		} else {
			out.TriggeredByImage = newVal.(*pkgapi.ObjectReference)
		}
	} else {
		out.TriggeredByImage = nil
	}
	if in.From != nil {
		if newVal, err := c.DeepCopy(in.From); err != nil {
			return err
		} else {
			out.From = newVal.(*pkgapi.ObjectReference)
		}
	} else {
		out.From = nil
	}
	if in.LastVersion != nil {
		out.LastVersion = new(int)
		*out.LastVersion = *in.LastVersion
	} else {
		out.LastVersion = nil
	}
	return nil
}

func deepCopy_api_BuildSource(in buildapi.BuildSource, out *buildapi.BuildSource, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.Dockerfile != nil {
		out.Dockerfile = new(string)
		*out.Dockerfile = *in.Dockerfile
	} else {
		out.Dockerfile = nil
	}
	if in.Git != nil {
		out.Git = new(buildapi.GitBuildSource)
		if err := deepCopy_api_GitBuildSource(*in.Git, out.Git, c); err != nil {
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
			out.SourceSecret = newVal.(*pkgapi.LocalObjectReference)
		}
	} else {
		out.SourceSecret = nil
	}
	return nil
}

func deepCopy_api_BuildSpec(in buildapi.BuildSpec, out *buildapi.BuildSpec, c *conversion.Cloner) error {
	out.ServiceAccount = in.ServiceAccount
	if err := deepCopy_api_BuildSource(in.Source, &out.Source, c); err != nil {
		return err
	}
	if in.Revision != nil {
		out.Revision = new(buildapi.SourceRevision)
		if err := deepCopy_api_SourceRevision(*in.Revision, out.Revision, c); err != nil {
			return err
		}
	} else {
		out.Revision = nil
	}
	if err := deepCopy_api_BuildStrategy(in.Strategy, &out.Strategy, c); err != nil {
		return err
	}
	if err := deepCopy_api_BuildOutput(in.Output, &out.Output, c); err != nil {
		return err
	}
	if newVal, err := c.DeepCopy(in.Resources); err != nil {
		return err
	} else {
		out.Resources = newVal.(pkgapi.ResourceRequirements)
	}
	return nil
}

func deepCopy_api_BuildStatus(in buildapi.BuildStatus, out *buildapi.BuildStatus, c *conversion.Cloner) error {
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
			out.Config = newVal.(*pkgapi.ObjectReference)
		}
	} else {
		out.Config = nil
	}
	return nil
}

func deepCopy_api_BuildStrategy(in buildapi.BuildStrategy, out *buildapi.BuildStrategy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.DockerStrategy != nil {
		out.DockerStrategy = new(buildapi.DockerBuildStrategy)
		if err := deepCopy_api_DockerBuildStrategy(*in.DockerStrategy, out.DockerStrategy, c); err != nil {
			return err
		}
	} else {
		out.DockerStrategy = nil
	}
	if in.SourceStrategy != nil {
		out.SourceStrategy = new(buildapi.SourceBuildStrategy)
		if err := deepCopy_api_SourceBuildStrategy(*in.SourceStrategy, out.SourceStrategy, c); err != nil {
			return err
		}
	} else {
		out.SourceStrategy = nil
	}
	if in.CustomStrategy != nil {
		out.CustomStrategy = new(buildapi.CustomBuildStrategy)
		if err := deepCopy_api_CustomBuildStrategy(*in.CustomStrategy, out.CustomStrategy, c); err != nil {
			return err
		}
	} else {
		out.CustomStrategy = nil
	}
	return nil
}

func deepCopy_api_BuildTriggerPolicy(in buildapi.BuildTriggerPolicy, out *buildapi.BuildTriggerPolicy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.GitHubWebHook != nil {
		out.GitHubWebHook = new(buildapi.WebHookTrigger)
		if err := deepCopy_api_WebHookTrigger(*in.GitHubWebHook, out.GitHubWebHook, c); err != nil {
			return err
		}
	} else {
		out.GitHubWebHook = nil
	}
	if in.GenericWebHook != nil {
		out.GenericWebHook = new(buildapi.WebHookTrigger)
		if err := deepCopy_api_WebHookTrigger(*in.GenericWebHook, out.GenericWebHook, c); err != nil {
			return err
		}
	} else {
		out.GenericWebHook = nil
	}
	if in.ImageChange != nil {
		out.ImageChange = new(buildapi.ImageChangeTrigger)
		if err := deepCopy_api_ImageChangeTrigger(*in.ImageChange, out.ImageChange, c); err != nil {
			return err
		}
	} else {
		out.ImageChange = nil
	}
	return nil
}

func deepCopy_api_CustomBuildStrategy(in buildapi.CustomBuildStrategy, out *buildapi.CustomBuildStrategy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(pkgapi.ObjectReference)
	}
	if in.PullSecret != nil {
		if newVal, err := c.DeepCopy(in.PullSecret); err != nil {
			return err
		} else {
			out.PullSecret = newVal.(*pkgapi.LocalObjectReference)
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(pkgapi.EnvVar)
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
			if err := deepCopy_api_SecretSpec(in.Secrets[i], &out.Secrets[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Secrets = nil
	}
	return nil
}

func deepCopy_api_DockerBuildStrategy(in buildapi.DockerBuildStrategy, out *buildapi.DockerBuildStrategy, c *conversion.Cloner) error {
	if in.From != nil {
		if newVal, err := c.DeepCopy(in.From); err != nil {
			return err
		} else {
			out.From = newVal.(*pkgapi.ObjectReference)
		}
	} else {
		out.From = nil
	}
	if in.PullSecret != nil {
		if newVal, err := c.DeepCopy(in.PullSecret); err != nil {
			return err
		} else {
			out.PullSecret = newVal.(*pkgapi.LocalObjectReference)
		}
	} else {
		out.PullSecret = nil
	}
	out.NoCache = in.NoCache
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(pkgapi.EnvVar)
			}
		}
	} else {
		out.Env = nil
	}
	out.ForcePull = in.ForcePull
	return nil
}

func deepCopy_api_GitBuildSource(in buildapi.GitBuildSource, out *buildapi.GitBuildSource, c *conversion.Cloner) error {
	out.URI = in.URI
	out.Ref = in.Ref
	out.HTTPProxy = in.HTTPProxy
	out.HTTPSProxy = in.HTTPSProxy
	return nil
}

func deepCopy_api_GitSourceRevision(in buildapi.GitSourceRevision, out *buildapi.GitSourceRevision, c *conversion.Cloner) error {
	out.Commit = in.Commit
	if err := deepCopy_api_SourceControlUser(in.Author, &out.Author, c); err != nil {
		return err
	}
	if err := deepCopy_api_SourceControlUser(in.Committer, &out.Committer, c); err != nil {
		return err
	}
	out.Message = in.Message
	return nil
}

func deepCopy_api_ImageChangeTrigger(in buildapi.ImageChangeTrigger, out *buildapi.ImageChangeTrigger, c *conversion.Cloner) error {
	out.LastTriggeredImageID = in.LastTriggeredImageID
	if in.From != nil {
		if newVal, err := c.DeepCopy(in.From); err != nil {
			return err
		} else {
			out.From = newVal.(*pkgapi.ObjectReference)
		}
	} else {
		out.From = nil
	}
	return nil
}

func deepCopy_api_SecretSpec(in buildapi.SecretSpec, out *buildapi.SecretSpec, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.SecretSource); err != nil {
		return err
	} else {
		out.SecretSource = newVal.(pkgapi.LocalObjectReference)
	}
	out.MountPath = in.MountPath
	return nil
}

func deepCopy_api_SourceBuildStrategy(in buildapi.SourceBuildStrategy, out *buildapi.SourceBuildStrategy, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(pkgapi.ObjectReference)
	}
	if in.PullSecret != nil {
		if newVal, err := c.DeepCopy(in.PullSecret); err != nil {
			return err
		} else {
			out.PullSecret = newVal.(*pkgapi.LocalObjectReference)
		}
	} else {
		out.PullSecret = nil
	}
	if in.Env != nil {
		out.Env = make([]pkgapi.EnvVar, len(in.Env))
		for i := range in.Env {
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(pkgapi.EnvVar)
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

func deepCopy_api_SourceControlUser(in buildapi.SourceControlUser, out *buildapi.SourceControlUser, c *conversion.Cloner) error {
	out.Name = in.Name
	out.Email = in.Email
	return nil
}

func deepCopy_api_SourceRevision(in buildapi.SourceRevision, out *buildapi.SourceRevision, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.Git != nil {
		out.Git = new(buildapi.GitSourceRevision)
		if err := deepCopy_api_GitSourceRevision(*in.Git, out.Git, c); err != nil {
			return err
		}
	} else {
		out.Git = nil
	}
	return nil
}

func deepCopy_api_WebHookTrigger(in buildapi.WebHookTrigger, out *buildapi.WebHookTrigger, c *conversion.Cloner) error {
	out.Secret = in.Secret
	return nil
}

func deepCopy_api_CustomDeploymentStrategyParams(in deployapi.CustomDeploymentStrategyParams, out *deployapi.CustomDeploymentStrategyParams, c *conversion.Cloner) error {
	out.Image = in.Image
	if in.Environment != nil {
		out.Environment = make([]pkgapi.EnvVar, len(in.Environment))
		for i := range in.Environment {
			if newVal, err := c.DeepCopy(in.Environment[i]); err != nil {
				return err
			} else {
				out.Environment[i] = newVal.(pkgapi.EnvVar)
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

func deepCopy_api_DeploymentCause(in deployapi.DeploymentCause, out *deployapi.DeploymentCause, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.ImageTrigger != nil {
		out.ImageTrigger = new(deployapi.DeploymentCauseImageTrigger)
		if err := deepCopy_api_DeploymentCauseImageTrigger(*in.ImageTrigger, out.ImageTrigger, c); err != nil {
			return err
		}
	} else {
		out.ImageTrigger = nil
	}
	return nil
}

func deepCopy_api_DeploymentCauseImageTrigger(in deployapi.DeploymentCauseImageTrigger, out *deployapi.DeploymentCauseImageTrigger, c *conversion.Cloner) error {
	out.RepositoryName = in.RepositoryName
	out.Tag = in.Tag
	return nil
}

func deepCopy_api_DeploymentConfig(in deployapi.DeploymentConfig, out *deployapi.DeploymentConfig, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if in.Triggers != nil {
		out.Triggers = make([]deployapi.DeploymentTriggerPolicy, len(in.Triggers))
		for i := range in.Triggers {
			if err := deepCopy_api_DeploymentTriggerPolicy(in.Triggers[i], &out.Triggers[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Triggers = nil
	}
	if err := deepCopy_api_DeploymentTemplate(in.Template, &out.Template, c); err != nil {
		return err
	}
	out.LatestVersion = in.LatestVersion
	if in.Details != nil {
		out.Details = new(deployapi.DeploymentDetails)
		if err := deepCopy_api_DeploymentDetails(*in.Details, out.Details, c); err != nil {
			return err
		}
	} else {
		out.Details = nil
	}
	return nil
}

func deepCopy_api_DeploymentConfigList(in deployapi.DeploymentConfigList, out *deployapi.DeploymentConfigList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]deployapi.DeploymentConfig, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_DeploymentConfig(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_DeploymentConfigRollback(in deployapi.DeploymentConfigRollback, out *deployapi.DeploymentConfigRollback, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if err := deepCopy_api_DeploymentConfigRollbackSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_DeploymentConfigRollbackSpec(in deployapi.DeploymentConfigRollbackSpec, out *deployapi.DeploymentConfigRollbackSpec, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(pkgapi.ObjectReference)
	}
	out.IncludeTriggers = in.IncludeTriggers
	out.IncludeTemplate = in.IncludeTemplate
	out.IncludeReplicationMeta = in.IncludeReplicationMeta
	out.IncludeStrategy = in.IncludeStrategy
	return nil
}

func deepCopy_api_DeploymentDetails(in deployapi.DeploymentDetails, out *deployapi.DeploymentDetails, c *conversion.Cloner) error {
	out.Message = in.Message
	if in.Causes != nil {
		out.Causes = make([]*deployapi.DeploymentCause, len(in.Causes))
		for i := range in.Causes {
			if newVal, err := c.DeepCopy(in.Causes[i]); err != nil {
				return err
			} else {
				out.Causes[i] = newVal.(*deployapi.DeploymentCause)
			}
		}
	} else {
		out.Causes = nil
	}
	return nil
}

func deepCopy_api_DeploymentStrategy(in deployapi.DeploymentStrategy, out *deployapi.DeploymentStrategy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.CustomParams != nil {
		out.CustomParams = new(deployapi.CustomDeploymentStrategyParams)
		if err := deepCopy_api_CustomDeploymentStrategyParams(*in.CustomParams, out.CustomParams, c); err != nil {
			return err
		}
	} else {
		out.CustomParams = nil
	}
	if in.RecreateParams != nil {
		out.RecreateParams = new(deployapi.RecreateDeploymentStrategyParams)
		if err := deepCopy_api_RecreateDeploymentStrategyParams(*in.RecreateParams, out.RecreateParams, c); err != nil {
			return err
		}
	} else {
		out.RecreateParams = nil
	}
	if in.RollingParams != nil {
		out.RollingParams = new(deployapi.RollingDeploymentStrategyParams)
		if err := deepCopy_api_RollingDeploymentStrategyParams(*in.RollingParams, out.RollingParams, c); err != nil {
			return err
		}
	} else {
		out.RollingParams = nil
	}
	if newVal, err := c.DeepCopy(in.Resources); err != nil {
		return err
	} else {
		out.Resources = newVal.(pkgapi.ResourceRequirements)
	}
	return nil
}

func deepCopy_api_DeploymentTemplate(in deployapi.DeploymentTemplate, out *deployapi.DeploymentTemplate, c *conversion.Cloner) error {
	if err := deepCopy_api_DeploymentStrategy(in.Strategy, &out.Strategy, c); err != nil {
		return err
	}
	if newVal, err := c.DeepCopy(in.ControllerTemplate); err != nil {
		return err
	} else {
		out.ControllerTemplate = newVal.(pkgapi.ReplicationControllerSpec)
	}
	return nil
}

func deepCopy_api_DeploymentTriggerImageChangeParams(in deployapi.DeploymentTriggerImageChangeParams, out *deployapi.DeploymentTriggerImageChangeParams, c *conversion.Cloner) error {
	out.Automatic = in.Automatic
	if in.ContainerNames != nil {
		out.ContainerNames = make([]string, len(in.ContainerNames))
		for i := range in.ContainerNames {
			out.ContainerNames[i] = in.ContainerNames[i]
		}
	} else {
		out.ContainerNames = nil
	}
	out.RepositoryName = in.RepositoryName
	if newVal, err := c.DeepCopy(in.From); err != nil {
		return err
	} else {
		out.From = newVal.(pkgapi.ObjectReference)
	}
	out.Tag = in.Tag
	out.LastTriggeredImage = in.LastTriggeredImage
	return nil
}

func deepCopy_api_DeploymentTriggerPolicy(in deployapi.DeploymentTriggerPolicy, out *deployapi.DeploymentTriggerPolicy, c *conversion.Cloner) error {
	out.Type = in.Type
	if in.ImageChangeParams != nil {
		out.ImageChangeParams = new(deployapi.DeploymentTriggerImageChangeParams)
		if err := deepCopy_api_DeploymentTriggerImageChangeParams(*in.ImageChangeParams, out.ImageChangeParams, c); err != nil {
			return err
		}
	} else {
		out.ImageChangeParams = nil
	}
	return nil
}

func deepCopy_api_ExecNewPodHook(in deployapi.ExecNewPodHook, out *deployapi.ExecNewPodHook, c *conversion.Cloner) error {
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
			if newVal, err := c.DeepCopy(in.Env[i]); err != nil {
				return err
			} else {
				out.Env[i] = newVal.(pkgapi.EnvVar)
			}
		}
	} else {
		out.Env = nil
	}
	out.ContainerName = in.ContainerName
	return nil
}

func deepCopy_api_LifecycleHook(in deployapi.LifecycleHook, out *deployapi.LifecycleHook, c *conversion.Cloner) error {
	out.FailurePolicy = in.FailurePolicy
	if in.ExecNewPod != nil {
		out.ExecNewPod = new(deployapi.ExecNewPodHook)
		if err := deepCopy_api_ExecNewPodHook(*in.ExecNewPod, out.ExecNewPod, c); err != nil {
			return err
		}
	} else {
		out.ExecNewPod = nil
	}
	return nil
}

func deepCopy_api_RecreateDeploymentStrategyParams(in deployapi.RecreateDeploymentStrategyParams, out *deployapi.RecreateDeploymentStrategyParams, c *conversion.Cloner) error {
	if in.Pre != nil {
		out.Pre = new(deployapi.LifecycleHook)
		if err := deepCopy_api_LifecycleHook(*in.Pre, out.Pre, c); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapi.LifecycleHook)
		if err := deepCopy_api_LifecycleHook(*in.Post, out.Post, c); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func deepCopy_api_RollingDeploymentStrategyParams(in deployapi.RollingDeploymentStrategyParams, out *deployapi.RollingDeploymentStrategyParams, c *conversion.Cloner) error {
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
	if newVal, err := c.DeepCopy(in.MaxUnavailable); err != nil {
		return err
	} else {
		out.MaxUnavailable = newVal.(util.IntOrString)
	}
	if newVal, err := c.DeepCopy(in.MaxSurge); err != nil {
		return err
	} else {
		out.MaxSurge = newVal.(util.IntOrString)
	}
	if in.UpdatePercent != nil {
		out.UpdatePercent = new(int)
		*out.UpdatePercent = *in.UpdatePercent
	} else {
		out.UpdatePercent = nil
	}
	if in.Pre != nil {
		out.Pre = new(deployapi.LifecycleHook)
		if err := deepCopy_api_LifecycleHook(*in.Pre, out.Pre, c); err != nil {
			return err
		}
	} else {
		out.Pre = nil
	}
	if in.Post != nil {
		out.Post = new(deployapi.LifecycleHook)
		if err := deepCopy_api_LifecycleHook(*in.Post, out.Post, c); err != nil {
			return err
		}
	} else {
		out.Post = nil
	}
	return nil
}

func deepCopy_api_DockerConfig(in imageapi.DockerConfig, out *imageapi.DockerConfig, c *conversion.Cloner) error {
	out.Hostname = in.Hostname
	out.Domainname = in.Domainname
	out.User = in.User
	out.Memory = in.Memory
	out.MemorySwap = in.MemorySwap
	out.CPUShares = in.CPUShares
	out.CPUSet = in.CPUSet
	out.AttachStdin = in.AttachStdin
	out.AttachStdout = in.AttachStdout
	out.AttachStderr = in.AttachStderr
	if in.PortSpecs != nil {
		out.PortSpecs = make([]string, len(in.PortSpecs))
		for i := range in.PortSpecs {
			out.PortSpecs[i] = in.PortSpecs[i]
		}
	} else {
		out.PortSpecs = nil
	}
	if in.ExposedPorts != nil {
		out.ExposedPorts = make(map[string]struct{})
		for key, val := range in.ExposedPorts {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.ExposedPorts[key] = newVal.(struct{})
			}
		}
	} else {
		out.ExposedPorts = nil
	}
	out.Tty = in.Tty
	out.OpenStdin = in.OpenStdin
	out.StdinOnce = in.StdinOnce
	if in.Env != nil {
		out.Env = make([]string, len(in.Env))
		for i := range in.Env {
			out.Env[i] = in.Env[i]
		}
	} else {
		out.Env = nil
	}
	if in.Cmd != nil {
		out.Cmd = make([]string, len(in.Cmd))
		for i := range in.Cmd {
			out.Cmd[i] = in.Cmd[i]
		}
	} else {
		out.Cmd = nil
	}
	if in.DNS != nil {
		out.DNS = make([]string, len(in.DNS))
		for i := range in.DNS {
			out.DNS[i] = in.DNS[i]
		}
	} else {
		out.DNS = nil
	}
	out.Image = in.Image
	if in.Volumes != nil {
		out.Volumes = make(map[string]struct{})
		for key, val := range in.Volumes {
			if newVal, err := c.DeepCopy(val); err != nil {
				return err
			} else {
				out.Volumes[key] = newVal.(struct{})
			}
		}
	} else {
		out.Volumes = nil
	}
	out.VolumesFrom = in.VolumesFrom
	out.WorkingDir = in.WorkingDir
	if in.Entrypoint != nil {
		out.Entrypoint = make([]string, len(in.Entrypoint))
		for i := range in.Entrypoint {
			out.Entrypoint[i] = in.Entrypoint[i]
		}
	} else {
		out.Entrypoint = nil
	}
	out.NetworkDisabled = in.NetworkDisabled
	if in.SecurityOpts != nil {
		out.SecurityOpts = make([]string, len(in.SecurityOpts))
		for i := range in.SecurityOpts {
			out.SecurityOpts[i] = in.SecurityOpts[i]
		}
	} else {
		out.SecurityOpts = nil
	}
	if in.OnBuild != nil {
		out.OnBuild = make([]string, len(in.OnBuild))
		for i := range in.OnBuild {
			out.OnBuild[i] = in.OnBuild[i]
		}
	} else {
		out.OnBuild = nil
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

func deepCopy_api_DockerImage(in imageapi.DockerImage, out *imageapi.DockerImage, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	out.ID = in.ID
	out.Parent = in.Parent
	out.Comment = in.Comment
	if newVal, err := c.DeepCopy(in.Created); err != nil {
		return err
	} else {
		out.Created = newVal.(util.Time)
	}
	out.Container = in.Container
	if err := deepCopy_api_DockerConfig(in.ContainerConfig, &out.ContainerConfig, c); err != nil {
		return err
	}
	out.DockerVersion = in.DockerVersion
	out.Author = in.Author
	if in.Config != nil {
		out.Config = new(imageapi.DockerConfig)
		if err := deepCopy_api_DockerConfig(*in.Config, out.Config, c); err != nil {
			return err
		}
	} else {
		out.Config = nil
	}
	out.Architecture = in.Architecture
	out.Size = in.Size
	return nil
}

func deepCopy_api_Image(in imageapi.Image, out *imageapi.Image, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	out.DockerImageReference = in.DockerImageReference
	if err := deepCopy_api_DockerImage(in.DockerImageMetadata, &out.DockerImageMetadata, c); err != nil {
		return err
	}
	out.DockerImageMetadataVersion = in.DockerImageMetadataVersion
	out.DockerImageManifest = in.DockerImageManifest
	return nil
}

func deepCopy_api_ImageList(in imageapi.ImageList, out *imageapi.ImageList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]imageapi.Image, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_Image(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_ImageStream(in imageapi.ImageStream, out *imageapi.ImageStream, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if err := deepCopy_api_ImageStreamSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_api_ImageStreamStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_ImageStreamImage(in imageapi.ImageStreamImage, out *imageapi.ImageStreamImage, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if err := deepCopy_api_Image(in.Image, &out.Image, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_ImageStreamList(in imageapi.ImageStreamList, out *imageapi.ImageStreamList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]imageapi.ImageStream, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_ImageStream(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_ImageStreamMapping(in imageapi.ImageStreamMapping, out *imageapi.ImageStreamMapping, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	out.DockerImageRepository = in.DockerImageRepository
	if err := deepCopy_api_Image(in.Image, &out.Image, c); err != nil {
		return err
	}
	out.Tag = in.Tag
	return nil
}

func deepCopy_api_ImageStreamSpec(in imageapi.ImageStreamSpec, out *imageapi.ImageStreamSpec, c *conversion.Cloner) error {
	out.DockerImageRepository = in.DockerImageRepository
	if in.Tags != nil {
		out.Tags = make(map[string]imageapi.TagReference)
		for key, val := range in.Tags {
			newVal := new(imageapi.TagReference)
			if err := deepCopy_api_TagReference(val, newVal, c); err != nil {
				return err
			}
			out.Tags[key] = *newVal
		}
	} else {
		out.Tags = nil
	}
	return nil
}

func deepCopy_api_ImageStreamStatus(in imageapi.ImageStreamStatus, out *imageapi.ImageStreamStatus, c *conversion.Cloner) error {
	out.DockerImageRepository = in.DockerImageRepository
	if in.Tags != nil {
		out.Tags = make(map[string]imageapi.TagEventList)
		for key, val := range in.Tags {
			newVal := new(imageapi.TagEventList)
			if err := deepCopy_api_TagEventList(val, newVal, c); err != nil {
				return err
			}
			out.Tags[key] = *newVal
		}
	} else {
		out.Tags = nil
	}
	return nil
}

func deepCopy_api_ImageStreamTag(in imageapi.ImageStreamTag, out *imageapi.ImageStreamTag, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if err := deepCopy_api_Image(in.Image, &out.Image, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_TagEvent(in imageapi.TagEvent, out *imageapi.TagEvent, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.Created); err != nil {
		return err
	} else {
		out.Created = newVal.(util.Time)
	}
	out.DockerImageReference = in.DockerImageReference
	out.Image = in.Image
	return nil
}

func deepCopy_api_TagEventList(in imageapi.TagEventList, out *imageapi.TagEventList, c *conversion.Cloner) error {
	if in.Items != nil {
		out.Items = make([]imageapi.TagEvent, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_TagEvent(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_TagReference(in imageapi.TagReference, out *imageapi.TagReference, c *conversion.Cloner) error {
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
			out.From = newVal.(*pkgapi.ObjectReference)
		}
	} else {
		out.From = nil
	}
	return nil
}

func deepCopy_api_OAuthAccessToken(in oauthapi.OAuthAccessToken, out *oauthapi.OAuthAccessToken, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
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

func deepCopy_api_OAuthAccessTokenList(in oauthapi.OAuthAccessTokenList, out *oauthapi.OAuthAccessTokenList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthAccessToken, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_OAuthAccessToken(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_OAuthAuthorizeToken(in oauthapi.OAuthAuthorizeToken, out *oauthapi.OAuthAuthorizeToken, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
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

func deepCopy_api_OAuthAuthorizeTokenList(in oauthapi.OAuthAuthorizeTokenList, out *oauthapi.OAuthAuthorizeTokenList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthAuthorizeToken, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_OAuthAuthorizeToken(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_OAuthClient(in oauthapi.OAuthClient, out *oauthapi.OAuthClient, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
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

func deepCopy_api_OAuthClientAuthorization(in oauthapi.OAuthClientAuthorization, out *oauthapi.OAuthClientAuthorization, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
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

func deepCopy_api_OAuthClientAuthorizationList(in oauthapi.OAuthClientAuthorizationList, out *oauthapi.OAuthClientAuthorizationList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthClientAuthorization, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_OAuthClientAuthorization(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_OAuthClientList(in oauthapi.OAuthClientList, out *oauthapi.OAuthClientList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]oauthapi.OAuthClient, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_OAuthClient(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_Project(in projectapi.Project, out *projectapi.Project, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if err := deepCopy_api_ProjectSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_api_ProjectStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_ProjectList(in projectapi.ProjectList, out *projectapi.ProjectList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]projectapi.Project, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_Project(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_ProjectRequest(in projectapi.ProjectRequest, out *projectapi.ProjectRequest, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	return nil
}

func deepCopy_api_ProjectSpec(in projectapi.ProjectSpec, out *projectapi.ProjectSpec, c *conversion.Cloner) error {
	if in.Finalizers != nil {
		out.Finalizers = make([]pkgapi.FinalizerName, len(in.Finalizers))
		for i := range in.Finalizers {
			out.Finalizers[i] = in.Finalizers[i]
		}
	} else {
		out.Finalizers = nil
	}
	return nil
}

func deepCopy_api_ProjectStatus(in projectapi.ProjectStatus, out *projectapi.ProjectStatus, c *conversion.Cloner) error {
	out.Phase = in.Phase
	return nil
}

func deepCopy_api_Route(in routeapi.Route, out *routeapi.Route, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if err := deepCopy_api_RouteSpec(in.Spec, &out.Spec, c); err != nil {
		return err
	}
	if err := deepCopy_api_RouteStatus(in.Status, &out.Status, c); err != nil {
		return err
	}
	return nil
}

func deepCopy_api_RouteList(in routeapi.RouteList, out *routeapi.RouteList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]routeapi.Route, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_Route(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_RouteSpec(in routeapi.RouteSpec, out *routeapi.RouteSpec, c *conversion.Cloner) error {
	out.Host = in.Host
	out.Path = in.Path
	if newVal, err := c.DeepCopy(in.To); err != nil {
		return err
	} else {
		out.To = newVal.(pkgapi.ObjectReference)
	}
	if in.TLS != nil {
		out.TLS = new(routeapi.TLSConfig)
		if err := deepCopy_api_TLSConfig(*in.TLS, out.TLS, c); err != nil {
			return err
		}
	} else {
		out.TLS = nil
	}
	return nil
}

func deepCopy_api_RouteStatus(in routeapi.RouteStatus, out *routeapi.RouteStatus, c *conversion.Cloner) error {
	return nil
}

func deepCopy_api_TLSConfig(in routeapi.TLSConfig, out *routeapi.TLSConfig, c *conversion.Cloner) error {
	out.Termination = in.Termination
	out.Certificate = in.Certificate
	out.Key = in.Key
	out.CACertificate = in.CACertificate
	out.DestinationCACertificate = in.DestinationCACertificate
	return nil
}

func deepCopy_api_ClusterNetwork(in sdnapi.ClusterNetwork, out *sdnapi.ClusterNetwork, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	out.Network = in.Network
	out.HostSubnetLength = in.HostSubnetLength
	out.ServiceNetwork = in.ServiceNetwork
	return nil
}

func deepCopy_api_ClusterNetworkList(in sdnapi.ClusterNetworkList, out *sdnapi.ClusterNetworkList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.ClusterNetwork, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_ClusterNetwork(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_HostSubnet(in sdnapi.HostSubnet, out *sdnapi.HostSubnet, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	out.Host = in.Host
	out.HostIP = in.HostIP
	out.Subnet = in.Subnet
	return nil
}

func deepCopy_api_HostSubnetList(in sdnapi.HostSubnetList, out *sdnapi.HostSubnetList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.HostSubnet, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_HostSubnet(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_NetNamespace(in sdnapi.NetNamespace, out *sdnapi.NetNamespace, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	out.NetName = in.NetName
	out.NetID = in.NetID
	return nil
}

func deepCopy_api_NetNamespaceList(in sdnapi.NetNamespaceList, out *sdnapi.NetNamespaceList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]sdnapi.NetNamespace, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_NetNamespace(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_Parameter(in templateapi.Parameter, out *templateapi.Parameter, c *conversion.Cloner) error {
	out.Name = in.Name
	out.DisplayName = in.DisplayName
	out.Description = in.Description
	out.Value = in.Value
	out.Generate = in.Generate
	out.From = in.From
	out.Required = in.Required
	return nil
}

func deepCopy_api_Template(in templateapi.Template, out *templateapi.Template, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if in.Parameters != nil {
		out.Parameters = make([]templateapi.Parameter, len(in.Parameters))
		for i := range in.Parameters {
			if err := deepCopy_api_Parameter(in.Parameters[i], &out.Parameters[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Parameters = nil
	}
	if in.Objects != nil {
		out.Objects = make([]runtime.Object, len(in.Objects))
		for i := range in.Objects {
			if newVal, err := c.DeepCopy(in.Objects[i]); err != nil {
				return err
			} else {
				out.Objects[i] = newVal.(runtime.Object)
			}
		}
	} else {
		out.Objects = nil
	}
	if in.ObjectLabels != nil {
		out.ObjectLabels = make(map[string]string)
		for key, val := range in.ObjectLabels {
			out.ObjectLabels[key] = val
		}
	} else {
		out.ObjectLabels = nil
	}
	return nil
}

func deepCopy_api_TemplateList(in templateapi.TemplateList, out *templateapi.TemplateList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]templateapi.Template, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_Template(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_Group(in userapi.Group, out *userapi.Group, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
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

func deepCopy_api_GroupList(in userapi.GroupList, out *userapi.GroupList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]userapi.Group, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_Group(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_Identity(in userapi.Identity, out *userapi.Identity, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	out.ProviderName = in.ProviderName
	out.ProviderUserName = in.ProviderUserName
	if newVal, err := c.DeepCopy(in.User); err != nil {
		return err
	} else {
		out.User = newVal.(pkgapi.ObjectReference)
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

func deepCopy_api_IdentityList(in userapi.IdentityList, out *userapi.IdentityList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]userapi.Identity, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_Identity(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func deepCopy_api_User(in userapi.User, out *userapi.User, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
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

func deepCopy_api_UserIdentityMapping(in userapi.UserIdentityMapping, out *userapi.UserIdentityMapping, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ObjectMeta); err != nil {
		return err
	} else {
		out.ObjectMeta = newVal.(pkgapi.ObjectMeta)
	}
	if newVal, err := c.DeepCopy(in.Identity); err != nil {
		return err
	} else {
		out.Identity = newVal.(pkgapi.ObjectReference)
	}
	if newVal, err := c.DeepCopy(in.User); err != nil {
		return err
	} else {
		out.User = newVal.(pkgapi.ObjectReference)
	}
	return nil
}

func deepCopy_api_UserList(in userapi.UserList, out *userapi.UserList, c *conversion.Cloner) error {
	if newVal, err := c.DeepCopy(in.TypeMeta); err != nil {
		return err
	} else {
		out.TypeMeta = newVal.(pkgapi.TypeMeta)
	}
	if newVal, err := c.DeepCopy(in.ListMeta); err != nil {
		return err
	} else {
		out.ListMeta = newVal.(pkgapi.ListMeta)
	}
	if in.Items != nil {
		out.Items = make([]userapi.User, len(in.Items))
		for i := range in.Items {
			if err := deepCopy_api_User(in.Items[i], &out.Items[i], c); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

func init() {
	err := pkgapi.Scheme.AddGeneratedDeepCopyFuncs(
		deepCopy_api_AuthorizationAttributes,
		deepCopy_api_ClusterPolicy,
		deepCopy_api_ClusterPolicyBinding,
		deepCopy_api_ClusterPolicyBindingList,
		deepCopy_api_ClusterPolicyList,
		deepCopy_api_ClusterRole,
		deepCopy_api_ClusterRoleBinding,
		deepCopy_api_ClusterRoleBindingList,
		deepCopy_api_ClusterRoleList,
		deepCopy_api_IsPersonalSubjectAccessReview,
		deepCopy_api_LocalResourceAccessReview,
		deepCopy_api_LocalSubjectAccessReview,
		deepCopy_api_Policy,
		deepCopy_api_PolicyBinding,
		deepCopy_api_PolicyBindingList,
		deepCopy_api_PolicyList,
		deepCopy_api_PolicyRule,
		deepCopy_api_ResourceAccessReview,
		deepCopy_api_ResourceAccessReviewResponse,
		deepCopy_api_Role,
		deepCopy_api_RoleBinding,
		deepCopy_api_RoleBindingList,
		deepCopy_api_RoleList,
		deepCopy_api_SubjectAccessReview,
		deepCopy_api_SubjectAccessReviewResponse,
		deepCopy_api_Build,
		deepCopy_api_BuildConfig,
		deepCopy_api_BuildConfigList,
		deepCopy_api_BuildConfigSpec,
		deepCopy_api_BuildConfigStatus,
		deepCopy_api_BuildList,
		deepCopy_api_BuildLog,
		deepCopy_api_BuildLogOptions,
		deepCopy_api_BuildOutput,
		deepCopy_api_BuildRequest,
		deepCopy_api_BuildSource,
		deepCopy_api_BuildSpec,
		deepCopy_api_BuildStatus,
		deepCopy_api_BuildStrategy,
		deepCopy_api_BuildTriggerPolicy,
		deepCopy_api_CustomBuildStrategy,
		deepCopy_api_DockerBuildStrategy,
		deepCopy_api_GitBuildSource,
		deepCopy_api_GitSourceRevision,
		deepCopy_api_ImageChangeTrigger,
		deepCopy_api_SecretSpec,
		deepCopy_api_SourceBuildStrategy,
		deepCopy_api_SourceControlUser,
		deepCopy_api_SourceRevision,
		deepCopy_api_WebHookTrigger,
		deepCopy_api_CustomDeploymentStrategyParams,
		deepCopy_api_DeploymentCause,
		deepCopy_api_DeploymentCauseImageTrigger,
		deepCopy_api_DeploymentConfig,
		deepCopy_api_DeploymentConfigList,
		deepCopy_api_DeploymentConfigRollback,
		deepCopy_api_DeploymentConfigRollbackSpec,
		deepCopy_api_DeploymentDetails,
		deepCopy_api_DeploymentStrategy,
		deepCopy_api_DeploymentTemplate,
		deepCopy_api_DeploymentTriggerImageChangeParams,
		deepCopy_api_DeploymentTriggerPolicy,
		deepCopy_api_ExecNewPodHook,
		deepCopy_api_LifecycleHook,
		deepCopy_api_RecreateDeploymentStrategyParams,
		deepCopy_api_RollingDeploymentStrategyParams,
		deepCopy_api_DockerConfig,
		deepCopy_api_DockerImage,
		deepCopy_api_Image,
		deepCopy_api_ImageList,
		deepCopy_api_ImageStream,
		deepCopy_api_ImageStreamImage,
		deepCopy_api_ImageStreamList,
		deepCopy_api_ImageStreamMapping,
		deepCopy_api_ImageStreamSpec,
		deepCopy_api_ImageStreamStatus,
		deepCopy_api_ImageStreamTag,
		deepCopy_api_TagEvent,
		deepCopy_api_TagEventList,
		deepCopy_api_TagReference,
		deepCopy_api_OAuthAccessToken,
		deepCopy_api_OAuthAccessTokenList,
		deepCopy_api_OAuthAuthorizeToken,
		deepCopy_api_OAuthAuthorizeTokenList,
		deepCopy_api_OAuthClient,
		deepCopy_api_OAuthClientAuthorization,
		deepCopy_api_OAuthClientAuthorizationList,
		deepCopy_api_OAuthClientList,
		deepCopy_api_Project,
		deepCopy_api_ProjectList,
		deepCopy_api_ProjectRequest,
		deepCopy_api_ProjectSpec,
		deepCopy_api_ProjectStatus,
		deepCopy_api_Route,
		deepCopy_api_RouteList,
		deepCopy_api_RouteSpec,
		deepCopy_api_RouteStatus,
		deepCopy_api_TLSConfig,
		deepCopy_api_ClusterNetwork,
		deepCopy_api_ClusterNetworkList,
		deepCopy_api_HostSubnet,
		deepCopy_api_HostSubnetList,
		deepCopy_api_NetNamespace,
		deepCopy_api_NetNamespaceList,
		deepCopy_api_Parameter,
		deepCopy_api_Template,
		deepCopy_api_TemplateList,
		deepCopy_api_Group,
		deepCopy_api_GroupList,
		deepCopy_api_Identity,
		deepCopy_api_IdentityList,
		deepCopy_api_User,
		deepCopy_api_UserIdentityMapping,
		deepCopy_api_UserList,
	)
	if err != nil {
		// if one of the deep copy functions is malformed, detect it immediately.
		panic(err)
	}
}

// AUTO-GENERATED FUNCTIONS END HERE
