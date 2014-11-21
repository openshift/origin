package v1beta1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// A deployment represents a single configuration of a pod deployed into the cluster, and may
// represent both a current deployment or a historical deployment.
type Deployment struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Strategy describes how a deployment is executed.
	Strategy DeploymentStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	// ControllerTemplate is the desired replication state the deployment works to materialize.
	ControllerTemplate kapi.ReplicationControllerState `json:"controllerTemplate,omitempty" yaml:"controllerTemplate,omitempty"`
	// Status is the execution status of the deployment.
	Status DeploymentStatus `json:"status,omitempty" yaml:"status,omitempty"`
	// Details captures the causes for the creation of this deployment resource.
	// This could be based on a change made by the user to the deployment config
	// or caused by an automatic trigger that was specified in the deployment config.
	// Multiple triggers could have caused this deployment.
	// If no trigger is specified here, then the deployment was likely created as a result of an
	// explicit client request to create a new deployment resource.
	Details *DeploymentDetails `json:"details,omitempty" yaml:"details,omitempty"`
}

// DeploymentStatus decribes the possible states a Deployment can be in.
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
	Type DeploymentStrategyType `json:"type,omitempty" yaml:"type,omitempty"`
	// CustomParams are the input to the Custom deployment strategy.
	CustomParams *CustomDeploymentStrategyParams `json:"customParams,omitempty" yaml:"customParams,omitempty"`
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
	Image string `json:"image,omitempty" yaml:"image,omitempty"`
	// Environment holds the environment which will be given to the container for Image.
	Environment []kapi.EnvVar `json:"environment,omitempty" yaml:"environment,omitempty"`
	// Command is optional and overrides CMD in the container Image.
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
}

// A DeploymentList is a collection of deployments.
type DeploymentList struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
	kapi.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items         []Deployment `json:"items,omitempty" yaml:"items,omitempty"`
}

// These constants represent annotation keys used for correlating objects related to deployments.
const (
	DeploymentConfigAnnotation = "deploymentConfig"
	DeploymentAnnotation       = "deployment"
	DeploymentPodAnnotation    = "pod"
)

// These constants represent label keys used for correlating objects related to deployment.
const (
	DeploymentConfigLabel = "deploymentconfig"
)

// DeploymentConfig represents a configuration for a single deployment of a replication controller:
// what the template is for the deployment, how new deployments are triggered, what the desired
// deployment state is.
type DeploymentConfig struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	// Triggers determine how updates to a DeploymentConfig result in new deployments. If no triggers
	// are defined, a new deployment can only occur as a result of an explicit client update to the
	// DeploymentConfig with a new LatestVersion.
	Triggers []DeploymentTriggerPolicy `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	// Template represents a desired deployment state and how to deploy it.
	Template DeploymentTemplate `json:"template,omitempty" yaml:"template,omitempty"`
	// LatestVersion is used to determine whether the current deployment associated with a DeploymentConfig
	// is out of sync.
	LatestVersion int `json:"latestVersion,omitempty" yaml:"latestVersion,omitempty"`
	// The reasons for the update to this deployment config.
	// This could be based on a change made by the user or caused by an automatic trigger
	Details *DeploymentDetails `json:"details,omitempty" yaml:"details,omitempty"`
}

// DeploymentTemplate templatizes the configurable fields of a Deployment.
type DeploymentTemplate struct {
	// Strategy describes how a deployment is executed.
	Strategy DeploymentStrategy `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	// ControllerTemplate is the desired replication state the deployment works to materialize.
	ControllerTemplate kapi.ReplicationControllerState `json:"controllerTemplate,omitempty" yaml:"controllerTemplate,omitempty"`
}

// DeploymentTriggerPolicy describes a policy for a single trigger that results in a new Deployment.
type DeploymentTriggerPolicy struct {
	Type DeploymentTriggerType `json:"type,omitempty" yaml:"type,omitempty"`
	// ImageChangeParams represents the parameters for the ImageChange trigger.
	ImageChangeParams *DeploymentTriggerImageChangeParams `json:"imageChangeParams,omitempty" yaml:"imageChangeParams,omitempty"`
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
	Automatic bool `json:"automatic,omitempty" yaml:"automatic,omitempty"`
	// ContainerNames is used to restrict tag updates to the specified set of container names in a pod.
	ContainerNames []string `json:"containerNames,omitempty" yaml:"containerNames,omitempty"`
	// RepositoryName is the identifier for a Docker image repository to watch for changes.
	RepositoryName string `json:"repositoryName,omitempty" yaml:"repositoryName,omitempty"`
	// Tag is the name of an image repository tag to watch for changes.
	Tag string `json:"tag,omitempty" yaml:"tag,omitempty"`
}

// DeploymentDetails captures information about the causes of a deployment.
type DeploymentDetails struct {
	// The user specified change message, if this deployment was triggered manually by the user
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	// Extended data associated with all the causes for creating a new deployment
	Causes []*DeploymentCause `json:"causes,omitempty" yaml:"causes,omitempty"`
}

// DeploymentCause captures information about a particular cause of a deployment.
type DeploymentCause struct {
	// The type of the trigger that resulted in the creation of a new deployment
	Type DeploymentTriggerType `json:"type" yaml:"type"`
	// The image trigger details, if this trigger was fired based on an image change
	ImageTrigger *DeploymentCauseImageTrigger `json:"imageTrigger,omitempty" yaml:"imageTrigger,omitempty"`
}

type DeploymentCauseImageTrigger struct {
	// RepositoryName is the identifier for a Docker image repository that was updated.
	RepositoryName string `json:"repositoryName,omitempty" yaml:"repositoryName,omitempty"`
	// Tag is the name of an image repository tag that is now pointing to a new image.
	Tag string `json:"tag,omitempty" yaml:"tag,omitempty"`
}

// A DeploymentConfigList is a collection of deployment configs.
type DeploymentConfigList struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
	kapi.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items         []DeploymentConfig `json:"items,omitempty" yaml:"items,omitempty"`
}
