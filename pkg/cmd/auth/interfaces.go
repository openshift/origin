package auth

import (
	"reflect"

	ktools "github.com/GoogleCloudPlatform/kubernetes/pkg/tools"

	authapi "github.com/openshift/origin/pkg/auth/api"
	authauthenticator "github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/requesthandlers"
	"github.com/openshift/origin/pkg/auth/authenticator/uniontoken"
	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/server/session"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
)

// best names in the world for Clayton's eyes

type AuthElementConfigInfo struct {
	AuthElementConfigType string            `json:"type" yaml:"type"`
	Config                map[string]string `json:"config,omitempty" yaml:"config,omitempty"`
}
type AuthConfigInfoMap map[string]AuthElementConfigInfo

type AuthConfigInfo struct {
	IdentityMappers                          AuthConfigInfoMap `json:"identityMappers" yaml:"identityMappers"`
	PasswordAuthenticators                   AuthConfigInfoMap `json:"passwordAuthenticators" yaml:"passwordAuthenticators"`
	TokenAuthenticators                      AuthConfigInfoMap `json:"tokenAuthenticators" yaml:"tokenAuthenticators"`
	RequestAuthenticators                    AuthConfigInfoMap `json:"requestAuthenticators" yaml:"requestAuthenticators"`
	AuthenticationSuccessHandlers            AuthConfigInfoMap `json:"authenticationSuccessHandlers" yaml:"authenticationSuccessHandlers"`
	AuthorizeAuthenticationChallengeHandlers AuthConfigInfoMap `json:"authorizeAuthenticationChallengeHandlers" yaml:"authorizeAuthenticationChallengeHandlers"`
	AuthorizeAuthenticationRedirectHandlers  AuthConfigInfoMap `json:"authorizeAuthenticationRedirectHandlers" yaml:"authorizeAuthenticationRedirectHandlers"`

	GrantHandler AuthElementConfigInfo `json:"grantHandler" yaml:"grantHandler"`
}

type AuthConfig struct {
	IdentityMappers                          map[string]authapi.UserIdentityMapper
	PasswordAuthenticators                   map[string]authauthenticator.Password
	TokenAuthenticators                      map[string]authauthenticator.Token
	RequestAuthenticators                    map[string]authauthenticator.Request
	AuthenticationSuccessHandlers            map[string]oauthhandlers.AuthenticationSuccessHandler
	AuthorizeAuthenticationChallengeHandlers map[string]oauthhandlers.ChallengeAuthHandler
	AuthorizeAuthenticationRedirectHandlers  map[string]oauthhandlers.RedirectAuthHandler

	GrantHandler oauthhandlers.GrantHandler
}

func NewAuthConfig() *AuthConfig {
	return &AuthConfig{
		IdentityMappers:                          make(map[string]authapi.UserIdentityMapper),
		PasswordAuthenticators:                   make(map[string]authauthenticator.Password),
		TokenAuthenticators:                      make(map[string]authauthenticator.Token),
		RequestAuthenticators:                    make(map[string]authauthenticator.Request),
		AuthenticationSuccessHandlers:            make(map[string]oauthhandlers.AuthenticationSuccessHandler),
		AuthorizeAuthenticationChallengeHandlers: make(map[string]oauthhandlers.ChallengeAuthHandler),
		AuthorizeAuthenticationRedirectHandlers:  make(map[string]oauthhandlers.RedirectAuthHandler),
	}
}

type EnvInfo struct {
	MasterAddr     string
	SessionSecrets []string
	SessionStore   session.Store
	EtcdHelper     ktools.EtcdHelper
	Mux            cmdutil.Mux
}

type AuthConfigInstantiator interface {
	Owns(resultingType reflect.Type, elementConfigInfo AuthElementConfigInfo) bool
	IsValid(elementConfigInfo AuthElementConfigInfo, authConfigInfo AuthConfigInfo) error
	// there's got to be a way to do this like your would with generics
	Instantiate(resultingType reflect.Type, elementConfigInfo AuthElementConfigInfo, authConfig AuthConfig, envInfo *EnvInfo) (interface{}, error)
}

func (authConfig AuthConfig) GetAuthorizeRequestAuthenticator() authauthenticator.Request {
	var authRequestHandlers []authauthenticator.Request
	for _, requestAuthenticator := range authConfig.RequestAuthenticators {
		authRequestHandlers = append(authRequestHandlers, requestAuthenticator)
	}

	unionAuthRequestAuthenticator := requesthandlers.NewUnionAuthentication(authRequestHandlers)
	return unionAuthRequestAuthenticator
}

func (authConfig AuthConfig) GetAuthorizeAuthenticationHandler() oauthhandlers.AuthenticationHandler {
	errorHandler := oauthhandlers.EmptyError{}

	authHandler := oauthhandlers.NewUnionAuthenticationHandler(
		authConfig.AuthorizeAuthenticationChallengeHandlers,
		authConfig.AuthorizeAuthenticationRedirectHandlers,
		errorHandler,
	)

	return authHandler
}

func (authConfig AuthConfig) GetTokenAuthenticator() authauthenticator.Token {
	var authTokenAuthenticators []authauthenticator.Token
	for _, tokenAuthenticator := range authConfig.TokenAuthenticators {
		authTokenAuthenticators = append(authTokenAuthenticators, tokenAuthenticator)
	}

	unionAuthTokenAuthenticators := uniontoken.NewUnionAuthentication(authTokenAuthenticators)
	return unionAuthTokenAuthenticators
}
