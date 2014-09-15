package handlers

import (
	"errors"
	"net/http"

	"github.com/RangelReale/osin"

	"github.com/openshift/origin/pkg/auth/api"
)

type GrantCheck struct {
	check   GrantChecker
	handler GrantHandler
}

func NewGrantCheck(check GrantChecker, handler GrantHandler) *GrantCheck {
	return &GrantCheck{check, handler}
}

func (h *GrantCheck) HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter, req *http.Request) (handled bool) {
	if !ar.Authorized {
		return
	}

	user, ok := ar.UserData.(api.UserInfo)
	if !ok || user == nil {
		h.handler.GrantError(errors.New("the provided user data is not api.UserInfo"), w, req)
		return true
	}

	grant := &api.Grant{
		Client:      ar.Client,
		Scope:       ar.Scope,
		Expiration:  int64(ar.Expiration),
		RedirectURI: ar.RedirectUri,
	}

	ok, err := h.check.HasAuthorizedClient(ar.Client, user, grant)
	if err != nil {
		h.handler.GrantError(err, w, req)
		return true
	}
	if !ok {
		h.handler.GrantNeeded(grant, w, req)
		return true
	}

	return
}
