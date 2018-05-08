package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	webconsolev1 "github.com/openshift/api/webconsole/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ServiceServingCertSignerConfig struct {
	metav1.TypeMeta `json:",inline"`

	// ServingInfo is the HTTP serving information for the controller's endpoints
	ServingInfo webconsolev1.HTTPServingInfo `json:"servingInfo"`

	// authentication allows configuration of authentication for the endpoints
	Authentication DelegatedAuthentication `json:"authentication,omitempty"`
	// authorization allows configuration of authentication for the endpoints
	Authorization DelegatedAuthorization `json:"authorization,omitempty"`

	// Signer holds the signing information used to automatically sign serving certificates.
	Signer webconsolev1.CertInfo `json:"signer"`
}

type DelegatedAuthentication struct {
	// disabled indicates that authentication should be disabled
	Disabled bool `json:"disabled,omitempty"`
}

type DelegatedAuthorization struct {
	// disabled indicates that authentication should be disabled
	Disabled bool `json:"disabled,omitempty"`
}
