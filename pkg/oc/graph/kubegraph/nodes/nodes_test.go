package nodes

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/apis/core"

	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
)

func TestPodSpecNode(t *testing.T) {
	g := osgraph.New()

	pod := &kapi.Pod{}
	pod.Namespace = "ns"
	pod.Name = "foo"
	pod.Spec.NodeName = "any-host"

	podNode := EnsurePodNode(g, pod)

	if len(g.Nodes()) != 2 {
		t.Errorf("expected 2 nodes, got %v", g.Nodes())
	}

	if len(g.Edges()) != 1 {
		t.Errorf("expected 1 edge, got %v", g.Edges())
	}

	edge := g.Edges()[0]
	if !g.EdgeKinds(edge).Has(osgraph.ContainsEdgeKind) {
		t.Errorf("expected %v, got %v", osgraph.ContainsEdgeKind, g.EdgeKinds(edge))
	}
	if edge.From().ID() != podNode.ID() {
		t.Errorf("expected %v, got %v", podNode.ID(), edge.From())
	}
}

func TestReplicationControllerSpecNode(t *testing.T) {
	g := osgraph.New()

	rc := &kapi.ReplicationController{}
	rc.Namespace = "ns"
	rc.Name = "foo"
	rc.Spec.Template = &kapi.PodTemplateSpec{}

	rcNode := EnsureReplicationControllerNode(g, rc)

	if len(g.Nodes()) != 4 {
		t.Errorf("expected 4 nodes, got %v", g.Nodes())
	}

	if len(g.Edges()) != 3 {
		t.Errorf("expected 3 edge, got %v", g.Edges())
	}

	rcEdges := g.OutboundEdges(rcNode)
	if len(rcEdges) != 1 {
		t.Fatalf("expected 1 edge, got %v for \n%v", rcEdges, g)
	}
	if !g.EdgeKinds(rcEdges[0]).Has(osgraph.ContainsEdgeKind) {
		t.Errorf("expected %v, got %v", osgraph.ContainsEdgeKind, rcEdges[0])
	}

	uncastRCSpec := rcEdges[0].To()
	rcSpec, ok := uncastRCSpec.(*ReplicationControllerSpecNode)
	if !ok {
		t.Fatalf("expected rcSpec, got %v", uncastRCSpec)
	}
	rcSpecEdges := g.OutboundEdges(rcSpec)
	if len(rcSpecEdges) != 1 {
		t.Fatalf("expected 1 edge, got %v", rcSpecEdges)
	}
	if !g.EdgeKinds(rcSpecEdges[0]).Has(osgraph.ContainsEdgeKind) {
		t.Errorf("expected %v, got %v", osgraph.ContainsEdgeKind, rcSpecEdges[0])
	}

	uncastPTSpec := rcSpecEdges[0].To()
	ptSpec, ok := uncastPTSpec.(*PodTemplateSpecNode)
	if !ok {
		t.Fatalf("expected ptspec, got %v", uncastPTSpec)
	}
	ptSpecEdges := g.OutboundEdges(ptSpec)
	if len(ptSpecEdges) != 1 {
		t.Fatalf("expected 1 edge, got %v", ptSpecEdges)
	}
	if !g.EdgeKinds(ptSpecEdges[0]).Has(osgraph.ContainsEdgeKind) {
		t.Errorf("expected %v, got %v", osgraph.ContainsEdgeKind, ptSpecEdges[0])
	}

}
