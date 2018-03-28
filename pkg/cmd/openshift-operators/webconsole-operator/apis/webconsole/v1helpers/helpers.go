package v1helpers

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/apis/webconsole/v1"
)

func SetErrors(versionAvailability *v1.WebConsoleVersionAvailablity, errors ...error) {
	versionAvailability.Errors = []string{}
	for _, err := range errors {
		versionAvailability.Errors = append(versionAvailability.Errors, err.Error())
	}
}

func SetVersionAvailablity(versionAvailability *[]v1.WebConsoleVersionAvailablity, newAvailibility v1.WebConsoleVersionAvailablity) {
	if versionAvailability == nil {
		versionAvailability = &[]v1.WebConsoleVersionAvailablity{}
	}
	existingAvailability := FindVersionAvailablity(*versionAvailability, newAvailibility.Version)
	if existingAvailability == nil {
		*versionAvailability = append(*versionAvailability, newAvailibility)
		return
	}

	existingAvailability.AvailableReplicas = newAvailibility.AvailableReplicas
	existingAvailability.UpdatedReplicas = newAvailibility.UpdatedReplicas
	existingAvailability.ReadyReplicas = newAvailibility.ReadyReplicas
	existingAvailability.Errors = newAvailibility.Errors
}

func RemoveAvailability(versionAvailability *[]v1.WebConsoleVersionAvailablity, version string) {
	if versionAvailability == nil {
		versionAvailability = &[]v1.WebConsoleVersionAvailablity{}
	}
	newAvailability := []v1.WebConsoleVersionAvailablity{}
	for _, availability := range *versionAvailability {
		if availability.Version != version {
			newAvailability = append(newAvailability, availability)
		}
	}

	*versionAvailability = newAvailability
}

func FilterAvailability(versionAvailability *[]v1.WebConsoleVersionAvailablity, versions ...string) {
	if versionAvailability == nil {
		versionAvailability = &[]v1.WebConsoleVersionAvailablity{}
	}
	allowedVersions := sets.NewString(versions...)
	newAvailability := []v1.WebConsoleVersionAvailablity{}
	for _, availability := range *versionAvailability {
		if allowedVersions.Has(availability.Version) {
			newAvailability = append(newAvailability, availability)
		}
	}

	*versionAvailability = newAvailability
}

func FindVersionAvailablity(versionAvailability []v1.WebConsoleVersionAvailablity, version string) *v1.WebConsoleVersionAvailablity {
	for i := range versionAvailability {
		if versionAvailability[i].Version == version {
			return &versionAvailability[i]
		}
	}

	return nil
}

func SetOperatorCondition(conditions *[]v1.OpenShiftOperatorCondition, newCondition v1.OpenShiftOperatorCondition) {
	if conditions == nil {
		conditions = &[]v1.OpenShiftOperatorCondition{}
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

func RemoveOperatorCondition(conditions *[]v1.OpenShiftOperatorCondition, conditionType string) {
	if conditions == nil {
		conditions = &[]v1.OpenShiftOperatorCondition{}
	}
	newConditions := []v1.OpenShiftOperatorCondition{}
	for _, condition := range *conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}

	conditions = &newConditions
}

func FindOperatorCondition(conditions []v1.OpenShiftOperatorCondition, conditionType string) *v1.OpenShiftOperatorCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

func IsOperatorConditionTrue(conditions []v1.OpenShiftOperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, v1.ConditionTrue)
}

func IsOperatorConditionFalse(conditions []v1.OpenShiftOperatorCondition, conditionType string) bool {
	return IsOperatorConditionPresentAndEqual(conditions, conditionType, v1.ConditionFalse)
}

func IsOperatorConditionPresentAndEqual(conditions []v1.OpenShiftOperatorCondition, conditionType string, status v1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}
