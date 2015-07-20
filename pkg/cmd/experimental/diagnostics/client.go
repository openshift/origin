package diagnostics

import (
	"fmt"

	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	clientdiagnostics "github.com/openshift/origin/pkg/diagnostics/client"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

const (
	ConfigContexts = "ConfigContexts"
)

var (
	AvailableClientDiagnostics = util.NewStringSet(ConfigContexts) // add more diagnostics as they are defined
)

func (o DiagnosticsOptions) buildClientDiagnostics(rawConfig *clientcmdapi.Config) ([]types.Diagnostic, bool /* ok */, error) {

	osClient, kubeClient, clientErr := o.Factory.Clients()
	_ = osClient   // remove once a diagnostic makes use of OpenShift client
	_ = kubeClient // remove once a diagnostic makes use of kube client
	if clientErr != nil {
		o.Logger.Notice("clLoadDefaultFailed", "Failed creating client from config; client diagnostics will be limited to config testing")
		AvailableClientDiagnostics = util.NewStringSet(ConfigContexts)
	}

	diagnostics := []types.Diagnostic{}
	requestedDiagnostics := intersection(util.NewStringSet(o.RequestedDiagnostics...), AvailableClientDiagnostics).List()
	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {
		case ConfigContexts:
			for contextName := range rawConfig.Contexts {
				diagnostics = append(diagnostics, clientdiagnostics.ConfigContext{rawConfig, contextName})
			}

		default:
			return nil, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}
	return diagnostics, true, clientErr
}
