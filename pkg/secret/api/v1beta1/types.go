package v1beta1

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"
)

// Type of secret data
type SecretType string

// Valid values for SecretType
const (
	// TextSecretType is a secret consisting of text data
	TextSecretType SecretType = "text"

	// Base64SecretType is a secret encoded in base64
	Base64SecretType SecretType = "base64"
)

// Secret is a resource that contains a key, password, etc to be used by pods
type Secret struct {
	kapi.TypeMeta   `json:",inline" yaml:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Type SecretType `json:"type" yaml:"type"`
	Data []string   `json:"data" yaml:"data"`
}

// SecretList is a collection of Secrets.
type SecretList struct {
	kapi.TypeMeta `json:",inline" yaml:",inline"`
	kapi.ListMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Items         []Secret `json:"items" yaml:"items"`
}
