package grant

import (
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"
	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/server/csrf"
	oapi "github.com/openshift/origin/pkg/oauth/api"
	clientregistry "github.com/openshift/origin/pkg/oauth/registry/client"
	"github.com/openshift/origin/pkg/oauth/registry/clientauthorization"
	"github.com/openshift/origin/pkg/oauth/scope"
)

const (
	thenParam        = "then"
	csrfParam        = "csrf"
	clientIDParam    = "client_id"
	userNameParam    = "user_name"
	scopesParam      = "scopes"
	redirectURIParam = "redirect_uri"
)

// GrantFormRenderer is responsible for rendering a GrantForm to prompt the user
// to approve or reject a requested OAuth scope grant.
type GrantFormRenderer interface {
	Render(form GrantForm, w http.ResponseWriter, req *http.Request)
}

type GrantForm struct {
	Action string
	Error  string
	Values GrantFormValues
}

type GrantFormValues struct {
	Then        string
	CSRF        string
	ClientID    string
	UserName    string
	Scopes      string
	RedirectURI string
}

type Grant struct {
	auth           authenticator.Request
	csrf           csrf.CSRF
	render         GrantFormRenderer
	clientregistry clientregistry.Registry
	authregistry   clientauthorization.Registry
}

func NewGrant(csrf csrf.CSRF, auth authenticator.Request, render GrantFormRenderer, clientregistry clientregistry.Registry, authregistry clientauthorization.Registry) *Grant {
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
		l.handleGrantForm(user, w, req)
	case "POST":
		l.handleGrant(user, w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (l *Grant) handleGrantForm(user authapi.UserInfo, w http.ResponseWriter, req *http.Request) {
	q := req.URL.Query()
	then := q.Get("then")
	clientID := q.Get("client_id")
	scopes := q.Get("scopes")
	redirectURI := q.Get("redirect_uri")

	client, err := l.clientregistry.GetClient(clientID)
	if err != nil || client == nil {
		l.failed("Could not find client for client_id", w, req)
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

	form := GrantForm{
		Action: uri.String(),
		Values: GrantFormValues{
			Then:        then,
			CSRF:        csrf,
			ClientID:    client.Name,
			UserName:    user.GetName(),
			Scopes:      scopes,
			RedirectURI: redirectURI,
		},
	}

	l.render.Render(form, w, req)
}

func (l *Grant) handleGrant(user authapi.UserInfo, w http.ResponseWriter, req *http.Request) {
	if ok, err := l.csrf.Check(req, req.FormValue("csrf")); !ok || err != nil {
		glog.Errorf("Unable to check CSRF token: %v", err)
		l.failed("Invalid CSRF token", w, req)
		return
	}

	then := req.FormValue("then")
	clientID := req.FormValue("client_id")
	scopes := req.FormValue("scopes")

	client, err := l.clientregistry.GetClient(clientID)
	if err != nil || client == nil {
		l.failed("Could not find client for client_id", w, req)
		return
	}

	clientAuthID := l.authregistry.ClientAuthorizationID(user.GetName(), client.Name)

	clientAuth, err := l.authregistry.GetClientAuthorization(clientAuthID)
	if err == nil && clientAuth != nil {
		// Add new scopes and update
		clientAuth.Scopes = scope.Add(clientAuth.Scopes, scope.Split(scopes))
		if err = l.authregistry.UpdateClientAuthorization(clientAuth); err != nil {
			glog.Errorf("Unable to update authorization: %v", err)
			l.failed("Could not update client authorization", w, req)
			return
		}
	} else {
		// Make sure client name, user name, grant scope, expiration, and redirect uri match
		clientAuth = &oapi.ClientAuthorization{
			UserName:   user.GetName(),
			UserUID:    user.GetUID(),
			ClientName: client.Name,
			Scopes:     scope.Split(scopes),
		}
		clientAuth.Name = clientAuthID

		if err = l.authregistry.CreateClientAuthorization(clientAuth); err != nil {
			glog.Errorf("Unable to create authorization: %v", err)
			l.failed("Could not create client authorization", w, req)
			return
		}
	}

	if len(then) == 0 {
		l.failed("Approval granted, but no redirect URL was specified", w, req)
		return
	}

	http.Redirect(w, req, then, http.StatusFound)
}

func (l *Grant) failed(reason string, w http.ResponseWriter, req *http.Request) {
	form := GrantForm{
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

// DefaultGrantFormRenderer displays a page prompting the user to approve an OAuth grant.
// The requesting client id, requested scopes, and redirect URI are displayed to the user.
var DefaultGrantFormRenderer = grantTemplateRenderer{}

type grantTemplateRenderer struct{}

func (r grantTemplateRenderer) Render(form GrantForm, w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	if err := grantTemplate.Execute(w, form); err != nil {
		glog.Errorf("Unable to render grant template: %v", err)
	}
}

var grantTemplate = template.Must(template.New("grantForm").Parse(`
{{ if .Error }}
<div class="message">{{ .Error }}</div>
{{ else }}
<form action="{{ .Action }}" method="POST">
  <input type="hidden" name="then" value="{{ .Values.Then }}">
  <input type="hidden" name="csrf" value="{{ .Values.CSRF }}">
  <input type="hidden" name="client_id" value="{{ .Values.ClientID }}">
  <input type="hidden" name="user_name" value="{{ .Values.UserName }}">
  <input type="hidden" name="scopes" value="{{ .Values.Scopes }}">
  <input type="hidden" name="redirect_uri" value="{{ .Values.RedirectURI }}">

  <div>Do you approve this client?</div>
  <div>Client:     {{ .Values.ClientID }}</div>
  <div>Scope:      {{ .Values.Scopes }}</div>
  <div>URI:        {{ .Values.RedirectURI }}</div>
  
  <input type="submit" value="Approve">
</form>
{{ end }}
`))
