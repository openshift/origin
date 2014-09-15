package login

import (
	"html/template"
	"net/http"

	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/authenticator"
)

type PasswordAuthenticator interface {
	authenticator.Password
	AuthenticationSucceeded(context api.UserInfo, then string, w http.ResponseWriter, req *http.Request)
}

type LoginFormRenderer interface {
	Render(form LoginForm, w http.ResponseWriter, req *http.Request)
}

type LoginForm struct {
	Action string
	Error  string
	Values LoginFormValues
}

type LoginFormValues struct {
	Then     string
	CSRF     string
	Username string
	Password string
}

type Login struct {
	csrf   CSRF
	auth   PasswordAuthenticator
	render LoginFormRenderer
}

func NewLogin(csrf CSRF, auth PasswordAuthenticator, render LoginFormRenderer) *Login {
	return &Login{
		csrf:   csrf,
		auth:   auth,
		render: render,
	}
}

func (l *Login) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		l.handleLoginForm(w, req)
	case "POST":
		l.handleLogin(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (l *Login) handleLoginForm(w http.ResponseWriter, req *http.Request) {
	uri, err := getBaseURL(req)
	if err != nil {
		glog.Errorf("Unable to generate base URL: %v", err)
		http.Error(w, "Unable to determine URL", http.StatusInternalServerError)
		return
	}

	form := LoginForm{
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
		form.Error = "An unknown error has occured. Please try again."
	}

	csrf, err := l.csrf.Generate()
	if err != nil {
		glog.Errorf("Unable to generate CSRF token: %v", err)
	}
	form.Values.CSRF = csrf

	l.render.Render(form, w, req)
}

func (l *Login) handleLogin(w http.ResponseWriter, req *http.Request) {
	if ok, err := l.csrf.Check(req.FormValue("csrf")); !ok || err != nil {
		glog.Errorf("Unable to check CSRF token: %v", err)
		failed("token expired", w, req)
		return
	}
	then := req.FormValue("then")
	user, password := req.FormValue("username"), req.FormValue("password")
	if user == "" {
		failed("user required", w, req)
		return
	}
	context, ok, err := l.auth.AuthenticatePassword(user, password)
	if err != nil {
		glog.Errorf("Unable to authenticate password: %v", err)
		failed("unknown error", w, req)
		return
	}
	if !ok {
		failed("access denied", w, req)
		return
	}
	l.auth.AuthenticationSucceeded(context, then, w, req)
}

var DefaultLoginFormRenderer = loginTemplateRenderer{}

type loginTemplateRenderer struct{}

func (r loginTemplateRenderer) Render(form LoginForm, w http.ResponseWriter, req *http.Request) {
	if err := loginTemplate.Execute(w, form); err != nil {
		glog.Errorf("Unable to render login template: %v", err)
	}
}

var loginTemplate = template.Must(template.New("loginForm").Parse(`
{{ if .Error }}<div class="message">{{ .Error }}</div>{{ end }}
<form action="{{ .Action }}" method="POST">
  <input type="hidden" name="then" value="{{ .Values.Then }}">
  <input type="hidden" name="csrf" value="{{ .Values.CSRF }}">
  <label>Login: <input type="text" name="username" value="{{ .Values.Username }}"></label>
  <label>Password: <input type="password" name="password" value=""></label>
  <input type="submit" name="Login">
</form>
`))
