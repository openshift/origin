package origin

import (
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"

	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kapilatest "k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/apiserver"
	"k8s.io/kubernetes/pkg/auth/user"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/storage"
	etcdstorage "k8s.io/kubernetes/pkg/storage/etcd"
	kutil "k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/request/bearertoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/paramtoken"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	"github.com/openshift/origin/pkg/auth/group"
	authnregistry "github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/authorization/authorizer"
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
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	"github.com/openshift/origin/pkg/cmd/util/plug"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	projectauth "github.com/openshift/origin/pkg/project/auth"
	"github.com/openshift/origin/pkg/serviceaccounts"
	usercache "github.com/openshift/origin/pkg/user/cache"
	groupregistry "github.com/openshift/origin/pkg/user/registry/group"
	groupstorage "github.com/openshift/origin/pkg/user/registry/group/etcd"
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

	PolicyCache               policycache.ReadOnlyCache
	GroupCache                *usercache.GroupCache
	ProjectAuthorizationCache *projectauth.AuthorizationCache

	// RequestContextMapper maps requests to contexts
	RequestContextMapper kapi.RequestContextMapper

	AdmissionControl admission.Interface

	TLS bool

	ControllerPlug plug.Plug

	// a function that returns the appropriate image to use for a named component
	ImageFor func(component string) string

	EtcdHelper storage.Interface
	// Storage interface no longer exposes the client since it is now generic.  This allows us
	// to provide access to the client for things that need it.
	EtcdClient *etcdclient.Client

	KubeletClientConfig *kclient.KubeletConfig

	// ClientCAs will be used to request client certificates in connections to the API.
	// This CertPool should contain all the CAs that will be used for client certificate verification.
	ClientCAs *x509.CertPool
	// APIClientCAs is used to verify client certificates presented for API auth
	APIClientCAs *x509.CertPool

	// PrivilegedLoopbackClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	PrivilegedLoopbackClientConfig kclient.Config

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

	// BuildControllerServiceAccount is the name of the service account in the infra namespace to use to run the build controller
	BuildControllerServiceAccount string
	// DeploymentControllerServiceAccount is the name of the service account in the infra namespace to use to run the deployment controller
	DeploymentControllerServiceAccount string
	// ReplicationControllerServiceAccount is the name of the service account in the infra namespace to use to run the replication controller
	ReplicationControllerServiceAccount string
}

func BuildMasterConfig(options configapi.MasterConfig) (*MasterConfig, error) {
	client, err := etcd.EtcdClient(options.EtcdClientInfo)
	if err != nil {
		return nil, err
	}
	etcdHelper, err := NewEtcdStorage(client, options.EtcdStorageConfig.OpenShiftStorageVersion, options.EtcdStorageConfig.OpenShiftStoragePrefix)
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

	kubeletClientConfig := configapi.GetKubeletClientConfig(options)

	// in-order list of plug-ins that should intercept admission decisions (origin only intercepts)
	admissionControlPluginNames := []string{"OriginNamespaceLifecycle", "BuildByStrategy"}

	admissionClient := admissionControlClient(privilegedLoopbackKubeClient, privilegedLoopbackOpenShiftClient)
	admissionController := admission.NewFromPlugins(admissionClient, admissionControlPluginNames, "")

	serviceAccountTokenGetter, err := newServiceAccountTokenGetter(options, client)
	if err != nil {
		return nil, err
	}

	config := &MasterConfig{
		Options: options,

		Authenticator:                 newAuthenticator(options, etcdHelper, serviceAccountTokenGetter, apiClientCAs, groupCache),
		Authorizer:                    newAuthorizer(policyClient, options.ProjectConfig.ProjectRequestMessage),
		AuthorizationAttributeBuilder: newAuthorizationAttributeBuilder(requestContextMapper),

		PolicyCache:               policyCache,
		GroupCache:                groupCache,
		ProjectAuthorizationCache: newProjectAuthorizationCache(privilegedLoopbackOpenShiftClient, privilegedLoopbackKubeClient, policyClient),

		RequestContextMapper: requestContextMapper,

		AdmissionControl: admissionController,

		TLS: configapi.UseTLS(options.ServingInfo.ServingInfo),

		ControllerPlug: plug.NewPlug(!options.PauseControllers),

		ImageFor:            imageTemplate.ExpandOrDie,
		EtcdHelper:          etcdHelper,
		EtcdClient:          client,
		KubeletClientConfig: kubeletClientConfig,

		ClientCAs:    clientCAs,
		APIClientCAs: apiClientCAs,

		PrivilegedLoopbackClientConfig:     *privilegedLoopbackClientConfig,
		PrivilegedLoopbackOpenShiftClient:  privilegedLoopbackOpenShiftClient,
		PrivilegedLoopbackKubernetesClient: privilegedLoopbackKubeClient,

		BuildControllerServiceAccount:       bootstrappolicy.InfraBuildControllerServiceAccountName,
		DeploymentControllerServiceAccount:  bootstrappolicy.InfraDeploymentControllerServiceAccountName,
		ReplicationControllerServiceAccount: bootstrappolicy.InfraReplicationControllerServiceAccountName,
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
		ketcdHelper, err := master.NewEtcdStorage(client, kapilatest.InterfacesFor, options.EtcdStorageConfig.KubernetesStorageVersion, options.EtcdStorageConfig.KubernetesStoragePrefix)
		if err != nil {
			return nil, fmt.Errorf("Error setting up Kubernetes server storage: %v", err)
		}
		tokenGetter = serviceaccount.NewGetterFromStorageInterface(ketcdHelper)
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
		authenticators = append(authenticators, bearertoken.New(tokenAuthenticator, true))
		// Allow token as access_token param for WebSockets
		authenticators = append(authenticators, paramtoken.New("access_token", tokenAuthenticator, true))
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

func newProjectAuthorizationCache(openshiftClient *osclient.Client, kubeClient *kclient.Client,
	policyClient policyclient.ReadOnlyPolicyClient) *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		projectauth.NewReviewer(openshiftClient),
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

func newAuthorizer(policyClient policyclient.ReadOnlyPolicyClient, projectRequestDenyMessage string) authorizer.Authorizer {
	authorizer := authorizer.NewAuthorizer(rulevalidation.NewDefaultRuleResolver(policyClient, policyClient, policyClient, policyClient), authorizer.NewForbiddenMessageResolver(projectRequestDenyMessage))
	return authorizer
}

func newAuthorizationAttributeBuilder(requestContextMapper kapi.RequestContextMapper) authorizer.AuthorizationAttributeBuilder {
	authorizationAttributeBuilder := authorizer.NewAuthorizationAttributeBuilder(requestContextMapper, &apiserver.APIRequestInfoResolver{kutil.NewStringSet("api", "osapi", "oapi"), latest.RESTMapper})
	return authorizationAttributeBuilder
}

func getEtcdTokenAuthenticator(etcdHelper storage.Interface, groupMapper identitymapper.UserToGroupMapper) authenticator.Token {
	accessTokenStorage := accesstokenetcd.NewREST(etcdHelper)
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
	osClient, kClient, err := c.GetServiceAccountClients(c.BuildControllerServiceAccount)
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

// DeploymentControllerClients returns the deployment controller client object
func (c *MasterConfig) DeploymentControllerClients() (*osclient.Client, *kclient.Client) {
	osClient, kClient, err := c.GetServiceAccountClients(c.DeploymentControllerServiceAccount)
	if err != nil {
		glog.Fatal(err)
	}
	return osClient, kClient
}

func (c *MasterConfig) DeployerPodControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}
func (c *MasterConfig) DeploymentConfigControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}
func (c *MasterConfig) DeploymentConfigChangeControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}
func (c *MasterConfig) DeploymentImageChangeTriggerControllerClient() *osclient.Client {
	return c.PrivilegedLoopbackOpenShiftClient
}

func (c *MasterConfig) SecurityAllocationControllerClient() *kclient.Client {
	return c.PrivilegedLoopbackKubernetesClient
}
func (c *MasterConfig) SDNControllerClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
}
func (c *MasterConfig) RouteAllocatorClients() (*osclient.Client, *kclient.Client) {
	return c.PrivilegedLoopbackOpenShiftClient, c.PrivilegedLoopbackKubernetesClient
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

// AdmissionControlClient returns a client to be used for admission control.
// TODO: Refactor admission control to allow more than one client to be passed in to plugins
func admissionControlClient(kClient *kclient.Client, osClient *osclient.Client) kclient.Interface {
	type kc struct{ *kclient.Client }
	type osc struct{ *osclient.Client }
	type compositeClient struct {
		*kc
		*osc
	}
	client := &compositeClient{
		&kc{kClient},
		&osc{osClient},
	}
	return client
}

// NewEtcdHelper returns an EtcdHelper for the provided storage version.
func NewEtcdStorage(client *etcdclient.Client, version, prefix string) (oshelper storage.Interface, err error) {
	interfaces, err := latest.InterfacesFor(version)
	if err != nil {
		return nil, err
	}
	return etcdstorage.NewEtcdStorage(client, interfaces.Codec, prefix), nil
}

// GetServiceAccountClients returns an OpenShift and Kubernetes client with the credentials of the
// named service account in the infra namespace
func (c *MasterConfig) GetServiceAccountClients(name string) (*osclient.Client, *kclient.Client, error) {
	if len(name) == 0 {
		return nil, nil, errors.New("No service account name specified")
	}
	return serviceaccounts.Clients(
		c.PrivilegedLoopbackClientConfig,
		&serviceaccounts.ClientLookupTokenRetriever{c.PrivilegedLoopbackKubernetesClient},
		c.Options.PolicyConfig.OpenShiftInfrastructureNamespace,
		name,
	)
}
