package legacynetworkmonitortests

import (
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

func testErrorUpdatingEndpointSlices(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-networking] pathological event should not see excessive FailedToUpdateEndpointSlices Error updating Endpoint Slices"

	return pathologicaleventlibrary.NewSingleEventThresholdCheck(testName,
		pathologicaleventlibrary.ErrorUpdatingEndpointSlices,
		pathologicaleventlibrary.ErrorUpdatingEndpointSlicesFailedThreshold,
		pathologicaleventlibrary.ErrorUpdatingEndpointSlicesFlakeThreshold).
		Test(events.Filter(monitorapi.IsInNamespaces(sets.NewString("openshift-ovn-kubernetes"))))
}
