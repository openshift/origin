package oauth

import (
	"fmt"
	"reflect"

	"github.com/openshift/origin/pkg/auth/oauth/external"
	"github.com/openshift/origin/pkg/auth/oauth/external/github"
	"github.com/openshift/origin/pkg/auth/oauth/external/google"
	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
	"github.com/openshift/origin/pkg/auth/server/session"
	"github.com/openshift/origin/pkg/cmd/auth"
)

const (
	identityMapperKey = "identityMapper"
	clientIdKey       = "clientId"
	clientSecretKey   = "clientSecret"
	flavorKey         = "flavor"
)

func init() {
	auth.RegisterInstantiator(newInstantiator())
}

type instantiator struct {
	ownedReturnType reflect.Type
	ownedConfigType string
}

func newInstantiator() *instantiator {
	return &instantiator{reflect.TypeOf((*oauthhandlers.RedirectAuthHandler)(nil)).Elem(), "external-oauth"}
}

func (a *instantiator) Owns(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo) bool {
	return (resultingType == a.ownedReturnType) && (elementConfigInfo.AuthElementConfigType == a.ownedConfigType)
}
func (a *instantiator) IsValid(elementConfigInfo auth.AuthElementConfigInfo, authConfigInfo auth.AuthConfigInfo) error {
	identityMapperName, _ := elementConfigInfo.Config[identityMapperKey]
	if len(identityMapperName) == 0 {
		return fmt.Errorf("%v is required", identityMapperKey)
	}
	if _, ok := authConfigInfo.IdentityMappers[identityMapperName]; !ok {
		return fmt.Errorf("IdentityMapper %v was not found", identityMapperName)
	}
	if len(elementConfigInfo.Config[clientIdKey]) == 0 {
		return fmt.Errorf("%v is required", clientIdKey)
	}
	if len(elementConfigInfo.Config[clientSecretKey]) == 0 {
		return fmt.Errorf("%v is required", clientSecretKey)
	}
	flavor := elementConfigInfo.Config[flavorKey]
	if len(flavor) == 0 {
		return fmt.Errorf("%v is required", flavorKey)
	}
	switch flavor {
	case "github":
	case "google":
	default:
		return fmt.Errorf("Flavor %v is not recognized", flavor)
	}

	return nil
}
func (a *instantiator) Instantiate(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo, authConfig auth.AuthConfig, envInfo *auth.EnvInfo) (interface{}, error) {
	if !a.Owns(resultingType, elementConfigInfo) {
		return nil, fmt.Errorf("%v does not own %v", a, elementConfigInfo)
	}

	identityMapperName, _ := elementConfigInfo.Config[identityMapperKey]
	identityMapper := authConfig.IdentityMappers[identityMapperName]
	clientId, _ := elementConfigInfo.Config[clientIdKey]
	clientSecret, _ := elementConfigInfo.Config[clientSecretKey]
	flavor := elementConfigInfo.Config[flavorKey]

	callbackPath := auth.OpenShiftOAuthCallbackPrefix + "/" + flavor
	errorHandler := oauthhandlers.EmptyError{}
	successHandler := getAuthenticationSuccessHandler(authConfig, envInfo)

	var oauthProvider external.Provider
	switch flavor {
	case "github":
		oauthProvider = github.NewProvider(clientId, clientSecret)
	case "google":
		oauthProvider = google.NewProvider(clientId, clientSecret)
	}

	state := external.DefaultState()
	oauthHandler, err := external.NewHandler(oauthProvider, state, envInfo.MasterAddr+callbackPath, successHandler, errorHandler, identityMapper)
	if err != nil {
		return nil, err
	}

	envInfo.Mux.Handle(callbackPath, oauthHandler)

	return oauthHandler, nil
}

func getAuthenticationSuccessHandler(authConfig auth.AuthConfig, envInfo *auth.EnvInfo) oauthhandlers.AuthenticationSuccessHandler {
	successHandlers := oauthhandlers.AuthenticationSuccessHandlers{}

	for _, currRequestAuthenticator := range authConfig.RequestAuthenticators {
		if _, ok := currRequestAuthenticator.(*session.SessionAuthenticator); ok {
			successHandlers = append(successHandlers, session.NewSessionAuthenticator(envInfo.SessionStore, "ssn"))
		}
	}

	successHandlers = append(successHandlers, external.DefaultState().(oauthhandlers.AuthenticationSuccessHandler))

	return successHandlers
}
