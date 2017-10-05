package network

import (
	"strings"
	"time"

	proxyconfig "k8s.io/kubernetes/pkg/proxy/config"
)

const (
	SingleTenantPluginName  = "redhat/openshift-ovs-subnet"
	MultiTenantPluginName   = "redhat/openshift-ovs-multitenant"
	NetworkPolicyPluginName = "redhat/openshift-ovs-networkpolicy"

	DefaultInformerResyncPeriod = 30 * time.Minute
)

func IsOpenShiftNetworkPlugin(pluginName string) bool {
	switch strings.ToLower(pluginName) {
	case SingleTenantPluginName, MultiTenantPluginName, NetworkPolicyPluginName:
		return true
	}
	return false
}

func IsOpenShiftMultitenantNetworkPlugin(pluginName string) bool {
	if strings.ToLower(pluginName) == MultiTenantPluginName {
		return true
	}
	return false
}

type NodeInterface interface {
	Start() error
}

type ProxyInterface interface {
	proxyconfig.EndpointsHandler

	Start(proxyconfig.EndpointsHandler) error
}
