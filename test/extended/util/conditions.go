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
	s := os.Getenv("TEST_LIMIT_START_TIME")
	if len(s) == 0 {
		return time.Time{}
	}
	seconds, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}

// DurationSinceStartInSeconds returns the current time minus the start
// limit time, and if no start limit time is set it returns one hour. It
// also rounds the duration to seconds. Used by tests that want to look
// at a range such as prometheus metrics instead of hardcoding an interval.
// This function clamps the returned value to [time.Minute, 24*time.Hour].
func DurationSinceStartInSeconds() time.Duration {
	start := LimitTestsToStartTime()
	if start.IsZero() {
		return time.Hour
	}
	interval := time.Now().Sub(start).Round(time.Second)
	switch {
	case interval < 0:
		return time.Hour
	case interval < time.Minute:
		return time.Minute
	case interval > 24*time.Hour:
		return 24 * time.Hour
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
