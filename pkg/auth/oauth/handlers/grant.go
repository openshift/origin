package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/RangelReale/osin"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/api"
	oapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	"github.com/openshift/origin/pkg/oauth/scope"
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
		h.handler.GrantNeeded(ar.Client, user, grant, w, req)
		return true
	}

	return
}

type emptyGrant struct{}

// NewEmptyGrant returns a no-op grant handler
func NewEmptyGrant() GrantHandler {
	return emptyGrant{}
}

// GrantNeeded implements the GrantHandler interface
func (emptyGrant) GrantNeeded(client api.Client, user api.UserInfo, grant *api.Grant, w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "<body>GrantNeeded - not implemented<pre>%#v\n%#v\n%#v</pre></body>", client, user, grant)
}

// GrantError implements the GrantHandler interface
func (emptyGrant) GrantError(err error, w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "<body>GrantError - %s</body>", err)
}

type autoGrant struct {
	authregistry clientauthorization.Registry
}

// NewAutoGrant returns a grant handler that automatically creates client authorizations
// when a grant is needed, then retries the original request
func NewAutoGrant(authregistry clientauthorization.Registry) GrantHandler {
	return &autoGrant{authregistry}
}

// GrantNeeded implements the GrantHandler interface
func (g *autoGrant) GrantNeeded(client api.Client, user api.UserInfo, grant *api.Grant, w http.ResponseWriter, req *http.Request) {
	clientAuthID := g.authregistry.ClientAuthorizationID(user.GetName(), client.GetId())
	clientAuth, err := g.authregistry.GetClientAuthorization(clientAuthID)
	if err == nil {
		// Add new scopes and update
		clientAuth.Scopes = scope.Add(clientAuth.Scopes, scope.Split(grant.Scope))
		err = g.authregistry.UpdateClientAuthorization(clientAuth)
		if err != nil {
			glog.Errorf("Unable to update authorization: %v", err)
			g.GrantError(err, w, req)
			return
		}
	} else {
		// Make sure client name, user name, grant scope, expiration, and redirect uri match
		clientAuth = &oapi.ClientAuthorization{
			UserName:   user.GetName(),
			UserUID:    user.GetUID(),
			ClientName: client.GetId(),
			Scopes:     scope.Split(grant.Scope),
		}
		clientAuth.Name = clientAuthID

		err = g.authregistry.CreateClientAuthorization(clientAuth)
		if err != nil {
			glog.Errorf("Unable to create authorization: %v", err)
			g.GrantError(err, w, req)
			return
		}
	}

	// Retry the request
	http.Redirect(w, req, req.URL.String(), http.StatusFound)
}

// GrantError implements the GrantHandler interface
func (g *autoGrant) GrantError(err error, w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "<body>GrantError - %s</body>", err)
}

type redirectGrant struct {
	url string
}

// NewRedirectGrant returns a grant handler that redirects to the given URL when a grant is needed.
// The following query parameters are added to the URL:
//   then - original request URL
//   client_id - requesting client's ID
//   scopes - grant scope requested
//   redirect_uri - original authorize request redirect_uri
func NewRedirectGrant(url string) GrantHandler {
	return &redirectGrant{url}
}

// GrantNeeded implements the GrantHandler interface
func (g *redirectGrant) GrantNeeded(client api.Client, user api.UserInfo, grant *api.Grant, w http.ResponseWriter, req *http.Request) {
	redirectURL, err := url.Parse(g.url)
	if err != nil {
		g.GrantError(err, w, req)
		return
	}
	redirectURL.RawQuery = url.Values{
		"then":         {req.URL.String()},
		"client_id":    {client.GetId()},
		"scopes":       {grant.Scope},
		"redirect_uri": {grant.RedirectURI},
	}.Encode()
	http.Redirect(w, req, redirectURL.String(), http.StatusFound)
}

// GrantError implements the GrantHandler interface
func (g *redirectGrant) GrantError(err error, w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, "<body>GrantError - %s</body>", err)
}
