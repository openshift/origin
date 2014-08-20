package template

import (
	"math/rand"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

type (
	Uri    string
	PValue string
)

type Template struct {
	api.JSONBase      `json:",inline" yaml:",inline"`
	BuildConfig       []BuildConfig      `json:"buildConfigs" yaml:"buildConfigs"`
	ImageRepositories []ImageRepository  `json:"imageRepositories" yaml:"imageRepositories"`
	Parameters        []Parameter        `json:"parameters" yaml:"parameters"`
	Services          []api.Service      `json:"services" yaml:"services"`
	DeploymentConfigs []DeploymentConfig `json:"deploymentConfigs" yaml:"deploymentConfigs"`
	Seed              *rand.Rand
}

type ImageRepositoryList struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Items        []ImageRepository `json:"items,omitempty" yaml:"items,omitempty"`
}

type ImageRepository struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Name         string `json:"name" yaml:"name"`
	Url          Uri    `json:"url" yaml:"url"`
}

type BuildConfigList struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Items        []BuildConfig `json:"items,omitempty" yaml:"items,omitempty"`
}

type BuildConfig struct {
	Name            string `json:"name" yaml:"name"`
	Type            string `json:"type" yaml:"type"`
	SourceUri       Uri    `json:"sourceUri" yaml:"sourceUri"`
	ImageRepository string `json:"imageRepository" yaml:"imageRepository"`
}

type ParameterList struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Items        []Parameter `json:"items,omitempty" yaml:"items,omitempty"`
}

type Parameter struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Type        string `json:"type" yaml:"type"`
	Generate    string `json:"generate" yaml:"generate"`
	Value       string `json:"value" yaml:"value"`

	Seed *rand.Rand
}

type Env []struct {
	Name  string `json:"name" yaml:"name"`
	Value PValue `json:"value" yaml:"value"`
}

type DeploymentConfigList struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Items        []DeploymentConfig `json:"items,omitempty" yaml:"items,omitempty"`
}

type DeploymentConfig struct {
	api.JSONBase `json:",inline" yaml:",inline"`
	Labels       map[string]string              `json:"labels,omitempty" yaml:"labels,omitempty"`
	DesiredState api.ReplicationControllerState `json:"desiredState" yaml:"desiredState"`
}
