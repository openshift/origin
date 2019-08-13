package legacy

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	oauthv1 "github.com/openshift/api/oauth/v1"
)

func InstallExternalLegacyOAuth(scheme *runtime.Scheme) {
	schemeBuilder := runtime.NewSchemeBuilder(
		addUngroupifiedOAuthTypes,
	)
	utilruntime.Must(schemeBuilder.AddToScheme(scheme))
}

func addUngroupifiedOAuthTypes(scheme *runtime.Scheme) error {
	types := []runtime.Object{
		&oauthv1.OAuthAccessToken{},
		&oauthv1.OAuthAccessTokenList{},
		&oauthv1.OAuthAuthorizeToken{},
		&oauthv1.OAuthAuthorizeTokenList{},
		&oauthv1.OAuthClient{},
		&oauthv1.OAuthClientList{},
		&oauthv1.OAuthClientAuthorization{},
		&oauthv1.OAuthClientAuthorizationList{},
		&oauthv1.OAuthRedirectReference{},
	}
	scheme.AddKnownTypes(GroupVersion, types...)
	return nil
}
