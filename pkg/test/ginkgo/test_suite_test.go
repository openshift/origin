package ginkgo

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestFilterTestsByROSA(t *testing.T) {
	const (
		commonTest  = "[sig-auth][Feature:UserAPI] users can manipulate groups [Suite:openshift/conformance/parallel]"
		classicTest = "[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] test"
		hcpTest     = "[sig-apps] Deployment should not disrupt a cloud load-balancer's connectivity during rollout"
		otherTest   = "[sig-api-machinery] an unrelated test"
	)

	tests := []*testCase{
		{name: commonTest},
		{name: classicTest},
		{name: hcpTest},
		{name: otherTest},
	}
	testCases := []struct {
		name      string
		topology  *configv1.TopologyMode
		remaining []string
	}{
		{
			name:      "ordinary OCP retains built-in tests",
			remaining: []string{commonTest, classicTest, hcpTest, otherTest},
		},
		{
			name:      "ROSA Classic filters common and Classic built-in tests",
			topology:  topologyPointer(configv1.HighlyAvailableTopologyMode),
			remaining: []string{hcpTest, otherTest},
		},
		{
			name:      "ROSA HCP filters common and HCP built-in tests",
			topology:  topologyPointer(configv1.ExternalTopologyMode),
			remaining: []string{classicTest, otherTest},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			filtered := filterTestsByROSA(tests, test.topology)
			if len(filtered) != len(test.remaining) {
				t.Fatalf("got remaining tests %v, want %v", testCaseNames(filtered), test.remaining)
			}
			for i := range filtered {
				if filtered[i].name != test.remaining[i] {
					t.Errorf("remaining test %d = %q, want %q", i, filtered[i].name, test.remaining[i])
				}
			}
		})
	}
}

func topologyPointer(topology configv1.TopologyMode) *configv1.TopologyMode {
	return &topology
}

func testCaseNames(tests []*testCase) []string {
	names := make([]string, 0, len(tests))
	for _, test := range tests {
		names = append(names, test.name)
	}
	return names
}
