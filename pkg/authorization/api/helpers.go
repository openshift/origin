package api

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/util/sets"

	// uservalidation "github.com/openshift/origin/pkg/user/api/validation"
)

// NormalizeResources expands all resource groups and forces all resources to lower case.
// If the rawResources are already normalized, it returns the original set to avoid the
// allocation and GC cost, since this is hit multiple times for every REST call.
// That means you should NEVER MODIFY THE RESULT of this call.
func NormalizeResources(rawResources sets.String) sets.String {
	// we only need to expand groups if the exist and we don't create them with groups
	// by default.  Only accept the cost of expansion if we're doing work.
	needsNormalization := false
	for currResource := range rawResources {
		if needsNormalizing(currResource) {
			needsNormalization = true
			break
		}

	}
	if !needsNormalization {
		return rawResources
	}

	ret := sets.String{}
	toVisit := rawResources.List()
	visited := sets.String{}

	for i := 0; i < len(toVisit); i++ {
		currResource := toVisit[i]
		if visited.Has(currResource) {
			continue
		}
		visited.Insert(currResource)

		if !strings.HasPrefix(currResource, ResourceGroupPrefix) {
			ret.Insert(strings.ToLower(currResource))
			continue
		}

		if resourceTypes, exists := GroupsToResources[currResource]; exists {
			toVisit = append(toVisit, resourceTypes...)
		}
	}

	return ret
}

func needsNormalizing(in string) bool {
	if strings.HasPrefix(in, ResourceGroupPrefix) {
		return true
	}
	for _, r := range in {
		if unicode.IsUpper(r) {
			return true
		}
	}
	return false
}

func (r PolicyRule) String() string {
	return fmt.Sprintf("PolicyRule{Verbs:%v, APIGroups:%v, Resources:%v, ResourceNames:%v, Restrictions:%v}", r.Verbs.List(), r.APIGroups, r.Resources.List(), r.ResourceNames.List(), r.AttributeRestrictions)
}

func getRoleBindingValues(roleBindingMap map[string]*RoleBinding) []*RoleBinding {
	ret := []*RoleBinding{}
	for _, currBinding := range roleBindingMap {
		ret = append(ret, currBinding)
	}

	return ret
}
func SortRoleBindings(roleBindingMap map[string]*RoleBinding, reverse bool) []*RoleBinding {
	roleBindings := getRoleBindingValues(roleBindingMap)
	if reverse {
		sort.Sort(sort.Reverse(RoleBindingSorter(roleBindings)))
	} else {
		sort.Sort(RoleBindingSorter(roleBindings))
	}

	return roleBindings
}

type PolicyBindingSorter []PolicyBinding

func (s PolicyBindingSorter) Len() int {
	return len(s)
}
func (s PolicyBindingSorter) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}
func (s PolicyBindingSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type RoleBindingSorter []*RoleBinding

func (s RoleBindingSorter) Len() int {
	return len(s)
}
func (s RoleBindingSorter) Less(i, j int) bool {
	return s[i].Name < s[j].Name
}
func (s RoleBindingSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func GetPolicyBindingName(policyRefNamespace string) string {
	return fmt.Sprintf("%s:%s", policyRefNamespace, PolicyName)
}

var ClusterPolicyBindingName = GetPolicyBindingName("")

func BuildSubjects(users, groups []string, userNameValidator, groupNameValidator validation.ValidateNameFunc) []kapi.ObjectReference {
	subjects := []kapi.ObjectReference{}

	for _, user := range users {
		saNamespace, saName, err := serviceaccount.SplitUsername(user)
		if err == nil {
			subjects = append(subjects, kapi.ObjectReference{Kind: ServiceAccountKind, Namespace: saNamespace, Name: saName})
			continue
		}

		kind := UserKind
		if valid, _ := userNameValidator(user, false); !valid {
			kind = SystemUserKind
		}

		subjects = append(subjects, kapi.ObjectReference{Kind: kind, Name: user})
	}

	for _, group := range groups {
		kind := GroupKind
		if valid, _ := groupNameValidator(group, false); !valid {
			kind = SystemGroupKind
		}

		subjects = append(subjects, kapi.ObjectReference{Kind: kind, Name: group})
	}

	return subjects
}

// StringSubjectsFor returns users and groups for comparison against user.Info.  currentNamespace is used to
// to create usernames for service accounts where namespace=="".
func StringSubjectsFor(currentNamespace string, subjects []kapi.ObjectReference) ([]string, []string) {
	// these MUST be nil to indicate empty
	var users, groups []string

	for _, subject := range subjects {
		switch subject.Kind {
		case ServiceAccountKind:
			namespace := currentNamespace
			if len(subject.Namespace) > 0 {
				namespace = subject.Namespace
			}
			if len(namespace) > 0 {
				users = append(users, serviceaccount.MakeUsername(namespace, subject.Name))
			}

		case UserKind, SystemUserKind:
			users = append(users, subject.Name)

		case GroupKind, SystemGroupKind:
			groups = append(groups, subject.Name)
		}
	}

	return users, groups
}

// SubjectsStrings returns users, groups, serviceaccounts, unknown for display purposes.  currentNamespace is used to
// hide the subject.Namespace for ServiceAccounts in the currentNamespace
func SubjectsStrings(currentNamespace string, subjects []kapi.ObjectReference) ([]string, []string, []string, []string) {
	users := []string{}
	groups := []string{}
	sas := []string{}
	others := []string{}

	for _, subject := range subjects {
		switch subject.Kind {
		case ServiceAccountKind:
			if len(subject.Namespace) > 0 && currentNamespace != subject.Namespace {
				sas = append(sas, subject.Namespace+"/"+subject.Name)
			} else {
				sas = append(sas, subject.Name)
			}

		case UserKind, SystemUserKind:
			users = append(users, subject.Name)

		case GroupKind, SystemGroupKind:
			groups = append(groups, subject.Name)

		default:
			others = append(others, fmt.Sprintf("%s/%s/%s", subject.Kind, subject.Namespace, subject.Name))

		}
	}

	return users, groups, sas, others
}
