package origin

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net/http"

	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/serviceaccount"
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
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicyetcd "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy/etcd"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	clusterpolicybindingetcd "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding/etcd"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policyetcd "github.com/openshift/origin/pkg/authorization/registry/policy/etcd"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	policybindingetcd "github.com/openshift/origin/pkg/authorization/registry/policybinding/etcd"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	osclient "github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
)

const (
	unauthenticatedUsername = "system:anonymous"
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

	// PrivilegedLoopbackClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	PrivilegedLoopbackClientConfig kclient.Config
	// DeployerPrivilegedLoopbackClientConfig is the client configuration used to call OpenShift APIs from launched deployer pods
	DeployerOSClientConfig kclient.Config

	// kubeClient is the client used to call Kubernetes APIs from system components, built from KubeClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	PrivilegedLoopbackKubernetesClient *kclient.Client
	// osClient is the client used to call OpenShift APIs from system components, built from PrivilegedLoopbackClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	PrivilegedLoopbackOpenShiftClient *osclient.Client
}

func BuildMasterConfig(options configapi.MasterConfig) (*MasterConfig, error) {
	client, err := etcd.GetAndTestEtcdClient(options.EtcdClientInfo)
	if err != nil {
		return nil, err
	}
	etcdHelper, err := NewEtcdHelper(client, options.EtcdStorageConfig.OpenShiftStorageVersion, options.EtcdStorageConfig.OpenShiftStoragePrefix)
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

	privilegedLoopbackKubeClient, _, err := configapi.GetKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig)
	if err != nil {
		return nil, err
	}
	privilegedLoopbackOpenShiftClient, privilegedLoopbackClientConfig, err := configapi.GetOpenShiftClient(options.MasterClients.OpenShiftLoopbackKubeConfig)
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
	admissionControlPluginNames := []string{"OriginNamespaceLifecycle"}
	admissionController := admission.NewFromPlugins(privilegedLoopbackKubeClient, admissionControlPluginNames, "")

	serviceAccountTokenGetter, err := newServiceAccountTokenGetter(options, client)
	if err != nil {
		return nil, err
	}

	config := &MasterConfig{
		Options: options,

		Authenticator:                 newAuthenticator(options, etcdHelper, serviceAccountTokenGetter, apiClientCAs),
		Authorizer:                    newAuthorizer(policyCache, options.ProjectConfig.ProjectRequestMessage),
		AuthorizationAttributeBuilder: newAuthorizationAttributeBuilder(requestContextMapper),

		PolicyCache:               policyCache,
		ProjectAuthorizationCache: newProjectAuthorizationCache(privilegedLoopbackOpenShiftClient, privilegedLoopbackKubeClient),

		RequestContextMapper: requestContextMapper,

		AdmissionControl: admissionController,

		TLS: configapi.UseTLS(options.ServingInfo),

		ImageFor:            imageTemplate.ExpandOrDie,
		EtcdHelper:          etcdHelper,
		KubeletClientConfig: kubeletClientConfig,

		ClientCAs:    clientCAs,
		APIClientCAs: apiClientCAs,

		DeployerOSClientConfig:             *deployerOSClientConfig,
		PrivilegedLoopbackClientConfig:     *privilegedLoopbackClientConfig,
		PrivilegedLoopbackOpenShiftClient:  privilegedLoopbackOpenShiftClient,
		PrivilegedLoopbackKubernetesClient: privilegedLoopbackKubeClient,
	}

	return config, nil
}

func newServiceAccountTokenGetter(options configapi.MasterConfig, client *etcdclient.Client) (serviceaccount.ServiceAccountTokenGetter, error) {
	var tokenGetter serviceaccount.ServiceAccountTokenGetter
	if options.KubernetesMasterConfig == nil {
		// When we're running against an external Kubernetes, use the external kubernetes client to validate service account tokens
		// This prevents infinite auth loops if the privilegedLoopbackKubeClient authenticates using a service account token
		kubeClient, _, err := configapi.GetKubeClient(options.MasterClients.ExternalKubernetesKubeConfig)
		if err != nil {
			return nil, err
		}
		tokenGetter = serviceaccount.NewGetterFromClient(kubeClient)
	} else {
		// When we're running in-process, go straight to etcd (using the KubernetesStorageVersion/KubernetesStoragePrefix, since service accounts are kubernetes objects)
		ketcdHelper, err := master.NewEtcdHelper(client, options.EtcdStorageConfig.KubernetesStorageVersion, options.EtcdStorageConfig.KubernetesStoragePrefix)
		if err != nil {
			return nil, fmt.Errorf("Error setting up Kubernetes server storage: %v", err)
		}
		tokenGetter = serviceaccount.NewGetterFromEtcdHelper(ketcdHelper)
	}
	return tokenGetter, nil
}

func newAuthenticator(config configapi.MasterConfig, etcdHelper tools.EtcdHelper, tokenGetter serviceaccount.ServiceAccountTokenGetter, apiClientCAs *x509.CertPool) authenticator.Request {
	authenticators := []authenticator.Request{}

	// ServiceAccount token
	if len(config.ServiceAccountConfig.PublicKeyFiles) > 0 {
		publicKeys := []*rsa.PublicKey{}
		for _, keyFile := range config.ServiceAccountConfig.PublicKeyFiles {
			publicKey, err := serviceaccount.ReadPublicKey(keyFile)
			if err != nil {
				glog.Fatalf("Error reading service account key file %s: %v", keyFile, err)
			}
			publicKeys = append(publicKeys, publicKey)
		}
		tokenAuthenticator := serviceaccount.JWTTokenAuthenticator(publicKeys, true, tokenGetter)
		authenticators = append(authenticators, bearertoken.New(tokenAuthenticator, true))
	}

	// OAuth token
	if config.OAuthConfig != nil {
		tokenAuthenticator := getEtcdTokenAuthenticator(etcdHelper)
		authenticators = append(authenticators, bearertoken.New(tokenAuthenticator, true))
		// Allow token as access_token param for WebSockets
		authenticators = append(authenticators, paramtoken.New("access_token", tokenAuthenticator, true))
	}

	if configapi.UseTLS(config.ServingInfo) {
		// build cert authenticator
		// TODO: add "system:" prefix in authenticator, limit cert to username
		// TODO: add "system:" prefix to groups in authenticator, limit cert to group name
		opts := x509request.DefaultVerifyOptions()
		opts.Roots = apiClientCAs
		certauth := x509request.New(opts, x509request.SubjectToUserConversion)
		authenticators = append(authenticators, certauth)
	}

	// TODO: make anonymous auth optional?
	ret := &unionrequest.Authenticator{
		FailOnError: true,
		Handlers: []authenticator.Request{
			group.NewGroupAdder(unionrequest.NewUnionAuthentication(authenticators...), []string{bootstrappolicy.AuthenticatedGroup}),
			authenticator.RequestFunc(func(req *http.Request) (user.Info, bool, error) {
				return &user.DefaultInfo{Name: unauthenticatedUsername, Groups: []string{bootstrappolicy.UnauthenticatedGroup}}, true, nil
			}),
		},
	}

	return ret
}

func newProjectAuthorizationCache(openshiftClient *osclient.Client, kubeClient *kclient.Client) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		projectauth.NewReviewer(openshiftClient),
		kubeClient.Namespaces(),
		openshiftClient,
		openshiftClient,
		openshiftClient,
		openshiftClient,
	)
}

func newPolicyCache(etcdHelper tools.EtcdHelper) *policycache.PolicyCache {
	policyRegistry := policyregistry.NewRegistry(policyetcd.NewStorage(etcdHelper))
	policyBindingRegistry := policybindingregistry.NewRegistry(policybindingetcd.NewStorage(etcdHelper))
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterpolicyetcd.NewStorage(etcdHelper))
	clusterPolicyBindingRegistry := clusterpolicybindingregistry.NewRegistry(clusterpolicybindingetcd.NewStorage(etcdHelper))

	return policycache.NewPolicyCache(policyBindingRegistry, policyRegistry, clusterPolicyBindingRegistry, clusterPolicyRegistry)
}

func newAuthorizer(policyCache *policycache.PolicyCache, projectRequestDenyMessage string) authorizer.Authorizer {
	authorizer := authorizer.NewAuthorizer(rulevalidation.NewDefaultRuleResolver(policyCache, policyCache, policyCache, policyCache), authorizer.NewForbiddenMessageResolver(projectRequestDenyMessage))
	return authorizer
}

func newAuthorizationAttributeBuilder(requestContextMapper kapi.RequestContextMapper) authorizer.AuthorizationAttributeBuilder {
	authorizationAttributeBuilder := authorizer.NewAuthorizationAttributeBuilder(requestContextMapper, &apiserver.APIRequestInfoResolver{kutil.NewStringSet("api", "osapi"), latest.RESTMapper})
	return authorizationAttributeBuilder
}

func getEtcdTokenAuthenticator(etcdHelper tools.EtcdHelper) authenticator.Token {
	accessTokenStorage := accesstokenetcd.NewREST(etcdHelper)
	accessTokenRegistry := accesstokenregistry.NewRegistry(accessTokenStorage)

	userStorage := useretcd.NewREST(etcdHelper)
	userRegistry := userregistry.NewRegistry(userStorage)

	return authnregistry.NewTokenAuthenticator(accessTokenRegistry, userRegistry)
}

// KubeClient returns the kubernetes client object
func (c *MasterConfig) KubeClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// PolicyClient returns the policy client object
// It must have the following capabilities:
//  list, watch all policyBindings in all namespaces
//  list, watch all policies in all namespaces
//  create resourceAccessReviews in all namespaces
func (c *MasterConfig) PolicyClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// SdnClient returns the sdn client object
// It must have the capability to get/list/watch/create/delete
// HostSubnets. And have the capability to get ClusterNetwork.
func (c *MasterConfig) SdnClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// DeploymentClient returns the deployment client object
func (c *MasterConfig) DeploymentClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// DNSServerClient returns the DNS server client object
// It must have the following capabilities:
//   list, watch all services in all namespaces
func (c *MasterConfig) DNSServerClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// BuildLogClient returns the build log client object
func (c *MasterConfig) BuildLogClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// WebHookClient returns the webhook client object
func (c *MasterConfig) WebHookClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// BuildControllerClients returns the build controller client objects
func (c *MasterConfig) BuildControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// ImageChangeControllerClient returns the openshift client object
func (c *MasterConfig) ImageChangeControllerClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// ImageImportControllerClient returns the deployment client object
func (c *MasterConfig) ImageImportControllerClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// DeploymentControllerClients returns the deployment controller client object
func (c *MasterConfig) DeploymentControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// DeployerClientConfig returns the client configuration a Deployer instance launched in a pod
// should use when making API calls.
func (c *MasterConfig) DeployerClientConfig() *kclient.Config {
	return &c.DeployerOSClientConfig
}

func (c *MasterConfig) DeploymentConfigControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}
func (c *MasterConfig) DeploymentConfigChangeControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}
func (c *MasterConfig) DeploymentImageChangeControllerClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// OriginNamespaceControllerClients returns a client for openshift and kubernetes.
// The openshift client object must have authority to delete openshift content in any namespace
// The kubernetes client object must have authority to execute a finalize request on a namespace
func (c *MasterConfig) OriginNamespaceControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// NewEtcdHelper returns an EtcdHelper for the provided storage version.
func NewEtcdHelper(client *etcdclient.Client, version, prefix string) (oshelper tools.EtcdHelper, err error) {
	interfaces, err := latest.InterfacesFor(version)
	if err != nil {
		return tools.EtcdHelper{}, err
	}
	return tools.NewEtcdHelper(client, interfaces.Codec, prefix), nil
}
