package tokenrequest

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/RangelReale/osincli"
	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/openshift/origin/pkg/oauth/urls"
	"github.com/openshift/origin/pkg/oauthserver"
	"github.com/openshift/origin/pkg/oauthserver/server/csrf"
)

const csrfParam = "csrf"

type tokenRequest struct {
	publicMasterURL string
	// osinOAuthClientGetter is used to initialize osinOAuthClient.
	// Since it can return an error, it may be called multiple times.
	osinOAuthClientGetter func() (*osincli.Client, error)

	csrf csrf.CSRF
}

func NewTokenRequest(publicMasterURL string, osinOAuthClientGetter func() (*osincli.Client, error), csrf csrf.CSRF) oauthserver.Endpoints {
	return &tokenRequest{
		publicMasterURL:       publicMasterURL,
		osinOAuthClientGetter: osinOAuthClientGetter,
		csrf: csrf,
	}
}

func (t *tokenRequest) Install(mux oauthserver.Mux, prefix string) {
	mux.HandleFunc(path.Join(prefix, urls.RequestTokenEndpoint), t.oauthClientHandler(t.requestToken))
	mux.HandleFunc(path.Join(prefix, urls.DisplayTokenEndpoint), t.oauthClientHandler(t.displayToken))
	mux.HandleFunc(path.Join(prefix, urls.ImplicitTokenEndpoint), t.implicitToken)
}

func (t *tokenRequest) oauthClientHandler(delegate func(*osincli.Client, http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, h *http.Request) {
		osinOAuthClient, err := t.osinOAuthClientGetter()
		if err != nil {
			utilruntime.HandleError(fmt.Errorf("failed to get Osin OAuth client for token endpoint: %v", err))
			http.Error(w, "OAuth token endpoint is not ready", http.StatusInternalServerError)
			return
		}
		delegate(osinOAuthClient, w, h)
	}
}

// requestToken works for getting a token in your browser and seeing what your token is
func (t *tokenRequest) requestToken(osinOAuthClient *osincli.Client, w http.ResponseWriter, req *http.Request) {
	authReq := osinOAuthClient.NewAuthorizeRequest(osincli.CODE)
	oauthURL := authReq.GetAuthorizeUrl()

	http.Redirect(w, req, oauthURL.String(), http.StatusFound)
}

func (t *tokenRequest) displayToken(osinOAuthClient *osincli.Client, w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		t.displayTokenGet(osinOAuthClient, w, req)
	case http.MethodPost:
		t.displayTokenPost(osinOAuthClient, w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (t *tokenRequest) displayTokenGet(osinOAuthClient *osincli.Client, w http.ResponseWriter, req *http.Request) {
	data := formData{}
	authorizeData, ok := displayTokenStart(osinOAuthClient, w, req, &data.sharedData)
	if !ok {
		renderForm(w, data)
		return
	}

	uri, err := getBaseURL(req)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to generate base URL: %v", err))
		http.Error(w, "Unable to determine URL", http.StatusInternalServerError)
		return
	}

	data.Action = uri.String()
	data.Code = authorizeData.Code
	data.CSRF = t.csrf.Generate(w, req)
	renderForm(w, data)
}

func (t *tokenRequest) displayTokenPost(osinOAuthClient *osincli.Client, w http.ResponseWriter, req *http.Request) {
	if ok := t.csrf.Check(req, req.FormValue(csrfParam)); !ok {
		glog.V(4).Infof("Invalid CSRF token: %s", req.FormValue(csrfParam))
		http.Error(w, "Could not check CSRF token. Please try again.", http.StatusBadRequest)
		return
	}

	data := tokenData{PublicMasterURL: t.publicMasterURL}
	authorizeData, ok := displayTokenStart(osinOAuthClient, w, req, &data.sharedData)
	if !ok {
		renderToken(w, data)
		return
	}

	accessReq := osinOAuthClient.NewAccessRequest(osincli.AUTHORIZATION_CODE, authorizeData)
	accessData, err := accessReq.GetToken()
	if err != nil {
		data.Error = fmt.Sprintf("Error getting token: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		renderToken(w, data)
		return
	}

	data.AccessToken = accessData.AccessToken
	renderToken(w, data)
}

func displayTokenStart(osinOAuthClient *osincli.Client, w http.ResponseWriter, req *http.Request, data *sharedData) (*osincli.AuthorizeData, bool) {
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	requestURL := urls.OpenShiftOAuthTokenRequestURL("") // relative url to token request endpoint
	data.RequestURL = requestURL                         // always set this field even on error cases

	authorizeReq := osinOAuthClient.NewAuthorizeRequest(osincli.CODE)
	authorizeData, err := authorizeReq.HandleRequest(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		data.Error = fmt.Sprintf("Error handling auth request: %v", err)
		return nil, false
	}

	return authorizeData, true
}

func renderToken(w io.Writer, data tokenData) {
	if err := tokenTemplate.Execute(w, data); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to render token template: %v", err))
	}
}

type sharedData struct {
	Error      string
	RequestURL string
}

type tokenData struct {
	sharedData

	AccessToken     string
	PublicMasterURL string
}

func getBaseURL(req *http.Request) (*url.URL, error) {
	uri, err := url.Parse(req.RequestURI)
	if err != nil {
		return nil, err
	}
	uri.Scheme, uri.Host, uri.RawQuery, uri.Fragment = req.URL.Scheme, req.URL.Host, "", ""
	return uri, nil
}

type formData struct {
	sharedData

	Action string
	Code   string
	CSRF   string
}

func renderForm(w io.Writer, data formData) {
	if err := formTemplate.Execute(w, data); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to render form template: %v", err))
	}
}

const cssStyle = `
<style>
	body     { font-family: sans-serif; font-size: 14px; margin: 2em 2%; background-color: #F9F9F9; }
	h2       { font-size: 1.4em;}
	h3       { font-size: 1em; margin: 1.5em 0 0; }
	code,pre { font-family: Menlo, Monaco, Consolas, monospace; }
	code     { font-weight: 300; font-size: 1.5em; margin-bottom: 1em; display: inline-block;  color: #646464;  }
	pre      { padding-left: 1em; border-radius: 5px; color: #003d6e; background-color: #EAEDF0; padding: 1.5em 0 1.5em 4.5em; white-space: normal; text-indent: -2em; }
	a        { color: #00f; text-decoration: none; }
	a:hover  { text-decoration: underline; }
	@media (min-width: 768px) {
		.nowrap { white-space: nowrap; }
	}
</style>
`

var tokenTemplate = template.Must(template.New("tokenTemplate").Parse(
	cssStyle + `
{{ if .Error }}
  {{ .Error }}
{{ else }}
  <h2>Your API token is</h2>
  <code>{{.AccessToken}}</code>

  <h2>Log in with this token</h2>
  <pre>oc login <span class="nowrap">--token={{.AccessToken}}</span> <span class="nowrap">--server={{.PublicMasterURL}}</span></pre>

  <h3>Use this token directly against the API</h3>
  <pre>curl <span class="nowrap">-H "Authorization: Bearer {{.AccessToken}}"</span> <span class="nowrap">"{{.PublicMasterURL}}/oapi/v1/users/~"</span></pre>
{{ end }}

<br><br>
<a href="{{.RequestURL}}">Request another token</a>
`))

var formTemplate = template.Must(template.New("formTemplate").Parse(
	cssStyle + `
{{ if .Error }}
  {{ .Error }}
  <br><br>
  <a href="{{.RequestURL}}">Request another token</a>
{{ else }}
  <form method="post" action="{{.Action}}">
    <input type="hidden" name="code" value="{{.Code}}">
    <input type="hidden" name="csrf" value="{{.CSRF}}">
    <button type="submit">
      Display Token
    </button>
  </form>
{{ end }}
`))

func (t *tokenRequest) implicitToken(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(`
You have reached this page by following a redirect Location header from an OAuth authorize request.

If a response_type=token parameter was passed to the /authorize endpoint, that requested an
"Implicit Grant" OAuth flow (see https://tools.ietf.org/html/rfc6749#section-4.2).

That flow requires the access token to be returned in the fragment portion of a redirect header.
Rather than following the redirect here, you can obtain the access token from the Location header
(see https://tools.ietf.org/html/rfc6749#section-4.2.2):

  1. Parse the URL in the Location header and extract the fragment portion
  2. Parse the fragment using the "application/x-www-form-urlencoded" format
  3. The access_token parameter contains the granted OAuth access token
`))
}
