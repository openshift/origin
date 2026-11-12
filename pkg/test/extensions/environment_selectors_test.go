package extensions

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestIsROSACluster(t *testing.T) {
	tests := []struct {
		name           string
		platformStatus *configv1.PlatformStatus
		want           bool
	}{
		{name: "missing platform status"},
		{name: "non-AWS platform", platformStatus: &configv1.PlatformStatus{}},
		{
			name: "AWS without ROSA tag",
			platformStatus: &configv1.PlatformStatus{AWS: &configv1.AWSPlatformStatus{
				ResourceTags: []configv1.AWSResourceTag{{Key: "owner", Value: "team"}},
			}},
		},
		{
			name: "ROSA Classic",
			platformStatus: &configv1.PlatformStatus{AWS: &configv1.AWSPlatformStatus{
				ResourceTags: []configv1.AWSResourceTag{{Key: "red-hat-clustertype", Value: "rosa"}},
			}},
			want: true,
		},
		{
			name: "ROSA HCP uses the same product tag",
			platformStatus: &configv1.PlatformStatus{AWS: &configv1.AWSPlatformStatus{
				ResourceTags: []configv1.AWSResourceTag{{Key: "red-hat-clustertype", Value: "ROSA"}},
			}},
			want: true,
		},
		{
			name: "usage cluster type alone is not used",
			platformStatus: &configv1.PlatformStatus{AWS: &configv1.AWSPlatformStatus{
				ResourceTags: []configv1.AWSResourceTag{{Key: "usage-cluster-type", Value: "rosa-hcp"}},
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsROSACluster(test.platformStatus); got != test.want {
				t.Errorf("IsROSACluster() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestFilterDiscoveredSpecsByROSA(t *testing.T) {
	const (
		commonTest         = "[sig-auth][Feature:UserAPI] users can manipulate groups [Suite:openshift/conformance/parallel]"
		legacyClassicTest  = "[Feature:NetworkSegmentation][ovn-kubernetes-ote][sig-network] legacy external test"
		currentClassicTest = "[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] current built-in test"
		hcpTest            = "[sig-apps] Deployment should not disrupt a cloud load-balancer's connectivity during rollout"
		otherTest          = "[sig-api-machinery] an unrelated test"
	)

	specs := ExtensionTestSpecs{
		{Name: commonTest},
		{Name: legacyClassicTest},
		{Name: currentClassicTest},
		{Name: hcpTest},
		{Name: otherTest},
	}

	tests := []struct {
		name      string
		facts     []string
		topology  configv1.TopologyMode
		remaining []string
	}{
		{
			name:      "ordinary OCP retains inventory",
			topology:  configv1.HighlyAvailableTopologyMode,
			remaining: []string{commonTest, legacyClassicTest, currentClassicTest, hcpTest, otherTest},
		},
		{
			name:      "ROSA Classic excludes common and Classic tests",
			facts:     []string{"product=ROSA"},
			topology:  configv1.HighlyAvailableTopologyMode,
			remaining: []string{hcpTest, otherTest},
		},
		{
			name:      "ROSA HCP excludes common and HCP tests",
			facts:     []string{"product=ROSA"},
			topology:  configv1.ExternalTopologyMode,
			remaining: []string{legacyClassicTest, currentClassicTest, otherTest},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			builder := (&EnvironmentFlagsBuilder{}).AddTopology(&test.topology)
			for _, fact := range test.facts {
				builder.AddFact(fact)
			}

			filtered := filterDiscoveredSpecsByROSA(specs, builder.Build())
			if len(filtered) != len(test.remaining) {
				t.Fatalf("got remaining tests %v, want %v", extensionSpecNames(filtered), test.remaining)
			}
			for i := range filtered {
				if filtered[i].Name != test.remaining[i] {
					t.Errorf("remaining test %d = %q, want %q", i, filtered[i].Name, test.remaining[i])
				}
			}
		})
	}
}

func extensionSpecNames(specs ExtensionTestSpecs) []string {
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	return names
}
