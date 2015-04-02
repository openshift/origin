package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// A deployment represents a single configuration of a pod deployed into the cluster, and may
// represent both a current deployment or a historical deployment.
//
// DEPRECATED: This type longer drives any system behavior. Deployments are now represented directly
// by ReplicationControllers. Use DeploymentConfig to drive deployments.
type Deployment struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// Strategy describes how a deployment is executed.
	Strategy DeploymentStrategy `json:"strategy,omitempty"`
	// ControllerTemplate is the desired replication state the deployment works to materialize.
	ControllerTemplate kapi.ReplicationControllerSpec `json:"controllerTemplate,omitempty"`
	// Status is the execution status of the deployment.
	Status DeploymentStatus `json:"status,omitempty"`
	// Details captures the causes for the creation of this deployment resource.
	// This could be based on a change made by the user to the deployment config
	// or caused by an automatic trigger that was specified in the deployment config.
	// Multiple triggers could have caused this deployment.
	// If no trigger is specified here, then the deployment was likely created as a result of an
	// explicit client request to create a new deployment resource.
	Details *DeploymentDetails `json:"details,omitempty"`
}

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
}

// DeploymentStrategyType refers to a specific DeploymentStrategy implementation.
type DeploymentStrategyType string

const (
	// DeploymentStrategyTypeRecreate is a simple strategy suitable as a default.
	DeploymentStrategyTypeRecreate DeploymentStrategyType = "Recreate"
	// DeploymentStrategyTypeCustom is a user defined strategy.
	DeploymentStrategyTypeCustom DeploymentStrategyType = "Custom"
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

// A DeploymentList is a collection of deployments.
// DEPRECATED: Like Deployment, this is no longer used.
type DeploymentList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []Deployment `json:"items"`
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
)

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
	ControllerTemplate kapi.ReplicationControllerSpec `json:"controllerTemplate,omitempty"`
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
	// DEPRECATED: will be removed in v1beta2.
	RepositoryName string `json:"repositoryName,omitempty"`
	// From is a reference to a Docker image repository to watch for changes. This field takes
	// precedence over RepositoryName, which is deprecated and will be removed in v1beta2. The
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
