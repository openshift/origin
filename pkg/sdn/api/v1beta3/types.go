package v1beta3

import (
	"k8s.io/kubernetes/pkg/api/unversioned"
	kapi "k8s.io/kubernetes/pkg/api/v1beta3"
)

type ClusterNetwork struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	Network          string `json:"network"`
	HostSubnetLength int    `json:"hostsubnetlength"`
	ServiceNetwork   string `json:"serviceNetwork"`
}

type ClusterNetworkList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	Items                []ClusterNetwork `json:"items"`
}

// HostSubnet encapsulates the inputs needed to define the container subnet network on a node
type HostSubnet struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	// host may just be an IP address, resolvable hostname or a complete DNS
	Host   string `json:"host"`
	HostIP string `json:"hostIP"`
	Subnet string `json:"subnet"`
}

// HostSubnetList is a collection of HostSubnets
type HostSubnetList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	Items                []HostSubnet `json:"items"`
}

// NetNamespace encapsulates the inputs needed to define a unique network namespace on the cluster
type NetNamespace struct {
	unversioned.TypeMeta `json:",inline"`
	kapi.ObjectMeta      `json:"metadata,omitempty"`

	NetName string `json:"netname"`
	NetID   uint   `json:"netid"`
}

// NetNamespaceList is a collection of NetNamespaces
type NetNamespaceList struct {
	unversioned.TypeMeta `json:",inline"`
	unversioned.ListMeta `json:"metadata,omitempty"`
	Items                []NetNamespace `json:"items"`
}
