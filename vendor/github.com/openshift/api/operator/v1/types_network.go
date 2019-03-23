package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Network describes the cluster's desired network configuration. It is
// consumed by the cluster-network-operator.
// +k8s:openapi-gen=true
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec,omitempty"`
	Status NetworkStatus `json:"status,omitempty"`
}

// NetworkStatus is currently unused. Instead, status
// is reported in the Network.config.openshift.io object.
type NetworkStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkList contains a list of Network configurations
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Network `json:"items"`
}

// NetworkSpec is the top-level network configuration object.
type NetworkSpec struct {
	// clusterNetwork is the IP address pool to use for pod IPs.
	// Some network providers, e.g. OpenShift SDN, support multiple ClusterNetworks.
	// Others only support one. This is equivalent to the cluster-cidr.
	ClusterNetwork []ClusterNetworkEntry `json:"clusterNetwork"`

	// serviceNetwork is the ip address pool to use for Service IPs
	// Currently, all existing network providers only support a single value
	// here, but this is an array to allow for growth.
	ServiceNetwork []string `json:"serviceNetwork"`

	// defaultNetwork is the "default" network that all pods will receive
	DefaultNetwork DefaultNetworkDefinition `json:"defaultNetwork"`

	// additionalNetworks is a list of extra networks to make available to pods
	// when multiple networks are enabled.
	AdditionalNetworks []AdditionalNetworkDefinition `json:"additionalNetworks,omitempty"`

	// disableMultiNetwork specifies whether or not multiple pod network
	// support should be disabled. If unset, this property defaults to
	// 'false' and multiple network support is enabled.
	DisableMultiNetwork *bool `json:"disableMultiNetwork,omitempty"`

	// deployKubeProxy specifies whether or not a standalone kube-proxy should
	// be deployed by the operator. Some network providers include kube-proxy
	// or similar functionality. If unset, the plugin will attempt to select
	// the correct value, which is false when OpenShift SDN and ovn-kubernetes are
	// used and true otherwise.
	// +optional
	DeployKubeProxy *bool `json:"deployKubeProxy,omitempty"`

	// kubeProxyConfig lets us configure desired proxy configuration.
	// If not specified, sensible defaults will be chosen by OpenShift directly.
	// Not consumed by all network providers - currently only openshift-sdn.
	KubeProxyConfig *ProxyConfig `json:"kubeProxyConfig,omitempty"`
}

// ClusterNetworkEntry is a subnet from which to allocate PodIPs. A network of size
// HostPrefix (in CIDR notation) will be allocated when nodes join the cluster.
// Not all network providers support multiple ClusterNetworks
type ClusterNetworkEntry struct {
	CIDR       string `json:"cidr"`
	HostPrefix uint32 `json:"hostPrefix"`
}

// DefaultNetworkDefinition represents a single network plugin's configuration.
// type must be specified, along with exactly one "Config" that matches the type.
type DefaultNetworkDefinition struct {
	// type is the type of network
	// All NetworkTypes are supported except for NetworkTypeRaw
	Type NetworkType `json:"type"`

	// openShiftSDNConfig configures the openshift-sdn plugin
	// +optional
	OpenShiftSDNConfig *OpenShiftSDNConfig `json:"openshiftSDNConfig,omitempty"`

	// oVNKubernetesConfig configures the ovn-kubernetes plugin. This is currently
	// not implemented.
	// +optional
	OVNKubernetesConfig *OVNKubernetesConfig `json:"ovnKubernetesConfig,omitempty"`
}

// AdditionalNetworkDefinition configures an extra network that is available but not
// created by default. Instead, pods must request them by name.
// type must be specified, along with exactly one "Config" that matches the type.
type AdditionalNetworkDefinition struct {
	// type is the type of network
	// The only supported value is NetworkTypeRaw
	Type NetworkType `json:"type"`

	// name is the name of the network. This will be populated in the resulting CRD
	// This must be unique.
	Name string `json:"name"`

	// rawCNIConfig is the raw CNI configuration json to create in the
	// NetworkAttachmentDefinition CRD
	RawCNIConfig string `json:"rawCNIConfig"`
}

// OpenShiftSDNConfig configures the three openshift-sdn plugins
type OpenShiftSDNConfig struct {
	// mode is one of "Multitenant", "Subnet", or "NetworkPolicy"
	Mode SDNMode `json:"mode"`

	// vxlanPort is the port to use for all vxlan packets. The default is 4789.
	// +optional
	VXLANPort *uint32 `json:"vxlanPort,omitempty"`

	// mtu is the mtu to use for the tunnel interface. Defaults to 1450 if unset.
	// This must be 50 bytes smaller than the machine's uplink.
	// +optional
	MTU *uint32 `json:"mtu,omitempty"`

	// useExternalOpenvswitch tells the operator not to install openvswitch, because
	// it will be provided separately. If set, you must provide it yourself.
	// +optional
	UseExternalOpenvswitch *bool `json:"useExternalOpenvswitch,omitempty"`
}

// ovnKubernetesConfig is the proposed configuration parameters for networks
// using the ovn-kubernetes network project
type OVNKubernetesConfig struct {
	// mtu is the MTU to use for the tunnel interface. This must be 100
	// bytes smaller than the uplink mtu.
	// Default is 1400
	MTU *uint32 `json:"mtu,omitempty"`
}

// NetworkType describes the network plugin type to configure
type NetworkType string

// ProxyConfig defines the configuration knobs for kubeproxy
// All of these are optional and have sensible defaults
type ProxyConfig struct {
	// The period that iptables rules are refreshed.
	// Default: 30s
	IptablesSyncPeriod string `json:"iptablesSyncPeriod,omitempty"`

	// The address to "bind" on
	// Defaults to 0.0.0.0
	BindAddress string `json:"bindAddress,omitempty"`

	// Any additional arguments to pass to the kubeproxy process
	ProxyArguments map[string][]string `json:"proxyArguments,omitempty"`
}

const (
	// NetworkTypeOpenShiftSDN means the openshift-sdn plugin will be configured
	NetworkTypeOpenShiftSDN NetworkType = "OpenShiftSDN"

	// NetworkTypeOVNKubernetes means the ovn-kubernetes project will be configured.
	// This is currently not implemented.
	NetworkTypeOVNKubernetes NetworkType = "OVNKubernetes"

	// NetworkTypeRaw
	NetworkTypeRaw NetworkType = "Raw"
)

// SDNMode is the Mode the openshift-sdn plugin is in
type SDNMode string

const (
	// SDNModeSubnet is a simple mode that offers no isolation between pods
	SDNModeSubnet SDNMode = "Subnet"

	// SDNModeMultitenant is a special "multitenant" mode that offers limited
	// isolation configuration between namespaces
	SDNModeMultitenant SDNMode = "Multitenant"

	// SDNModeNetworkPolicy is a full NetworkPolicy implementation that allows
	// for sophisticated network isolation and segmenting. This is the default.
	SDNModeNetworkPolicy SDNMode = "NetworkPolicy"
)
