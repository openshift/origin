package filters

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/extensions"
)

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

// isDisabled checks if a test is disabled based on its name
func isDisabled(name string) bool {
	if strings.Contains(name, "[Disabled") {
		return true
	}

	return shouldSkipUntil(name)
}

// shouldSkipUntil allows a test to be skipped with a time limit.
// the test should be annotated with the 'SkippedUntil' tag, as shown below.
//
//	[SkippedUntil:05092022:blocker-bz/123456]
//
// - the specified date should conform to the 'MMDDYYYY' format.
// - a valid blocker BZ must be specified
// if the specified date in the tag has not passed yet, the test
// will be skipped by the runner.
func shouldSkipUntil(name string) bool {
	re, err := regexp.Compile(`\[SkippedUntil:(\d{8}):blocker-bz\/([a-zA-Z0-9]+)\]`)
	if err != nil {
		// it should only happen with a programmer error and unit
		// test will prevent that
		return false
	}
	matches := re.FindStringSubmatch(name)
	if len(matches) != 3 {
		return false
	}

	dateStr := matches[1]
	// bzNumber := matches[2] // not used for now

	// parse the date
	date, err := time.Parse("01022006", dateStr)
	if err != nil {
		return false
	}

	// if the date has passed, don't skip the test
	return time.Now().Before(date)
}
