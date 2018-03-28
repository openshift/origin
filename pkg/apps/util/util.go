package util

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	api "k8s.io/kubernetes/pkg/apis/core"
	kapiv1 "k8s.io/kubernetes/pkg/apis/core/v1"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	kdeplutil "k8s.io/kubernetes/pkg/controller/deployment/util"

	appsapiv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

var (
	// DeploymentConfigControllerRefKind contains the schema.GroupVersionKind for the
	// deployment config. This is used in the ownerRef and GC client picks the appropriate
	// client to get the deployment config.
	DeploymentConfigControllerRefKind = appsapiv1.SchemeGroupVersion.WithKind("DeploymentConfig")
)

// NewDeploymentCondition creates a new deployment condition.
func NewDeploymentCondition(condType appsapi.DeploymentConditionType, status api.ConditionStatus, reason appsapi.DeploymentConditionReason, message string) *appsapi.DeploymentCondition {
	return &appsapi.DeploymentCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// GetDeploymentCondition returns the condition with the provided type.
func GetDeploymentCondition(status appsapi.DeploymentConfigStatus, condType appsapi.DeploymentConditionType) *appsapi.DeploymentCondition {
	for i := range status.Conditions {
		c := status.Conditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// SetDeploymentCondition updates the deployment to include the provided condition. If the condition that
// we are about to add already exists and has the same status and reason then we are not going to update.
func SetDeploymentCondition(status *appsapi.DeploymentConfigStatus, condition appsapi.DeploymentCondition) {
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
func RemoveDeploymentCondition(status *appsapi.DeploymentConfigStatus, condType appsapi.DeploymentConditionType) {
	status.Conditions = filterOutCondition(status.Conditions, condType)
}

// filterOutCondition returns a new slice of deployment conditions without conditions with the provided type.
func filterOutCondition(conditions []appsapi.DeploymentCondition, condType appsapi.DeploymentConditionType) []appsapi.DeploymentCondition {
	var newConditions []appsapi.DeploymentCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// LatestDeploymentNameForConfig returns a stable identifier for config based on its version.
func LatestDeploymentNameForConfig(config *appsapi.DeploymentConfig) string {
	return fmt.Sprintf("%s-%d", config.Name, config.Status.LatestVersion)
}

// LatestDeploymentInfo returns info about the latest deployment for a config,
// or nil if there is no latest deployment. The latest deployment is not
// always the same as the active deployment.
func LatestDeploymentInfo(config *appsapi.DeploymentConfig, deployments []*v1.ReplicationController) (bool, *v1.ReplicationController) {
	if config.Status.LatestVersion == 0 || len(deployments) == 0 {
		return false, nil
	}
	sort.Sort(ByLatestVersionDescV1(deployments))
	candidate := deployments[0]
	return DeploymentVersionFor(candidate) == config.Status.LatestVersion, candidate
}

// ActiveDeployment returns the latest complete deployment, or nil if there is
// no such deployment. The active deployment is not always the same as the
// latest deployment.
func ActiveDeployment(input []*api.ReplicationController) *api.ReplicationController {
	var activeDeployment *api.ReplicationController
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

// ActiveDeploymentV1 returns the latest complete deployment, or nil if there is
// no such deployment. The active deployment is not always the same as the
// latest deployment.
func ActiveDeploymentV1(input []*v1.ReplicationController) *v1.ReplicationController {
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

// DeployerPodSuffix is the suffix added to pods created from a deployment
const DeployerPodSuffix = "deploy"

// DeployerPodNameForDeployment returns the name of a pod for a given deployment
func DeployerPodNameForDeployment(deployment string) string {
	return apihelpers.GetPodName(deployment, DeployerPodSuffix)
}

// LabelForDeployment builds a string identifier for a Deployment.
func LabelForDeployment(deployment *api.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// LabelForDeployment builds a string identifier for a Deployment.
func LabelForDeploymentV1(deployment *v1.ReplicationController) string {
	return fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)
}

// LabelForDeploymentConfig builds a string identifier for a DeploymentConfig.
func LabelForDeploymentConfig(config *appsapi.DeploymentConfig) string {
	return fmt.Sprintf("%s/%s", config.Namespace, config.Name)
}

// DeploymentNameForConfigVersion returns the name of the version-th deployment
// for the config that has the provided name
func DeploymentNameForConfigVersion(name string, version int64) string {
	return fmt.Sprintf("%s-%d", name, version)
}

// ConfigSelector returns a label Selector which can be used to find all
// deployments for a DeploymentConfig.
//
// TODO: Using the annotation constant for now since the value is correct
// but we could consider adding a new constant to the public types.
func ConfigSelector(name string) labels.Selector {
	return labels.SelectorFromValidatedSet(labels.Set{appsapi.DeploymentConfigAnnotation: name})
}

// DeployerPodSelector returns a label Selector which can be used to find all
// deployer pods associated with a deployment with name.
func DeployerPodSelector(name string) labels.Selector {
	return labels.SelectorFromValidatedSet(labels.Set{appsapi.DeployerPodForDeploymentLabel: name})
}

// HasChangeTrigger returns whether the provided deployment configuration has
// a config change trigger or not
func HasChangeTrigger(config *appsapi.DeploymentConfig) bool {
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == appsapi.DeploymentTriggerOnConfigChange {
			return true
		}
	}
	return false
}

// HasImageChangeTrigger returns whether the provided deployment configuration has
// an image change trigger or not.
func HasImageChangeTrigger(config *appsapi.DeploymentConfig) bool {
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == appsapi.DeploymentTriggerOnImageChange {
			return true
		}
	}
	return false
}

// HasTrigger returns whether the provided deployment configuration has any trigger
// defined or not.
func HasTrigger(config *appsapi.DeploymentConfig) bool {
	return HasChangeTrigger(config) || HasImageChangeTrigger(config)
}

// HasLastTriggeredImage returns whether all image change triggers in provided deployment
// configuration has the lastTriggerImage field set (iow. all images were updated for
// them). Returns false if deployment configuration has no image change trigger defined.
func HasLastTriggeredImage(config *appsapi.DeploymentConfig) bool {
	hasImageTrigger := false
	for _, trigger := range config.Spec.Triggers {
		if trigger.Type == appsapi.DeploymentTriggerOnImageChange {
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
func IsInitialDeployment(config *appsapi.DeploymentConfig) bool {
	return config.Status.LatestVersion == 0
}

// RecordConfigChangeCause sets a deployment config cause for config change.
func RecordConfigChangeCause(config *appsapi.DeploymentConfig) {
	config.Status.Details = &appsapi.DeploymentDetails{
		Causes: []appsapi.DeploymentCause{
			{
				Type: appsapi.DeploymentTriggerOnConfigChange,
			},
		},
		Message: "config change",
	}
}

// RecordImageChangeCauses sets a deployment config cause for image change. It
// takes a list of changed images and record an cause for each image.
func RecordImageChangeCauses(config *appsapi.DeploymentConfig, imageNames []string) {
	config.Status.Details = &appsapi.DeploymentDetails{
		Message: "image change",
	}
	for _, imageName := range imageNames {
		config.Status.Details.Causes = append(config.Status.Details.Causes, appsapi.DeploymentCause{
			Type:         appsapi.DeploymentTriggerOnImageChange,
			ImageTrigger: &appsapi.DeploymentCauseImageTrigger{From: api.ObjectReference{Kind: "DockerImage", Name: imageName}},
		})
	}
}

func CopyApiResourcesToV1Resources(in *api.ResourceRequirements) v1.ResourceRequirements {
	in = in.DeepCopy()
	out := v1.ResourceRequirements{}
	if err := kapiv1.Convert_core_ResourceRequirements_To_v1_ResourceRequirements(in, &out, nil); err != nil {
		panic(err)
	}
	return out
}

func CopyApiEnvVarToV1EnvVar(in []api.EnvVar) []v1.EnvVar {
	out := make([]v1.EnvVar, len(in))
	for i := range in {
		if err := kapiv1.Convert_core_EnvVar_To_v1_EnvVar(in[i].DeepCopy(), &out[i], nil); err != nil {
			panic(err)
		}
	}
	return out
}

// HasLatestPodTemplate checks for differences between current deployment config
// template and deployment config template encoded in the latest replication
// controller. If they are different it will return an string diff containing
// the change.
func HasLatestPodTemplate(currentConfig *appsapi.DeploymentConfig, rc *v1.ReplicationController, codec runtime.Codec) (bool, string, error) {
	latestConfig, err := DecodeDeploymentConfig(rc, codec)
	if err != nil {
		return true, "", err
	}
	// The latestConfig represents an encoded DC in the latest deployment (RC).
	// TODO: This diverges from the upstream behavior where we compare deployment
	// template vs. replicaset template. Doing that will disallow any
	// modifications to the RC the deployment config controller create and manage
	// as a change to the RC will cause the DC to be reconciled and ultimately
	// trigger a new rollout because of skew between latest RC template and DC
	// template.
	if reflect.DeepEqual(currentConfig.Spec.Template, latestConfig.Spec.Template) {
		return true, "", nil
	}
	return false, diff.ObjectReflectDiff(currentConfig.Spec.Template, latestConfig.Spec.Template), nil
}

// HasUpdatedImages indicates if the deployment configuration images were updated.
func HasUpdatedImages(dc *appsapi.DeploymentConfig, rc *v1.ReplicationController) (bool, []string) {
	updatedImages := []string{}
	rcImages := sets.NewString()
	for _, c := range rc.Spec.Template.Spec.Containers {
		rcImages.Insert(c.Image)
	}
	for _, c := range dc.Spec.Template.Spec.Containers {
		if !rcImages.Has(c.Image) {
			updatedImages = append(updatedImages, c.Image)
		}
	}
	if len(updatedImages) == 0 {
		return false, nil
	}
	return true, updatedImages
}

// DecodeDeploymentConfig decodes a DeploymentConfig from controller using codec. An error is returned
// if the controller doesn't contain an encoded config.
func DecodeDeploymentConfig(controller runtime.Object, decoder runtime.Decoder) (*appsapi.DeploymentConfig, error) {
	encodedConfig := []byte(EncodedDeploymentConfigFor(controller))
	decoded, err := runtime.Decode(decoder, encodedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode DeploymentConfig from controller: %v", err)
	}
	config, ok := decoded.(*appsapi.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("decoded object from controller is not a DeploymentConfig")
	}
	return config, nil
}

// EncodeDeploymentConfig encodes config as a string using codec.
func EncodeDeploymentConfig(config *appsapi.DeploymentConfig, codec runtime.Codec) (string, error) {
	bytes, err := runtime.Encode(codec, config)
	if err != nil {
		return "", err
	}
	return string(bytes[:]), nil
}

func NewControllerRef(config *appsapi.DeploymentConfig) *metav1.OwnerReference {
	blockOwnerDeletion := true
	isController := true
	return &metav1.OwnerReference{
		APIVersion:         DeploymentConfigControllerRefKind.GroupVersion().String(),
		Kind:               DeploymentConfigControllerRefKind.Kind,
		Name:               config.Name,
		UID:                config.UID,
		BlockOwnerDeletion: &blockOwnerDeletion,
		Controller:         &isController,
	}
}

// MakeDeployment creates a deployment represented as an internal ReplicationController and based on the given
// DeploymentConfig. The controller replica count will be zero.
// DEPRECATED: Will be replaced with external version eventually.
func MakeDeployment(config *appsapi.DeploymentConfig, codec runtime.Codec) (*api.ReplicationController, error) {
	obj, err := MakeDeploymentV1(config, codec)
	if err != nil {
		return nil, err
	}
	kapiv1.SetObjectDefaults_ReplicationController(obj)
	converted, err := legacyscheme.Scheme.ConvertToVersion(obj, api.SchemeGroupVersion)
	if err != nil {
		return nil, err
	}
	deployment := converted.(*api.ReplicationController)
	return deployment, nil
}

// MakeDeploymentV1 creates a deployment represented as a ReplicationController and based on the given
// DeploymentConfig. The controller replica count will be zero.
func MakeDeploymentV1(config *appsapi.DeploymentConfig, codec runtime.Codec) (*v1.ReplicationController, error) {
	var err error
	var encodedConfig string

	if encodedConfig, err = EncodeDeploymentConfig(config, codec); err != nil {
		return nil, err
	}

	deploymentName := LatestDeploymentNameForConfig(config)

	podSpec := v1.PodSpec{}
	if err := legacyscheme.Scheme.Convert(&config.Spec.Template.Spec, &podSpec, nil); err != nil {
		return nil, fmt.Errorf("couldn't clone podSpec: %v", err)
	}

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
	controllerLabels[appsapi.DeploymentConfigAnnotation] = config.Name

	// Ensure that pods created by this deployment controller can be safely associated back
	// to the controller, and that multiple deployment controllers for the same config don't
	// manipulate each others' pods.
	selector := map[string]string{}
	for k, v := range config.Spec.Selector {
		selector[k] = v
	}
	selector[appsapi.DeploymentConfigLabel] = config.Name
	selector[appsapi.DeploymentLabel] = deploymentName

	podLabels := make(labels.Set)
	for k, v := range config.Spec.Template.Labels {
		podLabels[k] = v
	}
	podLabels[appsapi.DeploymentConfigLabel] = config.Name
	podLabels[appsapi.DeploymentLabel] = deploymentName

	podAnnotations := make(labels.Set)
	for k, v := range config.Spec.Template.Annotations {
		podAnnotations[k] = v
	}
	podAnnotations[appsapi.DeploymentAnnotation] = deploymentName
	podAnnotations[appsapi.DeploymentConfigAnnotation] = config.Name
	podAnnotations[appsapi.DeploymentVersionAnnotation] = strconv.FormatInt(config.Status.LatestVersion, 10)

	controllerRef := NewControllerRef(config)
	zero := int32(0)
	deployment := &v1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: config.Namespace,
			Annotations: map[string]string{
				appsapi.DeploymentConfigAnnotation:        config.Name,
				appsapi.DeploymentStatusAnnotation:        string(appsapi.DeploymentStatusNew),
				appsapi.DeploymentEncodedConfigAnnotation: encodedConfig,
				appsapi.DeploymentVersionAnnotation:       strconv.FormatInt(config.Status.LatestVersion, 10),
				// This is the target replica count for the new deployment.
				appsapi.DesiredReplicasAnnotation:    strconv.Itoa(int(config.Spec.Replicas)),
				appsapi.DeploymentReplicasAnnotation: strconv.Itoa(0),
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
				Spec: podSpec,
			},
		},
	}
	if config.Status.Details != nil && len(config.Status.Details.Message) > 0 {
		deployment.Annotations[appsapi.DeploymentStatusReasonAnnotation] = config.Status.Details.Message
	}
	if value, ok := config.Annotations[appsapi.DeploymentIgnorePodAnnotation]; ok {
		deployment.Annotations[appsapi.DeploymentIgnorePodAnnotation] = value
	}

	return deployment, nil
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

func DeploymentConfigNameFor(obj runtime.Object) string {
	return annotationFor(obj, appsapi.DeploymentConfigAnnotation)
}

func DeploymentNameFor(obj runtime.Object) string {
	return annotationFor(obj, appsapi.DeploymentAnnotation)
}

func DeployerPodNameFor(obj runtime.Object) string {
	return annotationFor(obj, appsapi.DeploymentPodAnnotation)
}

func DeploymentStatusFor(obj runtime.Object) appsapi.DeploymentStatus {
	return appsapi.DeploymentStatus(annotationFor(obj, appsapi.DeploymentStatusAnnotation))
}

func DeploymentStatusReasonFor(obj runtime.Object) string {
	return annotationFor(obj, appsapi.DeploymentStatusReasonAnnotation)
}

func DeploymentDesiredReplicas(obj runtime.Object) (int32, bool) {
	return int32AnnotationFor(obj, appsapi.DesiredReplicasAnnotation)
}

func EncodedDeploymentConfigFor(obj runtime.Object) string {
	return annotationFor(obj, appsapi.DeploymentEncodedConfigAnnotation)
}

func DeploymentVersionFor(obj runtime.Object) int64 {
	v, err := strconv.ParseInt(annotationFor(obj, appsapi.DeploymentVersionAnnotation), 10, 64)
	if err != nil {
		return -1
	}
	return v
}

func IsDeploymentCancelled(deployment runtime.Object) bool {
	value := annotationFor(deployment, appsapi.DeploymentCancelledAnnotation)
	return strings.EqualFold(value, appsapi.DeploymentCancelledAnnotationValue)
}

// HasSynced checks if the provided deployment config has been noticed by the deployment
// config controller.
func HasSynced(dc *appsapi.DeploymentConfig, generation int64) bool {
	return dc.Status.ObservedGeneration >= generation
}

// IsOwnedByConfig checks whether the provided replication controller is part of a
// deployment configuration.
// TODO: Switch to use owner references once we got those working.
func IsOwnedByConfig(obj metav1.Object) bool {
	_, ok := obj.GetAnnotations()[appsapi.DeploymentConfigAnnotation]
	return ok
}

// IsTerminatedDeployment returns true if the passed deployment has terminated (either
// complete or failed).
func IsTerminatedDeployment(deployment runtime.Object) bool {
	return IsCompleteDeployment(deployment) || IsFailedDeployment(deployment)
}

// IsCompleteDeployment returns true if the passed deployment is in state complete.
func IsCompleteDeployment(deployment runtime.Object) bool {
	current := DeploymentStatusFor(deployment)
	return current == appsapi.DeploymentStatusComplete
}

// IsFailedDeployment returns true if the passed deployment failed.
func IsFailedDeployment(deployment runtime.Object) bool {
	current := DeploymentStatusFor(deployment)
	return current == appsapi.DeploymentStatusFailed
}

// CanTransitionPhase returns whether it is allowed to go from the current to the next phase.
func CanTransitionPhase(current, next appsapi.DeploymentStatus) bool {
	switch current {
	case appsapi.DeploymentStatusNew:
		switch next {
		case appsapi.DeploymentStatusPending,
			appsapi.DeploymentStatusRunning,
			appsapi.DeploymentStatusFailed,
			appsapi.DeploymentStatusComplete:
			return true
		}
	case appsapi.DeploymentStatusPending:
		switch next {
		case appsapi.DeploymentStatusRunning,
			appsapi.DeploymentStatusFailed,
			appsapi.DeploymentStatusComplete:
			return true
		}
	case appsapi.DeploymentStatusRunning:
		switch next {
		case appsapi.DeploymentStatusFailed, appsapi.DeploymentStatusComplete:
			return true
		}
	}
	return false
}

// IsRollingConfig returns true if the strategy type is a rolling update.
func IsRollingConfig(config *appsapi.DeploymentConfig) bool {
	return config.Spec.Strategy.Type == appsapi.DeploymentStrategyTypeRolling
}

// IsProgressing expects a state deployment config and its updated status in order to
// determine if there is any progress.
func IsProgressing(config *appsapi.DeploymentConfig, newStatus *appsapi.DeploymentConfigStatus) bool {
	oldStatusOldReplicas := config.Status.Replicas - config.Status.UpdatedReplicas
	newStatusOldReplicas := newStatus.Replicas - newStatus.UpdatedReplicas

	return (newStatus.UpdatedReplicas > config.Status.UpdatedReplicas) || (newStatusOldReplicas < oldStatusOldReplicas)
}

// MaxUnavailable returns the maximum unavailable pods a rolling deployment config can take.
func MaxUnavailable(config *appsapi.DeploymentConfig) int32 {
	if !IsRollingConfig(config) {
		return int32(0)
	}
	// Error caught by validation
	_, maxUnavailable, _ := kdeplutil.ResolveFenceposts(&config.Spec.Strategy.RollingParams.MaxSurge, &config.Spec.Strategy.RollingParams.MaxUnavailable, config.Spec.Replicas)
	return maxUnavailable
}

// MaxSurge returns the maximum surge pods a rolling deployment config can take.
func MaxSurge(config appsapi.DeploymentConfig) int32 {
	if !IsRollingConfig(&config) {
		return int32(0)
	}
	// Error caught by validation
	maxSurge, _, _ := kdeplutil.ResolveFenceposts(&config.Spec.Strategy.RollingParams.MaxSurge, &config.Spec.Strategy.RollingParams.MaxUnavailable, config.Spec.Replicas)
	return maxSurge
}

// annotationFor returns the annotation with key for obj.
func annotationFor(obj runtime.Object, key string) string {
	objectMeta, err := meta.Accessor(obj)
	if err != nil {
		return ""
	}
	if objectMeta == nil || reflect.ValueOf(objectMeta).IsNil() {
		return ""
	}
	return objectMeta.GetAnnotations()[key]
}

func int32AnnotationFor(obj runtime.Object, key string) (int32, bool) {
	s := annotationFor(obj, key)
	if len(s) == 0 {
		return 0, false
	}
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(i), true
}

// DeploymentsForCleanup determines which deployments for a configuration are relevant for the
// revision history limit quota
func DeploymentsForCleanup(configuration *appsapi.DeploymentConfig, deployments []*v1.ReplicationController) []v1.ReplicationController {
	// if the past deployment quota has been exceeded, we need to prune the oldest deployments
	// until we are not exceeding the quota any longer, so we sort oldest first
	sort.Sort(ByLatestVersionAscV1(deployments))

	relevantDeployments := []v1.ReplicationController{}
	activeDeployment := ActiveDeploymentV1(deployments)
	if activeDeployment == nil {
		// if cleanup policy is set but no successful deployments have happened, there will be
		// no active deployment. We can consider all of the deployments in this case except for
		// the latest one
		for i := range deployments {
			deployment := deployments[i]
			if DeploymentVersionFor(deployment) != configuration.Status.LatestVersion {
				relevantDeployments = append(relevantDeployments, *deployment)
			}
		}
	} else {
		// if there is an active deployment, we need to filter out any deployments that we don't
		// care about, namely the active deployment and any newer deployments
		for i := range deployments {
			deployment := deployments[i]
			if deployment != activeDeployment && DeploymentVersionFor(deployment) < DeploymentVersionFor(activeDeployment) {
				relevantDeployments = append(relevantDeployments, *deployment)
			}
		}
	}

	return relevantDeployments
}

// GetTimeoutSecondsForStrategy returns the timeout in seconds defined in the
// deployment config strategy.
func GetTimeoutSecondsForStrategy(config *appsapi.DeploymentConfig) int64 {
	var timeoutSeconds int64
	switch config.Spec.Strategy.Type {
	case appsapi.DeploymentStrategyTypeRolling:
		timeoutSeconds = appsapi.DefaultRollingTimeoutSeconds
		if t := config.Spec.Strategy.RollingParams.TimeoutSeconds; t != nil {
			timeoutSeconds = *t
		}
	case appsapi.DeploymentStrategyTypeRecreate:
		timeoutSeconds = appsapi.DefaultRecreateTimeoutSeconds
		if t := config.Spec.Strategy.RecreateParams.TimeoutSeconds; t != nil {
			timeoutSeconds = *t
		}
	case appsapi.DeploymentStrategyTypeCustom:
		timeoutSeconds = appsapi.DefaultRecreateTimeoutSeconds
	}
	return timeoutSeconds
}

// RolloutExceededTimeoutSeconds returns true if the current deployment exceeded
// the timeoutSeconds defined for its strategy.
// Note that this is different than activeDeadlineSeconds which is the timeout
// set for the deployer pod. In some cases, the deployer pod cannot be created
// (like quota, etc...). In that case deployer controller use this function to
// measure if the created deployment (RC) exceeded the timeout.
func RolloutExceededTimeoutSeconds(config *appsapi.DeploymentConfig, latestRC *v1.ReplicationController) bool {
	timeoutSeconds := GetTimeoutSecondsForStrategy(config)
	// If user set the timeoutSeconds to 0, we assume there should be no timeout.
	if timeoutSeconds <= 0 {
		return false
	}
	return int64(time.Since(latestRC.CreationTimestamp.Time).Seconds()) > timeoutSeconds
}

// WaitForRunningDeployerPod waits a given period of time until the deployer pod
// for given replication controller is not running.
func WaitForRunningDeployerPod(podClient kcoreclient.PodsGetter, rc *api.ReplicationController, timeout time.Duration) error {
	podName := DeployerPodNameForDeployment(rc.Name)
	canGetLogs := func(p *api.Pod) bool {
		return api.PodSucceeded == p.Status.Phase || api.PodFailed == p.Status.Phase || api.PodRunning == p.Status.Phase
	}
	pod, err := podClient.Pods(rc.Namespace).Get(podName, metav1.GetOptions{})
	if err == nil && canGetLogs(pod) {
		return nil
	}
	watcher, err := podClient.Pods(rc.Namespace).Watch(
		metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", podName).String(),
		},
	)
	if err != nil {
		return err
	}

	defer watcher.Stop()
	_, err = watch.Until(timeout, watcher, func(e watch.Event) (bool, error) {
		if e.Type == watch.Error {
			return false, fmt.Errorf("encountered error while watching for pod: %v", e.Object)
		}
		obj, isPod := e.Object.(*api.Pod)
		if !isPod {
			return false, errors.New("received unknown object while watching for pods")
		}
		return canGetLogs(obj), nil
	})
	return err
}

// ByLatestVersionAsc sorts deployments by LatestVersion ascending.
type ByLatestVersionAsc []*api.ReplicationController

func (d ByLatestVersionAsc) Len() int      { return len(d) }
func (d ByLatestVersionAsc) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionAsc) Less(i, j int) bool {
	return DeploymentVersionFor(d[i]) < DeploymentVersionFor(d[j])
}

type ByLatestVersionAscV1 []*v1.ReplicationController

func (d ByLatestVersionAscV1) Len() int      { return len(d) }
func (d ByLatestVersionAscV1) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionAscV1) Less(i, j int) bool {
	return DeploymentVersionFor(d[i]) < DeploymentVersionFor(d[j])
}

// ByLatestVersionDesc sorts deployments by LatestVersion descending.
type ByLatestVersionDesc []*api.ReplicationController

func (d ByLatestVersionDesc) Len() int      { return len(d) }
func (d ByLatestVersionDesc) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionDesc) Less(i, j int) bool {
	return DeploymentVersionFor(d[j]) < DeploymentVersionFor(d[i])
}

// ByLatestVersionDescV1 sorts deployments by LatestVersion descending.
type ByLatestVersionDescV1 []*v1.ReplicationController

func (d ByLatestVersionDescV1) Len() int      { return len(d) }
func (d ByLatestVersionDescV1) Swap(i, j int) { d[i], d[j] = d[j], d[i] }
func (d ByLatestVersionDescV1) Less(i, j int) bool {
	return DeploymentVersionFor(d[j]) < DeploymentVersionFor(d[i])
}

// ByMostRecent sorts deployments by most recently created.
type ByMostRecent []*api.ReplicationController

func (s ByMostRecent) Len() int      { return len(s) }
func (s ByMostRecent) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s ByMostRecent) Less(i, j int) bool {
	return !s[i].CreationTimestamp.Before(&s[j].CreationTimestamp)
}
