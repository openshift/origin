package intervalcreation

import (
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
)

func IntervalsFromEvents_OperatorAvailable(events []*monitorapi.Event, beginning, end time.Time) monitorapi.EventIntervals {
	return intervalsFromEvents_OperatorStatus(events, beginning, end, configv1.OperatorAvailable, configv1.ConditionTrue, monitorapi.Error)
}

func IntervalsFromEvents_OperatorProgressing(events []*monitorapi.Event, beginning, end time.Time) monitorapi.EventIntervals {
	return intervalsFromEvents_OperatorStatus(events, beginning, end, configv1.OperatorProgressing, configv1.ConditionFalse, monitorapi.Warning)
}

func IntervalsFromEvents_OperatorDegraded(events []*monitorapi.Event, beginning, end time.Time) monitorapi.EventIntervals {
	return intervalsFromEvents_OperatorStatus(events, beginning, end, configv1.OperatorDegraded, configv1.ConditionFalse, monitorapi.Error)
}

func intervalsFromEvents_OperatorStatus(events []*monitorapi.Event, beginning, end time.Time, conditionType configv1.ClusterStatusConditionType, conditionGoodState configv1.ConditionStatus, level monitorapi.EventLevel) monitorapi.EventIntervals {
	ret := monitorapi.EventIntervals{}
	operatorToInterestingBadCondition := map[string]*configv1.ClusterOperatorStatusCondition{}

	for _, event := range events {
		operatorName, ok := monitorapi.OperatorFromLocator(event.Locator)
		if !ok {
			continue
		}
		currentCondition := monitorapi.GetOperatorConditionStatus(event.Message)
		if currentCondition == nil {
			continue
		}
		if currentCondition.Type != conditionType {
			continue
		}

		lastCondition := operatorToInterestingBadCondition[operatorName]
		if lastCondition != nil && lastCondition.Status == currentCondition.Status {
			// if the status didn't actually change (imagine degraded just changing reasons)
			// don't count as the interval
			continue
		}
		if currentCondition.Status != conditionGoodState {
			// don't overwrite a previous condition in a bad state
			if lastCondition == nil {
				// force teh last transition time, sinc we think we just transitioned at this instant
				currentCondition.LastTransitionTime.Time = event.At
				operatorToInterestingBadCondition[operatorName] = currentCondition
			}
			continue
		}

		// at this point we have transitioned to a good state.  Remove the previous "bad" state
		delete(operatorToInterestingBadCondition, operatorName)

		from := beginning
		lastStatus := "Unknown"
		lastMessage := "Unknown"
		if lastCondition != nil {
			from = lastCondition.LastTransitionTime.Time
			lastStatus = fmt.Sprintf("%v", lastCondition.Status)
			lastMessage = lastCondition.Message
		} else {
			// if we're in a good state now, then we were probably in a bad state before.  Let's start by assuming that anyway
			if conditionGoodState == configv1.ConditionTrue {
				lastStatus = "False"
			} else {
				lastStatus = "True"
			}
		}
		ret = append(ret, &monitorapi.EventInterval{
			Condition: &monitorapi.Condition{
				Level:   level,
				Locator: event.Locator,
				Message: fmt.Sprintf("condition/%s status/%s reason/%s", conditionType, lastStatus, lastMessage),
			},
			From: from,
			To:   event.At,
		})
	}

	for operatorName, lastCondition := range operatorToInterestingBadCondition {
		ret = append(ret, &monitorapi.EventInterval{
			Condition: &monitorapi.Condition{
				Level:   level,
				Locator: monitorapi.OperatorLocator(operatorName),
				Message: fmt.Sprintf("condition/%s status/%s reason/%s", conditionType, lastCondition.Status, lastCondition.Message),
			},
			From: lastCondition.LastTransitionTime.Time,
			To:   end,
		})
	}

	return ret
}
