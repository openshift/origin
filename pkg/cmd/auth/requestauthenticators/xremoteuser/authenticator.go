package xremoteuser

import (
	"fmt"
	"reflect"

	authauthenticator "github.com/openshift/origin/pkg/auth/authenticator"
	"github.com/openshift/origin/pkg/auth/authenticator/requestheader"
	"github.com/openshift/origin/pkg/cmd/auth"
)

const (
	identityMapperKey = "identityMapper"
	headerNameKey     = "headerName"
)

func init() {
	auth.RegisterInstantiator(newInstantiator())
}

type instantiator struct {
	ownedReturnType reflect.Type
	ownedConfigType string
}

func newInstantiator() *instantiator {
	return &instantiator{reflect.TypeOf((*authauthenticator.Request)(nil)).Elem(), "requestheader"}
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
	if len(elementConfigInfo.Config[headerNameKey]) == 0 {
		return fmt.Errorf("%v is required", headerNameKey)
	}

	return nil
}
func (a *instantiator) Instantiate(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo, authConfig auth.AuthConfig, envInfo *auth.EnvInfo) (interface{}, error) {
	if !a.Owns(resultingType, elementConfigInfo) {
		return nil, fmt.Errorf("%v does not own %v", a, elementConfigInfo)
	}

	identityMapperName, _ := elementConfigInfo.Config[identityMapperKey]
	identityMapper := authConfig.IdentityMappers[identityMapperName]
	headerName, _ := elementConfigInfo.Config[headerNameKey]
	return requestheader.NewAuthenticator(&requestheader.Config{headerName}, identityMapper), nil
}
