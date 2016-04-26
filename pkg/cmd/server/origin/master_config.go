package origin

import (
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"path"

	newetcdclient "github.com/coreos/etcd/client"
	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apiserver"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	clientadapter "k8s.io/kubernetes/pkg/client/unversioned/adapters/internalclientset"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	kubeletclient "k8s.io/kubernetes/pkg/kubelet/client"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/storage"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	kutilrand "k8s.io/kubernetes/pkg/util/rand"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/anonymous"
	"github.com/openshift/origin/pkg/auth/authenticator/request/bearertoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/paramtoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	"github.com/openshift/origin/pkg/auth/group"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	policycache "github.com/openshift/origin/pkg/authorization/cache"
	policyclient "github.com/openshift/origin/pkg/authorization/client"
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
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	projectcache "github.com/openshift/origin/pkg/project/cache"
	"github.com/openshift/origin/pkg/serviceaccounts"
	usercache "github.com/openshift/origin/pkg/user/cache"
	groupregistry "github.com/openshift/origin/pkg/user/registry/group"
	groupstorage "github.com/openshift/origin/pkg/user/registry/group/etcd"
	userregistry "github.com/openshift/origin/pkg/user/registry/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/user/etcd"
	"github.com/openshift/origin/pkg/util/leaderlease"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	Options configapi.MasterConfig

	RuleResolver                  rulevalidation.AuthorizationRuleResolver
	Authenticator                 authenticator.Request
	Authorizer                    authorizer.Authorizer
	AuthorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder

	PolicyCache               policycache.ReadOnlyCache
	GroupCache                *usercache.GroupCache
	ProjectAuthorizationCache *projectauth.AuthorizationCache
	ProjectCache              *projectcache.ProjectCache

	// RequestContextMapper maps requests to contexts
	RequestContextMapper kapi.RequestContextMapper

	AdmissionControl admission.Interface

	TLS bool

	ControllerPlug      plug.Plug
	ControllerPlugStart func()

	// ImageFor is a function that returns the appropriate image to use for a named component
	ImageFor func(component string) string

	EtcdHelper storage.Interface

	KubeletClientConfig *kubeletclient.KubeletClientConfig

	// ClientCAs will be used to request client certificates in connections to the API.
	// This CertPool should contain all the CAs that will be used for client certificate verification.
	ClientCAs *x509.CertPool
	// APIClientCAs is used to verify client certificates presented for API auth
	APIClientCAs *x509.CertPool

	// PluginInitializer carries types used when instantiating both origin and kubernetes admission control plugins
	PluginInitializer oadmission.PluginInitializer

	// PrivilegedLoopbackClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	PrivilegedLoopbackClientConfig restclient.Config

	// PrivilegedLoopbackKubernetesClient is the client used to call Kubernetes APIs from system components,
	// built from KubeClientConfig. It should only be accessed via the *Client() helper methods. To apply
	// different access control to a system component, create a separate client/config specifically for
	// that component.
	PrivilegedLoopbackKubernetesClient *kclient.Client
	// PrivilegedLoopbackOpenShiftClient is the client used to call OpenShift APIs from system components,
	// built from PrivilegedLoopbackClientConfig. It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically
	// for that component.
	PrivilegedLoopbackOpenShiftClient *osclient.Client
}

// BuildMasterConfig builds and returns the OpenShift master configuration based on the
// provided options
func BuildMasterConfig(options configapi.MasterConfig) (*MasterConfig, error) {
	client, err := etcd.EtcdClient(options.EtcdClientInfo)
	if err != nil {
		return nil, err
	}
	etcdClient, err := etcd.MakeNewEtcdClient(options.EtcdClientInfo)
	if err != nil {
		return nil, err
	}
	groupVersion := unversioned.GroupVersion{Group: "", Version: options.EtcdStorageConfig.OpenShiftStorageVersion}
	etcdHelper, err := NewEtcdStorage(etcdClient, groupVersion, options.EtcdStorageConfig.OpenShiftStoragePrefix)
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

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest

	policyCache, policyClient := newReadOnlyCacheAndClient(etcdHelper)
	requestContextMapper := kapi.NewRequestContextMapper()

	groupCache := usercache.NewGroupCache(groupregistry.NewRegistry(groupstorage.NewREST(etcdHelper)))
	projectCache := projectcache.NewProjectCache(privilegedLoopbackKubeClient.Namespaces(), options.ProjectConfig.DefaultNodeSelector)

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)

	// in-order list of plug-ins that should intercept admission decisions (origin only intercepts)
	admissionControlPluginNames := []string{"ProjectRequestLimit", "OriginNamespaceLifecycle", "PodNodeConstraints", "BuildByStrategy", "OriginResourceQuota"}
	if len(options.AdmissionConfig.PluginOrderOverride) > 0 {
		admissionControlPluginNames = options.AdmissionConfig.PluginOrderOverride
	}

	ruleResolver := rulevalidation.NewDefaultRuleResolver(
		rulevalidation.PolicyGetter(policyClient),
		rulevalidation.BindingLister(policyClient),
		rulevalidation.ClusterPolicyGetter(policyClient),
		rulevalidation.ClusterBindingLister(policyClient),
	)
	authorizer := newAuthorizer(ruleResolver, policyClient, options.ProjectConfig.ProjectRequestMessage)

	pluginInitializer := oadmission.PluginInitializer{
		OpenshiftClient: privilegedLoopbackOpenShiftClient,
		ProjectCache:    projectCache,
		Authorizer:      authorizer,
	}

	plugins := []admission.Interface{}
	clientsetClient := clientadapter.FromUnversionedClient(privilegedLoopbackKubeClient)
	for _, pluginName := range admissionControlPluginNames {
		configFile, err := pluginconfig.GetPluginConfig(options.AdmissionConfig.PluginConfig[pluginName])
		if err != nil {
			return nil, err
		}
		plugin := admission.InitPlugin(pluginName, clientsetClient, configFile)
		if plugin != nil {
			plugins = append(plugins, plugin)
		}
	}
	pluginInitializer.Initialize(plugins)
	// ensure that plugins have been properly initialized
	if err := oadmission.Validate(plugins); err != nil {
		return nil, err
	}
	admissionController := admission.NewChainHandler(plugins...)

	serviceAccountTokenGetter, err := newServiceAccountTokenGetter(options, etcdClient)
	if err != nil {
		return nil, err
	}

	plug, plugStart := newControllerPlug(options, client)

	config := &MasterConfig{
		Options: options,

		RuleResolver:                  ruleResolver,
		Authenticator:                 newAuthenticator(options, etcdHelper, serviceAccountTokenGetter, apiClientCAs, groupCache),
		Authorizer:                    authorizer,
		AuthorizationAttributeBuilder: newAuthorizationAttributeBuilder(requestContextMapper),

		PolicyCache:               policyCache,
		GroupCache:                groupCache,
		ProjectAuthorizationCache: newProjectAuthorizationCache(authorizer, privilegedLoopbackKubeClient, policyClient),
		ProjectCache:              projectCache,

		RequestContextMapper: requestContextMapper,

		AdmissionControl: admissionController,

		TLS: configapi.UseTLS(options.ServingInfo.ServingInfo),

		ControllerPlug:      plug,
		ControllerPlugStart: plugStart,

		ImageFor:            imageTemplate.ExpandOrDie,
		EtcdHelper:          etcdHelper,
		KubeletClientConfig: kubeletClientConfig,

		ClientCAs:    clientCAs,
		APIClientCAs: apiClientCAs,

		PluginInitializer: pluginInitializer,

		PrivilegedLoopbackClientConfig:     *privilegedLoopbackClientConfig,
		PrivilegedLoopbackOpenShiftClient:  privilegedLoopbackOpenShiftClient,
		PrivilegedLoopbackKubernetesClient: privilegedLoopbackKubeClient,
	}

	return config, nil
}

func newControllerPlug(options configapi.MasterConfig, client *etcdclient.Client) (plug.Plug, func()) {
	switch {
	case options.ControllerLeaseTTL > 0:
		// TODO: replace with future API for leasing from Kube
		id := fmt.Sprintf("master-%s", kutilrand.String(8))
		leaser := leaderlease.NewEtcd(
			client,
			path.Join(options.EtcdStorageConfig.OpenShiftStoragePrefix, "leases/controllers"),
			id,
			uint64(options.ControllerLeaseTTL),
		)
		leased := plug.NewLeased(leaser)
		return leased, func() {
			glog.V(2).Infof("Attempting to acquire controller lease as %s, renewing every %d seconds", id, options.ControllerLeaseTTL)
			go leased.Run()
		}
	default:
		return plug.New(!options.PauseControllers), func() {}
	}
}

func newServiceAccountTokenGetter(options configapi.MasterConfig, client newetcdclient.Client) (serviceaccount.ServiceAccountTokenGetter, error) {
	var tokenGetter serviceaccount.ServiceAccountTokenGetter
	if options.KubernetesMasterConfig == nil {
		// When we're running against an external Kubernetes, use the external kubernetes client to validate service account tokens
		// This prevents infinite auth loops if the privilegedLoopbackKubeClient authenticates using a service account token
		kubeClient, _, err := configapi.GetKubeClient(options.MasterClients.ExternalKubernetesKubeConfig)
		if err != nil {
			return nil, err
		}
		tokenGetter = sacontroller.NewGetterFromClient(clientadapter.FromUnversionedClient(kubeClient))
	} else {
		// When we're running in-process, go straight to etcd (using the KubernetesStorageVersion/KubernetesStoragePrefix, since service accounts are kubernetes objects)
		codec := kapi.Codecs.LegacyCodec(unversioned.GroupVersion{Group: kapi.GroupName, Version: options.EtcdStorageConfig.KubernetesStorageVersion})
		ketcdHelper := etcdstorage.NewEtcdStorage(client, codec, options.EtcdStorageConfig.KubernetesStoragePrefix, false)
		tokenGetter = sacontroller.NewGetterFromStorageInterface(ketcdHelper)
	}
	return tokenGetter, nil
}

func newAuthenticator(config configapi.MasterConfig, etcdHelper storage.Interface, tokenGetter serviceaccount.ServiceAccountTokenGetter, apiClientCAs *x509.CertPool, groupMapper identitymapper.UserToGroupMapper) authenticator.Request {
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
		tokenAuthenticator := getEtcdTokenAuthenticator(etcdHelper, groupMapper)
		tokenRequestAuthenticators := []authenticator.Request{
			bearertoken.New(tokenAuthenticator, true),
			// Allow token as access_token param for WebSockets
			paramtoken.New("access_token", tokenAuthenticator, true),
		}

		authenticators = append(authenticators,
			// if you have a bearer token, you're a human (usually)
			// if you change this, have a look at the impersonationFilter where we attach groups to the impersonated user
			group.NewGroupAdder(unionrequest.NewUnionAuthentication(tokenRequestAuthenticators...), []string{bootstrappolicy.AuthenticatedOAuthGroup}))
	}

	if configapi.UseTLS(config.ServingInfo.ServingInfo) {
		// build cert authenticator
		// TODO: add "system:" prefix in authenticator, limit cert to username
		// TODO: add "system:" prefix to groups in authenticator, limit cert to group name
		opts := x509request.DefaultVerifyOptions()
		opts.Roots = apiClientCAs
		certauth := x509request.New(opts, x509request.SubjectToUserConversion)
		authenticators = append(authenticators, certauth)
	}

	ret := &unionrequest.Authenticator{
		FailOnError: true,
		Handlers: []authenticator.Request{
			// if you change this, have a look at the impersonationFilter where we attach groups to the impersonated user
			group.NewGroupAdder(unionrequest.NewUnionAuthentication(authenticators...), []string{bootstrappolicy.AuthenticatedGroup}),
			anonymous.NewAuthenticator(),
		},
	}

	return ret
}

func newProjectAuthorizationCache(authorizer authorizer.Authorizer, kubeClient *kclient.Client, policyClient policyclient.ReadOnlyPolicyClient) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		projectauth.NewAuthorizerReviewer(authorizer),
		kubeClient.Namespaces(),
		policyClient,
	)
}

// newReadOnlyCacheAndClient returns a ReadOnlyCache for administrative interactions with the cache holding policies and bindings on a project
// and cluster level as well as a ReadOnlyPolicyClient for use in the project authorization cache and authorizer to query for the same data
func newReadOnlyCacheAndClient(etcdHelper storage.Interface) (cache policycache.ReadOnlyCache, client policyclient.ReadOnlyPolicyClient) {
	policyRegistry := policyregistry.NewRegistry(policyetcd.NewStorage(etcdHelper))
	policyBindingRegistry := policybindingregistry.NewRegistry(policybindingetcd.NewStorage(etcdHelper))
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterpolicyetcd.NewStorage(etcdHelper))
	clusterPolicyBindingRegistry := clusterpolicybindingregistry.NewRegistry(clusterpolicybindingetcd.NewStorage(etcdHelper))

	cache, client = policycache.NewReadOnlyCacheAndClient(policyBindingRegistry, policyRegistry, clusterPolicyBindingRegistry, clusterPolicyRegistry)
	return
}

func newAuthorizer(ruleResolver rulevalidation.AuthorizationRuleResolver, policyClient policyclient.ReadOnlyPolicyClient, projectRequestDenyMessage string) authorizer.Authorizer {
	messageMaker := authorizer.NewForbiddenMessageResolver(projectRequestDenyMessage)
	roleBasedAuthorizer := authorizer.NewAuthorizer(ruleResolver, messageMaker)
	scopeLimitedAuthorizer := scope.NewAuthorizer(roleBasedAuthorizer, policyClient, messageMaker)
	return scopeLimitedAuthorizer
}

func newAuthorizationAttributeBuilder(requestContextMapper kapi.RequestContextMapper) authorizer.AuthorizationAttributeBuilder {
	// Default API request resolver
	requestInfoResolver := &apiserver.RequestInfoResolver{APIPrefixes: sets.NewString("api", "osapi", "oapi", "apis"), GrouplessAPIPrefixes: sets.NewString("api", "osapi", "oapi")}
	// Wrap with a resolver that detects unsafe requests and modifies verbs/resources appropriately so policy can address them separately
	browserSafeRequestInfoResolver := authorizer.NewBrowserSafeRequestInfoResolver(
		requestContextMapper,
		sets.NewString(bootstrappolicy.AuthenticatedGroup),
		requestInfoResolver,
	)

	authorizationAttributeBuilder := authorizer.NewAuthorizationAttributeBuilder(requestContextMapper, browserSafeRequestInfoResolver)
	return authorizationAttributeBuilder
}

func getEtcdTokenAuthenticator(etcdHelper storage.Interface, groupMapper identitymapper.UserToGroupMapper) authenticator.Token {
	// this never does a create for access tokens, so we don't need to be able to validate scopes against the client
	accessTokenStorage := accesstokenetcd.NewREST(etcdHelper, nil)
	accessTokenRegistry := accesstokenregistry.NewRegistry(accessTokenStorage)

	userStorage := useretcd.NewREST(etcdHelper)
	userRegistry := userregistry.NewRegistry(userStorage)

	return authnregistry.NewTokenAuthenticator(accessTokenRegistry, userRegistry, groupMapper)
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

// ServiceAccountRoleBindingClient returns the client object used to bind roles to service accounts
// It must have the following capabilities:
//  get, list, update, create policyBindings and clusterPolicyBindings in all namespaces
func (c *MasterConfig) ServiceAccountRoleBindingClient() *osclient.Client {
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

// BuildConfigWebHookClient returns the webhook client object
func (c *MasterConfig) BuildConfigWebHookClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// BuildControllerClients returns the build controller client objects
func (c *MasterConfig) BuildControllerClients() (*osclient.Client, *kclient.Client) {
	_, osClient, kClient, err := c.GetServiceAccountClients(bootstrappolicy.InfraBuildControllerServiceAccountName)
	if err != nil {
		glog.Fatal(err)
	}
	return osClient, kClient
}

// BuildPodControllerClients returns the build pod controller client objects
func (c *MasterConfig) BuildPodControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// BuildImageChangeTriggerControllerClients returns the build image change trigger controller client objects
func (c *MasterConfig) BuildImageChangeTriggerControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// BuildConfigChangeControllerClients returns the build config change controller client objects
func (c *MasterConfig) BuildConfigChangeControllerClients() (*osclient.Client, *kclient.Client) {
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

// DeploymentConfigScaleClient returns the client used by the Scale subresource registry
func (c *MasterConfig) DeploymentConfigScaleClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// DeploymentControllerClients returns the deployment controller client objects
func (c *MasterConfig) DeploymentControllerClients() (*osclient.Client, *kclient.Client) {
	_, osClient, kClient, err := c.GetServiceAccountClients(bootstrappolicy.InfraDeploymentControllerServiceAccountName)
	if err != nil {
		glog.Fatal(err)
	}
	return osClient, kClient
}

// DeployerPodControllerClients returns the deployer pod controller client objects
func (c *MasterConfig) DeployerPodControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// DeploymentConfigClients returns deploymentConfig and deployment client objects
func (c *MasterConfig) DeploymentConfigClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// DeploymentConfigControllerClients returns the deploymentConfig controller client objects
func (c *MasterConfig) DeploymentConfigControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// DeploymentConfigChangeControllerClients returns the deploymentConfig config change controller client objects
func (c *MasterConfig) DeploymentConfigChangeControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// DeploymentImageChangeTriggerControllerClient returns the deploymentConfig image change controller client object
func (c *MasterConfig) DeploymentImageChangeTriggerControllerClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// DeploymentLogClient returns the deployment log client object
func (c *MasterConfig) DeploymentLogClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// SecurityAllocationControllerClient returns the security allocation controller client object
func (c *MasterConfig) SecurityAllocationControllerClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// SDNControllerClients returns the SDN controller client objects
func (c *MasterConfig) SDNControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// RouteAllocatorClients returns the route allocator client objects
func (c *MasterConfig) RouteAllocatorClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// ImageStreamSecretClient returns the client capable of retrieving secrets for an image secret wrapper
func (c *MasterConfig) ImageStreamSecretClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}

// ImageStreamImportSecretClient returns the client capable of retrieving image secrets for a namespace
func (c *MasterConfig) ImageStreamImportSecretClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

// ResourceQuotaManagerClients returns the client capable of retrieving resources needed for resource quota
// evaluation
func (c *MasterConfig) ResourceQuotaManagerClients() (*osclient.Client, *internalclientset.Clientset) {
	return c.PrivilegedLoopbackOpenShiftClient, clientadapter.FromUnversionedClient(c.PrivilegedLoopbackKubernetesClient)
}

// WebConsoleEnabled says whether web ui is not a disabled feature and asset service is configured.
func (c *MasterConfig) WebConsoleEnabled() bool {
	return c.Options.AssetConfig != nil && !c.Options.DisabledFeatures.Has(configapi.FeatureWebConsole)
}

// OriginNamespaceControllerClients returns a client for openshift and kubernetes.
// The openshift client object must have authority to delete openshift content in any namespace
// The kubernetes client object must have authority to execute a finalize request on a namespace
func (c *MasterConfig) OriginNamespaceControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}

// NewEtcdStorage returns a storage interface for the provided storage version.
func NewEtcdStorage(client newetcdclient.Client, version unversioned.GroupVersion, prefix string) (oshelper storage.Interface, err error) {
	return etcdstorage.NewEtcdStorage(client, kapi.Codecs.LegacyCodec(version), prefix, false), nil
}

// GetServiceAccountClients returns an OpenShift and Kubernetes client with the credentials of the
// named service account in the infra namespace
func (c *MasterConfig) GetServiceAccountClients(name string) (*restclient.Config, *osclient.Client, *kclient.Client, error) {
	if len(name) == 0 {
		return nil, nil, nil, errors.New("No service account name specified")
	}
	return serviceaccounts.Clients(
		c.PrivilegedLoopbackClientConfig,
		&serviceaccounts.ClientLookupTokenRetriever{Client: c.PrivilegedLoopbackKubernetesClient},
		c.Options.PolicyConfig.OpenShiftInfrastructureNamespace,
		name,
	)
}
