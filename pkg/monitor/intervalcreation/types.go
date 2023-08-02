package intervalcreation

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

type simpleIntervalCreationFunc func(intervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) monitorapi.Intervals

func defaultIntervalCreationFns() []simpleIntervalCreationFunc {
	return []simpleIntervalCreationFunc{
		IntervalsFromEvents_OperatorAvailable,
		IntervalsFromEvents_OperatorProgressing,
		IntervalsFromEvents_OperatorDegraded,
		IntervalsFromEvents_E2ETests,
		IntervalsFromEvents_NodeChanges,
	}
}

// CalculateMoreIntervals calculates intervals from the currently known interval set and saves them into the same list
func CalculateMoreIntervals(startingIntervals []monitorapi.Interval, recordedResources monitorapi.ResourcesMap, from, to time.Time) monitorapi.Intervals {
	ret := []monitorapi.Interval{}

	intervalCreationFns := defaultIntervalCreationFns()
	// create additional intervals from events
	for _, createIntervals := range intervalCreationFns {
		ret = append(ret, createIntervals(startingIntervals, recordedResources, from, to)...)
	}

	return ret
}
