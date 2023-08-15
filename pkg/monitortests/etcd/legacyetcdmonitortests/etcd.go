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
		if strings.Contains(event.Message, "slow fdatasync") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
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
		if strings.Contains(event.Message, "dropped internal Raft message since sending buffer is full") {
			failures = append(failures, fmt.Sprintf("%v - %v", event.Locator, event.Message))
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
