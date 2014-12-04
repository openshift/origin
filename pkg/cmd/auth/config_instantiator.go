package auth

import (
	"fmt"
	"reflect"

	authapi "github.com/openshift/origin/pkg/auth/api"
	authauthenticator "github.com/openshift/origin/pkg/auth/authenticator"
	oauthhandlers "github.com/openshift/origin/pkg/auth/oauth/handlers"
)

var (
	configInstantiators []AuthConfigInstantiator
)

func RegisterInstantiator(instantiator AuthConfigInstantiator) {
	configInstantiators = append(configInstantiators, instantiator)
}

func InstantiateAuthConfig(info AuthConfigInfo, envInfo *EnvInfo) (*AuthConfig, error) {
	ret := NewAuthConfig()

	// IdentityMappers
	requiredType := reflect.TypeOf((*authapi.UserIdentityMapper)(nil)).Elem()
	identityMappers, err := buildAuthConfigElements(requiredType, info.IdentityMappers, info, *ret, envInfo)
	if err != nil {
		return nil, err
	}
	// there must be a better way to do this cast, I just don't know it
	for name, value := range identityMappers {
		ret.IdentityMappers[name] = value.(authapi.UserIdentityMapper)
	}
	//

	// PasswordAuthenticators
	requiredType = reflect.TypeOf((*authauthenticator.Password)(nil)).Elem()
	passwordAuthenticators, err := buildAuthConfigElements(requiredType, info.PasswordAuthenticators, info, *ret, envInfo)
	if err != nil {
		return nil, err
	}
	// there must be a better way to do this cast, I just don't know it
	for name, value := range passwordAuthenticators {
		ret.PasswordAuthenticators[name] = value.(authauthenticator.Password)
	}
	//

	// TokenAuthenticators
	requiredType = reflect.TypeOf((*authauthenticator.Token)(nil)).Elem()
	tokenAuthenticators, err := buildAuthConfigElements(requiredType, info.TokenAuthenticators, info, *ret, envInfo)
	if err != nil {
		return nil, err
	}
	// there must be a better way to do this cast, I just don't know it
	for name, value := range tokenAuthenticators {
		ret.TokenAuthenticators[name] = value.(authauthenticator.Token)
	}
	//

	// RequestAuthenticators
	requiredType = reflect.TypeOf((*authauthenticator.Request)(nil)).Elem()
	requestAuthenticators, err := buildAuthConfigElements(requiredType, info.RequestAuthenticators, info, *ret, envInfo)
	if err != nil {
		return nil, err
	}
	// there must be a better way to do this cast, I just don't know it
	for name, value := range requestAuthenticators {
		ret.RequestAuthenticators[name] = value.(authauthenticator.Request)
	}
	//

	// RedirectAuthenticationHandlers
	requiredType = reflect.TypeOf((*oauthhandlers.RedirectAuthHandler)(nil)).Elem()
	redirectHandlers, err := buildAuthConfigElements(requiredType, info.AuthorizeAuthenticationRedirectHandlers, info, *ret, envInfo)
	if err != nil {
		return nil, err
	}
	// there must be a better way to do this cast, I just don't know it
	for name, value := range redirectHandlers {
		ret.AuthorizeAuthenticationRedirectHandlers[name] = value.(oauthhandlers.RedirectAuthHandler)
	}
	//

	// ChallengeAuthenticationHandlers
	requiredType = reflect.TypeOf((*oauthhandlers.ChallengeAuthHandler)(nil)).Elem()
	challengeHandlers, err := buildAuthConfigElements(requiredType, info.AuthorizeAuthenticationChallengeHandlers, info, *ret, envInfo)
	if err != nil {
		return nil, err
	}
	// there must be a better way to do this cast, I just don't know it
	for name, value := range challengeHandlers {
		ret.AuthorizeAuthenticationChallengeHandlers[name] = value.(oauthhandlers.ChallengeAuthHandler)
	}
	//

	requiredType = reflect.TypeOf((*oauthhandlers.GrantHandler)(nil)).Elem()
	grantHandler, err := buildAuthConfigElement(requiredType, "grantHandler", info.GrantHandler, info, *ret, envInfo)
	if err != nil {
		return nil, err
	}
	ret.GrantHandler = grantHandler.(oauthhandlers.GrantHandler)

	return ret, nil
}

func buildAuthConfigElements(requiredType reflect.Type, infoMap AuthConfigInfoMap, info AuthConfigInfo, authConfig AuthConfig, envInfo *EnvInfo) (map[string]interface{}, error) {
	ret := make(map[string]interface{})

	for name, elementInfo := range infoMap {
		instance, err := buildAuthConfigElement(requiredType, name, elementInfo, info, authConfig, envInfo)
		if err != nil {
			return nil, err
		}

		ret[name] = instance
	}

	return ret, nil
}
func buildAuthConfigElement(requiredType reflect.Type, name string, elementInfo AuthElementConfigInfo, info AuthConfigInfo, authConfig AuthConfig, envInfo *EnvInfo) (interface{}, error) {
	instantiator := findInstantiator(requiredType, elementInfo)
	if instantiator == nil {
		return nil, fmt.Errorf("Unable to find instantiator for [%v] %v", name, elementInfo)
	}

	err := instantiator.IsValid(elementInfo, info)
	if err != nil {
		return nil, fmt.Errorf("Validation failed for [%v] %v using %v due to %v", name, elementInfo, instantiator, err)
	}

	instance, err := instantiator.Instantiate(requiredType, elementInfo, authConfig, envInfo)
	if err != nil {
		return nil, fmt.Errorf("Unable to instantiate for [%v] %v using %v due to %v", name, elementInfo, instantiator, err)
	}
	if instantiator == nil {
		return nil, fmt.Errorf("Unable to instantiate for [%v] %v using %v", name, elementInfo, instantiator)
	}

	return instance, nil
}

// TODO error out on multiple handlers
func findInstantiator(resultingType reflect.Type, elementInfo AuthElementConfigInfo) AuthConfigInstantiator {
	for _, instantiator := range configInstantiators {
		if instantiator.Owns(resultingType, elementInfo) {
			return instantiator
		}
	}

	return nil
}
