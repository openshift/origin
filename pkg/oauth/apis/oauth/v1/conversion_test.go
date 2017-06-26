package v1_test

import (
	"testing"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	testutil "github.com/openshift/origin/test/util/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "OAuthAccessToken",
		// Ensure all currently returned labels are supported
		oauthapi.OAuthAccessTokenToSelectableFields(&oauthapi.OAuthAccessToken{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"clientName", "userName", "userUID", "authorizeToken",
	)

	testutil.CheckFieldLabelConversions(t, "v1", "OAuthAuthorizeToken",
		// Ensure all currently returned labels are supported
		oauthapi.OAuthAuthorizeTokenToSelectableFields(&oauthapi.OAuthAuthorizeToken{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"clientName", "userName", "userUID",
	)

	testutil.CheckFieldLabelConversions(t, "v1", "OAuthClient",
		// Ensure all currently returned labels are supported
		oauthapi.OAuthClientToSelectableFields(&oauthapi.OAuthClient{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "OAuthClientAuthorization",
		// Ensure all currently returned labels are supported
		oauthapi.OAuthClientAuthorizationToSelectableFields(&oauthapi.OAuthClientAuthorization{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"clientName", "userName", "userUID",
	)
}
