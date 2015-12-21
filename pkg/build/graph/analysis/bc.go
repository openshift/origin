package analysis

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gonum/graph"
	"github.com/gonum/graph/topo"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	imageedges "github.com/openshift/origin/pkg/image/graph"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	TagNotAvailableWarning     = "ImageStreamTagNotAvailable"
	LatestBuildFailedErr       = "LatestBuildFailed"
	MissingRequiredRegistryErr = "MissingRequiredRegistry"
	MissingImageStreamErr      = "MissingImageStream"
	CyclicBuildConfigWarning   = "CyclicBuildConfig"
)

// FindUnpushableBuildConfigs checks all build configs that will output to an IST backed by an ImageStream and checks to make sure their builds can push.
func FindUnpushableBuildConfigs(g osgraph.Graph) []osgraph.Marker {
	markers := []osgraph.Marker{}

bc:
	for _, bcNode := range g.NodesByKind(buildgraph.BuildConfigNodeKind) {
		for _, istNode := range g.SuccessorNodesByEdgeKind(bcNode, buildedges.BuildOutputEdgeKind) {
			for _, uncastImageStreamNode := range g.SuccessorNodesByEdgeKind(istNode, imageedges.ReferencedImageStreamGraphEdgeKind) {
				imageStreamNode := uncastImageStreamNode.(*imagegraph.ImageStreamNode)

				if !imageStreamNode.IsFound {
					markers = append(markers, osgraph.Marker{
						Node:         bcNode,
						RelatedNodes: []graph.Node{istNode},

						Severity: osgraph.ErrorSeverity,
						Key:      MissingImageStreamErr,
						Message: fmt.Sprintf("%s is pushing to %s that is using %s, but that image stream does not exist.",
							bcNode.(*buildgraph.BuildConfigNode).ResourceString(), istNode.(*imagegraph.ImageStreamTagNode).ResourceString(), imageStreamNode.ResourceString()),
					})

					continue
				}

				if len(imageStreamNode.Status.DockerImageRepository) == 0 {
					markers = append(markers, osgraph.Marker{
						Node:         bcNode,
						RelatedNodes: []graph.Node{istNode},

						Severity: osgraph.ErrorSeverity,
						Key:      MissingRequiredRegistryErr,
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

// FindPendingTags inspects all imageStreamTags that serve as outputs to builds.
//
// Precedence of failures:
// 1. A build config points to the non existent tag but no current build exists.
// 2. A build config points to the non existent tag but the latest build has failed.
func FindPendingTags(g osgraph.Graph) []osgraph.Marker {
	markers := []osgraph.Marker{}

	for _, uncastIstNode := range g.NodesByKind(imagegraph.ImageStreamTagNodeKind) {
		istNode := uncastIstNode.(*imagegraph.ImageStreamTagNode)
		if bcNode, points := buildPointsToTag(g, uncastIstNode); points && !istNode.Found() {
			latestBuild := latestBuild(g, bcNode)

			// A build config points to the non existent tag but no current build exists.
			if latestBuild == nil {
				markers = append(markers, osgraph.Marker{
					Node:         graph.Node(bcNode),
					RelatedNodes: []graph.Node{uncastIstNode},

					Severity:   osgraph.WarningSeverity,
					Key:        TagNotAvailableWarning,
					Message:    fmt.Sprintf("%s needs to be imported or created by a build.", istNode.ResourceString()),
					Suggestion: osgraph.Suggestion(fmt.Sprintf("oc start-build %s", bcNode.ResourceString())),
				})
				continue
			}

			// A build config points to the non existent tag but something is going on with
			// the latest build.
			// TODO: Handle other build phases.
			switch latestBuild.Build.Status.Phase {
			case buildapi.BuildPhaseCancelled:
				// TODO: Add a warning here.
			case buildapi.BuildPhaseError:
				// TODO: Add a warning here.
			case buildapi.BuildPhaseComplete:
				// We should never hit this. The output of our build is missing but the build is complete.
				// Most probably the user has messed up?
			case buildapi.BuildPhaseFailed:
				// Since the tag hasn't been populated yet, we assume there hasn't been a successful
				// build so far.
				markers = append(markers, osgraph.Marker{
					Node:         graph.Node(latestBuild),
					RelatedNodes: []graph.Node{uncastIstNode, graph.Node(bcNode)},

					Severity:   osgraph.ErrorSeverity,
					Key:        LatestBuildFailedErr,
					Message:    fmt.Sprintf("%s has failed.", latestBuild.ResourceString()),
					Suggestion: osgraph.Suggestion(fmt.Sprintf("Inspect the build failure with 'oc logs %s'", latestBuild.ResourceString())),
				})
			default:
				// Do nothing when latest build is new, pending, or running.
			}
		}
	}

	return markers
}

// buildPointsToTag returns the buildConfig that points to the provided imageStreamTag.
func buildPointsToTag(g osgraph.Graph, istag graph.Node) (*buildgraph.BuildConfigNode, bool) {
	for _, bcNode := range g.PredecessorNodesByEdgeKind(istag, buildedges.BuildOutputEdgeKind) {
		return bcNode.(*buildgraph.BuildConfigNode), true
	}
	return nil, false
}

// latestBuild returns the latest build for the provided buildConfig.
func latestBuild(g osgraph.Graph, bc graph.Node) *buildgraph.BuildNode {
	builds := []*buildapi.Build{}
	buildNameToNode := map[string]*buildgraph.BuildNode{}
	for _, buildNode := range g.SuccessorNodesByEdgeKind(bc, buildedges.BuildEdgeKind) {
		build := buildNode.(*buildgraph.BuildNode)
		buildNameToNode[build.Build.Name] = build
		builds = append(builds, build.Build)
	}
	if len(builds) == 0 {
		return nil
	}
	sort.Sort(sort.Reverse(buildapi.BuildPtrSliceByCreationTimestamp(builds)))
	return buildNameToNode[builds[0].Name]
}
