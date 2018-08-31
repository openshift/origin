package nodes

import (
	"fmt"
	"reflect"

	kappsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"

	osgraph "github.com/openshift/origin/pkg/oc/lib/graph/genericgraph"
)

var (
	ServiceNodeKind                   = reflect.TypeOf(corev1.Service{}).Name()
	PodNodeKind                       = reflect.TypeOf(corev1.Pod{}).Name()
	PodSpecNodeKind                   = reflect.TypeOf(corev1.PodSpec{}).Name()
	PodTemplateSpecNodeKind           = reflect.TypeOf(corev1.PodTemplateSpec{}).Name()
	ReplicationControllerNodeKind     = reflect.TypeOf(corev1.ReplicationController{}).Name()
	ReplicationControllerSpecNodeKind = reflect.TypeOf(corev1.ReplicationControllerSpec{}).Name()
	ServiceAccountNodeKind            = reflect.TypeOf(corev1.ServiceAccount{}).Name()
	SecretNodeKind                    = reflect.TypeOf(corev1.Secret{}).Name()
	PersistentVolumeClaimNodeKind     = reflect.TypeOf(corev1.PersistentVolumeClaim{}).Name()
	HorizontalPodAutoscalerNodeKind   = reflect.TypeOf(autoscalingv1.HorizontalPodAutoscaler{}).Name()
	StatefulSetNodeKind               = reflect.TypeOf(kappsv1.StatefulSet{}).Name()
	StatefulSetSpecNodeKind           = reflect.TypeOf(kappsv1.StatefulSetSpec{}).Name()
	DeploymentNodeKind                = reflect.TypeOf(kappsv1.Deployment{}).Name()
	DeploymentSpecNodeKind            = reflect.TypeOf(kappsv1.DeploymentSpec{}).Name()
	ReplicaSetNodeKind                = reflect.TypeOf(kappsv1.ReplicaSet{}).Name()
	ReplicaSetSpecNodeKind            = reflect.TypeOf(kappsv1.ReplicaSetSpec{}).Name()
	DaemonSetNodeKind                 = reflect.TypeOf(kappsv1.DaemonSet{}).Name()
)

func ServiceNodeName(o *corev1.Service) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ServiceNodeKind, o)
}

type ServiceNode struct {
	osgraph.Node
	*corev1.Service

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

func PodNodeName(o *corev1.Pod) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(PodNodeKind, o)
}

type PodNode struct {
	osgraph.Node
	*corev1.Pod
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

func PodSpecNodeName(o *corev1.PodSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", PodSpecNodeKind, ownerName))
}

type PodSpecNode struct {
	osgraph.Node
	*corev1.PodSpec
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

func ReplicaSetNodeName(o *kappsv1.ReplicaSet) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ReplicaSetNodeKind, o)
}

type ReplicaSetNode struct {
	osgraph.Node
	ReplicaSet *kappsv1.ReplicaSet

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

func ReplicaSetSpecNodeName(o *kappsv1.ReplicaSetSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", ReplicaSetSpecNodeKind, ownerName))
}

type ReplicaSetSpecNode struct {
	osgraph.Node
	ReplicaSetSpec *kappsv1.ReplicaSetSpec
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

func ReplicationControllerNodeName(o *corev1.ReplicationController) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ReplicationControllerNodeKind, o)
}

type ReplicationControllerNode struct {
	osgraph.Node
	ReplicationController *corev1.ReplicationController

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

func ReplicationControllerSpecNodeName(o *corev1.ReplicationControllerSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", ReplicationControllerSpecNodeKind, ownerName))
}

type ReplicationControllerSpecNode struct {
	osgraph.Node
	ReplicationControllerSpec *corev1.ReplicationControllerSpec
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

func PodTemplateSpecNodeName(o *corev1.PodTemplateSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", PodTemplateSpecNodeKind, ownerName))
}

type PodTemplateSpecNode struct {
	osgraph.Node
	*corev1.PodTemplateSpec
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

func ServiceAccountNodeName(o *corev1.ServiceAccount) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(ServiceAccountNodeKind, o)
}

type ServiceAccountNode struct {
	osgraph.Node
	*corev1.ServiceAccount

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

func SecretNodeName(o *corev1.Secret) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(SecretNodeKind, o)
}

type SecretNode struct {
	osgraph.Node
	*corev1.Secret

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

func PersistentVolumeClaimNodeName(o *corev1.PersistentVolumeClaim) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(PersistentVolumeClaimNodeKind, o)
}

type PersistentVolumeClaimNode struct {
	osgraph.Node
	PersistentVolumeClaim *corev1.PersistentVolumeClaim

	IsFound bool
}

func (n PersistentVolumeClaimNode) Found() bool {
	return n.IsFound
}

func (n PersistentVolumeClaimNode) Object() interface{} {
	return n.PersistentVolumeClaim
}

func (n PersistentVolumeClaimNode) String() string {
	return string(n.UniqueName())
}

func (*PersistentVolumeClaimNode) Kind() string {
	return PersistentVolumeClaimNodeKind
}

func (n PersistentVolumeClaimNode) UniqueName() osgraph.UniqueName {
	return PersistentVolumeClaimNodeName(n.PersistentVolumeClaim)
}

func HorizontalPodAutoscalerNodeName(o *autoscalingv1.HorizontalPodAutoscaler) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(HorizontalPodAutoscalerNodeKind, o)
}

type HorizontalPodAutoscalerNode struct {
	osgraph.Node
	HorizontalPodAutoscaler *autoscalingv1.HorizontalPodAutoscaler
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

func DeploymentNodeName(o *kappsv1.Deployment) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(DeploymentNodeKind, o)
}

type DeploymentNode struct {
	osgraph.Node
	Deployment *kappsv1.Deployment

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

func DeploymentSpecNodeName(o *kappsv1.DeploymentSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", DeploymentSpecNodeKind, ownerName))
}

type DeploymentSpecNode struct {
	osgraph.Node
	DeploymentSpec *kappsv1.DeploymentSpec
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

func StatefulSetNodeName(o *kappsv1.StatefulSet) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(StatefulSetNodeKind, o)
}

type StatefulSetNode struct {
	osgraph.Node
	StatefulSet *kappsv1.StatefulSet

	IsFound bool
}

func (n StatefulSetNode) Found() bool {
	return n.IsFound
}

func (n StatefulSetNode) Object() interface{} {
	return n.StatefulSet
}

func (n StatefulSetNode) String() string {
	return string(n.UniqueName())
}

func (n StatefulSetNode) UniqueName() osgraph.UniqueName {
	return StatefulSetNodeName(n.StatefulSet)
}

func (*StatefulSetNode) Kind() string {
	return StatefulSetNodeKind
}

func StatefulSetSpecNodeName(o *kappsv1.StatefulSetSpec, ownerName osgraph.UniqueName) osgraph.UniqueName {
	return osgraph.UniqueName(fmt.Sprintf("%s|%v", StatefulSetSpecNodeKind, ownerName))
}

type StatefulSetSpecNode struct {
	osgraph.Node
	StatefulSetSpec *kappsv1.StatefulSetSpec
	Namespace       string

	OwnerName osgraph.UniqueName
}

func (n StatefulSetSpecNode) Object() interface{} {
	return n.StatefulSetSpec
}

func (n StatefulSetSpecNode) String() string {
	return string(n.UniqueName())
}

func (n StatefulSetSpecNode) UniqueName() osgraph.UniqueName {
	return StatefulSetSpecNodeName(n.StatefulSetSpec, n.OwnerName)
}

func (*StatefulSetSpecNode) Kind() string {
	return StatefulSetSpecNodeKind
}

func DaemonSetNodeName(o *kappsv1.DaemonSet) osgraph.UniqueName {
	return osgraph.GetUniqueRuntimeObjectNodeName(DaemonSetNodeKind, o)
}

type DaemonSetNode struct {
	osgraph.Node
	DaemonSet *kappsv1.DaemonSet

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
