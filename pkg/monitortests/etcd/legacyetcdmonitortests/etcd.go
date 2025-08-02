package legacyetcdmonitortests

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

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

// etcdRequestsTookTooLongLimit is the max number of "took too long" etcd log message intervals we'll tolerate
// before we fail this test on the assumption etcd was simply not healthy through the run.
// Virtually all jobs log these messages at some point, we're just interested in the ones that do so excessively.
// At time of writing TRT's bigquery interals tables indicate that Azure and GCP can see values of 3-5k
// regularly, what we're worried about are the runs showing 30-70k.
const etcdRequestsTookTooLongLimit = 10000

func testEtcdDoesNotLogExcessiveTookTooLongMessages(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-etcd] etcd should not log excessive took too long messages"
	success := &junitapi.JUnitTestCase{Name: testName}

	counter := 0
	for _, event := range events {
		if event.Source == monitorapi.SourceEtcdLog &&
			strings.Contains(event.Message.HumanMessage, "took too long") {
			counter++
		}
	}

	if counter < etcdRequestsTookTooLongLimit {
		return []*junitapi.JUnitTestCase{success}
	}

	msg := fmt.Sprintf("Etcd logged %d 'took too long' messages, this test fails on any value over %d as "+
		"this is a strong indicator that etcd was very unhealthy throughout the run. This can cause sparodic e2e "+
		"failures and disruption and typically indicates faster disks are needed. These log message intervals are "+
		"included in spyglass chart artifacts and can be used to correlate with disruption and failed tests.",
		counter, etcdRequestsTookTooLongLimit)
	failure := &junitapi.JUnitTestCase{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			Output: msg,
		},
	}
	return []*junitapi.JUnitTestCase{failure}
}

// etcdOverloadedNetworkLimit is the max number of times etcd can log
//
//	dropped internal Raft message since sending buffer is full (overloaded network)
//
// before we fail this test. We've seen instances of total network failure due to this with up to
// 300k log lines.
const etcdOverloadedNetworkLimit = 10000

func testEtcdDoesNotLogExcessiveOverloadedNetworkMessages(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
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

	if counter < etcdOverloadedNetworkLimit {
		return []*junitapi.JUnitTestCase{success}
	}

	msg := fmt.Sprintf("Etcd logged %d 'overloaded network' messages, this test fails on any value over %d as "+
		"this is a strong indicator of a total network outage. Intervals charts for these runs will often show "+
		"mass sparodic disruption and subsequent failed tests.",
		counter, etcdOverloadedNetworkLimit)
	failure := &junitapi.JUnitTestCase{
		Name: testName,
		FailureOutput: &junitapi.FailureOutput{
			Output: msg,
		},
	}
	return []*junitapi.JUnitTestCase{failure}
}
