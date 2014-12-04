package login

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"

	authapi "github.com/openshift/origin/pkg/auth/api"
	authauthenticator "github.com/openshift/origin/pkg/auth/authenticator"
	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/server/login"
	"github.com/openshift/origin/pkg/auth/server/session"
	"github.com/openshift/origin/pkg/cmd/auth"
)

const (
	passwordAuthenticatorKey = "passwordAuthenticator"
)

func init() {
	auth.RegisterInstantiator(newInstantiator())
}

type instantiator struct {
	ownedReturnType reflect.Type
	ownedConfigType string
}

func newInstantiator() *instantiator {
	return &instantiator{reflect.TypeOf((*oauthhandlers.RedirectAuthHandler)(nil)).Elem(), "login"}
}

func (a *instantiator) Owns(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo) bool {
	return (resultingType == a.ownedReturnType) && (elementConfigInfo.AuthElementConfigType == a.ownedConfigType)
}
func (a *instantiator) IsValid(elementConfigInfo auth.AuthElementConfigInfo, authConfigInfo auth.AuthConfigInfo) error {
	passwordAuthenticatorName, _ := elementConfigInfo.Config[passwordAuthenticatorKey]
	if len(passwordAuthenticatorName) == 0 {
		return fmt.Errorf("%v is required", passwordAuthenticatorKey)
	}
	if _, ok := authConfigInfo.PasswordAuthenticators[passwordAuthenticatorName]; !ok {
		return fmt.Errorf("PasswordAuthenticator %v was not found", passwordAuthenticatorName)
	}

	return nil
}
func (a *instantiator) Instantiate(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo, authConfig auth.AuthConfig, envInfo *auth.EnvInfo) (interface{}, error) {
	if !a.Owns(resultingType, elementConfigInfo) {
		return nil, fmt.Errorf("%v does not own %v", a, elementConfigInfo)
	}

	passwordAuthenticatorName, _ := elementConfigInfo.Config[passwordAuthenticatorKey]
	passwordAuthenticator := authConfig.PasswordAuthenticators[passwordAuthenticatorName]
	successHandler := getAuthenticationSuccessHandler(authConfig, envInfo)

	authHandler := &redirectHandler{RedirectURL: auth.OpenShiftLoginPrefix, ThenParam: "then"}

	login := login.NewLogin(auth.GetCSRF(), &callbackPasswordAuthenticator{passwordAuthenticator, successHandler}, login.DefaultLoginFormRenderer)
	login.Install(envInfo.Mux, auth.OpenShiftLoginPrefix)

	return authHandler, nil
}

func getAuthenticationSuccessHandler(authConfig auth.AuthConfig, envInfo *auth.EnvInfo) oauthhandlers.AuthenticationSuccessHandler {
	successHandlers := oauthhandlers.AuthenticationSuccessHandlers{}

	for _, currRequestAuthenticator := range authConfig.RequestAuthenticators {
		if _, ok := currRequestAuthenticator.(*session.SessionAuthenticator); ok {
			successHandlers = append(successHandlers, session.NewSessionAuthenticator(envInfo.SessionStore, "ssn"))
		}
	}

	successHandlers = append(successHandlers, redirectSuccessHandler{})

	return successHandlers
}

//
// Combines password auth, successful login callback, and "then" param redirection
//
type callbackPasswordAuthenticator struct {
	authauthenticator.Password
	oauthhandlers.AuthenticationSuccessHandler
}

// Captures the original request url as a "then" param in a redirect to a login flow
type redirectHandler struct {
	RedirectURL string
	ThenParam   string
}

// AuthenticationRedirectNeeded redirects HTTP request to authorization URL
func (auth *redirectHandler) AuthenticationRedirectNeeded(w http.ResponseWriter, req *http.Request) error {
	redirectURL, err := url.Parse(auth.RedirectURL)
	if err != nil {
		return err
	}
	if len(auth.ThenParam) != 0 {
		redirectURL.RawQuery = url.Values{
			auth.ThenParam: {req.URL.String()},
		}.Encode()
	}
	http.Redirect(w, req, redirectURL.String(), http.StatusFound)
	return nil
}

// Redirects to the then param on successful authentication
type redirectSuccessHandler struct{}

// AuthenticationSuccess informs client when authentication was successful
func (redirectSuccessHandler) AuthenticationSucceeded(user authapi.UserInfo, then string, w http.ResponseWriter, req *http.Request) (bool, error) {
	if len(then) == 0 {
		return false, fmt.Errorf("Auth succeeded, but no redirect existed - user=%#v", user)
	}

	http.Redirect(w, req, then, http.StatusFound)
	return true, nil
}
