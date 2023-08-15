package platformidentification

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func DidUpgradeHappenDuringCollection(intervals monitorapi.Intervals, beginning, end time.Time) bool {
	pertinentIntervals := intervals.Slice(beginning, end)

	for _, event := range pertinentIntervals {
		if monitorapi.LocatorParts(event.Locator)["clusterversion"] != "cluster" {
			continue
		}
		reason := monitorapi.ReasonFrom(event.Message)
		if reason == "UpgradeStarted" || reason == "UpgradeRollback" {
			return true
		}
	}
	return false
}
