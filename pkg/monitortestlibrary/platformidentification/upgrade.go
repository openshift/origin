package platformidentification

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func DidUpgradeHappenDuringCollection(intervals monitorapi.Intervals, beginning, end time.Time) bool {
	return UpgradeNumberDuringCollection(intervals, beginning, end) > 0
}

func UpgradeNumberDuringCollection(intervals monitorapi.Intervals, beginning, end time.Time) int {
	var ret int
	pertinentIntervals := intervals.Slice(beginning, end)

	for _, event := range pertinentIntervals {
		if event.Source != monitorapi.SourceKubeEvent || event.Locator.Keys[monitorapi.LocatorClusterVersionKey] != "cluster" {
			continue
		}
		reason := event.Message.Reason
		if reason == monitorapi.UpgradeStartedReason || reason == monitorapi.UpgradeRollbackReason {
			ret++
		}
	}
	return ret
}
