// Package preconditions provides infrastructure for detecting and reporting cluster
// precondition failures in test suites.
//
// Tests use Log() to indicate precondition checking is being performed, and
// FormatSkipMessage() when a precondition fails. After all tests complete, the
// test runner scans test output for these markers and generates a single synthetic
// JUnit entry:
//   - PASSES if precondition checks ran but no tests were skipped
//   - FAILS if any tests were skipped due to unmet preconditions
//
// This provides the Technical Release Team (TRT) with a consistent test name that has
// meaningful pass/fail rates, rather than generating per-skip failures that would always
// have 100% failure rate.
package preconditions

import "fmt"

// SyntheticTestName is the consistent name used for the precondition validation JUnit entry.
// This name appears in every run where precondition checks were invoked, enabling the
// Technical Release Team (TRT) to track meaningful pass/fail rates.
const SyntheticTestName = "[openshift-tests] cluster precondition validation"

// CheckMarker is the prefix used to identify that precondition checking was invoked.
// When this marker appears in test output, the synthetic JUnit entry will be generated.
const CheckMarker = "[precondition-check]"

// SkipMarker is the prefix used in skip messages to identify precondition failures.
// When this marker appears in test output, the synthetic JUnit entry will fail.
const SkipMarker = "[precondition-skip]"

// RecordCheck returns a log message prefixed with CheckMarker.
// This MUST be called at the start of any precondition check to signal that
// precondition validation is being performed. Without this marker, the synthetic
// JUnit entry will not be generated.
// Supports printf-style formatting.
//
//	framework.Logf("%s", preconditions.RecordCheck("validating cluster health"))
//	framework.Logf("%s", preconditions.RecordCheck("checking topology is %s", wanted))
func RecordCheck(format string, args ...interface{}) string {
	message := fmt.Sprintf(format, args...)
	return fmt.Sprintf("%s %s", CheckMarker, message)
}

// FormatSkipMessage returns a skip message prefixed with SkipMarker.
// Use this when a precondition check fails and the test should be skipped.
//
//	e2eskipper.Skip(preconditions.FormatSkipMessage("etcd not healthy"))
func FormatSkipMessage(reason string) string {
	return fmt.Sprintf("%s %s", SkipMarker, reason)
}
