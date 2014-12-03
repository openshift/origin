package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// ProjectList is a list of Project objects.
type ProjectList struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
	kapi.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items         []Project `json:"items" yaml:"items"`
}

// Project is a logical top-level container for a set of origin resources
type Project struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	DisplayName     string `json:"displayName,omitempty" yaml:"displayName,omitempty"`
}
