package filters

import (
	"context"
	"fmt"

	"github.com/openshift/origin/pkg/test/extensions"
)

// QualifiersFilter filters tests based on qualifiers (CEL expressions)
type QualifiersFilter struct {
	qualifiers []string
}

func NewQualifiersFilter(qualifiers []string) *QualifiersFilter {
	return &QualifiersFilter{
		qualifiers: qualifiers,
	}
}

func (f *QualifiersFilter) Name() string {
	return "suite-qualifiers"
}

// Filter filters tests based on suite qualifying CEL expressions.
func (f *QualifiersFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	if len(f.qualifiers) == 0 {
		return tests, nil
	}

	// Apply qualifier filtering directly to the ExtensionTestSpecs
	filteredSpecs, err := extensions.FilterWrappedSpecs(tests, f.qualifiers)
	if err != nil {
		return nil, fmt.Errorf("failed to filter tests by qualifiers: %w", err)
	}

	return filteredSpecs, nil
}

func (f *QualifiersFilter) ShouldApply() bool {
	return len(f.qualifiers) > 0
}
