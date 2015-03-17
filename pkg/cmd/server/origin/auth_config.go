package origin

import (
	"crypto/x509"
	"fmt"
	"strings"

	"code.google.com/p/go-uuid/uuid"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	"github.com/openshift/origin/pkg/auth/server/session"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

type AuthConfig struct {
	// URL to call internally during token request
	MasterAddr string
	// URL to direct browsers to the master on
	MasterPublicAddr string
	// Valid redirectURI prefixes to direct browsers to the web console
	AssetPublicAddresses []string
	MasterRoots          *x509.CertPool
	EtcdHelper           tools.EtcdHelper

	// Max age of authorize tokens
	AuthorizeTokenMaxAgeSeconds int32
	// Max age of access tokens
	AccessTokenMaxAgeSeconds int32

	// AuthRequestHandlers contains an ordered list of authenticators that decide if a request is authenticated
	AuthRequestHandlers []AuthRequestHandlerType

	// AuthHandler specifies what handles unauthenticated requests
	AuthHandler AuthHandlerType

	// GrantHandler specifies what handles requests for new client authorizations
	GrantHandler GrantHandlerType

	// PasswordAuth specifies how to validate username/passwords. Used by AuthRequestHandlerBasicAuth and AuthHandlerLogin
	PasswordAuth PasswordAuthType
	// BasicAuthURL specifies the remote URL to validate username/passwords against using basic auth. Used by PasswordAuthBasicAuthURL.
	BasicAuthURL string
	// HTPasswdFile specifies the path to an htpasswd file to validate username/passwords against. Used by PasswordAuthHTPasswd.
	HTPasswdFile string

	// TokenStore specifies how to validate bearer tokens. Used by AuthRequestHandlerBearer.
	TokenStore TokenStoreType
	// TokenFilePath is a path to a CSV file to load valid tokens from. Used by TokenStoreFile.
	TokenFilePath string

	// RequestHeaders lists the headers to check (in order) for a username. Used by AuthRequestHandlerRequestHeader
	RequestHeaders []string
	// RequestHeaderCAFile specifies the path to a PEM-encoded certificate bundle.
	// If set, a client certificate must be presented and validate against the CA before the request headers are checked for usernames
	RequestHeaderCAFile string

	// SessionSecrets list the secret(s) to use to encrypt created sessions. Used by AuthRequestHandlerSession
	SessionSecrets []string
	// SessionMaxAgeSeconds specifies how long created sessions last. Used by AuthRequestHandlerSession
	SessionMaxAgeSeconds int32
	// SessionName is the cookie name used to store the session
	SessionName string
	// sessionAuth holds the Authenticator built from the exported Session* options. It should only be accessed via getSessionAuth(), since it is lazily built.
	sessionAuth *session.Authenticator

	// GoogleClientID is the client_id of a client registered with the Google OAuth provider.
	// It must be authorized to redirect to {MasterPublicAddr}/oauth2callback/google
	// Used by AuthHandlerGoogle
	GoogleClientID string
	// GoogleClientID is the client_secret of a client registered with the Google OAuth provider.
	GoogleClientSecret string

	// GithubClientID is the client_id of a client registered with the GitHub OAuth provider.
	// It must be authorized to redirect to {MasterPublicAddr}/oauth2callback/github
	// Used by AuthHandlerGithub
	GithubClientID string
	// GithubClientID is the client_secret of a client registered with the GitHub OAuth provider.
	GithubClientSecret string
}

func BuildAuthConfig(options configapi.MasterConfig) (*AuthConfig, error) {
	etcdHelper, err := etcd.NewOpenShiftEtcdHelper(options.EtcdClientInfo.URL)
	if err != nil {
		return nil, fmt.Errorf("Error setting up server storage: %v", err)
	}

	apiServerCAs, err := configapi.GetAPIServerCertCAPool(options)
	if err != nil {
		return nil, err
	}

	// Build the list of valid redirect_uri prefixes for a login using the openshift-web-console client to redirect to
	// TODO: allow configuring this
	// TODO: remove hard-coding of development UI server
	assetPublicURLs := []string{options.OAuthConfig.AssetPublicURL, "http://localhost:9000", "https://localhost:9000"}

	// Default to a session authenticator (for browsers), and a basicauth authenticator (for clients responding to WWW-Authenticate challenges)
	defaultAuthRequestHandlers := strings.Join([]string{
		string(AuthRequestHandlerSession),
		string(AuthRequestHandlerBasicAuth),
	}, ",")

	ret := &AuthConfig{
		MasterAddr:           options.OAuthConfig.MasterURL,
		MasterPublicAddr:     options.OAuthConfig.MasterPublicURL,
		AssetPublicAddresses: assetPublicURLs,
		MasterRoots:          apiServerCAs,
		EtcdHelper:           etcdHelper,

		// Max token ages
		AuthorizeTokenMaxAgeSeconds: cmdutil.EnvInt("OPENSHIFT_OAUTH_AUTHORIZE_TOKEN_MAX_AGE_SECONDS", 300, 1),
		AccessTokenMaxAgeSeconds:    cmdutil.EnvInt("OPENSHIFT_OAUTH_ACCESS_TOKEN_MAX_AGE_SECONDS", 3600, 1),
		// Handlers
		AuthRequestHandlers: ParseAuthRequestHandlerTypes(cmdutil.Env("OPENSHIFT_OAUTH_REQUEST_HANDLERS", defaultAuthRequestHandlers)),
		AuthHandler:         AuthHandlerType(cmdutil.Env("OPENSHIFT_OAUTH_HANDLER", string(AuthHandlerLogin))),
		GrantHandler:        GrantHandlerType(cmdutil.Env("OPENSHIFT_OAUTH_GRANT_HANDLER", string(GrantHandlerAuto))),
		// RequestHeader config
		RequestHeaders:      strings.Split(cmdutil.Env("OPENSHIFT_OAUTH_REQUEST_HEADERS", "X-Remote-User"), ","),
		RequestHeaderCAFile: options.OAuthConfig.ProxyCA,
		// Session config (default to unknowable secret)
		SessionSecrets:       []string{cmdutil.Env("OPENSHIFT_OAUTH_SESSION_SECRET", uuid.NewUUID().String())},
		SessionMaxAgeSeconds: cmdutil.EnvInt("OPENSHIFT_OAUTH_SESSION_MAX_AGE_SECONDS", 300, 1),
		SessionName:          cmdutil.Env("OPENSHIFT_OAUTH_SESSION_NAME", "ssn"),
		// Password config
		PasswordAuth: PasswordAuthType(cmdutil.Env("OPENSHIFT_OAUTH_PASSWORD_AUTH", string(PasswordAuthAnyPassword))),
		BasicAuthURL: cmdutil.Env("OPENSHIFT_OAUTH_BASIC_AUTH_URL", ""),
		HTPasswdFile: cmdutil.Env("OPENSHIFT_OAUTH_HTPASSWD_FILE", ""),
		// Token config
		TokenStore:    TokenStoreType(cmdutil.Env("OPENSHIFT_OAUTH_TOKEN_STORE", string(TokenStoreOAuth))),
		TokenFilePath: cmdutil.Env("OPENSHIFT_OAUTH_TOKEN_FILE_PATH", ""),
		// Google config
		GoogleClientID:     cmdutil.Env("OPENSHIFT_OAUTH_GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: cmdutil.Env("OPENSHIFT_OAUTH_GOOGLE_CLIENT_SECRET", ""),
		// GitHub config
		GithubClientID:     cmdutil.Env("OPENSHIFT_OAUTH_GITHUB_CLIENT_ID", ""),
		GithubClientSecret: cmdutil.Env("OPENSHIFT_OAUTH_GITHUB_CLIENT_SECRET", ""),
	}

	return ret, nil

}
