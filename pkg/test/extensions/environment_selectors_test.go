package extensions

import (
	"testing"

	et "github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/openshift-eng/openshift-tests-extension/pkg/flags"
	"github.com/openshift-eng/openshift-tests-extension/pkg/util/sets"
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

func TestFilterByROSA(t *testing.T) {
	const (
		commonTest  = "[sig-auth][Feature:UserAPI] users can manipulate groups [apigroup:user.openshift.io]"
		classicTest = "[sig-network-edge][OCPFeatureGate:GatewayAPIController] conformance test"
		hcpTest     = "[sig-apps] Deployment should not disrupt a cloud load-balancer's connectivity during rollout"
		otherTest   = "[sig-api-machinery] an unrelated test"
	)

	specs := et.ExtensionTestSpecs{
		newEnvironmentTestSpec(commonTest),
		newEnvironmentTestSpec(classicTest),
		newEnvironmentTestSpec(hcpTest),
		newEnvironmentTestSpec(otherTest),
	}

	filterByROSA(specs)

	tests := []struct {
		name      string
		facts     map[string]string
		topology  string
		remaining []string
	}{
		{
			name:      "non-ROSA",
			facts:     map[string]string{},
			topology:  "HighlyAvailable",
			remaining: []string{commonTest, classicTest, hcpTest, otherTest},
		},
		{
			name:      "ROSA Classic",
			facts:     map[string]string{"product": "ROSA"},
			topology:  "HighlyAvailable",
			remaining: []string{hcpTest, otherTest},
		},
		{
			name:      "ROSA HCP",
			facts:     map[string]string{"product": "ROSA"},
			topology:  "External",
			remaining: []string{classicTest, otherTest},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filtered, err := specs.FilterByEnvironment(flags.EnvironmentalFlags{
				Facts:    test.facts,
				Topology: test.topology,
			})
			if err != nil {
				t.Fatalf("unexpected filtering error: %v", err)
			}

			if len(filtered) != len(test.remaining) {
				t.Fatalf("got %d remaining tests, want %d: %v", len(filtered), len(test.remaining), specNames(filtered))
			}
			for i := range filtered {
				if filtered[i].Name != test.remaining[i] {
					t.Errorf("remaining test %d = %q, want %q", i, filtered[i].Name, test.remaining[i])
				}
			}
		})
	}

	if !specs[0].Labels.Has("[Skipped:ROSA]") {
		t.Errorf("common ROSA exclusion is missing its label")
	}
	if !specs[1].Labels.Has("[Skipped:ROSA-Classic]") {
		t.Errorf("ROSA Classic exclusion is missing its label")
	}
	if !specs[2].Labels.Has("[Skipped:ROSA-HCP]") {
		t.Errorf("ROSA HCP exclusion is missing its label")
	}
	if specs[3].Labels.Len() != 0 || !specs[3].EnvironmentSelector.IsEmpty() {
		t.Errorf("unrelated test was modified: %#v", specs[3])
	}
}

func newEnvironmentTestSpec(name string) *et.ExtensionTestSpec {
	return &et.ExtensionTestSpec{
		Name:   name,
		Labels: sets.New[string](),
	}
}

func specNames(specs et.ExtensionTestSpecs) []string {
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	return names
}
