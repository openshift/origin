package v1

import (
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/oauth/api"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthAccessToken",
		oapi.GetFieldLabelConversionFunc(api.OAuthAccessTokenToSelectableFields(&api.OAuthAccessToken{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthAuthorizeToken",
		oapi.GetFieldLabelConversionFunc(api.OAuthAuthorizeTokenToSelectableFields(&api.OAuthAuthorizeToken{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthClient",
		oapi.GetFieldLabelConversionFunc(api.OAuthClientToSelectableFields(&api.OAuthClient{}), nil),
	); err != nil {
		return err
	}

	if err := scheme.AddFieldLabelConversionFunc("v1", "OAuthClientAuthorization",
		oapi.GetFieldLabelConversionFunc(api.OAuthClientAuthorizationToSelectableFields(&api.OAuthClientAuthorization{}), nil),
	); err != nil {
		return err
	}
	return nil
}
