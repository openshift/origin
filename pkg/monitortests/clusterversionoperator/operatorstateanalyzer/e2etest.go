package operatorstateanalyzer

import (
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

// E2ETestEventIntervals returns only Intervals for e2e tests
func E2ETestEventIntervals(events monitorapi.Intervals) monitorapi.Intervals {
	e2eEventIntervals := monitorapi.Intervals{}
	for i := range events {
		event := events[i]
		if event.From == event.To {
			continue
		}
		if !monitorapi.IsE2ETest(event.Locator) {
			continue
		}
		e2eEventIntervals = append(e2eEventIntervals, event)
	}
	return e2eEventIntervals
}
