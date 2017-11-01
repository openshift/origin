package nodes

import (
	"reflect"

	kapisext "k8s.io/kubernetes/pkg/apis/extensions"

	osgraph "github.com/openshift/origin/pkg/api/graph"
	deployapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

var (
	DaemonSetNodeKind        = reflect.TypeOf(kapisext.DaemonSet{}).Name()
	DeploymentNodeKind       = reflect.TypeOf(kapisext.Deployment{}).Name()
	DeploymentConfigNodeKind = reflect.TypeOf(deployapi.DeploymentConfig{}).Name()
	ReplicaSetNodeKind       = reflect.TypeOf(kapisext.ReplicaSet{}).Name()
)

func DaemonSetNodeName(o *kapisext.DaemonSet) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(DaemonSetNodeKind, o)
}

type DaemonSetNode struct {
	osgraph.Node
	DaemonSet *kapisext.DaemonSet

	IsFound bool
}

func (n DaemonSetNode) Found() bool {
	return n.IsFound
}

func (n DaemonSetNode) Object() interface{} {
	return n.DaemonSet
}

func (n DaemonSetNode) String() string {
	return string(DaemonSetNodeName(n.DaemonSet))
}

func (*DaemonSetNode) Kind() string {
	return DaemonSetNodeKind
}

func DeploymentNodeName(o *kapisext.Deployment) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(DeploymentNodeKind, o)
}

type DeploymentNode struct {
	osgraph.Node
	Deployment *kapisext.Deployment

	IsFound bool
}

func (n DeploymentNode) Found() bool {
	return n.IsFound
}

func (n DeploymentNode) Object() interface{} {
	return n.Deployment
}

func (n DeploymentNode) String() string {
	return string(DeploymentNodeName(n.Deployment))
}

func (*DeploymentNode) Kind() string {
	return DeploymentNodeKind
}

func DeploymentConfigNodeName(o *deployapi.DeploymentConfig) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(DeploymentConfigNodeKind, o)
}

type DeploymentConfigNode struct {
	osgraph.Node
	DeploymentConfig *deployapi.DeploymentConfig

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

func ReplicaSetNodeName(o *kapisext.ReplicaSet) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ReplicaSetNodeKind, o)
}

type ReplicaSetNode struct {
	osgraph.Node
	ReplicaSet *kapisext.ReplicaSet

	IsFound bool
}

func (n ReplicaSetNode) Found() bool {
	return n.IsFound
}

func (n ReplicaSetNode) Object() interface{} {
	return n.ReplicaSet
}

func (n ReplicaSetNode) String() string {
	return string(ReplicaSetNodeName(n.ReplicaSet))
}

func (*ReplicaSetNode) Kind() string {
	return ReplicaSetNodeKind
}
