package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Project holds cluster-wide information about Project.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type Project struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec ProjectSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status ProjectStatus `json:"status"`
}

type ProjectSpec struct {
	// project request message
	// project request template
}

type ProjectStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}
