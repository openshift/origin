package v1_test

import (
	"testing"

	"github.com/openshift/origin/pkg/oauth/api"
	testutil "github.com/openshift/origin/test/util/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "OAuthAccessToken",
		// Ensure all currently returned labels are supported
		api.OAuthAccessTokenToSelectableFields(&api.OAuthAccessToken{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"clientName", "userName", "userUID", "authorizeToken",
	)

	testutil.CheckFieldLabelConversions(t, "v1", "OAuthAuthorizeToken",
		// Ensure all currently returned labels are supported
		api.OAuthAuthorizeTokenToSelectableFields(&api.OAuthAuthorizeToken{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"clientName", "userName", "userUID",
	)

	testutil.CheckFieldLabelConversions(t, "v1", "OAuthClient",
		// Ensure all currently returned labels are supported
		api.OAuthClientToSelectableFields(&api.OAuthClient{}),
	)

	testutil.CheckFieldLabelConversions(t, "v1", "OAuthClientAuthorization",
		// Ensure all currently returned labels are supported
		api.OAuthClientAuthorizationToSelectableFields(&api.OAuthClientAuthorization{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"clientName", "userName", "userUID",
	)
}
