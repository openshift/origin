package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// IdentityProvider holds cluster-wide information about IdentityProvider.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type IdentityProvider struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec IdentityProviderSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status IdentityProviderStatus `json:"status"`
}

type IdentityProviderSpec struct {
	// all the IDP settings
}

type IdentityProviderStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type IdentityProviderList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IdentityProvider `json:"items"`
}
