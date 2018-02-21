package diagnostics

import (
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	clientdiags "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/client"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/client/pod"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
)

// availableClientDiagnostics returns definitions of client diagnostics that can be executed
// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
func availableClientDiagnostics() types.DiagnosticList {
	return types.DiagnosticList{clientdiags.ConfigContext{}, &pod.DiagnosticPod{}}
}

// buildClientDiagnostics builds client Diagnostic objects based on the rawConfig passed in.
// Returns the Diagnostics built, and any fatal errors encountered during the building of diagnostics.
func (o DiagnosticsOptions) buildClientDiagnostics(rawConfig *clientcmdapi.Config) ([]types.Diagnostic, error) {
	available := availableClientDiagnostics().Names()

	kubeClient, clientErr := o.Factory.ClientSet()
	if clientErr != nil {
		o.Logger().Notice("CED0001", "Could not configure a client, so client diagnostics are limited to testing configuration and connection")
		available = sets.NewString(clientdiags.ConfigContextsName)
	}

	diagnostics := []types.Diagnostic{}
	requestedDiagnostics := available.Intersection(sets.NewString(o.RequestedDiagnostics.List()...)).List()
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
		case pod.DiagnosticPodName:
			dp := o.ParameterizedDiagnostics[diagnosticName].(*pod.DiagnosticPod)
			dp.KubeClient = kubeClient
			dp.Namespace = rawConfig.Contexts[rawConfig.CurrentContext].Namespace
			dp.Level = o.LogOptions.Level
			dp.Factory = o.Factory
			dp.PreventModification = dp.PreventModification || o.PreventModification
			diagnostics = append(diagnostics, dp)
		default:
			return nil, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}
	return diagnostics, clientErr
}
