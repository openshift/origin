package oauth

import (
	"fmt"

	"k8s.io/apimachinery/pkg/fields"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

func OAuthAccessTokenFieldSelector(obj runtime.Object, fieldSet fields.Set) error {
	oauthAccessToken, ok := obj.(*OAuthAccessToken)
	if !ok {
		return fmt.Errorf("%T not an OAuthAccessToken", obj)
	}
	fieldSet["clientName"] = oauthAccessToken.ClientName
	fieldSet["userName"] = oauthAccessToken.UserName
	fieldSet["userUID"] = oauthAccessToken.UserUID
	fieldSet["authorizeToken"] = oauthAccessToken.AuthorizeToken
	return nil
}

func OAuthAuthorizeTokenFieldSelector(obj runtime.Object, fieldSet fields.Set) error {
	oauthAuthorizeToken, ok := obj.(*OAuthAuthorizeToken)
	if !ok {
		return fmt.Errorf("%T not an OAuthAuthorizeToken", obj)
	}
	fieldSet["clientName"] = oauthAuthorizeToken.ClientName
	fieldSet["userName"] = oauthAuthorizeToken.UserName
	fieldSet["userUID"] = oauthAuthorizeToken.UserUID
	return nil
}

func OAuthClientAuthorizationFieldSelector(obj runtime.Object, fieldSet fields.Set) error {
	oauthClientAuthorization, ok := obj.(*OAuthClientAuthorization)
	if !ok {
		return fmt.Errorf("%T not an OAuthAuthorizeToken", obj)
	}
	fieldSet["clientName"] = oauthClientAuthorization.ClientName
	fieldSet["userName"] = oauthClientAuthorization.UserName
	fieldSet["userUID"] = oauthClientAuthorization.UserUID
	return nil
}
