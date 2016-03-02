package origin

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
	"github.com/emicklei/go-restful"
	"github.com/golang/glog"
	"github.com/pborman/uuid"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrs "k8s.io/kubernetes/pkg/api/errors"
	kuser "k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/unversioned"
	knet "k8s.io/kubernetes/pkg/util/net"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/challenger/passwordchallenger"
	"github.com/openshift/origin/pkg/auth/authenticator/challenger/placeholderchallenger"
	"github.com/openshift/origin/pkg/auth/authenticator/password/allowanypassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/basicauthpassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/denypassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/htpasswd"
	"github.com/openshift/origin/pkg/auth/authenticator/password/keystonepassword"
	"github.com/openshift/origin/pkg/auth/authenticator/password/ldappassword"
	"github.com/openshift/origin/pkg/auth/authenticator/redirector"
	"github.com/openshift/origin/pkg/auth/authenticator/request/basicauthrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/headerrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/unionrequest"
	"github.com/openshift/origin/pkg/auth/authenticator/request/x509request"
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/auth/oauth/external"
	"github.com/openshift/origin/pkg/auth/oauth/external/github"
	"github.com/openshift/origin/pkg/auth/oauth/external/gitlab"
	"github.com/openshift/origin/pkg/auth/oauth/external/google"
	"github.com/openshift/origin/pkg/auth/oauth/external/openid"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/oauth/registry"
	"github.com/openshift/origin/pkg/auth/server/csrf"
	"github.com/openshift/origin/pkg/auth/server/errorpage"
	"github.com/openshift/origin/pkg/auth/server/grant"
	"github.com/openshift/origin/pkg/auth/server/login"
	"github.com/openshift/origin/pkg/auth/server/selectprovider"
	"github.com/openshift/origin/pkg/auth/server/tokenrequest"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	accesstokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken"
	accesstokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthaccesstoken/etcd"
	authorizetokenregistry "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken"
	authorizetokenetcd "github.com/openshift/origin/pkg/oauth/registry/oauthauthorizetoken/etcd"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	clientetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclient/etcd"
	clientauthregistry "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization"
	clientauthetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization/etcd"
	"github.com/openshift/origin/pkg/oauth/server/osinserver"
	"github.com/openshift/origin/pkg/oauth/server/osinserver/registrystorage"
)

const (
	OpenShiftOAuthAPIPrefix      = "/oauth"
	openShiftLoginPrefix         = "/login"
	OpenShiftApprovePrefix       = "/oauth/approve"
	OpenShiftOAuthCallbackPrefix = "/oauth2callback"
	OpenShiftWebConsoleClientID  = "openshift-web-console"
	OpenShiftBrowserClientID     = "openshift-browser-client"
	OpenShiftCLIClientID         = "openshift-challenging-client"
)

// InstallAPI registers endpoints for an OAuth2 server into the provided mux,
// then returns an array of strings indicating what endpoints were started
// (these are format strings that will expect to be sent a single string value).
func (c *AuthConfig) InstallAPI(container *restful.Container) ([]string, error) {
	// TODO: register into container
	mux := container.ServeMux

	accessTokenStorage := accesstokenetcd.NewREST(c.EtcdHelper, c.EtcdBackends...)
	accessTokenRegistry := accesstokenregistry.NewRegistry(accessTokenStorage)
	authorizeTokenStorage := authorizetokenetcd.NewREST(c.EtcdHelper, c.EtcdBackends...)
	authorizeTokenRegistry := authorizetokenregistry.NewRegistry(authorizeTokenStorage)
	clientStorage := clientetcd.NewREST(c.EtcdHelper)
	clientRegistry := clientregistry.NewRegistry(clientStorage)
	clientAuthStorage := clientauthetcd.NewREST(c.EtcdHelper)
	clientAuthRegistry := clientauthregistry.NewRegistry(clientAuthStorage)

	errorPageHandler, err := c.getErrorHandler()
	if err != nil {
		glog.Fatal(err)
	}

	authRequestHandler, authHandler, authFinalizer, err := c.getAuthorizeAuthenticationHandlers(mux, errorPageHandler)
	if err != nil {
		glog.Fatal(err)
	}

	storage := registrystorage.New(accessTokenRegistry, authorizeTokenRegistry, clientRegistry, registry.NewUserConversion())
	config := osinserver.NewDefaultServerConfig()
	if c.Options.TokenConfig.AuthorizeTokenMaxAgeSeconds > 0 {
		config.AuthorizationExpiration = c.Options.TokenConfig.AuthorizeTokenMaxAgeSeconds
	}
	if c.Options.TokenConfig.AccessTokenMaxAgeSeconds > 0 {
		config.AccessExpiration = c.Options.TokenConfig.AccessTokenMaxAgeSeconds
	}

	grantChecker := registry.NewClientAuthorizationGrantChecker(clientAuthRegistry)
	grantHandler := c.getGrantHandler(mux, authRequestHandler, clientRegistry, clientAuthRegistry)

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
	server.Install(mux, OpenShiftOAuthAPIPrefix)

	if err := CreateOrUpdateDefaultOAuthClients(c.Options.MasterPublicURL, c.AssetPublicAddresses, clientRegistry); err != nil {
		glog.Fatal(err)
	}
	browserClient, err := clientRegistry.GetClient(kapi.NewContext(), OpenShiftBrowserClientID)
	if err != nil {
		glog.Fatal(err)
	}
	osOAuthClientConfig := c.NewOpenShiftOAuthClientConfig(browserClient)
	osOAuthClientConfig.RedirectUrl = c.Options.MasterPublicURL + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.DisplayTokenEndpoint)

	osOAuthClient, _ := osincli.NewClient(osOAuthClientConfig)
	if len(*c.Options.MasterCA) > 0 {
		rootCAs, err := cmdutil.CertPoolFromFile(*c.Options.MasterCA)
		if err != nil {
			glog.Fatal(err)
		}

		osOAuthClient.Transport = knet.SetTransportDefaults(&http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: rootCAs},
		})
	}

	tokenRequestEndpoints := tokenrequest.NewEndpoints(c.Options.MasterPublicURL, osOAuthClient)
	tokenRequestEndpoints.Install(mux, OpenShiftOAuthAPIPrefix)

	// glog.Infof("oauth server configured as: %#v", server)
	// glog.Infof("auth handler: %#v", authHandler)
	// glog.Infof("auth request handler: %#v", authRequestHandler)
	// glog.Infof("grant checker: %#v", grantChecker)
	// glog.Infof("grant handler: %#v", grantHandler)

	return []string{
		fmt.Sprintf("Started OAuth2 API at %%s%s", OpenShiftOAuthAPIPrefix),
	}, nil
}

func (c *AuthConfig) getErrorHandler() (*errorpage.ErrorPage, error) {
	errorTemplate := ""
	if c.Options.Templates != nil {
		errorTemplate = c.Options.Templates.Error
	}
	errorPageRenderer, err := errorpage.NewErrorPageTemplateRenderer(errorTemplate)
	if err != nil {
		return nil, err
	}
	return errorpage.NewErrorPageHandler(errorPageRenderer), nil
}

// NewOpenShiftOAuthClientConfig provides config for OpenShift OAuth client
func (c *AuthConfig) NewOpenShiftOAuthClientConfig(client *oauthapi.OAuthClient) *osincli.ClientConfig {
	config := &osincli.ClientConfig{
		ClientId:                 client.Name,
		ClientSecret:             client.Secret,
		ErrorsInStatusCode:       true,
		SendClientSecretInParams: true,
		AuthorizeUrl:             OpenShiftOAuthAuthorizeURL(c.Options.MasterPublicURL),
		TokenUrl:                 OpenShiftOAuthTokenURL(c.Options.MasterURL),
		Scope:                    "",
	}
	return config
}

func OpenShiftOAuthAuthorizeURL(masterAddr string) string {
	return masterAddr + path.Join(OpenShiftOAuthAPIPrefix, osinserver.AuthorizePath)
}
func OpenShiftOAuthTokenURL(masterAddr string) string {
	return masterAddr + path.Join(OpenShiftOAuthAPIPrefix, osinserver.TokenPath)
}
func OpenShiftOAuthTokenRequestURL(masterAddr string) string {
	return masterAddr + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.RequestTokenEndpoint)
}

func ensureOAuthClient(client oauthapi.OAuthClient, clientRegistry clientregistry.Registry, preserveExistingRedirects bool) error {
	ctx := kapi.NewContext()
	_, err := clientRegistry.CreateClient(ctx, &client)
	if err == nil || !kerrs.IsAlreadyExists(err) {
		return err
	}

	return unversioned.RetryOnConflict(unversioned.DefaultRetry, func() error {
		existing, err := clientRegistry.GetClient(ctx, client.Name)
		if err != nil {
			return err
		}

		// Ensure the correct challenge setting
		existing.RespondWithChallenges = client.RespondWithChallenges
		// Preserve an existing client secret
		if len(existing.Secret) == 0 {
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

		_, err = clientRegistry.UpdateClient(ctx, existing)
		return err
	})
}

func CreateOrUpdateDefaultOAuthClients(masterPublicAddr string, assetPublicAddresses []string, clientRegistry clientregistry.Registry) error {
	{
		webConsoleClient := oauthapi.OAuthClient{
			ObjectMeta:            kapi.ObjectMeta{Name: OpenShiftWebConsoleClientID},
			Secret:                uuid.New(),
			RespondWithChallenges: false,
			RedirectURIs:          assetPublicAddresses,
		}
		if err := ensureOAuthClient(webConsoleClient, clientRegistry, true); err != nil {
			return err
		}
	}

	{
		browserClient := oauthapi.OAuthClient{
			ObjectMeta:            kapi.ObjectMeta{Name: OpenShiftBrowserClientID},
			Secret:                uuid.New(),
			RespondWithChallenges: false,
			RedirectURIs:          []string{masterPublicAddr + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.DisplayTokenEndpoint)},
		}
		if err := ensureOAuthClient(browserClient, clientRegistry, true); err != nil {
			return err
		}
	}

	{
		cliClient := oauthapi.OAuthClient{
			ObjectMeta:            kapi.ObjectMeta{Name: OpenShiftCLIClientID},
			Secret:                uuid.New(),
			RespondWithChallenges: true,
			RedirectURIs:          []string{masterPublicAddr + path.Join(OpenShiftOAuthAPIPrefix, tokenrequest.ImplicitTokenEndpoint)},
		}
		if err := ensureOAuthClient(cliClient, clientRegistry, false); err != nil {
			return err
		}
	}

	return nil
}

// getCSRF returns the object responsible for generating and checking CSRF tokens
func (c *AuthConfig) getCSRF() csrf.CSRF {
	secure := isHTTPS(c.Options.MasterPublicURL)
	return csrf.NewCookieCSRF("csrf", "/", "", secure, true)
}

func (c *AuthConfig) getAuthorizeAuthenticationHandlers(mux cmdutil.Mux, errorHandler handlers.AuthenticationErrorHandler) (authenticator.Request, handlers.AuthenticationHandler, osinserver.AuthorizeHandler, error) {
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
func (c *AuthConfig) getGrantHandler(mux cmdutil.Mux, auth authenticator.Request, clientregistry clientregistry.Registry, authregistry clientauthregistry.Registry) handlers.GrantHandler {
	switch c.Options.GrantConfig.Method {
	case configapi.GrantHandlerDeny:
		return handlers.NewEmptyGrant()

	case configapi.GrantHandlerAuto:
		return handlers.NewAutoGrant()

	case configapi.GrantHandlerPrompt:
		grantServer := grant.NewGrant(c.getCSRF(), auth, grant.DefaultFormRenderer, clientregistry, authregistry)
		grantServer.Install(mux, OpenShiftApprovePrefix)
		return handlers.NewRedirectGrant(OpenShiftApprovePrefix)

	default:
		glog.Fatalf("No grant handler found that matches %v.  The oauth server cannot start!", c.Options.GrantConfig.Method)
	}

	return nil
}

// getAuthenticationFinalizer returns an authentication finalizer which is called just prior to writing a response to an authorization request
func (c *AuthConfig) getAuthenticationFinalizer() osinserver.AuthorizeHandler {
	if c.SessionAuth != nil {
		// The session needs to know the authorize flow is done so it can invalidate the session
		return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
			_ = c.SessionAuth.InvalidateAuthentication(w, ar.HttpRequest)
			return false, nil
		})
	}

	// Otherwise return a no-op finalizer
	return osinserver.AuthorizeHandlerFunc(func(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
		return false, nil
	})
}

func (c *AuthConfig) getAuthenticationHandler(mux cmdutil.Mux, errorHandler handlers.AuthenticationErrorHandler) (handlers.AuthenticationHandler, error) {
	// TODO: make this ordered once we can have more than one
	challengers := map[string]handlers.AuthenticationChallenger{}

	redirectors := new(handlers.AuthenticationRedirectors)

	// Determine if we have more than one password-based Identity Provider
	multiplePasswordProviders := false
	passwordProviderCount := 0
	for _, identityProvider := range c.Options.IdentityProviders {
		if configapi.IsPasswordAuthenticator(identityProvider) && identityProvider.UseAsLogin {
			passwordProviderCount++
			if passwordProviderCount > 1 {
				multiplePasswordProviders = true
				break
			}
		}
	}

	for _, identityProvider := range c.Options.IdentityProviders {
		identityMapper, err := identitymapper.NewIdentityUserMapper(c.IdentityRegistry, c.UserRegistry, identitymapper.MappingMethodType(identityProvider.MappingMethod))
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
				if c.SessionAuth == nil {
					return nil, errors.New("SessionAuth is required for password-based login")
				}
				passwordSuccessHandler := handlers.AuthenticationSuccessHandlers{c.SessionAuth, redirectSuccessHandler{}}

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
				redirectors.Add(identityProvider.Name, redirector.NewRedirector(nil, redirectLoginPath+"?then=${url}"))

				var loginTemplateFile string
				if c.Options.Templates != nil {
					loginTemplateFile = c.Options.Templates.Login
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
			if c.SessionAuth == nil {
				return nil, errors.New("SessionAuth is required for OAuth-based login")
			}
			oauthSuccessHandler := handlers.AuthenticationSuccessHandlers{c.SessionAuth, state}

			// If the specified errorHandler doesn't handle the login error, let the state error handler attempt to propagate specific errors back to the token requester
			oauthErrorHandler := handlers.AuthenticationErrorHandlers{errorHandler, state}

			callbackPath := path.Join(OpenShiftOAuthCallbackPrefix, identityProvider.Name)
			oauthHandler, err := external.NewExternalOAuthRedirector(oauthProvider, state, c.Options.MasterPublicURL+callbackPath, oauthSuccessHandler, oauthErrorHandler, identityMapper)
			if err != nil {
				return nil, fmt.Errorf("unexpected error: %v", err)
			}

			mux.Handle(callbackPath, oauthHandler)
			if identityProvider.UseAsLogin {
				redirectors.Add(identityProvider.Name, oauthHandler)
			}
			if identityProvider.UseAsChallenger {
				return nil, errors.New("oauth identity providers cannot issue challenges")
			}
		} else if requestHeaderProvider, isRequestHeader := identityProvider.Provider.(*configapi.RequestHeaderIdentityProvider); isRequestHeader {
			// We might be redirecting to an external site, we need to fully resolve the request URL to the public master
			baseRequestURL, err := url.Parse(c.Options.MasterPublicURL + OpenShiftOAuthAPIPrefix + osinserver.AuthorizePath)
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
		challengers["placeholder"] = placeholderchallenger.New(OpenShiftOAuthTokenRequestURL(c.Options.MasterPublicURL))
	}

	var selectProviderTemplateFile string
	if c.Options.Templates != nil {
		selectProviderTemplateFile = c.Options.Templates.ProviderSelection
	}
	selectProviderRenderer, err := selectprovider.NewSelectProviderRenderer(selectProviderTemplateFile)
	if err != nil {
		return nil, err
	}

	selectProvider := selectprovider.NewSelectProvider(selectProviderRenderer, c.Options.AlwaysShowProviderSelection)

	authHandler := handlers.NewUnionAuthenticationHandler(challengers, redirectors, errorHandler, selectProvider)
	return authHandler, nil
}

func (c *AuthConfig) getOAuthProvider(identityProvider configapi.IdentityProvider) (external.Provider, error) {
	switch provider := identityProvider.Provider.(type) {
	case (*configapi.GitHubIdentityProvider):
		clientSecret, err := configapi.ResolveStringValue(provider.ClientSecret)
		if err != nil {
			return nil, err
		}
		return github.NewProvider(identityProvider.Name, provider.ClientID, clientSecret, provider.Organizations), nil

	case (*configapi.GitLabIdentityProvider):
		transport, err := cmdutil.TransportFor(provider.CA, "", "")
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
		transport, err := cmdutil.TransportFor(provider.CA, "", "")
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

func (c *AuthConfig) getPasswordAuthenticator(identityProvider configapi.IdentityProvider) (authenticator.Password, error) {
	identityMapper, err := identitymapper.NewIdentityUserMapper(c.IdentityRegistry, c.UserRegistry, identitymapper.MappingMethodType(identityProvider.MappingMethod))
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
		transport, err := cmdutil.TransportFor(connectionInfo.CA, connectionInfo.ClientCert.CertFile, connectionInfo.ClientCert.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("Error building BasicAuthPasswordIdentityProvider client: %v", err)
		}
		return basicauthpassword.New(identityProvider.Name, connectionInfo.URL, transport, identityMapper), nil

	case (*configapi.KeystonePasswordIdentityProvider):
		connectionInfo := provider.RemoteConnectionInfo
		if len(connectionInfo.URL) == 0 {
			return nil, fmt.Errorf("URL is required for KeystonePasswordIdentityProvider")
		}
		transport, err := cmdutil.TransportFor(connectionInfo.CA, connectionInfo.ClientCert.CertFile, connectionInfo.ClientCert.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("Error building KeystonePasswordIdentityProvider client: %v", err)
		}

		return keystonepassword.New(identityProvider.Name, connectionInfo.URL, transport, provider.DomainName, identityMapper), nil

	default:
		return nil, fmt.Errorf("No password auth found that matches %v.  The OAuth server cannot start!", identityProvider)
	}

}

func (c *AuthConfig) getAuthenticationRequestHandler() (authenticator.Request, error) {
	var authRequestHandlers []authenticator.Request

	if c.SessionAuth != nil {
		authRequestHandlers = append(authRequestHandlers, c.SessionAuth)
	}

	for _, identityProvider := range c.Options.IdentityProviders {
		identityMapper, err := identitymapper.NewIdentityUserMapper(c.IdentityRegistry, c.UserRegistry, identitymapper.MappingMethodType(identityProvider.MappingMethod))
		if err != nil {
			return nil, err
		}

		if configapi.IsPasswordAuthenticator(identityProvider) {
			passwordAuthenticator, err := c.getPasswordAuthenticator(identityProvider)
			if err != nil {
				return nil, err
			}
			authRequestHandlers = append(authRequestHandlers, basicauthrequest.NewBasicAuthAuthentication(identityProvider.Name, passwordAuthenticator, true))

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

					authRequestHandler = x509request.NewVerifier(opts, authRequestHandler)
				}
				authRequestHandlers = append(authRequestHandlers, authRequestHandler)

			}
		}
	}

	authRequestHandler := unionrequest.NewUnionAuthentication(authRequestHandlers...)
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

// authenticationHandlerFilter creates a filter object that will enforce authentication directly
func authenticationHandlerFilter(handler http.Handler, authenticator authenticator.Request, contextMapper kapi.RequestContextMapper) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		user, ok, err := authenticator.AuthenticateRequest(req)
		if err != nil || !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx, ok := contextMapper.Get(req)
		if !ok {
			http.Error(w, "Unable to find request context", http.StatusInternalServerError)
			return
		}
		if err := contextMapper.Update(req, kapi.WithUser(ctx, user)); err != nil {
			glog.V(4).Infof("Error setting authenticated context: %v", err)
			http.Error(w, "Unable to set authenticated request context", http.StatusInternalServerError)
			return
		}

		handler.ServeHTTP(w, req)
	})
}
