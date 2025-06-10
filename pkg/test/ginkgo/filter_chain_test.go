package ginkgo

import (
	"context"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilters(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	logger.Logger.SetLevel(logrus.FatalLevel)

	tests := []*testCase{
		{name: "normal test"},
		{name: "test [Disabled:reason]"},
		{name: "another normal test"},
		{name: "test [Skipped:reason]"},
	}

	suite := &TestSuite{
		Name: "test-suite",
		SuiteMatcher: func(name string) bool {
			return name != "another normal test" && name != "test [Skipped:reason]"
		},
	}

	pipeline := NewFilterChain(logger).
		AddFilter(&DisabledTestsFilter{}).
		AddFilter(NewSuiteMatcherFilter(suite))

	result, err := pipeline.Apply(context.Background(), tests)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "normal test", result[0].name)
}

func TestDisabledTestsFilter(t *testing.T) {
	filter := &DisabledTestsFilter{}

	tests := []*testCase{
		{name: "normal test"},
		{name: "test [Disabled:reason]"},
		{name: "another normal test"},
		{name: "test [Skipped:reason]"}, // This won't be filtered by isDisabled
	}

	result, err := filter.Filter(context.Background(), tests)
	require.NoError(t, err)
	assert.Len(t, result, 3) // Only [Disabled:reason] is filtered out
	assert.Equal(t, "normal test", result[0].name)
	assert.Equal(t, "another normal test", result[1].name)
	assert.Equal(t, "test [Skipped:reason]", result[2].name)
}

func TestClusterStateFilter(t *testing.T) {
	clusterFilters := func(name string) bool {
		return !strings.Contains(name, "[Skipped:Disconnected]")
	}

	filter := NewClusterStateFilter(clusterFilters)

	tests := []*testCase{
		{name: "test one"},
		{name: "test two [Skipped:Disconnected]"},
		{name: "test three"},
	}

	result, err := filter.Filter(context.Background(), tests)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "test one", result[0].name)
	assert.Equal(t, "test three", result[1].name)
}

func TestClusterStateFilterShouldApply(t *testing.T) {
	filter := NewClusterStateFilter(func(string) bool { return true })
	assert.True(t, filter.ShouldApply())

	filterNil := NewClusterStateFilter(nil)
	assert.False(t, filterNil.ShouldApply())
}

func TestFilterPipelineErrorHandling(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	logger.Logger.SetLevel(logrus.FatalLevel) // Suppress logs during tests

	errorFilter := &testErrorFilter{}

	pipeline := NewFilterChain(logger).
		AddFilter(errorFilter)

	tests := []*testCase{{name: "test"}}

	result, err := pipeline.Apply(context.Background(), tests)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "test-error-filter failed")
}

type testErrorFilter struct{}

func (f *testErrorFilter) Name() string {
	return "test-error-filter"
}

func (f *testErrorFilter) Filter(ctx context.Context, tests []*testCase) ([]*testCase, error) {
	return nil, assert.AnError
}

func (f *testErrorFilter) ShouldApply() bool {
	return true
}
