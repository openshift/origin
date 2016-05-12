package graph

import (
	"fmt"
	"testing"

	"github.com/gonum/graph"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	"github.com/openshift/origin/pkg/build/api"
	nodes "github.com/openshift/origin/pkg/build/graph/nodes"
)

type objectifier interface {
	Object() interface{}
}

func TestNamespaceEdgeMatching(t *testing.T) {
	g := osgraph.New()

	fn := func(namespace string, g osgraph.Interface) {
		bc := &api.BuildConfig{}
		bc.Namespace = namespace
		bc.Name = "the-bc"
		nodes.EnsureBuildConfigNode(g, bc)

		b := &api.Build{}
		b.Namespace = namespace
		b.Name = "the-build"
		b.Labels = map[string]string{api.BuildConfigLabel: "the-bc"}
		b.Annotations = map[string]string{api.BuildConfigAnnotation: "the-bc"}
		nodes.EnsureBuildNode(g, b)
	}

	fn("ns", g)
	fn("other", g)
	AddAllBuildEdges(g)

	if len(g.Edges()) != 2 {
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
	default:
		return "", fmt.Errorf("unknown object: %#v", obj)
	}
}
