package nodes

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
)

func TestPodSpecNode(t *testing.T) {
	g := osgraph.New()

	pod := &kapi.Pod{}
	pod.Namespace = "ns"
	pod.Name = "foo"
	pod.Spec.NodeName = "any-host"

	podNode := EnsurePodNode(g, pod)

	if len(g.NodeList()) != 2 {
		t.Errorf("expected 2 nodes, got %v", g.NodeList())
	}

	if len(g.EdgeList()) != 1 {
		t.Errorf("expected 1 edge, got %v", g.EdgeList())
	}

	edge := g.EdgeList()[0]
	if !g.EdgeKinds(edge).Has(osgraph.ContainsEdgeKind) {
		t.Errorf("expected %v, got %v", osgraph.ContainsEdgeKind, g.EdgeKinds(edge))
	}
	if edge.Head().ID() != podNode.ID() {
		t.Errorf("expected %v, got %v", podNode.ID(), edge.Head())
	}
}

func TestReplicationControllerSpecNode(t *testing.T) {
	g := osgraph.New()

	rc := &kapi.ReplicationController{}
	rc.Namespace = "ns"
	rc.Name = "foo"
	rc.Spec.Template = &kapi.PodTemplateSpec{}

	rcNode := EnsureReplicationControllerNode(g, rc)

	if len(g.NodeList()) != 4 {
		t.Errorf("expected 4 nodes, got %v", g.NodeList())
	}

	if len(g.EdgeList()) != 3 {
		t.Errorf("expected 3 edge, got %v", g.EdgeList())
	}

	rcEdges := g.OutboundEdges(rcNode)
	if len(rcEdges) != 1 {
		t.Fatalf("expected 1 edge, got %v", rcEdges)
	}
	if !g.EdgeKinds(rcEdges[0]).Has(osgraph.ContainsEdgeKind) {
		t.Errorf("expected %v, got %v", osgraph.ContainsEdgeKind, rcEdges[0])
	}

	uncastRCSpec := rcEdges[0].Tail()
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

	uncastPTSpec := rcSpecEdges[0].Tail()
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
