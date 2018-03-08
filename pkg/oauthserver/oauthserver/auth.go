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
	"github.com/golang/glog"

	kerrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/request/union"
	x509request "k8s.io/apiserver/pkg/authentication/request/x509"
	kuser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/retry"

	oauthapi "github.com/openshift/api/oauth/v1"
	oauthclient "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	oauthutil "github.com/openshift/origin/pkg/oauth/util"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/challenger/passwordchallenger"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/challenger/placeholderchallenger"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/allowanypassword"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/basicauthpassword"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/denypassword"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/htpasswd"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/keystonepassword"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/password/ldappassword"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/redirector"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/request/basicauthrequest"
	"github.com/openshift/origin/pkg/oauthserver/authenticator/request/headerrequest"
	"github.com/openshift/origin/pkg/oauthserver/ldaputil"
	"github.com/openshift/origin/pkg/oauthserver/oauth/external"
	"github.com/openshift/origin/pkg/oauthserver/oauth/external/github"
	"github.com/openshift/origin/pkg/oauthserver/oauth/external/gitlab"
	"github.com/openshift/origin/pkg/oauthserver/oauth/external/google"
	"github.com/openshift/origin/pkg/oauthserver/oauth/external/openid"
	"github.com/openshift/origin/pkg/oauthserver/oauth/handlers"
	"github.com/openshift/origin/pkg/oauthserver/oauth/registry"
	"github.com/openshift/origin/pkg/oauthserver/osinserver"
	"github.com/openshift/origin/pkg/oauthserver/osinserver/registrystorage"
	"github.com/openshift/origin/pkg/oauthserver/server/csrf"
	"github.com/openshift/origin/pkg/oauthserver/server/errorpage"
	"github.com/openshift/origin/pkg/oauthserver/server/grant"
	"github.com/openshift/origin/pkg/oauthserver/server/login"
	"github.com/openshift/origin/pkg/oauthserver/server/selectprovider"
	"github.com/openshift/origin/pkg/oauthserver/server/tokenrequest"
	"github.com/openshift/origin/pkg/oauthserver/userregistry/identitymapper"
	saoauth "github.com/openshift/origin/pkg/serviceaccounts/oauthclient"
)

const (
	openShiftLoginPrefix         = "/login"
	openShiftApproveSubpath      = "approve"
	OpenShiftOAuthCallbackPrefix = "/oauth2callback"
	OpenShiftWebConsoleClientID  = "openshift-web-console"
	OpenShiftBrowserClientID     = "openshift-browser-client"
	OpenShiftCLIClientID         = "openshift-challenging-client"
)

// WithOAuth decorates the given handler by serving the OAuth2 endpoints while
// passing through all other requests to the given handler.
func (c *OAuthServerConfig) WithOAuth(handler http.Handler, requestContextMapper request.RequestContextMapper) (http.Handler, error) {
	baseMux := http.NewServeMux()
	mux := c.possiblyWrapMux(baseMux)

	// pass through all other requests
	mux.Handle("/", handler)

	combinedOAuthClientGetter := saoauth.NewServiceAccountOAuthClientGetter(
		c.ExtraOAuthConfig.KubeClient.CoreV1(),
		c.ExtraOAuthConfig.KubeClient.CoreV1(),
		c.ExtraOAuthConfig.EventsClient,
		c.ExtraOAuthConfig.RouteClient,
		c.ExtraOAuthConfig.OAuthClientClient,
		oauthapi.GrantHandlerType(c.ExtraOAuthConfig.Options.GrantConfig.ServiceAccountMethod),
	)

	errorPageHandler, err := c.getErrorHandler()
	if err != nil {
		glog.Fatal(err)
	}

	authRequestHandler, authHandler, authFinalizer, err := c.getAuthorizeAuthenticationHandlers(mux, errorPageHandler, requestContextMapper)
	if err != nil {
		glog.Fatal(err)
	}

	tokentimeout := int32(0)
	if timeout := c.ExtraOAuthConfig.Options.TokenConfig.AccessTokenInactivityTimeoutSeconds; timeout != nil {
		tokentimeout = *timeout
	}
	storage := registrystorage.New(c.ExtraOAuthConfig.OAuthAccessTokenClient, c.ExtraOAuthConfig.OAuthAuthorizeTokenClient, combinedOAuthClientGetter, registry.NewUserConversion(), tokentimeout)
	config := osinserver.NewDefaultServerConfig()
	if c.ExtraOAuthConfig.Options.TokenConfig.AuthorizeTokenMaxAgeSeconds > 0 {
		config.AuthorizationExpiration = c.ExtraOAuthConfig.Options.TokenConfig.AuthorizeTokenMaxAgeSeconds
	}
	if c.ExtraOAuthConfig.Options.TokenConfig.AccessTokenMaxAgeSeconds > 0 {
		config.AccessExpiration = c.ExtraOAuthConfig.Options.TokenConfig.AccessTokenMaxAgeSeconds
	}

	grantChecker := registry.NewClientAuthorizationGrantChecker(c.ExtraOAuthConfig.OAuthClientAuthorizationClient)
	grantHandler := c.getGrantHandler(mux, authRequestHandler, combinedOAuthClientGetter, c.ExtraOAuthConfig.OAuthClientAuthorizationClient)

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
	server.Install(mux, oauthutil.OpenShiftOAuthAPIPrefix)

	tokenRequestEndpoints := tokenrequest.NewEndpoints(c.ExtraOAuthConfig.Options.MasterPublicURL, c.getOsinOAuthClient)
	tokenRequestEndpoints.Install(mux, oauthutil.OpenShiftOAuthAPIPrefix)

	// glog.Infof("oauth server configured as: %#v", server)
	// glog.Infof("auth handler: %#v", authHandler)
	// glog.Infof("auth request handler: %#v", authRequestHandler)
	// glog.Infof("grant checker: %#v", grantChecker)
	// glog.Infof("grant handler: %#v", grantHandler)

	return baseMux, nil
}

func (c *OAuthServerConfig) getOsinOAuthClient() (*osincli.Client, error) {
	browserClient, err := c.ExtraOAuthConfig.OAuthClientClient.Get(OpenShiftBrowserClientID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	osOAuthClientConfig := newOpenShiftOAuthClientConfig(browserClient.Name, browserClient.Secret, c.ExtraOAuthConfig.Options.MasterPublicURL, c.ExtraOAuthConfig.Options.MasterURL)
	osOAuthClientConfig.RedirectUrl = oauthutil.OpenShiftOAuthTokenDisplayURL(c.ExtraOAuthConfig.Options.MasterPublicURL)

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

func (c *OAuthServerConfig) possiblyWrapMux(mux mux) mux {
	// Register directly into the given mux
	if c.ExtraOAuthConfig.HandlerWrapper == nil {
		return mux
	}

	// Wrap all handlers before registering into the container's mux
	// This lets us do things like defer session clearing to the end of a request
	return &handlerWrapperMux{
		mux:     mux,
		wrapper: c.ExtraOAuthConfig.HandlerWrapper,
	}
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
		AuthorizeUrl:             oauthutil.OpenShiftOAuthAuthorizeURL(masterPublicURL),
		TokenUrl:                 oauthutil.OpenShiftOAuthTokenURL(masterURL),
		Scope:                    "",
	}
	return config
}

func ensureOAuthClient(client oauthapi.OAuthClient, oauthClients oauthclient.OAuthClientInterface, preserveExistingRedirects, preserveExistingSecret bool) error {
	_, err := oauthClients.Create(&client)
	if err == nil || !kerrs.IsAlreadyExists(err) {
		return err
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		existing, err := oauthClients.Get(client.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		// Ensure the correct challenge setting
		existing.RespondWithChallenges = client.RespondWithChallenges
		// Preserve an existing client secret
		if !preserveExistingSecret || len(existing.Secret) == 0 {
			existing.Secret = client.Secret
		}

		// Preserve redirects for clients other than the CLI client
		// The CLI client doesn't care about the redirect URL, just the token or error fragment
		if preserveExistingRedirects {
			// Add in any redirects from the existing one
			// This preserves any additional customized redirects in the default clients
			redirects := sets.NewString(client.RedirectURIs...)
			for _, redirect := range existing.RedirectURIs {
				if !redirects.Has(redirect) {
					client.RedirectURIs = append(client.RedirectURIs, redirect)
					redirects.Insert(redirect)
				}
			}
		}
		existing.RedirectURIs = client.RedirectURIs

		// If the GrantMethod is present, keep it for compatibility
		// If it is empty, assign the requested strategy.
		if len(existing.GrantMethod) == 0 {
			existing.GrantMethod = client.GrantMethod
		}

		_, err = oauthClients.Update(existing)
		return err
	})
}

// getCSRF returns the object responsible for generating and checking CSRF tokens
func (c *OAuthServerConfig) getCSRF() csrf.CSRF {
	secure := isHTTPS(c.ExtraOAuthConfig.Options.MasterPublicURL)
	return csrf.NewCookieCSRF("csrf", "/", "", secure, true)
}

func (c *OAuthServerConfig) getAuthorizeAuthenticationHandlers(mux mux, errorHandler handlers.AuthenticationErrorHandler, requestContextMapper request.RequestContextMapper) (authenticator.Request, handlers.AuthenticationHandler, osinserver.AuthorizeHandler, error) {
	authRequestHandler, err := c.getAuthenticationRequestHandler()
	if err != nil {
		return nil, nil, nil, err
	}
	authHandler, err := c.getAuthenticationHandler(mux, errorHandler, requestContextMapper)
	if err != nil {
		return nil, nil, nil, err
	}
	authFinalizer := c.getAuthenticationFinalizer()

	return authRequestHandler, authHandler, authFinalizer, nil
}

// getGrantHandler returns the object that handles approving or rejecting grant requests
func (c *OAuthServerConfig) getGrantHandler(mux mux, auth authenticator.Request, clientregistry clientregistry.Getter, authregistry oauthclient.OAuthClientAuthorizationInterface) handlers.GrantHandler {
	// check that the global default strategy is something we honor
	if !configapi.ValidGrantHandlerTypes.Has(string(c.ExtraOAuthConfig.Options.GrantConfig.Method)) {
		glog.Fatalf("No grant handler found that matches %v.  The OAuth server cannot start!", c.ExtraOAuthConfig.Options.GrantConfig.Method)
	}

	// Since any OAuth client could require prompting, we will unconditionally
	// start the GrantServer here.
	grantServer := grant.NewGrant(c.getCSRF(), auth, grant.DefaultFormRenderer, clientregistry, authregistry)
	grantServer.Install(mux, path.Join(oauthutil.OpenShiftOAuthAPIPrefix, osinserver.AuthorizePath, openShiftApproveSubpath))

	// Set defaults for standard clients. These can be overridden.
	return handlers.NewPerClientGrant(
		handlers.NewRedirectGrant(openShiftApproveSubpath),
		oauthapi.GrantHandlerType(c.ExtraOAuthConfig.Options.GrantConfig.Method),
	)
}

// getAuthenticationFinalizer returns an authentication finalizer which is called just prior to writing a response to an authorization request
func (c *OAuthServerConfig) getAuthenticationFinalizer() osinserver.AuthorizeHandler {
	if c.ExtraOAuthConfig.SessionAuth != nil {
		// The session needs to know the authorize flow is done so it can invalidate the session
		return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error) {
			_ = c.ExtraOAuthConfig.SessionAuth.InvalidateAuthentication(w, ar.HttpRequest)
			return false, nil
		})
	}

	// Otherwise return a no-op finalizer
	return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error) {
		return false, nil
	})
}

func (c *OAuthServerConfig) getAuthenticationHandler(mux mux, errorHandler handlers.AuthenticationErrorHandler, requestContextMapper request.RequestContextMapper) (handlers.AuthenticationHandler, error) {
	// TODO: make this ordered once we can have more than one
	challengers := map[string]handlers.AuthenticationChallenger{}

	redirectors := new(handlers.AuthenticationRedirectors)

	// Determine if we have more than one password-based Identity Provider
	multiplePasswordProviders := false
	passwordProviderCount := 0
	for _, identityProvider := range c.ExtraOAuthConfig.Options.IdentityProviders {
		if configapi.IsPasswordAuthenticator(identityProvider) && identityProvider.UseAsLogin {
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
		if configapi.IsPasswordAuthenticator(identityProvider) {
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

				login := login.NewLogin(identityProvider.Name, c.getCSRF(), &callbackPasswordAuthenticator{passwordAuth, passwordSuccessHandler}, loginFormRenderer)
				login.Install(mux, loginPath)
			}
			if identityProvider.UseAsChallenger {
				// For now, all password challenges share a single basic challenger, since they'll all respond to any basic credentials
				challengers["basic-challenge"] = passwordchallenger.NewBasicAuthChallenger("openshift")
			}
		} else if configapi.IsOAuthIdentityProvider(identityProvider) {
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

			callbackPath := path.Join(OpenShiftOAuthCallbackPrefix, identityProvider.Name)
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
		} else if requestHeaderProvider, isRequestHeader := identityProvider.Provider.(*configapi.RequestHeaderIdentityProvider); isRequestHeader {
			// We might be redirecting to an external site, we need to fully resolve the request URL to the public master
			baseRequestURL, err := url.Parse(oauthutil.OpenShiftOAuthAuthorizeURL(c.ExtraOAuthConfig.Options.MasterPublicURL))
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
		challengers["placeholder"] = placeholderchallenger.New(oauthutil.OpenShiftOAuthTokenRequestURL(c.ExtraOAuthConfig.Options.MasterPublicURL))
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

	authHandler := handlers.NewUnionAuthenticationHandler(challengers, redirectors, errorHandler, selectProvider, requestContextMapper)
	return authHandler, nil
}

func (c *OAuthServerConfig) getOAuthProvider(identityProvider configapi.IdentityProvider) (external.Provider, error) {
	switch provider := identityProvider.Provider.(type) {
	case (*configapi.GitHubIdentityProvider):
		clientSecret, err := configapi.ResolveStringValue(provider.ClientSecret)
		if err != nil {
			return nil, err
		}
		return github.NewProvider(identityProvider.Name, provider.ClientID, clientSecret, provider.Organizations, provider.Teams), nil

	case (*configapi.GitLabIdentityProvider):
		transport, err := transportFor(provider.CA, "", "")
		if err != nil {
			return nil, err
		}
		clientSecret, err := configapi.ResolveStringValue(provider.ClientSecret)
		if err != nil {
			return nil, err
		}
		return gitlab.NewProvider(identityProvider.Name, transport, provider.URL, provider.ClientID, clientSecret)

	case (*configapi.GoogleIdentityProvider):
		clientSecret, err := configapi.ResolveStringValue(provider.ClientSecret)
		if err != nil {
			return nil, err
		}
		return google.NewProvider(identityProvider.Name, provider.ClientID, clientSecret, provider.HostedDomain)

	case (*configapi.OpenIDIdentityProvider):
		transport, err := transportFor(provider.CA, "", "")
		if err != nil {
			return nil, err
		}
		clientSecret, err := configapi.ResolveStringValue(provider.ClientSecret)
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

func (c *OAuthServerConfig) getPasswordAuthenticator(identityProvider configapi.IdentityProvider) (authenticator.Password, error) {
	identityMapper, err := identitymapper.NewIdentityUserMapper(c.ExtraOAuthConfig.IdentityClient, c.ExtraOAuthConfig.UserClient, c.ExtraOAuthConfig.UserIdentityMappingClient, identitymapper.MappingMethodType(identityProvider.MappingMethod))
	if err != nil {
		return nil, err
	}

	switch provider := identityProvider.Provider.(type) {
	case (*configapi.AllowAllPasswordIdentityProvider):
		return allowanypassword.New(identityProvider.Name, identityMapper), nil

	case (*configapi.DenyAllPasswordIdentityProvider):
		return denypassword.New(), nil

	case (*configapi.LDAPPasswordIdentityProvider):
		url, err := ldaputil.ParseURL(provider.URL)
		if err != nil {
			return nil, fmt.Errorf("Error parsing LDAPPasswordIdentityProvider URL: %v", err)
		}

		bindPassword, err := configapi.ResolveStringValue(provider.BindPassword)
		if err != nil {
			return nil, err
		}
		clientConfig, err := ldaputil.NewLDAPClientConfig(provider.URL,
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
			UserAttributeDefiner: ldaputil.NewLDAPUserAttributeDefiner(provider.Attributes),
		}
		return ldappassword.New(identityProvider.Name, opts, identityMapper)

	case (*configapi.HTPasswdPasswordIdentityProvider):
		htpasswdFile := provider.File
		if len(htpasswdFile) == 0 {
			return nil, fmt.Errorf("HTPasswdFile is required to support htpasswd auth")
		}
		if htpasswordAuth, err := htpasswd.New(identityProvider.Name, htpasswdFile, identityMapper); err != nil {
			return nil, fmt.Errorf("Error loading htpasswd file %s: %v", htpasswdFile, err)
		} else {
			return htpasswordAuth, nil
		}

	case (*configapi.BasicAuthPasswordIdentityProvider):
		connectionInfo := provider.RemoteConnectionInfo
		if len(connectionInfo.URL) == 0 {
			return nil, fmt.Errorf("URL is required for BasicAuthPasswordIdentityProvider")
		}
		transport, err := transportFor(connectionInfo.CA, connectionInfo.ClientCert.CertFile, connectionInfo.ClientCert.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("Error building BasicAuthPasswordIdentityProvider client: %v", err)
		}
		return basicauthpassword.New(identityProvider.Name, connectionInfo.URL, transport, identityMapper), nil

	case (*configapi.KeystonePasswordIdentityProvider):
		connectionInfo := provider.RemoteConnectionInfo
		if len(connectionInfo.URL) == 0 {
			return nil, fmt.Errorf("URL is required for KeystonePasswordIdentityProvider")
		}
		transport, err := transportFor(connectionInfo.CA, connectionInfo.ClientCert.CertFile, connectionInfo.ClientCert.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("Error building KeystonePasswordIdentityProvider client: %v", err)
		}

		return keystonepassword.New(identityProvider.Name, connectionInfo.URL, transport, provider.DomainName, identityMapper), nil

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

		if configapi.IsPasswordAuthenticator(identityProvider) {
			passwordAuthenticator, err := c.getPasswordAuthenticator(identityProvider)
			if err != nil {
				return nil, err
			}
			authRequestHandlers = append(authRequestHandlers, basicauthrequest.NewBasicAuthAuthentication(identityProvider.Name, passwordAuthenticator, true))

		} else if identityProvider.UseAsChallenger && configapi.IsOAuthIdentityProvider(identityProvider) {
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
			switch provider := identityProvider.Provider.(type) {
			case (*configapi.RequestHeaderIdentityProvider):
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
func transportFor(ca string, certFile string, keyFile string) (http.RoundTripper, error) {
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
