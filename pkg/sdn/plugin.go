package sdn

import (
	"strings"
)

const (
	SingleTenantPluginName  = "redhat/openshift-ovs-subnet"
	MultiTenantPluginName   = "redhat/openshift-ovs-multitenant"
	NetworkPolicyPluginName = "redhat/openshift-ovs-networkpolicy"
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
