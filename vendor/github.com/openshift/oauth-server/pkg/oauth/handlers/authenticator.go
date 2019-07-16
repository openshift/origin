package handlers

import (
	"context"
	"net/http"

	"github.com/RangelReale/osin"
	"k8s.io/klog"

	"k8s.io/apiserver/pkg/authentication/authenticator"

	"github.com/openshift/oauth-server/pkg/api"
	openshiftauthenticator "github.com/openshift/oauth-server/pkg/authenticator"
	"github.com/openshift/oauth-server/pkg/osinserver"
)

// authorizeAuthenticator implements osinserver.AuthorizeHandler to ensure requests are authenticated
type authorizeAuthenticator struct {
	request      authenticator.Request
	handler      AuthenticationHandler
	errorHandler AuthenticationErrorHandler
}

// NewAuthorizeAuthenticator returns a new Authenticator
func NewAuthorizeAuthenticator(request authenticator.Request, handler AuthenticationHandler, errorHandler AuthenticationErrorHandler) osinserver.AuthorizeHandler {
	return &authorizeAuthenticator{request: request, handler: handler, errorHandler: errorHandler}
}

type TokenMaxAgeSeconds interface {
	// GetTokenMaxAgeSeconds returns the max age of the token in seconds.
	// 0 means no expiration.
	// nil means to use the default expiration.
	GetTokenMaxAgeSeconds() *int32
}

type TokenTimeoutSeconds interface {
	// GetAccessTokenInactivityTimeoutSeconds returns the inactivity timeout
	// for the token in seconds. 0 means no timeout.
	// nil means to use the default expiration.
	GetAccessTokenInactivityTimeoutSeconds() *int32
}

// HandleAuthorize implements osinserver.AuthorizeHandler to ensure the AuthorizeRequest is authenticated.
// If the request is authenticated, UserData and Authorized are set and false is returned.
// If the request is not authenticated, the auth handler is called and the request is not authorized
func (h *authorizeAuthenticator) HandleAuthorize(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error) {
	info, ok, err := h.request.AuthenticateRequest(ar.HttpRequest)
	if err != nil {
		klog.V(4).Infof("OAuth authentication error: %v", err)
		return h.errorHandler.AuthenticationError(err, w, ar.HttpRequest)
	}
	if !ok {
		return h.handler.AuthenticationNeeded(ar.Client, w, ar.HttpRequest)
	}
	klog.V(4).Infof("OAuth authentication succeeded: %#v", info.User)
	ar.UserData = info.User
	ar.Authorized = true

	// If requesting a token directly, optionally override the expiration
	if ar.Type == osin.TOKEN {
		if e, ok := ar.Client.(TokenMaxAgeSeconds); ok {
			if maxAge := e.GetTokenMaxAgeSeconds(); maxAge != nil {
				ar.Expiration = *maxAge
			}
		}
	}

	return false, nil
}

// accessAuthenticator implements osinserver.AccessHandler to ensure non-token requests are authenticated
type accessAuthenticator struct {
	password  authenticator.Password
	assertion openshiftauthenticator.Assertion
	client    openshiftauthenticator.Client
}

// HandleAccess implements osinserver.AccessHandler
func (h *accessAuthenticator) HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter) error {
	var (
		info *authenticator.Response
		ok   bool
		err  error
	)

	switch ar.Type {
	case osin.AUTHORIZATION_CODE, osin.REFRESH_TOKEN:
		// auth codes and refresh tokens are assumed allowed
		ok = true
	case osin.PASSWORD:
		ctx := context.TODO()
		if ar.HttpRequest != nil {
			ctx = ar.HttpRequest.Context()
		}
		info, ok, err = h.password.AuthenticatePassword(ctx, ar.Username, ar.Password)
	case osin.ASSERTION:
		info, ok, err = h.assertion.AuthenticateAssertion(ar.AssertionType, ar.Assertion)
	case osin.CLIENT_CREDENTIALS:
		info, ok, err = h.client.AuthenticateClient(ar.Client)
	default:
		klog.Warningf("Received unknown access token type: %s", ar.Type)
	}

	if err != nil {
		klog.V(4).Infof("Unable to authenticate %s: %v", ar.Type, err)
		return err
	}

	if ok {
		// Disable refresh_token generation
		ar.GenerateRefresh = false
		ar.Authorized = true
		if info != nil {
			// TODO something with audiences?
			ar.AccessData.UserData = info.User
		}

		if e, ok := ar.Client.(TokenMaxAgeSeconds); ok {
			if maxAge := e.GetTokenMaxAgeSeconds(); maxAge != nil {
				ar.Expiration = *maxAge
			}
		}
	}

	return nil
}

// NewDenyAccessAuthenticator returns an AccessAuthenticator which rejects all non-token access requests
func NewDenyAccessAuthenticator() osinserver.AccessHandler {
	return &accessAuthenticator{password: deny, assertion: deny, client: deny}
}

// Deny implements Password, Assertion, and Client authentication to deny all requests
var deny = &denyAuthenticator{}

// denyAuthenticator implements Password, Assertion, and Client authentication to return a fixed unauthorized response
type denyAuthenticator struct{}

// AuthenticatePassword implements authenticator.Password
func (*denyAuthenticator) AuthenticatePassword(ctx context.Context, user, password string) (*authenticator.Response, bool, error) {
	return nil, false, nil
}

// AuthenticateAssertion implements authenticator.Assertion
func (*denyAuthenticator) AuthenticateAssertion(assertionType, data string) (*authenticator.Response, bool, error) {
	return nil, false, nil
}

// AuthenticateClient implements authenticator.Client
func (*denyAuthenticator) AuthenticateClient(client api.Client) (*authenticator.Response, bool, error) {
	return nil, false, nil
}
