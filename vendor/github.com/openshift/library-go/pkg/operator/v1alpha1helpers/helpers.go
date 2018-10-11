package v1alpha1helpers

import (
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
)

func SetErrors(versionAvailability *operatorsv1alpha1.VersionAvailability, errors ...error) {
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

	*conditions = newConditions
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

func SetStatusFromAvailability(status *operatorsv1alpha1.OperatorStatus, specGeneration int64, versionAvailability *operatorsv1alpha1.VersionAvailability) {
	// given the VersionAvailability and the status.Version, we can compute availability
	availableCondition := operatorsv1alpha1.OperatorCondition{
		Type:   operatorsv1alpha1.OperatorStatusTypeAvailable,
		Status: operatorsv1alpha1.ConditionUnknown,
	}
	if versionAvailability != nil && versionAvailability.ReadyReplicas > 0 {
		availableCondition.Status = operatorsv1alpha1.ConditionTrue
	} else {
		availableCondition.Status = operatorsv1alpha1.ConditionFalse
	}
	SetOperatorCondition(&status.Conditions, availableCondition)

	syncSuccessfulCondition := operatorsv1alpha1.OperatorCondition{
		Type:   operatorsv1alpha1.OperatorStatusTypeSyncSuccessful,
		Status: operatorsv1alpha1.ConditionTrue,
	}
	if versionAvailability != nil && len(versionAvailability.Errors) > 0 {
		syncSuccessfulCondition.Status = operatorsv1alpha1.ConditionFalse
		syncSuccessfulCondition.Message = strings.Join(versionAvailability.Errors, "\n")
	}
	if status.TargetAvailability != nil && len(status.TargetAvailability.Errors) > 0 {
		syncSuccessfulCondition.Status = operatorsv1alpha1.ConditionFalse
		if len(syncSuccessfulCondition.Message) == 0 {
			syncSuccessfulCondition.Message = strings.Join(status.TargetAvailability.Errors, "\n")
		} else {
			syncSuccessfulCondition.Message = availableCondition.Message + "\n" + strings.Join(status.TargetAvailability.Errors, "\n")
		}
	}
	SetOperatorCondition(&status.Conditions, syncSuccessfulCondition)
	if syncSuccessfulCondition.Status == operatorsv1alpha1.ConditionTrue {
		status.ObservedGeneration = specGeneration
	}

	status.CurrentAvailability = versionAvailability
}
