package status

import (
	"fmt"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// conditionMergeState indicates whether you want to merge all Falses or merge all Trues.  For instance, Failures merge
// on true, but Available merges on false.  Thing of it like an anti-default.
func unionCondition(conditionType string, defaultConditionState operatorv1.ConditionStatus, allConditions ...operatorv1.OperatorCondition) configv1.ClusterOperatorStatusCondition {
	var oppositeConditionStatus operatorv1.ConditionStatus
	if defaultConditionState == operatorv1.ConditionTrue {
		oppositeConditionStatus = operatorv1.ConditionFalse
	} else {
		oppositeConditionStatus = operatorv1.ConditionTrue
	}

	interestingConditions := []operatorv1.OperatorCondition{}
	badConditions := []operatorv1.OperatorCondition{}
	for _, condition := range allConditions {
		if strings.HasSuffix(condition.Type, conditionType) {
			interestingConditions = append(interestingConditions, condition)

			if condition.Status == oppositeConditionStatus {
				badConditions = append(badConditions, condition)
			}
		}
	}

	unionedCondition := operatorv1.OperatorCondition{Type: conditionType, Status: operatorv1.ConditionUnknown}
	if len(interestingConditions) == 0 {
		unionedCondition.Status = operatorv1.ConditionUnknown
		unionedCondition.Reason = "NoData"
		return OperatorConditionToClusterOperatorCondition(unionedCondition)
	}

	if len(badConditions) == 0 {
		unionedCondition.Status = defaultConditionState
		unionedCondition.Message = unionMessage(interestingConditions)
		unionedCondition.Reason = "AsExpected"
		unionedCondition.LastTransitionTime = latestTransitionTime(interestingConditions)

		return OperatorConditionToClusterOperatorCondition(unionedCondition)
	}

	// at this point we have bad conditions
	unionedCondition.Status = oppositeConditionStatus
	unionedCondition.Message = unionMessage(badConditions)
	unionedCondition.Reason = unionReason(badConditions)
	unionedCondition.LastTransitionTime = latestTransitionTime(badConditions)

	return OperatorConditionToClusterOperatorCondition(unionedCondition)
}

func latestTransitionTime(conditions []operatorv1.OperatorCondition) metav1.Time {
	latestTransitionTime := metav1.Time{}
	for _, condition := range conditions {
		if latestTransitionTime.Before(&condition.LastTransitionTime) {
			latestTransitionTime = condition.LastTransitionTime
		}
	}
	return latestTransitionTime
}

func uniq(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	j := 0
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		s[j] = v
		j++
	}
	return s[:j]
}

func unionMessage(conditions []operatorv1.OperatorCondition) string {
	messages := []string{}
	for _, condition := range conditions {
		if len(condition.Message) == 0 {
			continue
		}
		for _, message := range uniq(strings.Split(condition.Message, "\n")) {
			messages = append(messages, fmt.Sprintf("%s: %s", condition.Type, message))
		}
	}
	return strings.Join(messages, "\n")
}

func unionReason(conditions []operatorv1.OperatorCondition) string {
	if len(conditions) == 1 {
		if len(conditions[0].Reason) != 0 {
			return conditions[0].Type + conditions[0].Reason
		}
		return conditions[0].Type
	} else {
		return "MultipleConditionsMatching"
	}
}
