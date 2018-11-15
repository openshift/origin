package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Authentication holds cluster-wide information about Authentication.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type Authentication struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec AuthenticationSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status AuthenticationStatus `json:"status"`
}

type AuthenticationSpec struct {
	// webhook token auth config (ttl)
	// external token address
	// serviceAccountOAuthGrantMethod or remove/disallow it as an option
}

type AuthenticationStatus struct {
	// internal token address
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AuthenticationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Authentication `json:"items"`
}
