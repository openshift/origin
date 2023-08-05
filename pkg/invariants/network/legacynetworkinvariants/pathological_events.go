package legacynetworkinvariants

import (
	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/invariantlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/apimachinery/pkg/util/sets"
)

func testErrorUpdatingEndpointSlices(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	testName := "[sig-networking] pathological event should not see excessive FailedToUpdateEndpointSlices Error updating Endpoint Slices"

	return pathologicaleventlibrary.NewSingleEventCheckRegex(testName, duplicateevents.ErrorUpdatingEndpointSlicesRegex, duplicateevents.ErrorUpdatingEndpointSlicesFailedThreshold, duplicateevents.ErrorUpdatingEndpointSlicesFlakeThreshold).
		Test(events.Filter(monitorapi.IsInNamespaces(sets.NewString("openshift-ovn-kubernetes"))))
}
