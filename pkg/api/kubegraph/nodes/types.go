package nodes

import (
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
)

var (
	ServiceNodeKind               = reflect.TypeOf(kapi.Service{}).Name()
	PodNodeKind                   = reflect.TypeOf(kapi.Pod{}).Name()
	ReplicationControllerNodeKind = reflect.TypeOf(kapi.ReplicationController{}).Name()
)

func ServiceNodeName(o *kapi.Service) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ServiceNodeKind, o)
}

type ServiceNode struct {
	osgraph.Node
	*kapi.Service
}

func (n ServiceNode) Object() interface{} {
	return n.Service
}

func (n ServiceNode) String() string {
	return fmt.Sprintf("<service %s/%s>", n.Namespace, n.Name)
}

func (*ServiceNode) Kind() string {
	return ServiceNodeKind
}

func PodNodeName(o *kapi.Pod) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(PodNodeKind, o)
}

type PodNode struct {
	osgraph.Node
	*kapi.Pod
}

func (n PodNode) Object() interface{} {
	return n.Pod
}

func (n PodNode) String() string {
	return fmt.Sprintf("<pod %s/%s>", n.Namespace, n.Name)
}

func (*PodNode) Kind() string {
	return PodNodeKind
}

func ReplicationControllerNodeName(o *kapi.ReplicationController) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ReplicationControllerNodeKind, o)
}

type ReplicationControllerNode struct {
	osgraph.Node
	*kapi.ReplicationController
}

func (n ReplicationControllerNode) Object() interface{} {
	return n.ReplicationController
}

func (n ReplicationControllerNode) String() string {
	return fmt.Sprintf("<replicationcontroller %s/%s>", n.Namespace, n.Name)
}

func (*ReplicationControllerNode) Kind() string {
	return ReplicationControllerNodeKind
}
