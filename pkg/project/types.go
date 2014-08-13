package project

import "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

type (
	Uri    string
	PValue string
)

type Project struct {
	api.JSONBase    `json:",inline" yaml:",inline"`
	BuildConfig     []BuildConfig     `json:"buildConfig" yaml:"buildConfig"`
	ImageRepository []ImageRepository `json:"imageRepository" yaml:"imageRepository"`
	Parameters      []Parameter       `json:"parameters" yaml:"parameters"`
	ServiceLinks    []ServiceLink     `json:"serviceLinks" yaml:"serviceLinks"`
	Services        []Service         `json:"services" yaml:"services"`
}

type ImageRepository struct {
	Name string `json:"name" yaml:"name"`
	Url  Uri    `json:"url" yaml:"url"`
}

type BuildConfig struct {
	Name            string `json:"name" yaml:"name"`
	Type            string `json:"type" yaml:"type"`
	SourceUri       Uri    `json:"sourceUri" yaml:"sourceUri"`
	ImageRepository string `json:"imageRepository" yaml:"imageRepository"`
}

type Parameter struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Type        string `json:"type" yaml:"type"`
	Generate    string `json:"generate" yaml:"generate"`
	Value       string `json:"value" yaml:"value"`
}

type Env []struct {
	Name  string `json:"name" yaml:"name"`
	Value PValue `json:"value" yaml:"value"`
}

type ServiceLink struct {
	From   string `json:"from" yaml:"from"`
	To     string `json:"to" yaml:"to"`
	Export Env    `json:"export" yaml:"export"`
}

type DeploymentConfig struct {
	Deployment Deployment `json:"deployment" yaml:"deployment"`
}

type Deployment struct {
	PodTemplate PodTemplate `json:"podTemplate" yaml:"podTemplate"`
}

type PodTemplate struct {
	Containers []Container `json:"containers" yaml:"containers"`
	Replicas   int         `json:"replicas" yaml:"replicas"`
}

type Image struct {
	Name string `json:"name" yaml:"name"`
	Tag  string `json:"tag" yaml:"tag"`
}

type ContainerPort struct {
	ContainerPort int `json:"containerPort" yaml:"containerPort"`
	HostPort      int `json:"hostPort" yaml:"hostPort"`
}

type Container struct {
	Name  string          `json:"name" yaml:"name"`
	Image Image           `json:"image" yaml:"image"`
	Env   Env             `json:"env" yaml:"env"`
	Ports []ContainerPort `json:"ports" yaml:"ports"`
}

type Service struct {
	Name             string            `json:"name" yaml:"name"`
	Description      string            `json:"description" yaml:"description"`
	Labels           map[string]PValue `json:"labels" yaml:"labels"`
	DeploymentConfig DeploymentConfig  `json:"deploymentConfig" yaml:"deploymentConfig"`
}
