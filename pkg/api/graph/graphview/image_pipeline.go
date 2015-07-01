package graphview

import (
	"sort"

	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	buildedges "github.com/openshift/origin/pkg/build/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
)

// ImagePipeline represents a build, its output, and any inputs. The input
// to a build may be another ImagePipeline.
type ImagePipeline struct {
	Image ImageTagLocation
	Build *buildgraph.BuildConfigNode
	// If set, the base image used by the build
	BaseImage ImageTagLocation
	// If set, the source repository that inputs to the build
	Source SourceLocation
}

// ImageTagLocation identifies the source or destination of an image. Represents
// both a tag in a Docker image repository, as well as a tag in an OpenShift image stream.
type ImageTagLocation interface {
	ID() int
	ImageSpec() string
	ImageTag() string
}

// SourceLocation identifies a repository that is an input to a build.
type SourceLocation interface {
	ID() int
}

func AllImagePipelinesFromBuildConfig(g osgraph.Graph, excludeNodeIDs IntSet) ([]ImagePipeline, IntSet) {
	covered := IntSet{}
	pipelines := []ImagePipeline{}

	for _, uncastNode := range g.NodesByKind(buildgraph.BuildConfigNodeKind) {
		if excludeNodeIDs.Has(uncastNode.ID()) {
			continue
		}

		pipeline, covers := NewImagePipelineFromBuildConfigNode(g, uncastNode.(*buildgraph.BuildConfigNode))
		covered.Insert(covers.List()...)
		pipelines = append(pipelines, pipeline)
	}

	sort.Sort(SortedImagePipelines(pipelines))
	return pipelines, covered
}

// NewImagePipeline attempts to locate a build flow from the provided node. If no such
// build flow can be located, false is returned.
func NewImagePipelineFromBuildConfigNode(g osgraph.Graph, bcNode *buildgraph.BuildConfigNode) (ImagePipeline, IntSet) {
	covered := IntSet{}
	covered.Insert(bcNode.ID())

	flow := ImagePipeline{}

	base, src, coveredInputs, _ := findBuildInputs(g, bcNode)
	covered.Insert(coveredInputs.List()...)
	flow.Build = bcNode
	flow.BaseImage = base
	flow.Source = src

	return flow, covered
}

// NewImagePipelineFromImageTagLocation returns the ImagePipeline and all the nodes contributing to it
func NewImagePipelineFromImageTagLocation(g osgraph.Graph, node graph.Node, imageTagLocation ImageTagLocation) (ImagePipeline, IntSet) {
	covered := IntSet{}
	covered.Insert(node.ID())

	flow := ImagePipeline{}
	flow.Image = imageTagLocation

	for _, input := range g.PredecessorNodesByEdgeKind(node, buildedges.BuildOutputEdgeKind) {
		covered.Insert(input.ID())
		build := input.(*buildgraph.BuildConfigNode)
		if flow.Build != nil {
			// report this as an error (unexpected duplicate input build)
		}
		if build.BuildConfig == nil {
			// report this as as a missing build / broken link
			break
		}

		base, src, coveredInputs, _ := findBuildInputs(g, build)
		covered.Insert(coveredInputs.List()...)
		flow.Build = build
		flow.BaseImage = base
		flow.Source = src
	}

	return flow, covered
}

func findBuildInputs(g osgraph.Graph, bcNode *buildgraph.BuildConfigNode) (base ImageTagLocation, source SourceLocation, covered IntSet, err error) {
	covered = IntSet{}

	// find inputs to the build
	for _, input := range g.PredecessorNodesByEdgeKind(bcNode, buildedges.BuildInputEdgeKind) {
		if source != nil {
			// report this as an error (unexpected duplicate source)
		}
		covered.Insert(input.ID())
		source = input.(SourceLocation)
	}
	for _, input := range g.PredecessorNodesByEdgeKind(bcNode, buildedges.BuildInputImageEdgeKind) {
		if base != nil {
			// report this as an error (unexpected duplicate input build)
		}
		covered.Insert(input.ID())
		base = input.(ImageTagLocation)
	}

	return
}

type SortedImagePipelines []ImagePipeline

func (m SortedImagePipelines) Len() int      { return len(m) }
func (m SortedImagePipelines) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m SortedImagePipelines) Less(i, j int) bool {
	return CompareImagePipeline(&m[i], &m[j])
}

func CompareImagePipeline(a, b *ImagePipeline) bool {
	switch {
	case a.Build != nil && b.Build != nil:
		return CompareObjectMeta(&a.Build.ObjectMeta, &b.Build.ObjectMeta)
	case a.Build != nil:
		return true
	case b.Build != nil:
		return false
	}
	if a.Image == nil || b.Image == nil {
		return true
	}
	return a.Image.ImageSpec() < b.Image.ImageSpec()
}
