package ginkgo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultRetryStrategy = "once"

	// Aggressive strategy constants:
	// won't attempt to retry tests that take longer than this
	aggressiveMaximumTestDuration = 2 * time.Minute
	// won't attempt any retries in "catastrophic" runs > 5 failures
	aggressiveMaxTotalTestFailures = 5
	// will retry the test up to this many times - but could be fewer
	aggressiveMaxRetries = 10
	// will consider a test flaky if it fails less than this many times
	aggressiveMinFailureThreshold = 4
)

// RetryOutcome represents the decision for a multi-retry test
type RetryOutcome int

const (
	RetryOutcomeFail RetryOutcome = iota
	RetryOutcomeFlaky
	RetryOutcomeSkipped
)

// RetryStrategy controls both retry behavior and final outcome decisions
// Example usage:
//
//	options.RetryStrategy = NewRetryOnceStrategy()                          // Restrictive retry with rules
//	options.RetryStrategy = NewAggressiveRetryStrategy(10, 4)               // Aggressive multiple retries
type RetryStrategy interface {
	// Name returns the strategy name for CLI and logging
	Name() string

	// ShouldAttemptRetries determines if we attempt any retries given the list of failing tests
	ShouldAttemptRetries(failing []*testCase, suite *TestSuite) bool

	// GetMaxRetries returns the upper bound of retries (for reporting/planning)
	GetMaxRetries(testCase *testCase) int

	// ShouldContinue determines if we continue retrying. Allows for early termination of retries.
	ShouldContinue(testCase *testCase, allAttempts []*testCase, attemptNumber int) bool

	// DecideOutcome reports the final result of all attempts.
	DecideOutcome(testName string, attempts []*testCase) RetryOutcome
}

type RetryOnceStrategy struct {
	PermittedRetryImageTags []string
}

func NewRetryOnceStrategy() *RetryOnceStrategy {
	return &RetryOnceStrategy{
		PermittedRetryImageTags: []string{"tests"}, // tests = openshift-tests image
	}
}

func (s *RetryOnceStrategy) Name() string {
	return "once"
}

func (s *RetryOnceStrategy) ShouldAttemptRetries(failing []*testCase, suite *TestSuite) bool {
	return len(failing) > 0 && len(failing) <= suite.MaximumAllowedFlakes
}

func (s *RetryOnceStrategy) GetMaxRetries(testCase *testCase) int {
	if s.shouldRetryTest(testCase) {
		return 1
	}
	return 0
}

func (s *RetryOnceStrategy) ShouldContinue(testCase *testCase, allAttempts []*testCase, attemptNumber int) bool {
	// Stop after first retry
	if attemptNumber >= 2 {
		return false
	}

	// Check if test is eligible for retry based on image restrictions
	if !s.shouldRetryTest(testCase) {
		return false
	}

	// Allow one retry for failed tests
	lastAttempt := allAttempts[len(allAttempts)-1]
	return lastAttempt.failed
}

func (s *RetryOnceStrategy) DecideOutcome(testName string, attempts []*testCase) RetryOutcome {
	for _, attempt := range attempts {
		if attempt.skipped {
			return RetryOutcomeSkipped
		}
		if attempt.success {
			return RetryOutcomeFlaky
		}
	}
	return RetryOutcomeFail
}

func (s *RetryOnceStrategy) shouldRetryTest(test *testCase) bool {
	// Internal tests (no binary) are eligible for retry, we shouldn't really have any of these
	// now that origin is also an extension.
	if test.binary == nil {
		return true
	}

	tlog := logrus.WithField("test", test.name)

	// Test retries were disabled for some suites when they moved to OTE. This exposed small numbers of tests that
	// were actually flaky and nobody knew. We attempted to fix these, a few did not make it in time. Restore
	// retries for specific test names so the overall suite can continue to not retry.
	retryTestNames := []string{
		"[sig-instrumentation] Metrics should grab all metrics from kubelet /metrics/resource endpoint [Suite:openshift/conformance/parallel] [Suite:k8s]", // https://issues.redhat.com/browse/OCPBUGS-57477
		"[sig-network] Services should be rejected for evicted pods (no endpoints exist) [Suite:openshift/conformance/parallel] [Suite:k8s]",               // https://issues.redhat.com/browse/OCPBUGS-57665
		"[sig-node] Pods Extended Pod Container lifecycle evicted pods should be terminal [Suite:openshift/conformance/parallel] [Suite:k8s]",              // https://issues.redhat.com/browse/OCPBUGS-57658
	}
	for _, rtn := range retryTestNames {
		if test.name == rtn {
			tlog.Debug("test has an exception allowing retry")
			return true
		}
	}

	// Get extension info to check if it's from a permitted image
	info, err := test.binary.Info(context.Background())
	if err != nil {
		tlog.WithError(err).
			Debug("Failed to get binary info, skipping retry")
		return false
	}

	// Check if the test's source image is in the permitted retry list
	for _, permittedTag := range s.PermittedRetryImageTags {
		if strings.Contains(info.Source.SourceImage, permittedTag) {
			tlog.WithField("image", info.Source.SourceImage).
				Debug("Permitting retry")
			return true
		}
	}

	tlog.WithField("image", info.Source.SourceImage).
		Debug("Test not eligible for retry based on image tag")
	return false
}

// AggressiveRetryStrategy implements the multiple retry behavior with fixed failure threshold
type AggressiveRetryStrategy struct {
	maxRetries       int
	failureThreshold int
}

func NewAggressiveRetryStrategy(maxRetries, failureThreshold int) *AggressiveRetryStrategy {
	return &AggressiveRetryStrategy{
		maxRetries:       maxRetries,
		failureThreshold: failureThreshold,
	}
}

func (s *AggressiveRetryStrategy) Name() string {
	return "aggressive"
}

func (s *AggressiveRetryStrategy) ShouldAttemptRetries(failing []*testCase, suite *TestSuite) bool {
	return len(failing) > 0 && len(failing) <= aggressiveMaxTotalTestFailures
}

func (s *AggressiveRetryStrategy) GetMaxRetries(testCase *testCase) int {
	// Skip retries for tests that exceed duration limit
	if testCase.duration >= aggressiveMaximumTestDuration {
		return 0
	}
	return s.maxRetries
}

func (s *AggressiveRetryStrategy) ShouldContinue(testCase *testCase, allAttempts []*testCase, attemptNumber int) bool {
	// Stop if we've hit max attempts
	if attemptNumber > s.maxRetries {
		return false
	}

	// Skip retries for tests that exceed duration limit
	if testCase.duration >= aggressiveMaximumTestDuration {
		return false
	}

	// Count current failures to avoid unnecessary retries
	failureCount := 0
	for _, attempt := range allAttempts {
		if attempt.failed {
			failureCount++
		}
	}

	// Stop retrying if we've already reached the failure threshold - we know the test will fail
	if failureCount >= s.failureThreshold {
		return false
	}

	// In multi-retry mode, continue until we reach max attempts or failure threshold
	return true
}

func (s *AggressiveRetryStrategy) DecideOutcome(testName string, attempts []*testCase) RetryOutcome {
	failureCount := 0
	skippedCount := 0

	for _, attempt := range attempts {
		if attempt.failed {
			failureCount++
		} else if attempt.skipped {
			skippedCount++
		}
	}

	// Only consider skipped if majority of attempts were skipped
	if skippedCount > len(attempts)/2 {
		return RetryOutcomeSkipped
	}

	if failureCount < s.failureThreshold {
		return RetryOutcomeFlaky
	}

	return RetryOutcomeFail
}

type NoRetryStrategy struct{}

func (s *NoRetryStrategy) Name() string { return "none" }
func (s *NoRetryStrategy) ShouldAttemptRetries(failing []*testCase, suite *TestSuite) bool {
	return false
}
func (s *NoRetryStrategy) GetMaxRetries(testCase *testCase) int { return 0 }
func (s *NoRetryStrategy) ShouldContinue(testCase *testCase, allAttempts []*testCase, attemptNumber int) bool {
	return false
}
func (s *NoRetryStrategy) DecideOutcome(testName string, attempts []*testCase) RetryOutcome {
	panic("Unexpected call to NoRetryStrategy.DecideOutcome - this should never happen")
}

func getAvailableRetryStrategies() []string {
	return []string{"once", "aggressive", "none"}
}

func createRetryStrategy(name string) (RetryStrategy, error) {
	switch name {
	case "once":
		return NewRetryOnceStrategy(), nil
	case "aggressive":
		return NewAggressiveRetryStrategy(aggressiveMaxRetries, aggressiveMinFailureThreshold), nil
	case "none":
		return &NoRetryStrategy{}, nil
	default:
		return nil, fmt.Errorf("unknown retry strategy: %s (available: %v)", name, getAvailableRetryStrategies())
	}
}
