package oauth

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	GroupName = "oauth.openshift.io"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(
		addKnownTypes,
	)
	Install = schemeBuilder.AddToScheme

	// DEPRECATED kept for generated code
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: runtime.APIVersionInternal}
	// DEPRECATED kept for generated code
	AddToScheme = schemeBuilder.AddToScheme
)

// Resource kept for generated code
// DEPRECATED
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&OAuthAccessToken{},
		&OAuthAccessTokenList{},
		&OAuthAuthorizeToken{},
		&OAuthAuthorizeTokenList{},
		&OAuthClient{},
		&OAuthClientList{},
		&OAuthClientAuthorization{},
		&OAuthClientAuthorizationList{},
		&OAuthRedirectReference{},
	)
	return nil
}
