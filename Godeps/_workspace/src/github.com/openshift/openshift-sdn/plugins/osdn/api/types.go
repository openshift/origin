package api

import (
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
)

type OsdnPlugin interface {
	knetwork.NetworkPlugin

	StartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error
	StartNode(mtu uint) error
}

type FilteringEndpointsConfigHandler interface {
	pconfig.EndpointsConfigHandler
	Start(baseHandler pconfig.EndpointsConfigHandler) error
}
