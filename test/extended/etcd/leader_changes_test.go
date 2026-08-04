package etcd

import "testing"

func TestEtcdLeaderChangesQueryScopesNamespaceExactly(t *testing.T) {
	testCases := []struct {
		name      string
		namespace string
		expected  string
	}{
		{
			name:      "standalone control plane",
			namespace: "openshift-etcd",
			expected:  `max(max by (pod,job) (increase(etcd_server_leader_changes_seen_total{namespace="openshift-etcd"}[1h])))`,
		},
		{
			name:      "hosted control plane",
			namespace: "clusters-example",
			expected:  `max(max by (pod,job) (increase(etcd_server_leader_changes_seen_total{namespace="clusters-example"}[1h])))`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := etcdLeaderChangesQuery(testCase.namespace, "1h")
			if actual != testCase.expected {
				t.Fatalf("expected %q, got %q", testCase.expected, actual)
			}
		})
	}
}
