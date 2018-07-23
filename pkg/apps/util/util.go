package util

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// DecodeDeploymentConfig decodes a DeploymentConfig from controller using annotation codec.
// An error is returned if the controller doesn't contain an encoded config or decoding fail.
func DecodeDeploymentConfig(controller metav1.ObjectMetaAccessor) (*appsv1.DeploymentConfig, error) {
	encodedConfig, exists := controller.GetObjectMeta().GetAnnotations()[deploymentEncodedConfigAnnotation]
	if !exists {
		return nil, fmt.Errorf("object %s does not have encoded deployment config annotation", controller.GetObjectMeta().GetName())
	}
	config, err := runtime.Decode(annotationDecoder, []byte(encodedConfig))
	if err != nil {
		return nil, err
	}
	externalConfig, ok := config.(*appsv1.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("object %+v is not v1.DeploymentConfig", config)
	}
	return externalConfig, nil
}

// RolloutExceededTimeoutSeconds returns true if the current deployment exceeded
// the timeoutSeconds defined for its strategy.
// Note that this is different than activeDeadlineSeconds which is the timeout
// set for the deployer pod. In some cases, the deployer pod cannot be created
// (like quota, etc...). In that case deployer controller use this function to
// measure if the created deployment (RC) exceeded the timeout.
func RolloutExceededTimeoutSeconds(config *appsv1.DeploymentConfig, latestRC *v1.ReplicationController) bool {
	timeoutSeconds := GetTimeoutSecondsForStrategy(config)
	// If user set the timeoutSeconds to 0, we assume there should be no timeout.
	if timeoutSeconds <= 0 {
		return false
	}
	return int64(time.Since(latestRC.CreationTimestamp.Time).Seconds()) > timeoutSeconds
}

// GetTimeoutSecondsForStrategy returns the timeout in seconds defined in the
// deployment config strategy.
func GetTimeoutSecondsForStrategy(config *appsv1.DeploymentConfig) int64 {
	var timeoutSeconds int64
	switch config.Spec.Strategy.Type {
	case appsv1.DeploymentStrategyTypeRolling:
		timeoutSeconds = DefaultRollingTimeoutSeconds
		if t := config.Spec.Strategy.RollingParams.TimeoutSeconds; t != nil {
			timeoutSeconds = *t
		}
	case appsv1.DeploymentStrategyTypeRecreate:
		timeoutSeconds = DefaultRecreateTimeoutSeconds
		if t := config.Spec.Strategy.RecreateParams.TimeoutSeconds; t != nil {
			timeoutSeconds = *t
		}
	case appsv1.DeploymentStrategyTypeCustom:
		timeoutSeconds = DefaultRecreateTimeoutSeconds
	}
	return timeoutSeconds
}

func NewReplicationControllerScaler(client kubernetes.Interface) kubectl.Scaler {
	return kubectl.NewScaler(NewReplicationControllerScaleClient(client))
}

func NewReplicationControllerScaleClient(client kubernetes.Interface) scaleclient.ScalesGetter {
	return scaleclient.New(client.CoreV1().RESTClient(), rcMapper{}, dynamic.LegacyAPIPathResolverFunc, rcMapper{})
}

// DeployerPodNameForDeployment returns the name of a pod for a given deployment
func DeployerPodNameForDeployment(deployment string) string {
	return apihelpers.GetPodName(deployment, "deploy")
}

// LabelForDeployment builds a string identifier for a Deployment.
func LabelForDeployment(deployment *v1.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// DeploymentDesiredReplicas returns number of desired replica for the given replication controller
func DeploymentDesiredReplicas(obj runtime.Object) (int32, bool) {
	return int32AnnotationFor(obj, DesiredReplicasAnnotation)
}

// LatestDeploymentNameForConfig returns a stable identifier for deployment config
func LatestDeploymentNameForConfig(config *appsv1.DeploymentConfig) string {
	return LatestDeploymentNameForConfigAndVersion(config.Name, config.Status.LatestVersion)
}

// LatestDeploymentNameForConfigAndVersion returns a stable identifier for config based on its version.
func LatestDeploymentNameForConfigAndVersion(name string, version int64) string {
	return fmt.Sprintf("%s-%d", name, version)
}

func DeployerPodNameFor(obj runtime.Object) string {
	return AnnotationFor(obj, DeploymentPodAnnotation)
}

func DeploymentConfigNameFor(obj runtime.Object) string {
	return AnnotationFor(obj, DeploymentConfigAnnotation)
}

func DeploymentStatusReasonFor(obj runtime.Object) string {
	return AnnotationFor(obj, DeploymentStatusReasonAnnotation)
}

func DeleteStatusReasons(rc *v1.ReplicationController) {
	delete(rc.Annotations, DeploymentStatusReasonAnnotation)
	delete(rc.Annotations, deploymentCancelledAnnotation)
}

func SetCancellationReasons(rc *v1.ReplicationController) {
	rc.Annotations[deploymentCancelledAnnotation] = "true"
	rc.Annotations[DeploymentStatusReasonAnnotation] = deploymentCancelledByUser
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
	return DeploymentStatusFor(deployment) == DeploymentStatusComplete
}

// IsFailedDeployment returns true if the passed deployment failed.
func IsFailedDeployment(deployment runtime.Object) bool {
	return DeploymentStatusFor(deployment) == DeploymentStatusFailed
}

// IsTerminatedDeployment returns true if the passed deployment has terminated (either
// complete or failed).
func IsTerminatedDeployment(deployment runtime.Object) bool {
	return IsCompleteDeployment(deployment) || IsFailedDeployment(deployment)
}

func IsDeploymentCancelled(deployment runtime.Object) bool {
	value := AnnotationFor(deployment, deploymentCancelledAnnotation)
	return strings.EqualFold(value, "true")
}

// DeployerPodSelector returns a label Selector which can be used to find all
// deployer pods associated with a deployment with name.
func DeployerPodSelector(name string) labels.Selector {
	return labels.SelectorFromValidatedSet(labels.Set{DeployerPodForDeploymentLabel: name})
}

func DeploymentStatusFor(deployment runtime.Object) DeploymentStatus {
	return DeploymentStatus(AnnotationFor(deployment, DeploymentStatusAnnotation))
}

func DeploymentVersionFor(obj runtime.Object) int64 {
	v, err := strconv.ParseInt(AnnotationFor(obj, deploymentVersionAnnotation), 10, 64)
	if err != nil {
		return -1
	}
	return v
}

func DeploymentNameFor(obj runtime.Object) string {
	return AnnotationFor(obj, DeploymentAnnotation)
}

// GetDeploymentCondition returns the condition with the provided type.
func GetDeploymentCondition(status appsv1.DeploymentConfigStatus, condType appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// GetReplicaCountForDeployments returns the sum of all replicas for the
// given deployments.
func GetReplicaCountForDeployments(deployments []*v1.ReplicationController) int32 {
	totalReplicaCount := int32(0)
	for _, deployment := range deployments {
		count := deployment.Spec.Replicas
		if count == nil {
			continue
		}
		totalReplicaCount += *count
	}
	return totalReplicaCount
}

// GetStatusReplicaCountForDeployments returns the sum of the replicas reported in the
// status of the given deployments.
func GetStatusReplicaCountForDeployments(deployments []*v1.ReplicationController) int32 {
	totalReplicaCount := int32(0)
	for _, deployment := range deployments {
		totalReplicaCount += deployment.Status.Replicas
	}
	return totalReplicaCount
}

// GetReadyReplicaCountForReplicationControllers returns the number of ready pods corresponding to
// the given replication controller.
func GetReadyReplicaCountForReplicationControllers(replicationControllers []*v1.ReplicationController) int32 {
	totalReadyReplicas := int32(0)
	for _, rc := range replicationControllers {
		if rc != nil {
			totalReadyReplicas += rc.Status.ReadyReplicas
		}
	}
	return totalReadyReplicas
}

// GetAvailableReplicaCountForReplicationControllers returns the number of available pods corresponding to
// the given replication controller.
func GetAvailableReplicaCountForReplicationControllers(replicationControllers []*v1.ReplicationController) int32 {
	totalAvailableReplicas := int32(0)
	for _, rc := range replicationControllers {
		if rc != nil {
			totalAvailableReplicas += rc.Status.AvailableReplicas
		}
	}
	return totalAvailableReplicas
}

// HasImageChangeTrigger returns whether the provided deployment configuration has
// an image change trigger or not.
func HasImageChangeTrigger(config *appsv1.DeploymentConfig) bool {
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == appsv1.DeploymentTriggerOnImageChange {
			return true
		}
	}
	return false
}

// CanTransitionPhase returns whether it is allowed to go from the current to the next phase.
func CanTransitionPhase(current, next DeploymentStatus) bool {
	switch current {
	case DeploymentStatusNew:
		switch next {
		case DeploymentStatusPending,
			DeploymentStatusRunning,
			DeploymentStatusFailed,
			DeploymentStatusComplete:
			return true
		}
	case DeploymentStatusPending:
		switch next {
		case DeploymentStatusRunning,
			DeploymentStatusFailed,
			DeploymentStatusComplete:
			return true
		}
	case DeploymentStatusRunning:
		switch next {
		case DeploymentStatusFailed, DeploymentStatusComplete:
			return true
		}
	}
	return false
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
