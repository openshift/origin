package testsuites

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/test/ginkgo"
)

// SuitesString returns a string with the provided suites formatted. Prefix is
// printed at the beginning of the output.
func SuitesString(suites []*ginkgo.TestSuite, prefix string) string {
	buf := &bytes.Buffer{}
	fmt.Fprintf(buf, prefix)
	for _, suite := range suites {
		fmt.Fprintf(buf, "%s\n  %s\n\n", suite.Name, suite.Description)
	}
	return buf.String()
}

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

// isStandardEarlyTest returns true if a test is considered part of the normal
// pre or post condition tests.
func isStandardEarlyTest(name string) bool {
	if !strings.Contains(name, "[Early]") {
		return false
	}
	return strings.Contains(name, "[Suite:openshift/conformance/parallel")
}

// isStandardEarlyOrLateTest returns true if a test is considered part of the normal
// pre or post condition tests.
func isStandardEarlyOrLateTest(name string) bool {
	if !strings.Contains(name, "[Early]") && !strings.Contains(name, "[Late]") {
		return false
	}
	return strings.Contains(name, "[Suite:openshift/conformance/parallel")
}
