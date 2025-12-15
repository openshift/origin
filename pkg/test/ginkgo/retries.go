package ginkgo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/sirupsen/logrus"
)

const (
	defaultRetryStrategy = "once"

	// Aggressive strategy constants:
	// won't attempt to retry tests that take longer than this -
	// openshift-tests 95th percentile is just a little over 3 minutes
	aggressiveMaximumTestDuration = 4 * time.Minute
	// won't attempt any retries in "catastrophic" runs > 5 failures
	aggressiveMaxDistinctTestFailures = 5
	// will retry the test up to this many times - but could be fewer
	aggressiveMaxRetries = 10
	// will consider a test flaky if it fails less than this many times
	aggressiveMinFailureThreshold = 3
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
	ShouldContinue(testCase *testCase, allAttempts []*testCase, nextAttemptNumber int) bool

	// DecideOutcome reports the final result of all attempts.
	DecideOutcome(attempts []*testCase) RetryOutcome
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

func (s *RetryOnceStrategy) ShouldContinue(testCase *testCase, allAttempts []*testCase, nextAttemptNumber int) bool {
	if nextAttemptNumber > 2 {
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

func (s *RetryOnceStrategy) DecideOutcome(attempts []*testCase) RetryOutcome {
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
		"[sig-instrumentation] Metrics should grab all metrics from kubelet /metrics/resource endpoint [Suite:openshift/conformance/parallel] [Suite:k8s]",                                                                                                                              // https://issues.redhat.com/browse/OCPBUGS-57477
		"[sig-network] Services should be rejected for evicted pods (no endpoints exist) [Suite:openshift/conformance/parallel] [Suite:k8s]",                                                                                                                                            // https://issues.redhat.com/browse/OCPBUGS-57665
		"[sig-node] Pods Extended Pod Container lifecycle evicted pods should be terminal [Suite:openshift/conformance/parallel] [Suite:k8s]",                                                                                                                                           // https://issues.redhat.com/browse/OCPBUGS-57658
		"[sig-cli] Kubectl logs all pod logs the Deployment has 2 replicas and each pod has 2 containers should get logs from each pod and each container in Deployment [Suite:openshift/conformance/parallel] [Suite:k8s]",                                                             // https://issues.redhat.com/browse/OCPBUGS-61287
		"[sig-cli] Kubectl Port forwarding Shutdown client connection while the remote stream is writing data to the port-forward connection port-forward should keep working after detect broken connection [Suite:openshift/conformance/parallel] [Suite:k8s]",                        // https://issues.redhat.com/browse/OCPBUGS-61734
		"[sig-storage] OCP CSI Volumes [Driver: csi-hostpath-groupsnapshot] [OCPFeatureGate:VolumeGroupSnapshot] [Testpattern:  (delete policy)] volumegroupsnapshottable [Feature:volumegroupsnapshot] VolumeGroupSnapshottable should create snapshots for multiple volumes in a pod", // https://issues.redhat.com/browse/OCPBUGS-66967
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
	return len(failing) > 0 && len(failing) <= aggressiveMaxDistinctTestFailures
}

func (s *AggressiveRetryStrategy) GetMaxRetries(testCase *testCase) int {
	// Skip retries for tests that exceed duration limit
	if testCase.duration >= aggressiveMaximumTestDuration {
		return 0
	}
	return s.maxRetries
}

func (s *AggressiveRetryStrategy) ShouldContinue(originalFailure *testCase, allAttempts []*testCase, nextAttemptNumber int) bool {
	// Stop if we've hit max attempts
	if nextAttemptNumber > s.maxRetries {
		logrus.Debugf("Stopping retry: max retries %d reached for %q", nextAttemptNumber-1, originalFailure.name)
		return false
	}

	// Skip retries for tests that exceed duration limit
	if originalFailure.duration >= aggressiveMaximumTestDuration {
		logrus.Debugf("Skipping retry: test duration %s exceeds limit %s", originalFailure.duration, aggressiveMaximumTestDuration)
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
		logrus.Debugf("Stopping retry: failure threshold %d reached for %q", failureCount, originalFailure.name)
		return false
	}

	// Early termination optimization: stop if it's mathematically impossible to reach failure threshold
	// Calculate remaining attempts and check if we can still accumulate enough failures
	// nextAttemptNumber is the current attempt we're considering (1-based)
	// maxRetries includes the original attempt, so total attempts possible = maxRetries
	remainingAttempts := s.maxRetries - nextAttemptNumber + 1
	maxPossibleFailures := failureCount + remainingAttempts

	// If even with all remaining attempts failing, we can't reach the threshold, stop early
	if maxPossibleFailures < s.failureThreshold {
		logrus.Debugf("Stopping retry: mathematically impossible to reach failure threshold %d for %q", s.failureThreshold, originalFailure.name)
		return false
	}

	// In multi-retry mode, continue until we reach max attempts or failure threshold
	return true
}

func (s *AggressiveRetryStrategy) DecideOutcome(attempts []*testCase) RetryOutcome {
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
func (s *NoRetryStrategy) ShouldAttemptRetries(_ []*testCase, _ *TestSuite) bool {
	return false
}
func (s *NoRetryStrategy) GetMaxRetries(_ *testCase) int { return 0 }
func (s *NoRetryStrategy) ShouldContinue(_ *testCase, _ []*testCase, _ int) bool {
	return false
}
func (s *NoRetryStrategy) DecideOutcome(_ []*testCase) RetryOutcome {
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

// writeRetryStatistics writes retry statistics to an autodl file for BigQuery upload
func writeRetryStatistics(testAttempts map[string][]*testCase, retryStrategy RetryStrategy, junitDir string) error {
	if len(testAttempts) == 0 {
		// No retries occurred, nothing to write
		return nil
	}

	// Create the data file structure
	dataFile := dataloader.DataFile{
		TableName: "retry_statistics",
		Schema: map[string]dataloader.DataType{
			"TestName":                           dataloader.DataTypeString,
			"RetryStrategy":                      dataloader.DataTypeString,
			"TotalAttempts":                      dataloader.DataTypeInteger,
			"SuccessfulAttempts":                 dataloader.DataTypeInteger,
			"FailedAttempts":                     dataloader.DataTypeInteger,
			"FinalOutcome":                       dataloader.DataTypeString,
			"TotalDurationMilliseconds":          dataloader.DataTypeInteger,
			"MaxRetriesAllowed":                  dataloader.DataTypeInteger,
			"FirstAttemptDurationMilliseconds":   dataloader.DataTypeInteger,
			"AverageAttemptDurationMilliseconds": dataloader.DataTypeInteger,

			// Data from CI environment variables, so we don't have to join on Jobs table in BigQuery.
			"JobName":             dataloader.DataTypeString,
			"JobType":             dataloader.DataTypeString,
			"PullNumber":          dataloader.DataTypeString,
			"RepoName":            dataloader.DataTypeString,
			"RepoOwner":           dataloader.DataTypeString,
			"PullSha":             dataloader.DataTypeString,
			"ReleaseImageLatest":  dataloader.DataTypeString,
			"ReleaseImageInitial": dataloader.DataTypeString,
		},
		Rows: []map[string]string{},
	}

	// Process each test's retry attempts
	for testName, attempts := range testAttempts {
		if len(attempts) == 0 {
			continue
		}

		// Calculate statistics
		totalAttempts := len(attempts)
		successfulAttempts := 0
		failedAttempts := 0
		var totalDuration time.Duration

		for _, attempt := range attempts {
			totalDuration += attempt.duration
			if attempt.success {
				successfulAttempts++
			} else if attempt.failed {
				failedAttempts++
			}
		}

		// Determine final outcome using the retry strategy
		outcome := retryStrategy.DecideOutcome(attempts)
		var finalOutcomeStr string
		switch outcome {
		case RetryOutcomeFlaky:
			finalOutcomeStr = "flaky"
		case RetryOutcomeFail:
			finalOutcomeStr = "failed"
		case RetryOutcomeSkipped:
			finalOutcomeStr = "skipped"
		default:
			finalOutcomeStr = "unknown"
		}

		// Get max retries allowed for this test
		maxRetries := retryStrategy.GetMaxRetries(attempts[0])

		// Calculate average attempt duration in milliseconds
		averageDurationMs := totalDuration.Milliseconds() / int64(totalAttempts)

		// Create row data
		row := map[string]string{
			"TestName":                           testName,
			"RetryStrategy":                      retryStrategy.Name(),
			"TotalAttempts":                      strconv.Itoa(totalAttempts),
			"SuccessfulAttempts":                 strconv.Itoa(successfulAttempts),
			"FailedAttempts":                     strconv.Itoa(failedAttempts),
			"FinalOutcome":                       finalOutcomeStr,
			"TotalDurationMilliseconds":          strconv.FormatInt(totalDuration.Milliseconds(), 10),
			"MaxRetriesAllowed":                  strconv.Itoa(maxRetries),
			"FirstAttemptDurationMilliseconds":   strconv.FormatInt(attempts[0].duration.Milliseconds(), 10),
			"AverageAttemptDurationMilliseconds": strconv.FormatInt(averageDurationMs, 10),
			"JobName":                            os.Getenv("JOB_NAME"),
			"JobType":                            os.Getenv("JOB_TYPE"),
			"PullNumber":                         os.Getenv("PULL_NUMBER"),
			"RepoName":                           os.Getenv("REPO_NAME"),
			"RepoOwner":                          os.Getenv("REPO_OWNER"),
			"PullSha":                            os.Getenv("PULL_PULL_SHA"),
			"ReleaseImageLatest":                 os.Getenv("RELEASE_IMAGE_LATEST"),
			"ReleaseImageInitial":                os.Getenv("RELEASE_IMAGE_INITIAL"),
		}

		dataFile.Rows = append(dataFile.Rows, row)
	}

	// Generate filename with timestamp
	timeSuffix := time.Now().UTC().Format("20060102-150405")
	filename := filepath.Join(junitDir, fmt.Sprintf("retry-statistics-%s-"+dataloader.AutoDataLoaderSuffix, timeSuffix))

	// Write the autodl file
	err := dataloader.WriteDataFile(filename, dataFile)
	if err != nil {
		return fmt.Errorf("failed to write retry statistics autodl file %s: %w", filename, err)
	}

	logrus.Infof("Wrote retry statistics for %d tests to %s", len(dataFile.Rows), filename)
	return nil
}
