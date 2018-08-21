package nodes

import (
	"fmt"
	"reflect"

	buildv1 "github.com/openshift/api/build/v1"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
)

var (
	BuildConfigNodeKind = reflect.TypeOf(buildv1.BuildConfig{}).Name()
	BuildNodeKind       = reflect.TypeOf(buildv1.Build{}).Name()

	// non-api types
	SourceRepositoryNodeKind = reflect.TypeOf(buildv1.BuildSource{}).Name()
)

func BuildConfigNodeName(o *buildv1.BuildConfig) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(BuildConfigNodeKind, o)
}

type BuildConfigNode struct {
	osgraph.Node
	BuildConfig *buildv1.BuildConfig
}

func (n BuildConfigNode) Object() interface{} {
	return n.BuildConfig
}

func (n BuildConfigNode) String() string {
	return string(BuildConfigNodeName(n.BuildConfig))
}

func (n BuildConfigNode) UniqueName() osgraph.UniqueName {
	return BuildConfigNodeName(n.BuildConfig)
}

func (*BuildConfigNode) Kind() string {
	return BuildConfigNodeKind
}

func SourceRepositoryNodeName(source buildv1.BuildSource) osgraph.UniqueName {
	switch {
	case source.Git != nil:
		sourceType, uri, ref := "git", source.Git.URI, source.Git.Ref
		return osgraph.UniqueName(fmt.Sprintf("%s|%s|%s#%s", SourceRepositoryNodeKind, sourceType, uri, ref))
	default:
		panic(fmt.Sprintf("invalid build source: %v", source))
	}
}

type SourceRepositoryNode struct {
	osgraph.Node
	Source buildv1.BuildSource
}

func (n SourceRepositoryNode) String() string {
	return string(SourceRepositoryNodeName(n.Source))
}

func (SourceRepositoryNode) Kind() string {
	return SourceRepositoryNodeKind
}

func BuildNodeName(o *buildv1.Build) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(BuildNodeKind, o)
}

type BuildNode struct {
	osgraph.Node
	Build *buildv1.Build
}

func (n BuildNode) Object() interface{} {
	return n.Build
}

func (n BuildNode) String() string {
	return string(BuildNodeName(n.Build))
}

func (n BuildNode) UniqueName() osgraph.UniqueName {
	return BuildNodeName(n.Build)
}

func (*BuildNode) Kind() string {
	return BuildNodeKind
}
