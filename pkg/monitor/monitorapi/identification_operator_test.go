package monitorapi

import (
	"reflect"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestGetOperatorConditionStatus(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    *configv1.ClusterOperatorStatusCondition
	}{
		{
			name:    "simple",
			message: "condition/Degraded status/True reason/DNSDegraded changed: DNS default is degraded",
			want: &configv1.ClusterOperatorStatusCondition{
				Type:    configv1.OperatorDegraded,
				Status:  configv1.ConditionTrue,
				Reason:  "DNSDegraded",
				Message: "DNS default is degraded",
			},
		},
		{
			name:    "unknown",
			message: "condition/Upgradeable status/Unknown reason/NoData changed: blah blah",
			want: &configv1.ClusterOperatorStatusCondition{
				Type:    configv1.OperatorUpgradeable,
				Status:  configv1.ConditionUnknown,
				Reason:  "NoData",
				Message: "blah blah",
			},
		},
		{
			name:    "repeat reason",
			message: "condition/Available status/True reason/AsExpected changed: reason/again",
			want: &configv1.ClusterOperatorStatusCondition{
				Type:    configv1.OperatorAvailable,
				Status:  configv1.ConditionTrue,
				Reason:  "AsExpected",
				Message: "reason/again",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetOperatorConditionStatus(tt.message); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetOperatorConditionStatus() got = %v, want %v", got, tt.want)
			}
		})
	}
}
