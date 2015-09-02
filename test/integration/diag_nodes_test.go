// +build integration,!no-etcd

package integration

import (
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	clusterdiags "github.com/openshift/origin/pkg/diagnostics/cluster"
	diagtype "github.com/openshift/origin/pkg/diagnostics/types"
	testutil "github.com/openshift/origin/test/util"
)

func waitForNode(client *kclient.Client, t *testing.T) *kapi.NodeList {
	for i := 0; i < 25; i++ {
		time.Sleep(200 * time.Millisecond)
		nodeList, err := client.Nodes().List(labels.LabelSelector{}, fields.Everything())
		if err != nil {
			t.Fatalf("unexpected error fetching node list: %v", err)
		}
		if len(nodeList.Items) == 0 {
			continue
		}
		return nodeList
	}
	t.Fatal("Waited 5 seconds for all-in-one node to register itseld; giving up")
	return nil
}

func TestDiagNodeConditions(t *testing.T) {
	//masterConfig, clientFile, err := testutil.StartTestAllInOne()
	_, clientFile, err := testutil.StartTestAllInOne()
	//_, clientFile, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	client, err := testutil.GetClusterAdminKubeClient(clientFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodeDiag := clusterdiags.NodeDefinitions{KubeClient: client}
	nodeList := waitForNode(client, t)

	// start by testing that the diagnostic passes with all-in-one up.
	result := nodeDiag.Check()
	if warnings := result.Warnings(); len(warnings) > 0 {
		t.Fatalf("expected no warnings with one node ready, but: %#v", warnings)
	} else if errors := result.Errors(); len(errors) > 0 {
		t.Fatalf("expected no errors with one node ready, but: %#v", errors)
	}

	// Make the node unschedulable and verify diagnostics notices
	nodeList.Items[0].Spec.Unschedulable = true
	if _, err := client.Nodes().Update(&(nodeList.Items[0])); err != nil {
		t.Fatalf("expected no errors making node unschedulable, but: %#v", err)
	}
	result = nodeDiag.Check()
	if errors := result.Errors(); len(errors) != 1 ||
		!diagtype.MatchesDiagError(errors[0], "DClu0004") {
		t.Fatalf("expected 1 error about not having nodes, but: %#v", errors)
	} else if warnings := result.Warnings(); len(warnings) < 1 || !diagtype.MatchesDiagError(warnings[0], "DClu0003") {
		t.Fatalf("expected a warning about test-node not being schedulable, but: %#v", warnings)
	}

	// delete it and check with no nodes defined; should get an error about that.
	if err := client.Nodes().Delete(nodeList.Items[0].ObjectMeta.Name); err != nil {
		t.Fatalf("expected no errors deleting node, but: %#v", err)
	}
	if errors := nodeDiag.Check().Errors(); len(errors) != 1 ||
		!diagtype.MatchesDiagError(errors[0], "DClu0004") {
		t.Errorf("expected 1 error about not having nodes, not: %#v", errors)
	}

	// Next create a node and leave it in NotReady state. Should get a warning
	// about that, plus the previous error as there are still no nodes available.
	_, err = client.Nodes().Create(&kapi.Node{ObjectMeta: kapi.ObjectMeta{Name: "test-node"}})
	if err != nil {
		t.Fatalf("expected no errors creating a node: %#v", err)
	}
	result = nodeDiag.Check()
	if errors := result.Errors(); len(errors) != 1 ||
		!diagtype.MatchesDiagError(errors[0], "DClu0004") {
		t.Fatalf("expected 1 error about not having nodes, not: %#v", errors)
	} else if warnings := result.Warnings(); len(warnings) < 1 || !diagtype.MatchesDiagError(warnings[0], "DClu0002") {
		t.Fatalf("expected a warning about test-node not being ready, not: %#v", warnings)
	}
}
