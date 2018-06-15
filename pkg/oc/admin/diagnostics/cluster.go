package diagnostics

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	clientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	oauthorizationclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	networkclient "github.com/openshift/origin/pkg/network/generated/internalclientset"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	clustdiags "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster"
	agldiags "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/aggregated_logging"
	appcreate "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/app_create"
	networkdiags "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/network"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset"
	routeclient "github.com/openshift/origin/pkg/route/generated/internalclientset"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
	"k8s.io/kubernetes/pkg/apis/authorization"
)

// availableClusterDiagnostics contains the names of cluster diagnostics that can be executed
// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
func availableClusterDiagnostics() types.DiagnosticList {
	return types.DiagnosticList{
		&agldiags.AggregatedLogging{},
		appcreate.NewDefaultAppCreateDiagnostic(),
		&clustdiags.ClusterRegistry{},
		&clustdiags.ClusterRouter{},
		&clustdiags.ClusterRoles{},
		&clustdiags.ClusterRoleBindings{},
		&clustdiags.MasterNode{},
		&clustdiags.MetricsApiProxy{},
		&clustdiags.NodeDefinitions{},
		&clustdiags.RouteCertificateValidation{},
		&clustdiags.ServiceExternalIPs{},
		&networkdiags.NetworkDiagnostic{},
	}
}

// buildClusterDiagnostics builds cluster Diagnostic objects if a cluster-admin client can be extracted from the rawConfig passed in.
// Returns the Diagnostics built and any fatal error encountered during the building of diagnostics.
func (o DiagnosticsOptions) buildClusterDiagnostics(rawConfig *clientcmdapi.Config) ([]types.Diagnostic, error) {
	requestedDiagnostics := availableClusterDiagnostics().Names().Intersection(sets.NewString(o.RequestedDiagnostics.List()...)).List()
	if len(requestedDiagnostics) == 0 { // no diagnostics to run here
		return nil, nil // don't waste time on discovery
	}

	var kclusterClient kclientset.Interface

	config, kclusterClient, rawAdminConfig, err := o.findClusterClients(rawConfig)
	if err != nil {
		return nil, err
	}
	if config == nil {
		o.Logger().Notice("CED1002", "Could not configure a client with cluster-admin permissions for the current server, so cluster diagnostics will be skipped")
		return nil, nil
	}
	imageClient, err := imageclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	projectClient, err := projectclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	routeClient, err := routeclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	appsClient, err := appsclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	oauthClient, err := oauthclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	oauthorizationClient, err := oauthorizationclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	securityClient, err := securityclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	networkClient, err := networkclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	diagnostics := []types.Diagnostic{}
	for _, diagnosticName := range requestedDiagnostics {
		var d types.Diagnostic
		switch diagnosticName {
		case agldiags.AggregatedLoggingName:
			p := o.ParameterizedDiagnostics[agldiags.AggregatedLoggingName].(*agldiags.AggregatedLogging).Project
			d = agldiags.NewAggregatedLogging(p, kclusterClient, oauthClient.Oauth(), projectClient.Project(), routeClient.Route(), oauthorizationClient.Authorization(), appsClient.Apps(), securityClient.Security())
		case appcreate.AppCreateName:
			ac := o.ParameterizedDiagnostics[diagnosticName].(*appcreate.AppCreate)
			ac.KubeClient = kclusterClient
			ac.ProjectClient = projectClient.Project()
			ac.RouteClient = routeClient
			ac.RoleBindingClient = oauthorizationClient.Authorization()
			ac.SARClient = kclusterClient.Authorization()
			ac.AppsClient = appsClient
			ac.PreventModification = o.PreventModification
			d = ac
		case clustdiags.NodeDefinitionsName:
			d = &clustdiags.NodeDefinitions{KubeClient: kclusterClient}
		case clustdiags.MasterNodeName:
			serverUrl := rawAdminConfig.Clusters[rawAdminConfig.Contexts[rawAdminConfig.CurrentContext].Cluster].Server
			d = &clustdiags.MasterNode{KubeClient: kclusterClient, ServerUrl: serverUrl, MasterConfigFile: o.MasterConfigLocation}
		case clustdiags.ClusterRegistryName:
			d = &clustdiags.ClusterRegistry{KubeClient: kclusterClient, ImageStreamClient: imageClient.Image(), PreventModification: o.PreventModification}
		case clustdiags.ClusterRouterName:
			d = &clustdiags.ClusterRouter{KubeClient: kclusterClient, DCClient: appsClient.Apps()}
		case clustdiags.ClusterRolesName:
			d = &clustdiags.ClusterRoles{ClusterRolesClient: oauthorizationClient.Authorization().ClusterRoles(), SARClient: kclusterClient.Authorization()}
		case clustdiags.ClusterRoleBindingsName:
			d = &clustdiags.ClusterRoleBindings{ClusterRoleBindingsClient: oauthorizationClient.Authorization().ClusterRoleBindings(), SARClient: kclusterClient.Authorization()}
		case clustdiags.MetricsApiProxyName:
			d = &clustdiags.MetricsApiProxy{KubeClient: kclusterClient}
		case clustdiags.ServiceExternalIPsName:
			d = &clustdiags.ServiceExternalIPs{MasterConfigFile: o.MasterConfigLocation, KclusterClient: kclusterClient}
		case clustdiags.RouteCertificateValidationName:
			d = &clustdiags.RouteCertificateValidation{SARClient: kclusterClient.Authorization(), RESTConfig: config}
		case networkdiags.NetworkDiagnosticName:
			nd := o.ParameterizedDiagnostics[diagnosticName].(*networkdiags.NetworkDiagnostic)
			nd.KubeClient = kclusterClient
			nd.NetNamespacesClient = networkClient.Network()
			nd.ClusterNetworkClient = networkClient.Network()
			nd.ClientFlags = o.ClientFlags
			nd.Level = o.LogOptions.Level
			nd.Factory = o.Factory
			nd.RawConfig = rawAdminConfig
			nd.PreventModification = o.PreventModification
			d = nd
		default:
			return nil, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
		diagnostics = append(diagnostics, d)
	}
	return diagnostics, nil
}

// attempts to find which context in the config might be a cluster-admin for the server in the current context.
// returns openshift client config for the context chosen, kclusterClient and raw config of same, and any fatal error
func (o DiagnosticsOptions) findClusterClients(rawConfig *clientcmdapi.Config) (*rest.Config, kclientset.Interface, *clientcmdapi.Config, error) {
	if o.ClientClusterContext != "" { // user has specified cluster context to use
		context, exists := rawConfig.Contexts[o.ClientClusterContext]
		if !exists {
			configErr := fmt.Errorf("Specified '%s' as cluster-admin context, but it was not found in your client configuration.", o.ClientClusterContext)
			o.Logger().Error("CED1003", configErr.Error())
			return nil, nil, nil, configErr
		}
		return o.makeClusterClients(rawConfig, o.ClientClusterContext, context)
	}
	currentContext, exists := rawConfig.Contexts[rawConfig.CurrentContext]
	if !exists { // config specified cluster admin context that doesn't exist; complain and quit
		configErr := fmt.Errorf("Current context '%s' not found in client configuration; will not attempt cluster diagnostics.", rawConfig.CurrentContext)
		o.Logger().Error("CED1004", configErr.Error())
		return nil, nil, nil, configErr
	}

	// check if current context is already cluster admin
	config, kube, rawAdminConfig, err := o.makeClusterClients(rawConfig, rawConfig.CurrentContext, currentContext)
	if err == nil && config != nil {
		return config, kube, rawAdminConfig, nil
	}

	// otherwise, for convenience, search for a context with the same server but with the system:admin user
	for name, context := range rawConfig.Contexts {
		if context.Cluster == currentContext.Cluster && name != rawConfig.CurrentContext && strings.HasPrefix(context.AuthInfo, "system:admin/") {
			config, kube, rawAdminConfig, err := o.makeClusterClients(rawConfig, name, context)
			if err != nil || config == nil {
				break // don't try more than one such context, they'll probably fail the same
			}
			return config, kube, rawAdminConfig, nil
		}
	}
	return nil, nil, nil, nil
}

// makes the client from the specified context and determines whether it is a cluster-admin.
func (o DiagnosticsOptions) makeClusterClients(rawConfig *clientcmdapi.Config, contextName string, context *clientcmdapi.Context) (*rest.Config, kclientset.Interface, *clientcmdapi.Config, error) {
	overrides := &clientcmd.ConfigOverrides{Context: *context}
	clientConfig := clientcmd.NewDefaultClientConfig(*rawConfig, overrides)
	factory := osclientcmd.NewFactory(clientConfig)

	// create a config for making openshift clients
	config, err := factory.ClientConfig()
	if err != nil {
		o.Logger().Debug("CED1006", fmt.Sprintf("Error creating client config for context '%s':\n%v", contextName, err))
		return nil, nil, nil, nil
	}

	// create a kube client
	kubeClient, err := factory.ClientSet()
	if err != nil {
		o.Logger().Debug("CED1006", fmt.Sprintf("Error creating kube client for context '%s':\n%v", contextName, err))
		return nil, nil, nil, nil
	}

	o.Logger().Debug("CED1005", fmt.Sprintf("Checking if context is cluster-admin: '%s'", contextName))
	subjectAccessReview := &authorization.SelfSubjectAccessReview{
		Spec: authorization.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorization.ResourceAttributes{
				// if you can do everything, you're the cluster admin.
				Verb:     "*",
				Group:    "*",
				Resource: "*",
			},
		},
	}
	resp, err := kubeClient.Authorization().SelfSubjectAccessReviews().Create(subjectAccessReview)
	if err != nil && regexp.MustCompile(`User "[\w:]+" cannot create \w+ at the cluster scope`).MatchString(err.Error()) {
		o.Logger().Debug("CED1007", fmt.Sprintf("Context '%s' does not have cluster-admin access:\n%v", contextName, err))
		return nil, nil, nil, nil
	}
	if err != nil {
		o.Logger().Error("CED1008", fmt.Sprintf("Unknown error testing cluster-admin access for context '%s':\n%v", contextName, err))
		return nil, nil, nil, err
	}
	if !resp.Status.Allowed {
		o.Logger().Debug("CED1010", fmt.Sprintf("Context does not have cluster-admin access: '%s'", contextName))
		return nil, nil, nil, nil
	}

	o.Logger().Info("CED1009", fmt.Sprintf("Using context for cluster-admin access: '%s'", contextName))
	adminConfig := rawConfig.DeepCopy()
	adminConfig.CurrentContext = contextName
	if err := clientcmdapi.MinifyConfig(adminConfig); err != nil {
		return nil, nil, nil, err
	}
	if err := clientcmdapi.FlattenConfig(adminConfig); err != nil {
		return nil, nil, nil, err
	}
	return config, kubeClient, adminConfig, nil
}
