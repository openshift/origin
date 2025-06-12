package filters

import (
	"context"
	"strings"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/clioptions/clusterdiscovery"
	"github.com/openshift/origin/pkg/test/extensions"
)

func TestFilterChainBasic(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	logger.Logger.SetLevel(logrus.FatalLevel)

	tests := extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "normal test"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [Disabled:reason]"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "another normal test"}},
	}

	pipeline := NewFilterChain(logger).
		AddFilter(&DisabledTestsFilter{})

	result, err := pipeline.Apply(context.Background(), tests)

	require.NoError(t, err)
	assert.Len(t, result, 2) // Should filter out the disabled test
	assert.Equal(t, "normal test", result[0].Name)
	assert.Equal(t, "another normal test", result[1].Name)
}

func TestDisabledTestsFilter(t *testing.T) {
	filter := &DisabledTestsFilter{}

	tests := extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "normal test"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [Disabled:reason]"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "another normal test"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [Skipped:reason]"}}, // This won't be filtered by isDisabled
	}

	result, err := filter.Filter(context.Background(), tests)

	require.NoError(t, err)
	assert.Len(t, result, 3) // Only the [Disabled:reason] test should be filtered out

	// Check that the disabled test was filtered out
	for _, test := range result {
		assert.False(t, strings.Contains(test.Name, "[Disabled:reason]"))
	}
}

func TestSuiteMatcherFilter(t *testing.T) {
	// Test with nil matcher (should pass all tests through)
	filter := NewMatchFnFilter(nil)

	tests := extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test1"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test2"}},
	}

	result, err := filter.Filter(context.Background(), tests)

	require.NoError(t, err)
	assert.Len(t, result, 2) // All tests should pass through when suite is nil
	assert.Equal(t, "test1", result[0].Name)
	assert.Equal(t, "test2", result[1].Name)
}

func TestFilterPipelineErrorHandling(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	logger.Logger.SetLevel(logrus.FatalLevel) // Suppress logs during tests

	errorFilter := &testErrorFilter{}

	pipeline := NewFilterChain(logger).
		AddFilter(errorFilter)

	tests := extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test"}},
	}

	result, err := pipeline.Apply(context.Background(), tests)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "test-error-filter failed")
}

type testErrorFilter struct{}

func (f *testErrorFilter) Name() string {
	return "test-error-filter"
}

func (f *testErrorFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	return nil, assert.AnError
}

func (f *testErrorFilter) ShouldApply() bool {
	return true
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

func TestKubeRebaseFilter(t *testing.T) {
	// Test with nil config (should pass all tests through)
	filter := NewKubeRebaseTestsFilter(nil)

	tests := extensions.ExtensionTestSpecs{
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "normal test"}},
		&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "[sig-api-machinery] health handlers should contain necessary checks"}},
	}

	result, err := filter.Filter(context.Background(), tests)

	require.NoError(t, err)
	assert.Len(t, result, 2) // All tests should pass through when restConfig is nil
	assert.Equal(t, "normal test", result[0].Name)
	assert.Equal(t, "[sig-api-machinery] health handlers should contain necessary checks", result[1].Name)
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
		name       string
		config     *clusterdiscovery.ClusterConfiguration
		runTests   sets.Set[string]
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

func TestClusterStateFilterAPIGroupsAndFeatureGates(t *testing.T) {
	// Test API group filtering
	t.Run("API group filtering", func(t *testing.T) {
		config := &clusterdiscovery.ClusterConfiguration{
			ProviderName:         "gce",
			NetworkPlugin:        "OpenShiftSDN",
			HasIPv4:              true,
			HasIPv6:              false,
			APIGroups:            sets.New("apps", "extensions"),
			EnabledFeatureGates:  sets.New[string](),
			DisabledFeatureGates: sets.New[string](),
		}
		filter := NewClusterStateFilter(config)

		tests := extensions.ExtensionTestSpecs{
			&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "normal test"}},
			&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [apigroup:apps]"}},
			&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [apigroup:missing]"}},
		}

		result, err := filter.Filter(context.Background(), tests)

		require.NoError(t, err)
		assert.Len(t, result, 2) // Should filter out the test requiring missing API group
		assert.Equal(t, "normal test", result[0].Name)
		assert.Equal(t, "test [apigroup:apps]", result[1].Name)
	})

	// Test feature gate filtering
	t.Run("Feature gate filtering", func(t *testing.T) {
		config := &clusterdiscovery.ClusterConfiguration{
			ProviderName:         "gce",
			NetworkPlugin:        "OpenShiftSDN",
			HasIPv4:              true,
			HasIPv6:              false,
			APIGroups:            sets.New[string](),
			EnabledFeatureGates:  sets.New("FeatureA", "FeatureB"),
			DisabledFeatureGates: sets.New("DisabledFeature"),
		}
		filter := NewClusterStateFilter(config)

		tests := extensions.ExtensionTestSpecs{
			&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "normal test"}},
			&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [OCPFeatureGate:FeatureA]"}},
			&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [OCPFeatureGate:MissingFeature]"}},
			&extensions.ExtensionTestSpec{ExtensionTestSpec: &extensiontests.ExtensionTestSpec{Name: "test [OCPFeatureGate:DisabledFeature]"}},
		}

		result, err := filter.Filter(context.Background(), tests)

		require.NoError(t, err)
		assert.Len(t, result, 2) // Should filter out tests requiring missing or disabled features
		assert.Equal(t, "normal test", result[0].Name)
		assert.Equal(t, "test [OCPFeatureGate:FeatureA]", result[1].Name)
	})
}
