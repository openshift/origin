package filters

import (
	"context"

	"github.com/openshift/origin/pkg/test/extensions"
)

// MatchFnFilter applies a test matching function
type MatchFnFilter struct {
	matchFn func(name string) bool
}

func NewMatchFnFilter(matchFn func(name string) bool) *MatchFnFilter {
	return &MatchFnFilter{matchFn: matchFn}
}

func (f *MatchFnFilter) Name() string {
	return "match-function"
}

func (f *MatchFnFilter) Filter(ctx context.Context, tests extensions.ExtensionTestSpecs) (extensions.ExtensionTestSpecs, error) {
	if f.matchFn == nil {
		return tests, nil
	}

	matches := make(extensions.ExtensionTestSpecs, 0, len(tests))
	for _, test := range tests {
		if !f.matchFn(test.Name) {
			continue
		}
		matches = append(matches, test)
	}
	return matches, nil
}

func (f *MatchFnFilter) ShouldApply() bool {
	return f.matchFn != nil
}
