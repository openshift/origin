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

	// spec holds user settable values for configuration.
	// +required
	Spec NetworkSpec `json:"spec"`
	// status holds observed values from the cluster. They may not be overridden.
	// +optional
	Status NetworkStatus `json:"status"`
}

// NetworkSpec is the desired network configuration.
// As a general rule, this SHOULD NOT be read directly. Instead, you should
// consume the NetworkStatus, as it indicates the currently deployed configuration.
// Currently, none of these fields may be changed after installation.
type NetworkSpec struct {
	// IP address pool to use for pod IPs.
	ClusterNetwork []ClusterNetworkEntry `json:"clusterNetwork"`

	// IP address pool for services.
	// Currently, we only support a single entry here.
	ServiceNetwork []string `json:"serviceNetwork"`

	// NetworkType is the plugin that is to be deployed (e.g. OpenShiftSDN).
	// This should match a value that the cluster-network-operator understands,
	// or else no networking will be installed.
	// Currently supported values are:
	// - OpenShiftSDN
	NetworkType string `json:"networkType"`
}

// NetworkStatus is the current network configuration.
type NetworkStatus struct {
	// IP address pool to use for pod IPs.
	ClusterNetwork []ClusterNetworkEntry `json:"clusterNetwork,omitempty"`

	// IP address pool for services.
	// Currently, we only support a single entry here.
	ServiceNetwork []string `json:"serviceNetwork,omitempty"`

	// NetworkType is the plugin that is deployed (e.g. OpenShiftSDN).
	NetworkType string `json:"networkType,omitempty"`

	// ClusterNetworkMTU is the MTU for inter-pod networking.
	ClusterNetworkMTU int `json:"clusterNetworkMTU,omitempty"`
}

// ClusterNetworkEntry is a contiguous block of IP addresses from which pod IPs
// are allocated.
type ClusterNetworkEntry struct {
	// The complete block for pod IPs.
	CIDR string `json:"cidr"`

	// The size (prefix) of block to allocate to each node.
	HostPrefix uint32 `json:"hostPrefix"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ListMeta `json:"metadata"`
	Items           []Network `json:"items"`
}
