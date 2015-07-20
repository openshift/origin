package diagnostics

import (
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/openshift/origin/pkg/cmd/cli/config"

	clientdiagnostics "github.com/openshift/origin/pkg/diagnostics/client"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

func (o DiagnosticsOptions) detectClientConfig() (bool, []types.DiagnosticError, []types.DiagnosticError) {
	diagnostic := &clientdiagnostics.ConfigLoading{ConfFlagName: config.OpenShiftConfigFlagName, ClientFlags: o.ClientFlags}
	o.Logger.Noticet("diagRun", "Determining if client configuration exists for client/cluster diagnostics",
		log.Hash{"area": "client", "name": diagnostic.Name(), "diag": diagnostic.Description()})
	result := diagnostic.Check()
	for _, entry := range result.Logs() {
		o.Logger.LogEntry(entry)
	}
	return diagnostic.SuccessfulLoad(), result.Warnings(), result.Errors()
}

func (o DiagnosticsOptions) buildRawConfig() (*clientcmdapi.Config, error) {
	kubeConfig, configErr := o.Factory.OpenShiftClientConfig.RawConfig()
	if len(kubeConfig.Contexts) == 0 {
		return nil, configErr
	}
	return &kubeConfig, configErr
}
