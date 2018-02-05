package nodes

import (
	"reflect"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	osgraph "github.com/openshift/origin/pkg/oc/graph/genericgraph"
)

var (
	DeploymentConfigNodeKind = reflect.TypeOf(appsapi.DeploymentConfig{}).Name()
)

func DeploymentConfigNodeName(o *appsapi.DeploymentConfig) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(DeploymentConfigNodeKind, o)
}

type DeploymentConfigNode struct {
	osgraph.Node
	DeploymentConfig *appsapi.DeploymentConfig

	IsFound bool
}

func (n DeploymentConfigNode) Found() bool {
	return n.IsFound
}

func (n DeploymentConfigNode) Object() interface{} {
	return n.DeploymentConfig
}

func (n DeploymentConfigNode) String() string {
	return string(DeploymentConfigNodeName(n.DeploymentConfig))
}

func (*DeploymentConfigNode) Kind() string {
	return DeploymentConfigNodeKind
}
