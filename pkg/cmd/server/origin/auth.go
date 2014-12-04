package origin

import (
	"fmt"
	"strings"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/server/tokenrequest"

	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	oauthclient "github.com/openshift/origin/pkg/oauth/registry/client"
	oauthetcd "github.com/openshift/origin/pkg/oauth/registry/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"

	cmdauth "github.com/openshift/origin/pkg/cmd/auth"
	// instantiators that need initing
	_ "github.com/openshift/origin/pkg/cmd/auth/challengehandlers/basic"
	_ "github.com/openshift/origin/pkg/cmd/auth/granthandlers/auto"
	_ "github.com/openshift/origin/pkg/cmd/auth/granthandlers/empty"
	_ "github.com/openshift/origin/pkg/cmd/auth/granthandlers/prompt"
	_ "github.com/openshift/origin/pkg/cmd/auth/identitymappers/autocreate"
	_ "github.com/openshift/origin/pkg/cmd/auth/passwordauthenticators/basic"
	_ "github.com/openshift/origin/pkg/cmd/auth/passwordauthenticators/empty"
	_ "github.com/openshift/origin/pkg/cmd/auth/redirecthandlers/empty"
	_ "github.com/openshift/origin/pkg/cmd/auth/redirecthandlers/login"
	_ "github.com/openshift/origin/pkg/cmd/auth/redirecthandlers/oauth"
	_ "github.com/openshift/origin/pkg/cmd/auth/requestauthenticators/basic"
	_ "github.com/openshift/origin/pkg/cmd/auth/requestauthenticators/bearer"
	_ "github.com/openshift/origin/pkg/cmd/auth/requestauthenticators/session"
	_ "github.com/openshift/origin/pkg/cmd/auth/requestauthenticators/xremoteuser"
	_ "github.com/openshift/origin/pkg/cmd/auth/tokenauthenticators/csv"
	_ "github.com/openshift/origin/pkg/cmd/auth/tokenauthenticators/etcd"
)

const (
	OpenShiftOAuthAPIPrefix      = "/oauth"
	OpenShiftLoginPrefix         = "/login"
	OpenShiftApprovePrefix       = "/oauth/approve"
	OpenShiftOAuthCallbackPrefix = "/oauth2callback"
)

var (
	// OSBrowserClientBase is used as a skeleton for building a Client.  We can't set the allowed redirecturis because we don't yet know the host:port of the auth server
	OSBrowserClientBase = oauthapi.Client{
		ObjectMeta: kapi.ObjectMeta{
			Name: "openshift-browser-client",
		},
		Secret: uuid.NewUUID().String(), // random secret so no one knows what it is ahead of time.  This still allows us to loop back for /requestToken
	}
	OSCliClientBase = oauthapi.Client{
		ObjectMeta: kapi.ObjectMeta{
			Name: "openshift-challenging-client",
		},
		Secret:                uuid.NewUUID().String(), // random secret so no one knows what it is ahead of time.  This still allows us to loop back for /requestToken
		RespondWithChallenges: true,
	}
)

type AuthConfig struct {
	CmdAuthConfig *cmdauth.AuthConfig
	EnvInfo       *cmdauth.EnvInfo
}

// InstallAPI starts an OAuth2 server and registers the supported REST APIs
// into the provided mux, then returns an array of strings indicating what
// endpoints were started (these are format strings that will expect to be sent
// a single string value).
func (c *AuthConfig) InstallAPI(mux cmdutil.Mux) []string {
	oauthEtcd := oauthetcd.New(c.EnvInfo.EtcdHelper)
	storage := registrystorage.New(oauthEtcd, oauthEtcd, oauthEtcd, registry.NewUserConversion())
	grantChecker := registry.NewClientAuthorizationGrantChecker(oauthEtcd)
	config := osinserver.NewDefaultServerConfig()

	server := osinserver.New(
		config,
		storage,
		osinserver.AuthorizeHandlers{
			handlers.NewAuthorizeAuthenticator(
				c.CmdAuthConfig.GetAuthorizeRequestAuthenticator(),
				c.CmdAuthConfig.GetAuthorizeAuthenticationHandler(),
				handlers.EmptyError{},
			),
			handlers.NewGrantCheck(
				grantChecker,
				c.CmdAuthConfig.GrantHandler,
				handlers.EmptyError{},
			),
		},
		osinserver.AccessHandlers{
			handlers.NewDenyAccessAuthenticator(),
		},
		osinserver.NewDefaultErrorHandler(),
	)
	server.Install(mux, OpenShiftOAuthAPIPrefix)

	CreateOrUpdateDefaultOAuthClients(c.EnvInfo.MasterAddr, oauthEtcd)
	osOAuthClientConfig := c.NewOpenShiftOAuthClientConfig(&OSBrowserClientBase)
	osOAuthClientConfig.RedirectUrl = c.EnvInfo.MasterAddr + OpenShiftOAuthAPIPrefix + tokenrequest.DisplayTokenEndpoint
	osOAuthClient, _ := osincli.NewClient(osOAuthClientConfig)
	tokenRequestEndpoints := tokenrequest.NewEndpoints(osOAuthClient)
	tokenRequestEndpoints.Install(mux, OpenShiftOAuthAPIPrefix)

	// glog.Infof("oauth server configured as: %#v", server)
	// glog.Infof("auth handler: %#v", authHandler)
	// glog.Infof("auth request handler: %#v", authRequestHandler)
	// glog.Infof("grant checker: %#v", grantChecker)
	// glog.Infof("grant handler: %#v", grantHandler)

	return []string{
		fmt.Sprintf("Started OAuth2 API at %%s%s", OpenShiftOAuthAPIPrefix),
		fmt.Sprintf("Started login server at %%s%s", OpenShiftLoginPrefix),
	}
}

// NewOpenShiftOAuthClientConfig provides config for OpenShift OAuth client
func (c *AuthConfig) NewOpenShiftOAuthClientConfig(client *oauthapi.Client) *osincli.ClientConfig {
	config := &osincli.ClientConfig{
		ClientId:                 client.Name,
		ClientSecret:             client.Secret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             c.EnvInfo.MasterAddr + OpenShiftOAuthAPIPrefix + "/authorize",
		TokenUrl:                 c.EnvInfo.MasterAddr + OpenShiftOAuthAPIPrefix + "/token",
		Scope:                    "",
	}
	return config
}

func CreateOrUpdateDefaultOAuthClients(masterAddr string, clientRegistry oauthclient.Registry) {
	clientsToEnsure := []*oauthapi.Client{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OSBrowserClientBase.Name,
			},
			Secret:                OSBrowserClientBase.Secret,
			RespondWithChallenges: OSBrowserClientBase.RespondWithChallenges,
			RedirectURIs:          []string{masterAddr + OpenShiftOAuthAPIPrefix + tokenrequest.DisplayTokenEndpoint},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: OSCliClientBase.Name,
			},
			Secret:                OSCliClientBase.Secret,
			RespondWithChallenges: OSCliClientBase.RespondWithChallenges,
			RedirectURIs:          []string{masterAddr + OpenShiftOAuthAPIPrefix + tokenrequest.DisplayTokenEndpoint},
		},
	}

	for _, currClient := range clientsToEnsure {
		if existing, err := clientRegistry.GetClient(currClient.Name); err == nil || strings.Contains(err.Error(), " not found") {
			if existing != nil {
				clientRegistry.DeleteClient(currClient.Name)
			}
			if err = clientRegistry.CreateClient(currClient); err != nil {
				glog.Errorf("Error creating client: %v due to %v\n", currClient, err)
			}
		} else {
			glog.Errorf("Error getting client: %v due to %v\n", currClient, err)

		}
	}
}
