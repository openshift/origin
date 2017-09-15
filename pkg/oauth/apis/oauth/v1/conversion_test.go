package v1

import (
	"testing"

	runtime "k8s.io/apimachinery/pkg/runtime"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
)

func TestFieldSelectorConversions(t *testing.T) {
	converter := runtime.NewScheme()
	LegacySchemeBuilder.AddToScheme(converter)

	apitesting.TestFieldLabelConversions(t, converter, "v1", "OAuthAccessToken",
		// Ensure all currently returned labels are supported
		oauthapi.OAuthAccessTokenToSelectableFields(&oauthapi.OAuthAccessToken{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"clientName", "userName", "userUID", "authorizeToken",
	)

	apitesting.TestFieldLabelConversions(t, converter, "v1", "OAuthAuthorizeToken",
		// Ensure all currently returned labels are supported
		oauthapi.OAuthAuthorizeTokenToSelectableFields(&oauthapi.OAuthAuthorizeToken{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"clientName", "userName", "userUID",
	)

	apitesting.TestFieldLabelConversions(t, converter, "v1", "OAuthClientAuthorization",
		// Ensure all currently returned labels are supported
		oauthapi.OAuthClientAuthorizationToSelectableFields(&oauthapi.OAuthClientAuthorization{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"clientName", "userName", "userUID",
	)
}
