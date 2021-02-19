package util

import (
	"os"
	"strconv"
	"time"
)

// LimitTestsToStartTime returns a time.Time which should be the earliest
// that a test in the e2e suite will consider. By default this is the empty
// Time object, and if TEST_LIMIT_START_TIME is set to a unix timestamp in
// seconds from the epoch, will return that time.
//
// Tests should use this when looking at the historical record for failures -
// events that happen before this time are considered to be ignorable.
//
// Disruptive tests use this value to signal when the disruption has healed
// so that normal conformance results are not impacted.
func LimitTestsToStartTime() time.Time {
	return unixTimeFromEnv("TEST_LIMIT_START_TIME")
}

// SuiteStartTime returns a time.Time which is the beginning of the suite
// execution (the set of tests). If this is empty or invalid the Time will
// be the zero value.
//
// Tests that need to retrieve results newer than the suite start because
// they are only checking the behavior of the system while the suite is being
// run should use this time as the start of that interval.
func SuiteStartTime() time.Time {
	return unixTimeFromEnv("TEST_SUITE_START_TIME")
}

// DurationSinceStartInSeconds returns the current time minus the start
// limit time or suite time, or one hour by default. It rounds the duration
// to seconds. It always returns a one minute duration or longer, even if
// the start time is in the future.
//
// This method simplifies the default behavior of tests that check metrics
// from the current time to the preferred start time (either implicit via
// the suite start time, or explicit via TEST_LIMIT_START_TIME). Depending
// on the suite a user is running, if the invariants the suite tests should
// hold BEFORE the suite is run, then TEST_LIMIT_START_TIME should be set
// to that time, and tests will automatically check that interval as well.
func DurationSinceStartInSeconds() time.Duration {
	now := time.Now()
	start := LimitTestsToStartTime()
	if start.IsZero() {
		start = SuiteStartTime()
	}
	if start.IsZero() {
		start = now.Add(-time.Hour)
	}
	interval := now.Sub(start).Round(time.Second)
	switch {
	case interval < time.Minute:
		return time.Minute
	default:
		return interval
	}
}

// TolerateVersionSkewInTests returns true if the test invoker has indicated
// that version skew is known present via the TEST_UNSUPPORTED_ALLOW_VERSION_SKEW
// environment variable. Tests that may fail if the component is skewed may then
// be skipped or alter their behavior. The version skew is assumed to be one minor
// release - for instance, if a test would fail because the previous version of
// the kubelet does not yet support a value, when this function returns true it
// would be acceptable for the test to invoke Skipf().
//
// Used by the node version skew environment (Kube @ version N, nodes @ version N-1)
// to ensure OpenShift works when control plane and worker nodes are not updated,
// as can occur during test suites.
func TolerateVersionSkewInTests() bool {
	if len(os.Getenv("TEST_UNSUPPORTED_ALLOW_VERSION_SKEW")) > 0 {
		return true
	}
	return false
}

// unixTimeFromEnv reads a unix timestamp in seconds since the epoch and
// returns a Time object. If no env var is set or if it does not parse, a
// zero time is returned.
func unixTimeFromEnv(name string) time.Time {
	s := os.Getenv(name)
	if len(s) == 0 {
		return time.Time{}
	}
	seconds, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}
