package instantiator

import (
	"fmt"
	"reflect"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/auth/userregistry/identitymapper"
	"github.com/openshift/origin/pkg/cmd/auth"
	"github.com/openshift/origin/pkg/user"
	useretcd "github.com/openshift/origin/pkg/user/registry/etcd"
)

func init() {
	auth.RegisterInstantiator(newInstantiator())
}

type instantiator struct {
	ownedReturnType reflect.Type
	ownedConfigType string
}

func newInstantiator() *instantiator {
	return &instantiator{reflect.TypeOf((*authapi.UserIdentityMapper)(nil)).Elem(), "autocreate"}
}

func (a *instantiator) Owns(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo) bool {
	return (resultingType == a.ownedReturnType) && (elementConfigInfo.AuthElementConfigType == a.ownedConfigType)
}
func (a *instantiator) IsValid(elementConfigInfo auth.AuthElementConfigInfo, authConfigInfo auth.AuthConfigInfo) error {
	return nil
}
func (a *instantiator) Instantiate(resultingType reflect.Type, elementConfigInfo auth.AuthElementConfigInfo, authConfig auth.AuthConfig, envInfo *auth.EnvInfo) (interface{}, error) {
	if !a.Owns(resultingType, elementConfigInfo) {
		return nil, fmt.Errorf("%v does not own %v", a, elementConfigInfo)
	}

	userRegistry := useretcd.New(envInfo.EtcdHelper, user.NewDefaultUserInitStrategy())
	return identitymapper.NewAlwaysCreateUserIdentityToUserMapper(elementConfigInfo.Config["providerPrefix"], userRegistry), nil
}
