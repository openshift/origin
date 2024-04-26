package alerts

import (
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/prometheus/common/model"
)

type MetricCondition struct {
	AlertName      string
	AlertNamespace string
	AlertLevel     string

	// Text is the description of why this alert condition matched.
	Text string

	Matches func(sample *model.Sample) bool
}

type MetricConditions []MetricCondition

func (c MetricConditions) MatchesInterval(alertInterval monitorapi.Interval) *MetricCondition {

	if alertInterval.Source != monitorapi.SourceAlert {
		return nil
	}

	checkAlertName := alertInterval.Locator.Keys[monitorapi.LocatorAlertKey]
	checkAlertNamespace := alertInterval.Locator.Keys[monitorapi.LocatorNamespaceKey]

	for _, condition := range c {
		matches := true
		// We can assume AlertName is set:
		if condition.AlertName != checkAlertName {
			matches = false
			// But Namespace may not be:
		} else if condition.AlertNamespace != "" && condition.AlertNamespace != checkAlertNamespace {
			matches = false
		}

		if matches {
			return &condition
		}
	}
	return nil
}
