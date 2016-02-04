package login

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/server/csrf"
)

const (
	thenParam     = "then"
	csrfParam     = "csrf"
	usernameParam = "username"
	passwordParam = "password"
)

type PasswordAuthenticator interface {
	authenticator.Password
	handlers.AuthenticationSuccessHandler
}

type LoginFormRenderer interface {
	Render(form LoginForm, w http.ResponseWriter, req *http.Request)
}

type LoginForm struct {
	Action string
	Error  string
	Names  LoginFormFields
	Values LoginFormFields
}

type LoginFormFields struct {
	Then     string
	CSRF     string
	Username string
	Password string
}

type Login struct {
	csrf   csrf.CSRF
	auth   PasswordAuthenticator
	render LoginFormRenderer
}

func NewLogin(csrf csrf.CSRF, auth PasswordAuthenticator, render LoginFormRenderer) *Login {
	return &Login{
		csrf:   csrf,
		auth:   auth,
		render: render,
	}
}

// Install registers the login handler into a mux. It is expected that the
// provided prefix will serve all operations. Path MUST NOT end in a slash.
func (l *Login) Install(mux Mux, paths ...string) {
	for _, path := range paths {
		path = strings.TrimRight(path, "/")
		mux.HandleFunc(path, l.ServeHTTP)
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
		Names: LoginFormFields{
			Then:     thenParam,
			CSRF:     csrfParam,
			Username: usernameParam,
			Password: passwordParam,
		},
	}
	if then := req.URL.Query().Get("then"); then != "" {
		// TODO: sanitize 'then'
		form.Values.Then = then
	}
	switch req.URL.Query().Get("reason") {
	case "":
		break
	case "user required":
		form.Error = "Login is required. Please try again."
	case "token expired":
		form.Error = "Could not check CSRF token. Please try again."
	case "access denied":
		form.Error = "Invalid login or password. Please try again."
	default:
		form.Error = "An unknown error has occurred. Please try again."
	}

	csrf, err := l.csrf.Generate(w, req)
	if err != nil {
		util.HandleError(fmt.Errorf("unable to generate CSRF token: %v", err))
	}
	form.Values.CSRF = csrf

	l.render.Render(form, w, req)
}

func (l *Login) handleLogin(w http.ResponseWriter, req *http.Request) {
	if ok, err := l.csrf.Check(req, req.FormValue("csrf")); !ok || err != nil {
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

// NewLoginFormRenderer creates a login form renderer that takes in an optional custom template to
// allow branding of the login page. Uses the default if customLoginTemplateFile is not set.
func NewLoginFormRenderer(customLoginTemplateFile string) (*loginTemplateRenderer, error) {
	r := &loginTemplateRenderer{}
	if len(customLoginTemplateFile) > 0 {
		customTemplate, err := template.ParseFiles(customLoginTemplateFile)
		if err != nil {
			return nil, err
		}
		r.loginTemplate = customTemplate
	} else {
		r.loginTemplate = defaultLoginTemplate
	}

	return r, nil
}

func ValidateLoginTemplate(templateContent []byte) []error {
	var allErrs []error

	template, err := template.New("loginTemplateTest").Parse(string(templateContent))
	if err != nil {
		return append(allErrs, err)
	}

	// Execute the template with dummy values and check if they're there.
	form := LoginForm{
		Action: "MyAction",
		Error:  "MyError",
		Names: LoginFormFields{
			Then:     "MyThenName",
			CSRF:     "MyCSRFName",
			Username: "MyUsernameName",
			Password: "MyPasswordName",
		},
		Values: LoginFormFields{
			Then:     "MyThenValue",
			CSRF:     "MyCSRFValue",
			Username: "MyUsernameValue",
		},
	}

	var buffer bytes.Buffer
	err = template.Execute(&buffer, form)
	if err != nil {
		return append(allErrs, err)
	}
	output := buffer.Bytes()

	var testFields = map[string]string{
		"Action":          form.Action,
		"Error":           form.Error,
		"Names.Then":      form.Names.Then,
		"Names.CSRF":      form.Values.CSRF,
		"Names.Username":  form.Names.Username,
		"Names.Password":  form.Names.Password,
		"Values.Then":     form.Values.Then,
		"Values.CSRF":     form.Values.CSRF,
		"Values.Username": form.Values.Username,
	}

	for field, value := range testFields {
		if !bytes.Contains(output, []byte(value)) {
			allErrs = append(allErrs, errors.New(fmt.Sprintf("template is missing parameter {{ .%s }}", field)))
		}
	}

	return allErrs
}

type loginTemplateRenderer struct {
	loginTemplate *template.Template
}

func (r loginTemplateRenderer) Render(form LoginForm, w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	if err := r.loginTemplate.Execute(w, form); err != nil {
		util.HandleError(fmt.Errorf("unable to render login template: %v", err))
	}
}
