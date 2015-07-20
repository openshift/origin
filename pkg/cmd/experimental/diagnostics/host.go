package diagnostics

import (
	"fmt"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/diagnostics/host"
	systemddiagnostics "github.com/openshift/origin/pkg/diagnostics/systemd"
	"github.com/openshift/origin/pkg/diagnostics/types"
	"os"
)

const (
	StandardMasterConfigPath string = "/etc/openshift/master/master-config.yaml"
	StandardNodeConfigPath   string = "/etc/openshift/node/node-config.yaml"
)

var (
	AvailableHostDiagnostics = util.NewStringSet("AnalyzeLogs", "UnitStatus", "MasterConfigCheck", "NodeConfigCheck")
)

func (o DiagnosticsOptions) buildHostDiagnostics() ([]types.Diagnostic, bool /* ok */, error) {
	requestedDiagnostics := intersection(util.NewStringSet(o.RequestedDiagnostics...), AvailableHostDiagnostics).List()
	if len(requestedDiagnostics) == 0 { // no diagnostics to run here
		return nil, true, nil // don't waste time on discovery
	}
	isHost := o.IsHost
	// check for standard host config paths if not given
	if len(o.MasterConfigLocation) == 0 {
		if _, err := os.Stat(StandardMasterConfigPath); !os.IsNotExist(err) {
			o.MasterConfigLocation = StandardMasterConfigPath
			isHost = true
		}
	} else {
		isHost = true
	}
	if len(o.NodeConfigLocation) == 0 {
		if _, err := os.Stat(StandardNodeConfigPath); !os.IsNotExist(err) {
			o.NodeConfigLocation = StandardNodeConfigPath
			isHost = true
		}
	} else {
		isHost = true
	}

	// If we're not looking at a host, don't try the diagnostics
	if !isHost {
		return nil, true, nil
	}

	diagnostics := []types.Diagnostic{}
	systemdUnits := systemddiagnostics.GetSystemdUnits(o.Logger)
	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {
		case "AnalyzeLogs":
			diagnostics = append(diagnostics, systemddiagnostics.AnalyzeLogs{systemdUnits})

		case "UnitStatus":
			diagnostics = append(diagnostics, systemddiagnostics.UnitStatus{systemdUnits})

		case "MasterConfigCheck":
			if len(o.MasterConfigLocation) > 0 {
				diagnostics = append(diagnostics, host.MasterConfigCheck{o.MasterConfigLocation})
			}

		case "NodeConfigCheck":
			if len(o.NodeConfigLocation) > 0 {
				diagnostics = append(diagnostics, host.NodeConfigCheck{o.NodeConfigLocation})
			}

		default:
			return diagnostics, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}

	return diagnostics, true, nil
}
