package v1beta1

import (
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// ProjectList is a list of Project objects.
type ProjectList struct {
	kubeapi.JSONBase `json:",inline" yaml:",inline"`
	Items            []Project `json:"items,omitempty" yaml:"items,omitempty"`
}

// Project is a logical top-level container for a set of origin resources
type Project struct {
	kubeapi.JSONBase `json:",inline" yaml:",inline"`
	Labels           map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	DisplayName      string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Description      string            `json:"description,omitempty" yaml:"description,omitempty"`
}
