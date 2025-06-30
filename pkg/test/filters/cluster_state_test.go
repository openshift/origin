package filters

import (
	"context"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/test/extensions"
)

func TestClusterStateFilter(t *testing.T) {
	config := &clusterdiscovery.ClusterConfiguration{
		ProviderName:         "aws",
		NetworkPlugin:        "OVNKubernetes",
		HasIPv4:              true,
		HasIPv6:              false,
		EnabledFeatureGates:  sets.New[string](),
		DisabledFeatureGates: sets.New[string](),
		APIGroups:            sets.New[string](),
	}
	filter := NewClusterStateFilter(config)

	tests := extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "normal test"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [Skipped:aws]"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [Feature:Networking-IPv6]"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [Feature:Networking-IPv4]"}},
	}

	result, err := filter.Filter(context.Background(), tests)

	require.NoError(t, err)
	assert.Len(t, result, 2) // Should filter out aws-skipped and IPv6 tests
	assert.Equal(t, "normal test", result[0].Name)
	assert.Equal(t, "test [Feature:Networking-IPv4]", result[1].Name)
}

// Test data for comprehensive cluster state filter testing
var e2eTestNames = map[string]string{
	"everyone":              "[Skipped:Wednesday]",
	"not-gce":               "[Skipped:gce]",
	"not-aws":               "[Skipped:aws]",
	"not-sdn":               "[Skipped:Network/OpenShiftSDN]",
	"not-multitenant":       "[Skipped:Network/OpenShiftSDN/Multitenant]",
	"online":                "[Skipped:Disconnected]",
	"ipv4":                  "[Feature:Networking-IPv4]",
	"ipv6":                  "[Feature:Networking-IPv6]",
	"dual-stack":            "[Feature:IPv6DualStackAlpha]",
	"sctp":                  "[Feature:SCTPConnectivity]",
	"requires-optional-cap": "[Skipped:NoOptionalCapabilities]",
	"apigroup-apps":         "[apigroup:apps]",
	"apigroup-missing":      "[apigroup:missing]",
	"featuregate-enabled":   "[OCPFeatureGate:FeatureA]",
	"featuregate-missing":   "[OCPFeatureGate:MissingFeature]",
	"featuregate-disabled":  "[OCPFeatureGate:DisabledFeature]",
}

// Helper function to create test specs from test names
func createTestSpecs(testNames map[string]string) extensions.ExtensionTestSpecs {
	specs := make(extensions.ExtensionTestSpecs, 0, len(testNames))
	for name, tags := range testNames {
		fullName := name + " " + tags
		specs = append(specs, &extensions.ExtensionTestSpec{
			ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: fullName},
		})
	}
	return specs
}

// Helper function to extract test names from filtered results
func extractTestNames(specs extensions.ExtensionTestSpecs, testNames map[string]string) sets.Set[string] {
	result := sets.New[string]()
	for _, spec := range specs {
		for name, tags := range testNames {
			fullName := name + " " + tags
			if spec.Name == fullName {
				result.Insert(name)
				break
			}
		}
	}
	return result
}

func TestClusterStateFilterComprehensive(t *testing.T) {
	testCases := []struct {
		name     string
		config   *clusterdiscovery.ClusterConfiguration
		runTests sets.Set[string]
	}{
		{
			name: "simple GCE",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:              "gce",
				NetworkPlugin:             "OpenShiftSDN",
				HasIPv4:                   true,
				HasIPv6:                   false,
				HasNoOptionalCapabilities: false,
				EnabledFeatureGates:       sets.New[string](),
				DisabledFeatureGates:      sets.New[string](),
				APIGroups:                 sets.New[string](),
			},
			runTests: sets.New("everyone", "not-aws", "not-multitenant", "online", "ipv4", "requires-optional-cap"),
		},
		{
			name: "GCE multitenant",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:              "gce",
				NetworkPlugin:             "OpenShiftSDN",
				NetworkPluginMode:         "Multitenant",
				HasIPv4:                   true,
				HasIPv6:                   false,
				HasNoOptionalCapabilities: false,
				EnabledFeatureGates:       sets.New[string](),
				DisabledFeatureGates:      sets.New[string](),
				APIGroups:                 sets.New[string](),
			},
			runTests: sets.New("everyone", "not-aws", "online", "ipv4", "requires-optional-cap"),
		},
		{
			name: "simple non-cloud",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:              "skeleton",
				NetworkPlugin:             "OpenShiftSDN",
				HasIPv4:                   true,
				HasIPv6:                   false,
				HasNoOptionalCapabilities: false,
				EnabledFeatureGates:       sets.New[string](),
				DisabledFeatureGates:      sets.New[string](),
				APIGroups:                 sets.New[string](),
			},
			runTests: sets.New("everyone", "not-gce", "not-aws", "not-multitenant", "online", "ipv4", "requires-optional-cap"),
		},
		{
			name: "complex override dual-stack",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:              "aws",
				NetworkPlugin:             "OVNKubernetes",
				HasIPv4:                   true,
				HasIPv6:                   true,
				HasNoOptionalCapabilities: false,
				EnabledFeatureGates:       sets.New[string](),
				DisabledFeatureGates:      sets.New[string](),
				APIGroups:                 sets.New[string](),
			},
			runTests: sets.New("everyone", "not-gce", "not-sdn", "not-multitenant", "online", "ipv4", "ipv6", "dual-stack", "requires-optional-cap"),
		},
		{
			name: "disconnected",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:              "none",
				NetworkPlugin:             "OVNKubernetes",
				HasIPv4:                   true,
				HasIPv6:                   true,
				Disconnected:              true,
				HasNoOptionalCapabilities: false,
				EnabledFeatureGates:       sets.New[string](),
				DisabledFeatureGates:      sets.New[string](),
				APIGroups:                 sets.New[string](),
			},
			runTests: sets.New("everyone", "not-gce", "not-aws", "not-sdn", "not-multitenant", "ipv4", "ipv6", "dual-stack", "requires-optional-cap"),
		},
		{
			name: "override network plugin with SCTP",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:              "aws",
				NetworkPlugin:             "Calico",
				HasIPv4:                   false,
				HasIPv6:                   true,
				HasSCTP:                   true,
				HasNoOptionalCapabilities: false,
				EnabledFeatureGates:       sets.New[string](),
				DisabledFeatureGates:      sets.New[string](),
				APIGroups:                 sets.New[string](),
			},
			runTests: sets.New("everyone", "not-gce", "not-sdn", "not-multitenant", "online", "ipv6", "sctp", "requires-optional-cap"),
		},
		{
			name: "no optional capabilities",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:              "gce",
				NetworkPlugin:             "OpenShiftSDN",
				HasIPv4:                   true,
				HasIPv6:                   false,
				HasNoOptionalCapabilities: true,
				EnabledFeatureGates:       sets.New[string](),
				DisabledFeatureGates:      sets.New[string](),
				APIGroups:                 sets.New[string](),
			},
			runTests: sets.New("everyone", "not-aws", "not-multitenant", "online", "ipv4"),
		},
		{
			name: "API group filtering",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:         "gce",
				NetworkPlugin:        "OpenShiftSDN",
				HasIPv4:              true,
				HasIPv6:              false,
				APIGroups:            sets.New("apps", "extensions"),
				EnabledFeatureGates:  sets.New[string](),
				DisabledFeatureGates: sets.New[string](),
			},
			runTests: sets.New("everyone", "not-aws", "not-multitenant", "online", "ipv4", "requires-optional-cap", "apigroup-apps"),
		},
		{
			name: "Feature gate filtering",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:         "gce",
				NetworkPlugin:        "OpenShiftSDN",
				HasIPv4:              true,
				HasIPv6:              false,
				APIGroups:            sets.New[string](),
				EnabledFeatureGates:  sets.New("FeatureA", "FeatureB"),
				DisabledFeatureGates: sets.New("DisabledFeature"),
			},
			runTests: sets.New("everyone", "not-aws", "not-multitenant", "online", "ipv4", "requires-optional-cap", "featuregate-enabled", "featuregate-missing"),
		},
		{
			name: "Feature gate filtering - only disabled gates",
			config: &clusterdiscovery.ClusterConfiguration{
				ProviderName:         "gce",
				NetworkPlugin:        "OpenShiftSDN",
				HasIPv4:              true,
				HasIPv6:              false,
				APIGroups:            sets.New[string](),
				EnabledFeatureGates:  sets.New[string](),
				DisabledFeatureGates: sets.New("DisabledFeature"),
			},
			runTests: sets.New("everyone", "not-aws", "not-multitenant", "online", "ipv4", "requires-optional-cap", "featuregate-enabled", "featuregate-missing"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filter := NewClusterStateFilter(tc.config)
			testSpecs := createTestSpecs(e2eTestNames)

			result, err := filter.Filter(context.Background(), testSpecs)

			require.NoError(t, err)
			runTests := extractTestNames(result, e2eTestNames)
			assert.True(t, runTests.Equal(tc.runTests),
				"Expected tests: %v, got: %v", tc.runTests.UnsortedList(), runTests.UnsortedList())
		})
	}
}
