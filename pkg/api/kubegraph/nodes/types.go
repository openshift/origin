package nodes

import (
	"fmt"
	"reflect"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	osgraph "github.com/openshift/origin/pkg/api/graph"
)

var (
	ServiceNodeKind                   = reflect.TypeOf(kapi.Service{}).Name()
	PodNodeKind                       = reflect.TypeOf(kapi.Pod{}).Name()
	PodSpecNodeKind                   = reflect.TypeOf(kapi.PodSpec{}).Name()
	PodTemplateSpecNodeKind           = reflect.TypeOf(kapi.PodTemplateSpec{}).Name()
	ReplicationControllerNodeKind     = reflect.TypeOf(kapi.ReplicationController{}).Name()
	ReplicationControllerSpecNodeKind = reflect.TypeOf(kapi.ReplicationControllerSpec{}).Name()
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

func (n PodNode) UniqueName() osgraph.UniqueName {
	return PodNodeName(n.Pod)
}

func (*PodNode) Kind() string {
	return PodNodeKind
}

func PodSpecNodeName(o *kapi.PodSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", PodSpecNodeKind, ownerName))
}

type PodSpecNode struct {
	osgraph.Node
	*kapi.PodSpec

	OwnerName osgraph.UniqueName
}

func (n PodSpecNode) Object() interface{} {
	return n.PodSpec
}

func (n PodSpecNode) String() string {
	return string(n.UniqueName())
}

func (n PodSpecNode) UniqueName() osgraph.UniqueName {
	return PodSpecNodeName(n.PodSpec, n.OwnerName)
}

func (*PodSpecNode) Kind() string {
	return PodSpecNodeKind
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

func (n ReplicationControllerNode) UniqueName() osgraph.UniqueName {
	return ReplicationControllerNodeName(n.ReplicationController)
}

func (*ReplicationControllerNode) Kind() string {
	return ReplicationControllerNodeKind
}

func ReplicationControllerSpecNodeName(o *kapi.ReplicationControllerSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", ReplicationControllerSpecNodeKind, ownerName))
}

type ReplicationControllerSpecNode struct {
	osgraph.Node
	*kapi.ReplicationControllerSpec

	OwnerName osgraph.UniqueName
}

func (n ReplicationControllerSpecNode) Object() interface{} {
	return n.ReplicationControllerSpec
}

func (n ReplicationControllerSpecNode) String() string {
	return string(n.UniqueName())
}

func (n ReplicationControllerSpecNode) UniqueName() osgraph.UniqueName {
	return ReplicationControllerSpecNodeName(n.ReplicationControllerSpec, n.OwnerName)
}

func (*ReplicationControllerSpecNode) Kind() string {
	return ReplicationControllerSpecNodeKind
}

func PodTemplateSpecNodeName(o *kapi.PodTemplateSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", PodTemplateSpecNodeKind, ownerName))
}

type PodTemplateSpecNode struct {
	osgraph.Node
	*kapi.PodTemplateSpec

	OwnerName osgraph.UniqueName
}

func (n PodTemplateSpecNode) Object() interface{} {
	return n.PodTemplateSpec
}

func (n PodTemplateSpecNode) String() string {
	return string(n.UniqueName())
}

func (n PodTemplateSpecNode) UniqueName() osgraph.UniqueName {
	return PodTemplateSpecNodeName(n.PodTemplateSpec, n.OwnerName)
}

func (*PodTemplateSpecNode) Kind() string {
	return PodTemplateSpecNodeKind
}
