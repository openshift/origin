package legacynodemonitortests

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
)

const externalTopologyNodeStartupGracePeriod = 5 * time.Minute

func filterExternalTopologyStartupNodeIntervals(events monitorapi.Intervals, clusterData platformidentification.ClusterData, beginning time.Time) monitorapi.Intervals {
	if clusterData.Topology != "external" || beginning.IsZero() {
		return events
	}

	graceEnd := beginning.Add(externalTopologyNodeStartupGracePeriod)
	filtered := make(monitorapi.Intervals, 0, len(events))
	for _, event := range events {
		if !isExternalTopologyStartupNodeInterval(event) {
			filtered = append(filtered, event)
			continue
		}

		if !event.To.IsZero() && !event.To.After(graceEnd) {
			continue
		}
		if event.From.Before(graceEnd) {
			event.From = graceEnd
		}
		filtered = append(filtered, event)
	}

	return filtered
}

func isExternalTopologyStartupNodeInterval(event monitorapi.Interval) bool {
	if event.Message.Reason == monitorapi.PodReasonForceDelete &&
		event.Locator.Keys[monitorapi.LocatorNamespaceKey] == "kube-system" {
		return true
	}

	if event.Source == monitorapi.SourceKubeletLog && event.Message.Reason == monitorapi.FailedToAuthenticateWithOpenShiftUser {
		return true
	}

	return errImagePullQPSExceededRE.MatchString(event.String())
}
