package kubegraph

import (
	"github.com/gonum/graph"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

const (
	// ExposedThroughServiceEdgeKind goes from a PodTemplateSpec or a Pod to Service.  The head should make the service's selector.
	ExposedThroughServiceEdgeKind = "ExposedThroughService"
	// ManagedByRCEdgeKind goes from Pod to ReplicationController when the Pod satisfies the ReplicationController's label selector
	ManagedByRCEdgeKind = "ManagedByRC"
	// MountedSecretEdgeKind goes from PodSpec to Secret indicating that is or will be a request to mount a volume with the Secret.
	MountedSecretEdgeKind = "MountedSecret"
	// MountableSecretEdgeKind goes from ServiceAccount to Secret indicating that the SA allows the Secret to be mounted
	MountableSecretEdgeKind = "MountableSecret"
	// ReferencedServiceAccountEdgeKind goes from PodSpec to ServiceAccount indicating that Pod is or will be running as the SA.
	ReferencedServiceAccountEdgeKind = "ReferencedServiceAccount"
)

// AddExposedPodTemplateSpecEdges ensures that a directed edge exists between a service and all the PodTemplateSpecs
// in the graph that match the service selector
func AddExposedPodTemplateSpecEdges(g osgraph.MutableUniqueGraph, node *kubegraph.ServiceNode) {
	if node.Service.Spec.Selector == nil {
		return
	}
	query := labels.SelectorFromSet(node.Service.Spec.Selector)
	for _, n := range g.(graph.Graph).NodeList() {
		switch target := n.(type) {
		case *kubegraph.PodTemplateSpecNode:
			if query.Matches(labels.Set(target.PodTemplateSpec.Labels)) {
				g.AddEdge(target, node, ExposedThroughServiceEdgeKind)
			}
		}
	}
}

// AddAllExposedPodTemplateSpecEdges calls AddExposedPodTemplateSpecEdges for every ServiceNode in the graph
func AddAllExposedPodTemplateSpecEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).NodeList() {
		if serviceNode, ok := node.(*kubegraph.ServiceNode); ok {
			AddExposedPodTemplateSpecEdges(g, serviceNode)
		}
	}
}

// AddExposedPodEdges ensures that a directed edge exists between a service and all the pods
// in the graph that match the service selector
func AddExposedPodEdges(g osgraph.MutableUniqueGraph, node *kubegraph.ServiceNode) {
	if node.Service.Spec.Selector == nil {
		return
	}
	query := labels.SelectorFromSet(node.Service.Spec.Selector)
	for _, n := range g.(graph.Graph).NodeList() {
		switch target := n.(type) {
		case *kubegraph.PodNode:
			if query.Matches(labels.Set(target.Labels)) {
				g.AddEdge(target, node, ExposedThroughServiceEdgeKind)
			}
		}
	}
}

// AddAllExposedPodEdges calls AddExposedPodEdges for every ServiceNode in the graph
func AddAllExposedPodEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).NodeList() {
		if serviceNode, ok := node.(*kubegraph.ServiceNode); ok {
			AddExposedPodEdges(g, serviceNode)
		}
	}
}

// AddManagedByRCPodEdges ensures that a directed edge exists between an RC and all the pods
// in the graph that match the label selector
func AddManagedByRCPodEdges(g osgraph.MutableUniqueGraph, rcNode *kubegraph.ReplicationControllerNode) {
	if rcNode.Spec.Selector == nil {
		return
	}
	query := labels.SelectorFromSet(rcNode.Spec.Selector)
	for _, n := range g.(graph.Graph).NodeList() {
		switch target := n.(type) {
		case *kubegraph.PodNode:
			if query.Matches(labels.Set(target.Labels)) {
				g.AddEdge(target, rcNode, ManagedByRCEdgeKind)
			}
		}
	}
}

// AddAllManagedByRCPodEdges calls AddManagedByRCPodEdges for every ServiceNode in the graph
func AddAllManagedByRCPodEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).NodeList() {
		if rcNode, ok := node.(*kubegraph.ReplicationControllerNode); ok {
			AddManagedByRCPodEdges(g, rcNode)
		}
	}
}

func AddMountedSecretEdges(g osgraph.Graph, podSpec *kubegraph.PodSpecNode) {
	//pod specs are always contained.  We'll get the toplevel container so that we can pull a namespace from it
	containerNode := osgraph.GetTopLevelContainerNode(g, podSpec)
	containerObj := g.GraphDescriber.Object(containerNode)

	meta, err := kapi.ObjectMetaFor(containerObj.(runtime.Object))
	if err != nil {
		// this should never happen.  it means that a podSpec is owned by a top level container that is not a runtime.Object
		panic(err)
	}

	for _, volume := range podSpec.Volumes {
		source := volume.VolumeSource
		if source.Secret == nil {
			continue
		}

		// pod secrets must be in the same namespace
		syntheticSecret := &kapi.Secret{}
		syntheticSecret.Namespace = meta.Namespace
		syntheticSecret.Name = source.Secret.SecretName

		secretNode := kubegraph.FindOrCreateSyntheticSecretNode(g, syntheticSecret)
		g.AddEdge(podSpec, secretNode, MountedSecretEdgeKind)
	}
}

func AddAllMountedSecretEdges(g osgraph.Graph) {
	for _, node := range g.NodeList() {
		if podSpecNode, ok := node.(*kubegraph.PodSpecNode); ok {
			AddMountedSecretEdges(g, podSpecNode)
		}
	}
}

func AddMountableSecretEdges(g osgraph.Graph, saNode *kubegraph.ServiceAccountNode) {
	for _, mountableSecret := range saNode.ServiceAccount.Secrets {
		syntheticSecret := &kapi.Secret{}
		syntheticSecret.Namespace = saNode.ServiceAccount.Namespace
		syntheticSecret.Name = mountableSecret.Name

		secretNode := kubegraph.FindOrCreateSyntheticSecretNode(g, syntheticSecret)
		g.AddEdge(saNode, secretNode, MountableSecretEdgeKind)
	}
}

func AddAllMountableSecretEdges(g osgraph.Graph) {
	for _, node := range g.NodeList() {
		if saNode, ok := node.(*kubegraph.ServiceAccountNode); ok {
			AddMountableSecretEdges(g, saNode)
		}
	}
}

func AddRequestedServiceAccountEdges(g osgraph.Graph, podSpecNode *kubegraph.PodSpecNode) {
	//pod specs are always contained.  We'll get the toplevel container so that we can pull a namespace from it
	containerNode := osgraph.GetTopLevelContainerNode(g, podSpecNode)
	containerObj := g.GraphDescriber.Object(containerNode)

	meta, err := kapi.ObjectMetaFor(containerObj.(runtime.Object))
	if err != nil {
		panic(err)
	}

	// if no SA name is present, admission will set 'default'
	name := "default"
	if len(podSpecNode.ServiceAccountName) > 0 {
		name = podSpecNode.ServiceAccountName
	}

	syntheticSA := &kapi.ServiceAccount{}
	syntheticSA.Namespace = meta.Namespace
	syntheticSA.Name = name

	saNode := kubegraph.FindOrCreateSyntheticServiceAccountNode(g, syntheticSA)
	g.AddEdge(podSpecNode, saNode, ReferencedServiceAccountEdgeKind)
}

func AddAllRequestedServiceAccountEdges(g osgraph.Graph) {
	for _, node := range g.NodeList() {
		if podSpecNode, ok := node.(*kubegraph.PodSpecNode); ok {
			AddRequestedServiceAccountEdges(g, podSpecNode)
		}
	}
}
