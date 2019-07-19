package appsutil

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	intstrutil "k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"

	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/library-go/pkg/apps/appsserialization"
	"github.com/openshift/library-go/pkg/build/naming"
)

// DeployerPodNameForDeployment returns the name of a pod for a given deployment
func DeployerPodNameForDeployment(deployment string) string {
	return naming.GetPodName(deployment, "deploy")
}

// WaitForRunningDeployerPod waits a given period of time until the deployer pod
// for given replication controller is not running.
func WaitForRunningDeployerPod(podClient corev1client.PodsGetter, rc *corev1.ReplicationController, timeout time.Duration) error {
	podName := DeployerPodNameForDeployment(rc.Name)
	canGetLogs := func(p *corev1.Pod) bool {
		return corev1.PodSucceeded == p.Status.Phase || corev1.PodFailed == p.Status.Phase || corev1.PodRunning == p.Status.Phase
	}

	fieldSelector := fields.OneTermEqualSelector("metadata.name", podName).String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fieldSelector
			return podClient.Pods(rc.Namespace).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fieldSelector
			return podClient.Pods(rc.Namespace).Watch(options)
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := watchtools.UntilWithSync(ctx, lw, &corev1.Pod{}, nil, func(e watch.Event) (bool, error) {
		switch e.Type {
		case watch.Added, watch.Modified:
			newPod, ok := e.Object.(*corev1.Pod)
			if !ok {
				return true, fmt.Errorf("unknown event object %#v", e.Object)
			}

			return canGetLogs(newPod), nil

		case watch.Deleted:
			return true, fmt.Errorf("pod got deleted %#v", e.Object)

		case watch.Error:
			return true, fmt.Errorf("encountered error while watching for pod: %v", e.Object)

		default:
			return true, fmt.Errorf("unexpected event type: %T", e.Type)
		}
	})
	return err
}

func newControllerRef(config *appsv1.DeploymentConfig) *metav1.OwnerReference {
	deploymentConfigControllerRefKind := appsv1.GroupVersion.WithKind("DeploymentConfig")
	blockOwnerDeletion := true
	isController := true
	return &metav1.OwnerReference{
		APIVersion:         deploymentConfigControllerRefKind.GroupVersion().String(),
		Kind:               deploymentConfigControllerRefKind.Kind,
		Name:               config.Name,
		UID:                config.UID,
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
}

// MakeDeployment creates a deployment represented as a ReplicationController and based on the given DeploymentConfig.
// The controller replica count will be zero.
func MakeDeployment(config *appsv1.DeploymentConfig) (*v1.ReplicationController, error) {
	// EncodeDeploymentConfig encodes config as a string using codec.
	encodedConfig, err := appsserialization.EncodeDeploymentConfig(config)
	if err != nil {
		return nil, err
	}

	deploymentName := LatestDeploymentNameForConfig(config)
	podSpec := config.Spec.Template.Spec.DeepCopy()

	// Fix trailing and leading whitespace in the image field
	// This is needed to sanitize old deployment configs where spaces were permitted but
	// kubernetes 3.7 (#47491) tightened the validation of container image fields.
	for i := range podSpec.Containers {
		podSpec.Containers[i].Image = strings.TrimSpace(podSpec.Containers[i].Image)
	}

	controllerLabels := make(labels.Set)
	for k, v := range config.Labels {
		controllerLabels[k] = v
	}
	// Correlate the deployment with the config.
	// TODO: Using the annotation constant for now since the value is correct
	// but we could consider adding a new constant to the public types.
	controllerLabels[appsv1.DeploymentConfigAnnotation] = config.Name

	// Ensure that pods created by this deployment controller can be safely associated back
	// to the controller, and that multiple deployment controllers for the same config don't
	// manipulate each others' pods.
	selector := map[string]string{}
	for k, v := range config.Spec.Selector {
		selector[k] = v
	}
	selector[DeploymentConfigLabel] = config.Name
	selector[DeploymentLabel] = deploymentName

	podLabels := make(labels.Set)
	for k, v := range config.Spec.Template.Labels {
		podLabels[k] = v
	}
	podLabels[DeploymentConfigLabel] = config.Name
	podLabels[DeploymentLabel] = deploymentName

	podAnnotations := make(labels.Set)
	for k, v := range config.Spec.Template.Annotations {
		podAnnotations[k] = v
	}
	podAnnotations[appsv1.DeploymentAnnotation] = deploymentName
	podAnnotations[appsv1.DeploymentConfigAnnotation] = config.Name
	podAnnotations[appsv1.DeploymentVersionAnnotation] = strconv.FormatInt(config.Status.LatestVersion, 10)

	controllerRef := newControllerRef(config)
	zero := int32(0)
	deployment := &v1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: config.Namespace,
			Annotations: map[string]string{
				appsv1.DeploymentConfigAnnotation:        config.Name,
				appsv1.DeploymentEncodedConfigAnnotation: string(encodedConfig),
				appsv1.DeploymentStatusAnnotation:        string(appsv1.DeploymentStatusNew),
				appsv1.DeploymentVersionAnnotation:       strconv.FormatInt(config.Status.LatestVersion, 10),
				// This is the target replica count for the new deployment.
				appsv1.DesiredReplicasAnnotation: strconv.Itoa(int(config.Spec.Replicas)),
				DeploymentReplicasAnnotation:     strconv.Itoa(0),
			},
			Labels:          controllerLabels,
			OwnerReferences: []metav1.OwnerReference{*controllerRef},
		},
		Spec: v1.ReplicationControllerSpec{
			// The deployment should be inactive initially
			Replicas:        &zero,
			Selector:        selector,
			MinReadySeconds: config.Spec.MinReadySeconds,
			Template: &v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: *podSpec,
			},
		},
	}
	if config.Status.Details != nil && len(config.Status.Details.Message) > 0 {
		deployment.Annotations[appsv1.DeploymentStatusReasonAnnotation] = config.Status.Details.Message
	}
	if value, ok := config.Annotations[DeploymentIgnorePodAnnotation]; ok {
		deployment.Annotations[DeploymentIgnorePodAnnotation] = value
	}

	return deployment, nil
}

// SetDeploymentCondition updates the deployment to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then we are not going to update.
func SetDeploymentCondition(status *appsv1.DeploymentConfigStatus, condition appsv1.DeploymentCondition) {
	currentCond := GetDeploymentCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Preserve lastTransitionTime if we are not switching between statuses of a condition.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}

	newConditions := filterOutCondition(status.Conditions, condition.Type)
	status.Conditions = append(newConditions, condition)
}

// RemoveDeploymentCondition removes the deployment condition with the provided type.
func RemoveDeploymentCondition(status *appsv1.DeploymentConfigStatus, condType appsv1.DeploymentConditionType) {
	status.Conditions = filterOutCondition(status.Conditions, condType)
}

// filterOutCondition returns a new slice of deployment conditions without conditions with the provided type.
func filterOutCondition(conditions []appsv1.DeploymentCondition, condType appsv1.DeploymentConditionType) []appsv1.DeploymentCondition {
	var newConditions []appsv1.DeploymentCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// IsOwnedByConfig checks whether the provided replication controller is part of a
// deployment configuration.
// TODO: Switch to use owner references once we got those working.
func IsOwnedByConfig(obj metav1.Object) bool {
	_, ok := obj.GetAnnotations()[appsv1.DeploymentConfigAnnotation]
	return ok
}

// DeploymentsForCleanup determines which deployments for a configuration are relevant for the
// revision history limit quota
func DeploymentsForCleanup(configuration *appsv1.DeploymentConfig, deployments []*v1.ReplicationController) []v1.ReplicationController {
	// if the past deployment quota has been exceeded, we need to prune the oldest deployments
	// until we are not exceeding the quota any longer, so we sort oldest first
	sort.Sort(sort.Reverse(ByLatestVersionDesc(deployments)))

	relevantDeployments := []v1.ReplicationController{}
	activeDeployment := ActiveDeployment(deployments)
	if activeDeployment == nil {
		// if cleanup policy is set but no successful deployments have happened, there will be
		// no active deployment. We can consider all of the deployments in this case except for
		// the latest one
		for i := range deployments {
			deployment := deployments[i]
			if deploymentVersionFor(deployment) != configuration.Status.LatestVersion {
				relevantDeployments = append(relevantDeployments, *deployment)
			}
		}
	} else {
		// if there is an active deployment, we need to filter out any deployments that we don't
		// care about, namely the active deployment and any newer deployments
		for i := range deployments {
			deployment := deployments[i]
			if deployment != activeDeployment && deploymentVersionFor(deployment) < deploymentVersionFor(activeDeployment) {
				relevantDeployments = append(relevantDeployments, *deployment)
			}
		}
	}

	return relevantDeployments
}

// LabelForDeployment builds a string identifier for a Deployment.
func LabelForDeployment(deployment *v1.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// LabelForDeploymentConfig builds a string identifier for a DeploymentConfig.
func LabelForDeploymentConfig(config runtime.Object) string {
	accessor, _ := meta.Accessor(config)
	return fmt.Sprintf("%s/%s", accessor.GetNamespace(), accessor.GetName())
}

// LatestDeploymentNameForConfig returns a stable identifier for deployment config
func LatestDeploymentNameForConfig(config *appsv1.DeploymentConfig) string {
	return LatestDeploymentNameForConfigAndVersion(config.Name, config.Status.LatestVersion)
}

// DeploymentNameForConfigVersion returns the name of the version-th deployment
// for the config that has the provided name
func DeploymentNameForConfigVersion(name string, version int64) string {
	return fmt.Sprintf("%s-%d", name, version)
}

// LatestDeploymentNameForConfigAndVersion returns a stable identifier for config based on its version.
func LatestDeploymentNameForConfigAndVersion(name string, version int64) string {
	return fmt.Sprintf("%s-%d", name, version)
}

func DeployerPodNameFor(obj runtime.Object) string {
	return AnnotationFor(obj, appsv1.DeploymentPodAnnotation)
}

func DeploymentConfigNameFor(obj runtime.Object) string {
	return AnnotationFor(obj, appsv1.DeploymentConfigAnnotation)
}

func DeploymentStatusReasonFor(obj runtime.Object) string {
	return AnnotationFor(obj, appsv1.DeploymentStatusReasonAnnotation)
}

func DeleteStatusReasons(rc *v1.ReplicationController) {
	delete(rc.Annotations, appsv1.DeploymentStatusReasonAnnotation)
	delete(rc.Annotations, appsv1.DeploymentCancelledAnnotation)
}

func SetCancelledByUserReason(rc *v1.ReplicationController) {
	rc.Annotations[appsv1.DeploymentCancelledAnnotation] = "true"
	rc.Annotations[appsv1.DeploymentStatusReasonAnnotation] = deploymentCancelledByUser
}

func SetCancelledByNewerDeployment(rc *v1.ReplicationController) {
	rc.Annotations[appsv1.DeploymentCancelledAnnotation] = "true"
	rc.Annotations[appsv1.DeploymentStatusReasonAnnotation] = deploymentCancelledNewerDeploymentExists
}

// HasSynced checks if the provided deployment config has been noticed by the deployment
// config controller.
func HasSynced(dc *appsv1.DeploymentConfig, generation int64) bool {
	return dc.Status.ObservedGeneration >= generation
}

// HasChangeTrigger returns whether the provided deployment configuration has
// a config change trigger or not
func HasChangeTrigger(config *appsv1.DeploymentConfig) bool {
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == appsv1.DeploymentTriggerOnConfigChange {
			return true
		}
	}
	return false
}

// HasTrigger returns whether the provided deployment configuration has any trigger
// defined or not.
func HasTrigger(config *appsv1.DeploymentConfig) bool {
	return HasChangeTrigger(config) || HasImageChangeTrigger(config)
}

// HasLastTriggeredImage returns whether all image change triggers in provided deployment
// configuration has the lastTriggerImage field set (iow. all images were updated for
// them). Returns false if deployment configuration has no image change trigger defined.
func HasLastTriggeredImage(config *appsv1.DeploymentConfig) bool {
	hasImageTrigger := false
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == appsv1.DeploymentTriggerOnImageChange {
			hasImageTrigger = true
			if len(trigger.ImageChangeParams.LastTriggeredImage) == 0 {
				return false
			}
		}
	}
	return hasImageTrigger
}

// IsInitialDeployment returns whether the deployment configuration is the first version
// of this configuration.
func IsInitialDeployment(config *appsv1.DeploymentConfig) bool {
	return config.Status.LatestVersion == 0
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
	return labels.SelectorFromValidatedSet(labels.Set{appsv1.DeploymentConfigAnnotation: name})
}

// IsCompleteDeployment returns true if the passed deployment is in state complete.
func IsCompleteDeployment(deployment runtime.Object) bool {
	return DeploymentStatusFor(deployment) == appsv1.DeploymentStatusComplete
}

// IsFailedDeployment returns true if the passed deployment failed.
func IsFailedDeployment(deployment runtime.Object) bool {
	return DeploymentStatusFor(deployment) == appsv1.DeploymentStatusFailed
}

// IsTerminatedDeployment returns true if the passed deployment has terminated (either
// complete or failed).
func IsTerminatedDeployment(deployment runtime.Object) bool {
	return IsCompleteDeployment(deployment) || IsFailedDeployment(deployment)
}

func IsDeploymentCancelled(deployment runtime.Object) bool {
	value := AnnotationFor(deployment, appsv1.DeploymentCancelledAnnotation)
	return strings.EqualFold(value, "true")
}

// DeployerPodSelector returns a label Selector which can be used to find all
// deployer pods associated with a deployment with name.
func DeployerPodSelector(name string) labels.Selector {
	return labels.SelectorFromValidatedSet(labels.Set{appsv1.DeployerPodForDeploymentLabel: name})
}

func DeploymentStatusFor(deployment runtime.Object) appsv1.DeploymentStatus {
	return appsv1.DeploymentStatus(AnnotationFor(deployment, appsv1.DeploymentStatusAnnotation))
}

func SetDeploymentLatestVersionAnnotation(rc *v1.ReplicationController, version string) {
	if rc.Annotations == nil {
		rc.Annotations = map[string]string{}
	}
	rc.Annotations[appsv1.DeploymentVersionAnnotation] = version
}

func DeploymentVersionFor(obj runtime.Object) int64 {
	v, err := strconv.ParseInt(AnnotationFor(obj, appsv1.DeploymentVersionAnnotation), 10, 64)
	if err != nil {
		return -1
	}
	return v
}

func DeploymentNameFor(obj runtime.Object) string {
	return AnnotationFor(obj, appsv1.DeploymentAnnotation)
}

func deploymentVersionFor(obj runtime.Object) int64 {
	v, err := strconv.ParseInt(AnnotationFor(obj, appsv1.DeploymentVersionAnnotation), 10, 64)
	if err != nil {
		return -1
	}
	return v
}

// LatestDeploymentInfo returns info about the latest deployment for a config,
// or nil if there is no latest deployment. The latest deployment is not
// always the same as the active deployment.
func LatestDeploymentInfo(config *appsv1.DeploymentConfig, deployments []*v1.ReplicationController) (bool, *v1.ReplicationController) {
	if config.Status.LatestVersion == 0 || len(deployments) == 0 {
		return false, nil
	}
	sort.Sort(ByLatestVersionDesc(deployments))
	candidate := deployments[0]
	return deploymentVersionFor(candidate) == config.Status.LatestVersion, candidate
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
func CanTransitionPhase(current, next appsv1.DeploymentStatus) bool {
	switch current {
	case appsv1.DeploymentStatusNew:
		switch next {
		case appsv1.DeploymentStatusPending,
			appsv1.DeploymentStatusRunning,
			appsv1.DeploymentStatusFailed,
			appsv1.DeploymentStatusComplete:
			return true
		}
	case appsv1.DeploymentStatusPending:
		switch next {
		case appsv1.DeploymentStatusRunning,
			appsv1.DeploymentStatusFailed,
			appsv1.DeploymentStatusComplete:
			return true
		}
	case appsv1.DeploymentStatusRunning:
		switch next {
		case appsv1.DeploymentStatusFailed, appsv1.DeploymentStatusComplete:
			return true
		}
	}
	return false
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
