package diagnostics

import (
	"fmt"
	"strings"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	clientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"

	clustdiags "github.com/openshift/origin/pkg/diagnostics/cluster"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

var (
	AvailableClusterDiagnostics = util.NewStringSet("NodeDefinitions")
)

func (o DiagnosticsOptions) buildClusterDiagnostics(rawConfig *clientcmdapi.Config) ([]types.Diagnostic, bool /* ok */, error) {
	requestedDiagnostics := intersection(util.NewStringSet(o.RequestedDiagnostics...), AvailableClusterDiagnostics).List()
	if len(requestedDiagnostics) == 0 { // no diagnostics to run here
		return nil, true, nil // don't waste time on discovery
	}

	var clusterClient *client.Client
	var kclusterClient *kclient.Client

	clusterClient, kclusterClient, found, err := o.findClusterClients(rawConfig)
	if !found {
		o.Logger.Notice("noClustCtx", "No cluster-admin client config found; skipping cluster diagnostics.")
		return nil, false, err
	}

	diagnostics := []types.Diagnostic{}
	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {
		case "NodeDefinitions":
			diagnostics = append(diagnostics, clustdiags.NodeDefinitions{kclusterClient})

		default:
			return nil, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}
	return diagnostics, true, nil
}

func (o DiagnosticsOptions) findClusterClients(rawConfig *clientcmdapi.Config) (*client.Client, *kclient.Client, bool, error) {
	if o.ClientClusterContext != "" { // user has specified cluster context to use
		if context, exists := rawConfig.Contexts[o.ClientClusterContext]; exists {
			configErr := fmt.Errorf("Specified '%s' as cluster-admin context, but it was not found in your client configuration.", o.ClientClusterContext)
			o.Logger.Error("discClustCtx", configErr.Error())
			return nil, nil, false, configErr
		} else if os, kube, found, err := o.makeClusterClients(rawConfig, o.ClientClusterContext, context); found {
			return os, kube, true, err
		} else {
			return nil, nil, false, err
		}
	}
	currentContext, exists := rawConfig.Contexts[rawConfig.CurrentContext]
	if !exists { // config specified cluster admin context that doesn't exist; complain and quit
		configErr := fmt.Errorf("Current context '%s' not found in client configuration; will not attempt cluster diagnostics.", rawConfig.CurrentContext)
		o.Logger.Errorf("discClustCtx", configErr.Error())
		return nil, nil, false, configErr
	}
	// check if current context is already cluster admin
	if os, kube, found, err := o.makeClusterClients(rawConfig, rawConfig.CurrentContext, currentContext); found {
		return os, kube, true, err
	}
	// otherwise, for convenience, search for a context with the same server but with the system:admin user
	for name, context := range rawConfig.Contexts {
		if context.Cluster == currentContext.Cluster && name != rawConfig.CurrentContext && strings.HasPrefix(context.AuthInfo, "system:admin/") {
			if os, kube, found, err := o.makeClusterClients(rawConfig, name, context); found {
				return os, kube, true, err
			} else {
				return nil, nil, false, err // don't try more than one such context, they'll probably fail the same
			}
		}
	}
	return nil, nil, false, nil
}

func (o DiagnosticsOptions) makeClusterClients(rawConfig *clientcmdapi.Config, contextName string, context *clientcmdapi.Context) (*client.Client, *kclient.Client, bool, error) {
	overrides := &clientcmd.ConfigOverrides{Context: *context}
	clientConfig := clientcmd.NewDefaultClientConfig(*rawConfig, overrides)
	factory := osclientcmd.NewFactory(clientConfig)
	o.Logger.Debugf("discClustCtxStart", "Checking if context is cluster-admin: '%s'", contextName)
	if osClient, kubeClient, err := factory.Clients(); err != nil {
		o.Logger.Debugf("discClustCtx", "Error creating client for context '%s':\n%v", contextName, err)
		return nil, nil, false, nil
	} else {
		subjectAccessReview := authorizationapi.SubjectAccessReview{
			// we assume if you can list nodes, you're the cluster admin.
			Verb:     "list",
			Resource: "nodes",
		}
		if resp, err := osClient.SubjectAccessReviews("default").Create(&subjectAccessReview); err != nil {
			o.Logger.Errorf("discClustCtx", "Error testing cluster-admin access for context '%s':\n%v", contextName, err)
			return nil, nil, false, err
		} else if resp.Allowed {
			o.Logger.Infof("discClustCtxFound", "Using context for cluster-admin access: '%s'", contextName)
			return osClient, kubeClient, true, nil
		}
	}
	o.Logger.Debugf("discClustCtx", "Context does not have cluster-admin access: '%s'", contextName)
	return nil, nil, false, nil
}
