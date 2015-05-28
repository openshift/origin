package v1beta1

import (
	v1beta1 "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta1"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
)

// DeploymentStatus describes the possible states a deployment can be in.
type DeploymentStatus string

const (
	// DeploymentStatusNew means the deployment has been accepted but not yet acted upon.
	DeploymentStatusNew DeploymentStatus = "New"
	// DeploymentStatusPending means the deployment been handed over to a deployment strategy,
	// but the strategy has not yet declared the deployment to be running.
	DeploymentStatusPending DeploymentStatus = "Pending"
	// DeploymentStatusRunning means the deployment strategy has reported the deployment as
	// being in-progress.
	DeploymentStatusRunning DeploymentStatus = "Running"
	// DeploymentStatusComplete means the deployment finished without an error.
	DeploymentStatusComplete DeploymentStatus = "Complete"
	// DeploymentStatusFailed means the deployment finished with an error.
	DeploymentStatusFailed DeploymentStatus = "Failed"
)

// DeploymentStrategy describes how to perform a deployment.
type DeploymentStrategy struct {
	// Type is the name of a deployment strategy.
	Type DeploymentStrategyType `json:"type,omitempty"`
	// CustomParams are the input to the Custom deployment strategy.
	CustomParams *CustomDeploymentStrategyParams `json:"customParams,omitempty"`
	// RecreateParams are the input to the Recreate deployment strategy.
	RecreateParams *RecreateDeploymentStrategyParams `json:"recreateParams,omitempty"`
	// RollingParams are the input to the Rolling deployment strategy.
	RollingParams *RollingDeploymentStrategyParams `json:"rollingParams,omitempty"`
	// Compute resource requirements to execute the deployment
	Resources kapi.ResourceRequirements `json:"resources,omitempty"`
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
	Image string `json:"image,omitempty"`
	// Environment holds the environment which will be given to the container for Image.
	Environment []kapi.EnvVar `json:"environment,omitempty"`
	// Command is optional and overrides CMD in the container Image.
	Command []string `json:"command,omitempty"`
}

// RecreateDeploymentStrategyParams are the input to the Recreate deployment
// strategy.
type RecreateDeploymentStrategyParams struct {
	// Pre is a lifecycle hook which is executed before the strategy manipulates
	// the deployment. All LifecycleHookFailurePolicy values are supported.
	Pre *LifecycleHook `json:"pre,omitempty"`
	// Post is a lifecycle hook which is executed after the strategy has
	// finished all deployment logic. The LifecycleHookFailurePolicyAbort policy
	// is NOT supported.
	Post *LifecycleHook `json:"post,omitempty"`
}

// Handler defines a specific deployment lifecycle action.
type LifecycleHook struct {
	// FailurePolicy specifies what action to take if the hook fails.
	FailurePolicy LifecycleHookFailurePolicy `json:"failurePolicy"`
	// ExecNewPod specifies the options for a lifecycle hook backed by a pod.
	ExecNewPod *ExecNewPodHook `json:"execNewPod,omitempty"`
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
	Command []string `json:"command"`
	// Env is a set of environment variables to supply to the hook pod's container.
	Env []kapi.EnvVar `json:"env,omitempty"`
	// ContainerName is the name of a container in the deployment pod template
	// whose Docker image will be used for the hook pod's container.
	ContainerName string `json:"containerName"`
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
	DeploymentConfigAnnotation = "deploymentConfig"
	// DeploymentAnnotation is an annotation on a deployer Pod. The annotation value is the name
	// of the deployment (a ReplicationController) on which the deployer Pod acts.
	DeploymentAnnotation = "deployment"
	// DeploymentPodAnnotation is an annotation on a deployment (a ReplicationController). The
	// annotation value is the name of the deployer Pod which will act upon the ReplicationController
	// to implement the deployment behavior.
	DeploymentPodAnnotation = "pod"
	// DeploymentStatusAnnotation is an annotation name used to retrieve the DeploymentStatus of
	// a deployment.
	DeploymentStatusAnnotation = "deploymentStatus"
	// DeploymentEncodedConfigAnnotation is an annotation name used to retrieve specific encoded
	// DeploymentConfig on which a given deployment is based.
	DeploymentEncodedConfigAnnotation = "encodedDeploymentConfig"
	// DeploymentVersionAnnotation is an annotation on a deployment (a ReplicationController). The
	// annotation value is the LatestVersion value of the DeploymentConfig which was the basis for
	// the deployment.
	DeploymentVersionAnnotation = "deploymentVersion"
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
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`
	// Triggers determine how updates to a DeploymentConfig result in new deployments. If no triggers
	// are defined, a new deployment can only occur as a result of an explicit client update to the
	// DeploymentConfig with a new LatestVersion.
	Triggers []DeploymentTriggerPolicy `json:"triggers,omitempty"`
	// Template represents a desired deployment state and how to deploy it.
	Template DeploymentTemplate `json:"template,omitempty"`
	// LatestVersion is used to determine whether the current deployment associated with a DeploymentConfig
	// is out of sync.
	LatestVersion int `json:"latestVersion,omitempty"`
	// The reasons for the update to this deployment config.
	// This could be based on a change made by the user or caused by an automatic trigger
	Details *DeploymentDetails `json:"details,omitempty"`
}

// DeploymentTemplate contains all the necessary information to create a deployment from a
// DeploymentStrategy.
type DeploymentTemplate struct {
	// Strategy describes how a deployment is executed.
	Strategy DeploymentStrategy `json:"strategy,omitempty"`
	// ControllerTemplate is the desired replication state the deployment works to materialize.
	ControllerTemplate v1beta1.ReplicationControllerState `json:"controllerTemplate,omitempty"`
}

// DeploymentTriggerPolicy describes a policy for a single trigger that results in a new deployment.
type DeploymentTriggerPolicy struct {
	Type DeploymentTriggerType `json:"type,omitempty"`
	// ImageChangeParams represents the parameters for the ImageChange trigger.
	ImageChangeParams *DeploymentTriggerImageChangeParams `json:"imageChangeParams,omitempty"`
}

// DeploymentTriggerType refers to a specific DeploymentTriggerPolicy implementation.
type DeploymentTriggerType string

const (
	// DeploymentTriggerManual is a placeholder implementation which does nothing.
	DeploymentTriggerManual DeploymentTriggerType = "Manual"
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
	Automatic bool `json:"automatic,omitempty"`
	// ContainerNames is used to restrict tag updates to the specified set of container names in a pod.
	ContainerNames []string `json:"containerNames,omitempty"`
	// RepositoryName is the identifier for a Docker image repository to watch for changes.
	// DEPRECATED: will be removed in v1beta3.
	RepositoryName string `json:"repositoryName,omitempty"`
	// From is a reference to a Docker image repository to watch for changes. This field takes
	// precedence over RepositoryName, which is deprecated and will be removed in v1beta3. The
	// Kind may be left blank, in which case it defaults to "ImageRepository". The "Name" is
	// the only required subfield - if Namespace is blank, the namespace of the current deployment
	// trigger will be used.
	From kapi.ObjectReference `json:"from"`
	// Tag is the name of an image repository tag to watch for changes.
	Tag string `json:"tag,omitempty"`
	// LastTriggeredImage is the last image to be triggered.
	LastTriggeredImage string `json:"lastTriggeredImage"`
}

// DeploymentDetails captures information about the causes of a deployment.
type DeploymentDetails struct {
	// The user specified change message, if this deployment was triggered manually by the user
	Message string `json:"message,omitempty"`
	// Extended data associated with all the causes for creating a new deployment
	Causes []*DeploymentCause `json:"causes,omitempty"`
}

// DeploymentCause captures information about a particular cause of a deployment.
type DeploymentCause struct {
	// The type of the trigger that resulted in the creation of a new deployment
	Type DeploymentTriggerType `json:"type"`
	// The image trigger details, if this trigger was fired based on an image change
	ImageTrigger *DeploymentCauseImageTrigger `json:"imageTrigger,omitempty"`
}

type DeploymentCauseImageTrigger struct {
	// RepositoryName is the identifier for a Docker image repository that was updated.
	RepositoryName string `json:"repositoryName,omitempty"`
	// Tag is the name of an image repository tag that is now pointing to a new image.
	Tag string `json:"tag,omitempty"`
}

// A DeploymentConfigList is a collection of deployment configs.
type DeploymentConfigList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []DeploymentConfig `json:"items"`
}

// DeploymentConfigRollback provides the input to rollback generation.
type DeploymentConfigRollback struct {
	kapi.TypeMeta `json:",inline"`
	// Spec defines the options to rollback generation.
	Spec DeploymentConfigRollbackSpec `json:"spec"`
}

// DeploymentConfigRollbackSpec represents the options for rollback generation.
type DeploymentConfigRollbackSpec struct {
	// From points to a ReplicationController which is a deployment.
	From kapi.ObjectReference `json:"from"`
	// IncludeTriggers specifies whether to include config Triggers.
	IncludeTriggers bool `json:"includeTriggers"`
	// IncludeTemplate specifies whether to include the PodTemplateSpec.
	IncludeTemplate bool `json:"includeTemplate"`
	// IncludeReplicationMeta specifies whether to include the replica count and selector.
	IncludeReplicationMeta bool `json:"includeReplicationMeta"`
	// IncludeStrategy specifies whether to include the deployment Strategy.
	IncludeStrategy bool `json:"includeStrategy"`
}
