package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Scheduler holds cluster-wide information about Scheduler.  The canonical name is `cluster`
type Scheduler struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	// +required
	Spec SchedulerSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status SchedulerStatus `json:"status"`
}

type SchedulerSpec struct {
	// policy is a reference to a ConfigMap containing scheduler policy which has
	// user specified predicates and priorities. If this ConfigMap is not available
	// scheduler will default to use DefaultAlgorithmProvider.
	// The namespace for this configmap is openshift-config.
	// +optional
	Policy ConfigMapNameReference `json:"policy"`
}

type SchedulerStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SchedulerList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []Scheduler `json:"items"`
}
