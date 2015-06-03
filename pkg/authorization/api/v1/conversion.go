package v1

import (
	"sort"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	newer "github.com/openshift/origin/pkg/authorization/api"
)

func init() {
	err := api.Scheme.AddConversionFuncs(
		func(in *SubjectAccessReview, out *newer.SubjectAccessReview, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
				return err
			}

			out.Groups = util.NewStringSet(in.GroupsSlice...)

			return nil
		},
		func(in *newer.SubjectAccessReview, out *SubjectAccessReview, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
				return err
			}

			out.GroupsSlice = in.Groups.List()

			return nil
		},
		func(in *ResourceAccessReviewResponse, out *newer.ResourceAccessReviewResponse, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
				return err
			}

			out.Users = util.NewStringSet(in.UsersSlice...)
			out.Groups = util.NewStringSet(in.GroupsSlice...)

			return nil
		},
		func(in *newer.ResourceAccessReviewResponse, out *ResourceAccessReviewResponse, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields); err != nil {
				return err
			}

			out.UsersSlice = in.Users.List()
			out.GroupsSlice = in.Groups.List()

			return nil
		},
		func(in *PolicyRule, out *newer.PolicyRule, s conversion.Scope) error {
			if err := s.Convert(&in.AttributeRestrictions, &out.AttributeRestrictions, 0); err != nil {
				return err
			}

			out.Resources = util.StringSet{}
			out.Resources.Insert(in.Resources...)

			out.Verbs = util.StringSet{}
			out.Verbs.Insert(in.Verbs...)

			out.ResourceNames = util.NewStringSet(in.ResourceNames...)

			out.NonResourceURLs = util.NewStringSet(in.NonResourceURLsSlice...)

			return nil
		},
		func(in *newer.PolicyRule, out *PolicyRule, s conversion.Scope) error {
			if err := s.Convert(&in.AttributeRestrictions, &out.AttributeRestrictions, 0); err != nil {
				return err
			}

			out.Resources = []string{}
			out.Resources = append(out.Resources, in.Resources.List()...)

			out.Verbs = []string{}
			out.Verbs = append(out.Verbs, in.Verbs.List()...)

			out.ResourceNames = in.ResourceNames.List()

			out.NonResourceURLsSlice = in.NonResourceURLs.List()

			return nil
		},
		func(in *Policy, out *newer.Policy, s conversion.Scope) error {
			out.LastModified = in.LastModified
			out.Roles = make(map[string]newer.Role)
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *newer.Policy, out *Policy, s conversion.Scope) error {
			out.LastModified = in.LastModified
			out.Roles = make([]NamedRole, 0, 0)
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *RoleBinding, out *newer.RoleBinding, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields|conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}

			out.Users = util.NewStringSet(in.UserNames...)
			out.Groups = util.NewStringSet(in.GroupNames...)

			return nil
		},
		func(in *newer.RoleBinding, out *RoleBinding, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields|conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}

			out.UserNames = in.Users.List()
			out.GroupNames = in.Groups.List()

			return nil
		},
		func(in *[]NamedRole, out *map[string]newer.Role, s conversion.Scope) error {
			for _, curr := range *in {
				newRole := &newer.Role{}
				if err := s.Convert(&curr.Role, newRole, 0); err != nil {
					return err
				}
				(*out)[curr.Name] = *newRole
			}

			return nil
		},
		func(in *map[string]newer.Role, out *[]NamedRole, s conversion.Scope) error {
			allKeys := make([]string, 0, len(*in))
			for key := range *in {
				allKeys = append(allKeys, key)
			}
			sort.Strings(allKeys)

			for _, key := range allKeys {
				newRole := (*in)[key]
				oldRole := &Role{}
				if err := s.Convert(&newRole, oldRole, 0); err != nil {
					return err
				}

				namedRole := NamedRole{key, *oldRole}
				*out = append(*out, namedRole)
			}

			return nil
		},
		func(in *PolicyBinding, out *newer.PolicyBinding, s conversion.Scope) error {
			out.LastModified = in.LastModified
			out.RoleBindings = make(map[string]newer.RoleBinding)
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *newer.PolicyBinding, out *PolicyBinding, s conversion.Scope) error {
			out.LastModified = in.LastModified
			out.RoleBindings = make([]NamedRoleBinding, 0, 0)
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *[]NamedRoleBinding, out *map[string]newer.RoleBinding, s conversion.Scope) error {
			for _, curr := range *in {
				newRoleBinding := &newer.RoleBinding{}
				if err := s.Convert(&curr.RoleBinding, newRoleBinding, 0); err != nil {
					return err
				}
				(*out)[curr.Name] = *newRoleBinding
			}

			return nil
		},
		func(in *map[string]newer.RoleBinding, out *[]NamedRoleBinding, s conversion.Scope) error {
			allKeys := make([]string, 0, len(*in))
			for key := range *in {
				allKeys = append(allKeys, key)
			}
			sort.Strings(allKeys)

			for _, key := range allKeys {
				newRoleBinding := (*in)[key]
				oldRoleBinding := &RoleBinding{}
				if err := s.Convert(&newRoleBinding, oldRoleBinding, 0); err != nil {
					return err
				}

				namedRoleBinding := NamedRoleBinding{key, *oldRoleBinding}
				*out = append(*out, namedRoleBinding)
			}

			return nil
		},

		// and now the globals
		func(in *ClusterPolicy, out *newer.ClusterPolicy, s conversion.Scope) error {
			out.LastModified = in.LastModified
			out.Roles = make(map[string]newer.ClusterRole)
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *newer.ClusterPolicy, out *ClusterPolicy, s conversion.Scope) error {
			out.LastModified = in.LastModified
			out.Roles = make([]NamedClusterRole, 0, 0)
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *ClusterRoleBinding, out *newer.ClusterRoleBinding, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields|conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}

			out.Users = util.NewStringSet(in.UserNames...)
			out.Groups = util.NewStringSet(in.GroupNames...)

			return nil
		},
		func(in *newer.ClusterRoleBinding, out *ClusterRoleBinding, s conversion.Scope) error {
			if err := s.DefaultConvert(in, out, conversion.IgnoreMissingFields|conversion.AllowDifferentFieldTypeNames); err != nil {
				return err
			}

			out.UserNames = in.Users.List()
			out.GroupNames = in.Groups.List()

			return nil
		},
		func(in *[]NamedClusterRole, out *map[string]newer.ClusterRole, s conversion.Scope) error {
			for _, curr := range *in {
				newRole := &newer.ClusterRole{}
				if err := s.Convert(&curr.Role, newRole, 0); err != nil {
					return err
				}
				(*out)[curr.Name] = *newRole
			}

			return nil
		},
		func(in *map[string]newer.ClusterRole, out *[]NamedClusterRole, s conversion.Scope) error {
			allKeys := make([]string, 0, len(*in))
			for key := range *in {
				allKeys = append(allKeys, key)
			}
			sort.Strings(allKeys)

			for _, key := range allKeys {
				newRole := (*in)[key]
				oldRole := &ClusterRole{}
				if err := s.Convert(&newRole, oldRole, 0); err != nil {
					return err
				}

				namedRole := NamedClusterRole{key, *oldRole}
				*out = append(*out, namedRole)
			}

			return nil
		},
		func(in *ClusterPolicyBinding, out *newer.ClusterPolicyBinding, s conversion.Scope) error {
			out.LastModified = in.LastModified
			out.RoleBindings = make(map[string]newer.ClusterRoleBinding)
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *newer.ClusterPolicyBinding, out *ClusterPolicyBinding, s conversion.Scope) error {
			out.LastModified = in.LastModified
			out.RoleBindings = make([]NamedClusterRoleBinding, 0, 0)
			return s.DefaultConvert(in, out, conversion.IgnoreMissingFields)
		},
		func(in *[]NamedClusterRoleBinding, out *map[string]newer.ClusterRoleBinding, s conversion.Scope) error {
			for _, curr := range *in {
				newRoleBinding := &newer.ClusterRoleBinding{}
				if err := s.Convert(&curr.RoleBinding, newRoleBinding, 0); err != nil {
					return err
				}
				(*out)[curr.Name] = *newRoleBinding
			}

			return nil
		},
		func(in *map[string]newer.ClusterRoleBinding, out *[]NamedClusterRoleBinding, s conversion.Scope) error {
			allKeys := make([]string, 0, len(*in))
			for key := range *in {
				allKeys = append(allKeys, key)
			}
			sort.Strings(allKeys)

			for _, key := range allKeys {
				newRoleBinding := (*in)[key]
				oldRoleBinding := &ClusterRoleBinding{}
				if err := s.Convert(&newRoleBinding, oldRoleBinding, 0); err != nil {
					return err
				}

				namedRoleBinding := NamedClusterRoleBinding{key, *oldRoleBinding}
				*out = append(*out, namedRoleBinding)
			}

			return nil
		},
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
