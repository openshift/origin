package apihelpers

import (
	mcfgv1alpha1 "github.com/openshift/api/machineconfiguration/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewMachineOSBuildCondition creates a new MachineOSBuild condition.
func NewMachineOSBuildCondition(condType string, status metav1.ConditionStatus, reason, message string) *metav1.Condition {
	return &metav1.Condition{
		Type:               condType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

func GetMachineOSBuildCondition(status mcfgv1alpha1.MachineOSBuildStatus, condType mcfgv1alpha1.BuildProgress) *metav1.Condition {
	// in case of sync errors, return the last condition that matches, not the first
	// this exists for redundancy and potential race conditions.
	var LatestState *metav1.Condition
	for i := range status.Conditions {
		c := status.Conditions[i]
		if mcfgv1alpha1.BuildProgress(c.Type) == condType {
			LatestState = &c
		}
	}
	return LatestState
}

// SetMachineOSBuildCondition updates the MachineOSBuild to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then we are not going to update.
func SetMachineOSBuildCondition(status *mcfgv1alpha1.MachineOSBuildStatus, condition metav1.Condition) {
	currentCond := GetMachineOSBuildCondition(*status, mcfgv1alpha1.BuildProgress(condition.Type))
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason && currentCond.Message == condition.Message {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}

	// this may not be necessary
	newConditions := filterOutMachineOSBuildCondition(status.Conditions, mcfgv1alpha1.BuildProgress(condition.Type))
	newConditions = append(newConditions, condition)
	status.Conditions = newConditions
}

// RemoveMachineOSBuildCondition removes the MachineOSBuild condition with the provided type.
func RemoveMachineOSBuildCondition(status *mcfgv1alpha1.MachineOSBuildStatus, condType mcfgv1alpha1.BuildProgress) {
	status.Conditions = filterOutMachineOSBuildCondition(status.Conditions, condType)
}

// filterOutMachineOSBuildCondition returns a new slice of MachineOSBuild conditions without conditions with the provided type.
func filterOutMachineOSBuildCondition(conditions []metav1.Condition, condType mcfgv1alpha1.BuildProgress) []metav1.Condition {
	var newConditions []metav1.Condition
	for _, c := range conditions {
		if mcfgv1alpha1.BuildProgress(c.Type) == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

func IsMachineOSBuildConditionTrue(conditions []metav1.Condition, conditionType mcfgv1alpha1.BuildProgress) bool {
	return IsMachineOSBuildConditionPresentAndEqual(conditions, conditionType, metav1.ConditionTrue)
}

// IsMachineOSBuildConditionPresentAndEqual returns true when conditionType is present and equal to status.
func IsMachineOSBuildConditionPresentAndEqual(conditions []metav1.Condition, conditionType mcfgv1alpha1.BuildProgress, status metav1.ConditionStatus) bool {
	for _, condition := range conditions {
		if mcfgv1alpha1.BuildProgress(condition.Type) == conditionType {
			return condition.Status == status
		}
	}
	return false
}
