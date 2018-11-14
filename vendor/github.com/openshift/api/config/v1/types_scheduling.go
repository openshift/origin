package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Scheduling holds cluster-wide information about Scheduling.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type Scheduling struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec SchedulingSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status SchedulingStatus `json:"status"`
}

type SchedulingSpec struct {
	// default node selector (I would be happy to see this die....)
}

type SchedulingStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SchedulingList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Scheduling `json:"items"`
}
