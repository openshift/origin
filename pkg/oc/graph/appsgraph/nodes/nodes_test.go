package nodes

import (
	"testing"

	"github.com/gonum/graph/topo"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	"github.com/openshift/origin/pkg/apps/apis/apps/test"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
	kubetypes "github.com/openshift/origin/pkg/oc/graph/kubegraph/nodes"
)

func TestDCPodTemplateSpecNode(t *testing.T) {
	g := osgraph.New()

	dc := &appsapi.DeploymentConfig{}
	dc.Namespace = "ns"
	dc.Name = "foo"
	dc.Spec.Template = test.OkPodTemplate()

	_ = EnsureDeploymentConfigNode(g, dc)

	edges := g.Edges()
	if len(edges) != 2 {
		t.Errorf("expected 2 edges, got %d", len(edges))
		return
	}
	for i := range edges {
		if !g.EdgeKinds(edges[i]).Has(osgraph.ContainsEdgeKind) {
			t.Errorf("expected %v, got %v", osgraph.ContainsEdgeKind, g.EdgeKinds(edges[i]))
			return
		}
	}

	nodes := g.Nodes()
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(nodes))
		return
	}
	sorted, err := topo.Sort(g)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	// Just to be sure
	if len(sorted) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(sorted))
		return
	}
	if _, ok := sorted[0].(*DeploymentConfigNode); !ok {
		t.Errorf("expected first node to be a DeploymentConfigNode")
		return
	}
	if _, ok := sorted[1].(*kubetypes.PodTemplateSpecNode); !ok {
		t.Errorf("expected second node to be a PodTemplateSpecNode")
		return
	}
	if _, ok := sorted[2].(*kubetypes.PodSpecNode); !ok {
		t.Errorf("expected third node to be a PodSpecNode")
	}
}
