package intervalcreation

import (
	"sort"
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
		CreatePodIntervalsFromInstants,
	}
}

// InsertCalculatedIntervals calculates intervals from the currently known interval set and saves them into the same list
func InsertCalculatedIntervals(startingIntervals []monitorapi.EventInterval, recordedResources monitorapi.ResourcesMap, from, to time.Time) monitorapi.Intervals {
	ret := make([]monitorapi.EventInterval, 0, len(startingIntervals))
	copy(ret, startingIntervals)

	intervalCreationFns := defaultIntervalCreationFns()
	// create additional intervals from events
	for _, createIntervals := range intervalCreationFns {
		ret = append(ret, createIntervals(startingIntervals, recordedResources, from, to)...)
	}

	// we must sort the result
	sort.Sort(monitorapi.Intervals(ret))

	return ret
}
