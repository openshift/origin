package api

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

type ClusterNetwork struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	Network          string
	HostSubnetLength int
}

type ClusterNetworkList struct {
	kapi.TypeMeta
	kapi.ListMeta
	Items []ClusterNetwork
}

// HostSubnet encapsulates the inputs needed to define the container subnet network on a minion
type HostSubnet struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	// host may just be an IP address, resolvable hostname or a complete DNS
	Host   string
	HostIP string
	Subnet string
}

// HostSubnetList is a collection of HostSubnets
type HostSubnetList struct {
	kapi.TypeMeta
	kapi.ListMeta
	Items []HostSubnet
}

// NetNamespace holds the network id against its name
type NetNamespace struct {
	kapi.TypeMeta
	kapi.ObjectMeta

	NetName string
	NetID   uint
}

// NetNamespaceList is a collection of NetNamespaces
type NetNamespaceList struct {
	kapi.TypeMeta
	kapi.ListMeta
	Items []NetNamespace
}
