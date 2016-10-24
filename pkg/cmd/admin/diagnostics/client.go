package diagnostics

import (
	"fmt"

	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/util/sets"

	clientdiags "github.com/openshift/origin/pkg/diagnostics/client"
	networkdiags "github.com/openshift/origin/pkg/diagnostics/network"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

var (
	// availableClientDiagnostics contains the names of client diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availableClientDiagnostics = sets.NewString(clientdiags.ConfigContextsName, clientdiags.DiagnosticPodName, networkdiags.NetworkDiagnosticName)
)

// buildClientDiagnostics builds client Diagnostic objects based on the rawConfig passed in.
// Returns the Diagnostics built, "ok" bool for whether to proceed or abort, and an error if any was encountered during the building of diagnostics.) {
func (o DiagnosticsOptions) buildClientDiagnostics(rawConfig *clientcmdapi.Config) ([]types.Diagnostic, bool, error) {
	available := availableClientDiagnostics

	osClient, kubeClient, clientErr := o.Factory.Clients()
	if clientErr != nil {
		o.Logger.Notice("CED0001", "Could not configure a client, so client diagnostics are limited to testing configuration and connection")
		available = sets.NewString(clientdiags.ConfigContextsName)
	}

	diagnostics := []types.Diagnostic{}
	requestedDiagnostics := available.Intersection(sets.NewString(o.RequestedDiagnostics...)).List()
	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {
		case clientdiags.ConfigContextsName:
			seen := map[string]bool{}
			for contextName := range rawConfig.Contexts {
				diagnostic := clientdiags.ConfigContext{RawConfig: rawConfig, ContextName: contextName}
				if clusterUser, defined := diagnostic.ContextClusterUser(); !defined {
					// definitely want to diagnose the broken context
					diagnostics = append(diagnostics, diagnostic)
				} else if !seen[clusterUser] {
					seen[clusterUser] = true // avoid validating same user for multiple projects
					diagnostics = append(diagnostics, diagnostic)
				}
			}
		case clientdiags.DiagnosticPodName:
			diagnostics = append(diagnostics, &clientdiags.DiagnosticPod{
				KubeClient:          *kubeClient,
				Namespace:           rawConfig.Contexts[rawConfig.CurrentContext].Namespace,
				Level:               o.LogOptions.Level,
				Factory:             o.Factory,
				PreventModification: o.PreventModification,
				ImageTemplate:       o.ImageTemplate,
			})
		case networkdiags.NetworkDiagnosticName:
			diagnostics = append(diagnostics, &networkdiags.NetworkDiagnostic{
				KubeClient:          kubeClient,
				OSClient:            osClient,
				ClientFlags:         o.ClientFlags,
				Level:               o.LogOptions.Level,
				Factory:             o.Factory,
				PreventModification: o.PreventModification,
				LogDir:              o.NetworkDiagLogDir,
			})
		default:
			return nil, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}
	return diagnostics, true, clientErr
}
