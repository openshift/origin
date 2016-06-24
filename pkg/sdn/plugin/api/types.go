package api

import (
	pconfig "k8s.io/kubernetes/pkg/proxy/config"

	cnitypes "github.com/containernetworking/cni/pkg/types"
)

type FilteringEndpointsConfigHandler interface {
	pconfig.EndpointsConfigHandler
	Start(baseHandler pconfig.EndpointsConfigHandler) error
}

// Standard CNI network configuration block written to /etc/cni/net.d
// which is picked up by the Kubernetes CNI network plugin and used to
// run our openshift-sdn CNI plugin
type CNINetConfig struct {
	cnitypes.NetConf
	MasterKubeConfig string `json:"masterKubeConfig"`
	NodeName         string `json:"nodeName"`
	MTU              uint32 `json:"mtu"`
}
