package v1

import (
	kapi "k8s.io/kubernetes/pkg/api"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/oauth/api"
)

func init() {
	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "OAuthAccessToken",
		oapi.GetFieldLabelConversionFunc(api.OAuthAccessTokenToSelectableFields(&api.OAuthAccessToken{}), nil),
	); err != nil {
		panic(err)
	}

	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "OAuthAuthorizeToken",
		oapi.GetFieldLabelConversionFunc(api.OAuthAuthorizeTokenToSelectableFields(&api.OAuthAuthorizeToken{}), nil),
	); err != nil {
		panic(err)
	}

	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "OAuthClient",
		oapi.GetFieldLabelConversionFunc(api.OAuthClientToSelectableFields(&api.OAuthClient{}), nil),
	); err != nil {
		panic(err)
	}

	if err := kapi.Scheme.AddFieldLabelConversionFunc("v1", "OAuthClientAuthorization",
		oapi.GetFieldLabelConversionFunc(api.OAuthClientAuthorizationToSelectableFields(&api.OAuthClientAuthorization{}), nil),
	); err != nil {
		panic(err)
	}
}
