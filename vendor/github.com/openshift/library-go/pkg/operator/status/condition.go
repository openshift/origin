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
func unionCondition(conditionType string, conditionMergeState operatorv1.ConditionStatus, allConditions ...operatorv1.OperatorCondition) configv1.ClusterOperatorStatusCondition {
	var interestingConditions []operatorv1.OperatorCondition
	for _, condition := range allConditions {
		if strings.HasSuffix(condition.Type, conditionType) && condition.Status == conditionMergeState {
			interestingConditions = append(interestingConditions, condition)
		}
	}

	unionedCondition := operatorv1.OperatorCondition{Type: conditionType, Status: operatorv1.ConditionUnknown}
	if len(interestingConditions) > 0 {
		unionedCondition.Status = conditionMergeState
		var messages []string
		latestTransitionTime := metav1.Time{}
		for _, condition := range interestingConditions {
			if latestTransitionTime.Before(&condition.LastTransitionTime) {
				latestTransitionTime = condition.LastTransitionTime
			}

			if len(condition.Message) == 0 {
				continue
			}
			for _, message := range strings.Split(condition.Message, "\n") {
				messages = append(messages, fmt.Sprintf("%s: %s", condition.Type, message))
			}
		}
		if len(messages) > 0 {
			unionedCondition.Message = strings.Join(messages, "\n")
		}
		if len(interestingConditions) == 1 {
			unionedCondition.Reason = interestingConditions[0].Type
		} else {
			unionedCondition.Reason = "MultipleConditionsMatching"
		}
		unionedCondition.LastTransitionTime = latestTransitionTime

	} else {
		if conditionMergeState == operatorv1.ConditionTrue {
			unionedCondition.Status = operatorv1.ConditionFalse
		} else {
			unionedCondition.Status = operatorv1.ConditionTrue
		}
	}

	return OperatorConditionToClusterOperatorCondition(unionedCondition)
}
