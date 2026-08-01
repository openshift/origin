package operators

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openshift/origin/test/extended/node"
)

func TestIsKnownEphemeralDebugPod(t *testing.T) {
	tests := []struct {
		name string
		pod  v1.Pod
		want bool
	}{
		{
			name: "machine config operator debug pod",
			pod: v1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: node.DebugNamespace,
				Name:      "worker-a-debug-abcde",
			}},
			want: true,
		},
		{
			name: "node tuning operator debug pod from older oc",
			pod: v1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: nodeTuningOperatorNamespace,
				Name:      "worker-a-debug-abcde",
			}},
			want: true,
		},
		{
			name: "transient debug namespace pod",
			pod: v1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: "openshift-debug-abcde",
				Labels: map[string]string{
					"debug.openshift.io/managed-by": "oc-debug",
				},
			}},
			want: true,
		},
		{
			name: "ordinary node tuning operator pod",
			pod: v1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: nodeTuningOperatorNamespace,
				Name:      "tuned-abcde",
			}},
			want: false,
		},
		{
			name: "debug named pod outside known namespaces",
			pod: v1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: "openshift-example-operator",
				Name:      "worker-a-debug-abcde",
			}},
			want: false,
		},
		{
			name: "ordinary pod in transient debug namespace",
			pod: v1.Pod{ObjectMeta: metav1.ObjectMeta{
				Namespace: "openshift-debug-abcde",
				Name:      "controller-abcde",
			}},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isKnownEphemeralDebugPod(&tt.pod); got != tt.want {
				t.Fatalf("isKnownEphemeralDebugPod() = %t, want %t", got, tt.want)
			}
		})
	}
}
