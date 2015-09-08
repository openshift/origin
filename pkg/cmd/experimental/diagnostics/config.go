package diagnostics

import (
	"github.com/openshift/origin/pkg/cmd/cli/config"
	clientcmdapi "k8s.io/kubernetes/pkg/client/clientcmd/api"

	clientdiagnostics "github.com/openshift/origin/pkg/diagnostics/client"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// determine if we even have a client config
func (o DiagnosticsOptions) detectClientConfig() (bool, []types.DiagnosticError, []types.DiagnosticError) {
	diagnostic := &clientdiagnostics.ConfigLoading{ConfFlagName: config.OpenShiftConfigFlagName, ClientFlags: o.ClientFlags}
	o.Logger.Notice("CED2011", "Determining if client configuration exists for client/cluster diagnostics")
	result := diagnostic.Check()
	for _, entry := range result.Logs() {
		o.Logger.LogEntry(entry)
	}
	return diagnostic.SuccessfulLoad(), result.Warnings(), result.Errors()
}

// use the base factory to return a raw config (not specific to a context)
func (o DiagnosticsOptions) buildRawConfig() (*clientcmdapi.Config, error) {
	kubeConfig, configErr := o.Factory.OpenShiftClientConfig.RawConfig()
	if len(kubeConfig.Contexts) == 0 {
		return nil, configErr
	}
	return &kubeConfig, configErr
}
