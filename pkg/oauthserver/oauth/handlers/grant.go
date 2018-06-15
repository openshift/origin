package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/RangelReale/osin"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"

	oauthapi "github.com/openshift/api/oauth/v1"
	scopeauthorizer "github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/oauth/apis/oauth/validation"
	"github.com/openshift/origin/pkg/oauth/scope"
	"github.com/openshift/origin/pkg/oauthserver/api"
	"github.com/openshift/origin/pkg/oauthserver/osinserver"
)

// GrantCheck implements osinserver.AuthorizeHandler to ensure requested scopes have been authorized
type GrantCheck struct {
	check        GrantChecker
	handler      GrantHandler
	errorHandler GrantErrorHandler
}

// NewGrantCheck returns a new GrantCheck
func NewGrantCheck(check GrantChecker, handler GrantHandler, errorHandler GrantErrorHandler) osinserver.AuthorizeHandler {
	return &GrantCheck{check, handler, errorHandler}
}

// HandleAuthorize implements osinserver.AuthorizeHandler to ensure the requested scopes have been authorized.
// The AuthorizeRequest.Authorized field must already be set to true for the grant check to occur.
// If the requested scopes are authorized, the AuthorizeRequest is unchanged.
// If the requested scopes are not authorized, or an error occurs, AuthorizeRequest.Authorized is set to false.
// If the response is written, true is returned.
// If the response is not written, false is returned.
func (h *GrantCheck) HandleAuthorize(ar *osin.AuthorizeRequest, resp *osin.Response, w http.ResponseWriter) (bool, error) {

	// Requests must already be authorized before we will check grants
	if !ar.Authorized {
		return false, nil
	}

	// Reset request to unauthorized until we verify the grant
	ar.Authorized = false

	user, ok := ar.UserData.(user.Info)
	if !ok || user == nil {
		utilruntime.HandleError(fmt.Errorf("the provided user data is not a user.Info object: %#v", user))
		resp.SetError("server_error", "")
		return false, nil
	}

	client, ok := ar.Client.GetUserData().(*oauthapi.OAuthClient)
	if !ok || client == nil {
		utilruntime.HandleError(fmt.Errorf("the provided client is not an *api.OAuthClient object: %#v", client))
		resp.SetError("server_error", "")
		return false, nil
	}

	// Normalize the scope request, and ensure all tokens contain a scope
	scopes := scope.Split(ar.Scope)
	if len(scopes) == 0 {
		scopes = append(scopes, scopeauthorizer.UserFull)
	}
	ar.Scope = scope.Join(scopes)

	// Validate the requested scopes
	if scopeErrors := validation.ValidateScopes(scopes, nil); len(scopeErrors) > 0 {
		resp.SetError("invalid_scope", scopeErrors.ToAggregate().Error())
		return false, nil
	}

	invalidScopes := sets.NewString()
	for _, scope := range scopes {
		if err := scopeauthorizer.ValidateScopeRestrictions(client, scope); err != nil {
			invalidScopes.Insert(scope)
		}
	}
	if len(invalidScopes) > 0 {
		resp.SetError("access_denied", fmt.Sprintf("scope denied: %s", strings.Join(invalidScopes.List(), " ")))
		return false, nil
	}

	grant := &api.Grant{
		Client:      ar.Client,
		Scope:       ar.Scope,
		Expiration:  int64(ar.Expiration),
		RedirectURI: ar.RedirectUri,
	}

	// Check if the user has already authorized this grant
	authorized, err := h.check.HasAuthorizedClient(user, grant)
	if err != nil {
		utilruntime.HandleError(err)
		resp.SetError("server_error", "")
		return false, nil
	}
	if authorized {
		ar.Authorized = true
		return false, nil
	}

	// React to an unauthorized grant
	authorized, handled, err := h.handler.GrantNeeded(user, grant, w, ar.HttpRequest)
	if authorized {
		ar.Authorized = true
	}
	return handled, err
}

type emptyGrant struct{}

// NewEmptyGrant returns a no-op grant handler
func NewEmptyGrant() GrantHandler {
	return emptyGrant{}
}

// GrantNeeded implements the GrantHandler interface
func (emptyGrant) GrantNeeded(user user.Info, grant *api.Grant, w http.ResponseWriter, req *http.Request) (bool, bool, error) {
	return false, false, nil
}

type autoGrant struct {
}

// NewAutoGrant returns a grant handler that automatically approves client authorizations
func NewAutoGrant() GrantHandler {
	return &autoGrant{}
}

// GrantNeeded implements the GrantHandler interface
func (g *autoGrant) GrantNeeded(user user.Info, grant *api.Grant, w http.ResponseWriter, req *http.Request) (bool, bool, error) {
	return true, false, nil
}

type redirectGrant struct {
	subpath string
}

// NewRedirectGrant returns a grant handler that redirects to the given subpath when a grant is needed.
// The following query parameters are added to the URL:
//   then - original request URL
//   client_id - requesting client's ID
//   scopes - grant scope requested
//   redirect_uri - original authorize request redirect_uri
func NewRedirectGrant(subpath string) GrantHandler {
	return &redirectGrant{subpath}
}

// GrantNeeded implements the GrantHandler interface
func (g *redirectGrant) GrantNeeded(user user.Info, grant *api.Grant, w http.ResponseWriter, req *http.Request) (bool, bool, error) {
	_, lastSegment := path.Split(req.URL.Path)

	// We're going to descend one dir for the approve endpoint.
	// Make our "then" URL a relative backstep, so we can return to this URL (with no trailing slash) even via an auth proxy.
	// Depends on any auth proxies matching the last segment of the URL we used to get here, proxying subpaths of this URL, and passing through the complete query.
	// Example:
	//   User -> https://auth.example.com/foo/oauth/authorize?... -> https://api.example.com/oauth/authorize?...
	//        <- Location: authorize/approve?then=../authorize?...
	//   User -> https://auth.example.com/foo/oauth/authorize/approve?then=../authorize?...
	//           submits grant approval form, gets redirected to 'then' URL
	//        <- Location: ../authorize?...
	//   User -> https://auth.example.com/foo/oauth/authorize?...
	reqURL := &(*req.URL)
	reqURL.Host = ""
	reqURL.Scheme = ""
	reqURL.Path = path.Join("..", lastSegment)

	// Make our redirect URL a relative redirect to the subpath
	redirectURL := &url.URL{
		Path: path.Join(lastSegment, g.subpath),
		RawQuery: url.Values{
			"then":         {reqURL.String()},
			"client_id":    {grant.Client.GetId()},
			"scope":        {grant.Scope},
			"redirect_uri": {grant.RedirectURI},
		}.Encode(),
	}
	w.Header().Set("Location", redirectURL.String())
	w.WriteHeader(http.StatusFound)
	return false, true, nil
}

type perClientGrant struct {
	auto          GrantHandler
	prompt        GrantHandler
	deny          GrantHandler
	defaultMethod oauthapi.GrantHandlerType
}

// NewPerClientGrant returns a grant handler that determines what to do based on the grant method in the client
func NewPerClientGrant(prompt GrantHandler, defaultMethod oauthapi.GrantHandlerType) GrantHandler {
	return &perClientGrant{
		auto:          NewAutoGrant(),
		prompt:        prompt,
		deny:          NewEmptyGrant(),
		defaultMethod: defaultMethod,
	}
}

func (g *perClientGrant) GrantNeeded(user user.Info, grant *api.Grant, w http.ResponseWriter, req *http.Request) (bool, bool, error) {
	client, ok := grant.Client.GetUserData().(*oauthapi.OAuthClient)
	if !ok {
		return false, false, errors.New("unrecognized OAuth client type")
	}

	method := client.GrantMethod
	if len(method) == 0 {
		// Use the global default
		method = g.defaultMethod
	}

	switch method {
	case oauthapi.GrantHandlerAuto:
		return g.auto.GrantNeeded(user, grant, w, req)

	case oauthapi.GrantHandlerPrompt:
		return g.prompt.GrantNeeded(user, grant, w, req)

	case oauthapi.GrantHandlerDeny:
		return g.deny.GrantNeeded(user, grant, w, req)

	default:
		return false, false, fmt.Errorf("OAuth client grant method %q unrecognized", method)
	}
}
