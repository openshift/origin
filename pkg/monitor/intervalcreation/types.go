package intervalcreation

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitor/apiserveravailability"

	"github.com/openshift/origin/pkg/monitor/nodedetails"

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

// IntervalsFromCluster contacts the cluster, retrieves information deemed pertinent, and creates intervals for them.
func IntervalsFromCluster(ctx context.Context, kubeConfig *rest.Config, from, to time.Time) (*nodedetails.AuditLogSummary, monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}

	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, ret, err
	}

	allErrors := []error{}

	podLogIntervals, err := IntervalsFromPodLogs(kubeClient, from, to)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	ret = append(ret, podLogIntervals...)

	apiserverAvailabilityIntervals, err := apiserveravailability.APIServerAvailabilityIntervalsFromCluster(kubeClient, from, to)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	ret = append(ret, apiserverAvailabilityIntervals...)

	auditLogSummary, auditEvents, err := IntervalsFromAuditLogs(ctx, kubeClient, from, to)
	if err != nil {
		allErrors = append(allErrors, err)
	}
	ret = append(ret, auditEvents...)

	return auditLogSummary, ret, utilerrors.NewAggregate(allErrors)
}
