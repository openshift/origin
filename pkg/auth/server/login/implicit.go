package login

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/server/csrf"
)

type RequestAuthenticator interface {
	authenticator.Request
	handlers.AuthenticationSuccessHandler
}

type ConfirmFormRenderer interface {
	Render(form ConfirmForm, w http.ResponseWriter, req *http.Request)
}

type ConfirmForm struct {
	Action string
	Error  string
	User   user.Info
	Values ConfirmFormValues
}

type ConfirmFormValues struct {
	Then string
	CSRF string
}

type Confirm struct {
	csrf   csrf.CSRF
	auth   RequestAuthenticator
	render ConfirmFormRenderer
}

func NewConfirm(csrf csrf.CSRF, auth RequestAuthenticator, render ConfirmFormRenderer) *Confirm {
	return &Confirm{
		csrf:   csrf,
		auth:   auth,
		render: render,
	}
}

func (c *Confirm) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		c.handleConfirmForm(w, req)
	case "POST":
		c.handleConfirm(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (c *Confirm) handleConfirmForm(w http.ResponseWriter, req *http.Request) {
	uri, err := getBaseURL(req)
	if err != nil {
		glog.Errorf("Unable to generate base URL: %v", err)
		http.Error(w, "Unable to determine URL", http.StatusInternalServerError)
		return
	}

	form := ConfirmForm{
		Action: uri.String(),
	}
	if then := req.URL.Query().Get("then"); then != "" {
		// TODO: sanitize 'then'
		form.Values.Then = then
	}
	switch req.URL.Query().Get("reason") {
	case "":
		break
	default:
		form.Error = "An unknown error has occurred. Please try again."
	}

	csrf, err := c.csrf.Generate(w, req)
	if err != nil {
		util.HandleError(fmt.Errorf("unable to generate CSRF token: %v", err))
	}
	form.Values.CSRF = csrf

	user, ok, err := c.auth.AuthenticateRequest(req)
	if err != nil || !ok {
		glog.Errorf("Unable to authenticate request: %v", err)
		form.Error = "An unknown error has occurred. Contact your administrator."
		c.render.Render(form, w, req)
		return
	}

	form.User = user

	c.render.Render(form, w, req)
}

func (c *Confirm) handleConfirm(w http.ResponseWriter, req *http.Request) {
	if ok, err := c.csrf.Check(req, req.FormValue("csrf")); !ok || err != nil {
		glog.Errorf("Unable to check CSRF token: %v", err)
		failed("token expired", w, req)
		return
	}

	user, ok, err := c.auth.AuthenticateRequest(req)
	if err != nil || !ok {
		if err != nil {
			glog.Errorf("Unable authenticate request: %v", err)
		}
		failed("access denied", w, req)
		return
	}

	then := req.FormValue("then")

	c.auth.AuthenticationSucceeded(user, then, w, req)
}

var DefaultConfirmFormRenderer = confirmTemplateRenderer{}

type confirmTemplateRenderer struct{}

func (r confirmTemplateRenderer) Render(form ConfirmForm, w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	if err := confirmTemplate.Execute(w, form); err != nil {
		util.HandleError(fmt.Errorf("unable render confirm template: %v", err))
	}
}

var confirmTemplate = template.Must(template.New("ConfirmForm").Parse(`
{{ if .Error }}<div class="message">{{ .Error }}</div>{{ end }}
<form action="{{ .Action }}" method="POST">
  <input type="hidden" name="then" value="{{ .Values.Then }}">
  <input type="hidden" name="csrf" value="{{ .Values.CSRF }}">
  {{ if .User }}
  <p>You are now logged in as <strong>{{ .User.GetName }}</strong></p>
  <input type="submit" name="Continue">
  {{ else }}
  <p>You are not currently logged in.</p>
  <input type="submit" disabled="disabled" name="Continue">
  {{ end }}
</form>
`))
