package csv

import (
	"fmt"
	"reflect"

	authauthenticator "github.com/openshift/origin/pkg/auth/authenticator"
	authfile "github.com/openshift/origin/pkg/auth/authenticator/file"
	"github.com/openshift/origin/pkg/cmd/auth"
)

const (
	fileLocationKey = "fileLocation"
)

func init() {
	auth.RegisterInstantiator(newInstantiator())
}

type instantiator struct {
	ownedReturnType reflect.Type
	ownedConfigType string
}

func newInstantiator() *instantiator {
	return &instantiator{reflect.TypeOf((*authauthenticator.Token)(nil)).Elem(), "csv"}
}

func (a *instantiator) Owns(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo) bool {
	return (resultingType == a.ownedReturnType) && (elementConfigInfo.AuthElementConfigType == a.ownedConfigType)
}
func (a *instantiator) IsValid(elementConfigInfo auth.AuthElementConfigInfo, authConfigInfo auth.AuthConfigInfo) error {
	fileLocation, _ := elementConfigInfo.Config[fileLocationKey]
	if len(fileLocation) == 0 {
		return fmt.Errorf("%v is required", fileLocationKey)
	}

	return nil
}
func (a *instantiator) Instantiate(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo, authConfig auth.AuthConfig, envInfo *auth.EnvInfo) (interface{}, error) {
	if !a.Owns(resultingType, elementConfigInfo) {
		return nil, fmt.Errorf("%v does not own %v", a, elementConfigInfo)
	}

	return authfile.NewTokenAuthenticator(elementConfigInfo.Config[fileLocationKey])
}
