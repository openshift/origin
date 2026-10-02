package operators

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_isManagedByClusterVersionOperator(t *testing.T) {
	cvoOwnerRef := metav1.OwnerReference{
		APIVersion: "config.openshift.io/v1",
		Kind:       "ClusterVersion",
		Name:       "version",
		UID:        "test-uid",
	}

	tests := []struct {
		name string
		co   configv1.ClusterOperator
		want bool
	}{
		{
			name: "real operator with CVO owner reference",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "authentication",
					OwnerReferences: []metav1.OwnerReference{cvoOwnerRef},
				},
			},
			want: true,
		},
		{
			name: "test fixture with no owner references",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{Name: "test-instance"},
			},
			want: false,
		},
		{
			name: "test fixture with non-CVO owner reference",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-instance",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "some-controller",
							UID:        "other-uid",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "operator with CVO owner among multiple references",
			co: configv1.ClusterOperator{
				ObjectMeta: metav1.ObjectMeta{
					Name: "network",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "apps/v1",
							Kind:       "Deployment",
							Name:       "some-controller",
							UID:        "other-uid",
						},
						cvoOwnerRef,
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
