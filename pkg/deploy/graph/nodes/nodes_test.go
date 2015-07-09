package nodes

import (
	"testing"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

func TestDCRCSpecNode(t *testing.T) {
	g := osgraph.New()

	dc := &deployapi.DeploymentConfig{}
	dc.Namespace = "ns"
	dc.Name = "foo"

	dcNode := EnsureDeploymentConfigNode(g, dc)

	if len(g.NodeList()) != 2 {
		t.Errorf("expected 2 nodes, got %v", g.NodeList())
	}

	if len(g.EdgeList()) != 1 {
		t.Errorf("expected 2 edge, got %v", g.EdgeList())
	}

	edge := g.EdgeList()[0]
	if !g.EdgeKinds(edge).Has(osgraph.ContainsEdgeKind) {
		t.Errorf("expected %v, got %v", osgraph.ContainsEdgeKind, g.EdgeKinds(edge))
	}
	if edge.Head().ID() != dcNode.ID() {
		t.Errorf("expected %v, got %v", dcNode.ID(), edge.Head())
	}
}
