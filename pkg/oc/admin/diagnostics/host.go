package diagnostics

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	hostdiags "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/host"
	systemddiags "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/systemd"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
)

var (
	// defaultSkipHostDiagnostics is a list of diagnostics to skip by default
	defaultSkipHostDiagnostics = sets.NewString(
		hostdiags.EtcdWriteName,
	)
)

// availableHostDiagnostics returns host diagnostics that can be executed
// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
func availableHostDiagnostics() types.DiagnosticList {
	return types.DiagnosticList{
		&systemddiags.AnalyzeLogs{},
		&systemddiags.UnitStatus{},
		&hostdiags.EtcdWriteVolume{},
	}
}

// buildHostDiagnostics builds host Diagnostic objects based on the host environment.
// Returns the Diagnostics built, and an error if any was encountered during the building of diagnostics.
func (o DiagnosticsOptions) buildHostDiagnostics() ([]types.Diagnostic, error) {
	requestedDiagnostics := availableHostDiagnostics().Names().Intersection(sets.NewString(o.RequestedDiagnostics.List()...)).List()
	if len(requestedDiagnostics) == 0 { // no diagnostics to run here
		return nil, nil // don't waste time on discovery
	}
	isHost := o.IsHost
	if len(o.MasterConfigLocation) > 0 || len(o.NodeConfigLocation) > 0 {
		isHost = true
	}

	// If we're not looking at a host, don't try the diagnostics
	if !isHost {
		return nil, nil
	}

	diagnostics := []types.Diagnostic{}
	systemdUnits := systemddiags.GetSystemdUnits(o.Logger())
	for _, diagnosticName := range requestedDiagnostics {
		var d types.Diagnostic
		switch diagnosticName {
		case systemddiags.AnalyzeLogsName:
			d = &systemddiags.AnalyzeLogs{SystemdUnits: systemdUnits}
		case systemddiags.UnitStatusName:
			d = &systemddiags.UnitStatus{SystemdUnits: systemdUnits}
		case hostdiags.EtcdWriteName:
			etcd := o.ParameterizedDiagnostics[hostdiags.EtcdWriteName].(*hostdiags.EtcdWriteVolume)
			etcd.MasterConfigLocation = o.MasterConfigLocation
			d = etcd
		default:
			return diagnostics, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
		diagnostics = append(diagnostics, d)
	}

	return diagnostics, nil
}
