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

	// KuryrConfig configures the kuryr plugin
	// +optional
	KuryrConfig *KuryrConfig `json:"kuryrConfig,omitempty"`
}

// SimpleMacvlanConfig contains configurations for macvlan interface.
type SimpleMacvlanConfig struct {
	// master is the host interface to create the macvlan interface from.
	// If not specified, it will be default route interface
	// +optional
	Master string `json:"master,omitempty"`

	// IPAMConfig configures IPAM module will be used for IP Address Management (IPAM).
	// +optional
	IPAMConfig *IPAMConfig `json:"ipamConfig,omitempty"`

	// mode is the macvlan mode: bridge, private, vepa, passthru. The default is bridge
	// +optional
	Mode MacvlanMode `json:"mode,omitempty"`

	// mtu is the mtu to use for the macvlan interface. if unset, host's
	// kernel will select the value.
	// +optional
	MTU uint32 `json:"mtu,omitempty"`
}

// StaticIPAMAddresses provides IP address and Gateway for static IPAM addresses
type StaticIPAMAddresses struct {
	// Address is the IP address in CIDR format
	// +optional
	Address string `json:"address"`
	// Gateway is IP inside of subnet to designate as the gateway
	// +optional
	Gateway string `json:"gateway,omitempty"`
}

// StaticIPAMRoutes provides Destination/Gateway pairs for static IPAM routes
type StaticIPAMRoutes struct {
	// Destination points the IP route destination
	Destination string `json:"destination"`
	// Gateway is the route's next-hop IP address
	// If unset, a default gateway is assumed (as determined by the CNI plugin).
	// +optional
	Gateway string `json:"gateway,omitempty"`
}

// StaticIPAMDNS provides DNS related information for static IPAM
type StaticIPAMDNS struct {
	// Nameservers points DNS servers for IP lookup
	// +optional
	Nameservers []string `json:"nameservers,omitempty"`
	// Domain configures the domainname the local domain used for short hostname lookups
	// +optional
	Domain string `json:"domain,omitempty"`
	// Search configures priority ordered search domains for short hostname lookups
	// +optional
	Search []string `json:"search,omitempty"`
}

// StaticIPAMConfig contains configurations for static IPAM (IP Address Management)
type StaticIPAMConfig struct {
	// Addresses configures IP address for the interface
	// +optional
	Addresses []StaticIPAMAddresses `json:"addresses,omitempty"`
	// Routes configures IP routes for the interface
	// +optional
	Routes []StaticIPAMRoutes `json:"routes,omitempty"`
	// DNS configures DNS for the interface
	// +optional
	DNS *StaticIPAMDNS `json:"dns,omitempty"`
}

// IPAMConfig contains configurations for IPAM (IP Address Management)
type IPAMConfig struct {
	// Type is the type of IPAM module will be used for IP Address Management(IPAM).
	// The supported values are IPAMTypeDHCP, IPAMTypeStatic
	Type IPAMType `json:"type"`

	// StaticIPAMConfig configures the static IP address in case of type:IPAMTypeStatic
	// +optional
	StaticIPAMConfig *StaticIPAMConfig `json:"staticIPAMConfig,omitempty"`
}

// AdditionalNetworkDefinition configures an extra network that is available but not
// created by default. Instead, pods must request them by name.
// type must be specified, along with exactly one "Config" that matches the type.
type AdditionalNetworkDefinition struct {
	// type is the type of network
	// The supported values are NetworkTypeRaw, NetworkTypeSimpleMacvlan
	Type NetworkType `json:"type"`

	// name is the name of the network. This will be populated in the resulting CRD
	// This must be unique.
	Name string `json:"name"`

	// namespace is the namespace of the network. This will be populated in the resulting CRD
	// If not given the network will be created in the default namespace.
	Namespace string `json:"namespace,omitempty"`

	// rawCNIConfig is the raw CNI configuration json to create in the
	// NetworkAttachmentDefinition CRD
	RawCNIConfig string `json:"rawCNIConfig,omitempty"`

	// SimpleMacvlanConfig configures the macvlan interface in case of type:NetworkTypeSimpleMacvlan
	// +optional
	SimpleMacvlanConfig *SimpleMacvlanConfig `json:"simpleMacvlanConfig,omitempty"`
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

	// enableUnidling controls whether or not the service proxy will support idling
	// and unidling of services. By default, unidling is enabled.
	EnableUnidling *bool `json:"enableUnidling,omitempty"`
}

// KuryrConfig configures the Kuryr-Kubernetes SDN
type KuryrConfig struct {
	// The port kuryr-daemon will listen for readiness and liveness requests.
	// +optional
	DaemonProbesPort *uint32 `json:"daemonProbesPort,omitempty"`

	// The port kuryr-controller will listen for readiness and liveness requests.
	// +optional
	ControllerProbesPort *uint32 `json:"controllerProbesPort,omitempty"`
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

// ProxyArgumentList is a list of arguments to pass to the kubeproxy process
type ProxyArgumentList []string

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
	ProxyArguments map[string]ProxyArgumentList `json:"proxyArguments,omitempty"`
}

const (
	// NetworkTypeOpenShiftSDN means the openshift-sdn plugin will be configured
	NetworkTypeOpenShiftSDN NetworkType = "OpenShiftSDN"

	// NetworkTypeOVNKubernetes means the ovn-kubernetes project will be configured.
	// This is currently not implemented.
	NetworkTypeOVNKubernetes NetworkType = "OVNKubernetes"

	// NetworkTypeKuryr means the kuryr-kubernetes project will be configured.
	NetworkTypeKuryr NetworkType = "Kuryr"

	// NetworkTypeRaw
	NetworkTypeRaw NetworkType = "Raw"

	// NetworkTypeSimpleMacvlan
	NetworkTypeSimpleMacvlan NetworkType = "SimpleMacvlan"
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

// MacvlanMode is the Mode of macvlan. The value are lowercase to match the CNI plugin
// config values. See "man ip-link" for its detail.
type MacvlanMode string

const (
	// MacvlanModeBridge is the macvlan with thin bridge function.
	MacvlanModeBridge MacvlanMode = "Bridge"
	// MacvlanModePrivate
	MacvlanModePrivate MacvlanMode = "Private"
	// MacvlanModeVEPA is used with Virtual Ethernet Port Aggregator
	// (802.1qbg) swtich
	MacvlanModeVEPA MacvlanMode = "VEPA"
	// MacvlanModePassthru
	MacvlanModePassthru MacvlanMode = "Passthru"
)

// IPAMType describes the IP address management type to configure
type IPAMType string

const (
	// IPAMTypeDHCP uses DHCP for IP management
	IPAMTypeDHCP IPAMType = "DHCP"
	// IPAMTypeStatic uses static IP
	IPAMTypeStatic IPAMType = "Static"
)
