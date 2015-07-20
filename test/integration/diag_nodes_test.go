// +build integration,!no-etcd

package integration

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	clusterdiags "github.com/openshift/origin/pkg/diagnostics/cluster"
	diagtype "github.com/openshift/origin/pkg/diagnostics/types"
	testutil "github.com/openshift/origin/test/util"
)

func TestDiagNodeConditions(t *testing.T) {
	//masterConfig, clientFile, err := testutil.StartTestAllInOne()
	_, clientFile, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	client, err := testutil.GetClusterAdminKubeClient(clientFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodeDiag := clusterdiags.NodeDefinitions{KubeClient: client}
	// First check with no nodes defined; should get an error about that.
	// ok, logs, warnings, errors := nodeDiag.Check()
	if errors := nodeDiag.Check().Errors(); len(errors) != 1 ||
		!diagtype.MatchesDiagError(errors[0], "clNoAvailNodes") {
		t.Errorf("expected 1 error about not having nodes, not: %#v", errors)
	}

	// Next create a node and leave it in NotReady state. Should get a warning
	// about that, plus the previous error as there are still no nodes available.
	node, err := client.Nodes().Create(&kapi.Node{ObjectMeta: kapi.ObjectMeta{Name: "test-node"}})
	if err != nil {
		t.Fatalf("expected no errors creating a node: %#v", err)
	}
	result := nodeDiag.Check()
	if errors := result.Errors(); len(errors) != 1 ||
		!diagtype.MatchesDiagError(errors[0], "clNoAvailNodes") {
		t.Fatalf("expected 1 error about not having nodes, not: %#v", errors)
	} else if warnings := result.Warnings(); len(warnings) < 1 || !diagtype.MatchesDiagError(warnings[0], "clNodeNotReady") {
		t.Fatalf("expected a warning about test-node not being ready, not: %#v", warnings)
	}

	_ = node
	/*
		// Put the new node in Ready state and verify the diagnostic is clean
		if _, err := client.Nodes().UpdateStatus(node); err != nil {
			t.Fatalf("expected no errors updating node status, but: %#v", err)
		}
		result = nodeDiag.Check()
		if warnings := result.Warnings(); len(warnings) > 0 {
			t.Fatalf("expected no warning with one node ready, but: %#v", warnings)
		} else if errors := result.Errors(); len(warnings) > 0 {
			t.Fatalf("expected no errors with one node ready, but: %#v", errors)
		}

		// Make the node unschedulable and verify diagnostics notices
		node.Spec.Unschedulable = true
		if _, err := client.Nodes().Update(node); err != nil {
			t.Fatalf("expected no errors making node unschedulable, but: %#v", err)
		}
		if errors := result.Errors(); len(errors) != 1 ||
			!diagtype.MatchesDiagError(errors[0], "clNoAvailNodes") {
			t.Fatalf("expected 1 error about not having nodes, but: %#v", errors)
		} else if warnings := result.Warnings(); len(warnings) < 1 || !diagtype.MatchesDiagError(warnings[0], "clNodeNotSched") {
			t.Fatalf("expected a warning about test-node not being schedulable, but: %#v", warnings)
		}
	*/
}
