package monitorapi

import (
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
)

// GetOperatorConditionStatus reconstructs a ClusterOperatorStatusCondition from an interval.
func GetOperatorConditionStatus(interval Interval) *configv1.ClusterOperatorStatusCondition {
	c, ok := interval.Message.Annotations[AnnotationCondition]
	if !ok {
		return nil
	}

	condition := &configv1.ClusterOperatorStatusCondition{}
	condition.Type = configv1.ClusterStatusConditionType(c)
	s, ok := interval.Message.Annotations[AnnotationStatus]
	if ok {
		condition.Status = configv1.ConditionStatus(s)
	}
	r, ok := interval.Message.Annotations[AnnotationReason]
	if ok {
		condition.Reason = r
	}

	condition.Message = interval.Message.HumanMessage
	return condition
}

// GetOperatorConditionHumanMessage constructs a human-readable message from a given ClusterOperatorStatusCondition with a given prefix
func GetOperatorConditionHumanMessage(s *configv1.ClusterOperatorStatusCondition, prefix string) string {
	if s == nil {
		return ""
	}
	switch {
	case len(s.Reason) > 0 && len(s.Message) > 0:
		return fmt.Sprintf("%s%s=%s: %s: %s", prefix, s.Type, s.Status, s.Reason, s.Message)
	case len(s.Message) > 0:
		return fmt.Sprintf("%s%s=%s: %s", prefix, s.Type, s.Status, s.Message)
	default:
		return fmt.Sprintf("%s%s=%s", prefix, s.Type, s.Status)
	}
}
