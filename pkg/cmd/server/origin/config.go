package origin

import (
	"crypto/x509"
	"net/http"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/admission"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

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
	"github.com/openshift/origin/pkg/cmd/util/variable"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	projectauth "github.com/openshift/origin/pkg/project/auth"
)

const (
	unauthenticatedUsername = "system:anonymous"

	authenticatedGroup   = "system:authenticated"
	unauthenticatedGroup = "system:unauthenticated"
)

type MasterConfigParameters struct {
	// host:port to bind master to
	MasterBindAddr string
	// host:port to bind asset server to
	AssetBindAddr string
	// url to access the master API on within the cluster
	MasterAddr string
	// url to access kubernetes API on within the cluster
	KubernetesAddr string
	// external clients may need to access APIs at different addresses than internal components do
	MasterPublicAddr     string
	KubernetesPublicAddr string
	AssetPublicAddr      string
	// LogoutURI is an optional, absolute URI to redirect web browsers to after logging out of the web console.
	// If not specified, the built-in logout page is shown.
	LogoutURI string

	CORSAllowedOrigins []string

	EtcdHelper tools.EtcdHelper

	MasterCertFile string
	MasterKeyFile  string
	AssetCertFile  string
	AssetKeyFile   string

	// ClientCAs will be used to request client certificates in connections to the API.
	// This CertPool should contain all the CAs that will be used for client certificate verification.
	ClientCAs *x509.CertPool

	MasterAuthorizationNamespace string

	// kubeClient is the client used to call Kubernetes APIs from system components, built from KubeClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	KubeClient *kclient.Client
	// KubeClientConfig is the client configuration used to call Kubernetes APIs from system components.
	// To apply different access control to a system component, create a client config specifically for that component.
	KubeClientConfig kclient.Config

	// osClient is the client used to call OpenShift APIs from system components, built from OSClientConfig.
	// It should only be accessed via the *Client() helper methods.
	// To apply different access control to a system component, create a separate client/config specifically for that component.
	OSClient *osclient.Client
	// OSClientConfig is the client configuration used to call OpenShift APIs from system components
	// To apply different access control to a system component, create a client config specifically for that component.
	OSClientConfig kclient.Config

	// DeployerOSClientConfig is the client configuration used to call OpenShift APIs from launched deployer pods
	DeployerOSClientConfig kclient.Config
}

// MasterConfig defines the required parameters for starting the OpenShift master
type MasterConfig struct {
	MasterConfigParameters

	Authenticator                 authenticator.Request
	Authorizer                    authorizer.Authorizer
	AuthorizationAttributeBuilder authorizer.AuthorizationAttributeBuilder

	PolicyCache               *policycache.PolicyCache
	ProjectAuthorizationCache *projectauth.AuthorizationCache

	// Map requests to contexts
	RequestContextMapper kapi.RequestContextMapper

	AdmissionControl admission.Interface

	// a function that returns the appropriate image to use for a named component
	ImageFor func(component string) string

	TLS bool
}

func BuildMasterConfig(configParams MasterConfigParameters) (*MasterConfig, error) {

	policyCache := configParams.newPolicyCache()
	requestContextMapper := kapi.NewRequestContextMapper()

	imageTemplate := variable.NewDefaultImageTemplate()

	config := &MasterConfig{
		MasterConfigParameters: configParams,

		Authenticator:                 configParams.newAuthenticator(),
		Authorizer:                    newAuthorizer(policyCache, configParams.MasterAuthorizationNamespace),
		AuthorizationAttributeBuilder: newAuthorizationAttributeBuilder(requestContextMapper),

		PolicyCache:               policyCache,
		ProjectAuthorizationCache: configParams.newProjectAuthorizationCache(),

		RequestContextMapper: requestContextMapper,

		AdmissionControl: admit.NewAlwaysAdmit(),

		ImageFor: imageTemplate.ExpandOrDie,

		TLS: strings.HasPrefix(configParams.MasterAddr, "https://"),
	}

	return config, nil
}

func (c MasterConfigParameters) newAuthenticator() authenticator.Request {
	useTLS := strings.HasPrefix(c.MasterAddr, "https://")

	tokenAuthenticator := getEtcdTokenAuthenticator(c.EtcdHelper)

	authenticators := []authenticator.Request{}
	authenticators = append(authenticators, bearertoken.New(tokenAuthenticator, true))
	// Allow token as access_token param for WebSockets
	// TODO: make the param name configurable
	// TODO: limit this authenticator to watch methods, if possible
	// TODO: prevent access_token param from getting logged, if possible
	authenticators = append(authenticators, paramtoken.New("access_token", tokenAuthenticator, true))

	if useTLS {
		// build cert authenticator
		// TODO: add cert users to etcd?
		opts := x509request.DefaultVerifyOptions()
		opts.Roots = c.ClientCAs
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

func (c MasterConfigParameters) newProjectAuthorizationCache() *projectauth.AuthorizationCache {
	return projectauth.NewAuthorizationCache(
		projectauth.NewReviewer(c.OSClient),
		c.KubeClient.Namespaces(),
		c.OSClient,
		c.OSClient,
		c.MasterAuthorizationNamespace)
}

func (c MasterConfigParameters) newPolicyCache() *policycache.PolicyCache {
	authorizationEtcd := authorizationetcd.New(c.EtcdHelper)
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
