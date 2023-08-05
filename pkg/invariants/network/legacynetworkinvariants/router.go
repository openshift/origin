package legacynetworkinvariants

import (
	"strings"
	"time"

	"github.com/openshift/origin/pkg/invariantlibrary/platformidentification"

	"github.com/openshift/origin/pkg/invariantlibrary/disruptionlibrary"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/openshift/origin/test/extended/util/disruption/externalservice"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestAllIngressBackendsForDisruption(events monitorapi.Intervals, jobRunDuration time.Duration, jobType *platformidentification.JobType) []*junitapi.JUnitTestCase {
	disruptLocators := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent)
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := monitorapi.DisruptionFrom(monitorapi.LocatorParts(eventInterval.Locator))
		if strings.HasPrefix(backend, "ingress-") {
			disruptLocators.Insert(eventInterval.Locator)
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, locator := range disruptLocators.List() {
		ret = append(ret, disruptionlibrary.TestServerAvailability("sig-network-edge", locator, allDisruptionEventsIntervals, jobRunDuration, jobType)...)
	}

	return ret
}

// TestExternalBackendsForDisruption runs synthetic tests for disruption backends that don't fit into the above two categories.
func TestExternalBackendsForDisruption(events monitorapi.Intervals, jobRunDuration time.Duration, jobType *platformidentification.JobType) []*junitapi.JUnitTestCase {
	disruptLocators := sets.String{}
	allDisruptionEventsIntervals := events.Filter(monitorapi.IsDisruptionEvent)
	for _, eventInterval := range allDisruptionEventsIntervals {
		backend := monitorapi.DisruptionFrom(monitorapi.LocatorParts(eventInterval.Locator))
		if backend == externalservice.LivenessProbeBackend {
			disruptLocators.Insert(eventInterval.Locator)
		}
	}

	ret := []*junitapi.JUnitTestCase{}
	for _, locator := range disruptLocators.List() {
		ret = append(ret, disruptionlibrary.TestServerAvailability("sig-trt", locator, allDisruptionEventsIntervals, jobRunDuration, jobType)...)
	}

	return ret
}
