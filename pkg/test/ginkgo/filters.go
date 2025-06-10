package ginkgo

import (
	"regexp"
	"strings"
	"time"
)

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

	skipUntil, err := time.Parse("01022006", matches[1])
	if err != nil {
		return false
	}

	if skipUntil.After(time.Now()) {
		return true
	}
	return false
}
