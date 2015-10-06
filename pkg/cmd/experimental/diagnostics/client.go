package diagnostics

import (
	"fmt"

	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/util/sets"

	clientdiags "github.com/openshift/origin/pkg/diagnostics/client"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

var (
	// availableClientDiagnostics contains the names of client diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availableClientDiagnostics = sets.NewString(clientdiags.ConfigContextsName,
		clientdiags.LatestBuildsName)
)

// buildClientDiagnostics builds client Diagnostic objects based on the rawConfig passed in.
// Returns the Diagnostics built, "ok" bool for whether to proceed or abort, and an error if any was encountered during the building of diagnostics.) {
func (o DiagnosticsOptions) buildClientDiagnostics(rawConfig *clientcmdapi.Config) ([]types.Diagnostic, bool, error) {
	available := availableClientDiagnostics

	// osClient, kubeClient, clientErr := o.Factory.Clients() // use with a diagnostic that needs OpenShift/Kube client
	_, _, clientErr := o.Factory.Clients()
	if clientErr != nil {
		o.Logger.Notice("CED0001", "Failed creating client from config; client diagnostics will be limited to config testing")
		available = sets.NewString(clientdiags.ConfigContextsName)
	}

	diagnostics := []types.Diagnostic{}
	requestedDiagnostics := intersection(sets.NewString(o.RequestedDiagnostics...), available).List()

	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {
		case clientdiags.ConfigContextsName:
			for contextName := range rawConfig.Contexts {
				diagnostics = append(diagnostics, clientdiags.ConfigContext{RawConfig: rawConfig, ContextName: contextName})
			}
		case clientdiags.LatestBuildsName:
			diagnostics = append(diagnostics, &clientdiags.LatestBuilds{RawConfig: rawConfig})
		default:
			return nil, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}
	return diagnostics, true, clientErr
}
