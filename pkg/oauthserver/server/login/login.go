package login

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/authenticator"

	"github.com/openshift/origin/pkg/oauthserver/oauth/handlers"
	"github.com/openshift/origin/pkg/oauthserver/prometheus"
	"github.com/openshift/origin/pkg/oauthserver/server/csrf"
	"github.com/openshift/origin/pkg/oauthserver/server/errorpage"
	"github.com/openshift/origin/pkg/oauthserver/server/headers"
)

const (
	thenParam     = "then"
	csrfParam     = "csrf"
	usernameParam = "username"
	passwordParam = "password"

	// these can be used by custom templates, and should not be changed
	// these error codes are specific to the login flow.
	// general authentication error codes are found in the errorpage package
	errorCodeUserRequired = "user_required"
	errorCodeTokenExpired = "token_expired"
	errorCodeAccessDenied = "access_denied"
)

// Error messages that correlate to the error codes above.
// General authentication error messages are found in the error page package
var errorMessages = map[string]string{
	errorCodeUserRequired: "Login is required. Please try again.",
	errorCodeTokenExpired: "Could not check CSRF token. Please try again.",
	errorCodeAccessDenied: "Invalid login or password. Please try again.",
}

type PasswordAuthenticator interface {
	authenticator.Password
	handlers.AuthenticationSuccessHandler
}

type LoginFormRenderer interface {
	Render(form LoginForm, w http.ResponseWriter, req *http.Request)
}

type LoginForm struct {
	ProviderName string

	Action string

	Error     string
	ErrorCode string

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
	provider string
	csrf     csrf.CSRF
	auth     PasswordAuthenticator
	render   LoginFormRenderer
}

func NewLogin(provider string, csrf csrf.CSRF, auth PasswordAuthenticator, render LoginFormRenderer) *Login {
	return &Login{
		provider: provider,
		csrf:     csrf,
		auth:     auth,
		render:   render,
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
	headers.SetStandardHeaders(w)
	switch req.Method {
	case "GET":
		l.handleLoginForm(w, req)
	case "POST":
		l.handleLogin(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func isServerRelativeURL(then string) bool {
	if len(then) == 0 {
		return false
	}
	u, err := url.Parse(then)
	if err != nil {
		return false
	}
	return len(u.Scheme) == 0 && len(u.Host) == 0 && strings.HasPrefix(u.Path, "/")
}

func (l *Login) handleLoginForm(w http.ResponseWriter, req *http.Request) {
	uri, err := getBaseURL(req)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to generate base URL: %v", err))
		http.Error(w, "Unable to determine URL", http.StatusInternalServerError)
		return
	}

	form := LoginForm{
		ProviderName: l.provider,
		Action:       uri.String(),
		Names: LoginFormFields{
			Then:     thenParam,
			CSRF:     csrfParam,
			Username: usernameParam,
			Password: passwordParam,
		},
	}
	if then := req.URL.Query().Get("then"); isServerRelativeURL(then) {
		form.Values.Then = then
	} else {
		http.Redirect(w, req, "/", http.StatusFound)
		return
	}

	form.ErrorCode = req.URL.Query().Get("reason")
	if len(form.ErrorCode) > 0 {
		if msg, hasMsg := errorMessages[form.ErrorCode]; hasMsg {
			form.Error = msg
		} else {
			form.Error = errorpage.AuthenticationErrorMessage(form.ErrorCode)
		}
	}

	csrf, err := l.csrf.Generate(w, req)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to generate CSRF token: %v", err))
	}
	form.Values.CSRF = csrf

	l.render.Render(form, w, req)
}

func (l *Login) handleLogin(w http.ResponseWriter, req *http.Request) {
	if ok, err := l.csrf.Check(req, req.FormValue("csrf")); !ok || err != nil {
		utilruntime.HandleError(fmt.Errorf("Unable to check CSRF token: %v", err))
		failed(errorCodeTokenExpired, w, req)
		return
	}
	then := req.FormValue("then")
	if !isServerRelativeURL(then) {
		http.Redirect(w, req, "/", http.StatusFound)
		return
	}
	username, password := req.FormValue("username"), req.FormValue("password")
	if username == "" {
		failed(errorCodeUserRequired, w, req)
		return
	}
	var result string = metrics.SuccessResult
	defer func() {
		metrics.RecordFormPasswordAuth(result)
	}()
	user, ok, err := l.auth.AuthenticatePassword(username, password)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf(`Error authenticating %q with provider %q: %v`, username, l.provider, err))
		failed(errorpage.AuthenticationErrorCode(err), w, req)
		result = metrics.ErrorResult
		return
	}
	if !ok {
		glog.V(4).Infof(`Login with provider %q failed for %q`, l.provider, username)
		failed(errorCodeAccessDenied, w, req)
		result = metrics.FailResult
		return
	}
	glog.V(4).Infof(`Login with provider %q succeeded for %q: %#v`, l.provider, username, user)
	l.auth.AuthenticationSucceeded(user, then, w, req)
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
	w.Header().Add("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := r.loginTemplate.Execute(w, form); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to render login template: %v", err))
	}
}
