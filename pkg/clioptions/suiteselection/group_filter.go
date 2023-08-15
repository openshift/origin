package suiteselection

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
)

type apiGroupFilter struct {
	apiGroups sets.String
}

func newApiGroupFilter(discoveryClient discovery.AggregatedDiscoveryInterface) (*apiGroupFilter, error) {
	// Check if the groups is served by the server
	groups, err := discoveryClient.ServerGroups()
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve served resources: %v", err)
	}
	apiGroups := sets.NewString()
	for _, apiGroup := range groups.Groups {
		// ignore the empty group
		if apiGroup.Name == "" {
			continue
		}
		apiGroups.Insert(apiGroup.Name)
	}

	return &apiGroupFilter{
		apiGroups: apiGroups,
	}, nil
}

var (
	apiGroupRegex = regexp.MustCompile(`\[apigroup:([^]]*)\]`)
)

func (agf *apiGroupFilter) includeTest(name string) bool {
	apiGroups := []string{}
	matches := apiGroupRegex.FindAllStringSubmatch(name, -1)
	for _, match := range matches {
		if len(match) < 2 {
			panic(fmt.Errorf("regexp match %v is invalid: len(match) < 2 for %v", match, name))
		}
		apigroup := match[1]
		apiGroups = append(apiGroups, apigroup)
	}

	return agf.apiGroups.HasAll(apiGroups...)
}
