package ipfailover

import (
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

const (
	// Default IP Failover resource name.
	DefaultName = "ipfailover"

	// Default IP Failover type.
	DefaultType = "keepalived"

	// Default service port.
	DefaultServicePort = 1985

	// Default IP Failover watched port number.
	DefaultWatchPort = 80

	// Default resource selector.
	DefaultSelector = "ipfailover=<name>"

	// Default network interface.
	DefaultInterface = "eth0"
)

// Options supported by the IP Failover admin command.
type IPFailoverConfigCmdOptions struct {
	Type          string
	ImageTemplate variable.ImageTemplate
	Credentials   string
	ServicePort   int
	Selector      string
	Create        bool

	//  Failover options.
	VirtualIPs       string
	NetworkInterface string
	WatchPort        int
	Replicas         int

	//  For the future - currently unused.
	UseUnicast bool
}
