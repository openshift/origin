package tokenrequest

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/RangelReale/osincli"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	restclient "k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
	"github.com/openshift/library-go/pkg/oauth/tokenrequest/challengehandlers"
)

const (
	// csrfTokenHeader is a marker header that indicates we are not a browser that got tricked into requesting basic auth
	// Corresponds to the header expected by basic-auth challenging authenticators
	// Copied from pkg/auth/authenticator/challenger/passwordchallenger/password_auth_handler.go
	csrfTokenHeader = "X-CSRF-Token"

	// Discovery endpoint for OAuth 2.0 Authorization Server Metadata
	// See IETF Draft:
	// https://tools.ietf.org/html/draft-ietf-oauth-discovery-04#section-2
	// Copied from pkg/cmd/server/origin/nonapiserver.go
	oauthMetadataEndpoint = "/.well-known/oauth-authorization-server"

	// openShiftCLIClientID is the name of the CLI OAuth client, copied from pkg/oauth/apiserver/auth.go
	openShiftCLIClientID = "openshift-challenging-client"

	// pkce_s256 is sha256 hash per RFC7636, copied from github.com/RangelReale/osincli/pkce.go
	pkce_s256 = "S256"

	// token fakes the missing osin.TOKEN const
	token osincli.AuthorizeRequestType = "token"

	// BasicAuthNoUsernameMessage will differentiate unauthorized errors from basic login with no username
	BasicAuthNoUsernameMessage = "BasicChallengeNoUsername"
)

type RequestTokenOptions struct {
	ClientConfig *restclient.Config
	Handler      challengehandlers.ChallengeHandler
	OsinConfig   *osincli.ClientConfig
	Issuer       string
	TokenFlow    bool
}

// RequestToken uses the cmd arguments to locate an openshift oauth server and attempts to authenticate via an
// OAuth code flow and challenge handling.  It returns the access token if it gets one or an error if it does not.
func RequestToken(clientCfg *restclient.Config, challengeHandlers ...challengehandlers.ChallengeHandler) (string, error) {
	return NewRequestTokenOptions(clientCfg, false, challengeHandlers...).RequestToken()
}

func NewRequestTokenOptions(
	clientCfg *restclient.Config,
	tokenFlow bool,
	challengeHandlers ...challengehandlers.ChallengeHandler,
) *RequestTokenOptions {

	var handler challengehandlers.ChallengeHandler
	if len(challengeHandlers) == 1 {
		handler = challengeHandlers[0]
	} else {
		handler = challengehandlers.NewMultiHandler(challengeHandlers...)
	}

	return &RequestTokenOptions{
		ClientConfig: clientCfg,
		Handler:      handler,
		TokenFlow:    tokenFlow,
	}
}

// SetDefaultOsinConfig overwrites RequestTokenOptions.OsinConfig with the default CLI
// OAuth client and PKCE support if the server supports S256 / a code flow is being used
func (o *RequestTokenOptions) SetDefaultOsinConfig() error {
	if o.OsinConfig != nil {
		return fmt.Errorf("osin config is already set to: %#v", *o.OsinConfig)
	}

	// get the OAuth metadata directly from the api server
	// we only want to use the ca data from our config
	rt, err := restclient.TransportFor(o.ClientConfig)
	if err != nil {
		return err
	}

	requestURL := strings.TrimRight(o.ClientConfig.Host, "/") + oauthMetadataEndpoint
	resp, err := request(rt, requestURL, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("couldn't get %v: unexpected response status %v", requestURL, resp.StatusCode)
	}

	metadata := &oauthdiscovery.OauthAuthorizationServerMetadata{}
	if err := json.NewDecoder(resp.Body).Decode(metadata); err != nil {
		return err
	}

	// use the metadata to build the osin config
	config := &osincli.ClientConfig{
		ClientId:     openShiftCLIClientID,
		AuthorizeUrl: metadata.AuthorizationEndpoint,
		TokenUrl:     metadata.TokenEndpoint,
		RedirectUrl:  oauthdiscovery.OpenShiftOAuthTokenImplicitURL(metadata.Issuer),
	}
	if !o.TokenFlow && sets.NewString(metadata.CodeChallengeMethodsSupported...).Has(pkce_s256) {
		if err := osincli.PopulatePKCE(config); err != nil {
			return err
		}
	}

	o.OsinConfig = config
	o.Issuer = metadata.Issuer
	return nil
}

// RequestToken locates an openshift oauth server and attempts to authenticate.
// It returns the access token if it gets one, or an error if it does not.
// It should only be invoked once on a given RequestTokenOptions instance.
// The Handler held by the options is released as part of this call.
// If RequestTokenOptions.OsinConfig is nil, it will be defaulted using SetDefaultOsinConfig.
// The caller is responsible for setting up the entire OsinConfig if the value is not nil.
func (o *RequestTokenOptions) RequestToken() (string, error) {
	defer func() {
		// Always release the handler
		if err := o.Handler.Release(); err != nil {
			// Release errors shouldn't fail the token request, just log
			klog.V(4).Infof("error releasing handler: %v", err)
		}
	}()

	if o.OsinConfig == nil {
		if err := o.SetDefaultOsinConfig(); err != nil {
			return "", err
		}
	}

	// we are going to use this transport to talk
	// with a server that may not be the api server
	// thus we need to include the system roots
	// in our ca data otherwise an external
	// oauth server with a valid cert will fail with
	// error: x509: certificate signed by unknown authority
	rt, err := transportWithSystemRoots(o.Issuer, o.ClientConfig)
	if err != nil {
		return "", err
	}

	client, err := osincli.NewClient(o.OsinConfig)
	if err != nil {
		return "", err
	}
	client.Transport = rt
	authorizeRequest := client.NewAuthorizeRequest(osincli.CODE) // assume code flow to start with

	var oauthTokenFunc func(redirectURL string) (accessToken string, oauthError error)
	if o.TokenFlow {
		// access_token in fragment or error parameter
		authorizeRequest.Type = token // manually override to token flow if necessary
		oauthTokenFunc = oauthTokenFlow
	} else {
		// code or error parameter
		oauthTokenFunc = func(redirectURL string) (accessToken string, oauthError error) {
			return oauthCodeFlow(client, authorizeRequest, redirectURL)
		}
	}

	// requestURL holds the current URL to make requests to. This can change if the server responds with a redirect
	requestURL := authorizeRequest.GetAuthorizeUrl().String()
	// requestHeaders holds additional headers to add to the request. This can be changed by o.Handlers
	requestHeaders := http.Header{}
	// requestedURLSet/requestedURLList hold the URLs we have requested, to prevent redirect loops. Gets reset when a challenge is handled.
	requestedURLSet := sets.NewString()
	requestedURLList := []string{}
	handledChallenge := false

	for {
		// Make the request
		resp, err := request(rt, requestURL, requestHeaders)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusUnauthorized {
			if len(resp.Header.Get("WWW-Authenticate")) > 0 {
				if !o.Handler.CanHandle(resp.Header) {
					return "", apierrs.NewUnauthorized("unhandled challenge")
				}
				// Handle the challenge
				newRequestHeaders, shouldRetry, err := o.Handler.HandleChallenge(requestURL, resp.Header)
				if err != nil {
					if _, ok := err.(*challengehandlers.BasicAuthNoUsernameError); ok {
						tokenPromptErr := apierrs.NewUnauthorized(BasicAuthNoUsernameMessage)
						klog.V(2).Infof("%v", err)
						tokenPromptErr.ErrStatus.Details = &metav1.StatusDetails{
							Causes: []metav1.StatusCause{
								{Message: fmt.Sprintf("You must obtain an API token by visiting %s/request", o.OsinConfig.TokenUrl)},
							},
						}
						return "", tokenPromptErr
					}
					return "", err
				}
				if !shouldRetry {
					return "", apierrs.NewUnauthorized("challenger chose not to retry the request")
				}
				// Remember if we've ever handled a challenge
				handledChallenge = true

				// Reset request set/list. Since we're setting different headers, it is legitimate to request the same urls
				requestedURLSet = sets.NewString()
				requestedURLList = []string{}
				// Use the response to the challenge as the new headers
				requestHeaders = newRequestHeaders
				continue
			}

			// Unauthorized with no challenge
			unauthorizedError := apierrs.NewUnauthorized("")
			// Attempt to read body content and include as an error detail
			if details, err := ioutil.ReadAll(resp.Body); err == nil && len(details) > 0 {
				unauthorizedError.ErrStatus.Details = &metav1.StatusDetails{
					Causes: []metav1.StatusCause{
						{Message: string(details)},
					},
				}
			}

			return "", unauthorizedError
		}

		// if we've ever handled a challenge, see if the handler also considers the interaction complete.
		// this is required for negotiate flows with mutual authentication.
		if handledChallenge {
			if err := o.Handler.CompleteChallenge(requestURL, resp.Header); err != nil {
				return "", err
			}
		}

		if resp.StatusCode == http.StatusFound {
			redirectURL := resp.Header.Get("Location")

			// OAuth response case
			accessToken, err := oauthTokenFunc(redirectURL)
			if err != nil {
				return "", err
			}
			if len(accessToken) > 0 {
				return accessToken, nil
			}

			// Non-OAuth response, just follow the URL
			// add to our list of redirects
			requestedURLList = append(requestedURLList, redirectURL)
			// detect loops
			if !requestedURLSet.Has(redirectURL) {
				requestedURLSet.Insert(redirectURL)
				requestURL = redirectURL
				continue
			}
			return "", apierrs.NewInternalError(fmt.Errorf("redirect loop: %s", strings.Join(requestedURLList, " -> ")))
		}

		// Unknown response
		return "", apierrs.NewInternalError(fmt.Errorf("unexpected response: %d", resp.StatusCode))
	}
}

// oauthTokenFlow attempts to extract an OAuth token from location's fragment's access_token value.
// It only returns an error if something "impossible" happens (location is not a valid URL) or a definite
// OAuth error is contained in the location URL.  No error is returned if location does not contain a token.
// It is assumed that location was not part of the OAuth flow; it was a redirect that the client needs to follow
// as part of the challenge flow (an authenticating proxy for example) and not a redirect step in the OAuth flow.
func oauthTokenFlow(location string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}

	if oauthErr := oauthErrFromValues(u.Query()); oauthErr != nil {
		return "", oauthErr
	}

	// Grab the raw fragment ourselves, since the stdlib URL parsing decodes parts of it
	fragment := ""
	if parts := strings.SplitN(location, "#", 2); len(parts) == 2 {
		fragment = parts[1]
	}
	fragmentValues, err := url.ParseQuery(fragment)
	if err != nil {
		return "", err
	}

	return fragmentValues.Get("access_token"), nil
}

// oauthCodeFlow performs the OAuth code flow if location has a code parameter.
// It only returns an error if something "impossible" happens (location is not a valid URL)
// or a definite OAuth error is encountered during the code flow.  Other errors are assumed to be caused
// by location not being part of the OAuth flow; it was a redirect that the client needs to follow as part
// of the challenge flow (an authenticating proxy for example) and not a redirect step in the OAuth flow.
func oauthCodeFlow(client *osincli.Client, authorizeRequest *osincli.AuthorizeRequest, location string) (string, error) {
	// Make a request out of the URL since that is what AuthorizeRequest.HandleRequest expects to extract data from
	req, err := http.NewRequest(http.MethodGet, location, nil)
	if err != nil {
		return "", err
	}

	req.ParseForm()
	if oauthErr := oauthErrFromValues(req.Form); oauthErr != nil {
		return "", oauthErr
	}
	if len(req.Form.Get("code")) == 0 {
		return "", nil // no code parameter so this is not part of the OAuth flow
	}

	// any errors after this are fatal because we are committed to an OAuth flow now
	authorizeData, err := authorizeRequest.HandleRequest(req)
	if err != nil {
		return "", osinToOAuthError(err)
	}

	accessRequest := client.NewAccessRequest(osincli.AUTHORIZATION_CODE, authorizeData)
	accessData, err := accessRequest.GetToken()
	if err != nil {
		return "", osinToOAuthError(err)
	}

	return accessData.AccessToken, nil
}

// osinToOAuthError creates a better error message for osincli.Error
func osinToOAuthError(err error) error {
	if osinErr, ok := err.(*osincli.Error); ok {
		return createOAuthError(osinErr.Id, osinErr.Description)
	}
	return err
}

func oauthErrFromValues(values url.Values) error {
	if errorCode := values.Get("error"); len(errorCode) > 0 {
		errorDescription := values.Get("error_description")
		return createOAuthError(errorCode, errorDescription)
	}
	return nil
}

func createOAuthError(errorCode, errorDescription string) error {
	return fmt.Errorf("%s %s", errorCode, errorDescription)
}

func request(rt http.RoundTripper, requestURL string, requestHeaders http.Header) (*http.Response, error) {
	// Build the request
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range requestHeaders {
		req.Header[k] = v
	}
	req.Header.Set(csrfTokenHeader, "1")

	// Make the request
	return rt.RoundTrip(req)
}

// transportWithSystemRoots tries to retrieve the serving certificate from the
// issuer, validates it against the system roots and if the validation passes,
// returns transport using just system roots, otherwise it returns a transport
// that uses the CA from kubeconfig
func transportWithSystemRoots(issuer string, clientConfig *restclient.Config) (http.RoundTripper, error) {
	issuerURL, err := url.Parse(issuer)
	if err != nil {
		return nil, err
	}

	port := issuerURL.Port()
	if len(port) == 0 {
		port = "443"
	}
	// perform the retrieval with insecure transport, otherwise oauth-server
	// logs remote tls error which is confusing during troubleshooting
	client := http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{},
				InsecureSkipVerify: true,
				ServerName:         issuerURL.Hostname(),
			},
		},
	}
	resp, err := client.Head(issuer)
	if err != nil {
		return nil, err
	}
	resp.Body.Close()

	_, err = verifyServerCertChain(issuerURL.Hostname(), resp.TLS.PeerCertificates)
	switch err.(type) {
	case nil:
		// copy the config so we can freely mutate it
		configWithSystemRoots := restclient.CopyConfig(clientConfig)

		// explicitly unset CA cert information
		// this will make the transport use the system roots or OS specific verification
		// this is required to have reasonable behavior on windows (cannot get system roots)
		// in general there is no good with to say "I want system roots plus this CA bundle"
		// so we just try system roots first before using the kubeconfig CA bundle
		configWithSystemRoots.CAFile = ""
		configWithSystemRoots.CAData = nil

		// no error meaning the system roots work with the OAuth server
		klog.V(4).Info("using system roots as no error was encountered")
		systemRootsRT, err := restclient.TransportFor(configWithSystemRoots)
		if err != nil {
			return nil, err
		}
		return systemRootsRT, nil
	case x509.UnknownAuthorityError, x509.HostnameError, x509.CertificateInvalidError, x509.SystemRootsError,
		tls.RecordHeaderError, *net.OpError:
		// fallback to the CA in the kubeconfig since the system roots did not work
		// we are very broad on the errors here to avoid failing when we should fallback
		klog.V(4).Infof("falling back to kubeconfig CA due to possible x509 error: %v", err)
		return restclient.TransportFor(clientConfig)
	default:
		switch err {
		case io.EOF, io.ErrUnexpectedEOF, io.ErrNoProgress:
			// also fallback on various io errors
			klog.V(4).Infof("falling back to kubeconfig CA due to possible IO error: %v", err)
			return restclient.TransportFor(clientConfig)
		}
		// unknown error, fail (ideally should never occur)
		klog.V(4).Infof("unexpected error during system roots probe: %v", err)
		return nil, err
	}
}

// verifyCertChain uses the system trust bundle in order to perform validation
// of a certificate chain
func verifyServerCertChain(dnsName string, chain []*x509.Certificate) ([][]*x509.Certificate, error) {
	if len(chain) == 0 {
		return nil, fmt.Errorf("the server presented an empty certificate chain")
	}
	intermediates := x509.NewCertPool()

	for _, c := range chain[1:] {
		intermediates.AddCert(c)
	}

	return chain[0].Verify(x509.VerifyOptions{
		Intermediates: intermediates,
		DNSName:       dnsName,
	})
}
