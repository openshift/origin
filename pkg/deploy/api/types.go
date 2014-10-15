package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// CustomPodDeploymentStrategy describes a deployment carried out by a custom pod.
type CustomPodDeploymentStrategy struct {
	Image       string       `json:"image,omitempty" yaml:"image,omitempty"`
	Environment []api.EnvVar `json:"environment,omitempty" yaml:"environment,omitempty"`
}

// DeploymentStrategy describes how to perform a deployment.
type DeploymentStrategy struct {
	Type      string                       `json:"type,omitempty" yaml:"type,omitempty"`
	CustomPod *CustomPodDeploymentStrategy `json:"customPod,omitempty" yaml:"customPod,omitempty"`
}

// DeploymentTemplate contains all the necessary information to create a Deployment from a
// DeploymentStrategy.
type DeploymentTemplate struct {
	Strategy           DeploymentStrategy             `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	ControllerTemplate api.ReplicationControllerState `json:"controllerTemplate,omitempty" yaml:"controllerTemplate,omitempty"`
}

// DeploymentState decribes the possible states a Deployment can be in.
type DeploymentState string

// TODO: review which of these states are needed.
const (
	DeploymentStateNew       DeploymentState = "New"
	DeploymentStatePending   DeploymentState = "Pending"
	DeploymentStateRunning   DeploymentState = "Running"
	DeploymentStateComplete  DeploymentState = "Complete"
	DeploymentStateFailed    DeploymentState = "Failed"
	DeploymentStateCancelled DeploymentState = "Cancelled"
)

// A Deployment represents a single unique realization of a DeploymentConfig.
type Deployment struct {
	api.JSONBase       `json:",inline" yaml:",inline"`
	Labels             map[string]string              `json:"labels,omitempty" yaml:"labels,omitempty"`
	Strategy           DeploymentStrategy             `json:"strategy,omitempty" yaml:"strategy,omitempty"`
	ControllerTemplate api.ReplicationControllerState `json:"controllerTemplate,omitempty" yaml:"controllerTemplate,omitempty"`
	State              DeploymentState                `json:"state,omitempty" yaml:"state,omitempty"`
}

// DeploymentConfigIDLabel is the key of a Deployment label whose value is the ID of a DeploymentConfig
// on which the Deployment is based.
const DeploymentConfigIDLabel = "deploymentConfigID"

// DeploymentTriggerPolicy describes a policy for a single trigger that results in a new Deployment.
type DeploymentTriggerPolicy struct {
	Type              DeploymentTriggerType               `json:"type,omitempty" yaml:"type,omitempty"`
	ImageChangeParams *DeploymentTriggerImageChangeParams `json:"imageChangeParams,omitempty" yaml:"imageChangeParams,omitempty"`
}

type DeploymentTriggerImageChangeParams struct {
	Automatic      bool     `json:"automatic,omitempty" yaml:"automatic,omitempty"`
	ContainerNames []string `json:"containerNames,omitempty" yaml:"containerNames,omitempty"`
	RepositoryName string   `json:"repositoryName,omitempty" yaml:"repositoryName,omitempty"`
	Tag            string   `json:"tag,omitempty" yaml:"tag,omitempty"`
}

type DeploymentTriggerType string

const (
	DeploymentTriggerManual         DeploymentTriggerType = "Manual"
	DeploymentTriggerOnImageChange  DeploymentTriggerType = "ImageChange"
	DeploymentTriggerOnConfigChange DeploymentTriggerType = "ConfigChange"
)

// DeploymentConfig represents a configuration for a single deployment of a replication controller:
// what the template for the deployment, how new deployments are triggered, what the current
// deployed state is.
type DeploymentConfig struct {
	api.JSONBase  `json:",inline" yaml:",inline"`
	Labels        map[string]string         `json:"labels,omitempty" yaml:"labels,omitempty"`
	Triggers      []DeploymentTriggerPolicy `json:"triggers,omitempty" yaml:"triggers,omitempty"`
	Template      DeploymentTemplate        `json:"template,omitempty" yaml:"template,omitempty"`
	LatestVersion int                       `json:"latestVersion,omitempty" yaml:"latestVersion,omitempty"`
}

// A DeploymentConfigList is a collection of deployment configs
type DeploymentConfigList struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Items        []DeploymentConfig `json:"items,omitempty" yaml:"items,omitempty"`
}

// A DeploymentList is a collection of deployments.
type DeploymentList struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Items        []Deployment `json:"items,omitempty" yaml:"items,omitempty"`
}
