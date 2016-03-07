package analysis

import (
	"fmt"

	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	deployedges "github.com/openshift/origin/pkg/deploy/graph"
	deploygraph "github.com/openshift/origin/pkg/deploy/graph/nodes"
	imageedges "github.com/openshift/origin/pkg/image/graph"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	MissingImageStreamErr        = "MissingImageStream"
	MissingImageStreamTagWarning = "MissingImageStreamTag"
	MissingReadinessProbeWarning = "MissingReadinessProbe"
)

// FindDeploymentConfigTriggerErrors checks for possible failures in deployment config
// image change triggers.
//
// Precedence of failures:
// 1. The image stream for the tag of interest does not exist.
// 2. The image stream tag does not exist.
func FindDeploymentConfigTriggerErrors(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
	markers := []osgraph.Marker{}

dc:
	for _, uncastDcNode := range g.NodesByKind(deploygraph.DeploymentConfigNodeKind) {
		for _, uncastIstNode := range g.PredecessorNodesByEdgeKind(uncastDcNode, deployedges.TriggersDeploymentEdgeKind) {
			if istNode := uncastIstNode.(*imagegraph.ImageStreamTagNode); !istNode.Found() {
				dcNode := uncastDcNode.(*deploygraph.DeploymentConfigNode)

				// The image stream for the tag of interest does not exist.
				// TODO: Suggest `oc create imagestream` once we have that.
				if isNode, exists := doesImageStreamExist(g, uncastIstNode); !exists {
					markers = append(markers, osgraph.Marker{
						Node:         uncastDcNode,
						RelatedNodes: []graph.Node{uncastIstNode, isNode},

						Severity: osgraph.ErrorSeverity,
						Key:      MissingImageStreamErr,
						Message: fmt.Sprintf("The image trigger for %s will have no effect because %s does not exist.",
							f.ResourceName(dcNode), f.ResourceName(isNode)),
					})
					continue dc
				}

				// The image stream tag of interest does not exist.
				markers = append(markers, osgraph.Marker{
					Node:         uncastDcNode,
					RelatedNodes: []graph.Node{uncastIstNode},

					Severity: osgraph.WarningSeverity,
					Key:      MissingImageStreamTagWarning,
					Message: fmt.Sprintf("The image trigger for %s will have no effect until %s is imported or created by a build.",
						f.ResourceName(dcNode), f.ResourceName(istNode)),
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
	for _, imagestream := range g.SuccessorNodesByEdgeKind(istag, imageedges.ReferencedImageStreamImageGraphEdgeKind) {
		return imagestream, imagestream.(*imagegraph.ImageStreamNode).Found()
	}
	return nil, false
}

// FindDeploymentConfigReadinessWarnings
func FindDeploymentConfigReadinessWarnings(g osgraph.Graph, f osgraph.Namer, setProbeCommand string) []osgraph.Marker {
	markers := []osgraph.Marker{}

Node:
	for _, uncastDcNode := range g.NodesByKind(deploygraph.DeploymentConfigNodeKind) {
		dcNode := uncastDcNode.(*deploygraph.DeploymentConfigNode)
		if t := dcNode.Spec.Template; t != nil && len(t.Spec.Containers) > 0 {
			for _, container := range t.Spec.Containers {
				if container.ReadinessProbe != nil {
					continue Node
				}
			}
			// All of the containers in the deployment config lack a readiness probe
			markers = append(markers, osgraph.Marker{
				Node:     uncastDcNode,
				Severity: osgraph.WarningSeverity,
				Key:      MissingReadinessProbeWarning,
				Message: fmt.Sprintf("%s has no readiness probe to verify pods are ready to accept traffic or ensure deployment is successful.",
					f.ResourceName(dcNode)),
				Suggestion: osgraph.Suggestion(fmt.Sprintf("%s %s --readiness ...", setProbeCommand, f.ResourceName(dcNode))),
			})
			continue Node
		}
	}

	return markers
}
