package api

import (
	knetwork "k8s.io/kubernetes/pkg/kubelet/network"
	pconfig "k8s.io/kubernetes/pkg/proxy/config"
)

type OsdnNodePlugin interface {
	knetwork.NetworkPlugin

	Start() error
}

type FilteringEndpointsConfigHandler interface {
	pconfig.EndpointsConfigHandler
	Start(baseHandler pconfig.EndpointsConfigHandler) error
}
