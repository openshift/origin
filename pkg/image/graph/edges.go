package graph

import (
	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	// ReferencedImageStreamGraphEdgeKind is an edge that goes from an ImageStreamTag node back to an ImageStream
	ReferencedImageStreamGraphEdgeKind = "ReferencedImageStreamGraphEdge"
)

// AddImageStreamRefEdge ensures that a directed edge exists between an IST Node and the IS it references
func AddImageStreamRefEdge(g osgraph.MutableUniqueGraph, node *imagegraph.ImageStreamTagNode) {
	isName, _, _ := imageapi.SplitImageStreamTag(node.Name)
	imageStream := &imageapi.ImageStream{}
	imageStream.Namespace = node.Namespace
	imageStream.Name = isName

	imageStreamNode := imagegraph.FindOrCreateSyntheticImageStreamNode(g, imageStream)
	g.AddEdge(node, imageStreamNode, ReferencedImageStreamGraphEdgeKind)
}

// AddAllImageStreamRefEdges calls AddImageStreamRefEdge for every ImageStreamTagNode in the graph
func AddAllImageStreamRefEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).NodeList() {
		if istNode, ok := node.(*imagegraph.ImageStreamTagNode); ok {
			AddImageStreamRefEdge(g, istNode)
		}
	}
}
