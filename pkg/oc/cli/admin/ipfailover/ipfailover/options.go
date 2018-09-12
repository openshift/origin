package ipfailover

import (
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

// IPFailoverConfigOptions are options supported by the IP Failover admin command.
type IPFailoverConfigOptions struct {
	Type           string
	ImageTemplate  variable.ImageTemplate
	ServicePort    int
	Selector       string
	Create         bool
	ServiceAccount string

	ConfiguratorPlugin IPFailoverConfiguratorPlugin

	VirtualIPs       string
	VIPGroups        uint
	IptablesChain    string
	NotifyScript     string
	CheckScript      string
	CheckInterval    int
	Preemption       string
	NetworkInterface string
	WatchPort        int
	VRRPIDOffset     int
	Replicas         int32
}

func NewIPFailoverConfigOptions() *IPFailoverConfigOptions {
	return &IPFailoverConfigOptions{
		ImageTemplate: variable.NewDefaultImageTemplate(),

		CheckInterval:    DefaultCheckInterval,
		IptablesChain:    DefaultIptablesChain,
		NetworkInterface: DefaultInterface,
		Selector:         DefaultSelector,
		ServiceAccount:   DefaultName,
		ServicePort:      DefaultServicePort,
		Type:             DefaultType,
		WatchPort:        DefaultWatchPort,

		Preemption: "preempt_delay 300",

		Replicas:     1,
		VRRPIDOffset: 0,
	}
}
