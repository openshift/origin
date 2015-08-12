package api

import (
	"fmt"
	"sort"
	"strings"

	kutil "k8s.io/kubernetes/pkg/util"
)

func ExpandResources(rawResources kutil.StringSet) kutil.StringSet {
	ret := kutil.StringSet{}
	toVisit := rawResources.List()
	visited := kutil.StringSet{}

	for i := 0; i < len(toVisit); i++ {
		currResource := toVisit[i]
		if visited.Has(currResource) {
			continue
		}
		visited.Insert(currResource)

		if strings.Index(currResource, ResourceGroupPrefix+":") != 0 {
			ret.Insert(strings.ToLower(currResource))
			continue
		}

		if resourceTypes, exists := GroupsToResources[currResource]; exists {
			toVisit = append(toVisit, resourceTypes...)
		}
	}

	return ret
}

func (r PolicyRule) String() string {
	return fmt.Sprintf("PolicyRule{Verbs:%v, Resources:%v, ResourceNames:%v, Restrictions:%v}", r.Verbs.List(), r.Resources.List(), r.ResourceNames.List(), r.AttributeRestrictions)
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
