package filters

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"

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
		flog := p.logger.WithField("filter", filter.Name())

		if !filter.ShouldApply() {
			flog.Debug("Skipping filter (not applicable)")
			continue
		}

		origCount := len(current)
		flog.WithField("before", origCount).
			Infof("Applying filter: %s", filter.Name())

		filtered, err := filter.Filter(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("filter %s failed: %w", filter.Name(), err)
		}

		filteredCount := len(filtered)
		removedCount := origCount - filteredCount
		flog.WithField("before", origCount).
			WithField("after", filteredCount).
			WithField("removed", removedCount).
			Infof("Filter %s completed - removed %d tests", filter.Name(), removedCount)

		current = filtered
	}

	p.logger.WithField("final_count", len(current)).
		Infof("Filter chain completed with %d tests", len(current))

	return current, nil
}
