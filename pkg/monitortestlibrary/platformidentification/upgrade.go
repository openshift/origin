package platformidentification

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func DidUpgradeHappenDuringCollection(intervals monitorapi.Intervals, beginning, end time.Time) bool {
	pertinentIntervals := intervals.Slice(beginning, end)

	for _, event := range pertinentIntervals {
		if event.Source != monitorapi.SourceKubeEvent || event.Locator.Keys[monitorapi.LocatorClusterVersionKey] != "cluster" {
			continue
		}
		reason := string(event.Message.Reason)
		if reason == "UpgradeStarted" || reason == "UpgradeRollback" {
			return true
		}
	}
	return false
}
