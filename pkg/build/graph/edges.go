package graph

import (
	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	BuildInputImageEdgeKind = "BuildInputImage"
	BuildOutputEdgeKind     = "BuildOutput"
	BuildInputEdgeKind      = "BuildInput"

	// BuildEdgeKind goes from a BuildConfigNode to a BuildNode and indicates that the buildConfig owns the build
	BuildEdgeKind = "Build"
)

func AddBuildEdges(g osgraph.MutableUniqueGraph, node *buildgraph.BuildConfigNode) {
	for _, n := range g.(graph.Graph).NodeList() {
		if buildNode, ok := n.(*buildgraph.BuildNode); ok {
			if belongsToBuildConfig(node.BuildConfig, buildNode.Build) {
				g.AddEdge(node, buildNode, BuildEdgeKind)
			}
		}
	}
}

func AddAllBuildEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).NodeList() {
		if bcNode, ok := node.(*buildgraph.BuildConfigNode); ok {
			AddBuildEdges(g, bcNode)
		}
	}
}

// AddInputOutputEdges links the build config to other nodes for the images and source repositories it depends on.
func AddInputOutputEdges(g osgraph.MutableUniqueGraph, node *buildgraph.BuildConfigNode) *buildgraph.BuildConfigNode {
	output := node.BuildConfig.Spec.Output
	to := output.To
	switch {
	case to == nil:
	case to.Kind == "DockerImage":
		out := imagegraph.EnsureDockerRepositoryNode(g, to.Name, "")
		g.AddEdge(node, out, BuildOutputEdgeKind)
	case to.Kind == "ImageStreamTag":
		out := imagegraph.FindOrCreateSyntheticImageStreamTagNode(g, imagegraph.MakeImageStreamTagObjectMeta2(defaultNamespace(to.Namespace, node.BuildConfig.Namespace), to.Name))
		g.AddEdge(node, out, BuildOutputEdgeKind)
	}

	if in := buildgraph.EnsureSourceRepositoryNode(g, node.BuildConfig.Spec.Source); in != nil {
		g.AddEdge(in, node, BuildInputEdgeKind)
	}

	from := buildutil.GetImageStreamForStrategy(node.BuildConfig.Spec.Strategy)
	if from != nil {
		switch from.Kind {
		case "DockerImage":
			if ref, err := imageapi.ParseDockerImageReference(from.Name); err == nil {
				tag := ref.Tag
				ref.Tag = ""
				in := imagegraph.EnsureDockerRepositoryNode(g, ref.String(), tag)
				g.AddEdge(in, node, BuildInputImageEdgeKind)
			}
		case "ImageStream":
			in := imagegraph.FindOrCreateSyntheticImageStreamTagNode(g, imagegraph.MakeImageStreamTagObjectMeta(defaultNamespace(from.Namespace, node.BuildConfig.Namespace), from.Name, imageapi.DefaultImageTag))
			g.AddEdge(in, node, BuildInputImageEdgeKind)
		case "ImageStreamTag":
			in := imagegraph.FindOrCreateSyntheticImageStreamTagNode(g, imagegraph.MakeImageStreamTagObjectMeta2(defaultNamespace(from.Namespace, node.BuildConfig.Namespace), from.Name))
			g.AddEdge(in, node, BuildInputImageEdgeKind)
		case "ImageStreamImage":
			in := imagegraph.FindOrCreateSyntheticImageStreamImageNode(g, imagegraph.MakeImageStreamImageObjectMeta(defaultNamespace(from.Namespace, node.BuildConfig.Namespace), from.Name))
			g.AddEdge(in, node, BuildInputImageEdgeKind)
		}
	}
	return node
}

func AddAllInputOutputEdges(g osgraph.MutableUniqueGraph) {
	for _, node := range g.(graph.Graph).NodeList() {
		if bcNode, ok := node.(*buildgraph.BuildConfigNode); ok {
			AddInputOutputEdges(g, bcNode)
		}
	}
}
