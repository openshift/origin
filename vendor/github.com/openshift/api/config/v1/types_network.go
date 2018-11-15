package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Network holds cluster-wide information about Network.  The canonical name is `cluster`
// TODO this object is an example of a possible grouping and is subject to change or removal
type Network struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec holds user settable values for configuration
	Spec NetworkSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	Status NetworkStatus `json:"status"`
}

type NetworkSpec struct {
	// serviceCIDR
	// servicePortRange
	// vxlanPort
	// 	ClusterNetworks    []ClusterNetworkEntry `json:"clusterNetworks"`
}

type NetworkStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}
