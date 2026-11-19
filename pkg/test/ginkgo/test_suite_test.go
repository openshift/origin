package ginkgo

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/test/extensions"
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
		facts     []string
		topology  configv1.TopologyMode
		remaining []string
	}{
		{
			name:      "ordinary OCP retains built-in tests",
			topology:  configv1.HighlyAvailableTopologyMode,
			remaining: []string{commonTest, classicTest, hcpTest, otherTest},
		},
		{
			name:      "ROSA Classic filters common and Classic built-in tests",
			facts:     []string{"product=ROSA"},
			topology:  configv1.HighlyAvailableTopologyMode,
			remaining: []string{hcpTest, otherTest},
		},
		{
			name:      "ROSA HCP filters common and HCP built-in tests",
			facts:     []string{"product=ROSA"},
			topology:  configv1.ExternalTopologyMode,
			remaining: []string{classicTest, otherTest},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			builder := (&extensions.EnvironmentFlagsBuilder{}).AddTopology(&test.topology)
			for _, fact := range test.facts {
				builder.AddFact(fact)
			}
			filtered := filterTestsByROSA(tests, builder.Build())
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

func testCaseNames(tests []*testCase) []string {
	names := make([]string, 0, len(tests))
	for _, test := range tests {
		names = append(names, test.name)
	}
	return names
}
