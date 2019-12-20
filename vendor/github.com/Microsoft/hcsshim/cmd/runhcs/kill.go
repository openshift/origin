package main

import (
	"github.com/Microsoft/hcsshim/internal/appargs"
	"github.com/Microsoft/hcsshim/internal/hcs"
	"github.com/Microsoft/hcsshim/internal/schema1"
	"github.com/Microsoft/hcsshim/internal/signals"
	"github.com/Microsoft/hcsshim/osversion"
	"github.com/urfave/cli"
)

var killCommand = cli.Command{
	Name:  "kill",
	Usage: "kill sends the specified signal (default: SIGTERM) to the container's init process",
	ArgsUsage: `<container-id> [signal]

Where "<container-id>" is the name for the instance of the container and
"[signal]" is the signal to be sent to the init process.

EXAMPLE:
For example, if the container id is "ubuntu01" the following will send a "KILL"
signal to the init process of the "ubuntu01" container:

       # runhcs kill ubuntu01 KILL`,
	Flags:  []cli.Flag{},
	Before: appargs.Validate(argID, appargs.Optional(appargs.String)),
	Action: func(context *cli.Context) error {
		id := context.Args().First()
		c, err := getContainer(id, true)
		if err != nil {
			return err
		}
		defer c.Close()
		status, err := c.Status()
		if err != nil {
			return err
		}
		if status != containerRunning {
			return errContainerStopped
		}

		signalsSupported := false

		// The Signal feature was added in RS5
		if osversion.Get().Build >= osversion.RS5 {
			if c.IsHost || c.HostID != "" {
				var hostID string
				if c.IsHost {
					// This is the LCOW, Pod Sandbox, or Windows Xenon V2 for RS5+
					hostID = vmID(c.ID)
				} else {
					// This is the Nth container in a Pod
					hostID = c.HostID
				}
				uvm, err := hcs.OpenComputeSystem(hostID)
				if err != nil {
					return err
				}
				defer uvm.Close()
				if props, err := uvm.Properties(schema1.PropertyTypeGuestConnection); err == nil &&
					props.GuestConnectionInfo.GuestDefinedCapabilities.SignalProcessSupported {
					signalsSupported = true
				}
			} else if c.Spec.Linux == nil && c.Spec.Windows.HyperV == nil {
				// RS5+ Windows Argon
				signalsSupported = true
			}
		}

		var sigOptions interface{}
		if signalsSupported {
			sigStr := context.Args().Get(1)
			if c.Spec.Linux == nil {
				opts, err := signals.ValidateSigstrWCOW(sigStr, signalsSupported)
				if err != nil {
					return err
				}
				sigOptions = opts
			} else {
				opts, err := signals.ValidateSigstrLCOW(sigStr, signalsSupported)
				if err != nil {
					return err
				}
				sigOptions = opts
			}
		}

		var pid int
		if err := stateKey.Get(id, keyInitPid, &pid); err != nil {
			return err
		}

		p, err := c.hc.OpenProcess(pid)
		if err != nil {
			return err
		}
		defer p.Close()

		if signalsSupported && sigOptions != nil && (c.Spec.Linux != nil || !c.Spec.Process.Terminal) {
			return p.Signal(sigOptions)
		}

		// Legacy signal issue a kill
		return p.Kill()
	},
}
