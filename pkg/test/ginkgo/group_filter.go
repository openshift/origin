package ginkgo

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/clientcmd"
)

func getDiscoveryClient() (*discovery.DiscoveryClient, error) {
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{})
	clusterConfig, err := cfg.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not load client configuration: %v", err)
	}

	return discovery.NewDiscoveryClientForConfig(clusterConfig)
}

type apiGroupFilter struct {
	apiGroups sets.String
}

func newApiGroupFilter(discoveryClient *discovery.DiscoveryClient) (*apiGroupFilter, error) {
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

func (agf *apiGroupFilter) markSkippedWhenAPIGroupNotServed(tests []*testCase) {
	for _, test := range tests {
		if !agf.apiGroups.HasAll(test.apigroups...) {
			missingAPIGroups := sets.NewString(test.apigroups...).Difference(agf.apiGroups)
			test.skipped = true
			test.testOutputBytes = []byte(fmt.Sprintf("skipped because the following required API groups are missing: %v", strings.Join(missingAPIGroups.List(), ",")))
			continue
		}
	}

}
