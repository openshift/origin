package nodes

import (
	"reflect"

	appsv1 "github.com/openshift/api/apps/v1"
	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
)

var (
	DeploymentConfigNodeKind = reflect.TypeOf(appsv1.DeploymentConfig{}).Name()
)

func DeploymentConfigNodeName(o *appsv1.DeploymentConfig) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(DeploymentConfigNodeKind, o)
}

type DeploymentConfigNode struct {
	osgraph.Node
	DeploymentConfig *appsv1.DeploymentConfig

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
