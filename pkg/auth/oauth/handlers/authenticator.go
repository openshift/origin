package handlers

import (
	"net/http"

	"github.com/RangelReale/osin"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

type AuthorizeAuthenticator struct {
	handler AuthenticationHandler
	request authenticator.Request
}

func NewAuthorizeAuthenticator(handler AuthenticationHandler, request authenticator.Request) *AuthorizeAuthenticator {
	return &AuthorizeAuthenticator{handler, request}
}

func (h *AuthorizeAuthenticator) HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter, req *http.Request) (handled bool) {
	info, ok, err := h.request.AuthenticateRequest(req)
	if err != nil {
		h.handler.AuthenticationError(err, w, req)
		return true
	}
	if !ok {
		h.handler.AuthenticationNeeded(w, req)
		return true
	}
	ar.UserData = info
	ar.Authorized = true
	return
}

type AccessAuthenticator struct {
	password  authenticator.Password
	assertion authenticator.Assertion
	client    authenticator.Client
}

func NewAccessAuthenticator(password authenticator.Password, assertion authenticator.Assertion, client authenticator.Client) *AccessAuthenticator {
	return &AccessAuthenticator{password, assertion, client}
}

func (h *AccessAuthenticator) HandleAccess(ar *osin.AccessRequest, w http.ResponseWriter, req *http.Request) {
	switch ar.Type {
	case osin.AUTHORIZATION_CODE, osin.REFRESH_TOKEN:
		// auth codes and refresh tokens are assumed allowed
	case osin.PASSWORD:
		info, ok, err := h.password.AuthenticatePassword(ar.Username, ar.Password)
		if err != nil {
			glog.Errorf("Unable to authenticate password: %v", err)
			return
		}
		if !ok {
			return
		}
		ar.AccessData.UserData = info
	case osin.ASSERTION:
		info, ok, err := h.assertion.AuthenticateAssertion(ar.AssertionType, ar.Assertion)
		if err != nil {
			glog.Errorf("Unable to authenticate password: %v", err)
			return
		}
		if !ok {
			return
		}
		ar.AccessData.UserData = info
	case osin.CLIENT_CREDENTIALS:
		info, ok, err := h.client.AuthenticateClient(ar.Client)
		if err != nil {
			glog.Errorf("Unable to authenticate password: %v", err)
			return
		}
		if !ok {
			return
		}
		ar.AccessData.UserData = info
		return
	default:
		glog.Warningf("Received unknown access token type: %s", ar.Type)
		return
	}

	ar.Authorized = true
}

func NewDenyAccessAuthenticator() *AccessAuthenticator {
	return &AccessAuthenticator{Deny, Deny, Deny}
}

var Deny = denyAuthenticator{}

type denyAuthenticator struct{}

func (denyAuthenticator) AuthenticatePassword(user, password string) (api.UserInfo, bool, error) {
	return nil, false, nil
}

func (denyAuthenticator) AuthenticateAssertion(assertionType, data string) (api.UserInfo, bool, error) {
	return nil, false, nil
}

func (denyAuthenticator) AuthenticateClient(client api.Client) (api.UserInfo, bool, error) {
	return nil, false, nil
}
