package analysis

import (
	"fmt"
	"strings"

	"github.com/gonum/graph"
	"github.com/gonum/graph/topo"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	imageedges "github.com/openshift/origin/pkg/image/graph"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	MissingRequiredRegistryWarning = "MissingRequiredRegistry"
	CyclicBuildConfigWarning       = "CyclicBuildConfig"
)

// FindUnpushableBuildConfigs checks all build configs that will output to an IST backed by an ImageStream and checks to make sure their builds can push.
func FindUnpushableBuildConfigs(g osgraph.Graph) []osgraph.Marker {
	markers := []osgraph.Marker{}

bc:
	for _, bcNode := range g.NodesByKind(buildgraph.BuildConfigNodeKind) {
		for _, istNode := range g.SuccessorNodesByEdgeKind(bcNode, buildedges.BuildOutputEdgeKind) {
			for _, uncastImageStreamNode := range g.SuccessorNodesByEdgeKind(istNode, imageedges.ReferencedImageStreamGraphEdgeKind) {
				imageStreamNode := uncastImageStreamNode.(*imagegraph.ImageStreamNode)

				if len(imageStreamNode.Status.DockerImageRepository) == 0 {
					markers = append(markers, osgraph.Marker{
						Node:         bcNode,
						RelatedNodes: []graph.Node{istNode},

						Severity: osgraph.WarningSeverity,
						Key:      MissingRequiredRegistryWarning,
						Message: fmt.Sprintf("%s is pushing to %s that is using %s, but the administrator has not configured the integrated Docker registry.  (oadm registry)",
							bcNode.(*buildgraph.BuildConfigNode).ResourceString(), istNode.(*imagegraph.ImageStreamTagNode).ResourceString(), imageStreamNode.ResourceString()),
					})

					continue bc
				}
			}
		}
	}

	return markers
}

// FindCircularBuilds checks all build configs for cycles
func FindCircularBuilds(g osgraph.Graph) []osgraph.Marker {
	// Filter out all but ImageStreamTag and BuildConfig nodes
	nodeFn := osgraph.NodesOfKind(imagegraph.ImageStreamTagNodeKind, buildgraph.BuildConfigNodeKind)
	// Filter out all but BuildInputImage and BuildOutput edges
	edgeFn := osgraph.EdgesOfKind(buildedges.BuildInputImageEdgeKind, buildedges.BuildOutputEdgeKind)

	// Create desired subgraph
	sub := g.Subgraph(nodeFn, edgeFn)

	markers := []osgraph.Marker{}

	// Check for cycles
	for _, cycle := range topo.CyclesIn(sub) {
		nodeNames := []string{}
		for _, node := range cycle {
			if resourceStringer, ok := node.(osgraph.ResourceNode); ok {
				nodeNames = append(nodeNames, resourceStringer.ResourceString())
			}
		}

		markers = append(markers, osgraph.Marker{
			Node:         cycle[0],
			RelatedNodes: cycle,

			Severity: osgraph.WarningSeverity,
			Key:      CyclicBuildConfigWarning,
			Message:  fmt.Sprintf("Cycle detected in build configurations: %s", strings.Join(nodeNames, " -> ")),
		})

	}

	return markers
}
