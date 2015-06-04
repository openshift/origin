package grant

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/server/csrf"
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
	scopesParam      = "scopes"
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
	Values FormValues
}

type FormValues struct {
	Then             string
	ThenParam        string
	CSRF             string
	CSRFParam        string
	ClientID         string
	ClientIDParam    string
	UserName         string
	UserNameParam    string
	Scopes           string
	ScopesParam      string
	RedirectURI      string
	RedirectURIParam string
	ApproveParam     string
	DenyParam        string
}

type Grant struct {
	auth           authenticator.Request
	csrf           csrf.CSRF
	render         FormRenderer
	clientregistry oauthclient.Registry
	authregistry   oauthclientauthorization.Registry
}

func NewGrant(csrf csrf.CSRF, auth authenticator.Request, render FormRenderer, clientregistry oauthclient.Registry, authregistry oauthclientauthorization.Registry) *Grant {
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
	then := q.Get("then")
	clientID := q.Get("client_id")
	scopes := q.Get("scopes")
	redirectURI := q.Get("redirect_uri")

	client, err := l.clientregistry.GetClient(kapi.NewContext(), clientID)
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

	form := Form{
		Action: uri.String(),
		Values: FormValues{
			Then:             then,
			ThenParam:        thenParam,
			CSRF:             csrf,
			CSRFParam:        csrfParam,
			ClientID:         client.Name,
			ClientIDParam:    clientIDParam,
			UserName:         user.GetName(),
			UserNameParam:    userNameParam,
			Scopes:           scopes,
			ScopesParam:      scopesParam,
			RedirectURI:      redirectURI,
			RedirectURIParam: redirectURIParam,
			ApproveParam:     approveParam,
			DenyParam:        denyParam,
		},
	}

	l.render.Render(form, w, req)
}

func (l *Grant) handleGrant(user user.Info, w http.ResponseWriter, req *http.Request) {
	if ok, err := l.csrf.Check(req, req.FormValue("csrf")); !ok || err != nil {
		glog.Errorf("Unable to check CSRF token: %v", err)
		l.failed("Invalid CSRF token", w, req)
		return
	}

	then := req.FormValue("then")
	scopes := req.FormValue("scopes")

	if len(req.FormValue(approveParam)) == 0 {
		// Redirect with rejection param
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

	clientID := req.FormValue("client_id")
	client, err := l.clientregistry.GetClient(kapi.NewContext(), clientID)
	if err != nil || client == nil {
		l.failed("Could not find client for client_id", w, req)
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

	if len(then) == 0 {
		l.failed("Approval granted, but no redirect URL was specified", w, req)
		return
	}

	http.Redirect(w, req, then, http.StatusFound)
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

// DefaultFormRenderer displays a page prompting the user to approve an OAuth grant.
// The requesting client id, requested scopes, and redirect URI are displayed to the user.
var DefaultFormRenderer = grantTemplateRenderer{}

type grantTemplateRenderer struct{}

func (r grantTemplateRenderer) Render(form Form, w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	if err := grantTemplate.Execute(w, form); err != nil {
		util.HandleError(fmt.Errorf("unable to render grant template: %v", err))
	}
}

// TODO: allow template to be read from an external file
var grantTemplate = template.Must(template.New("grantForm").Parse(`
<style>
	body    { font-family: sans-serif; font-size: 12pt; margin: 2em 5%; background-color: #F9F9F9; }
	pre     { padding-left: 1em; border-left: .25em solid #eee; }
	a       { color: #00f; text-decoration: none; }
	a:hover { text-decoration: underline; }
</style>
{{ if .Error }}
<div class="message">{{ .Error }}</div>
{{ else }}
<form action="{{ .Action }}" method="POST">
  <input type="hidden" name="{{ .Values.ThenParam }}" value="{{ .Values.Then }}">
  <input type="hidden" name="{{ .Values.CSRFParam }}" value="{{ .Values.CSRF }}">
  <input type="hidden" name="{{ .Values.ClientIDParam }}" value="{{ .Values.ClientID }}">
  <input type="hidden" name="{{ .Values.UserNameParam }}" value="{{ .Values.UserName }}">
  <input type="hidden" name="{{ .Values.ScopesParam }}" value="{{ .Values.Scopes }}">
  <input type="hidden" name="{{ .Values.RedirectURIParam }}" value="{{ .Values.RedirectURI }}">

<h3>Approve Client?</h3>
<p>Do you approve granting an access token to the following OAuth client?</p>
<pre>
Client: {{ .Values.ClientID }}
Scope:  {{ .Values.Scopes }}
URI:    {{ .Values.RedirectURI }}
</pre>
  
  <input type="submit" name="{{ .Values.ApproveParam }}" value="Approve">
  <input type="submit" name="{{ .Values.DenyParam }}" value="Reject">
</form>
{{ end }}
`))
