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

// realisticExternalCSITestName returns a spec name shaped like upstream kubernetes external
// CSI tests: framework.Describe joins string fragments with a single space (see
// k8s.io/kubernetes/test/e2e/framework/ginkgowrapper.go registerInSuite), and
// k8s.io/kubernetes/test/e2e/storage/external/external.go registers
// framework.Describe("External Storage", "[Driver: <name>]", ...).
// Ginkgo full names include nested container text; they always contain this substring
// when the driver matches.
func realisticExternalCSITestName(driver string) string {
	return "External Storage [Driver: " + driver + "] [Testpattern: Dynamic PV (default fs)] volume should allow exec of files on the volume"
}

func TestClusterStateFilterDisabledCSIDrivers(t *testing.T) {
	const (
		azureDriver = "disk.csi.azure.com"
		awsDriver   = "ebs.csi.aws.com"
	)

	baseConfig := func() *clusterdiscovery.ClusterConfiguration {
		return &clusterdiscovery.ClusterConfiguration{
			ProviderName:         "aws",
			NetworkPlugin:        "OVNKubernetes",
			HasIPv4:              true,
			HasIPv6:              false,
			EnabledFeatureGates:  sets.New[string](),
			DisabledFeatureGates: sets.New[string](),
			APIGroups:            sets.New[string](),
		}
	}

	tests := []struct {
		name string
		// disabledDriverNames: when empty (nil or len 0), ClusterConfiguration.DisabledCSIDrivers is
		// left unset (nil). When non-empty, set to sets.New(names...). For this filter, nil and a
		// non-nil empty set behave the same (no CSI names skipped); empty slice here models
		// "nothing disabled" without assigning the field.
		disabledDriverNames []string
		inputNames          []string
		wantNames           []string
	}{
		{
			name:                "filters when driver management state is removed",
			disabledDriverNames: []string{azureDriver},
			inputNames: []string{
				realisticExternalCSITestName(azureDriver),
				realisticExternalCSITestName(awsDriver),
				"openshift/conformance unrelated test",
			},
			wantNames: []string{
				realisticExternalCSITestName(awsDriver),
				"openshift/conformance unrelated test",
			},
		},
		{
			name:                "no removed CSI drivers does not filter",
			disabledDriverNames: nil,
			inputNames:          []string{realisticExternalCSITestName(azureDriver)},
			wantNames:           []string{realisticExternalCSITestName(azureDriver)},
		},
		{
			name:                "multiple removed drivers",
			disabledDriverNames: []string{azureDriver, awsDriver},
			inputNames: []string{
				realisticExternalCSITestName(azureDriver),
				realisticExternalCSITestName(awsDriver),
			},
			wantNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := baseConfig()
			if len(tt.disabledDriverNames) > 0 {
				cfg.DisabledCSIDrivers = sets.New(tt.disabledDriverNames...)
			}

			filter := NewClusterStateFilter(cfg)
			specs := make(extensions.ExtensionTestSpecs, 0, len(tt.inputNames))
			for _, n := range tt.inputNames {
				specs = append(specs, &extensions.ExtensionTestSpec{
					ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: n},
				})
			}

			out, err := filter.Filter(context.Background(), specs)
			require.NoError(t, err)
			require.Len(t, out, len(tt.wantNames))

			got := make([]string, len(out))
			for i, spec := range out {
				got[i] = spec.Name
			}
			assert.Equal(t, tt.wantNames, got)
		})
	}
}

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
