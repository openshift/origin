package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthAccessToken",
		oapi.GetFieldLabelConversionFunc(oauthapi.OAuthAccessTokenToSelectableFields(&oauthapi.OAuthAccessToken{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthAuthorizeToken",
		oapi.GetFieldLabelConversionFunc(oauthapi.OAuthAuthorizeTokenToSelectableFields(&oauthapi.OAuthAuthorizeToken{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthClientAuthorization",
		oapi.GetFieldLabelConversionFunc(oauthapi.OAuthClientAuthorizationToSelectableFields(&oauthapi.OAuthClientAuthorization{}), nil),
	); err != nil {
		return err
	}
	return nil
}
