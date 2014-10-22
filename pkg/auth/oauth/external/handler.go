package external

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
)

type Handler struct {
	provider     Provider
	state        State
	clientConfig *osincli.ClientConfig
	client       *osincli.Client
	success      handlers.AuthenticationSuccessHandler
	error        handlers.AuthenticationErrorHandler
}

func NewHandler(provider Provider, state State, redirectUrl string, success handlers.AuthenticationSuccessHandler, error handlers.AuthenticationErrorHandler) (*Handler, error) {
	clientConfig, err := provider.NewConfig()
	if err != nil {
		return nil, err
	}

	clientConfig.RedirectUrl = redirectUrl

	client, err := osincli.NewClient(clientConfig)
	if err != nil {
		return nil, err
	}

	return &Handler{
		provider:     provider,
		state:        state,
		clientConfig: clientConfig,
		client:       client,
		success:      success,
		error:        error,
	}, nil
}

// Implements oauth.handlers.AuthenticationHandler
func (h *Handler) AuthenticationNeeded(w http.ResponseWriter, req *http.Request) {
	glog.V(4).Infof("Authentication needed for %v", h)

	authReq := h.client.NewAuthorizeRequest(osincli.CODE)
	h.provider.AddCustomParameters(authReq)

	state, err := h.state.Generate(w, req)
	if err != nil {
		glog.V(4).Infof("Error generating state: %v", err)
		h.AuthenticationError(err, w, req)
		return
	}

	oauthUrl := authReq.GetAuthorizeUrlWithParams(state)
	glog.V(4).Infof("redirect to %v", oauthUrl)

	http.Redirect(w, req, oauthUrl.String(), http.StatusFound)
}

// Implements oauth.handlers.AuthenticationHandler
func (h *Handler) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) {
	h.error.AuthenticationError(err, w, req)
}

// Handles the callback request in response to an external oauth flow
func (h *Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	// Extract auth code
	authReq := h.client.NewAuthorizeRequest(osincli.CODE)
	authData, err := authReq.HandleRequest(req)
	if err != nil {
		glog.V(4).Infof("Error handling request: %v", err)
		h.AuthenticationError(err, w, req)
		return
	}

	glog.V(4).Infof("Got auth data")

	// Exchange code for a token
	accessReq := h.client.NewAccessRequest(osincli.AUTHORIZATION_CODE, authData)
	accessData, err := accessReq.GetToken()
	if err != nil {
		glog.V(4).Infof("Error getting access token:", err)
		h.AuthenticationError(err, w, req)
		return
	}

	glog.V(4).Infof("Got access data")

	user, ok, err := h.provider.GetUserInfo(accessData)
	if err != nil {
		glog.V(4).Infof("Error getting user info: %v", err)
		h.AuthenticationError(err, w, req)
		return
	}
	if !ok || user == nil {
		glog.V(4).Infof("Could not get user info from access token")
		h.AuthenticationError(fmt.Errorf("Could not get user info from access token"), w, req)
		return
	}

	glog.V(4).Infof("Got user data: %#v", user)

	ok, err = h.state.Check(authData.State, w, req)
	if !ok {
		glog.V(4).Infof("State is invalid")
		h.AuthenticationError(fmt.Errorf("State is invalid"), w, req)
		return
	}
	if err != nil {
		glog.V(4).Infof("Error verifying state: %v", err)
		h.AuthenticationError(err, w, req)
		return
	}

	err = h.success.AuthenticationSucceeded(user, authData.State, w, req)
	if err != nil {
		glog.V(4).Infof("Error calling success handler: %v", err)
		h.AuthenticationError(err, w, req)
		return
	}
}

// Provides default state-building, validation, and parsing to contain CSRF and "then" redirection
type defaultState struct{}

func DefaultState() State {
	return defaultState{}
}
func (defaultState) Generate(w http.ResponseWriter, req *http.Request) (string, error) {
	state := url.Values{
		"csrf": {"..."}, // TODO: get csrf
		"then": {req.URL.String()},
	}
	return state.Encode(), nil
}
func (defaultState) Check(state string, w http.ResponseWriter, req *http.Request) (bool, error) {
	values, err := url.ParseQuery(state)
	if err != nil {
		return false, err
	}
	csrf := values.Get("csrf")
	if csrf != "..." {
		return false, fmt.Errorf("State did not contain valid CSRF token (expected %s, got %s)", "...", csrf)
	}

	then := values.Get("then")
	if then == "" {
		return false, fmt.Errorf("State did not contain a redirect")
	}

	return true, nil
}
func (defaultState) AuthenticationSucceeded(user api.UserInfo, state string, w http.ResponseWriter, req *http.Request) error {
	values, err := url.ParseQuery(state)
	if err != nil {
		return err
	}

	then := values.Get("then")
	if len(then) == 0 {
		return fmt.Errorf("No redirect given")
	}

	http.Redirect(w, req, then, http.StatusFound)
	return nil
}
