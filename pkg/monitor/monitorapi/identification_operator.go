package monitorapi

import (
	configv1 "github.com/openshift/api/config/v1"
)

// GetOperatorConditionStatus reconstructs a ClusterOperatorStatusCondition from an interval.
func GetOperatorConditionStatus(interval Interval) *configv1.ClusterOperatorStatusCondition {
	c, ok := interval.StructuredMessage.Annotations[AnnotationCondition]
	if !ok {
		return nil
	}

	condition := &configv1.ClusterOperatorStatusCondition{}
	condition.Type = configv1.ClusterStatusConditionType(c)
	s, ok := interval.StructuredMessage.Annotations[AnnotationStatus]
	if ok {
		condition.Status = configv1.ConditionStatus(s)
	}
	r, ok := interval.StructuredMessage.Annotations[AnnotationReason]
	if ok {
		condition.Reason = r
	}

	condition.Message = interval.StructuredMessage.HumanMessage
	return condition
}
