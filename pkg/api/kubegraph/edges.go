package kubegraph

import (
	"github.com/gonum/graph"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	kubegraph "github.com/openshift/origin/pkg/api/kubegraph/nodes"
)

const (
	// ExposedThroughServiceEdgeKind is an edge that goes from a podtemplatespec or a pod to service.
	// The head should make the service's selector
	ExposedThroughServiceEdgeKind = "ExposedThroughService"
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
