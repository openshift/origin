package start

import (
	"github.com/spf13/pflag"
)

// NetworkArgs is a struct that the command stores flag values into.
type NetworkArgs struct {
	// NetworkPluginName is the name of the networking plugin to be used for networking.
	NetworkPluginName string
	// ClusterNetworkCIDR is the CIDR string representing the network that all containers
	// should belong to.
	ClusterNetworkCIDR string
	// HostSubnetLength is the length of subnet each host is given from the network-cidr.
	HostSubnetLength uint
	// ServiceNetworkCIDR is the CIDR string representing the network that service IP
	// addresses will be allocated from
	ServiceNetworkCIDR string
}

// BindNetworkArgs binds values to the given arguments by using flags
func BindNetworkArgs(args *NetworkArgs, flags *pflag.FlagSet, prefix string) {
	flags.StringVar(&args.NetworkPluginName, prefix+"network-plugin", args.NetworkPluginName, "The name of the networking plugin to be used for networking.")
	flags.StringVar(&args.ClusterNetworkCIDR, prefix+"network-cidr", args.ClusterNetworkCIDR, "The CIDR string representing the network that all containers should belong to.")
	flags.UintVar(&args.HostSubnetLength, prefix+"host-subnet-length", args.HostSubnetLength, "The length of subnet each host is given from the network-cidr.")
	flags.StringVar(&args.ServiceNetworkCIDR, prefix+"portal-net", args.ServiceNetworkCIDR, "The CIDR string representing the network that portal/service IPs will be assigned from. This must not overlap with any IP ranges assigned to nodes for pods.")
}

// NewDefaultNetworkArgs returns a new set of network arguments
func NewDefaultNetworkArgs() *NetworkArgs {
	config := &NetworkArgs{
		NetworkPluginName:  "",
		ClusterNetworkCIDR: "10.1.0.0/16",
		HostSubnetLength:   8,
		ServiceNetworkCIDR: "172.30.0.0/16",
	}

	return config
}
