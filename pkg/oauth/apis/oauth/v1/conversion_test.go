package v1

import (
	"testing"

	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

func TestFieldSelectorConversions(t *testing.T) {
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{LegacySchemeBuilder.AddToScheme, oauthapi.LegacySchemeBuilder.AddToScheme},
		Kind:          LegacySchemeGroupVersion.WithKind("OAuthAccessToken"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"clientName", "userName", "userUID", "authorizeToken"},
		FieldKeyEvaluatorFn:      oauthapi.OAuthAccessTokenFieldSelector,
	}.Check(t)
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{LegacySchemeBuilder.AddToScheme, oauthapi.LegacySchemeBuilder.AddToScheme},
		Kind:          LegacySchemeGroupVersion.WithKind("OAuthAuthorizeToken"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"clientName", "userName", "userUID"},
		FieldKeyEvaluatorFn:      oauthapi.OAuthAuthorizeTokenFieldSelector,
	}.Check(t)
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{LegacySchemeBuilder.AddToScheme, oauthapi.LegacySchemeBuilder.AddToScheme},
		Kind:          LegacySchemeGroupVersion.WithKind("OAuthClientAuthorization"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"clientName", "userName", "userUID"},
		FieldKeyEvaluatorFn:      oauthapi.OAuthClientAuthorizationFieldSelector,
	}.Check(t)

	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{SchemeBuilder.AddToScheme, oauthapi.SchemeBuilder.AddToScheme},
		Kind:          SchemeGroupVersion.WithKind("OAuthAccessToken"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"clientName", "userName", "userUID", "authorizeToken"},
		FieldKeyEvaluatorFn:      oauthapi.OAuthAccessTokenFieldSelector,
	}.Check(t)
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{SchemeBuilder.AddToScheme, oauthapi.SchemeBuilder.AddToScheme},
		Kind:          SchemeGroupVersion.WithKind("OAuthAuthorizeToken"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"clientName", "userName", "userUID"},
		FieldKeyEvaluatorFn:      oauthapi.OAuthAuthorizeTokenFieldSelector,
	}.Check(t)
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{SchemeBuilder.AddToScheme, oauthapi.SchemeBuilder.AddToScheme},
		Kind:          SchemeGroupVersion.WithKind("OAuthClientAuthorization"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"clientName", "userName", "userUID"},
		FieldKeyEvaluatorFn:      oauthapi.OAuthClientAuthorizationFieldSelector,
	}.Check(t)
}
