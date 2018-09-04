package appsgraph

import (
	"fmt"
	"testing"

	"github.com/gonum/graph"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	appsv1 "github.com/openshift/api/apps/v1"
	nodes "github.com/openshift/origin/pkg/oc/lib/graph/appsgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	kubegraph "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
)

type objectifier interface {
	Object() interface{}
}

func TestNamespaceEdgeMatching(t *testing.T) {
	g := osgraph.New()

	fn := func(namespace string, g osgraph.Interface) {
		dc := &appsv1.DeploymentConfig{}
		dc.Namespace = namespace
		dc.Name = "the-dc"
		dc.Spec.Selector = map[string]string{"a": "1"}
		nodes.EnsureDeploymentConfigNode(g, dc)

		rc := &corev1.ReplicationController{}
		rc.Namespace = namespace
		rc.Name = "the-rc"
		rc.Annotations = map[string]string{appsv1.DeploymentConfigAnnotation: "the-dc"}
		kubegraph.EnsureReplicationControllerNode(g, rc)
	}

	fn("ns", g)
	fn("other", g)
	AddAllDeploymentConfigsDeploymentEdges(g)

	if len(g.Edges()) != 6 {
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
		meta, err := meta.Accessor(t)
		if err != nil {
			return "", err
		}
		return meta.GetNamespace(), nil
	case *corev1.PodSpec:
		return node.(*kubegraph.PodSpecNode).Namespace, nil
	case *corev1.ReplicationControllerSpec:
		return node.(*kubegraph.ReplicationControllerSpecNode).Namespace, nil
	default:
		return "", fmt.Errorf("unknown object: %#v", obj)
	}
}
