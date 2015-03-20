package origin

import (
	"crypto/x509"
	"fmt"
	"net/http"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/request/bearertoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/paramtoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	"github.com/openshift/origin/pkg/auth/group"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	policycache "github.com/openshift/origin/pkg/authorization/cache"
	authorizationetcd "github.com/openshift/origin/pkg/authorization/registry/etcd"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	projectauth "github.com/openshift/origin/pkg/project/auth"

	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

const (
	unauthenticatedUsername = "system:anonymous"

	authenticatedGroup   = "system:authenticated"
	unauthenticatedGroup = "system:unauthenticated"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	Options configapi.MasterConfig

	Authenticator                 authenticator.Request
	Authorizer                    authorizer.Authorizer
	AuthorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder

	PolicyCache               *policycache.PolicyCache
	ProjectAuthorizationCache *projectauth.AuthorizationCache

	// Map requests to contexts
	RequestContextMapper kapi.RequestContextMapper

	AdmissionControl admission.Interface

	TLS bool

	// a function that returns the appropriate image to use for a named component
	ImageFor func(component string) string

	EtcdHelper          tools.EtcdHelper
	KubeletClientConfig *kclient.KubeletConfig

	// ClientCAs will be used to request client certificates in connections to the API.
	// This CertPool should contain all the CAs that will be used for client certificate verification.
	ClientCAs *x509.CertPool
	// APIClientCAs is used to verify client certificates presented for API auth
	APIClientCAs *x509.CertPool

	// KubeClientConfig is the client configuration used to call Kubernetes APIs from system components.
	// To apply different access control to a system component, create a client config specifically for that component.
	KubeClientConfig kclient.Config
	// OSClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	OSClientConfig kclient.Config
	// DeployerOSClientConfig is the client configuration used to call OpenShift APIs from launched deployer pods
	DeployerOSClientConfig kclient.Config

	// kubeClient is the client used to call Kubernetes APIs from system components, built from KubeClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	KubernetesClient *kclient.Client
	// osClient is the client used to call OpenShift APIs from system components, built from OSClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	OSClient *osclient.Client
}

func BuildMasterConfig(options configapi.MasterConfig) (*MasterConfig, error) {

	etcdHelper, err := etcd.NewOpenShiftEtcdHelper(options.EtcdClientInfo)
	if err != nil {
		return nil, fmt.Errorf("Error setting up server storage: %v", err)
	}

	clientCAs, err := configapi.GetClientCertCAPool(options)
	if err != nil {
		return nil, err
	}
	apiClientCAs, err := configapi.GetAPIClientCertCAPool(options)
	if err != nil {
		return nil, err
	}

	kubeClient, kubeClientConfig, err := configapi.GetKubeClient(options.MasterClients.KubernetesKubeConfig)
	if err != nil {
		return nil, err
	}
	openshiftClient, osClientConfig, err := configapi.GetOpenShiftClient(options.MasterClients.OpenShiftLoopbackKubeConfig)
	if err != nil {
		return nil, err
	}
	_, deployerOSClientConfig, err := configapi.GetOpenShiftClient(options.MasterClients.DeployerKubeConfig)
	if err != nil {
		return nil, err
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	policyCache := newPolicyCache(etcdHelper)
	requestContextMapper := kapi.NewRequestContextMapper()

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)

	// in-order list of plug-ins that should intercept admission decisions (origin only intercepts)
	admissionControlPluginNames := []string{"AlwaysAdmit"}
	admissionController := admission.NewFromPlugins(kubeClient, admissionControlPluginNames, "")

	config := &MasterConfig{
		Options: options,

		Authenticator:                 newAuthenticator(options.ServingInfo, etcdHelper, apiClientCAs),
		Authorizer:                    newAuthorizer(policyCache, options.PolicyConfig.MasterAuthorizationNamespace),
		AuthorizationAttributeBuilder: newAuthorizationAttributeBuilder(requestContextMapper),

		PolicyCache:               policyCache,
		ProjectAuthorizationCache: newProjectAuthorizationCache(options.PolicyConfig.MasterAuthorizationNamespace, openshiftClient, kubeClient),

		RequestContextMapper: requestContextMapper,

		AdmissionControl: admissionController,

		TLS: configapi.UseTLS(options.ServingInfo),

		ImageFor:            imageTemplate.ExpandOrDie,
		EtcdHelper:          etcdHelper,
		KubeletClientConfig: kubeletClientConfig,

		ClientCAs:    clientCAs,
		APIClientCAs: apiClientCAs,

		KubeClientConfig:       *kubeClientConfig,
		OSClientConfig:         *osClientConfig,
		DeployerOSClientConfig: *deployerOSClientConfig,
		OSClient:               openshiftClient,
		KubernetesClient:       kubeClient,
	}

	return config, nil
}

func newAuthenticator(servingInfo configapi.ServingInfo, etcdHelper tools.EtcdHelper, apiClientCAs *x509.CertPool) authenticator.Request {
	tokenAuthenticator := getEtcdTokenAuthenticator(etcdHelper)

	authenticators := []authenticator.Request{}
	authenticators = append(authenticators, bearertoken.New(tokenAuthenticator, true))
	// Allow token as access_token param for WebSockets
	// TODO: make the param name configurable
	// TODO: limit this authenticator to watch methods, if possible
	// TODO: prevent access_token param from getting logged, if possible
	authenticators = append(authenticators, paramtoken.New("access_token", tokenAuthenticator, true))

	if configapi.UseTLS(servingInfo) {
		// build cert authenticator
		// TODO: add cert users to etcd?
		opts := x509request.DefaultVerifyOptions()
		opts.Roots = apiClientCAs
		certauth := x509request.New(opts, x509request.SubjectToUserConversion)
		authenticators = append(authenticators, certauth)
	}

	// TODO: make anonymous auth optional?
	ret := &unionrequest.Authenticator{
		FailOnError: true,
		Handlers: []authenticator.Request{
			group.NewGroupAdder(unionrequest.NewUnionAuthentication(authenticators...), []string{authenticatedGroup}),
			authenticator.RequestFunc(func(req *http.Request) (user.Info, bool, error) {
				return &user.DefaultInfo{Name: unauthenticatedUsername, Groups: []string{unauthenticatedGroup}}, true, nil
			}),
		},
	}

	return ret
}

func newProjectAuthorizationCache(masterAuthorizationNamespace string, openshiftClient *osclient.Client, kubeClient *kclient.Client) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		projectauth.NewReviewer(openshiftClient),
		kubeClient.Namespaces(),
		openshiftClient,
		openshiftClient,
		masterAuthorizationNamespace)
}

func newPolicyCache(etcdHelper tools.EtcdHelper) *policycache.PolicyCache {
	authorizationEtcd := authorizationetcd.New(etcdHelper)
	return policycache.NewPolicyCache(authorizationEtcd, authorizationEtcd)
}

func newAuthorizer(policyCache *policycache.PolicyCache, masterAuthorizationNamespace string) authorizer.Authorizer {
	authorizer := authorizer.NewAuthorizer(masterAuthorizationNamespace, rulevalidation.NewDefaultRuleResolver(policyCache, policyCache))
	return authorizer
}

func newAuthorizationAttributeBuilder(requestContextMapper kapi.RequestContextMapper) authorizer.AuthorizationAttributeBuilder {
	authorizationAttributeBuilder := authorizer.NewAuthorizationAttributeBuilder(requestContextMapper, &apiserver.APIRequestInfoResolver{kutil.NewStringSet("api", "osapi"), latest.RESTMapper})
	return authorizationAttributeBuilder
}

func getEtcdTokenAuthenticator(etcdHelper tools.EtcdHelper) authenticator.Token {
	oauthRegistry := oauthetcd.New(etcdHelper)
	return authnregistry.NewTokenAuthenticator(oauthRegistry)
}

// KubeClient returns the kubernetes client object
func (c *MasterConfig) KubeClient() *kclient.Client {
	return c.KubernetesClient
}

// PolicyClient returns the policy client object
// It must have the following capabilities:
//  list, watch all policyBindings in all namespaces
//  list, watch all policies in all namespaces
//  create resourceAccessReviews in all namespaces
func (c *MasterConfig) PolicyClient() *osclient.Client {
	return c.OSClient
}

// DeploymentClient returns the deployment client object
func (c *MasterConfig) DeploymentClient() *kclient.Client {
	return c.KubernetesClient
}

// DNSServerClient returns the DNS server client object
// It must have the following capabilities:
//   list, watch all services in all namespaces
func (c *MasterConfig) DNSServerClient() *kclient.Client {
	return c.KubernetesClient
}

// BuildLogClient returns the build log client object
func (c *MasterConfig) BuildLogClient() *kclient.Client {
	return c.KubernetesClient
}

// WebHookClient returns the webhook client object
func (c *MasterConfig) WebHookClient() *osclient.Client {
	return c.OSClient
}

// BuildControllerClients returns the build controller client objects
func (c *MasterConfig) BuildControllerClients() (*osclient.Client, *kclient.Client) {
	return c.OSClient, c.KubernetesClient
}

// ImageChangeControllerClient returns the openshift client object
func (c *MasterConfig) ImageChangeControllerClient() *osclient.Client {
	return c.OSClient
}

// ImageImportControllerClient returns the deployment client object
func (c *MasterConfig) ImageImportControllerClient() *osclient.Client {
	return c.OSClient
}

// DeploymentControllerClients returns the deployment controller client object
func (c *MasterConfig) DeploymentControllerClients() (*osclient.Client, *kclient.Client) {
	return c.OSClient, c.KubernetesClient
}

// DeployerClientConfig returns the client configuration a Deployer instance launched in a pod
// should use when making API calls.
func (c *MasterConfig) DeployerClientConfig() *kclient.Config {
	return &c.DeployerOSClientConfig
}

func (c *MasterConfig) DeploymentConfigControllerClients() (*osclient.Client, *kclient.Client) {
	return c.OSClient, c.KubernetesClient
}
func (c *MasterConfig) DeploymentConfigChangeControllerClients() (*osclient.Client, *kclient.Client) {
	return c.OSClient, c.KubernetesClient
}
func (c *MasterConfig) DeploymentImageChangeControllerClient() *osclient.Client {
	return c.OSClient
}

// OriginNamespaceControllerClients returns a client for openshift and kubernetes.
// The openshift client object must have authority to delete openshift content in any namespace
// The kubernetes client object must have authority to execute a finalize request on a namespace
func (c *MasterConfig) OriginNamespaceControllerClients() (*osclient.Client, *kclient.Client) {
	return c.OSClient, c.KubernetesClient
}
