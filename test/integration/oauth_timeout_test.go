package integration

import (
	"testing"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthvalidation "github.com/openshift/origin/pkg/oauth/apis/oauth/validation"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	"github.com/openshift/origin/pkg/oc/util/tokencmd"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func testTokenWorks(t *testing.T, anonConfig *restclient.Config, token string, expectTimeout bool) {
	// Make sure we can use the token, and it represents who we expect
	userConfig := *anonConfig
	userConfig.BearerToken = token
	userClient, err := userclient.NewForConfig(&userConfig)
	if err != nil {
		t.Fatal(err)
	}

	user, err := userClient.Users().Get("~", metav1.GetOptions{})
	if err != nil {
		if expectTimeout && kerrors.IsUnauthorized(err) {
			return
		}
		t.Errorf("Unexpected error getting user ~: %v", err)
	}
	if user.Name != "username" {
		t.Errorf("Expected username as the user, got %v", user)
	}
}

func testTimeoutOAuthFlows(t *testing.T, tokens oauthclient.OAuthAccessTokenInterface, oauthClient *oauthapi.OAuthClient, anonConfig *restclient.Config, expectedTimeout int32) string {
	var lastToken string

	// token flow followed by code flow
	for _, tokenFlow := range []bool{true, false} {
		tokenOpts := tokencmd.NewRequestTokenOptions(anonConfig, nil, "username", "password", tokenFlow)
		if err := tokenOpts.SetDefaultOsinConfig(); err != nil {
			t.Fatal(err)
		}
		tokenOpts.OsinConfig.ClientId = oauthClient.Name
		tokenOpts.OsinConfig.RedirectUrl = oauthClient.RedirectURIs[0]
		token, err := tokenOpts.RequestToken()
		if err != nil {
			t.Fatal(err)
		}

		// Make sure the token exists with the overridden time
		tokenObj, err := tokens.Get(token, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if tokenObj.InactivityTimeoutSeconds != expectedTimeout {
			t.Errorf("Token flow=%v, expected timeout of %d, got %#v",
				tokenFlow, expectedTimeout, tokenObj.InactivityTimeoutSeconds)
		}

		testTokenWorks(t, anonConfig, token, false)

		lastToken = token
	}

	return lastToken
}

func TestOAuthTimeout(t *testing.T) {
	testTimeout := int32(oauthvalidation.MinimumInactivityTimeoutSeconds * 2)
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatal(err)
	}
	masterOptions.OAuthConfig.TokenConfig.AccessTokenInactivityTimeoutSeconds = &testTimeout
	defer testserver.CleanupMasterEtcd(t, masterOptions)

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatal(err)
	}

	clientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	oauthClient := oauthclient.NewForConfigOrDie(clientConfig)

	// Use the server and CA info
	anonConfig := restclient.AnonymousClientConfig(clientConfig)

	{
		client, err := oauthClient.OAuthClients().Create(&oauthapi.OAuthClient{
			ObjectMeta:            metav1.ObjectMeta{Name: "defaulttimeout"},
			RespondWithChallenges: true,
			RedirectURIs:          []string{"http://localhost"},
			GrantMethod:           oauthapi.GrantHandlerAuto,
		})
		if err != nil {
			t.Fatal(err)
		}

		testTimeoutOAuthFlows(t, oauthClient.OAuthAccessTokens(), client, anonConfig, testTimeout)
	}

	{
		client, err := oauthClient.OAuthClients().Create(&oauthapi.OAuthClient{
			ObjectMeta:                          metav1.ObjectMeta{Name: "notimeout"},
			RespondWithChallenges:               true,
			RedirectURIs:                        []string{"http://localhost"},
			AccessTokenInactivityTimeoutSeconds: new(int32),
			GrantMethod:                         oauthapi.GrantHandlerAuto,
		})
		if err != nil {
			t.Fatal(err)
		}

		testTimeoutOAuthFlows(t, oauthClient.OAuthAccessTokens(), client, anonConfig, 0)
	}

	// check that we get an error trying to set a client value that is less
	// than the allowable minimum
	{
		invalid := int32(oauthvalidation.MinimumInactivityTimeoutSeconds - 1)
		_, err := oauthClient.OAuthClients().Create(&oauthapi.OAuthClient{
			ObjectMeta:                          metav1.ObjectMeta{Name: "notvalid"},
			RespondWithChallenges:               true,
			RedirectURIs:                        []string{"http://localhost"},
			AccessTokenInactivityTimeoutSeconds: &invalid,
			GrantMethod:                         oauthapi.GrantHandlerAuto,
		})
		if !kerrors.IsInvalid(err) {
			t.Errorf("The 'notvalid' test is supposed to be invalid but gave=%v", err)
		}
	}

	{
		min := int32(oauthvalidation.MinimumInactivityTimeoutSeconds)
		client, err := oauthClient.OAuthClients().Create(&oauthapi.OAuthClient{
			ObjectMeta:                          metav1.ObjectMeta{Name: "mintimeout"},
			RespondWithChallenges:               true,
			RedirectURIs:                        []string{"http://localhost"},
			AccessTokenInactivityTimeoutSeconds: &min,
			GrantMethod:                         oauthapi.GrantHandlerAuto,
		})
		if err != nil {
			t.Fatal(err)
		}

		token := testTimeoutOAuthFlows(t, oauthClient.OAuthAccessTokens(), client, anonConfig, min)

		// wait 50% of timeout time, then try token and see it still work
		time.Sleep(time.Duration(min/2) * time.Second)
		testTokenWorks(t, anonConfig, token, false)

		// Then Ensure the token times out
		time.Sleep(time.Duration(min+1) * time.Second)
		testTokenWorks(t, anonConfig, token, true)
	}
}

func TestOAuthTimeoutNotEnabled(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatal(err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatal(err)
	}
	oauthClient := oauthclient.NewForConfigOrDie(clientConfig)

	// Use the server and CA info
	anonConfig := restclient.AnonymousClientConfig(clientConfig)

	min := int32(oauthvalidation.MinimumInactivityTimeoutSeconds)
	client, err := oauthClient.OAuthClients().Create(&oauthapi.OAuthClient{
		ObjectMeta:                          metav1.ObjectMeta{Name: "shorttimeoutthatisignored"},
		RespondWithChallenges:               true,
		RedirectURIs:                        []string{"http://localhost"},
		AccessTokenInactivityTimeoutSeconds: &min,
		GrantMethod:                         oauthapi.GrantHandlerAuto,
	})
	if err != nil {
		t.Fatal(err)
	}

	token := testTimeoutOAuthFlows(t, oauthClient.OAuthAccessTokens(), client, anonConfig, min)

	// ensure the token does not timeout because the feature is not active by default
	time.Sleep(time.Duration(min+30) * time.Second)
	testTokenWorks(t, anonConfig, token, false)
}
