package watchnodes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNodeRoles(t *testing.T) {
	var testCases = []struct {
		node     *corev1.Node
		expected string
	}{
		{
			node:     &corev1.Node{},
			expected: "",
		},
		{
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/master": "",
					},
				},
			},
			expected: "master",
		},
		{
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/worker": "",
					},
				},
			},
			expected: "worker",
		},
		{
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"node-role.kubernetes.io/master": "",
						"node-role.kubernetes.io/worker": "",
					},
				},
			},
			expected: "master,worker",
		},
	}

	for _, tc := range testCases {
		if actual := nodeRoles(tc.node); tc.expected != actual {
			t.Errorf("mismatch roles. expected: %s, actual: %s", tc.expected, actual)
		}
	}
}
