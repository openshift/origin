package tokenrequest

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path"

	"github.com/RangelReale/osincli"

	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/auth/server/login"
)

const (
	RequestTokenEndpoint  = "/token/request"
	DisplayTokenEndpoint  = "/token/display"
	ImplicitTokenEndpoint = "/token/implicit"
)

type endpointDetails struct {
	publicMasterURL   string
	originOAuthClient *osincli.Client
}

type Endpoints interface {
	Install(mux login.Mux, paths ...string)
}

func NewEndpoints(publicMasterURL string, originOAuthClient *osincli.Client) Endpoints {
	return &endpointDetails{publicMasterURL, originOAuthClient}
}

// Install registers the request token endpoints into a mux. It is expected that the
// provided prefix will serve all operations
func (endpoints *endpointDetails) Install(mux login.Mux, paths ...string) {
	for _, prefix := range paths {
		mux.HandleFunc(path.Join(prefix, RequestTokenEndpoint), endpoints.requestToken)
		mux.HandleFunc(path.Join(prefix, DisplayTokenEndpoint), endpoints.displayToken)
		mux.HandleFunc(path.Join(prefix, ImplicitTokenEndpoint), endpoints.implicitToken)
	}
}

// requestToken works for getting a token in your browser and seeing what your token is
func (endpoints *endpointDetails) requestToken(w http.ResponseWriter, req *http.Request) {
	authReq := endpoints.originOAuthClient.NewAuthorizeRequest(osincli.CODE)
	oauthURL := authReq.GetAuthorizeUrlWithParams("")

	http.Redirect(w, req, oauthURL.String(), http.StatusFound)
}

func (endpoints *endpointDetails) displayToken(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	data := tokenData{RequestURL: "request", PublicMasterURL: endpoints.publicMasterURL}

	authorizeReq := endpoints.originOAuthClient.NewAuthorizeRequest(osincli.CODE)
	authorizeData, err := authorizeReq.HandleRequest(req)
	if err != nil {
		data.Error = fmt.Sprintf("Error handling auth request: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		renderToken(w, data)
		return
	}

	accessReq := endpoints.originOAuthClient.NewAccessRequest(osincli.AUTHORIZATION_CODE, authorizeData)
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

func renderToken(w io.Writer, data tokenData) {
	if err := tokenTemplate.Execute(w, data); err != nil {
		util.HandleError(fmt.Errorf("unable to render token template: %v", err))
	}
}

type tokenData struct {
	Error           string
	AccessToken     string
	RequestURL      string
	PublicMasterURL string
}

// TODO: allow template to be read from an external file
var tokenTemplate = template.Must(template.New("tokenTemplate").Parse(`
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

func (endpoints *endpointDetails) implicitToken(w http.ResponseWriter, req *http.Request) {
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
