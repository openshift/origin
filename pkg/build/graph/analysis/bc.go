package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gonum/graph"
	"github.com/gonum/graph/topo"

	"k8s.io/kubernetes/pkg/api/unversioned"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imageedges "github.com/openshift/origin/pkg/image/graph"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	TagNotAvailableWarning         = "ImageStreamTagNotAvailable"
	LatestBuildFailedErr           = "LatestBuildFailed"
	MissingRequiredRegistryErr     = "MissingRequiredRegistry"
	MissingOutputImageStreamErr    = "MissingOutputImageStream"
	CyclicBuildConfigWarning       = "CyclicBuildConfig"
	MissingImageStreamTagWarning   = "MissingImageStreamTag"
	MissingImageStreamImageWarning = "MissingImageStreamImage"
)

// FindUnpushableBuildConfigs checks all build configs that will output to an IST backed by an ImageStream and checks to make sure their builds can push.
func FindUnpushableBuildConfigs(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
	markers := []osgraph.Marker{}

	// note, unlike with Inputs, ImageStreamImage is not a valid type for build output

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
						Key:      MissingOutputImageStreamErr,
						Message: fmt.Sprintf("%s is pushing to %s, but the image stream for that tag does not exist.",
							f.ResourceName(bcNode), f.ResourceName(istNode)),
					})

					continue
				}

				if len(imageStreamNode.Status.DockerImageRepository) == 0 {
					markers = append(markers, osgraph.Marker{
						Node:         bcNode,
						RelatedNodes: []graph.Node{istNode},

						Severity: osgraph.ErrorSeverity,
						Key:      MissingRequiredRegistryErr,
						Message: fmt.Sprintf("%s is pushing to %s, but the administrator has not configured the integrated Docker registry.",
							f.ResourceName(bcNode), f.ResourceName(istNode)),
						Suggestion: osgraph.Suggestion("oc adm registry -h"),
					})

					continue bc
				}
			}
		}
	}

	return markers
}

// FindMissingInputImageStreams checks all build configs and confirms that their From element exists
//
// Precedence of failures:
// 1. A build config's input points to an image stream that does not exist
// 2. A build config's input uses an image stream tag reference in an existing image stream, but no images within the image stream have that tag assigned
// 3. A build config's input uses an image stream image reference in an exisiting image stream, but no images within the image stream have the supplied image hexadecimal ID
func FindMissingInputImageStreams(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
	markers := []osgraph.Marker{}

	for _, bcNode := range g.NodesByKind(buildgraph.BuildConfigNodeKind) {
		for _, bcInputNode := range g.PredecessorNodesByEdgeKind(bcNode, buildedges.BuildInputImageEdgeKind) {
			switch bcInputNode.(type) {
			case *imagegraph.ImageStreamTagNode:

				for _, uncastImageStreamNode := range g.SuccessorNodesByEdgeKind(bcInputNode, imageedges.ReferencedImageStreamGraphEdgeKind) {
					imageStreamNode := uncastImageStreamNode.(*imagegraph.ImageStreamNode)

					// note, BuildConfig.Spec.BuildSpec.Strategy.[Docker|Source|Custom]Stragegy.From Input of ImageStream has been converted to ImageStreamTag on the vX to api conversion
					// prior to our reaching this point in the code; so there is not need to check for that type vs. ImageStreamTag or ImageStreamImage;

					tagNode, _ := bcInputNode.(*imagegraph.ImageStreamTagNode)
					imageStream := imageStreamNode.Object().(*imageapi.ImageStream)
					if _, ok := imageStream.Status.Tags[tagNode.ImageTag()]; !ok {

						markers = append(markers, osgraph.Marker{
							Node: bcNode,
							RelatedNodes: []graph.Node{bcInputNode,
								imageStreamNode},
							Severity:   osgraph.WarningSeverity,
							Key:        MissingImageStreamTagWarning,
							Message:    fmt.Sprintf("%s builds from %s, but the image stream tag does not exist.", f.ResourceName(bcNode), f.ResourceName(bcInputNode)),
							Suggestion: osgraph.Suggestion(fmt.Sprintf("examine analysis of build config outputs from this command and see if they build %s", f.ResourceName(bcInputNode))),
						})

					}

				}

			case *imagegraph.ImageStreamImageNode:

				for _, uncastImageStreamNode := range g.SuccessorNodesByEdgeKind(bcInputNode, imageedges.ReferencedImageStreamImageGraphEdgeKind) {
					imageStreamNode := uncastImageStreamNode.(*imagegraph.ImageStreamNode)

					imageNode, _ := bcInputNode.(*imagegraph.ImageStreamImageNode)
					imageStream := imageStreamNode.Object().(*imageapi.ImageStream)
					found, imageID, suggestion := validImageStreamImage(imageNode, imageStream)
					if !found {

						markers = append(markers, osgraph.Marker{
							Node: bcNode,
							RelatedNodes: []graph.Node{bcInputNode,
								imageStreamNode},
							Severity:   osgraph.WarningSeverity,
							Key:        MissingImageStreamImageWarning,
							Message:    fmt.Sprintf("%s builds from %s, but the image stream image does not exist.", f.ResourceName(bcNode), f.ResourceName(bcInputNode)),
							Suggestion: osgraph.Suggestion(fmt.Sprintf(suggestion, imageID, f.ResourceName(imageStreamNode))),
						})

					}

				}

			}

		}
	}
	return markers
}

// FindCircularBuilds checks all build configs for cycles
func FindCircularBuilds(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
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
			nodeNames = append(nodeNames, f.ResourceName(node))
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
func FindPendingTags(g osgraph.Graph, f osgraph.Namer) []osgraph.Marker {
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
					Message:    fmt.Sprintf("%s needs to be imported or created by a build.", f.ResourceName(istNode)),
					Suggestion: osgraph.Suggestion(fmt.Sprintf("oc start-build %s", f.ResourceName(bcNode))),
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
					Message:    fmt.Sprintf("%s has failed.", f.ResourceName(latestBuild)),
					Suggestion: osgraph.Suggestion(fmt.Sprintf("Inspect the build failure with 'oc logs %s'", f.ResourceName(latestBuild))),
				})
			default:
				// Do nothing when latest build is new, pending, or running.
			}
		}
	}

	return markers
}

// validImageStreamImage will cycle through the imageStream.Status.Tags.[]TagEvent.DockerImageReference and  determine whether an image with the hexadecimal image id
// associated with an ImageStreamImage reference in fact exists in a given ImageStream; on return, this method returns a true if does exist, and as well as the hexadecimal image
// id from the ImageStreamImage, as well as the appropriate message to add to the marker if the image was not found
func validImageStreamImage(imageNode *imagegraph.ImageStreamImageNode, imageStream *imageapi.ImageStream) (bool, string, string) {
	dockerImageReference, err := imageapi.ParseDockerImageReference(imageNode.Name)
	if err == nil {
		for _, tagEventList := range imageStream.Status.Tags {
			for _, tagEvent := range tagEventList.Items {
				if strings.Contains(tagEvent.DockerImageReference, dockerImageReference.ID) {
					return true, dockerImageReference.ID, ""
				}
			}
		}
	}

	// check the images stream to see if any import images are in flight or have failed
	annotation, ok := imageStream.Annotations[imageapi.DockerImageRepositoryCheckAnnotation]
	if !ok {
		return false, dockerImageReference.ID, "import the image with hexadecimal ID %s into the image stream %s"
	}

	if checkTime, err := time.Parse(time.RFC3339, annotation); err == nil {
		// this time based annotation is set by pkg/image/controller/controller.go whenever import/tag operations are performed; unless
		// in the midst of an import/tag operation, it stays set and serves as a timestamp for when the last operation occurred;
		// so we will check if the image stream has been updated "recently";
		// in case it is a slow link to the remote repo, see if if the check annotation occured within the last 5 minutes; if so, consider that as potentially "in progress"
		compareTime := checkTime.Add(5 * time.Minute)
		currentTime, _ := time.Parse(time.RFC3339, unversioned.Now().UTC().Format(time.RFC3339))
		if compareTime.Before(currentTime) {
			return false, dockerImageReference.ID, "import the image with hexadecimal ID %s into the image stream %s"
		}

		return false, dockerImageReference.ID, "a import of the image with hexadecimal ID %s into the image stream %s could be in progress; check again after a couple of minutes"

	}
	return false, dockerImageReference.ID, "an error occurred importing the image with hexadecimal ID %s into the image stream %s; inspect the images stream annotations for details"
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
