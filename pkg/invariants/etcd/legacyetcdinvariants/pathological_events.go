package legacyetcdinvariants

import (
	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/invariantlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
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
	return pathologicaleventlibrary.NewSingleEventCheckRegex(testName, duplicateevents.RequiredResourcesMissingRegEx, duplicateevents.DuplicateEventThreshold, duplicateevents.RequiredResourceMissingFlakeThreshold).Test(events)
}

func testOperatorStatusChanged(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] pathological event OperatorStatusChanged condition does not occur too often"
	return pathologicaleventlibrary.EventExprMatchThresholdTest(testName, events, duplicateevents.OperatorStatusChanged, duplicateevents.DuplicateEventThreshold)
}
