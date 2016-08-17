package nodes

import (
	"fmt"
	"reflect"

	kapi "k8s.io/kubernetes/pkg/api"
	kapps "k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/autoscaling"
	"k8s.io/kubernetes/pkg/apis/extensions"

	osgraph "github.com/openshift/origin/pkg/api/graph"
)

var (
	ServiceNodeKind                   = reflect.TypeOf(kapi.Service{}).Name()
	PodNodeKind                       = reflect.TypeOf(kapi.Pod{}).Name()
	PodSpecNodeKind                   = reflect.TypeOf(kapi.PodSpec{}).Name()
	PodTemplateSpecNodeKind           = reflect.TypeOf(kapi.PodTemplateSpec{}).Name()
	ReplicationControllerNodeKind     = reflect.TypeOf(kapi.ReplicationController{}).Name()
	ReplicationControllerSpecNodeKind = reflect.TypeOf(kapi.ReplicationControllerSpec{}).Name()
	ServiceAccountNodeKind            = reflect.TypeOf(kapi.ServiceAccount{}).Name()
	SecretNodeKind                    = reflect.TypeOf(kapi.Secret{}).Name()
	HorizontalPodAutoscalerNodeKind   = reflect.TypeOf(autoscaling.HorizontalPodAutoscaler{}).Name()
	PetSetNodeKind                    = reflect.TypeOf(kapps.PetSet{}).Name()
	PetSetSpecNodeKind                = reflect.TypeOf(kapps.PetSetSpec{}).Name()
	ReplicaSetNodeKind                = reflect.TypeOf(extensions.ReplicaSet{}).Name()
	ReplicaSetSpecNodeKind            = reflect.TypeOf(extensions.ReplicaSetSpec{}).Name()
	DeploymentNodeKind                = reflect.TypeOf(extensions.Deployment{}).Name()
	DeploymentSpecNodeKind            = reflect.TypeOf(extensions.DeploymentSpec{}).Name()
)

func ServiceNodeName(o *kapi.Service) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ServiceNodeKind, o)
}

type ServiceNode struct {
	osgraph.Node
	*kapi.Service

	IsFound bool
}

func (n ServiceNode) Object() interface{} {
	return n.Service
}

func (n ServiceNode) String() string {
	return string(ServiceNodeName(n.Service))
}

func (*ServiceNode) Kind() string {
	return ServiceNodeKind
}

func (n ServiceNode) Found() bool {
	return n.IsFound
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
	return string(PodNodeName(n.Pod))
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
	Namespace string

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
	ReplicationController *kapi.ReplicationController

	IsFound bool
}

func (n ReplicationControllerNode) Found() bool {
	return n.IsFound
}

func (n ReplicationControllerNode) Object() interface{} {
	return n.ReplicationController
}

func (n ReplicationControllerNode) String() string {
	return string(ReplicationControllerNodeName(n.ReplicationController))
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
	ReplicationControllerSpec *kapi.ReplicationControllerSpec
	Namespace                 string

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
	Namespace string

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

func ServiceAccountNodeName(o *kapi.ServiceAccount) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ServiceAccountNodeKind, o)
}

type ServiceAccountNode struct {
	osgraph.Node
	*kapi.ServiceAccount

	IsFound bool
}

func (n ServiceAccountNode) Found() bool {
	return n.IsFound
}

func (n ServiceAccountNode) Object() interface{} {
	return n.ServiceAccount
}

func (n ServiceAccountNode) String() string {
	return string(ServiceAccountNodeName(n.ServiceAccount))
}

func (*ServiceAccountNode) Kind() string {
	return ServiceAccountNodeKind
}

func SecretNodeName(o *kapi.Secret) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(SecretNodeKind, o)
}

type SecretNode struct {
	osgraph.Node
	*kapi.Secret

	IsFound bool
}

func (n SecretNode) Found() bool {
	return n.IsFound
}

func (n SecretNode) Object() interface{} {
	return n.Secret
}

func (n SecretNode) String() string {
	return string(SecretNodeName(n.Secret))
}

func (*SecretNode) Kind() string {
	return SecretNodeKind
}

func HorizontalPodAutoscalerNodeName(o *autoscaling.HorizontalPodAutoscaler) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(HorizontalPodAutoscalerNodeKind, o)
}

type HorizontalPodAutoscalerNode struct {
	osgraph.Node
	HorizontalPodAutoscaler *autoscaling.HorizontalPodAutoscaler
}

func (n HorizontalPodAutoscalerNode) Object() interface{} {
	return n.HorizontalPodAutoscaler
}

func (n HorizontalPodAutoscalerNode) String() string {
	return string(n.UniqueName())
}

func (*HorizontalPodAutoscalerNode) Kind() string {
	return HorizontalPodAutoscalerNodeKind
}

func (n HorizontalPodAutoscalerNode) UniqueName() osgraph.UniqueName {
	return HorizontalPodAutoscalerNodeName(n.HorizontalPodAutoscaler)
}

func PetSetNodeName(o *kapps.PetSet) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(PetSetNodeKind, o)
}

type PetSetNode struct {
	osgraph.Node
	PetSet *kapps.PetSet
}

func (n PetSetNode) Object() interface{} {
	return n.PetSet
}

func (n PetSetNode) String() string {
	return string(n.UniqueName())
}

func (n PetSetNode) UniqueName() osgraph.UniqueName {
	return PetSetNodeName(n.PetSet)
}

func (*PetSetNode) Kind() string {
	return PetSetNodeKind
}

func PetSetSpecNodeName(o *kapps.PetSetSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", PetSetSpecNodeKind, ownerName))
}

type PetSetSpecNode struct {
	osgraph.Node
	PetSetSpec *kapps.PetSetSpec
	Namespace  string

	OwnerName osgraph.UniqueName
}

func (n PetSetSpecNode) Object() interface{} {
	return n.PetSetSpec
}

func (n PetSetSpecNode) String() string {
	return string(n.UniqueName())
}

func (n PetSetSpecNode) UniqueName() osgraph.UniqueName {
	return PetSetSpecNodeName(n.PetSetSpec, n.OwnerName)
}

func (*PetSetSpecNode) Kind() string {
	return PetSetSpecNodeKind
}

type DeploymentNode struct {
	osgraph.Node
	Deployment *extensions.Deployment

	IsFound bool
}

func (n DeploymentNode) Found() bool {
	return n.IsFound
}

func (n DeploymentNode) Object() interface{} {
	return n.Deployment
}

func (n DeploymentNode) String() string {
	return string(n.UniqueName())
}

func (n DeploymentNode) UniqueName() osgraph.UniqueName {
	return DeploymentNodeName(n.Deployment)
}

func (*DeploymentNode) Kind() string {
	return DeploymentNodeKind
}

func DeploymentNodeName(o *extensions.Deployment) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(DeploymentNodeKind, o)
}

type DeploymentSpecNode struct {
	osgraph.Node
	DeploymentSpec *extensions.DeploymentSpec
	Namespace      string

	OwnerName osgraph.UniqueName
}

func (n DeploymentSpecNode) Object() interface{} {
	return n.DeploymentSpec
}

func (n DeploymentSpecNode) String() string {
	return string(n.UniqueName())
}

func (n DeploymentSpecNode) UniqueName() osgraph.UniqueName {
	return DeploymentSpecNodeName(n.DeploymentSpec, n.OwnerName)
}

func (*DeploymentSpecNode) Kind() string {
	return DeploymentSpecNodeKind
}

func DeploymentSpecNodeName(o *extensions.DeploymentSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", DeploymentSpecNodeKind, ownerName))
}

func ReplicaSetNodeName(o *extensions.ReplicaSet) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ReplicaSetNodeKind, o)
}

type ReplicaSetNode struct {
	osgraph.Node
	ReplicaSet *extensions.ReplicaSet

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

func (n ReplicaSetNode) UniqueName() osgraph.UniqueName {
	return ReplicaSetNodeName(n.ReplicaSet)
}

func (*ReplicaSetNode) Kind() string {
	return ReplicaSetNodeKind
}

func ReplicaSetSpecNodeName(o *extensions.ReplicaSetSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", ReplicaSetSpecNodeKind, ownerName))
}

type ReplicaSetSpecNode struct {
	osgraph.Node
	ReplicaSetSpec *extensions.ReplicaSetSpec
	Namespace      string

	OwnerName osgraph.UniqueName
}

func (n ReplicaSetSpecNode) Object() interface{} {
	return n.ReplicaSetSpec
}

func (n ReplicaSetSpecNode) String() string {
	return string(n.UniqueName())
}

func (n ReplicaSetSpecNode) UniqueName() osgraph.UniqueName {
	return ReplicaSetSpecNodeName(n.ReplicaSetSpec, n.OwnerName)
}

func (*ReplicaSetSpecNode) Kind() string {
	return ReplicaSetSpecNodeKind
}
