package etcd

import "testing"

func TestLeaderChangesQueryUsesExactNamespace(t *testing.T) {
	want := `max(max by (pod,job) (increase(etcd_server_leader_changes_seen_total{namespace="clusters-example"}[2h3m4s])))`
	if got := leaderChangesQuery("clusters-example", "2h3m4s"); got != want {
		t.Fatalf("unexpected query:\nwant: %s\n got: %s", want, got)
	}
}
