package v1

import (
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthAccessToken",
		apihelpers.GetFieldLabelConversionFunc(oauthapi.OAuthAccessTokenToSelectableFields(&oauthapi.OAuthAccessToken{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthAuthorizeToken",
		apihelpers.GetFieldLabelConversionFunc(oauthapi.OAuthAuthorizeTokenToSelectableFields(&oauthapi.OAuthAuthorizeToken{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthClientAuthorization",
		apihelpers.GetFieldLabelConversionFunc(oauthapi.OAuthClientAuthorizationToSelectableFields(&oauthapi.OAuthClientAuthorization{}), nil),
	); err != nil {
		return err
	}
	return nil
}
