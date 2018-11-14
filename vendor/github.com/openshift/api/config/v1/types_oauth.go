package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// OAuth holds cluster-wide information about OAuth.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type OAuth struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec OAuthSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status OAuthStatus `json:"status"`
}

type OAuthSpec struct {
	// options for configuring the embedded oauth server.
	// possibly wellknown?
}

type OAuthStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type OAuthList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OAuth `json:"items"`
}
