package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Ingress holds cluster-wide information about Ingress.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type Ingress struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec IngressSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status IngressStatus `json:"status"`
}

type IngressSpec struct {
	// default suffix.  It goes here or it gets removed from server
}

type IngressStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type IngressList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Ingress `json:"items"`
}
