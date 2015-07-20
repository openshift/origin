package systemd

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// UnitStatus
type UnitStatus struct {
	SystemdUnits map[string]types.SystemdUnit
}

func (d UnitStatus) Name() string {
	return "UnitStatus"
}

func (d UnitStatus) Description() string {
	return "Check status for OpenShift-related systemd units"
}
func (d UnitStatus) CanRun() (bool, error) {
	if runtime.GOOS == "linux" {
		if _, err := exec.LookPath("systemctl"); err == nil {
			return true, nil
		}
	}

	return false, errors.New("systemd is not present on this host")
}
func (d UnitStatus) Check() *types.DiagnosticResult {
	r := types.NewDiagnosticResult("UnitStatus")

	unitRequiresUnit(r, d.SystemdUnits["openshift-node"], d.SystemdUnits["iptables"], nodeRequiresIPTables)
	unitRequiresUnit(r, d.SystemdUnits["openshift-node"], d.SystemdUnits["docker"], `OpenShift nodes use Docker to run containers.`)
	unitRequiresUnit(r, d.SystemdUnits["openshift-node"], d.SystemdUnits["openvswitch"], sdUnitSDNreqOVS)
	unitRequiresUnit(r, d.SystemdUnits["openshift-master"], d.SystemdUnits["openvswitch"], `OpenShift masters use openvswitch for access to cluster SDN networking`)
	// all-in-one networking *could* be simpler, so fewer checks
	unitRequiresUnit(r, d.SystemdUnits["openshift"], d.SystemdUnits["docker"], `OpenShift nodes use Docker to run containers.`)

	// Anything that is enabled but not running deserves notice
	for name, unit := range d.SystemdUnits {
		if unit.Enabled && !unit.Active {
			r.Errort("sdUnitInactive", nil, sdUnitInactive, log.Hash{"unit": name})
		}
	}
	return r
}

func unitRequiresUnit(r *types.DiagnosticResult, unit types.SystemdUnit, requires types.SystemdUnit, reason string) {
	templateData := log.Hash{"unit": unit.Name, "required": requires.Name, "reason": reason}

	if (unit.Active || unit.Enabled) && !requires.Exists {
		r.Errort("sdUnitReqLoaded", nil, sdUnitReqLoaded, templateData)
	} else if unit.Active && !requires.Active {
		r.Errort("sdUnitReqActive", nil, sdUnitReqActive, templateData)
	}
}

func errStr(err error) string {
	return fmt.Sprintf("(%T) %[1]v", err)
}

const (
	nodeRequiresIPTables = `
iptables is used by OpenShift nodes for container networking.
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
The {{.unit}} systemd unit is intended to start at boot but is not currently active.
An administrator can start the {{.unit}} unit with:

  # systemctl start {{.unit}}

To ensure it is not failing to run, check the status and logs with:

  # systemctl status {{.unit}}
  # journalctl -ru {{.unit}}`

	sdUnitReqLoaded = `
systemd unit {{.unit}} depends on unit {{.required}}, which is not loaded.
{{.reason}}
An administrator probably needs to install the {{.required}} unit with:

  # yum install {{.required}}

If it is already installed, you may to reload the definition with:

  # systemctl reload {{.required}}
  `

	sdUnitReqActive = `
systemd unit {{.unit}} is running but {{.required}} is not.
{{.reason}}
An administrator can start the {{.required}} unit with:

  # systemctl start {{.required}}

To ensure it is not failing to run, check the status and logs with:

  # systemctl status {{.required}}
  # journalctl -ru {{.required}}
  `
)
