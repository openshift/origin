package edge_topologies

import (
	"testing"

	etcdv1 "github.com/openshift/api/etcd/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExpectTNFAlertStatus(t *testing.T) {
	healthy := &etcdv1.PacemakerCluster{Status: etcdv1.PacemakerClusterStatus{
		Conditions: []metav1.Condition{{Type: "Healthy", Status: metav1.ConditionTrue}},
	}}
	nodes := []etcdv1.PacemakerClusterNodeStatus{}
	for _, name := range []string{"master-0", "master-1"} {
		node := etcdv1.PacemakerClusterNodeStatus{
			NodeName: name,
			Conditions: []metav1.Condition{
				{Type: "Healthy", Status: metav1.ConditionTrue},
				{Type: "AlertAgentsHealthy", Status: metav1.ConditionTrue},
			},
		}
		for _, agent := range []etcdv1.PacemakerClusterAlertAgentName{etcdv1.PacemakerClusterAlertAgentNameTaint, etcdv1.PacemakerClusterAlertAgentNameUntaint} {
			node.AlertAgents = append(node.AlertAgents, etcdv1.PacemakerClusterAlertAgentStatus{
				Name: agent,
				Conditions: []metav1.Condition{
					{Type: "Healthy", Status: metav1.ConditionTrue},
					{Type: "Configured", Status: metav1.ConditionTrue},
					{Type: "Available", Status: metav1.ConditionTrue},
				},
			})
		}
		nodes = append(nodes, node)
	}
	healthy.Status.Nodes = &nodes
	drift := healthy.DeepCopy()
	drift.Status.Conditions[0].Status = metav1.ConditionFalse
	target := &(*drift.Status.Nodes)[0]
	for i := range target.Conditions {
		target.Conditions[i].Status = metav1.ConditionFalse
	}
	target.AlertAgents[0].Conditions[0].Status = metav1.ConditionFalse
	target.AlertAgents[0].Conditions[0].Reason = etcdv1.AlertAgentHealthyReasonUnhealthy
	target.AlertAgents[0].Conditions[2].Status = metav1.ConditionFalse
	target.AlertAgents[0].Conditions[2].Reason = etcdv1.AlertAgentAvailableReasonUnavailable
	failure := tnfAlertFailure{node: "master-0", agent: etcdv1.PacemakerClusterAlertAgentNameTaint,
		condition: etcdv1.AlertAgentAvailableConditionType, reason: etcdv1.AlertAgentAvailableReasonUnavailable}
	for _, tc := range []struct {
		name     string
		pc       *etcdv1.PacemakerCluster
		failures []tnfAlertFailure
		mutate   func(*etcdv1.PacemakerCluster)
		wantErr  bool
	}{
		{name: "healthy", pc: healthy},
		{name: "node-local failure with healthy peer", pc: drift, failures: []tnfAlertFailure{failure}},
		{name: "missing agent", pc: healthy, wantErr: true, mutate: func(pc *etcdv1.PacemakerCluster) {
			(*pc.Status.Nodes)[0].AlertAgents = (*pc.Status.Nodes)[0].AlertAgents[:1]
		}},
		{name: "missing node", pc: healthy, wantErr: true, mutate: func(pc *etcdv1.PacemakerCluster) { *pc.Status.Nodes = (*pc.Status.Nodes)[:1] }},
		{name: "unknown is not healthy", pc: healthy, wantErr: true, mutate: func(pc *etcdv1.PacemakerCluster) {
			(*pc.Status.Nodes)[1].AlertAgents[1].Conditions[2].Status = metav1.ConditionUnknown
		}},
		{name: "wrong failure reason", pc: drift, failures: []tnfAlertFailure{failure}, wantErr: true, mutate: func(pc *etcdv1.PacemakerCluster) {
			(*pc.Status.Nodes)[0].AlertAgents[0].Conditions[2].Reason = "Pending"
		}},
		{name: "failure must reach cluster health", pc: drift, failures: []tnfAlertFailure{failure}, wantErr: true, mutate: func(pc *etcdv1.PacemakerCluster) { pc.Status.Conditions[0].Status = metav1.ConditionTrue }},
		{name: "peer must stay healthy", pc: drift, failures: []tnfAlertFailure{failure}, wantErr: true, mutate: func(pc *etcdv1.PacemakerCluster) { (*pc.Status.Nodes)[1].Conditions[0].Status = metav1.ConditionFalse }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pc := tc.pc.DeepCopy()
			if tc.mutate != nil {
				tc.mutate(pc)
			}
			err := expectTNFAlertStatus(pc, []string{"master-0", "master-1"}, tc.failures...)
			if (err != nil) != tc.wantErr {
				t.Fatalf("expectTNFAlertStatus() error = %v, wantErr %t", err, tc.wantErr)
			}
		})
	}
}
