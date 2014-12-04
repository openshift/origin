package empty

import (
	"fmt"
	"reflect"

	authauthenticator "github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/anyauthpassword"
	"github.com/openshift/origin/pkg/cmd/auth"
)

const (
	identityMapperKey = "identityMapper"
)

func init() {
	auth.RegisterInstantiator(newInstantiator())
}

type instantiator struct {
	ownedReturnType reflect.Type
	ownedConfigType string
}

func newInstantiator() *instantiator {
	return &instantiator{reflect.TypeOf((*authauthenticator.Password)(nil)).Elem(), "empty"}
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

	return nil
}
func (a *instantiator) Instantiate(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo, authConfig auth.AuthConfig, envInfo *auth.EnvInfo) (interface{}, error) {
	if !a.Owns(resultingType, elementConfigInfo) {
		return nil, fmt.Errorf("%v does not own %v", a, elementConfigInfo)
	}

	identityMapperName, _ := elementConfigInfo.Config[identityMapperKey]
	return anyauthpassword.New(authConfig.IdentityMappers[identityMapperName]), nil
}
