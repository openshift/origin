package operatorstateanalyzer

import (
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/sirupsen/logrus"
)

func intervalsFromEvents_OperatorAvailable(intervals monitorapi.Intervals, _ monitorapi.ResourcesMap, beginning, end time.Time) monitorapi.Intervals {
	return intervalsFromEvents_OperatorStatus(intervals, beginning, end, configv1.OperatorAvailable, configv1.ConditionTrue, monitorapi.Error)
}

func intervalsFromEvents_OperatorProgressing(intervals monitorapi.Intervals, _ monitorapi.ResourcesMap, beginning, end time.Time) monitorapi.Intervals {
	return intervalsFromEvents_OperatorStatus(intervals, beginning, end, configv1.OperatorProgressing, configv1.ConditionFalse, monitorapi.Warning)
}

func intervalsFromEvents_OperatorDegraded(intervals monitorapi.Intervals, _ monitorapi.ResourcesMap, beginning, end time.Time) monitorapi.Intervals {
	return intervalsFromEvents_OperatorStatus(intervals, beginning, end, configv1.OperatorDegraded, configv1.ConditionFalse, monitorapi.Error)
}

func intervalsFromEvents_OperatorStatus(intervals monitorapi.Intervals, beginning, end time.Time, conditionType configv1.ClusterStatusConditionType, conditionGoodState configv1.ConditionStatus, level monitorapi.IntervalLevel) monitorapi.Intervals {
	ret := monitorapi.Intervals{}
	operatorToInterestingBadCondition := map[string]*configv1.ClusterOperatorStatusCondition{}

	for _, event := range intervals {
		if event.Source != monitorapi.SourceClusterOperatorMonitor {
			continue
		}
		operatorName := event.Locator.Keys[monitorapi.LocatorClusterOperatorKey]
		logrus.WithField("event", event).WithField("operator", operatorName).
			Infof("operator status: processing event")

		currentCondition := monitorapi.GetOperatorConditionStatus(event)
		if currentCondition == nil {
			logrus.Info("currentCondition came back nil, event does not appear to be a condition")
			continue
		}
		logrus.Infof("checking if currentCondition.Type %v != %v", currentCondition.Type, conditionType)
		if currentCondition.Type != conditionType {
			logrus.Info("condition types not equal")
			continue
		}

		lastCondition := operatorToInterestingBadCondition[operatorName]
		if lastCondition != nil && lastCondition.Status == currentCondition.Status {
			// if the status didn't actually change (imagine degraded just changing reasons)
			// don't count as the interval
			continue
		}
		if currentCondition.Status != conditionGoodState {
			// don't overwrite a previous condition in a bad State
			if lastCondition == nil {
				// force the last transition time, since we think we just transitioned at this instant
				currentCondition.LastTransitionTime.Time = event.From
				operatorToInterestingBadCondition[operatorName] = currentCondition
				logrus.WithField("currentCondition", currentCondition).
					WithField("operatorName", operatorName).
					Info("mapped to bad condition")

			}
			continue
		}

		// at this point we have transitioned to a good State.  Remove the previous "bad" State
		delete(operatorToInterestingBadCondition, operatorName)
		logrus.WithField("operatorName", operatorName).Info("cleared bad condition")

		from := beginning
		lastStatus := "Unknown"
		lastMessage := "Unknown"
		lastReason := "Unknown"
		if lastCondition != nil {
			from = lastCondition.LastTransitionTime.Time
			lastStatus = fmt.Sprintf("%v", lastCondition.Status)
			lastMessage = lastCondition.Message
			lastReason = lastCondition.Reason
		} else {
			// if we're in a good State now, then we were probably in a bad State before.  Let's start by assuming that anyway
			if conditionGoodState == configv1.ConditionTrue {
				lastStatus = "False"
			} else {
				lastStatus = "True"
			}
		}
		ret = append(ret, monitorapi.NewInterval(monitorapi.SourceOperatorState, level).
			Locator(event.Locator).
			Message(monitorapi.NewMessage().Reason(monitorapi.IntervalReason(lastReason)).
					HumanMessage(lastMessage).
					WithAnnotation(monitorapi.AnnotationCondition, string(conditionType)).
					WithAnnotation(monitorapi.AnnotationStatus, lastStatus)).
			Display(). // this is the variant of interval we want to chart
			Build(from, event.From),
		)
	}

	for operatorName, lastCondition := range operatorToInterestingBadCondition {
		ret = append(ret, monitorapi.NewInterval(monitorapi.SourceOperatorState, level).
			Locator(monitorapi.NewLocator().ClusterOperator(operatorName)).
			Message(monitorapi.NewMessage().Reason(monitorapi.IntervalReason(lastCondition.Reason)).
					HumanMessage(lastCondition.Message).
					WithAnnotation(monitorapi.AnnotationCondition, string(conditionType)).
					WithAnnotation(monitorapi.AnnotationStatus, string(lastCondition.Status)).HumanMessage(lastCondition.Message)).
			Display(). // this is the variant of interval we want to chart
			Build(lastCondition.LastTransitionTime.Time, end),
		)
		fmt.Printf("from: %s, to: %s\n", lastCondition.LastTransitionTime, end)
	}

	return ret
}
