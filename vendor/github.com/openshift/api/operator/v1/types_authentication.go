package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Authentication provides information to configure an operator to manage authentication.
type Authentication struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:Required
	// +required
	Spec AuthenticationSpec `json:"spec,omitempty"`
	// +optional
	Status AuthenticationStatus `json:"status,omitempty"`
}

type AuthenticationSpec struct {
	OperatorSpec `json:",inline"`
}

type AuthenticationStatus struct {
	// ManagingOAuthAPIServer indicates whether this operator is managing OAuth related APIs. Setting this field to true will cause OAS-O to step down.
	// Note that this field will be removed in the future releases, once https://github.com/openshift/enhancements/blob/master/enhancements/authentication/separate-oauth-resources.md is fully implemented
	// +optional
	ManagingOAuthAPIServer bool `json:"managingOAuthAPIServer,omitempty"`

	// OAuthAPIServer holds status specific only to oauth-apiserver
	// +optional
	OAuthAPIServer OAuthAPIServerStatus `json:"oauthAPIServer,omitempty"`

	OperatorStatus `json:",inline"`
}

type OAuthAPIServerStatus struct {
	// LatestAvailableRevision is the latest revision used as suffix of revisioned
	// secrets like encryption-config. A new revision causes a new deployment of pods.
	// +optional
	// +kubebuilder:validation:Minimum=0
	LatestAvailableRevision int32 `json:"latestAvailableRevision,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AuthenticationList is a collection of items
type AuthenticationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Authentication `json:"items"`
}
