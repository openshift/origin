package handlers

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/RangelReale/osin"
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/openshift/origin/pkg/auth/api"
	oapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	"github.com/openshift/origin/pkg/oauth/scope"
)

// GrantCheck implements osinserver.AuthorizeHandler to ensure requested scopes have been authorized
type GrantCheck struct {
	check        GrantChecker
	handler      GrantHandler
	errorHandler GrantErrorHandler
}

// NewGrantCheck returns a new GrantCheck
func NewGrantCheck(check GrantChecker, handler GrantHandler, errorHandler GrantErrorHandler) *GrantCheck {
	return &GrantCheck{check, handler, errorHandler}
}

// HandleAuthorize implements osinserver.AuthorizeHandler to ensure the requested scopes have been authorized.
// The AuthorizeRequest.Authorized field must already be set to true for the grant check to occur.
// If the requested scopes are authorized, the AuthorizeRequest is unchanged.
// If the requested scopes are not authorized, or an error occurs, AuthorizeRequest.Authorized is set to false.
// If the response is written, true is returned.
// If the response is not written, false is returned.
func (h *GrantCheck) HandleAuthorize(ar *osin.AuthorizeRequest, w http.ResponseWriter) (bool, error) {
	// Requests must already be authorized before we will check grants
	if !ar.Authorized {
		return false, nil
	}

	// Reset request to unauthorized until we verify the grant
	ar.Authorized = false

	user, ok := ar.UserData.(user.Info)
	if !ok || user == nil {
		return h.errorHandler.GrantError(errors.New("the provided user data is not user.Info"), w, ar.HttpRequest)
	}

	grant := &api.Grant{
		Client:      ar.Client,
		Scope:       ar.Scope,
		Expiration:  int64(ar.Expiration),
		RedirectURI: ar.RedirectUri,
	}

	ok, err := h.check.HasAuthorizedClient(user, grant)
	if err != nil {
		return h.errorHandler.GrantError(err, w, ar.HttpRequest)
	}
	if !ok {
		return h.handler.GrantNeeded(user, grant, w, ar.HttpRequest)
	}

	// Grant is verified
	ar.Authorized = true

	return false, nil
}

type emptyGrant struct{}

// NewEmptyGrant returns a no-op grant handler
func NewEmptyGrant() GrantHandler {
	return emptyGrant{}
}

// GrantNeeded implements the GrantHandler interface
func (emptyGrant) GrantNeeded(user user.Info, grant *api.Grant, w http.ResponseWriter, req *http.Request) (bool, error) {
	return false, nil
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
func (g *autoGrant) GrantNeeded(user user.Info, grant *api.Grant, w http.ResponseWriter, req *http.Request) (bool, error) {
	clientAuthID := g.authregistry.ClientAuthorizationName(user.GetName(), grant.Client.GetId())
	clientAuth, err := g.authregistry.GetClientAuthorization(clientAuthID)
	if err == nil {
		// Add new scopes and update
		clientAuth.Scopes = scope.Add(clientAuth.Scopes, scope.Split(grant.Scope))
		err = g.authregistry.UpdateClientAuthorization(clientAuth)
		if err != nil {
			glog.V(4).Infof("Unable to update authorization: %v", err)
			return false, err
		}
	} else {
		// Make sure client name, user name, grant scope, expiration, and redirect uri match
		clientAuth = &oapi.OAuthClientAuthorization{
			UserName:   user.GetName(),
			UserUID:    user.GetUID(),
			ClientName: grant.Client.GetId(),
			Scopes:     scope.Split(grant.Scope),
		}
		clientAuth.Name = clientAuthID

		err = g.authregistry.CreateClientAuthorization(clientAuth)
		if err != nil {
			glog.V(4).Infof("Unable to create authorization: %v", err)
			return false, err
		}
	}

	// Retry the request
	http.Redirect(w, req, req.URL.String(), http.StatusFound)
	return true, nil
}

type redirectGrant struct {
	url string
}

// If a user denies a grant, a grant handler can return control to the /authorize handler with an error=grant_denied parameter
// and the denial will be returned to the client, rather than re-calling GrantNeeded
const GrantDeniedError = "grant_denied"

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
func (g *redirectGrant) GrantNeeded(user user.Info, grant *api.Grant, w http.ResponseWriter, req *http.Request) (bool, error) {
	// If the current request has an error=grant_denied parameter, the user denied the grant
	if err := req.FormValue("error"); err == GrantDeniedError {
		return false, nil
	}

	redirectURL, err := url.Parse(g.url)
	if err != nil {
		return false, err
	}
	redirectURL.RawQuery = url.Values{
		"then":         {req.URL.String()},
		"client_id":    {grant.Client.GetId()},
		"scopes":       {grant.Scope},
		"redirect_uri": {grant.RedirectURI},
	}.Encode()
	http.Redirect(w, req, redirectURL.String(), http.StatusFound)
	return true, nil
}
