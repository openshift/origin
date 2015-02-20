package api

import (
	"fmt"
	"strings"

	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
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
