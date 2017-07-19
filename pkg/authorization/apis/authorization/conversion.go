package authorization

import (
	"fmt"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apis/rbac"

	"github.com/openshift/origin/pkg/user/apis/user/validation"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddConversionFuncs(
		Convert_authorization_ClusterRole_To_rbac_ClusterRole,
		Convert_authorization_Role_To_rbac_Role,
		Convert_authorization_ClusterRoleBinding_To_rbac_ClusterRoleBinding,
		Convert_authorization_RoleBinding_To_rbac_RoleBinding,
		Convert_rbac_ClusterRole_To_authorization_ClusterRole,
		Convert_rbac_Role_To_authorization_Role,
		Convert_rbac_ClusterRoleBinding_To_authorization_ClusterRoleBinding,
		Convert_rbac_RoleBinding_To_authorization_RoleBinding,
	); err != nil { // If one of the conversion functions is malformed, detect it immediately.
		return err
	}
	return nil
}

func Convert_authorization_ClusterRole_To_rbac_ClusterRole(in *ClusterRole, out *rbac.ClusterRole, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Rules = convert_api_PolicyRules_To_rbac_PolicyRules(in.Rules)
	return nil
}

func Convert_authorization_Role_To_rbac_Role(in *Role, out *rbac.Role, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Rules = convert_api_PolicyRules_To_rbac_PolicyRules(in.Rules)
	return nil
}

func Convert_authorization_ClusterRoleBinding_To_rbac_ClusterRoleBinding(in *ClusterRoleBinding, out *rbac.ClusterRoleBinding, _ conversion.Scope) error {
	if len(in.RoleRef.Namespace) != 0 {
		return fmt.Errorf("invalid origin cluster role binding %s: attempts to reference role in namespace %q instead of cluster scope", in.Name, in.RoleRef.Namespace)
	}
	var err error
	if out.Subjects, err = convert_api_Subjects_To_rbac_Subjects(in.Subjects); err != nil {
		return err
	}
	out.RoleRef = convert_api_RoleRef_To_rbac_RoleRef(&in.RoleRef)
	out.ObjectMeta = in.ObjectMeta
	return nil
}

func Convert_authorization_RoleBinding_To_rbac_RoleBinding(in *RoleBinding, out *rbac.RoleBinding, _ conversion.Scope) error {
	if len(in.RoleRef.Namespace) != 0 && in.RoleRef.Namespace != in.Namespace {
		return fmt.Errorf("invalid origin role binding %s: attempts to reference role in namespace %q instead of current namespace %q", in.Name, in.RoleRef.Namespace, in.Namespace)
	}
	var err error
	if out.Subjects, err = convert_api_Subjects_To_rbac_Subjects(in.Subjects); err != nil {
		return err
	}
	out.RoleRef = convert_api_RoleRef_To_rbac_RoleRef(&in.RoleRef)
	out.ObjectMeta = in.ObjectMeta
	return nil
}

func convert_api_PolicyRules_To_rbac_PolicyRules(in []PolicyRule) []rbac.PolicyRule {
	rules := make([]rbac.PolicyRule, 0, len(in))
	for _, rule := range in {
		// Origin's authorizer's RuleMatches func ignores rules that have AttributeRestrictions.
		// Since we know this rule will never be respected in Origin, we do not preserve it during conversion.
		if rule.AttributeRestrictions != nil {
			continue
		}
		// We need to split this rule into multiple rules for RBAC
		if isResourceRule(&rule) && isNonResourceRule(&rule) {
			r1 := rbac.PolicyRule{
				Verbs:         rule.Verbs.List(),
				APIGroups:     rule.APIGroups,
				Resources:     rule.Resources.List(),
				ResourceNames: rule.ResourceNames.List(),
			}
			r2 := rbac.PolicyRule{
				Verbs:           rule.Verbs.List(),
				NonResourceURLs: rule.NonResourceURLs.List(),
			}
			rules = append(rules, r1, r2)
		} else {
			r := rbac.PolicyRule{
				APIGroups:       rule.APIGroups,
				Verbs:           rule.Verbs.List(),
				Resources:       rule.Resources.List(),
				ResourceNames:   rule.ResourceNames.List(),
				NonResourceURLs: rule.NonResourceURLs.List(),
			}
			rules = append(rules, r)
		}
	}
	return rules
}

func isResourceRule(rule *PolicyRule) bool {
	return len(rule.APIGroups) > 0 || len(rule.Resources) > 0 || len(rule.ResourceNames) > 0
}

func isNonResourceRule(rule *PolicyRule) bool {
	return len(rule.NonResourceURLs) > 0
}

func convert_api_Subjects_To_rbac_Subjects(in []api.ObjectReference) ([]rbac.Subject, error) {
	subjects := make([]rbac.Subject, 0, len(in))
	for _, subject := range in {
		s := rbac.Subject{
			Name: subject.Name,
		}

		switch subject.Kind {
		case ServiceAccountKind:
			s.Kind = rbac.ServiceAccountKind
			s.Namespace = subject.Namespace
		case UserKind, SystemUserKind:
			s.APIGroup = rbac.GroupName
			s.Kind = rbac.UserKind
		case GroupKind, SystemGroupKind:
			s.APIGroup = rbac.GroupName
			s.Kind = rbac.GroupKind
		default:
			return nil, fmt.Errorf("invalid kind for origin subject: %q", subject.Kind)
		}

		subjects = append(subjects, s)
	}
	return subjects, nil
}

func convert_api_RoleRef_To_rbac_RoleRef(in *api.ObjectReference) rbac.RoleRef {
	return rbac.RoleRef{
		APIGroup: rbac.GroupName,
		Kind:     getRBACRoleRefKind(in.Namespace),
		Name:     in.Name,
	}
}

// Infers the scope of the kind based on the presence of the namespace
func getRBACRoleRefKind(namespace string) string {
	kind := "ClusterRole"
	if len(namespace) != 0 {
		kind = "Role"
	}
	return kind
}

func Convert_rbac_ClusterRole_To_authorization_ClusterRole(in *rbac.ClusterRole, out *ClusterRole, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Rules = convert_rbac_PolicyRules_To_authorization_PolicyRules(in.Rules)
	return nil
}

func Convert_rbac_Role_To_authorization_Role(in *rbac.Role, out *Role, _ conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	out.Rules = convert_rbac_PolicyRules_To_authorization_PolicyRules(in.Rules)
	return nil
}

func Convert_rbac_ClusterRoleBinding_To_authorization_ClusterRoleBinding(in *rbac.ClusterRoleBinding, out *ClusterRoleBinding, _ conversion.Scope) error {
	var err error
	if out.Subjects, err = convert_rbac_Subjects_To_authorization_Subjects(in.Subjects); err != nil {
		return err
	}
	if out.RoleRef, err = convert_rbac_RoleRef_To_authorization_RoleRef(&in.RoleRef, ""); err != nil {
		return err
	}
	out.ObjectMeta = in.ObjectMeta
	return nil
}

func Convert_rbac_RoleBinding_To_authorization_RoleBinding(in *rbac.RoleBinding, out *RoleBinding, _ conversion.Scope) error {
	var err error
	if out.Subjects, err = convert_rbac_Subjects_To_authorization_Subjects(in.Subjects); err != nil {
		return err
	}
	if out.RoleRef, err = convert_rbac_RoleRef_To_authorization_RoleRef(&in.RoleRef, in.Namespace); err != nil {
		return err
	}
	out.ObjectMeta = in.ObjectMeta
	return nil
}

func convert_rbac_Subjects_To_authorization_Subjects(in []rbac.Subject) ([]api.ObjectReference, error) {
	subjects := make([]api.ObjectReference, 0, len(in))
	for _, subject := range in {
		s := api.ObjectReference{
			Name: subject.Name,
		}

		switch subject.Kind {
		case rbac.ServiceAccountKind:
			s.Kind = ServiceAccountKind
			s.Namespace = subject.Namespace
		case rbac.UserKind:
			s.Kind = determineUserKind(subject.Name, validation.ValidateUserName)
		case rbac.GroupKind:
			s.Kind = determineGroupKind(subject.Name, validation.ValidateGroupName)
		default:
			return nil, fmt.Errorf("invalid kind for rbac subject: %q", subject.Kind)
		}

		subjects = append(subjects, s)
	}
	return subjects, nil
}

// rbac.RoleRef has no namespace field since that can be inferred from the kind of referenced role.
// The Origin role ref (api.ObjectReference) requires its namespace value to match the binding's namespace
// for a binding to a role.  For a binding to a cluster role, the namespace value must be the empty string.
// Thus we have to explicitly provide the namespace value as a parameter and use it based on the role's kind.
func convert_rbac_RoleRef_To_authorization_RoleRef(in *rbac.RoleRef, namespace string) (api.ObjectReference, error) {
	switch in.Kind {
	case "ClusterRole":
		return api.ObjectReference{Name: in.Name}, nil
	case "Role":
		return api.ObjectReference{Name: in.Name, Namespace: namespace}, nil
	default:
		return api.ObjectReference{}, fmt.Errorf("invalid kind %q for rbac role ref %q", in.Kind, in.Name)
	}
}

func convert_rbac_PolicyRules_To_authorization_PolicyRules(in []rbac.PolicyRule) []PolicyRule {
	rules := make([]PolicyRule, 0, len(in))
	for _, rule := range in {
		r := PolicyRule{
			APIGroups:       rule.APIGroups,
			Verbs:           sets.NewString(rule.Verbs...),
			Resources:       sets.NewString(rule.Resources...),
			ResourceNames:   sets.NewString(rule.ResourceNames...),
			NonResourceURLs: sets.NewString(rule.NonResourceURLs...),
		}
		rules = append(rules, r)
	}
	return rules
}
