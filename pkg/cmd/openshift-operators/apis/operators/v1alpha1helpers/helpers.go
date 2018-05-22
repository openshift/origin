package v1alpha1helpers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

func SetErrors(versionAvailability *operatorsv1alpha1.VersionAvailablity, errors ...error) {
	versionAvailability.Errors = []string{}
	for _, err := range errors {
		versionAvailability.Errors = append(versionAvailability.Errors, err.Error())
	}
}

func SetOperatorCondition(conditions *[]operatorsv1alpha1.OperatorCondition, newCondition operatorsv1alpha1.OperatorCondition) {
	if conditions == nil {
		conditions = &[]operatorsv1alpha1.OperatorCondition{}
	}
	existingCondition := FindOperatorCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		newCondition.LastTransitionTime = metav1.NewTime(time.Now())
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

func RemoveOperatorCondition(conditions *[]operatorsv1alpha1.OperatorCondition, conditionType string) {
	if conditions == nil {
		conditions = &[]operatorsv1alpha1.OperatorCondition{}
	}
	newConditions := []operatorsv1alpha1.OperatorCondition{}
	for _, condition := range *conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}

	conditions = &newConditions
}

func FindOperatorCondition(conditions []operatorsv1alpha1.OperatorCondition, conditionType string) *operatorsv1alpha1.OperatorCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func IsOperatorConditionTrue(conditions []operatorsv1alpha1.OperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorsv1alpha1.ConditionTrue)
}

func IsOperatorConditionFalse(conditions []operatorsv1alpha1.OperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, operatorsv1alpha1.ConditionFalse)
}

func IsOperatorConditionPresentAndEqual(conditions []operatorsv1alpha1.OperatorCondition, conditionType string, status operatorsv1alpha1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}
