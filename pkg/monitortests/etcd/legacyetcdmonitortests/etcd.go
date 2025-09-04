package legacyetcdmonitortests

import (
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

// calculateRateBasedLimit calculates the allowed message count based on elapsed time and rate
// This helper function makes the calculation logic testable and consistent across both functions
func calculateRateBasedLimit(startTime time.Time, maxRatePerFourHours int) int {
	actualDuration := time.Since(startTime)
	// Four hours is chosen just as a ballpark point at which we're concerned about the rate of messages in
	// a standard run.
	fourHours := 4 * time.Hour
	maxAllowedCount := int(float64(maxRatePerFourHours) * (float64(actualDuration) / float64(fourHours)))

	// Ensure we have at least some minimum threshold even for very short runs
	if maxAllowedCount < 100 {
		maxAllowedCount = 100
	}

	return maxAllowedCount
}

func testEtcdShouldNotLogSlowFdataSyncs(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-etcd] etcd pod logs do not log slow fdatasync"
	success := &junitapi.JUnitTestCase{Name: testName}

	var failures []string
	for _, event := range events {
		if strings.Contains(event.Message.HumanMessage, "slow fdatasync") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("etcd pod logs indicated a slow fdatasync "+
				"%d times. This may imply a disk i/o problem on the master.\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}
	// TODO: marked flaky until we have monitored it for consistency
	return []*junitapi.JUnitTestCase{failure, success}
}

func testEtcdShouldNotLogDroppedRaftMessages(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-etcd] etcd pod logs do not log dropped internal Raft messages"
	success := &junitapi.JUnitTestCase{Name: testName}

	var failures []string
	for _, event := range events {
		if strings.Contains(event.Message.HumanMessage, "dropped internal Raft message since sending buffer is full") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("etcd pod logs indicated dropped internal Raft messages"+
				"%d times.\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}
	// TODO: marked flaky until we have monitored it for consistency
	return []*junitapi.JUnitTestCase{failure, success}
}

// etcdOverloadedNetworkMaxRatePerFourHours uses the same rate-based approach for overloaded network messages.
// We use the same rate limit as the "took too long" messages since both indicate severe etcd health issues.
//
// Before we fail this test. We've seen instances of total network failure due to this with up to
// 300k log lines.
const etcdOverloadedNetworkMaxRatePerFourHours = 40000

func testEtcdDoesNotLogExcessiveOverloadedNetworkMessages(events monitorapi.Intervals, startTime time.Time) []*junitapi.JUnitTestCase {
	const testName = "[sig-etcd] etcd should not log excessive overloaded network messages"
	success := &junitapi.JUnitTestCase{Name: testName}

	counter := 0
	for _, event := range events {
		if event.Source == monitorapi.SourceEtcdLog &&
			// Actual message: dropped internal Raft message since sending buffer is full (overloaded network)
			strings.Contains(event.Message.HumanMessage, "overloaded network") {
			counter++
		}
	}

	maxAllowedCount := calculateRateBasedLimit(startTime, etcdOverloadedNetworkMaxRatePerFourHours)
	actualDuration := time.Since(startTime)

	if counter <= maxAllowedCount {
		return []*junitapi.JUnitTestCase{success}
	}

	msg := fmt.Sprintf("Etcd logged %d 'overloaded network' messages in %v, exceeding the rate-based limit of %d "+
		"(based on max rate of %d messages per 4 hours). This is a strong indicator of a total network outage. "+
		"Intervals charts for these runs will often show mass sporadic disruption and subsequent failed tests.",
		counter, actualDuration.Round(time.Minute), maxAllowedCount, etcdOverloadedNetworkMaxRatePerFourHours)
	failure := &junitapi.JUnitTestCase{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			Output: msg,
		},
	}
	// TODO: marked flaky, this was appearing too often on AWS and GCP, in runs
	// where nothing actually failed.
	return []*junitapi.JUnitTestCase{failure, success}
}
