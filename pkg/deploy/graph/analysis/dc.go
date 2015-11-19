package analysis

import (
	"fmt"

	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	deployedges "github.com/openshift/origin/pkg/deploy/graph"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	imageedges "github.com/openshift/origin/pkg/image/graph"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	ImageStreamTagNotAvailableInfo = "ImageStreamTagNotAvailable"
	MissingImageStreamWarning      = "MissingImageStream"
	MissingImageStreamTagWarning   = "MissingImageStreamTag"
)

// FindDeploymentConfigTriggerErrors checks for possible failures in deployment config
// image change triggers.
//
// Precedence of failures:
// 1. The image stream of the tag of interest does not exist.
// 2. The image stream tag does not exist but a build config points to it.
// 3. The image stream tag does not exist.
func FindDeploymentConfigTriggerErrors(g osgraph.Graph) []osgraph.Marker {
	markers := []osgraph.Marker{}

dc:
	for _, uncastDcNode := range g.NodesByKind(deploygraph.DeploymentConfigNodeKind) {
		for _, uncastIstNode := range g.PredecessorNodesByEdgeKind(uncastDcNode, deployedges.TriggersDeploymentEdgeKind) {
			if istNode := uncastIstNode.(*imagegraph.ImageStreamTagNode); !istNode.Found() {
				dcNode := uncastDcNode.(*deploygraph.DeploymentConfigNode)

				// 1. Image stream for tag of interest does not exist
				if isNode, exists := doesImageStreamExist(g, uncastIstNode); !exists {
					markers = append(markers, osgraph.Marker{
						Node:         uncastDcNode,
						RelatedNodes: []graph.Node{uncastIstNode, isNode},

						Severity: osgraph.WarningSeverity,
						Key:      MissingImageStreamWarning,
						Message: fmt.Sprintf("The image trigger for %s will have no effect because %s does not exist.",
							dcNode.ResourceString(), isNode.(*imagegraph.ImageStreamNode).ResourceString()),
					})
					continue dc
				}

				// 2. Build config points to image stream tag of interest
				if bcNode, points := buildPointsToTag(g, uncastIstNode); points {
					markers = append(markers, osgraph.Marker{
						Node:         uncastDcNode,
						RelatedNodes: []graph.Node{uncastIstNode, bcNode},

						Severity: osgraph.InfoSeverity,
						Key:      ImageStreamTagNotAvailableInfo,
						Message: fmt.Sprintf("The image trigger for %s will have no effect because %s does not exist but %s points to %s.",
							dcNode.ResourceString(), istNode.ResourceString(), bcNode.(*buildgraph.BuildConfigNode).ResourceString(), istNode.ResourceString()),
					})
					continue dc
				}

				// 3. Image stream tag of interest does not exist
				markers = append(markers, osgraph.Marker{
					Node:         uncastDcNode,
					RelatedNodes: []graph.Node{uncastIstNode},

					Severity: osgraph.WarningSeverity,
					Key:      MissingImageStreamTagWarning,
					Message: fmt.Sprintf("The image trigger for %s will have no effect because %s does not exist.",
						dcNode.ResourceString(), istNode.ResourceString()),
				})
				continue dc
			}
		}
	}

	return markers
}

func doesImageStreamExist(g osgraph.Graph, istag graph.Node) (graph.Node, bool) {
	for _, imagestream := range g.SuccessorNodesByEdgeKind(istag, imageedges.ReferencedImageStreamGraphEdgeKind) {
		return imagestream, imagestream.(*imagegraph.ImageStreamNode).Found()
	}
	return nil, false
}

func buildPointsToTag(g osgraph.Graph, istag graph.Node) (graph.Node, bool) {
	for _, bcNode := range g.PredecessorNodesByEdgeKind(istag, buildedges.BuildOutputEdgeKind) {
		return bcNode, true
	}
	return nil, false
}
