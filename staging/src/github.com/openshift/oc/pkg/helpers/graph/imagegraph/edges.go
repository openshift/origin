package imagegraph

import (
	"github.com/gonum/graph"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/image/imageutil"
	"github.com/openshift/library-go/pkg/image/reference"
	osgraph "github.com/openshift/oc/pkg/helpers/graph/genericgraph"
	imagegraph "github.com/openshift/oc/pkg/helpers/graph/imagegraph/nodes"
)

const (
	// ReferencedImageStreamGraphEdgeKind is an edge that goes from an ImageStreamTag node back to an ImageStream
	ReferencedImageStreamGraphEdgeKind = "ReferencedImageStreamGraphEdge"
	// ReferencedImageStreamImageGraphEdgeKind is an edge that goes from an ImageStreamImage node back to an ImageStream
	ReferencedImageStreamImageGraphEdgeKind = "ReferencedImageStreamImageGraphEdgeKind"
)

// AddImageStreamTagRefEdge ensures that a directed edge exists between an IST Node and the IS it references
func AddImageStreamTagRefEdge(g osgraph.MutableUniqueGraph, node *imagegraph.ImageStreamTagNode) {
	isName, _, _ := imageutil.SplitImageStreamTag(node.Name)
	imageStream := &imagev1.ImageStream{}
	imageStream.Namespace = node.Namespace
	imageStream.Name = isName

	imageStreamNode := imagegraph.FindOrCreateSyntheticImageStreamNode(g, imageStream)
	g.AddEdge(node, imageStreamNode, ReferencedImageStreamGraphEdgeKind)
}

// AddImageStreamImageRefEdge ensures that a directed edge exists between an ImageStreamImage Node and the IS it references
func AddImageStreamImageRefEdge(g osgraph.MutableUniqueGraph, node *imagegraph.ImageStreamImageNode) {
	dockImgRef, _ := reference.Parse(node.Name)
	imageStream := &imagev1.ImageStream{}
	imageStream.Namespace = node.Namespace
	imageStream.Name = dockImgRef.Name

	imageStreamNode := imagegraph.FindOrCreateSyntheticImageStreamNode(g, imageStream)
	g.AddEdge(node, imageStreamNode, ReferencedImageStreamImageGraphEdgeKind)
}

// AddAllImageStreamRefEdges calls AddImageStreamRefEdge for every ImageStreamTagNode in the graph
func AddAllImageStreamRefEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		if istNode, ok := node.(*imagegraph.ImageStreamTagNode); ok {
			AddImageStreamTagRefEdge(g, istNode)
		}
	}
}

// AddAllImageStreamImageRefEdges calls AddImageStreamImageRefEdge for every ImageStreamImageNode in the graph
func AddAllImageStreamImageRefEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).Nodes() {
		if isimageNode, ok := node.(*imagegraph.ImageStreamImageNode); ok {
			AddImageStreamImageRefEdge(g, isimageNode)
		}
	}
}
