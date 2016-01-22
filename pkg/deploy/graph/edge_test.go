package graph

import (
	"fmt"
	"testing"

	"github.com/gonum/graph"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
	"github.com/openshift/origin/pkg/deploy/api"
	nodes "github.com/openshift/origin/pkg/deploy/graph/nodes"
)

type objectifier interface {
	Object() interface{}
}

func TestNamespaceEdgeMatching(t *testing.T) {
	g := osgraph.New()

	fn := func(namespace string, g osgraph.Interface) {
		dc := &api.DeploymentConfig{}
		dc.Namespace = namespace
		dc.Name = "the-dc"
		dc.Spec.Selector = map[string]string{"a": "1"}
		nodes.EnsureDeploymentConfigNode(g, dc)

		rc := &kapi.ReplicationController{}
		rc.Namespace = namespace
		rc.Name = "the-rc"
		rc.Annotations = map[string]string{api.DeploymentConfigAnnotation: "the-dc"}
		kubegraph.EnsureReplicationControllerNode(g, rc)
	}

	fn("ns", g)
	fn("other", g)
	AddAllDeploymentEdges(g)

	if len(g.Edges()) != 4 {
		t.Fatal(g)
	}
	for _, edge := range g.Edges() {
		nsTo, err := namespaceFor(edge.To())
		if err != nil {
			t.Fatal(err)
		}
		nsFrom, err := namespaceFor(edge.From())
		if err != nil {
			t.Fatal(err)
		}
		if nsFrom != nsTo {
			t.Errorf("edge %#v crosses namespace: %s %s", edge, nsFrom, nsTo)
		}
	}
}

func namespaceFor(node graph.Node) (string, error) {
	obj := node.(objectifier).Object()
	switch t := obj.(type) {
	case runtime.Object:
		meta, err := kapi.ObjectMetaFor(t)
		if err != nil {
			return "", err
		}
		return meta.Namespace, nil
	case *kapi.PodSpec:
		return node.(*kubegraph.PodSpecNode).Namespace, nil
	case *kapi.ReplicationControllerSpec:
		return node.(*kubegraph.ReplicationControllerSpecNode).Namespace, nil
	default:
		return "", fmt.Errorf("unknown object: %#v", obj)
	}
}
