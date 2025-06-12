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
