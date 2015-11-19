package v1

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&ClusterNetwork{},
		&ClusterNetworkList{},
		&HostSubnet{},
		&HostSubnetList{},
		&NetNamespace{},
		&NetNamespaceList{},
	)
}

func (*ClusterNetwork) IsAnAPIObject()     {}
func (*ClusterNetworkList) IsAnAPIObject() {}
func (*HostSubnet) IsAnAPIObject()         {}
func (*HostSubnetList) IsAnAPIObject()     {}
func (*NetNamespace) IsAnAPIObject()       {}
func (*NetNamespaceList) IsAnAPIObject()   {}
