package v1

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1"
)

const (
	ClusterNetworkDefault = "default"
)

// +genclient=true

// ClusterNetwork describes the cluster network. There is normally only one object of this type,
// named "default", which is created by the SDN network plugin based on the master configuration
// when the cluster is brought up for the first time.
type ClusterNetwork struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Network is a CIDR string specifying the global overlay network's L3 space
	Network string `json:"network" protobuf:"bytes,2,opt,name=network"`
	// HostSubnetLength is the number of bits of network to allocate to each node. eg, 8 would mean that each node would have a /24 slice of the overlay network for its pods
	HostSubnetLength uint32 `json:"hostsubnetlength" protobuf:"varint,3,opt,name=hostsubnetlength"`
	// ServiceNetwork is the CIDR range that Service IP addresses are allocated from
	ServiceNetwork string `json:"serviceNetwork" protobuf:"bytes,4,opt,name=serviceNetwork"`
	// PluginName is the name of the network plugin being used
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

// HostSubnet describes the container subnet network on a node. The HostSubnet object must have the
// same name as the Node object it corresponds to.
type HostSubnet struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Host is the name of the node. (This is redundant with the object's name, and this
	// field is not actually used any more.)
	Host string `json:"host" protobuf:"bytes,2,opt,name=host"`
	// HostIP is the IP address to be used as a VTEP by other nodes in the overlay network
	HostIP string `json:"hostIP" protobuf:"bytes,3,opt,name=hostIP"`
	// Subnet is the CIDR range of the overlay network assigned to the node for its pods
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

// NetNamespace describes a single isolated network. When using the redhat/openshift-ovs-multitenant
// plugin, every Namespace will have a corresponding NetNamespace object with the same name.
// (When using redhat/openshift-ovs-subnet, NetNamespaces are not used.)
type NetNamespace struct {
	unversioned.TypeMeta `json:",inline"`
	// Standard object's metadata.
	kapi.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// NetName is the name of the network namespace. (This is the same as the object's name, but both fields must be set.)
	NetName string `json:"netname" protobuf:"bytes,2,opt,name=netname"`
	// NetID is the network identifier of the network namespace assigned to each overlay network packet. This can be manipulated with the "oadm pod-network" commands.
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
