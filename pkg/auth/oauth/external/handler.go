package external

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"
	"k8s.io/kubernetes/pkg/auth/user"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/server/csrf"
)

// Handler exposes an external oauth provider flow (including the call back) as an oauth.handlers.AuthenticationHandler to allow our internal oauth
// server to use an external oauth provider for authentication
type Handler struct {
	provider     Provider
	state        State
	clientConfig *osincli.ClientConfig
	client       *osincli.Client
	success      handlers.AuthenticationSuccessHandler
	errorHandler handlers.AuthenticationErrorHandler
	mapper       authapi.UserIdentityMapper
}

func NewExternalOAuthRedirector(provider Provider, state State, redirectURL string, success handlers.AuthenticationSuccessHandler, errorHandler handlers.AuthenticationErrorHandler, mapper authapi.UserIdentityMapper) (*Handler, error) {
	clientConfig, err := provider.NewConfig()
	if err != nil {
		return nil, err
	}

	clientConfig.RedirectUrl = redirectURL

	client, err := osincli.NewClient(clientConfig)
	if err != nil {
		return nil, err
	}

	transport, err := provider.GetTransport()
	if err != nil {
		return nil, err
	}
	client.Transport = transport

	return &Handler{
		provider:     provider,
		state:        state,
		clientConfig: clientConfig,
		client:       client,
		success:      success,
		errorHandler: errorHandler,
		mapper:       mapper,
	}, nil
}

// AuthenticationRedirect implements oauth.handlers.RedirectAuthHandler
func (h *Handler) AuthenticationRedirect(w http.ResponseWriter, req *http.Request) error {
	glog.V(4).Infof("Authentication needed for %v", h)

	authReq := h.client.NewAuthorizeRequest(osincli.CODE)
	h.provider.AddCustomParameters(authReq)

	state, err := h.state.Generate(w, req)
	if err != nil {
		glog.V(4).Infof("Error generating state: %v", err)
		return err
	}

	oauthURL := authReq.GetAuthorizeUrlWithParams(state)
	glog.V(4).Infof("redirect to %v", oauthURL)

	http.Redirect(w, req, oauthURL.String(), http.StatusFound)
	return nil
}

// ServeHTTP handles the callback request in response to an external oauth flow
func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	// Extract auth code
	authReq := h.client.NewAuthorizeRequest(osincli.CODE)
	authData, err := authReq.HandleRequest(req)
	if err != nil {
		glog.V(4).Infof("Error handling request: %v", err)
		h.handleError(err, w, req)
		return
	}

	glog.V(4).Infof("Got auth data")

	// Validate state before making any server-to-server calls
	ok, err := h.state.Check(authData.State, req)
	if !ok {
		glog.V(4).Infof("State is invalid")
		err := errors.New("state is invalid")
		h.handleError(err, w, req)
		return
	}
	if err != nil {
		glog.V(4).Infof("Error verifying state: %v", err)
		h.handleError(err, w, req)
		return
	}

	// Exchange code for a token
	accessReq := h.client.NewAccessRequest(osincli.AUTHORIZATION_CODE, authData)
	accessData, err := accessReq.GetToken()
	if err != nil {
		glog.V(4).Infof("Error getting access token: %v", err)
		h.handleError(err, w, req)
		return
	}

	glog.V(4).Infof("Got access data")

	identity, ok, err := h.provider.GetUserIdentity(accessData)
	if err != nil {
		glog.V(4).Infof("Error getting userIdentityInfo info: %v", err)
		h.handleError(err, w, req)
		return
	}
	if !ok {
		glog.V(4).Infof("Could not get userIdentityInfo info from access token")
		err := errors.New("could not get userIdentityInfo info from access token")
		h.handleError(err, w, req)
		return
	}

	user, err := h.mapper.UserFor(identity)
	glog.V(4).Infof("Got userIdentityMapping: %#v", user)
	if err != nil {
		glog.V(4).Infof("Error creating or updating mapping for: %#v due to %v", identity, err)
		h.handleError(err, w, req)
		return
	}

	_, err = h.success.AuthenticationSucceeded(user, authData.State, w, req)
	if err != nil {
		glog.V(4).Infof("Error calling success handler: %v", err)
		h.handleError(err, w, req)
		return
	}
}

func (h *Handler) handleError(err error, w http.ResponseWriter, req *http.Request) {
	handled, err := h.errorHandler.AuthenticationError(err, w, req)
	if handled {
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(`An error occurred`))
}

// defaultState provides default state-building, validation, and parsing to contain CSRF and "then" redirection
type defaultState struct {
	csrf csrf.CSRF
}

// RedirectorState combines state generation/verification with redirections on authentication success and error
type RedirectorState interface {
	State
	handlers.AuthenticationSuccessHandler
	handlers.AuthenticationErrorHandler
}

func CSRFRedirectingState(csrf csrf.CSRF) RedirectorState {
	return &defaultState{csrf}
}

func (d *defaultState) Generate(w http.ResponseWriter, req *http.Request) (string, error) {
	then := req.URL.String()
	if len(then) == 0 {
		return "", errors.New("cannot generate state: request has no URL")
	}

	csrfToken, err := d.csrf.Generate(w, req)
	if err != nil {
		return "", err
	}

	state := url.Values{
		"csrf": {csrfToken},
		"then": {then},
	}
	return encodeState(state)
}

func (d *defaultState) Check(state string, req *http.Request) (bool, error) {
	values, err := decodeState(state)
	if err != nil {
		return false, err
	}
	csrf := values.Get("csrf")

	ok, err := d.csrf.Check(req, csrf)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, fmt.Errorf("state did not contain a valid CSRF token")
	}

	then := values.Get("then")
	if then == "" {
		return false, errors.New("state did not contain a redirect")
	}

	return true, nil
}

func (d *defaultState) AuthenticationSucceeded(user user.Info, state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	values, err := decodeState(state)
	if err != nil {
		return false, err
	}

	then := values.Get("then")
	if len(then) == 0 {
		return false, errors.New("no redirect given")
	}

	http.Redirect(w, req, then, http.StatusFound)
	return true, nil
}

// AuthenticationError handles the very specific case where the remote OAuth provider returned an error
// In that case, attempt to redirect to the "then" URL with all error parameters echoed
// In any other case, or if an error is encountered, returns false and the original error
func (d *defaultState) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	// only handle errors that came from the remote OAuth provider...
	osinErr, ok := err.(*osincli.Error)
	if !ok {
		return false, err
	}

	// with an OAuth error...
	if len(osinErr.Id) == 0 {
		return false, err
	}

	// if they embedded valid state...
	ok, stateErr := d.Check(osinErr.State, req)
	if !ok || stateErr != nil {
		return false, err
	}

	// if the state decodes...
	values, err := decodeState(osinErr.State)
	if err != nil {
		return false, err
	}

	// if it contains a redirect...
	then := values.Get("then")
	if len(then) == 0 {
		return false, err
	}

	// which parses...
	thenURL, urlErr := url.Parse(then)
	if urlErr != nil {
		return false, err
	}

	// Add in the error, error_description, error_uri params to the "then" redirect
	q := thenURL.Query()
	q.Set("error", osinErr.Id)
	if len(osinErr.Description) > 0 {
		q.Set("error_description", osinErr.Description)
	}
	if len(osinErr.URI) > 0 {
		q.Set("error_uri", osinErr.URI)
	}
	thenURL.RawQuery = q.Encode()

	http.Redirect(w, req, thenURL.String(), http.StatusFound)

	return true, nil
}

// URL-encode, then base-64 encode for OAuth providers that don't do a good job of treating the state param like an opaque value
func encodeState(values url.Values) (string, error) {
	return base64.URLEncoding.EncodeToString([]byte(values.Encode())), nil
}

func decodeState(state string) (url.Values, error) {
	decodedState, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return nil, err
	}
	return url.ParseQuery(string(decodedState))
}
