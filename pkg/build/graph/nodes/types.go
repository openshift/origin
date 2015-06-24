package nodes

import (
	"fmt"
	"reflect"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

var (
	BuildConfigNodeKind = reflect.TypeOf(buildapi.BuildConfig{}).Name()
	BuildNodeKind       = reflect.TypeOf(buildapi.Build{}).Name()

	// non-api types
	SourceRepositoryNodeKind = reflect.TypeOf(buildapi.BuildSource{}).Name()
)

func BuildConfigNodeName(o *buildapi.BuildConfig) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(BuildConfigNodeKind, o)
}

type BuildConfigNode struct {
	osgraph.Node
	*buildapi.BuildConfig

	LastSuccessfulBuild   *buildapi.Build
	LastUnsuccessfulBuild *buildapi.Build
	ActiveBuilds          []buildapi.Build
}

func (n BuildConfigNode) Object() interface{} {
	return n.BuildConfig
}

func (n BuildConfigNode) String() string {
	return fmt.Sprintf("<buildconfig %s/%s>", n.Namespace, n.Name)
}

func (*BuildConfigNode) Kind() string {
	return BuildConfigNodeKind
}

func SourceRepositoryNodeName(source buildapi.BuildSource) osgraph.UniqueName {
	switch {
	case source.Git != nil:
		sourceType, uri, ref := "git", source.Git.URI, source.Git.Ref
		return osgraph.UniqueName(fmt.Sprintf("%s|%s|%s#%s", SourceRepositoryNodeKind, sourceType, uri, ref))
	default:
		panic(fmt.Sprintf("invalid build source", source))
	}
}

type SourceRepositoryNode struct {
	osgraph.Node
	Source buildapi.BuildSource
}

func (n SourceRepositoryNode) String() string {
	if n.Source.Git != nil {
		return fmt.Sprintf("<sourcerepository %s#%s>", n.Source.Git.URI, n.Source.Git.Ref)
	}
	return fmt.Sprintf("<source repository unknown>")
}

func (SourceRepositoryNode) Kind() string {
	return SourceRepositoryNodeKind
}

func BuildNodeName(o *buildapi.Build) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(BuildNodeKind, o)
}

type BuildNode struct {
	osgraph.Node
	Build *buildapi.Build
}

func (n BuildNode) Object() interface{} {
	return n.Build
}

func (n BuildNode) String() string {
	return fmt.Sprintf("<build %s/%s>", n.Build.Namespace, n.Build.Name)
}

func (*BuildNode) Kind() string {
	return BuildNodeKind
}
