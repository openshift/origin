package ipfailover

import (
	"github.com/openshift/origin/pkg/cmd/util/variable"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
)

const (
	// DefaultName is the default IP Failover resource name.
	DefaultName = "ipfailover"

	// DefaultType is the default IP Failover type.
	DefaultType = "keepalived"

	// DefaultServicePort is the port associated with the ipfailover config.
	// Each ipfailover config has a different ServicePort.
	DefaultServicePort = 63000

	// DefaultWatchPort is the default IP Failover watched port number.
	DefaultWatchPort = 80

	// DefaultSelector is the default resource selector.
	DefaultSelector = "ipfailover=<name>"

	// DefaultCheckInterval is the default default interval in seconds
	// between calls to the CheckScript
	DefaultCheckInterval = 2

	// DefaultIptablesChain is the default iptables chain on which to add
	// a rule that accesses 224.0.0.18 (if none exists).
	DefaultIptablesChain = "INPUT"

	// DefaultInterface is the default network interface.
	DefaultInterface = "eth0"
)

// IPFailoverConfigCmdOptions are options supported by the IP Failover admin command.
type IPFailoverConfigCmdOptions struct {
	Action configcmd.BulkAction

	Type           string
	ImageTemplate  variable.ImageTemplate
	ServicePort    int
	Selector       string
	Create         bool
	ServiceAccount string

	//  Failover options.
	VirtualIPs       string
	IptablesChain    string
	NotifyScript     string
	CheckScript      string
	CheckInterval    int
	NetworkInterface string
	WatchPort        int
	VRRPIDOffset     int
	Replicas         int32
}
