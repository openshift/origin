package systemd

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/openshift/origin/pkg/diagnostics/types"
)

// UnitStatus is a Diagnostic to check status of systemd units that are related to each other.
type UnitStatus struct {
	SystemdUnits map[string]types.SystemdUnit
}

const UnitStatusName = "UnitStatus"

func (d UnitStatus) Name() string {
	return UnitStatusName
}

func (d UnitStatus) Description() string {
	return "Check status for related systemd units"
}
func (d UnitStatus) CanRun() (bool, error) {
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			return true, nil
		}
	}

	return false, errors.New("systemd is not present on this host")
}
func (d UnitStatus) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(UnitStatusName)

	unitRequiresUnit(r, d.SystemdUnits["openshift-node"], d.SystemdUnits["iptables"], nodeRequiresIPTables)
	unitRequiresUnit(r, d.SystemdUnits["openshift-node"], d.SystemdUnits["docker"], `Nodes use Docker to run containers.`)
	unitRequiresUnit(r, d.SystemdUnits["openshift-node"], d.SystemdUnits["openvswitch"], sdUnitSDNreqOVS)
	unitRequiresUnit(r, d.SystemdUnits["openshift-master"], d.SystemdUnits["openvswitch"], `Masters use openvswitch for access to cluster SDN networking`)
	// all-in-one networking *could* be simpler, so fewer checks
	unitRequiresUnit(r, d.SystemdUnits["openshift"], d.SystemdUnits["docker"], `Nodes use Docker to run containers.`)

	// Anything that is enabled but not running deserves notice
	for name, unit := range d.SystemdUnits {
		if unit.Enabled && !unit.Active {
			r.Error("DS3001", nil, fmt.Sprintf(sdUnitInactive, name))
		}
	}
	return r
}

func unitRequiresUnit(r types.DiagnosticResult, unit types.SystemdUnit, requires types.SystemdUnit, reason string) {

	if (unit.Active || unit.Enabled) && !requires.Exists {
		r.Error("DS3002", nil, fmt.Sprintf(sdUnitReqLoaded, unit.Name, requires.Name, reason))
	} else if unit.Active && !requires.Active {
		r.Error("DS3003", nil, fmt.Sprintf(sdUnitReqActive, unit.Name, requires.Name, reason))
	}
}

func errStr(err error) string {
	return fmt.Sprintf("(%T) %[1]v", err)
}

const (
	nodeRequiresIPTables = `
iptables is used by nodes for container networking.
Connections to a container will fail without it.`

	sdUnitSDNreqOVS = `
systemd unit openshift-node is running but openvswitch is not.
Normally openshift-node starts openvswitch once initialized.
It is likely that openvswitch has crashed or been stopped.

The software-defined network (SDN) enables networking between
containers on different nodes. Containers will not be able to
connect to each other without the openvswitch service carrying
this traffic.

An administrator can start openvswitch with:

  # systemctl start openvswitch

To ensure it is not repeatedly failing to run, check the status and logs with:

  # systemctl status openvswitch
  # journalctl -ru openvswitch `

	sdUnitInactive = `
The %[1]s systemd unit is intended to start at boot but is not currently active.
An administrator can start the %[1]s unit with:

  # systemctl start %[1]s

To ensure it is not failing to run, check the status and logs with:

  # systemctl status %[1]s
  # journalctl -ru %[1]s`

	sdUnitReqLoaded = `
systemd unit %[1]s depends on unit %[2]s, which is not loaded.
%[3]s
An administrator probably needs to install the %[2]s unit with:

  # yum install %[2]s

If it is already installed, you may to reload the definition with:

  # systemctl reload %[2]s
  `

	sdUnitReqActive = `
systemd unit %[1]s is running but %[2]s is not.
%[1]s
An administrator can start the %[2]s unit with:

  # systemctl start %[2]s

To ensure it is not failing to run, check the status and logs with:

  # systemctl status %[2]s
  # journalctl -ru %[2]s
  `
)
