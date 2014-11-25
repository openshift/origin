package handlers

import (
	"net/http"

	"github.com/RangelReale/osin"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

// AuthorizeAuthenticator implements osinserver.AuthorizeHandler to ensure requests are authenticated
type AuthorizeAuthenticator struct {
	request      authenticator.Request
	handler      AuthenticationHandler
	errorHandler AuthenticationErrorHandler
}

// NewAuthorizeAuthenticator returns a new Authenticator
func NewAuthorizeAuthenticator(request authenticator.Request, handler AuthenticationHandler, errorHandler AuthenticationErrorHandler) *AuthorizeAuthenticator {
	return &AuthorizeAuthenticator{request, handler, errorHandler}
}

// HandleAuthorize implements osinserver.AuthorizeHandler to ensure the AuthorizeRequest is authenticated.
// If the request is authenticated, UserData and Authorized are set and false is returned.
// If the request is not authenticated, the auth handler is called and the request is not authorized
func (h *AuthorizeAuthenticator) HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
	info, ok, err := h.request.AuthenticateRequest(ar.HttpRequest)
	if err != nil {
		return h.errorHandler.AuthenticationError(err, w, ar.HttpRequest)
	}
	if !ok {
		return h.handler.AuthenticationNeeded(ar.Client, w, ar.HttpRequest)
	}
	ar.UserData = info
	ar.Authorized = true
	return false, nil
}

// AccessAuthenticator implements osinserver.AccessHandler to ensure non-token requests are authenticated
type AccessAuthenticator struct {
	password  authenticator.Password
	assertion authenticator.Assertion
	client    authenticator.Client
}

// NewAccessAuthenticator returns a new AccessAuthenticator
func NewAccessAuthenticator(password authenticator.Password, assertion authenticator.Assertion, client authenticator.Client) *AccessAuthenticator {
	return &AccessAuthenticator{password, assertion, client}
}

// HandleAccess implements osinserver.AccessHandler
func (h *AccessAuthenticator) HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter) error {
	var (
		info api.UserInfo
		ok   bool
		err  error
	)

	switch ar.Type {
	case osin.AUTHORIZATION_CODE, osin.REFRESH_TOKEN:
		// auth codes and refresh tokens are assumed allowed
		ok = true
	case osin.PASSWORD:
		info, ok, err = h.password.AuthenticatePassword(ar.Username, ar.Password)
	case osin.ASSERTION:
		info, ok, err = h.assertion.AuthenticateAssertion(ar.AssertionType, ar.Assertion)
	case osin.CLIENT_CREDENTIALS:
		info, ok, err = h.client.AuthenticateClient(ar.Client)
	default:
		glog.Warningf("Received unknown access token type: %s", ar.Type)
	}

	if err != nil {
		glog.V(4).Infof("Unable to authenticate %s: %v", ar.Type, err)
		return err
	}

	if ok {
		ar.Authorized = true
		if info != nil {
			ar.AccessData.UserData = info
		}
	}
	return nil
}

// NewDenyAuthenticator returns an Authenticator which rejects all non-token access requests
func NewDenyAccessAuthenticator() *AccessAuthenticator {
	return &AccessAuthenticator{Deny, Deny, Deny}
}

// Deny implements Password, Assertion, and Client authentication to deny all requests
var Deny = &fixedAuthenticator{false}

// Allow implements Password, Assertion, and Client authentication to allow all requests
var Allow = &fixedAuthenticator{true}

// fixedAuthenticator implements Password, Assertion, and Client authentication to return a fixed response
type fixedAuthenticator struct {
	allow bool
}

// AuthenticatePassword implements authenticator.Password
func (f *fixedAuthenticator) AuthenticatePassword(user, password string) (api.UserInfo, bool, error) {
	return nil, f.allow, nil
}

// AuthenticateAssertion implements authenticator.Assertion
func (f *fixedAuthenticator) AuthenticateAssertion(assertionType, data string) (api.UserInfo, bool, error) {
	return nil, f.allow, nil
}

// AuthenticateClient implements authenticator.Client
func (f *fixedAuthenticator) AuthenticateClient(client api.Client) (api.UserInfo, bool, error) {
	return nil, f.allow, nil
}
