package basic

import (
	"fmt"
	"reflect"

	authauthenticator "github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/requesthandlers"
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
	return &instantiator{reflect.TypeOf((*authauthenticator.Request)(nil)).Elem(), "basicauth"}
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
		return fmt.Errorf("TokenAuthenticator %v was not found", passwordAuthenticatorName)
	}

	return nil
}
func (a *instantiator) Instantiate(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo, authConfig auth.AuthConfig, envInfo *auth.EnvInfo) (interface{}, error) {
	if !a.Owns(resultingType, elementConfigInfo) {
		return nil, fmt.Errorf("%v does not own %v", a, elementConfigInfo)
	}

	passwordAuthenticatorName, _ := elementConfigInfo.Config[passwordAuthenticatorKey]
	passwordAuthenticator := authConfig.PasswordAuthenticators[passwordAuthenticatorName]
	return requesthandlers.NewBasicAuthAuthentication(passwordAuthenticator), nil
}
