package rosa

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestIsCluster(t *testing.T) {
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsCluster(test.platformStatus); got != test.want {
				t.Errorf("IsCluster() = %t, want %t", got, test.want)
			}
		})
	}
}

func TestIsTestExcluded(t *testing.T) {
	tests := []struct {
		name     string
		testName string
		topology configv1.TopologyMode
		want     bool
	}{
		{
			name:     "common exclusion",
			testName: "[sig-auth][Feature:UserAPI] users can manipulate groups [Suite:openshift/conformance/parallel]",
			topology: configv1.ExternalTopologyMode,
			want:     true,
		},
		{
			name:     "classic legacy network segmentation exclusion",
			testName: "[Feature:NetworkSegmentation][ovn-kubernetes-ote][sig-network] legacy test",
			topology: configv1.HighlyAvailableTopologyMode,
			want:     true,
		},
		{
			name:     "classic current network segmentation exclusion",
			testName: "[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] current test",
			topology: configv1.HighlyAvailableTopologyMode,
			want:     true,
		},
		{
			name:     "HCP exclusion",
			testName: "[sig-apps] Deployment should not disrupt a cloud load-balancer's connectivity during rollout",
			topology: configv1.ExternalTopologyMode,
			want:     true,
		},
		{
			name:     "unrelated test",
			testName: "[sig-api-machinery] an unrelated test",
			topology: configv1.ExternalTopologyMode,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsTestExcluded(test.testName, test.topology); got != test.want {
				t.Errorf("IsTestExcluded() = %t, want %t", got, test.want)
			}
		})
	}
}
