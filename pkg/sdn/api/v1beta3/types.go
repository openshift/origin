package v1beta3

import (
	kapi "k8s.io/kubernetes/pkg/api/v1beta3"
)

type ClusterNetwork struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	Network          string `json:"network" description:"CIDR string to specify the global overlay network's L3 space"`
	HostSubnetLength int    `json:"hostsubnetlength" description:"number of bits to allocate to each host's subnet e.g. 8 would mean a /24 network on the host"`
	ServiceNetwork   string `json:"serviceNetwork" description:"CIDR string to specify the service network"`
}

type ClusterNetworkList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []ClusterNetwork `json:"items"`
}

// HostSubnet encapsulates the inputs needed to define the container subnet network on a node
type HostSubnet struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	// host may just be an IP address, resolvable hostname or a complete DNS
	Host   string `json:"host" description:"Name of the host that is registered at the master. A lease will be sought after this name."`
	HostIP string `json:"hostIP" description:"IP address to be used as vtep by other hosts in the overlay network"`
	Subnet string `json:"subnet" description:"Actual subnet CIDR lease assigned to the host"`
}

// HostSubnetList is a collection of HostSubnets
type HostSubnetList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []HostSubnet `json:"items"`
}

// NetNamespace encapsulates the inputs needed to define a unique network namespace on the cluster
type NetNamespace struct {
	kapi.TypeMeta   `json:",inline"`
	kapi.ObjectMeta `json:"metadata,omitempty"`

	NetName string `json:"netname" description:"Name of the network namespace."`
	NetID   uint   `json:"netid" description:"NetID of the network namespace assigned to each overlay network packet."`
}

// NetNamespaceList is a collection of NetNamespaces
type NetNamespaceList struct {
	kapi.TypeMeta `json:",inline"`
	kapi.ListMeta `json:"metadata,omitempty"`
	Items         []NetNamespace `json:"items" description:"list of net namespaces"`
}
