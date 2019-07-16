package oauthserver

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"

	"github.com/RangelReale/osin"
	"github.com/RangelReale/osincli"
	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/union"
	x509request "k8s.io/apiserver/pkg/authentication/request/x509"
	kuser "k8s.io/apiserver/pkg/authentication/user"
	ktransport "k8s.io/client-go/transport"
	"k8s.io/client-go/util/cert"

	oauthapi "github.com/openshift/api/oauth/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	bootstrap "github.com/openshift/library-go/pkg/authentication/bootstrapauthenticator"
	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
	"github.com/openshift/library-go/pkg/oauth/oauthserviceaccountclient"
	"github.com/openshift/library-go/pkg/security/ldapclient"
	"github.com/openshift/library-go/pkg/security/ldaputil"
	oauthserver "github.com/openshift/oauth-server/pkg"
	"github.com/openshift/oauth-server/pkg/api"
	"github.com/openshift/oauth-server/pkg/authenticator/challenger/passwordchallenger"
	"github.com/openshift/oauth-server/pkg/authenticator/challenger/placeholderchallenger"
	"github.com/openshift/oauth-server/pkg/authenticator/password/allowanypassword"
	"github.com/openshift/oauth-server/pkg/authenticator/password/basicauthpassword"
	"github.com/openshift/oauth-server/pkg/authenticator/password/denypassword"
	"github.com/openshift/oauth-server/pkg/authenticator/password/htpasswd"
	"github.com/openshift/oauth-server/pkg/authenticator/password/keystonepassword"
	"github.com/openshift/oauth-server/pkg/authenticator/password/ldappassword"
	"github.com/openshift/oauth-server/pkg/authenticator/redirector"
	"github.com/openshift/oauth-server/pkg/authenticator/request/basicauthrequest"
	"github.com/openshift/oauth-server/pkg/authenticator/request/headerrequest"
	"github.com/openshift/oauth-server/pkg/config"
	"github.com/openshift/oauth-server/pkg/oauth/external"
	"github.com/openshift/oauth-server/pkg/oauth/external/github"
	"github.com/openshift/oauth-server/pkg/oauth/external/gitlab"
	"github.com/openshift/oauth-server/pkg/oauth/external/google"
	"github.com/openshift/oauth-server/pkg/oauth/external/openid"
	"github.com/openshift/oauth-server/pkg/oauth/handlers"
	"github.com/openshift/oauth-server/pkg/oauth/registry"
	"github.com/openshift/oauth-server/pkg/osinserver"
	"github.com/openshift/oauth-server/pkg/osinserver/registrystorage"
	"github.com/openshift/oauth-server/pkg/server/csrf"
	"github.com/openshift/oauth-server/pkg/server/errorpage"
	"github.com/openshift/oauth-server/pkg/server/grant"
	"github.com/openshift/oauth-server/pkg/server/login"
	"github.com/openshift/oauth-server/pkg/server/logout"
	"github.com/openshift/oauth-server/pkg/server/selectprovider"
	"github.com/openshift/oauth-server/pkg/server/tokenrequest"
	"github.com/openshift/oauth-server/pkg/userregistry/identitymapper"
)

const (
	openShiftLoginPrefix         = "/login"
	openShiftLogoutPrefix        = "/logout"
	openShiftApproveSubpath      = "approve"
	openShiftOAuthCallbackPrefix = "/oauth2callback"
	openShiftBrowserClientID     = "openshift-browser-client"
	openShiftCLIClientID         = "openshift-challenging-client"
)

// WithOAuth decorates the given handler by serving the OAuth2 endpoints while
// passing through all other requests to the given handler.
func (c *OAuthServerConfig) WithOAuth(handler http.Handler) (http.Handler, error) {
	mux := http.NewServeMux()

	// pass through all other requests
	mux.Handle("/", handler)

	combinedOAuthClientGetter := oauthserviceaccountclient.NewServiceAccountOAuthClientGetter(
		c.ExtraOAuthConfig.KubeClient.CoreV1(),
		c.ExtraOAuthConfig.KubeClient.CoreV1(),
		c.ExtraOAuthConfig.EventsClient,
		c.ExtraOAuthConfig.RouteClient,
		c.ExtraOAuthConfig.OAuthClientClient,
		oauthapi.GrantHandlerType(c.ExtraOAuthConfig.Options.GrantConfig.ServiceAccountMethod),
	)

	errorPageHandler, err := c.getErrorHandler()
	if err != nil {
		return nil, err
	}

	authRequestHandler, authHandler, authFinalizer, err := c.getAuthorizeAuthenticationHandlers(mux, errorPageHandler)
	if err != nil {
		return nil, err
	}

	tokentimeout := int32(0)
	if timeout := c.ExtraOAuthConfig.Options.TokenConfig.AccessTokenInactivityTimeoutSeconds; timeout != nil {
		tokentimeout = *timeout
	}
	storage := registrystorage.New(c.ExtraOAuthConfig.OAuthAccessTokenClient, c.ExtraOAuthConfig.OAuthAuthorizeTokenClient, combinedOAuthClientGetter, tokentimeout)
	config := osinserver.NewDefaultServerConfig()
	if authorizationExpiration := c.ExtraOAuthConfig.Options.TokenConfig.AuthorizeTokenMaxAgeSeconds; authorizationExpiration > 0 {
		config.AuthorizationExpiration = authorizationExpiration
	}
	if accessExpiration := c.ExtraOAuthConfig.Options.TokenConfig.AccessTokenMaxAgeSeconds; accessExpiration > 0 {
		config.AccessExpiration = accessExpiration
	}

	grantChecker := registry.NewClientAuthorizationGrantChecker(c.ExtraOAuthConfig.OAuthClientAuthorizationClient)
	grantHandler, err := c.getGrantHandler(mux, authRequestHandler, combinedOAuthClientGetter, c.ExtraOAuthConfig.OAuthClientAuthorizationClient)
	if err != nil {
		return nil, err
	}

	server := osinserver.New(
		config,
		storage,
		osinserver.AuthorizeHandlers{
			handlers.NewAuthorizeAuthenticator(
				authRequestHandler,
				authHandler,
				errorPageHandler,
			),
			handlers.NewGrantCheck(
				grantChecker,
				grantHandler,
				errorPageHandler,
			),
			authFinalizer,
		},
		osinserver.AccessHandlers{
			handlers.NewDenyAccessAuthenticator(),
		},
		osinserver.NewDefaultErrorHandler(),
	)
	server.Install(mux, oauthdiscovery.OpenShiftOAuthAPIPrefix)

	loginURL := c.ExtraOAuthConfig.Options.LoginURL
	if len(loginURL) == 0 {
		loginURL = c.ExtraOAuthConfig.Options.MasterPublicURL
	}

	tokenRequestEndpoints := tokenrequest.NewTokenRequest(loginURL, openShiftLogoutPrefix, c.getOsinOAuthClient, c.ExtraOAuthConfig.OAuthAccessTokenClient, c.getCSRF())
	tokenRequestEndpoints.Install(mux, oauthdiscovery.OpenShiftOAuthAPIPrefix)

	if session := c.ExtraOAuthConfig.SessionAuth; session != nil {
		logoutHandler := logout.NewLogout(session, c.ExtraOAuthConfig.Options.AssetPublicURL)
		logoutHandler.Install(mux, openShiftLogoutPrefix)
	}

	return mux, nil
}

func (c *OAuthServerConfig) getOsinOAuthClient() (*osincli.Client, error) {
	browserClient, err := c.ExtraOAuthConfig.OAuthClientClient.Get(openShiftBrowserClientID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	osOAuthClientConfig := newOpenShiftOAuthClientConfig(browserClient.Name, browserClient.Secret, c.ExtraOAuthConfig.Options.MasterPublicURL, c.ExtraOAuthConfig.Options.MasterURL)
	osOAuthClientConfig.RedirectUrl = oauthdiscovery.OpenShiftOAuthTokenDisplayURL(c.ExtraOAuthConfig.Options.MasterPublicURL)

	osOAuthClient, err := osincli.NewClient(osOAuthClientConfig)
	if err != nil {
		return nil, err
	}

	if len(*c.ExtraOAuthConfig.Options.MasterCA) > 0 {
		rootCAs, err := cert.NewPool(*c.ExtraOAuthConfig.Options.MasterCA)
		if err != nil {
			return nil, err
		}

		osOAuthClient.Transport = knet.SetTransportDefaults(&http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		})
	}

	return osOAuthClient, nil
}

func (c *OAuthServerConfig) getErrorHandler() (*errorpage.ErrorPage, error) {
	errorTemplate := ""
	if c.ExtraOAuthConfig.Options.Templates != nil {
		errorTemplate = c.ExtraOAuthConfig.Options.Templates.Error
	}
	errorPageRenderer, err := errorpage.NewErrorPageTemplateRenderer(errorTemplate)
	if err != nil {
		return nil, err
	}
	return errorpage.NewErrorPageHandler(errorPageRenderer), nil
}

// newOpenShiftOAuthClientConfig provides config for OpenShift OAuth client
func newOpenShiftOAuthClientConfig(clientId, clientSecret, masterPublicURL, masterURL string) *osincli.ClientConfig {
	config := &osincli.ClientConfig{
		ClientId:                 clientId,
		ClientSecret:             clientSecret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             oauthdiscovery.OpenShiftOAuthAuthorizeURL(masterPublicURL),
		TokenUrl:                 oauthdiscovery.OpenShiftOAuthTokenURL(masterURL),
		Scope:                    "",
	}
	return config
}

// getCSRF returns the object responsible for generating and checking CSRF tokens
func (c *OAuthServerConfig) getCSRF() csrf.CSRF {
	// TODO we really need to enforce HTTPS always
	secure := isHTTPS(c.ExtraOAuthConfig.Options.MasterPublicURL)
	return csrf.NewCookieCSRF("csrf", "/", "", secure)
}

func (c *OAuthServerConfig) getAuthorizeAuthenticationHandlers(mux oauthserver.Mux, errorHandler handlers.AuthenticationErrorHandler) (authenticator.Request, handlers.AuthenticationHandler, osinserver.AuthorizeHandler, error) {
	authRequestHandler, err := c.getAuthenticationRequestHandler()
	if err != nil {
		return nil, nil, nil, err
	}
	authHandler, err := c.getAuthenticationHandler(mux, errorHandler)
	if err != nil {
		return nil, nil, nil, err
	}
	authFinalizer := c.getAuthenticationFinalizer()

	return authRequestHandler, authHandler, authFinalizer, nil
}

// getGrantHandler returns the object that handles approving or rejecting grant requests
func (c *OAuthServerConfig) getGrantHandler(mux oauthserver.Mux, auth authenticator.Request, clientregistry api.OAuthClientGetter, authregistry oauthclient.OAuthClientAuthorizationInterface) (handlers.GrantHandler, error) {
	// check that the global default strategy is something we honor
	if !config.ValidGrantHandlerTypes.Has(string(c.ExtraOAuthConfig.Options.GrantConfig.Method)) {
		return nil, fmt.Errorf("No grant handler found that matches %v.  The OAuth server cannot start!", c.ExtraOAuthConfig.Options.GrantConfig.Method)
	}

	// Since any OAuth client could require prompting, we will unconditionally
	// start the GrantServer here.
	grantServer := grant.NewGrant(c.getCSRF(), auth, grant.DefaultFormRenderer, clientregistry, authregistry)
	grantServer.Install(mux, path.Join(oauthdiscovery.OpenShiftOAuthAPIPrefix, oauthdiscovery.AuthorizePath, openShiftApproveSubpath))

	// Set defaults for standard clients. These can be overridden.
	return handlers.NewPerClientGrant(
		handlers.NewRedirectGrant(openShiftApproveSubpath),
		oauthapi.GrantHandlerType(c.ExtraOAuthConfig.Options.GrantConfig.Method),
	), nil
}

// getAuthenticationFinalizer returns an authentication finalizer which is called just prior to writing a response to an authorization request
func (c *OAuthServerConfig) getAuthenticationFinalizer() osinserver.AuthorizeHandler {
	if c.ExtraOAuthConfig.SessionAuth != nil {
		// The session needs to know the authorize flow is done so it can invalidate the session
		return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error) {
			user, ok := ar.UserData.(kuser.Info)
			if !ok {
				klog.Errorf("the provided user data is not a user.Info object: %#v", user)
				user = &kuser.DefaultInfo{} // set non-nil so we always try to invalidate
			}

			if err := c.ExtraOAuthConfig.SessionAuth.InvalidateAuthentication(w, user); err != nil {
				klog.V(5).Infof("error invaliding cookie session: %v", err)
			}
			// do not fail the OAuth flow if we cannot invalidate the cookie
			// it will expire on its own regardless
			return false, nil
		})
	}

	// Otherwise return a no-op finalizer
	return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error) {
		return false, nil
	})
}

func (c *OAuthServerConfig) getAuthenticationHandler(mux oauthserver.Mux, errorHandler handlers.AuthenticationErrorHandler) (handlers.AuthenticationHandler, error) {
	// TODO: make this ordered once we can have more than one
	challengers := map[string]handlers.AuthenticationChallenger{}

	redirectors := new(handlers.AuthenticationRedirectors)

	// Determine if we have more than one password-based Identity Provider
	multiplePasswordProviders := false
	passwordProviderCount := 0
	for _, identityProvider := range c.ExtraOAuthConfig.Options.IdentityProviders {
		if config.IsPasswordAuthenticator(identityProvider) && identityProvider.UseAsLogin {
			passwordProviderCount++
			if passwordProviderCount > 1 {
				multiplePasswordProviders = true
				break
			}
		}
	}

	for _, identityProvider := range c.ExtraOAuthConfig.Options.IdentityProviders {
		identityMapper, err := identitymapper.NewIdentityUserMapper(c.ExtraOAuthConfig.IdentityClient, c.ExtraOAuthConfig.UserClient, c.ExtraOAuthConfig.UserIdentityMappingClient, identitymapper.MappingMethodType(identityProvider.MappingMethod))
		if err != nil {
			return nil, err
		}

		// TODO: refactor handler building per type
		if config.IsPasswordAuthenticator(identityProvider) {
			passwordAuth, err := c.getPasswordAuthenticator(identityProvider)
			if err != nil {
				return nil, err
			}

			if identityProvider.UseAsLogin {
				// Password auth requires:
				// 1. a session success handler (to remember you logged in)
				// 2. a redirectSuccessHandler (to go back to the "then" param)
				if c.ExtraOAuthConfig.SessionAuth == nil {
					return nil, errors.New("SessionAuth is required for password-based login")
				}
				passwordSuccessHandler := handlers.AuthenticationSuccessHandlers{c.ExtraOAuthConfig.SessionAuth, redirectSuccessHandler{}}

				var (
					// loginPath is unescaped, the way the mux will see it once URL-decoding is done
					loginPath = openShiftLoginPrefix
					// redirectLoginPath is escaped, the way we would need to send a Location redirect to a client
					redirectLoginPath = openShiftLoginPrefix
				)

				if multiplePasswordProviders {
					// If there is more than one Identity Provider acting as a login
					// provider, we need to give each of them their own login path,
					// to avoid ambiguity.
					loginPath = path.Join(openShiftLoginPrefix, identityProvider.Name)
					// url-encode the provider name for redirecting
					redirectLoginPath = path.Join(openShiftLoginPrefix, (&url.URL{Path: identityProvider.Name}).String())
				}

				// Since we're redirecting to a local login page, we don't need to force absolute URL resolution
				redirectors.Add(identityProvider.Name, redirector.NewRedirector(nil, redirectLoginPath+"?then=${server-relative-url}"))

				var loginTemplateFile string
				if c.ExtraOAuthConfig.Options.Templates != nil {
					loginTemplateFile = c.ExtraOAuthConfig.Options.Templates.Login
				}
				loginFormRenderer, err := login.NewLoginFormRenderer(loginTemplateFile)
				if err != nil {
					return nil, err
				}

				login := login.NewLogin(identityProvider.Name, c.getCSRF(), &callbackPasswordAuthenticator{Password: passwordAuth, AuthenticationSuccessHandler: passwordSuccessHandler}, loginFormRenderer)
				login.Install(mux, loginPath)
			}
			if identityProvider.UseAsChallenger {
				// For now, all password challenges share a single basic challenger, since they'll all respond to any basic credentials
				challengers["basic-challenge"] = passwordchallenger.NewBasicAuthChallenger("openshift")
			}
		} else if config.IsOAuthIdentityProvider(identityProvider) {
			oauthProvider, err := c.getOAuthProvider(identityProvider)
			if err != nil {
				return nil, err
			}

			// Default state builder, combining CSRF and return URL handling
			state := external.CSRFRedirectingState(c.getCSRF())

			// OAuth auth requires
			// 1. a session success handler (to remember you logged in)
			// 2. a state success handler (to go back to the URL encoded in the state)
			if c.ExtraOAuthConfig.SessionAuth == nil {
				return nil, errors.New("SessionAuth is required for OAuth-based login")
			}
			oauthSuccessHandler := handlers.AuthenticationSuccessHandlers{c.ExtraOAuthConfig.SessionAuth, state}

			// If the specified errorHandler doesn't handle the login error, let the state error handler attempt to propagate specific errors back to the token requester
			oauthErrorHandler := handlers.AuthenticationErrorHandlers{errorHandler, state}

			callbackPath := path.Join(openShiftOAuthCallbackPrefix, identityProvider.Name)
			oauthRedirector, oauthHandler, err := external.NewExternalOAuthRedirector(oauthProvider, state, c.ExtraOAuthConfig.Options.MasterPublicURL+callbackPath, oauthSuccessHandler, oauthErrorHandler, identityMapper)
			if err != nil {
				return nil, fmt.Errorf("unexpected error: %v", err)
			}

			mux.Handle(callbackPath, oauthHandler)
			if identityProvider.UseAsLogin {
				redirectors.Add(identityProvider.Name, oauthRedirector)
			}
			if identityProvider.UseAsChallenger {
				// For now, all password challenges share a single basic challenger, since they'll all respond to any basic credentials
				challengers["basic-challenge"] = passwordchallenger.NewBasicAuthChallenger("openshift")
			}
		} else if requestHeaderProvider, isRequestHeader := identityProvider.Provider.Object.(*osinv1.RequestHeaderIdentityProvider); isRequestHeader {
			// We might be redirecting to an external site, we need to fully resolve the request URL to the public master
			baseRequestURL, err := url.Parse(oauthdiscovery.OpenShiftOAuthAuthorizeURL(c.ExtraOAuthConfig.Options.MasterPublicURL))
			if err != nil {
				return nil, err
			}
			if identityProvider.UseAsChallenger {
				challengers["requestheader-"+identityProvider.Name+"-redirect"] = redirector.NewChallenger(baseRequestURL, requestHeaderProvider.ChallengeURL)
			}
			if identityProvider.UseAsLogin {
				redirectors.Add(identityProvider.Name, redirector.NewRedirector(baseRequestURL, requestHeaderProvider.LoginURL))
			}
		}
	}

	if redirectors.Count() > 0 && len(challengers) == 0 {
		// Add a default challenger that will warn and give a link to the web browser token-granting location
		challengers["placeholder"] = placeholderchallenger.New(oauthdiscovery.OpenShiftOAuthTokenRequestURL(c.ExtraOAuthConfig.Options.MasterPublicURL))
	}

	var selectProviderTemplateFile string
	if c.ExtraOAuthConfig.Options.Templates != nil {
		selectProviderTemplateFile = c.ExtraOAuthConfig.Options.Templates.ProviderSelection
	}
	selectProviderRenderer, err := selectprovider.NewSelectProviderRenderer(selectProviderTemplateFile)
	if err != nil {
		return nil, err
	}

	selectProvider := selectprovider.NewSelectProvider(selectProviderRenderer, c.ExtraOAuthConfig.Options.AlwaysShowProviderSelection)

	// the bootstrap user IDP is always set as the first one when sessions are enabled
	if c.ExtraOAuthConfig.Options.SessionConfig != nil {
		selectProvider = selectprovider.NewBootstrapSelectProvider(selectProvider, c.ExtraOAuthConfig.BootstrapUserDataGetter)
	}

	authHandler := handlers.NewUnionAuthenticationHandler(challengers, redirectors, errorHandler, selectProvider)
	return authHandler, nil
}

func (c *OAuthServerConfig) getOAuthProvider(identityProvider osinv1.IdentityProvider) (external.Provider, error) {
	switch provider := identityProvider.Provider.Object.(type) {
	case *osinv1.GitHubIdentityProvider:
		transport, err := transportFor(provider.CA, "", "")
		if err != nil {
			return nil, err
		}
		clientSecret, err := config.ResolveStringValue(provider.ClientSecret)
		if err != nil {
			return nil, err
		}
		return github.NewProvider(identityProvider.Name, provider.ClientID, clientSecret, provider.Hostname, transport, provider.Organizations, provider.Teams), nil

	case *osinv1.GitLabIdentityProvider:
		transport, err := transportFor(provider.CA, "", "")
		if err != nil {
			return nil, err
		}
		clientSecret, err := config.ResolveStringValue(provider.ClientSecret)
		if err != nil {
			return nil, err
		}
		return gitlab.NewProvider(identityProvider.Name, provider.URL, provider.ClientID, clientSecret, transport, provider.Legacy)

	case *osinv1.GoogleIdentityProvider:
		transport, err := transportFor("", "", "")
		if err != nil {
			return nil, err
		}
		clientSecret, err := config.ResolveStringValue(provider.ClientSecret)
		if err != nil {
			return nil, err
		}
		return google.NewProvider(identityProvider.Name, provider.ClientID, clientSecret, provider.HostedDomain, transport)

	case *osinv1.OpenIDIdentityProvider:
		transport, err := transportFor(provider.CA, "", "")
		if err != nil {
			return nil, err
		}
		clientSecret, err := config.ResolveStringValue(provider.ClientSecret)
		if err != nil {
			return nil, err
		}

		// OpenID Connect requests MUST contain the openid scope value
		// http://openid.net/specs/openid-connect-core-1_0.html#AuthRequest
		scopes := sets.NewString("openid")
		scopes.Insert(provider.ExtraScopes...)

		config := openid.Config{
			ClientID:     provider.ClientID,
			ClientSecret: clientSecret,

			Scopes: scopes.List(),

			ExtraAuthorizeParameters: provider.ExtraAuthorizeParameters,

			AuthorizeURL: provider.URLs.Authorize,
			TokenURL:     provider.URLs.Token,
			UserInfoURL:  provider.URLs.UserInfo,

			IDClaims:                provider.Claims.ID,
			PreferredUsernameClaims: provider.Claims.PreferredUsername,
			EmailClaims:             provider.Claims.Email,
			NameClaims:              provider.Claims.Name,
		}

		return openid.NewProvider(identityProvider.Name, transport, config)

	default:
		return nil, fmt.Errorf("No OAuth provider found that matches %v.  The OAuth server cannot start!", identityProvider)
	}

}

func (c *OAuthServerConfig) getPasswordAuthenticator(identityProvider osinv1.IdentityProvider) (authenticator.Password, error) {
	identityMapper, err := identitymapper.NewIdentityUserMapper(c.ExtraOAuthConfig.IdentityClient, c.ExtraOAuthConfig.UserClient, c.ExtraOAuthConfig.UserIdentityMappingClient, identitymapper.MappingMethodType(identityProvider.MappingMethod))
	if err != nil {
		return nil, err
	}

	switch provider := identityProvider.Provider.Object.(type) {
	case *osinv1.AllowAllPasswordIdentityProvider:
		return allowanypassword.New(identityProvider.Name, identityMapper), nil

	case *osinv1.DenyAllPasswordIdentityProvider:
		return denypassword.New(), nil

	case *osinv1.LDAPPasswordIdentityProvider:
		url, err := ldaputil.ParseURL(provider.URL)
		if err != nil {
			return nil, fmt.Errorf("Error parsing LDAPPasswordIdentityProvider URL: %v", err)
		}

		bindPassword, err := config.ResolveStringValue(provider.BindPassword)
		if err != nil {
			return nil, err
		}
		clientConfig, err := ldapclient.NewLDAPClientConfig(provider.URL,
			provider.BindDN,
			bindPassword,
			provider.CA,
			provider.Insecure)
		if err != nil {
			return nil, err
		}

		opts := ldappassword.Options{
			URL:                  url,
			ClientConfig:         clientConfig,
			UserAttributeDefiner: ldappassword.NewLDAPUserAttributeDefiner(provider.Attributes),
		}
		return ldappassword.New(identityProvider.Name, opts, identityMapper)

	case *osinv1.HTPasswdPasswordIdentityProvider:
		htpasswdFile := provider.File
		if len(htpasswdFile) == 0 {
			return nil, fmt.Errorf("HTPasswdFile is required to support htpasswd auth")
		}
		if htpasswordAuth, err := htpasswd.New(identityProvider.Name, htpasswdFile, identityMapper); err != nil {
			return nil, fmt.Errorf("Error loading htpasswd file %s: %v", htpasswdFile, err)
		} else {
			return htpasswordAuth, nil
		}

	case *osinv1.BasicAuthPasswordIdentityProvider:
		connectionInfo := provider.RemoteConnectionInfo
		if len(connectionInfo.URL) == 0 {
			return nil, fmt.Errorf("URL is required for BasicAuthPasswordIdentityProvider")
		}
		transport, err := transportFor(connectionInfo.CA, connectionInfo.CertInfo.CertFile, connectionInfo.CertInfo.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("Error building BasicAuthPasswordIdentityProvider client: %v", err)
		}
		return basicauthpassword.New(identityProvider.Name, connectionInfo.URL, transport, identityMapper), nil

	case *osinv1.KeystonePasswordIdentityProvider:
		connectionInfo := provider.RemoteConnectionInfo
		if len(connectionInfo.URL) == 0 {
			return nil, fmt.Errorf("URL is required for KeystonePasswordIdentityProvider")
		}
		transport, err := transportFor(connectionInfo.CA, connectionInfo.CertInfo.CertFile, connectionInfo.CertInfo.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("Error building KeystonePasswordIdentityProvider client: %v", err)
		}

		return keystonepassword.New(identityProvider.Name, connectionInfo.URL, transport, provider.DomainName, identityMapper, provider.UseKeystoneIdentity), nil

	case *config.BootstrapIdentityProvider:
		return bootstrap.New(c.ExtraOAuthConfig.BootstrapUserDataGetter), nil

	default:
		return nil, fmt.Errorf("No password auth found that matches %v.  The OAuth server cannot start!", identityProvider)
	}

}

func (c *OAuthServerConfig) getAuthenticationRequestHandler() (authenticator.Request, error) {
	var authRequestHandlers []authenticator.Request

	if c.ExtraOAuthConfig.SessionAuth != nil {
		authRequestHandlers = append(authRequestHandlers, c.ExtraOAuthConfig.SessionAuth)
	}

	for _, identityProvider := range c.ExtraOAuthConfig.Options.IdentityProviders {
		identityMapper, err := identitymapper.NewIdentityUserMapper(c.ExtraOAuthConfig.IdentityClient, c.ExtraOAuthConfig.UserClient, c.ExtraOAuthConfig.UserIdentityMappingClient, identitymapper.MappingMethodType(identityProvider.MappingMethod))
		if err != nil {
			return nil, err
		}

		if config.IsPasswordAuthenticator(identityProvider) {
			passwordAuthenticator, err := c.getPasswordAuthenticator(identityProvider)
			if err != nil {
				return nil, err
			}
			authRequestHandlers = append(authRequestHandlers, basicauthrequest.NewBasicAuthAuthentication(identityProvider.Name, passwordAuthenticator, true))

		} else if identityProvider.UseAsChallenger && config.IsOAuthIdentityProvider(identityProvider) {
			oauthProvider, err := c.getOAuthProvider(identityProvider)
			if err != nil {
				return nil, err
			}
			oauthPasswordAuthenticator, err := external.NewOAuthPasswordAuthenticator(oauthProvider, identityMapper)
			if err != nil {
				return nil, fmt.Errorf("unexpected error: %v", err)
			}

			authRequestHandlers = append(authRequestHandlers, basicauthrequest.NewBasicAuthAuthentication(identityProvider.Name, oauthPasswordAuthenticator, true))

		} else {
			switch provider := identityProvider.Provider.Object.(type) {
			case *osinv1.RequestHeaderIdentityProvider:
				var authRequestHandler authenticator.Request

				authRequestConfig := &headerrequest.Config{
					IDHeaders:                provider.Headers,
					NameHeaders:              provider.NameHeaders,
					EmailHeaders:             provider.EmailHeaders,
					PreferredUsernameHeaders: provider.PreferredUsernameHeaders,
				}
				authRequestHandler = headerrequest.NewAuthenticator(identityProvider.Name, authRequestConfig, identityMapper)

				// Wrap with an x509 verifier
				if len(provider.ClientCA) > 0 {
					caData, err := ioutil.ReadFile(provider.ClientCA)
					if err != nil {
						return nil, fmt.Errorf("Error reading %s: %v", provider.ClientCA, err)
					}
					opts := x509request.DefaultVerifyOptions()
					opts.Roots = x509.NewCertPool()
					if ok := opts.Roots.AppendCertsFromPEM(caData); !ok {
						return nil, fmt.Errorf("Error loading certs from %s: %v", provider.ClientCA, err)
					}

					authRequestHandler = x509request.NewVerifier(opts, authRequestHandler, sets.NewString(provider.ClientCommonNames...))
				}
				authRequestHandlers = append(authRequestHandlers, authRequestHandler)

			}
		}
	}

	authRequestHandler := union.New(authRequestHandlers...)
	return authRequestHandler, nil
}

// callbackPasswordAuthenticator combines password auth, successful login callback,
// and "then" param redirection
type callbackPasswordAuthenticator struct {
	authenticator.Password
	handlers.AuthenticationSuccessHandler
}

// redirectSuccessHandler redirects to the then param on successful authentication
type redirectSuccessHandler struct{}

// AuthenticationSucceeded informs client when authentication was successful
func (redirectSuccessHandler) AuthenticationSucceeded(user kuser.Info, then string, w http.ResponseWriter, req *http.Request) (bool, error) {
	if len(then) == 0 {
		return false, fmt.Errorf("Auth succeeded, but no redirect existed - user=%#v", user)
	}

	http.Redirect(w, req, then, http.StatusFound)
	return true, nil
}

// transportFor returns an http.Transport for the given ca and client cert (which may be empty strings)
func transportFor(ca, certFile, keyFile string) (http.RoundTripper, error) {
	transport, err := transportForInner(ca, certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return ktransport.DebugWrappers(transport), nil
}

func transportForInner(ca, certFile, keyFile string) (http.RoundTripper, error) {
	if len(ca) == 0 && len(certFile) == 0 && len(keyFile) == 0 {
		return http.DefaultTransport, nil
	}

	if (len(certFile) == 0) != (len(keyFile) == 0) {
		return nil, errors.New("certFile and keyFile must be specified together")
	}

	// Copy default transport
	transport := knet.SetTransportDefaults(&http.Transport{
		TLSClientConfig: &tls.Config{},
	})

	if len(ca) != 0 {
		roots, err := cert.NewPool(ca)
		if err != nil {
			return nil, fmt.Errorf("error loading cert pool from ca file %s: %v", ca, err)
		}
		transport.TLSClientConfig.RootCAs = roots
	}

	if len(certFile) != 0 {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("error loading x509 keypair from cert file %s and key file %s: %v", certFile, keyFile, err)
		}
		transport.TLSClientConfig.Certificates = []tls.Certificate{cert}
	}

	return transport, nil
}
