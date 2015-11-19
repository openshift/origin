package v1beta3

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1beta3"
	kutil "k8s.io/kubernetes/pkg/util"
)

// DeploymentPhase describes the possible states a deployment can be in.
type DeploymentPhase string

const (
	// DeploymentPhaseNew means the deployment has been accepted but not yet acted upon.
	DeploymentPhaseNew DeploymentPhase = "New"
	// DeploymentPhasePending means the deployment been handed over to a deployment strategy,
	// but the strategy has not yet declared the deployment to be running.
	DeploymentPhasePending DeploymentPhase = "Pending"
	// DeploymentPhaseRunning means the deployment strategy has reported the deployment as
	// being in-progress.
	DeploymentPhaseRunning DeploymentPhase = "Running"
	// DeploymentPhaseComplete means the deployment finished without an error.
	DeploymentPhaseComplete DeploymentPhase = "Complete"
	// DeploymentPhaseFailed means the deployment finished with an error.
	DeploymentPhaseFailed DeploymentPhase = "Failed"
)

// DeploymentStrategy describes how to perform a deployment.
type DeploymentStrategy struct {
	// Type is the name of a deployment strategy.
	Type DeploymentStrategyType `json:"type,omitempty" description:"the name of a deployment strategy"`
	// CustomParams are the input to the Custom deployment strategy.
	CustomParams *CustomDeploymentStrategyParams `json:"customParams,omitempty" description:"input to the Custom deployment strategy"`
	// RecreateParams are the input to the Recreate deployment strategy.
	RecreateParams *RecreateDeploymentStrategyParams `json:"recreateParams,omitempty" description:"input to the Recreate deployment strategy"`
	// RollingParams are the input to the Rolling deployment strategy.
	RollingParams *RollingDeploymentStrategyParams `json:"rollingParams,omitempty" description:"input to the Rolling deployment strategy"`
	// Compute resource requirements to execute the deployment
	Resources kapi.ResourceRequirements `json:"resources,omitempty" description:"resource requirements to execute the deployment"`
	// Labels is a set of key, value pairs added to custom deployer and lifecycle pre/post hook pods.
	Labels map[string]string `json:"labels,omitempty" description:"labels for deployer and hook pods"`
	// Annotations is a set of key, value pairs added to custom deployer and lifecycle pre/post hook pods.
	Annotations map[string]string `json:"annotations,omitempty" description:"annotations for deployer and hook pods"`
}

// DeploymentStrategyType refers to a specific DeploymentStrategy implementation.
type DeploymentStrategyType string

const (
	// DeploymentStrategyTypeRecreate is a simple strategy suitable as a default.
	DeploymentStrategyTypeRecreate DeploymentStrategyType = "Recreate"
	// DeploymentStrategyTypeCustom is a user defined strategy.
	DeploymentStrategyTypeCustom DeploymentStrategyType = "Custom"
	// DeploymentStrategyTypeRolling uses the Kubernetes RollingUpdater.
	DeploymentStrategyTypeRolling DeploymentStrategyType = "Rolling"
)

// CustomParams are the input to the Custom deployment strategy.
type CustomDeploymentStrategyParams struct {
	// Image specifies a Docker image which can carry out a deployment.
	Image string `json:"image,omitempty" description:"a Docker image which can carry out a deployment"`
	// Environment holds the environment which will be given to the container for Image.
	Environment []kapi.EnvVar `json:"environment,omitempty" description:"environment variables provided to the deployment process container"`
	// Command is optional and overrides CMD in the container Image.
	Command []string `json:"command,omitempty" description:"optionally overrides the container command (default is specified by the image)"`
}

// RecreateDeploymentStrategyParams are the input to the Recreate deployment
// strategy.
type RecreateDeploymentStrategyParams struct {
	// Pre is a lifecycle hook which is executed before the strategy manipulates
	// the deployment. All LifecycleHookFailurePolicy values are supported.
	Pre *LifecycleHook `json:"pre,omitempty" description:"a hook executed before the strategy starts the deployment"`
	// Post is a lifecycle hook which is executed after the strategy has
	// finished all deployment logic. The LifecycleHookFailurePolicyAbort policy
	// is NOT supported.
	Post *LifecycleHook `json:"post,omitempty" description:"a hook executed after the strategy finishes the deployment"`
}

// Handler defines a specific deployment lifecycle action.
type LifecycleHook struct {
	// FailurePolicy specifies what action to take if the hook fails.
	FailurePolicy LifecycleHookFailurePolicy `json:"failurePolicy" description:"what action to take if the hook fails"`
	// ExecNewPod specifies the options for a lifecycle hook backed by a pod.
	ExecNewPod *ExecNewPodHook `json:"execNewPod,omitempty" description:"options for an ExecNewPodHook"`
}

// HandlerFailurePolicy describes possibles actions to take if a hook fails.
type LifecycleHookFailurePolicy string

const (
	// LifecycleHookFailurePolicyRetry means retry the hook until it succeeds.
	LifecycleHookFailurePolicyRetry LifecycleHookFailurePolicy = "Retry"
	// LifecycleHookFailurePolicyAbort means abort the deployment (if possible).
	LifecycleHookFailurePolicyAbort LifecycleHookFailurePolicy = "Abort"
	// LifecycleHookFailurePolicyIgnore means ignore failure and continue the deployment.
	LifecycleHookFailurePolicyIgnore LifecycleHookFailurePolicy = "Ignore"
)

// ExecNewPodHook is a hook implementation which runs a command in a new pod
// based on the specified container which is assumed to be part of the
// deployment template.
type ExecNewPodHook struct {
	// Command is the action command and its arguments.
	Command []string `json:"command" description:"the hook command and its arguments"`
	// Env is a set of environment variables to supply to the hook pod's container.
	Env []kapi.EnvVar `json:"env,omitempty" description:"environment variables provided to the hook container"`
	// ContainerName is the name of a container in the deployment pod template
	// whose Docker image will be used for the hook pod's container.
	ContainerName string `json:"containerName" description:"the name of a container from the pod template whose image will be used for the hook container"`
	// Volumes is a list of named volumes from the pod template which should be
	// copied to the hook pod. Volumes names not found in pod spec are ignored.
	// An empty list means no volumes will be copied.
	Volumes []string `json:"volumes,omitempty" description:"the names of volumes from the pod template which should be included in the hook pod; an empty list means no volumes will be copied, and names not found in the pod spec will be ignored"`
}

// RollingDeploymentStrategyParams are the input to the Rolling deployment
// strategy.
type RollingDeploymentStrategyParams struct {
	// UpdatePeriodSeconds is the time to wait between individual pod updates.
	// If the value is nil, a default will be used.
	UpdatePeriodSeconds *int64 `json:"updatePeriodSeconds,omitempty" description:"the time to wait between individual pod updates"`
	// IntervalSeconds is the time to wait between polling deployment status
	// after update. If the value is nil, a default will be used.
	IntervalSeconds *int64 `json:"intervalSeconds,omitempty" description:"the time to wait between polling deployment status after update"`
	// TimeoutSeconds is the time to wait for updates before giving up. If the
	// value is nil, a default will be used.
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty" description:"the time to wait for updates before giving up"`
	// MaxUnavailable is the maximum number of pods that can be unavailable
	// during the update. Value can be an absolute number (ex: 5) or a
	// percentage of total pods at the start of update (ex: 10%). Absolute
	// number is calculated from percentage by rounding up.
	//
	// This cannot be 0 if MaxSurge is 0. By default, 25% is used.
	//
	// Example: when this is set to 30%, the old RC can be scaled down by 30%
	// immediately when the rolling update starts. Once new pods are ready, old
	// RC can be scaled down further, followed by scaling up the new RC,
	// ensuring that at least 70% of original number of pods are available at
	// all times during the update.
	MaxUnavailable *kutil.IntOrString `json:"maxUnavailable,omitempty" description:"max number of pods that can be unavailable during the update; value can be an absolute number or a percentage of total pods at start of update"`
	// MaxSurge is the maximum number of pods that can be scheduled above the
	// original number of pods. Value can be an absolute number (ex: 5) or a
	// percentage of total pods at the start of the update (ex: 10%). Absolute
	// number is calculated from percentage by rounding up.
	//
	// This cannot be 0 if MaxUnavailable is 0. By default, 25% is used.
	//
	// Example: when this is set to 30%, the new RC can be scaled up by 30%
	// immediately when the rolling update starts. Once old pods have been
	// killed, new RC can be scaled up further, ensuring that total number of
	// pods running at any time during the update is atmost 130% of original
	// pods.
	MaxSurge *kutil.IntOrString `json:"maxSurge,omitempty" description:"max number of pods that can be scheduled above the original number of pods; value can be an absolute number or a percentage of total pods at start of update"`
	// UpdatePercent is the percentage of replicas to scale up or down each
	// interval. If nil, one replica will be scaled up and down each interval.
	// If negative, the scale order will be down/up instead of up/down.
	// DEPRECATED: Use MaxUnavailable/MaxSurge instead.
	UpdatePercent *int `json:"updatePercent,omitempty" description:"the percentage of replicas to scale up or down each interval (negative value switches scale order to down/up instead of up/down)"`
	// Pre is a lifecycle hook which is executed before the deployment process
	// begins. All LifecycleHookFailurePolicy values are supported.
	Pre *LifecycleHook `json:"pre,omitempty" description:"a hook executed before the strategy starts the deployment"`
	// Post is a lifecycle hook which is executed after the strategy has
	// finished all deployment logic. The LifecycleHookFailurePolicyAbort policy
	// is NOT supported.
	Post *LifecycleHook `json:"post,omitempty" description:"a hook executed after the strategy finishes the deployment"`
}

// These constants represent keys used for correlating objects related to deployments.
const (
	// DeploymentConfigAnnotation is an annotation name used to correlate a deployment with the
	// DeploymentConfig on which the deployment is based.
	DeploymentConfigAnnotation = "openshift.io/deployment-config.name"
	// DeploymentAnnotation is an annotation on a deployer Pod. The annotation value is the name
	// of the deployment (a ReplicationController) on which the deployer Pod acts.
	DeploymentAnnotation = "openshift.io/deployment.name"
	// DeploymentPodAnnotation is an annotation on a deployment (a ReplicationController). The
	// annotation value is the name of the deployer Pod which will act upon the ReplicationController
	// to implement the deployment behavior.
	DeploymentPodAnnotation = "openshift.io/deployer-pod.name"
	// DeployerPodForDeploymentLabel is a label which groups pods related to a
	// deployment. The value is a deployment name. The deployer pod and hook pods
	// created by the internal strategies will have this label. Custom
	// strategies can apply this label to any pods they create, enabling
	// platform-provided cancellation and garbage collection support.
	DeployerPodForDeploymentLabel = "openshift.io/deployer-pod-for.name"
	// DeploymentPhaseAnnotation is an annotation name used to retrieve the DeploymentPhase of
	// a deployment.
	DeploymentPhaseAnnotation = "openshift.io/deployment.phase"
	// DeploymentEncodedConfigAnnotation is an annotation name used to retrieve specific encoded
	// DeploymentConfig on which a given deployment is based.
	DeploymentEncodedConfigAnnotation = "openshift.io/encoded-deployment-config"
	// DeploymentVersionAnnotation is an annotation on a deployment (a ReplicationController). The
	// annotation value is the LatestVersion value of the DeploymentConfig which was the basis for
	// the deployment.
	DeploymentVersionAnnotation = "openshift.io/deployment-config.latest-version"
	// DeploymentLabel is the name of a label used to correlate a deployment with the Pod created
	// to execute the deployment logic.
	// TODO: This is a workaround for upstream's lack of annotation support on PodTemplate. Once
	// annotations are available on PodTemplate, audit this constant with the goal of removing it.
	DeploymentLabel = "deployment"
	// DeploymentConfigLabel is the name of a label used to correlate a deployment with the
	// DeploymentConfigs on which the deployment is based.
	DeploymentConfigLabel = "deploymentconfig"
	// DeploymentStatusReasonAnnotation represents the reason for deployment being in a given state
	// Used for specifying the reason for cancellation or failure of a deployment
	DeploymentStatusReasonAnnotation = "openshift.io/deployment.status-reason"
	// DeploymentCancelledAnnotation indicates that the deployment has been cancelled
	// The annotation value does not matter and its mere presence indicates cancellation
	DeploymentCancelledAnnotation = "openshift.io/deployment.cancelled"
)

// These constants represent the various reasons for cancelling a deployment
// or for a deployment being placed in a failed state
const (
	DeploymentCancelledByUser                 = "The deployment was cancelled by the user"
	DeploymentCancelledNewerDeploymentExists  = "The deployment was cancelled as a newer deployment was found running"
	DeploymentFailedUnrelatedDeploymentExists = "The deployment failed as an unrelated pod with the same name as this deployment is already running"
	DeploymentFailedDeployerPodNoLongerExists = "The deployment failed as the deployer pod no longer exists"
)

// This constant represents the maximum duration that a deployment is allowed to run
// This is set as the default value for ActiveDeadlineSeconds for the deployer pod
// Currently set to 6 hours
const MaxDeploymentDurationSeconds int64 = 21600

// This constant represents the value for the DeploymentCancelledAnnotation annotation
// that signifies that the deployment should be cancelled
const DeploymentCancelledAnnotationValue = "true"

// DeploymentConfig represents a configuration for a single deployment (represented as a
// ReplicationController). It also contains details about changes which resulted in the current
// state of the DeploymentConfig. Each change to the DeploymentConfig which should result in
// a new deployment results in an increment of LatestVersion.
type DeploymentConfig struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`
	// Spec represents a desired deployment state and how to deploy to it.
	Spec DeploymentConfigSpec `json:"spec" description:"a desired deployment state and how to deploy it"`
	// Status represents a desired deployment state and how to deploy to it.
	Status DeploymentConfigStatus `json:"status" description:"the current state of the latest deployment"`
}

// DeploymentTemplate contains all the necessary information to create a deployment from a
// DeploymentStrategy.
type DeploymentConfigSpec struct {
	// Strategy describes how a deployment is executed.
	Strategy DeploymentStrategy `json:"strategy,omitempty" description:"how a deployment is executed"`

	// Triggers determine how updates to a DeploymentConfig result in new deployments. If no triggers
	// are defined, a new deployment can only occur as a result of an explicit client update to the
	// DeploymentConfig with a new LatestVersion.
	Triggers []DeploymentTriggerPolicy `json:"triggers,omitempty" description:"how new deployments are triggered"`

	// Replicas is the number of desired replicas.
	Replicas int `json:"replicas" description:"the desired number of replicas"`

	// Selector is a label query over pods that should match the Replicas count.
	Selector map[string]string `json:"selector" description:"a label query over pods that should match the replicas count"`

	// TODO removed from rc spec
	// TemplateRef is a reference to an object that describes the pod that will be created if
	// insufficient replicas are detected. This reference is ignored if a Template is set.
	// Must be set before converting to a v1beta3 API object
	// TemplateRef *kapi.ObjectReference `json:"templateRef,omitempty" description:"a reference to an object that describes the pod that will be created if insufficient replicas are detected; ignored if template is set"`

	// Template is the object that describes the pod that will be created if
	// insufficient replicas are detected. Internally, this takes precedence over a
	// TemplateRef.
	// Must be set before converting to a v1beta1 or v1beta2 API object.
	Template *kapi.PodTemplateSpec `json:"template,omitempty" description:"describes the pod that will be created if insufficient replicas are detected; takes precedence over a template reference"`
}

type DeploymentConfigStatus struct {
	// LatestVersion is used to determine whether the current deployment associated with a DeploymentConfig
	// is out of sync.
	LatestVersion int `json:"latestVersion,omitempty" description:"used to determine whether the current deployment is out of sync"`
	// The reasons for the update to this deployment config.
	// This could be based on a change made by the user or caused by an automatic trigger
	Details *DeploymentDetails `json:"details,omitempty" description:"reasons for the last update to the config"`
}

// DeploymentTriggerPolicy describes a policy for a single trigger that results in a new deployment.
type DeploymentTriggerPolicy struct {
	Type DeploymentTriggerType `json:"type,omitempty" description:"the type of the trigger"`
	// ImageChangeParams represents the parameters for the ImageChange trigger.
	ImageChangeParams *DeploymentTriggerImageChangeParams `json:"imageChangeParams,omitempty" description:"input to the ImageChange trigger"`
}

// DeploymentTriggerType refers to a specific DeploymentTriggerPolicy implementation.
type DeploymentTriggerType string

const (
	// DeploymentTriggerOnImageChange will create new deployments in response to updated tags from
	// a Docker image repository.
	DeploymentTriggerOnImageChange DeploymentTriggerType = "ImageChange"
	// DeploymentTriggerOnConfigChange will create new deployments in response to changes to
	// the ControllerTemplate of a DeploymentConfig.
	DeploymentTriggerOnConfigChange DeploymentTriggerType = "ConfigChange"
)

// DeploymentTriggerImageChangeParams represents the parameters to the ImageChange trigger.
type DeploymentTriggerImageChangeParams struct {
	// Automatic means that the detection of a new tag value should result in a new deployment.
	Automatic bool `json:"automatic,omitempty" description:"whether detection of a new tag value should trigger a deployment"`
	// ContainerNames is used to restrict tag updates to the specified set of container names in a pod.
	ContainerNames []string `json:"containerNames,omitempty" description:"restricts tag updates to a set of container names in the pod"`
	// From is a reference to a Docker image repository tag to watch for changes. The
	// Kind may be left blank, in which case it defaults to "ImageStreamTag". The "Name" is
	// the only required subfield - if Namespace is blank, the namespace of the current deployment
	// trigger will be used.
	From kapi.ObjectReference `json:"from" description:"a reference to an ImageRepository, ImageStream, or ImageStreamTag to watch for changes"`
	// LastTriggeredImage is the last image to be triggered.
	LastTriggeredImage string `json:"lastTriggeredImage" description:"the last image to be triggered"`
}

// DeploymentDetails captures information about the causes of a deployment.
type DeploymentDetails struct {
	// The user specified change message, if this deployment was triggered manually by the user
	Message string `json:"message,omitempty" description:"a user specified change message"`
	// Extended data associated with all the causes for creating a new deployment
	Causes []*DeploymentCause `json:"causes,omitempty" description:"extended data associated with all the causes for creating a new deployment"`
}

// DeploymentCause captures information about a particular cause of a deployment.
type DeploymentCause struct {
	// The type of the trigger that resulted in the creation of a new deployment
	Type DeploymentTriggerType `json:"type" description:"the type of trigger that resulted in a new deployment"`
	// The image trigger details, if this trigger was fired based on an image change
	ImageTrigger *DeploymentCauseImageTrigger `json:"imageTrigger,omitempty" description:"image trigger details (if applicable)"`
}

// DeploymentCauseImageTrigger represents details about the cause of a deployment originating
// from an image change trigger
type DeploymentCauseImageTrigger struct {
	// From is a reference to the changed object which triggered a deployment. The field may have
	// the kinds DockerImage, ImageStreamTag, or ImageStreamImage.
	From kapi.ObjectReference `json:"from" description:"a reference the changed object which triggered a deployment"`
}

// A DeploymentConfigList is a collection of deployment configs.
type DeploymentConfigList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	Items                []DeploymentConfig `json:"items" description:"a list of deployment configs"`
}

// DeploymentConfigRollback provides the input to rollback generation.
type DeploymentConfigRollback struct {
	unversioned.TypeMeta `json:",inline"`
	// Spec defines the options to rollback generation.
	Spec DeploymentConfigRollbackSpec `json:"spec" description:"options for rollback generation"`
}

// DeploymentConfigRollbackSpec represents the options for rollback generation.
type DeploymentConfigRollbackSpec struct {
	// From points to a ReplicationController which is a deployment.
	From kapi.ObjectReference `json:"from" description:"a reference to a deployment, which is a ReplicationController"`
	// IncludeTriggers specifies whether to include config Triggers.
	IncludeTriggers bool `json:"includeTriggers" description:"whether to include old config triggers in the rollback"`
	// IncludeTemplate specifies whether to include the PodTemplateSpec.
	IncludeTemplate bool `json:"includeTemplate" description:"whether to include the old pod template spec in the rollback"`
	// IncludeReplicationMeta specifies whether to include the replica count and selector.
	IncludeReplicationMeta bool `json:"includeReplicationMeta" description:"whether to include the replica count and replica selector in the rollback"`
	// IncludeStrategy specifies whether to include the deployment Strategy.
	IncludeStrategy bool `json:"includeStrategy" description:"whether to include the deployment strategy in the rollback"`
}

// DeploymentLog represents the logs for a deployment
type DeploymentLog struct {
	unversioned.TypeMeta `json:",inline"`
}

// DeploymentLogOptions is the REST options for a deployment log
type DeploymentLogOptions struct {
	unversioned.TypeMeta `json:",inline"`

	// The container for which to stream logs. Defaults to only container if there is one container in the pod.
	Container string `json:"container,omitempty" description:"the container for which to stream logs; defaults to only container if there is one container in the pod"`
	// Follow if true indicates that the build log should be streamed until
	// the build terminates.
	Follow bool `json:"follow,omitempty" description:"if true indicates that the log should be streamed; defaults to false"`
	// Return previous terminated container logs. Defaults to false.
	Previous bool `json:"previous,omitempty" description:"return previous terminated container logs; defaults to false."`
	// A relative time in seconds before the current time from which to show logs. If this value
	// precedes the time a pod was started, only logs since the pod start will be returned.
	// If this value is in the future, no logs will be returned.
	// Only one of sinceSeconds or sinceTime may be specified.
	SinceSeconds *int64 `json:"sinceSeconds,omitempty" description:"relative time in seconds before the current time from which to show logs"`
	// An RFC3339 timestamp from which to show logs. If this value
	// preceeds the time a pod was started, only logs since the pod start will be returned.
	// If this value is in the future, no logs will be returned.
	// Only one of sinceSeconds or sinceTime may be specified.
	SinceTime *unversioned.Time `json:"sinceTime,omitempty" description:"relative time in seconds before the current time from which to show logs"`
	// If true, add an RFC3339 or RFC3339Nano timestamp at the beginning of every line
	// of log output. Defaults to false.
	Timestamps bool `json:"timestamps,omitempty" description:"add an RFC3339 or RFC3339Nano timestamp at the beginning of every line of log output"`
	// If set, the number of lines from the end of the logs to show. If not specified,
	// logs are shown from the creation of the container or sinceSeconds or sinceTime
	TailLines *int64 `json:"tailLines,omitempty" description:"the number of lines from the end of the logs to show"`
	// If set, the number of bytes to read from the server before terminating the
	// log output. This may not display a complete final line of logging, and may return
	// slightly more or slightly less than the specified limit.
	LimitBytes *int64 `json:"limitBytes,omitempty" description:"the number of bytes to read from the server before terminating the log output"`

	// NoWait if true causes the call to return immediately even if the deployment
	// is not available yet. Otherwise the server will wait until the deployment has started.
	NoWait bool `json:"nowait,omitempty" description:"if true indicates that the server should not wait for a log to be available before returning; defaults to false"`

	// Version of the deployment for which to view logs.
	Version *int64 `json:"version,omitempty" description:"the version of the deployment for which to view logs"`
}
