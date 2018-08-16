package kubegraph

import (
	"fmt"
	"testing"

	"github.com/gonum/graph"

	kappsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	_ "k8s.io/kubernetes/pkg/apis/core/install"

	appsv1 "github.com/openshift/api/apps/v1"
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
	appsgraph "github.com/openshift/origin/pkg/oc/lib/graph/appsgraph/nodes"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
	kubegraph "github.com/openshift/origin/pkg/oc/lib/graph/kubegraph/nodes"
)

type objectifier interface {
	Object() interface{}
}

func TestNamespaceEdgeMatching(t *testing.T) {
	g := osgraph.New()

	fn := func(namespace string, g osgraph.Interface) {
		pod := &corev1.Pod{}
		pod.Namespace = namespace
		pod.Name = "the-pod"
		pod.Labels = map[string]string{"a": "1"}
		kubegraph.EnsurePodNode(g, pod)

		rc := &corev1.ReplicationController{}
		rc.Namespace = namespace
		rc.Name = "the-rc"
		rc.Spec.Selector = map[string]string{"a": "1"}
		kubegraph.EnsureReplicationControllerNode(g, rc)

		p := &kappsv1.StatefulSet{}
		p.Namespace = namespace
		p.Name = "the-statefulset"
		p.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: map[string]string{"a": "1"},
		}
		kubegraph.EnsureStatefulSetNode(g, p)

		svc := &corev1.Service{}
		svc.Namespace = namespace
		svc.Name = "the-svc"
		svc.Spec.Selector = map[string]string{"a": "1"}
		kubegraph.EnsureServiceNode(g, svc)
	}

	fn("ns", g)
	fn("other", g)
	AddAllExposedPodEdges(g)
	AddAllExposedPodTemplateSpecEdges(g)
	AddAllManagedByControllerPodEdges(g)

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
	case *kappsv1.StatefulSetSpec:
		return node.(*kubegraph.StatefulSetSpecNode).Namespace, nil
	case *corev1.PodTemplateSpec:
		return node.(*kubegraph.PodTemplateSpecNode).Namespace, nil
	default:
		return "", fmt.Errorf("unknown object: %#v", obj)
	}
}

func TestSecretEdges(t *testing.T) {
	sa := &corev1.ServiceAccount{}
	sa.Namespace = "ns"
	sa.Name = "shultz"
	sa.Secrets = []corev1.ObjectReference{{Name: "i-know-nothing"}, {Name: "missing"}}

	secret1 := &corev1.Secret{}
	secret1.Namespace = "ns"
	secret1.Name = "i-know-nothing"

	pod := &corev1.Pod{}
	pod.Namespace = "ns"
	pod.Name = "the-pod"
	pod.Spec.Volumes = []corev1.Volume{{Name: "rose", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{SecretName: "i-know-nothing"}}}}

	g := osgraph.New()
	saNode := kubegraph.EnsureServiceAccountNode(g, sa)
	secretNode := kubegraph.EnsureSecretNode(g, secret1)
	podNode := kubegraph.EnsurePodNode(g, pod)

	AddAllMountableSecretEdges(g)
	AddAllMountedSecretEdges(g)

	if edge := g.Edge(saNode, secretNode); edge == nil {
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

	if edge := g.Edge(podSpecNodes[0], secretNode); edge == nil {
		t.Errorf("edge missing")
	} else {
		if !g.EdgeKinds(edge).Has(MountedSecretEdgeKind) {
			t.Errorf("expected %v, got %v", MountedSecretEdgeKind, edge)
		}
	}
}

func TestHPARCEdges(t *testing.T) {
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	hpa.Namespace = "test-ns"
	hpa.Name = "test-hpa"
	hpa.Spec = autoscalingv1.HorizontalPodAutoscalerSpec{
		ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
			Name: "test-rc",
			Kind: "ReplicationController",
		},
	}

	rc := &corev1.ReplicationController{}
	rc.Name = "test-rc"
	rc.Namespace = "test-ns"

	g := osgraph.New()
	hpaNode := kubegraph.EnsureHorizontalPodAutoscalerNode(g, hpa)
	rcNode := kubegraph.EnsureReplicationControllerNode(g, rc)

	AddHPAScaleRefEdges(g, testrestmapper.TestOnlyStaticRESTMapper(legacyscheme.Scheme))

	if edge := g.Edge(hpaNode, rcNode); edge == nil {
		t.Fatalf("edge between HPA and RC missing")
	} else {
		if !g.EdgeKinds(edge).Has(ScalingEdgeKind) {
			t.Errorf("expected edge to have kind %v, got %v", ScalingEdgeKind, edge)
		}
	}
}

func TestHPADCEdges(t *testing.T) {
	hpa := &autoscalingv1.HorizontalPodAutoscaler{}
	hpa.Namespace = "test-ns"
	hpa.Name = "test-hpa"
	hpa.Spec = autoscalingv1.HorizontalPodAutoscalerSpec{
		ScaleTargetRef: autoscalingv1.CrossVersionObjectReference{
			Name:       "test-dc",
			Kind:       "DeploymentConfig",
			APIVersion: "apps.openshift.io/v1",
		},
	}

	dc := &appsv1.DeploymentConfig{}
	dc.Name = "test-dc"
	dc.Namespace = "test-ns"

	g := osgraph.New()
	hpaNode := kubegraph.EnsureHorizontalPodAutoscalerNode(g, hpa)
	dcNode := appsgraph.EnsureDeploymentConfigNode(g, dc)

	AddHPAScaleRefEdges(g, testrestmapper.TestOnlyStaticRESTMapper(legacyscheme.Scheme))

	if edge := g.Edge(hpaNode, dcNode); edge == nil {
		t.Fatalf("edge between HPA and DC missing")
	} else {
		if !g.EdgeKinds(edge).Has(ScalingEdgeKind) {
			t.Errorf("expected edge to have kind %v, got %v", ScalingEdgeKind, edge)
		}
	}
}
