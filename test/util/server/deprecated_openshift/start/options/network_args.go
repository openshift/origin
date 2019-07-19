package options

// NetworkArgs is a struct that the command stores flag values into.
type NetworkArgs struct {
	// NetworkPluginName is the name of the networking plugin to be used for networking.
	NetworkPluginName string
	// ClusterNetworkCIDR is the CIDR string representing the network that all containers
	// should belong to.
	ClusterNetworkCIDR string
	// HostSubnetLength is the length of subnet each host is given from the network-cidr.
	HostSubnetLength uint32
	// ServiceNetworkCIDR is the CIDR string representing the network that service IP
	// addresses will be allocated from
	ServiceNetworkCIDR string
}

// NewDefaultMasterNetworkArgs returns a new set of network arguments
func NewDefaultMasterNetworkArgs() *NetworkArgs {
	config := &NetworkArgs{
		NetworkPluginName:  "",
		ClusterNetworkCIDR: "10.128.0.0/14",
		HostSubnetLength:   9,
		ServiceNetworkCIDR: "172.30.0.0/16",
	}

	return config
}
