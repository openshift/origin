package ginkgo

import (
	"context"
	"fmt"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/test/extensions"
)

// TestFilter represents a single filtering step in the chain
type TestFilter interface {
	// Name returns a human-readable name for this filter
	Name() string

	// Filter applies the filtering logic and returns the filtered tests
	Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error)

	// ShouldApply returns true if this filter should be applied
	ShouldApply() bool
}

// TestFilterChain manages a sequence of test filters
type TestFilterChain struct {
	filters []TestFilter
	logger  *logrus.Entry
}

// NewFilterChain creates a new filter chain
func NewFilterChain(logger *logrus.Entry) *TestFilterChain {
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}
	return &TestFilterChain{
		filters: make([]TestFilter, 0),
		logger:  logger,
	}
}

// AddFilter adds a filter to the chain
func (p *TestFilterChain) AddFilter(filter TestFilter) *TestFilterChain {
	p.filters = append(p.filters, filter)
	return p
}

// Apply runs all filters in sequence, logging each step
func (p *TestFilterChain) Apply(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	current := tests

	for _, filter := range p.filters {
		if !filter.ShouldApply() {
			p.logger.WithField("filter", filter.Name()).
				Debug("Skipping filter (not applicable)")
			continue
		}

		origCount := len(current)
		p.logger.WithField("filter", filter.Name()).
			WithField("before", origCount).
			Infof("Applying filter: %s", filter.Name())

		filtered, err := filter.Filter(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("filter %s failed: %w", filter.Name(), err)
		}

		filteredCount := len(filtered)
		removedCount := origCount - filteredCount
		p.logger.WithField("filter", filter.Name()).
			WithField("before", origCount).
			WithField("after", filteredCount).
			WithField("removed", removedCount).
			Infof("Filter %s completed - removed %d tests", filter.Name(), removedCount)

		current = filtered
	}

	p.logger.WithField("final_count", len(current)).
		Infof("Filter chain completed with %d tests", len(current))

	return current, nil
}

// DisabledTestsFilter filters out disabled tests
type DisabledTestsFilter struct{}

func (f *DisabledTestsFilter) Name() string {
	return "disabled-tests"
}

func (f *DisabledTestsFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	enabled := make(extensions.ExtensionTestSpecs, 0, len(tests))
	for _, test := range tests {
		if isDisabled(test.Name) {
			continue
		}
		enabled = append(enabled, test)
	}
	return enabled, nil
}

func (f *DisabledTestsFilter) ShouldApply() bool {
	return true
}

// SuiteMatcherFilter applies the suite's SuiteMatcher function
type SuiteMatcherFilter struct {
	suite *TestSuite
}

func NewSuiteMatcherFilter(suite *TestSuite) *SuiteMatcherFilter {
	return &SuiteMatcherFilter{suite: suite}
}

func (f *SuiteMatcherFilter) Name() string {
	return "suite-matcher"
}

func (f *SuiteMatcherFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	if f.suite.SuiteMatcher == nil {
		return tests, nil
	}

	matches := make(extensions.ExtensionTestSpecs, 0, len(tests))
	for _, test := range tests {
		if !f.suite.SuiteMatcher(test.Name) {
			continue
		}
		matches = append(matches, test)
	}
	return matches, nil
}

func (f *SuiteMatcherFilter) ShouldApply() bool {
	return f.suite.SuiteMatcher != nil
}

// ClusterStateFilter filters tests based on cluster environment
type ClusterStateFilter struct {
	clusterFilters func(string) bool
}

func NewClusterStateFilter(clusterFilters func(string) bool) *ClusterStateFilter {
	return &ClusterStateFilter{clusterFilters: clusterFilters}
}

func (f *ClusterStateFilter) Name() string {
	return "cluster-state"
}

func (f *ClusterStateFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	filtered := make(extensions.ExtensionTestSpecs, 0, len(tests))
	for _, test := range tests {
		if !f.clusterFilters(test.Name) {
			continue
		}
		filtered = append(filtered, test)
	}
	return filtered, nil
}

func (f *ClusterStateFilter) ShouldApply() bool {
	return f.clusterFilters != nil
}

// KubeRebaseTestsFilter filters out tests during k8s rebase
type KubeRebaseTestsFilter struct {
	options    *GinkgoRunSuiteOptions
	restConfig *rest.Config
}

func NewKubeRebaseTestsFilter(options *GinkgoRunSuiteOptions, restConfig *rest.Config) *KubeRebaseTestsFilter {
	return &KubeRebaseTestsFilter{
		options:    options,
		restConfig: restConfig,
	}
}

func (f *KubeRebaseTestsFilter) Name() string {
	return "kube-rebase-tests"
}

func (f *KubeRebaseTestsFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	return f.options.filterOutRebaseTestsFromSpecs(f.restConfig, tests)
}

func (f *KubeRebaseTestsFilter) ShouldApply() bool {
	return true
}

// SuiteQualifiersFilter filters tests based on suite qualifiers (CEL expressions)
type SuiteQualifiersFilter struct {
	suite     *TestSuite
	extension *extension.Extension
}

func NewSuiteQualifiersFilter(suite *TestSuite, extension *extension.Extension) *SuiteQualifiersFilter {
	return &SuiteQualifiersFilter{
		suite:     suite,
		extension: extension,
	}
}

func (f *SuiteQualifiersFilter) Name() string {
	return "suite-qualifiers"
}

// Filter filters tests based on suite qualifying CEL expressions.
func (f *SuiteQualifiersFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	if len(f.suite.Qualifiers) == 0 {
		return tests, nil
	}

	// Apply qualifier filtering directly to the ExtensionTestSpecs
	filteredSpecs, err := extensions.FilterWrappedSpecs(tests, f.suite.Qualifiers)
	if err != nil {
		return nil, fmt.Errorf("failed to filter tests by qualifiers: %w", err)
	}

	return filteredSpecs, nil
}

func (f *SuiteQualifiersFilter) ShouldApply() bool {
	return len(f.suite.Qualifiers) > 0
}
