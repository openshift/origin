package nodes

import (
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

var (
	DeploymentConfigNodeKind = reflect.TypeOf(deployapi.DeploymentConfig{}).Name()
)

func DeploymentConfigNodeName(o *deployapi.DeploymentConfig) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(DeploymentConfigNodeKind, o)
}

type DeploymentConfigNode struct {
	osgraph.Node
	*deployapi.DeploymentConfig

	ActiveDeployment *kapi.ReplicationController
	Deployments      []*kapi.ReplicationController
}

func (n DeploymentConfigNode) Object() interface{} {
	return n.DeploymentConfig
}

func (n DeploymentConfigNode) String() string {
	return fmt.Sprintf("<deploymentconfig %s/%s>", n.Namespace, n.Name)
}

func (*DeploymentConfigNode) Kind() string {
	return DeploymentConfigNodeKind
}
