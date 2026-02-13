package legacyetcdmonitortests

import (
	"regexp"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

// testRequiredInstallerResourcesMissing looks for this symptom:
//
//	reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-3
//
// and fails if it happens more than the failure threshold count of 20 and flakes more than the
// flake threshold.  See https://bugzilla.redhat.com/show_bug.cgi?id=2031564.
func testRequiredInstallerResourcesMissing(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[bz-etcd] pathological event should not see excessive RequiredInstallerResourcesMissing secrets"

	return pathologicaleventlibrary.NewSingleEventThresholdCheck(
		testName,
		pathologicaleventlibrary.EtcdRequiredResourcesMissing,
		pathologicaleventlibrary.DuplicateEventThreshold,
		pathologicaleventlibrary.RequiredResourceMissingFlakeThreshold,
	).Test(
		events.Filter(
			func(event monitorapi.Interval) bool {
				return event.Message.HumanMessageDoesNotMatchAny(
					// XXX configmap check-endpoints-config was added on 4.22 so
					// we expect events referring to it to show up when testing the
					// upgrade from 4.21.
					regexp.MustCompile(`^configmaps: check-endpoints-config-\d+$`),
				)
			},
		),
	)
}

func testOperatorStatusChanged(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pathological event OperatorStatusChanged condition does not occur too often"
	return pathologicaleventlibrary.EventExprMatchThresholdTest(testName, events,
		pathologicaleventlibrary.EtcdClusterOperatorStatusChanged,
		pathologicaleventlibrary.DuplicateEventThreshold)
}
