package diagnostics

import (
	"fmt"
	"regexp"
	"strings"

	kclient "k8s.io/kubernetes/pkg/client"
	clientcmd "k8s.io/kubernetes/pkg/client/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/clientcmd/api"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/client"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"

	clustdiags "github.com/openshift/origin/pkg/diagnostics/cluster"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

var (
	// availableClusterDiagnostics contains the names of cluster diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availableClusterDiagnostics = util.NewStringSet(clustdiags.NodeDefinitionsName, clustdiags.ClusterRegistryName, clustdiags.ClusterRouterName)
)

// buildClusterDiagnostics builds cluster Diagnostic objects if a cluster-admin client can be extracted from the rawConfig passed in.
// Returns the Diagnostics built, "ok" bool for whether to proceed or abort, and an error if any was encountered during the building of diagnostics.) {
func (o DiagnosticsOptions) buildClusterDiagnostics(rawConfig *clientcmdapi.Config) ([]types.Diagnostic, bool, error) {
	requestedDiagnostics := intersection(util.NewStringSet(o.RequestedDiagnostics...), availableClusterDiagnostics).List()
	if len(requestedDiagnostics) == 0 { // no diagnostics to run here
		return nil, true, nil // don't waste time on discovery
	}

	var (
		clusterClient  *client.Client
		kclusterClient *kclient.Client
	)

	clusterClient, kclusterClient, found, err := o.findClusterClients(rawConfig)
	if !found {
		o.Logger.Notice("CED1002", "No cluster-admin client config found; skipping cluster diagnostics.")
		return nil, true, err
	}

	diagnostics := []types.Diagnostic{}
	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {
		case clustdiags.NodeDefinitionsName:
			diagnostics = append(diagnostics, &clustdiags.NodeDefinitions{kclusterClient, clusterClient})
		case clustdiags.ClusterRegistryName:
			diagnostics = append(diagnostics, &clustdiags.ClusterRegistry{kclusterClient, clusterClient})
		case clustdiags.ClusterRouterName:
			diagnostics = append(diagnostics, &clustdiags.ClusterRouter{kclusterClient, clusterClient})

		default:
			return nil, false, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}
	return diagnostics, true, nil
}

// attempts to find which context in the config might be a cluster-admin for the server in the current context.
func (o DiagnosticsOptions) findClusterClients(rawConfig *clientcmdapi.Config) (*client.Client, *kclient.Client, bool, error) {
	if o.ClientClusterContext != "" { // user has specified cluster context to use
		if context, exists := rawConfig.Contexts[o.ClientClusterContext]; exists {
			configErr := fmt.Errorf("Specified '%s' as cluster-admin context, but it was not found in your client configuration.", o.ClientClusterContext)
			o.Logger.Error("CED1003", configErr.Error())
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
		o.Logger.Error("CED1004", configErr.Error())
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

// makes the client from the specified context and determines whether it is a cluster-admin.
func (o DiagnosticsOptions) makeClusterClients(rawConfig *clientcmdapi.Config, contextName string, context *clientcmdapi.Context) (*client.Client, *kclient.Client, bool, error) {
	overrides := &clientcmd.ConfigOverrides{Context: *context}
	clientConfig := clientcmd.NewDefaultClientConfig(*rawConfig, overrides)
	factory := osclientcmd.NewFactory(clientConfig)
	o.Logger.Debug("CED1005", fmt.Sprintf("Checking if context is cluster-admin: '%s'", contextName))
	if osClient, kubeClient, err := factory.Clients(); err != nil {
		o.Logger.Debug("CED1006", fmt.Sprintf("Error creating client for context '%s':\n%v", contextName, err))
		return nil, nil, false, nil
	} else {
		subjectAccessReview := authorizationapi.SubjectAccessReview{Action: authorizationapi.AuthorizationAttributes{
			// if you can do everything, you're the cluster admin.
			Verb:     "*",
			Resource: "*",
		}}
		if resp, err := osClient.SubjectAccessReviews().Create(&subjectAccessReview); err != nil {
			if regexp.MustCompile(`User "[\w:]+" cannot create \w+ at the cluster scope`).MatchString(err.Error()) {
				o.Logger.Debug("CED1007", fmt.Sprintf("Context '%s' does not have cluster-admin access:\n%v", contextName, err))
				return nil, nil, false, nil
			} else {
				o.Logger.Error("CED1008", fmt.Sprintf("Unknown error testing cluster-admin access for context '%s':\n%v", contextName, err))
				return nil, nil, false, err
			}
		} else if resp.Allowed {
			o.Logger.Info("CED1009", fmt.Sprintf("Using context for cluster-admin access: '%s'", contextName))
			return osClient, kubeClient, true, nil
		}
	}
	o.Logger.Debug("CED1010", fmt.Sprintf("Context does not have cluster-admin access: '%s'", contextName))
	return nil, nil, false, nil
}
