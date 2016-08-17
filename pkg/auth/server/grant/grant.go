package grant

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/serviceaccount"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/server/csrf"
	scopeauthorizer "github.com/openshift/origin/pkg/authorization/authorizer/scope"
	oapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclient"
	"github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization"
	"github.com/openshift/origin/pkg/oauth/scope"
)

const (
	thenParam        = "then"
	csrfParam        = "csrf"
	clientIDParam    = "client_id"
	userNameParam    = "user_name"
	scopeParam       = "scope"
	redirectURIParam = "redirect_uri"

	approveParam = "approve"
	denyParam    = "deny"
)

// FormRenderer is responsible for rendering a Form to prompt the user
// to approve or reject a requested OAuth scope grant.
type FormRenderer interface {
	Render(form Form, w http.ResponseWriter, req *http.Request)
}

type Form struct {
	Action string
	Error  string

	ServiceAccountName      string
	ServiceAccountNamespace string

	GrantedScopes interface{}

	Names  GrantFormFields
	Values GrantFormFields
}

type GrantFormFields struct {
	Then        string
	CSRF        string
	ClientID    string
	UserName    string
	Scopes      interface{}
	RedirectURI string
	Approve     string
	Deny        string
}

type Scope struct {
	// Name is the string included in the OAuth scope parameter
	Name string
	// Description is a human-readable description of the scope. May be empty.
	Description string
	// Warning is a human-readable warning about the scope. Typically used to scare the user about escalating permissions. May be empty.
	Warning string
	// Error is a human-readable error, typically around the validity of the scope. May be empty.
	Error string
	// Granted indicates whether the user has already granted this scope.
	Granted bool
}

type Grant struct {
	auth           authenticator.Request
	csrf           csrf.CSRF
	render         FormRenderer
	clientregistry oauthclient.Getter
	authregistry   oauthclientauthorization.Registry
}

func NewGrant(csrf csrf.CSRF, auth authenticator.Request, render FormRenderer, clientregistry oauthclient.Getter, authregistry oauthclientauthorization.Registry) *Grant {
	return &Grant{
		auth:           auth,
		csrf:           csrf,
		render:         render,
		clientregistry: clientregistry,
		authregistry:   authregistry,
	}
}

// Install registers the grant handler into a mux. It is expected that the
// provided prefix will serve all operations. Path MUST NOT end in a slash.
func (l *Grant) Install(mux Mux, paths ...string) {
	for _, path := range paths {
		path = strings.TrimRight(path, "/")
		mux.HandleFunc(path, l.ServeHTTP)
	}
}

func (l *Grant) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	user, ok, err := l.auth.AuthenticateRequest(req)
	if err != nil || !ok {
		l.redirect("You must reauthenticate before continuing", w, req)
		return
	}

	switch req.Method {
	case "GET":
		l.handleForm(user, w, req)
	case "POST":
		l.handleGrant(user, w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (l *Grant) handleForm(user user.Info, w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	then := q.Get(thenParam)
	clientID := q.Get(clientIDParam)
	scopes := scope.Split(q.Get(scopeParam))
	redirectURI := q.Get(redirectURIParam)

	client, err := l.clientregistry.GetClient(kapi.NewContext(), clientID)
	if err != nil || client == nil {
		l.failed("Could not find client for client_id", w, req)
		return
	}

	if err := scopeauthorizer.ValidateScopeRestrictions(client, scopes...); err != nil {
		failure := fmt.Sprintf("%v requested illegal scopes (%v): %v", client.Name, scopes, err)
		l.failed(failure, w, req)
		return
	}

	uri, err := getBaseURL(req)
	if err != nil {
		glog.Errorf("Unable to generate base URL: %v", err)
		http.Error(w, "Unable to determine URL", http.StatusInternalServerError)
		return
	}

	csrf, err := l.csrf.Generate(w, req)
	if err != nil {
		glog.Errorf("Unable to generate CSRF token: %v", err)
		l.failed("Could not generate CSRF token", w, req)
		return
	}

	grantedScopeNames := []string{}
	grantedScopes := []Scope{}
	requestedScopes := []Scope{}

	clientAuthID := l.authregistry.ClientAuthorizationName(user.GetName(), client.Name)
	if clientAuth, err := l.authregistry.GetClientAuthorization(kapi.NewContext(), clientAuthID); err == nil {
		grantedScopeNames = clientAuth.Scopes
	}

	for _, s := range scopes {
		requestedScopes = append(requestedScopes, getScopeData(s, grantedScopeNames))
	}
	for _, s := range grantedScopeNames {
		grantedScopes = append(grantedScopes, getScopeData(s, grantedScopeNames))
	}

	form := Form{
		Action:        uri.String(),
		GrantedScopes: grantedScopes,
		Names: GrantFormFields{
			Then:        thenParam,
			CSRF:        csrfParam,
			ClientID:    clientIDParam,
			UserName:    userNameParam,
			Scopes:      scopeParam,
			RedirectURI: redirectURIParam,
			Approve:     approveParam,
			Deny:        denyParam,
		},
		Values: GrantFormFields{
			Then:        then,
			CSRF:        csrf,
			ClientID:    client.Name,
			UserName:    user.GetName(),
			Scopes:      requestedScopes,
			RedirectURI: redirectURI,
		},
	}

	if saNamespace, saName, err := serviceaccount.SplitUsername(client.Name); err == nil {
		form.ServiceAccountName = saName
		form.ServiceAccountNamespace = saNamespace
	}

	l.render.Render(form, w, req)
}

func (l *Grant) handleGrant(user user.Info, w http.ResponseWriter, req *http.Request) {
	if ok, err := l.csrf.Check(req, req.FormValue(csrfParam)); !ok || err != nil {
		glog.Errorf("Unable to check CSRF token: %v", err)
		l.failed("Invalid CSRF token", w, req)
		return
	}

	req.ParseForm()
	then := req.FormValue(thenParam)
	scopes := scope.Join(req.Form[scopeParam])
	username := req.FormValue(userNameParam)

	if username != user.GetName() {
		glog.Errorf("User (%v) did not match authenticated user (%v)", username, user.GetName())
		l.failed("User did not match", w, req)
		return
	}

	if len(req.FormValue(approveParam)) == 0 || len(scopes) == 0 {
		// Redirect with an error param
		url, err := url.Parse(then)
		if len(then) == 0 || err != nil {
			l.failed("Access denied, but no redirect URL was specified", w, req)
			return
		}
		q := url.Query()
		q.Set("error", "access_denied")
		url.RawQuery = q.Encode()
		http.Redirect(w, req, url.String(), http.StatusFound)
		return
	}

	clientID := req.FormValue(clientIDParam)
	client, err := l.clientregistry.GetClient(kapi.NewContext(), clientID)
	if err != nil || client == nil {
		l.failed("Could not find client for client_id", w, req)
		return
	}
	if err := scopeauthorizer.ValidateScopeRestrictions(client, scope.Split(scopes)...); err != nil {
		failure := fmt.Sprintf("%v requested illegal scopes (%v): %v", client.Name, scopes, err)
		l.failed(failure, w, req)
		return
	}

	clientAuthID := l.authregistry.ClientAuthorizationName(user.GetName(), client.Name)

	ctx := kapi.NewContext()
	clientAuth, err := l.authregistry.GetClientAuthorization(ctx, clientAuthID)
	if err == nil && clientAuth != nil {
		// Add new scopes and update
		clientAuth.Scopes = scope.Add(clientAuth.Scopes, scope.Split(scopes))
		if _, err = l.authregistry.UpdateClientAuthorization(ctx, clientAuth); err != nil {
			glog.Errorf("Unable to update authorization: %v", err)
			l.failed("Could not update client authorization", w, req)
			return
		}
	} else {
		// Make sure client name, user name, grant scope, expiration, and redirect uri match
		clientAuth = &oapi.OAuthClientAuthorization{
			UserName:   user.GetName(),
			UserUID:    user.GetUID(),
			ClientName: client.Name,
			Scopes:     scope.Split(scopes),
		}
		clientAuth.Name = clientAuthID

		if _, err = l.authregistry.CreateClientAuthorization(ctx, clientAuth); err != nil {
			glog.Errorf("Unable to create authorization: %v", err)
			l.failed("Could not create client authorization", w, req)
			return
		}
	}

	// Redirect, overriding the scope param on the redirect with the scopes that were actually granted
	url, err := url.Parse(then)
	if len(then) == 0 || err != nil {
		l.failed("Access granted, but no redirect URL was specified", w, req)
		return
	}
	q := url.Query()
	q.Set("scope", scopes)
	url.RawQuery = q.Encode()
	http.Redirect(w, req, url.String(), http.StatusFound)
}

func (l *Grant) failed(reason string, w http.ResponseWriter, req *http.Request) {
	form := Form{
		Error: reason,
	}
	l.render.Render(form, w, req)
}
func (l *Grant) redirect(reason string, w http.ResponseWriter, req *http.Request) {
	then := req.FormValue("then")

	// TODO: validate then
	if len(then) == 0 {
		l.failed(reason, w, req)
		return
	}
	http.Redirect(w, req, then, http.StatusFound)
}

func getBaseURL(req *http.Request) (*url.URL, error) {
	uri, err := url.Parse(req.RequestURI)
	if err != nil {
		return nil, err
	}
	uri.Scheme, uri.Host, uri.RawQuery, uri.Fragment = req.URL.Scheme, req.URL.Host, "", ""
	return uri, nil
}

func getScopeData(scopeName string, grantedScopeNames []string) Scope {
	scopeData := Scope{
		Name:    scopeName,
		Error:   fmt.Sprintf("Unknown scope"),
		Granted: scope.Covers(grantedScopeNames, []string{scopeName}),
	}
	for _, evaluator := range scopeauthorizer.ScopeEvaluators {
		if !evaluator.Handles(scopeName) {
			continue
		}
		description, warning, err := evaluator.Describe(scopeName)
		scopeData.Description = description
		scopeData.Warning = warning
		if err == nil {
			scopeData.Error = ""
		} else {
			scopeData.Error = err.Error()
		}
		break
	}
	return scopeData
}

// DefaultFormRenderer displays a page prompting the user to approve an OAuth grant.
// The requesting client id, requested scopes, and redirect URI are displayed to the user.
var DefaultFormRenderer = grantTemplateRenderer{}

type grantTemplateRenderer struct{}

func (r grantTemplateRenderer) Render(form Form, w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	if err := defaultGrantTemplate.Execute(w, form); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to render grant template: %v", err))
	}
}
