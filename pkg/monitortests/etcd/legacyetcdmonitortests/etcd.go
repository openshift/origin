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

// etcdMaxRatePerFourHours is the max rate of messages allowed over a 4-hour period.
// This replaces the fixed limit approach with a rate-based approach.
// Virtually all jobs log these messages at some point, we're just interested in the ones that do so excessively.
const etcdMaxRatePerFourHours = 12000

func testEtcdDoesNotLogExcessiveTookTooLongMessages(events monitorapi.Intervals, startTime time.Time) []*junitapi.JUnitTestCase {
	const testName = "[sig-etcd] etcd should not log excessive took too long messages"
	success := &junitapi.JUnitTestCase{Name: testName}

	counter := 0
	for _, event := range events {
		if event.Source == monitorapi.SourceEtcdLog &&
			strings.Contains(event.Message.HumanMessage, "took too long") {
			counter++
		}
	}

	// Calculate rate-based limit: max 10000 messages in 4 hours, scaled by actual test duration
	maxAllowedCount := calculateRateBasedLimit(startTime, etcdMaxRatePerFourHours)
	actualDuration := time.Since(startTime)

	if counter <= maxAllowedCount {
		return []*junitapi.JUnitTestCase{success}
	}

	msg := fmt.Sprintf("Etcd logged %d 'took too long' messages in %v, exceeding the rate-based limit of %d "+
		"(based on max rate of %d messages per 4 hours). This is a strong indicator that etcd was very unhealthy "+
		"throughout the run. This can cause sporadic e2e failures and disruption and typically indicates faster "+
		"disks are needed. These log message intervals are included in spyglass chart artifacts and can be used "+
		"to correlate with disruption and failed tests.",
		counter, actualDuration.Round(time.Minute), maxAllowedCount, etcdMaxRatePerFourHours)
	failure := &junitapi.JUnitTestCase{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			Output: msg,
		},
	}
	return []*junitapi.JUnitTestCase{failure}
}

// etcdOverloadedNetworkMaxRatePerFourHours uses the same rate-based approach for overloaded network messages.
// We use the same rate limit as the "took too long" messages since both indicate severe etcd health issues.
//
// Before we fail this test. We've seen instances of total network failure due to this with up to
// 300k log lines.
const etcdOverloadedNetworkMaxRatePerFourHours = 20000

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

	// Calculate rate-based limit: max 10000 messages in 4 hours, scaled by actual test duration
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
	return []*junitapi.JUnitTestCase{failure}
}
