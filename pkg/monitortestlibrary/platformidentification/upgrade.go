package platformidentification

import (
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func DidUpgradeHappenDuringCollection(intervals monitorapi.Intervals, beginning, end time.Time) bool {
	pertinentIntervals := intervals.Slice(beginning, end)

	for _, event := range pertinentIntervals {
		if event.StructuredLocator.Type != monitorapi.LocatorTypeKubeEvent || event.StructuredLocator.Keys[monitorapi.LocatorClusterVersionKey] != "cluster" {
			continue
		}
		reason := string(event.StructuredMessage.Reason)
		if reason == "UpgradeStarted" || reason == "UpgradeRollback" {
			return true
		}
	}
	return false
}
