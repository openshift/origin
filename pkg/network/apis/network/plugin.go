package network

const (
	// Pod annotations
	IngressBandwidthAnnotation = "kubernetes.io/ingress-bandwidth"
	EgressBandwidthAnnotation  = "kubernetes.io/egress-bandwidth"
	AssignMacvlanAnnotation    = "pod.network.openshift.io/assign-macvlan"

	// HostSubnet annotations. (Note: should be "hostsubnet.network.openshift.io/", but the incorrect name is now part of the API.)
	AssignHostSubnetAnnotation = "pod.network.openshift.io/assign-subnet"
	FixedVNIDHostAnnotation    = "pod.network.openshift.io/fixed-vnid-host"
	NodeUIDAnnotation          = "pod.network.openshift.io/node-uid"

	// NetNamespace annotations
	MulticastEnabledAnnotation = "netnamespace.network.openshift.io/multicast-enabled"
)
