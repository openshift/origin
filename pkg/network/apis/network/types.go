package network

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ClusterNetworkDefault       = "default"
	EgressNetworkPolicyMaxRules = 50
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterNetwork struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	ClusterNetworks  []ClusterNetworkEntry
	Network          string
	HostSubnetLength uint32
	ServiceNetwork   string
	PluginName       string
}

type ClusterNetworkEntry struct {
	CIDR             string
	HostSubnetLength uint32
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterNetworkList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []ClusterNetwork
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostSubnet encapsulates the inputs needed to define the container subnet network on a node
type HostSubnet struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	// host may just be an IP address, resolvable hostname or a complete DNS
	Host   string
	HostIP string
	Subnet string

	EgressIPs []string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostSubnetList is a collection of HostSubnets
type HostSubnetList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []HostSubnet
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetNamespace holds information about the SDN configuration of a Namespace
type NetNamespace struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	NetName string
	NetID   uint32

	EgressIPs []string
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetNamespaceList is a collection of NetNamespaces
type NetNamespaceList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []NetNamespace
}

// EgressNetworkPolicyRuleType gives the type of an EgressNetworkPolicyRule
type EgressNetworkPolicyRuleType string

const (
	EgressNetworkPolicyRuleAllow EgressNetworkPolicyRuleType = "Allow"
	EgressNetworkPolicyRuleDeny  EgressNetworkPolicyRuleType = "Deny"
)

// EgressNetworkPolicyPeer specifies a target to apply egress policy to
type EgressNetworkPolicyPeer struct {
	CIDRSelector string
	DNSName      string
}

// EgressNetworkPolicyRule contains a single egress network policy rule
type EgressNetworkPolicyRule struct {
	Type EgressNetworkPolicyRuleType
	To   EgressNetworkPolicyPeer
}

// EgressNetworkPolicySpec provides a list of policies on outgoing traffic
type EgressNetworkPolicySpec struct {
	Egress []EgressNetworkPolicyRule
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EgressNetworkPolicy describes the current egress network policy
type EgressNetworkPolicy struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec EgressNetworkPolicySpec
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EgressNetworkPolicyList is a collection of EgressNetworkPolicy
type EgressNetworkPolicyList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []EgressNetworkPolicy
}
