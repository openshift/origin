package haconfig

import (
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

const (
	// Default ha-config resource name.
	DefaultName = "ha-config"
	// Default ha-config type.
	DefaultType = "keepalived"
	// Default ha-config watched port number.
	DefaultWatchPort = 80
	// Default resource selector.
	DefaultSelector = "ha-config=<name>"
	// Default network interface.
	DefaultInterface = "eth0"
)

// Options supported by the ha-config admin command.
type HAConfigCmdOptions struct {
	Type          string
	ImageTemplate variable.ImageTemplate
	Selector      string
	Credentials   string

	//  Create/delete configuration.
	Create bool
	Delete bool

	VirtualIPs       string
	NetworkInterface string
	WatchPort        string
	Replicas         int

	//  For the future - currently unused.
	UseUnicast bool
}
