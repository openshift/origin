package nodes

import (
	"github.com/gonum/graph"

	buildv1 "github.com/openshift/api/build/v1"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
)

// EnsureBuildConfigNode adds a graph node for the specific build config if it does not exist
func EnsureBuildConfigNode(g osgraph.MutableUniqueGraph, config *buildv1.BuildConfig) *BuildConfigNode {
	return osgraph.EnsureUnique(
		g,
		BuildConfigNodeName(config),
		func(node osgraph.Node) graph.Node {
			return &BuildConfigNode{
				Node:        node,
				BuildConfig: config,
			}
		},
	).(*BuildConfigNode)
}

// EnsureSourceRepositoryNode adds the specific BuildSource to the graph if it does not already exist.
func EnsureSourceRepositoryNode(g osgraph.MutableUniqueGraph, source buildv1.BuildSource) *SourceRepositoryNode {
	switch {
	case source.Git != nil:
	default:
		return nil
	}
	return osgraph.EnsureUnique(g,
		SourceRepositoryNodeName(source),
		func(node osgraph.Node) graph.Node {
			return &SourceRepositoryNode{node, source}
		},
	).(*SourceRepositoryNode)
}

// EnsureBuildNode adds a graph node for the build if it does not already exist.
func EnsureBuildNode(g osgraph.MutableUniqueGraph, build *buildv1.Build) *BuildNode {
	return osgraph.EnsureUnique(g,
		BuildNodeName(build),
		func(node osgraph.Node) graph.Node {
			return &BuildNode{node, build}
		},
	).(*BuildNode)
}
