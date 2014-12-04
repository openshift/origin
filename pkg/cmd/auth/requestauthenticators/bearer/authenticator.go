package bearer

import (
	"fmt"
	"reflect"

	authauthenticator "github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/bearertoken"
	"github.com/openshift/origin/pkg/cmd/auth"
)

const (
	tokenAuthenticatorKey = "tokenAuthenticator"
)

func init() {
	auth.RegisterInstantiator(newInstantiator())
}

type instantiator struct {
	ownedReturnType reflect.Type
	ownedConfigType string
}

func newInstantiator() *instantiator {
	return &instantiator{reflect.TypeOf((*authauthenticator.Request)(nil)).Elem(), "bearer"}
}

func (a *instantiator) Owns(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo) bool {
	return (resultingType == a.ownedReturnType) && (elementConfigInfo.AuthElementConfigType == a.ownedConfigType)
}
func (a *instantiator) IsValid(elementConfigInfo auth.AuthElementConfigInfo, authConfigInfo auth.AuthConfigInfo) error {
	tokenAuthenticatorName, _ := elementConfigInfo.Config[tokenAuthenticatorKey]
	if len(tokenAuthenticatorName) == 0 {
		return fmt.Errorf("%v is required", tokenAuthenticatorKey)
	}
	if _, ok := authConfigInfo.TokenAuthenticators[tokenAuthenticatorName]; !ok {
		return fmt.Errorf("TokenAuthenticator %v was not found", tokenAuthenticatorName)
	}

	return nil
}
func (a *instantiator) Instantiate(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo, authConfig auth.AuthConfig, envInfo *auth.EnvInfo) (interface{}, error) {
	if !a.Owns(resultingType, elementConfigInfo) {
		return nil, fmt.Errorf("%v does not own %v", a, elementConfigInfo)
	}

	tokenAuthenticatorName, _ := elementConfigInfo.Config[tokenAuthenticatorKey]
	return bearertoken.New(authConfig.TokenAuthenticators[tokenAuthenticatorName]), nil
}
