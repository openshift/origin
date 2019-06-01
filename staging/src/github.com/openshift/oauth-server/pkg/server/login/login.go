package login

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"k8s.io/klog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/authentication/authenticator"

	"github.com/openshift/oauth-server/pkg"
	"github.com/openshift/oauth-server/pkg/oauth/handlers"
	"github.com/openshift/oauth-server/pkg/prometheus"
	"github.com/openshift/oauth-server/pkg/server/csrf"
	"github.com/openshift/oauth-server/pkg/server/errorpage"
	"github.com/openshift/oauth-server/pkg/server/redirect"
)

const (
	thenParam     = "then"
	csrfParam     = "csrf"
	usernameParam = "username"
	passwordParam = "password"
	reasonParam   = "reason"

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

func (l *Login) Install(mux oauthserver.Mux, prefix string) {
	mux.Handle(prefix, l)
}

func (l *Login) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		l.handleLoginForm(w, req)
	case http.MethodPost:
		l.handleLogin(w, req)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
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
	if then := req.URL.Query().Get(thenParam); redirect.IsServerRelativeURL(then) {
		form.Values.Then = then
	} else {
		http.Redirect(w, req, "/", http.StatusFound)
		return
	}

	form.ErrorCode = req.URL.Query().Get(reasonParam)
	if len(form.ErrorCode) > 0 {
		if msg, hasMsg := errorMessages[form.ErrorCode]; hasMsg {
			form.Error = msg
		} else {
			form.Error = errorpage.AuthenticationErrorMessage(form.ErrorCode)
		}
	}

	form.Values.CSRF = l.csrf.Generate(w, req)

	l.render.Render(form, w, req)
}

func (l *Login) handleLogin(w http.ResponseWriter, req *http.Request) {
	if ok := l.csrf.Check(req, req.FormValue(csrfParam)); !ok {
		klog.V(4).Infof("Invalid CSRF token: %s", req.FormValue(csrfParam))
		failed(errorCodeTokenExpired, w, req)
		return
	}
	then := req.FormValue(thenParam)
	if !redirect.IsServerRelativeURL(then) {
		http.Redirect(w, req, "/", http.StatusFound)
		return
	}
	username, password := req.FormValue(usernameParam), req.FormValue(passwordParam)
	if len(username) == 0 {
		failed(errorCodeUserRequired, w, req)
		return
	}
	if len(password) == 0 {
		failed(errorCodeAccessDenied, w, req)
		return
	}
	result := metrics.SuccessResult
	defer func() {
		metrics.RecordFormPasswordAuth(result)
	}()
	authResponse, ok, err := l.auth.AuthenticatePassword(context.TODO(), username, password)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf(`Error authenticating %q with provider %q: %v`, username, l.provider, err))
		failed(errorpage.AuthenticationErrorCode(err), w, req)
		result = metrics.ErrorResult
		return
	}
	if !ok {
		klog.V(4).Infof(`Login with provider %q failed for %q`, l.provider, username)
		failed(errorCodeAccessDenied, w, req)
		result = metrics.FailResult
		return
	}
	klog.V(4).Infof(`Login with provider %q succeeded for %q: %#v`, l.provider, username, authResponse.User)
	l.auth.AuthenticationSucceeded(authResponse.User, then, w, req)
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
