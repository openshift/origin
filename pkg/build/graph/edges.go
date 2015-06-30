package graph

import (
	"sort"

	"github.com/golang/glog"
	"github.com/gonum/graph"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildgraph "github.com/openshift/origin/pkg/build/graph/nodes"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagegraph "github.com/openshift/origin/pkg/image/graph/nodes"
)

const (
	BuildInputImageEdgeKind = "BuildInputImage"
	BuildOutputEdgeKind     = "BuildOutput"
	BuildInputEdgeKind      = "BuildInput"
)

// AddInputOutputEdges links the build config to other nodes for the images and source repositories it depends on.
func AddInputOutputEdges(g osgraph.MutableUniqueGraph, node *buildgraph.BuildConfigNode) *buildgraph.BuildConfigNode {
	output := node.BuildConfig.Parameters.Output
	to := output.To
	switch {
	case to != nil && len(to.Name) > 0:
		out := imagegraph.EnsureImageStreamTagNode(g, defaultNamespace(to.Namespace, node.BuildConfig.Namespace), to.Name, output.Tag)
		g.AddEdge(node, out, BuildOutputEdgeKind)
	case len(output.DockerImageReference) > 0:
		out := imagegraph.EnsureDockerRepositoryNode(g, output.DockerImageReference, output.Tag)
		g.AddEdge(node, out, BuildOutputEdgeKind)
	}

	if in := buildgraph.EnsureSourceRepositoryNode(g, node.BuildConfig.Parameters.Source); in != nil {
		g.AddEdge(in, node, BuildInputEdgeKind)
	}

	from := buildutil.GetImageStreamForStrategy(node.BuildConfig.Parameters.Strategy)
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
			tag := imageapi.DefaultImageTag
			in := imagegraph.EnsureImageStreamTagNode(g, defaultNamespace(from.Namespace, node.BuildConfig.Namespace), from.Name, tag)
			g.AddEdge(in, node, BuildInputImageEdgeKind)
		case "ImageStreamTag":
			name, tag, _ := imageapi.SplitImageStreamTag(from.Name)
			in := imagegraph.EnsureImageStreamTagNode(g, defaultNamespace(from.Namespace, node.BuildConfig.Namespace), name, tag)
			g.AddEdge(in, node, BuildInputImageEdgeKind)
		case "ImageStreamImage":
			glog.V(4).Infof("Ignoring ImageStreamImage reference in BuildConfig %s/%s", node.BuildConfig.Namespace, node.BuildConfig.Name)
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

// TODO kill this.  It should be based on an edge traversal to loaded builds
func JoinBuilds(node *buildgraph.BuildConfigNode, builds []buildapi.Build) {
	matches := []*buildapi.Build{}
	for i := range builds {
		if belongsToBuildConfig(node.BuildConfig, &builds[i]) {
			matches = append(matches, &builds[i])
		}
	}
	if len(matches) == 0 {
		return
	}
	sort.Sort(RecentBuildReferences(matches))
	for i := range matches {
		switch matches[i].Status {
		case buildapi.BuildStatusComplete:
			if node.LastSuccessfulBuild == nil {
				node.LastSuccessfulBuild = matches[i]
			}
		case buildapi.BuildStatusFailed, buildapi.BuildStatusCancelled, buildapi.BuildStatusError:
			if node.LastUnsuccessfulBuild == nil {
				node.LastUnsuccessfulBuild = matches[i]
			}
		default:
			node.ActiveBuilds = append(node.ActiveBuilds, *matches[i])
		}
	}
}

func belongsToBuildConfig(config *buildapi.BuildConfig, b *buildapi.Build) bool {
	if b.Labels == nil {
		return false
	}
	if b.Labels[buildapi.BuildConfigLabel] == config.Name {
		return true
	}
	return false
}

type RecentBuildReferences []*buildapi.Build

func (m RecentBuildReferences) Len() int      { return len(m) }
func (m RecentBuildReferences) Swap(i, j int) { m[i], m[j] = m[j], m[i] }
func (m RecentBuildReferences) Less(i, j int) bool {
	return m[i].CreationTimestamp.After(m[j].CreationTimestamp.Time)
}

func defaultNamespace(value, defaultValue string) string {
	if len(value) == 0 {
		return defaultValue
	}
	return value
}
