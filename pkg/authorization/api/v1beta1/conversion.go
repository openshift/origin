package v1beta1

import (
	"sort"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/conversion"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	newer "github.com/openshift/origin/pkg/authorization/api"
)

func init() {
	err := api.Scheme.AddConversionFuncs(
		func(in *PolicyRule, out *newer.PolicyRule, s conversion.Scope) error {
			if err := s.Convert(&in.AttributeRestrictions, &out.AttributeRestrictions, 0); err != nil {
				return err
			}

			out.Verbs = []string{}
			out.Verbs = append(out.Verbs, in.Verbs...)

			out.Resources = []string{}
			out.Resources = append(out.Resources, in.Resources...)
			out.Resources = append(out.Resources, in.ResourceKinds...)

			out.ResourceNames = util.NewStringSet(in.ResourceNames...)

			return nil
		},
		func(in *newer.PolicyRule, out *PolicyRule, s conversion.Scope) error {
			if err := s.Convert(&in.AttributeRestrictions, &out.AttributeRestrictions, 0); err != nil {
				return err
			}

			out.Verbs = []string{}
			out.Verbs = append(out.Verbs, in.Verbs...)

			out.Resources = []string{}
			out.Resources = append(out.Resources, in.Resources...)

			out.ResourceNames = in.ResourceNames.List()

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
	)
	if err != nil {
		// If one of the conversion functions is malformed, detect it immediately.
		panic(err)
	}
}
