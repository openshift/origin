package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// ProjectList is a list of Project objects.
type ProjectList struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:",inline" yaml:",inline"`
	Items           []Project `json:"items,omitempty" yaml:"items,omitempty"`
}

// Project is a logical top-level container for a set of origin resources
type Project struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:",inline" yaml:",inline"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	DisplayName     string            `json:"displayName,omitempty" yaml:"displayName,omitempty"`
	Description     string            `json:"description,omitempty" yaml:"description,omitempty"`
}
