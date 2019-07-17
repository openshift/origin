package config

import "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BootstrapIdentityProvider serves as a marker for an "IDP" that is backed by osin
// this allows us to reuse most of the logic from existing identity providers
type BootstrapIdentityProvider struct {
	v1.TypeMeta
}
