package monitorapi

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
)

func TestGetOperatorConditionStatus(t *testing.T) {
	tests := []struct {
		name     string
		interval Interval
		want     *configv1.ClusterOperatorStatusCondition
	}{
		{
			name: "simple",
			interval: Interval{
				Source: SourceClusterOperatorMonitor,
				Condition: Condition{
					Message: Message{
						Reason:       "DNSDegraded",
						HumanMessage: "DNS default is degraded",
						Annotations: map[AnnotationKey]string{
							AnnotationCondition: "Degraded",
							AnnotationStatus:    "True",
							AnnotationReason:    "DNSDegraded",
						},
					},
				},
			},
			want: &configv1.ClusterOperatorStatusCondition{
				Type:    configv1.OperatorDegraded,
				Status:  configv1.ConditionTrue,
				Reason:  "DNSDegraded",
				Message: "DNS default is degraded",
			},
		},
		{
			name: "unknown",
			interval: Interval{
				Source: SourceClusterOperatorMonitor,
				Condition: Condition{
					Message: Message{
						Reason:       "NoData",
						HumanMessage: "blah blah",
						Annotations: map[AnnotationKey]string{
							AnnotationCondition: "Upgradeable",
							AnnotationStatus:    "Unknown",
							AnnotationReason:    "NoData",
						},
					},
				},
			},
			want: &configv1.ClusterOperatorStatusCondition{
				Type:    configv1.OperatorUpgradeable,
				Status:  configv1.ConditionUnknown,
				Reason:  "NoData",
				Message: "blah blah",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetOperatorConditionStatus(tt.interval)
			assert.Equal(t, tt.want, got)
		})
	}
}
