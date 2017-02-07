package api

import (
	"strings"
)

const (
	SingleTenantPluginName  = "redhat/openshift-ovs-subnet"
	MultiTenantPluginName   = "redhat/openshift-ovs-multitenant"
	NetworkPolicyPluginName = "redhat/openshift-ovs-networkpolicy"

	// Pod annotations
	IngressBandwidthAnnotation = "kubernetes.io/ingress-bandwidth"
	EgressBandwidthAnnotation  = "kubernetes.io/egress-bandwidth"
	AssignMacvlanAnnotation    = "pod.network.openshift.io/assign-macvlan"

	// HostSubnet annotations. (Note: should be "hostsubnet.network.openshift.io/", but the incorrect name is now part of the API.)
	AssignHostSubnetAnnotation = "pod.network.openshift.io/assign-subnet"
	FixedVNIDHostAnnotation    = "pod.network.openshift.io/fixed-vnid-host"

	// NetNamespace annotations
	MulticastEnabledAnnotation = "netnamespace.network.openshift.io/multicast-enabled"
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
