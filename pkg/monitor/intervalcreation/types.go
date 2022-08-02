package intervalcreation

import (
	"context"
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	ret := make([]monitorapi.EventInterval, len(startingIntervals))
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

// InsertIntervalsFromCluster contacts the cluster, retrieves information deemed pertinent, and creates intervals for them.
func InsertIntervalsFromCluster(ctx context.Context, kubeConfig *rest.Config, startingIntervals []monitorapi.EventInterval, recordedResources monitorapi.ResourcesMap, from, to time.Time) (monitorapi.Intervals, error) {
	ret := make([]monitorapi.EventInterval, len(startingIntervals))
	copy(ret, startingIntervals)

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return ret, err
	}

	allErrors := []error{}
	nodeEvents, err := IntervalsFromNodeLogs(ctx, kubeClient, from, to)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	ret = append(ret, nodeEvents...)

	// we must sort the result
	sort.Sort(monitorapi.Intervals(ret))

	return ret, utilerrors.NewAggregate(allErrors)
}
