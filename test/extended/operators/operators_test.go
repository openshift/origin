package operators

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_isManagedByClusterVersionOperator(t *testing.T) {
	tests := []struct {
		name string
		co   configv1.ClusterOperator
		want bool
	}{
		{
			name: "real operator with Available condition",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "authentication"},
				Status: configv1.ClusterOperatorStatus{
					Conditions: []configv1.ClusterOperatorStatusCondition{
						{Type: configv1.OperatorAvailable, Status: configv1.ConditionTrue},
						{Type: configv1.OperatorProgressing, Status: configv1.ConditionFalse},
						{Type: configv1.OperatorDegraded, Status: configv1.ConditionFalse},
					},
				},
			},
			want: true,
		},
		{
			name: "test fixture with no conditions",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "test-instance"},
				Status:     configv1.ClusterOperatorStatus{},
			},
			want: false,
		},
		{
			name: "test fixture with only custom conditions",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "test-instance"},
				Status: configv1.ClusterOperatorStatus{
					Conditions: []configv1.ClusterOperatorStatusCondition{
						{Type: "FirstType", Status: configv1.ConditionTrue},
						{Type: "SecondType", Status: configv1.ConditionTrue},
					},
				},
			},
			want: false,
		},
		{
			name: "operator with only Upgradeable condition",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver"},
				Status: configv1.ClusterOperatorStatus{
					Conditions: []configv1.ClusterOperatorStatusCondition{
						{Type: configv1.OperatorUpgradeable, Status: configv1.ConditionTrue},
					},
				},
			},
			want: true,
		},
		{
			name: "operator with only Degraded condition",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "etcd"},
				Status: configv1.ClusterOperatorStatus{
					Conditions: []configv1.ClusterOperatorStatusCondition{
						{Type: configv1.OperatorDegraded, Status: configv1.ConditionFalse},
					},
				},
			},
			want: true,
		},
		{
			name: "operator with mixed standard and custom conditions",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "network"},
				Status: configv1.ClusterOperatorStatus{
					Conditions: []configv1.ClusterOperatorStatusCondition{
						{Type: "CustomType", Status: configv1.ConditionTrue},
						{Type: configv1.OperatorAvailable, Status: configv1.ConditionTrue},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isManagedByClusterVersionOperator(tt.co)
			if got != tt.want {
				t.Errorf("isManagedByClusterVersionOperator(%q) = %v, want %v", tt.co.Name, got, tt.want)
			}
		})
	}
}
