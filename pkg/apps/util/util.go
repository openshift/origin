package util

import (
	"fmt"
	"reflect"
	"strconv"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	intstrutil "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	scaleclient "k8s.io/client-go/scale"
	autoscalingv1 "k8s.io/kubernetes/pkg/apis/autoscaling/v1"
	kapiv1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/kubectl"

	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
)

const (
	// DeploymentStatusAnnotation is an annotation name used to retrieve the DeploymentPhase of
	// a deployment.
	// TODO: This is used by CLI, should be moved to library-go
	DeploymentStatusAnnotation = "openshift.io/deployment.phase"

	DeploymentVersionAnnotation = "openshift.io/deployment-config.latest-version"
	// DeploymentConfigAnnotation is an annotation name used to correlate a deployment with the
	DeploymentConfigAnnotation = "openshift.io/deployment-config.name"

	// DesiredReplicasAnnotation represents the desired number of replicas for a
	DesiredReplicasAnnotation = "kubectl.kubernetes.io/desired-replicas"
	// DeployerPodForDeploymentLabel is a label which groups pods related to a
	DeployerPodForDeploymentLabel = "openshift.io/deployer-pod-for.name"

	// DeploymentPodAnnotation is an annotation on a deployment (a ReplicationController). The
	DeploymentPodAnnotation = "openshift.io/deployer-pod.name"

	// deployerPodSuffix is the suffix added to pods created from a deployment
	deployerPodSuffix = "deploy"
)

// DeploymentStatus describes the possible states a deployment can be in.
type DeploymentStatus string

// rcMapper pins preferred version to v1 and scale kind to autoscaling/v1 Scale
// this avoids putting complete server discovery (including extension APIs) in the critical path for deployments
type rcMapper struct{}

func (rcMapper) ResourceFor(gvr schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	if gvr.Group == "" && gvr.Resource == "replicationcontrollers" {
		return kapiv1.SchemeGroupVersion.WithResource("replicationcontrollers"), nil
	}
	return schema.GroupVersionResource{}, fmt.Errorf("unknown replication controller resource: %#v", gvr)
}

func (rcMapper) ScaleForResource(gvr schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	if gvr == kapiv1.SchemeGroupVersion.WithResource("replicationcontrollers") {
		return autoscalingv1.SchemeGroupVersion.WithKind("Scale"), nil
	}
	return schema.GroupVersionKind{}, fmt.Errorf("unknown replication controller resource: %#v", gvr)
}

func NewReplicationControllerScaler(client kubernetes.Interface) kubectl.Scaler {
	return kubectl.NewScaler(NewReplicationControllerScaleClient(client))
}

func NewReplicationControllerScaleClient(client kubernetes.Interface) scaleclient.ScalesGetter {
	return scaleclient.New(client.CoreV1().RESTClient(), rcMapper{}, dynamic.LegacyAPIPathResolverFunc, rcMapper{})
}

// DeployerPodNameForDeployment returns the name of a pod for a given deployment
func DeployerPodNameForDeployment(deployment string) string {
	return apihelpers.GetPodName(deployment, deployerPodSuffix)
}

// LabelForDeployment builds a string identifier for a Deployment.
func LabelForDeployment(deployment *v1.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// DeploymentDesiredReplicas returns number of desired replica for the given replication controller
func DeploymentDesiredReplicas(obj runtime.Object) (int32, bool) {
	return int32AnnotationFor(obj, DesiredReplicasAnnotation)
}

// LatestDeploymentNameForConfigV1 returns a stable identifier for config based on its version.
func LatestDeploymentNameForConfigV1(config *appsv1.DeploymentConfig) string {
	return fmt.Sprintf("%s-%d", config.Name, config.Status.LatestVersion)
}

func DeployerPodNameFor(obj runtime.Object) string {
	return AnnotationFor(obj, DeploymentPodAnnotation)
}

func DeploymentConfigNameFor(obj runtime.Object) string {
	return AnnotationFor(obj, DeploymentConfigAnnotation)
}

// HasSynced checks if the provided deployment config has been noticed by the deployment
// config controller.
func HasSynced(dc *appsv1.DeploymentConfig, generation int64) bool {
	return dc.Status.ObservedGeneration >= generation
}

// IsRollingConfig returns true if the strategy type is a rolling update.
func IsRollingConfig(config *appsv1.DeploymentConfig) bool {
	return config.Spec.Strategy.Type == appsv1.DeploymentStrategyTypeRolling
}

// ResolveFenceposts is copy from k8s deployment_utils to avoid unnecessary imports
func ResolveFenceposts(maxSurge, maxUnavailable *intstrutil.IntOrString, desired int32) (int32, int32, error) {
	surge, err := intstrutil.GetValueFromIntOrPercent(maxSurge, int(desired), true)
	if err != nil {
		return 0, 0, err
	}
	unavailable, err := intstrutil.GetValueFromIntOrPercent(maxUnavailable, int(desired), false)
	if err != nil {
		return 0, 0, err
	}

	if surge == 0 && unavailable == 0 {
		// Validation should never allow the user to explicitly use zero values for both maxSurge
		// maxUnavailable. Due to rounding down maxUnavailable though, it may resolve to zero.
		// If both fenceposts resolve to zero, then we should set maxUnavailable to 1 on the
		// theory that surge might not work due to quota.
		unavailable = 1
	}

	return int32(surge), int32(unavailable), nil
}

// MaxUnavailable returns the maximum unavailable pods a rolling deployment config can take.
func MaxUnavailable(config *appsv1.DeploymentConfig) int32 {
	if !IsRollingConfig(config) {
		return int32(0)
	}
	// Error caught by validation
	_, maxUnavailable, _ := ResolveFenceposts(config.Spec.Strategy.RollingParams.MaxSurge, config.Spec.Strategy.RollingParams.MaxUnavailable, config.Spec.Replicas)
	return maxUnavailable
}

// MaxSurge returns the maximum surge pods a rolling deployment config can take.
func MaxSurge(config appsv1.DeploymentConfig) int32 {
	if !IsRollingConfig(&config) {
		return int32(0)
	}
	// Error caught by validation
	maxSurge, _, _ := ResolveFenceposts(config.Spec.Strategy.RollingParams.MaxSurge, config.Spec.Strategy.RollingParams.MaxUnavailable, config.Spec.Replicas)
	return maxSurge
}

// AnnotationFor returns the annotation with key for obj.
func AnnotationFor(obj runtime.Object, key string) string {
	objectMeta, err := meta.Accessor(obj)
	if err != nil {
		return ""
	}
	if objectMeta == nil || reflect.ValueOf(objectMeta).IsNil() {
		return ""
	}
	return objectMeta.GetAnnotations()[key]
}

// ActiveDeployment returns the latest complete deployment, or nil if there is
// no such deployment. The active deployment is not always the same as the
// latest deployment.
func ActiveDeployment(input []*v1.ReplicationController) *v1.ReplicationController {
	var activeDeployment *v1.ReplicationController
	var lastCompleteDeploymentVersion int64 = 0
	for i := range input {
		deployment := input[i]
		deploymentVersion := DeploymentVersionFor(deployment)
		if IsCompleteDeployment(deployment) && deploymentVersion > lastCompleteDeploymentVersion {
			activeDeployment = deployment
			lastCompleteDeploymentVersion = deploymentVersion
		}
	}
	return activeDeployment
}

// ConfigSelector returns a label Selector which can be used to find all
// deployments for a DeploymentConfig.
//
// TODO: Using the annotation constant for now since the value is correct
// but we could consider adding a new constant to the public types.
func ConfigSelector(name string) labels.Selector {
	return labels.SelectorFromValidatedSet(labels.Set{DeploymentConfigAnnotation: name})
}

// IsCompleteDeployment returns true if the passed deployment is in state complete.
func IsCompleteDeployment(deployment runtime.Object) bool {
	return AnnotationFor(deployment, DeploymentStatusAnnotation) == "Complete"
}

// DeployerPodSelector returns a label Selector which can be used to find all
// deployer pods associated with a deployment with name.
func DeployerPodSelector(name string) labels.Selector {
	return labels.SelectorFromValidatedSet(labels.Set{DeployerPodForDeploymentLabel: name})
}

func DeploymentVersionFor(obj runtime.Object) int64 {
	v, err := strconv.ParseInt(AnnotationFor(obj, DeploymentVersionAnnotation), 10, 64)
	if err != nil {
		return -1
	}
	return v
}

func int32AnnotationFor(obj runtime.Object, key string) (int32, bool) {
	s := AnnotationFor(obj, key)
	if len(s) == 0 {
		return 0, false
	}
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(i), true
}

type ByLatestVersionAsc []*v1.ReplicationController

func (d ByLatestVersionAsc) Len() int      { return len(d) }
func (d ByLatestVersionAsc) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionAsc) Less(i, j int) bool {
	return DeploymentVersionFor(d[i]) < DeploymentVersionFor(d[j])
}

// ByLatestVersionDesc sorts deployments by LatestVersion descending.
type ByLatestVersionDesc []*v1.ReplicationController

func (d ByLatestVersionDesc) Len() int      { return len(d) }
func (d ByLatestVersionDesc) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionDesc) Less(i, j int) bool {
	return DeploymentVersionFor(d[j]) < DeploymentVersionFor(d[i])
}
