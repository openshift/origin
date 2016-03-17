package api

import (
	osapi "github.com/openshift/origin/pkg/sdn/api"

	kapi "k8s.io/kubernetes/pkg/api"
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
)

type EventType string

const (
	Added    EventType = "ADDED"
	Deleted  EventType = "DELETED"
	Modified EventType = "MODIFIED"
)

type HostSubnetEvent struct {
	Type       EventType
	HostSubnet *osapi.HostSubnet
}

type NodeEvent struct {
	Type EventType
	Node *kapi.Node
}

type NetNamespaceEvent struct {
	Type         EventType
	NetNamespace *osapi.NetNamespace
}

type NamespaceEvent struct {
	Type      EventType
	Namespace *kapi.Namespace
}

type ServiceEvent struct {
	Type    EventType
	Service *kapi.Service
}

type OsdnPlugin interface {
	knetwork.NetworkPlugin

	StartMaster(clusterNetworkCIDR string, clusterBitsPerSubnet uint, serviceNetworkCIDR string) error
	StartNode(mtu uint) error
}

type FilteringEndpointsConfigHandler interface {
	pconfig.EndpointsConfigHandler
	SetBaseEndpointsHandler(base pconfig.EndpointsConfigHandler)
}
