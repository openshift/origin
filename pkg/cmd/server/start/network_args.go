package start

import (
	"github.com/spf13/pflag"
)

// NetworkArgs is a struct that the command stores flag values into.
type NetworkArgs struct {
	NetworkPluginName  string
	ClusterNetworkCIDR string
	HostSubnetLength   uint
}

func BindNetworkArgs(args *NetworkArgs, flags *pflag.FlagSet, prefix string) {
	flags.StringVar(&args.NetworkPluginName, prefix+"network-plugin", args.NetworkPluginName, "The name of the networking plugin to be used for networking.")
	flags.StringVar(&args.ClusterNetworkCIDR, prefix+"network-cidr", args.ClusterNetworkCIDR, "The CIDR string representing the network that all containers should belong to.")
	flags.UintVar(&args.HostSubnetLength, prefix+"host-subnet-length", args.HostSubnetLength, "The length of subnet each host is given from the network-cidr.")
}

func NewDefaultNetworkArgs() *NetworkArgs {
	config := &NetworkArgs{
		NetworkPluginName:  "",
		ClusterNetworkCIDR: "10.1.0.0/16",
		HostSubnetLength:   8,
	}

	return config
}
