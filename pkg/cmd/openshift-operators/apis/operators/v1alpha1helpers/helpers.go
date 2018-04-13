package v1alpha1helpers

import (
	"time"
)

func SetErrors(versionAvailability *operatorsv1alpha1apiWebConsoleVersionAvailablity, errors ...error) {
	versionAvailability.Errors = []string{}
	for _, err := range errors {
		versionAvailability.Errors = append(versionAvailability.Errors, err.Error())
	}
}

func SetOperatorCondition(conditions *[]operatorsv1alpha1apiOpenShiftOperatorCondition, newCondition operatorsv1alpha1apiOpenShiftOperatorCondition) {
	if conditions == nil {
		conditions = &[]operatorsv1alpha1apiOpenShiftOperatorCondition{}
	}
	existingCondition := FindOperatorCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		newCondition.LastTransitionTime = metaoperatorsv1alpha1apiNewTime(time.Now())
		*conditions = append(*conditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		existingCondition.LastTransitionTime = newCondition.LastTransitionTime
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
}

func RemoveOperatorCondition(conditions *[]operatorsv1alpha1apiOpenShiftOperatorCondition, conditionType string) {
	if conditions == nil {
		conditions = &[]operatorsv1alpha1apiOpenShiftOperatorCondition{}
	}
	newConditions := []operatorsv1alpha1apiOpenShiftOperatorCondition{}
	for _, condition := range *conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}

	conditions = &newConditions
}

func FindOperatorCondition(conditions []operatorsv1alpha1apiOpenShiftOperatorCondition, conditionType string) *operatorsv1alpha1apiOpenShiftOperatorCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func IsOperatorConditionTrue(conditions []operatorsv1alpha1apiOpenShiftOperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorsv1alpha1apiConditionTrue)
}

func IsOperatorConditionFalse(conditions []operatorsv1alpha1apiOpenShiftOperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorsv1alpha1apiConditionFalse)
}

func IsOperatorConditionPresentAndEqual(conditions []operatorsv1alpha1apiOpenShiftOperatorCondition, conditionType string, status operatorsv1alpha1apiConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}
