package kubegraph

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

func TestSecretEdges(t *testing.T) {
	sa := &kapi.ServiceAccount{}
	sa.Namespace = "ns"
	sa.Name = "shultz"
	sa.Secrets = []kapi.ObjectReference{{Name: "i-know-nothing"}, {Name: "missing"}}

	secret1 := &kapi.Secret{}
	secret1.Namespace = "ns"
	secret1.Name = "i-know-nothing"

	pod := &kapi.Pod{}
	pod.Namespace = "ns"
	pod.Name = "the-pod"
	pod.Spec.Volumes = []kapi.Volume{{Name: "rose", VolumeSource: kapi.VolumeSource{Secret: &kapi.SecretVolumeSource{SecretName: "i-know-nothing"}}}}

	g := osgraph.New()
	saNode := kubegraph.EnsureServiceAccountNode(g, sa)
	secretNode := kubegraph.EnsureSecretNode(g, secret1)
	podNode := kubegraph.EnsurePodNode(g, pod)

	AddAllMountableSecretEdges(g)
	AddAllMountedSecretEdges(g)

	if edge := g.EdgeBetween(saNode, secretNode); edge == nil {
		t.Errorf("edge missing")
	} else {
		if !g.EdgeKinds(edge).Has(MountableSecretEdgeKind) {
			t.Errorf("expected %v, got %v", MountableSecretEdgeKind, edge)
		}
	}

	podSpecNodes := g.SuccessorNodesByNodeAndEdgeKind(podNode, kubegraph.PodSpecNodeKind, osgraph.ContainsEdgeKind)
	if len(podSpecNodes) != 1 {
		t.Fatalf("wrong number of podspecs: %v", podSpecNodes)
	}

	if edge := g.EdgeBetween(podSpecNodes[0], secretNode); edge == nil {
		t.Errorf("edge missing")
	} else {
		if !g.EdgeKinds(edge).Has(MountedSecretEdgeKind) {
			t.Errorf("expected %v, got %v", MountedSecretEdgeKind, edge)
		}
	}
}
