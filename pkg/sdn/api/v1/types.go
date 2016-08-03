package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

const (
	ClusterNetworkDefault = "default"
)

// +genclient=true

// ClusterNetwork describes a cluster network
type ClusterNetwork struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Network is a CIDR string to specify the global overlay network's L3 space
	Network string `json:"network" protobuf:"bytes,2,opt,name=network"`
	// HostSubnetLength is the number of bits to allocate to each host's subnet e.g. 8 would mean a /24 network on the host
	HostSubnetLength uint32 `json:"hostsubnetlength" protobuf:"varint,3,opt,name=hostsubnetlength"`
	// ServiceNetwork is the CIDR string to specify the service network
	ServiceNetwork string `json:"serviceNetwork" protobuf:"bytes,4,opt,name=serviceNetwork"`
	// PluginName is the name of the network plugin
	PluginName string `json:"pluginName,omitempty" protobuf:"bytes,5,opt,name=pluginName"`
}

// ClusterNetworkList is a collection of ClusterNetworks
type ClusterNetworkList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of cluster networks
	Items []ClusterNetwork `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// HostSubnet encapsulates the inputs needed to define the container subnet network on a node
type HostSubnet struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Host is the name of the host that is registered at the master. May just be an IP address, resolvable hostname or a complete DNS.
	// A lease will be sought after this name.
	Host string `json:"host" protobuf:"bytes,2,opt,name=host"`
	// HostIP is the IP address to be used as vtep by other hosts in the overlay network
	HostIP string `json:"hostIP" protobuf:"bytes,3,opt,name=hostIP"`
	// Subnet is the actual subnet CIDR lease assigned to the host
	Subnet string `json:"subnet" protobuf:"bytes,4,opt,name=subnet"`
}

// HostSubnetList is a collection of HostSubnets
type HostSubnetList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of host subnets
	Items []HostSubnet `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// NetNamespace encapsulates the inputs needed to define a unique network namespace on the cluster
type NetNamespace struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// NetName is the name of the network namespace
	NetName string `json:"netname" protobuf:"bytes,2,opt,name=netname"`
	// NetID is the network identifier of the network namespace assigned to each overlay network packet
	NetID uint32 `json:"netid" protobuf:"varint,3,opt,name=netid"`
}

// NetNamespaceList is a collection of NetNamespaces
type NetNamespaceList struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of net namespaces
	Items []NetNamespace `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// EgressNetworkPolicyRuleType indicates whether an EgressNetworkPolicyRule allows or denies traffic
type EgressNetworkPolicyRuleType string

const (
	EgressNetworkPolicyRuleAllow EgressNetworkPolicyRuleType = "Allow"
	EgressNetworkPolicyRuleDeny  EgressNetworkPolicyRuleType = "Deny"
)

// EgressNetworkPolicyPeer specifies a target to apply egress network policy to
type EgressNetworkPolicyPeer struct {
	// cidrSelector is the CIDR range to allow/deny traffic to
	CIDRSelector string `json:"cidrSelector" protobuf:"bytes,1,rep,name=cidrSelector"`
}

// EgressNetworkPolicyRule contains a single egress network policy rule
type EgressNetworkPolicyRule struct {
	// type marks this as an "Allow" or "Deny" rule
	Type EgressNetworkPolicyRuleType `json:"type" protobuf:"bytes,1,rep,name=type"`
	// to is the target that traffic is allowed/denied to
	To EgressNetworkPolicyPeer `json:"to" protobuf:"bytes,2,rep,name=to"`
}

// EgressNetworkPolicySpec provides a list of policies on outgoing network traffic
type EgressNetworkPolicySpec struct {
	// egress contains the list of egress policy rules
	Egress []EgressNetworkPolicyRule `json:"egress" protobuf:"bytes,1,rep,name=egress"`
}

// EgressNetworkPolicy describes the current egress network policy for a Namespace. When using
// the 'redhat/openshift-ovs-multitenant' network plugin, traffic from a pod to an IP address
// outside the cluster will be checked against each EgressNetworkPolicyRule in the pod's
// namespace's EgressNetworkPolicy, in order. If no rule matches (or no EgressNetworkPolicy
// is present) then the traffic will be allowed by default.
type EgressNetworkPolicy struct {
	unversioned.TypeMeta `json:",inline"`
	// metadata for EgressNetworkPolicy
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// spec is the specification of the current egress network policy
	Spec EgressNetworkPolicySpec `json:"spec" protobuf:"bytes,2,opt,name=spec"`
}

// EgressNetworkPolicyList is a collection of EgressNetworkPolicy
type EgressNetworkPolicyList struct {
	unversioned.TypeMeta `json:",inline"`
	// metadata for EgressNetworkPolicyList
	unversioned.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// items is the list of policies
	Items []EgressNetworkPolicy `json:"items" protobuf:"bytes,2,rep,name=items"`
}
