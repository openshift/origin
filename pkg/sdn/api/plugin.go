package api

import (
	"strings"
)

const (
	SingleTenantPluginName  = "redhat/openshift-ovs-subnet"
	MultiTenantPluginName   = "redhat/openshift-ovs-multitenant"
	NetworkPolicyPluginName = "redhat/openshift-ovs-networkpolicy"

	IngressBandwidthAnnotation = "kubernetes.io/ingress-bandwidth"
	EgressBandwidthAnnotation  = "kubernetes.io/egress-bandwidth"
	AssignMacvlanAnnotation    = "pod.network.openshift.io/assign-macvlan"
	AssignHostSubnetAnnotation = "pod.network.openshift.io/assign-subnet"
	FixedVnidHost              = "pod.network.openshift.io/fixed-vnid-host"
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
