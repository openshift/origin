package v1helpers

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/json"

	configv1 "github.com/openshift/api/config/v1"
)

// SetStatusCondition sets the corresponding condition in conditions to newCondition.
func SetStatusCondition(conditions *[]configv1.ClusterOperatorStatusCondition, newCondition configv1.ClusterOperatorStatusCondition) {
	if conditions == nil {
		conditions = &[]configv1.ClusterOperatorStatusCondition{}
	}
	existingCondition := FindStatusCondition(*conditions, newCondition.Type)
	if existingCondition == nil {
		newCondition.LastTransitionTime = metav1.NewTime(time.Now())
		*conditions = append(*conditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status {
		existingCondition.Status = newCondition.Status
		existingCondition.LastTransitionTime = metav1.NewTime(time.Now())
	}

	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
}

// RemoveStatusCondition removes the corresponding conditionType from conditions.
func RemoveStatusCondition(conditions *[]configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType) {
	if conditions == nil {
		conditions = &[]configv1.ClusterOperatorStatusCondition{}
	}
	newConditions := []configv1.ClusterOperatorStatusCondition{}
	for _, condition := range *conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}

	*conditions = newConditions
}

// FindStatusCondition finds the conditionType in conditions.
func FindStatusCondition(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}

	return nil
}

// GetStatusDiff returns a string representing change in condition status in human readable form.
func GetStatusDiff(oldStatus configv1.ClusterOperatorStatus, newStatus configv1.ClusterOperatorStatus) string {
	messages := []string{}
	for _, newCondition := range newStatus.Conditions {
		existingStatusCondition := FindStatusCondition(oldStatus.Conditions, newCondition.Type)
		if existingStatusCondition == nil {
			messages = append(messages, fmt.Sprintf("%s set to %s (%q)", newCondition.Type, newCondition.Status, newCondition.Message))
			continue
		}
		if existingStatusCondition.Status != newCondition.Status {
			messages = append(messages, fmt.Sprintf("%s changed from %s to %s (%q)", existingStatusCondition.Type, existingStatusCondition.Status, newCondition.Status, newCondition.Message))
			continue
		}
		if existingStatusCondition.Message != newCondition.Message {
			messages = append(messages, fmt.Sprintf("%s message changed from %q to %q", existingStatusCondition.Type, existingStatusCondition.Message, newCondition.Message))
		}
	}
	for _, oldCondition := range oldStatus.Conditions {
		// This should not happen. It means we removed old condition entirely instead of just changing its status
		if c := FindStatusCondition(newStatus.Conditions, oldCondition.Type); c == nil {
			messages = append(messages, fmt.Sprintf("%s was removed", oldCondition.Type))
		}
	}

	if !equality.Semantic.DeepEqual(oldStatus.RelatedObjects, newStatus.RelatedObjects) {
		messages = append(messages, fmt.Sprintf("status.relatedObjects changed from %q to %q", oldStatus.RelatedObjects, newStatus.RelatedObjects))
	}
	if !equality.Semantic.DeepEqual(oldStatus.Extension, newStatus.Extension) {
		messages = append(messages, fmt.Sprintf("status.extension changed from %q to %q", oldStatus.Extension, newStatus.Extension))
	}

	if len(messages) == 0 {
		// ignore errors
		originalJSON := &bytes.Buffer{}
		json.NewEncoder(originalJSON).Encode(oldStatus)
		newJSON := &bytes.Buffer{}
		json.NewEncoder(newJSON).Encode(newStatus)
		messages = append(messages, diff.StringDiff(originalJSON.String(), newJSON.String()))
	}

	return strings.Join(messages, ",")
}

// IsStatusConditionTrue returns true when the conditionType is present and set to `configv1.ConditionTrue`
func IsStatusConditionTrue(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType) bool {
	return IsStatusConditionPresentAndEqual(conditions, conditionType, configv1.ConditionTrue)
}

// IsStatusConditionFalse returns true when the conditionType is present and set to `configv1.ConditionFalse`
func IsStatusConditionFalse(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType) bool {
	return IsStatusConditionPresentAndEqual(conditions, conditionType, configv1.ConditionFalse)
}

// IsStatusConditionPresentAndEqual returns true when conditionType is present and equal to status.
func IsStatusConditionPresentAndEqual(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType, status configv1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}

// IsStatusConditionNotIn returns true when the conditionType does not match the status.
func IsStatusConditionNotIn(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType, status ...configv1.ConditionStatus) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			for _, s := range status {
				if s == condition.Status {
					return false
				}
			}
			return true
		}
	}
	return true
}
