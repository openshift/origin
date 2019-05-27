package v1

import (
	"testing"

	runtime "k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/openshift/api/oauth/v1"
	oauthapi "github.com/openshift/openshift-apiserver/pkg/oauth/apis/oauth"
	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
)

func TestFieldSelectorConversions(t *testing.T) {
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{Install},
		Kind:          v1.GroupVersion.WithKind("OAuthAccessToken"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"clientName", "userName", "userUID", "authorizeToken"},
		FieldKeyEvaluatorFn:      oauthapi.OAuthAccessTokenFieldSelector,
	}.Check(t)
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{Install},
		Kind:          v1.GroupVersion.WithKind("OAuthAuthorizeToken"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"clientName", "userName", "userUID"},
		FieldKeyEvaluatorFn:      oauthapi.OAuthAuthorizeTokenFieldSelector,
	}.Check(t)
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{Install},
		Kind:          v1.GroupVersion.WithKind("OAuthClientAuthorization"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"clientName", "userName", "userUID"},
		FieldKeyEvaluatorFn:      oauthapi.OAuthClientAuthorizationFieldSelector,
	}.Check(t)
}
