package diagnostics

import (
	"fmt"

	"k8s.io/kubernetes/pkg/util/sets"

	hostdiags "github.com/openshift/origin/pkg/diagnostics/host"
	systemddiags "github.com/openshift/origin/pkg/diagnostics/systemd"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

var (
	// availableHostDiagnostics contains the names of host diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availableHostDiagnostics = sets.NewString(systemddiags.AnalyzeLogsName, systemddiags.UnitStatusName, hostdiags.MasterConfigCheckName, hostdiags.NodeConfigCheckName)
)

// buildHostDiagnostics builds host Diagnostic objects based on the host environment.
// Returns the Diagnostics built, "ok" bool for whether to proceed or abort, and an error if any was encountered during the building of diagnostics.) {
func (o DiagnosticsOptions) buildHostDiagnostics() ([]types.Diagnostic, bool, error) {
	requestedDiagnostics := availableHostDiagnostics.Intersection(sets.NewString(o.RequestedDiagnostics...)).List()
	if len(requestedDiagnostics) == 0 { // no diagnostics to run here
		return nil, true, nil // don't waste time on discovery
	}
	isHost := o.IsHost
	if len(o.MasterConfigLocation) > 0 || len(o.NodeConfigLocation) > 0 {
		isHost = true
	}

	// If we're not looking at a host, don't try the diagnostics
	if !isHost {
		return nil, true, nil
	}

	diagnostics := []types.Diagnostic{}
	systemdUnits := systemddiags.GetSystemdUnits(o.Logger)
	for _, diagnosticName := range requestedDiagnostics {
		var d types.Diagnostic
		switch diagnosticName {
		case systemddiags.AnalyzeLogsName:
			d = systemddiags.AnalyzeLogs{SystemdUnits: systemdUnits}
		case systemddiags.UnitStatusName:
			d = systemddiags.UnitStatus{SystemdUnits: systemdUnits}
		case hostdiags.MasterConfigCheckName:
			d = hostdiags.MasterConfigCheck{MasterConfigFile: o.MasterConfigLocation}
		case hostdiags.NodeConfigCheckName:
			d = hostdiags.NodeConfigCheck{NodeConfigFile: o.NodeConfigLocation}
		default:
			return diagnostics, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
		diagnostics = append(diagnostics, d)
	}

	return diagnostics, true, nil
}
