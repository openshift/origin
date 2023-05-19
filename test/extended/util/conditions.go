package util

import (
	"os"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
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

func BestStartTime() time.Time {
	start := LimitTestsToStartTime()
	if start.IsZero() {
		start = SuiteStartTime()
	}
	if start.IsZero() {
		start = time.Now().Add(-time.Hour)
	}
	return start
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
	start := BestStartTime()
	interval := now.Sub(start).Round(time.Second)
	switch {
	case interval < time.Minute:
		return time.Minute
	default:
		return interval
	}
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

// ServiceAccountHasSecrets returns true if the service account has at least one secret,
// false if it does not, or an error.
// This is originally from k8s.io/kubernetes/pkg/client/conditions/conditions.go, but it
// got removed in https://github.com/kubernetes/kubernetes/pull/115110.
func ServiceAccountHasSecrets(event watch.Event) (bool, error) {
	switch event.Type {
	case watch.Deleted:
		return false, errors.NewNotFound(schema.GroupResource{Resource: "serviceaccounts"}, "")
	}
	switch t := event.Object.(type) {
	case *v1.ServiceAccount:
		return len(t.Secrets) > 0, nil
	}
	return false, nil
}
