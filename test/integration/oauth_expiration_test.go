package integration

import (
	"net/http"
	"testing"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"

	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	"github.com/openshift/origin/pkg/oc/util/tokencmd"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestOAuthExpiration(t *testing.T) {
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterOptions)

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	oauthClient := oauthclient.NewForConfigOrDie(clientConfig)

	// Use the server and CA info
	anonConfig := restclient.AnonymousClientConfig(clientConfig)

	{
		zero := int32(0)
		client, err := oauthClient.OAuthClients().Create(&oauthapi.OAuthClient{
			ObjectMeta:               metav1.ObjectMeta{Name: "nonexpiring"},
			RespondWithChallenges:    true,
			RedirectURIs:             []string{"http://localhost"},
			AccessTokenMaxAgeSeconds: &zero,
			GrantMethod:              oauthapi.GrantHandlerAuto,
		})
		if err != nil {
			t.Fatal(err)
		}

		testExpiringOAuthFlows(t, clientConfig, client, anonConfig, 0)
	}

	{
		ten := int32(10)
		client, err := oauthClient.OAuthClients().Create(&oauthapi.OAuthClient{
			ObjectMeta:               metav1.ObjectMeta{Name: "shortexpiring"},
			RespondWithChallenges:    true,
			RedirectURIs:             []string{"http://localhost"},
			AccessTokenMaxAgeSeconds: &ten,
			GrantMethod:              oauthapi.GrantHandlerAuto,
		})
		if err != nil {
			t.Fatal(err)
		}

		token := testExpiringOAuthFlows(t, clientConfig, client, anonConfig, 10)

		// Ensure the token goes away after the time expiration
		if err := wait.Poll(1*time.Second, time.Minute, func() (bool, error) {
			_, err := oauthClient.OAuthAccessTokens().Get(token, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return false, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func testExpiringOAuthFlows(t *testing.T, clusterAdminClientConfig *restclient.Config, oauthClient *oauthapi.OAuthClient, anonConfig *restclient.Config, expectedExpires int) string {
	oauthClientset := oauthclient.NewForConfigOrDie(clusterAdminClientConfig)

	// token flow
	{
		tokenOpts := tokencmd.NewRequestTokenOptions(anonConfig, nil, "username", "password", true)
		if err := tokenOpts.SetDefaultOsinConfig(); err != nil {
			t.Fatal(err)
		}
		tokenOpts.OsinConfig.ClientId = oauthClient.Name
		tokenOpts.OsinConfig.RedirectUrl = oauthClient.RedirectURIs[0]
		if len(tokenOpts.OsinConfig.CodeChallenge) != 0 || len(tokenOpts.OsinConfig.CodeChallengeMethod) != 0 || len(tokenOpts.OsinConfig.CodeVerifier) != 0 {
			t.Fatalf("incorrectly set PKCE for OAuth client %q during token flow", oauthClient.Name)
		}
		token, err := tokenOpts.RequestToken()
		if err != nil {
			t.Fatal(err)
		}

		// Make sure we can use the token, and it represents who we expect
		userConfig := *anonConfig
		userConfig.BearerToken = token
		userClient, err := userclient.NewForConfig(&userConfig)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		user, err := userClient.Users().Get("~", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if user.Name != "username" {
			t.Fatalf("Expected username as the user, got %v", user)
		}

		// Make sure the token exists with the overridden time
		tokenObj, err := oauthClientset.OAuthAccessTokens().Get(token, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if tokenObj.ExpiresIn != int64(expectedExpires) {
			t.Fatalf("Expected expiration of %d, got %#v", expectedExpires, tokenObj.ExpiresIn)
		}
	}

	// code flow
	{
		rt, err := restclient.TransportFor(anonConfig)
		if err != nil {
			t.Fatal(err)
		}

		conf := &oauth2.Config{
			ClientID:     oauthClient.Name,
			ClientSecret: oauthClient.Secret,
			RedirectURL:  oauthClient.RedirectURIs[0],
			Endpoint: oauth2.Endpoint{
				AuthURL:  anonConfig.Host + "/oauth/authorize",
				TokenURL: anonConfig.Host + "/oauth/token",
			},
		}

		// get code
		req, err := http.NewRequest("GET", conf.AuthCodeURL(""), nil)
		if err != nil {
			t.Fatal(err)
		}
		req.SetBasicAuth("username", "password")
		resp, err := rt.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusFound {
			t.Fatalf("unexpected status %v", resp.StatusCode)
		}
		location, err := resp.Location()
		if err != nil {
			t.Fatal(err)
		}
		code := location.Query().Get("code")
		if len(code) == 0 {
			t.Fatalf("Unexpected response: %v", location)
		}

		// Make sure the code exists with the default time
		codeObj, err := oauthClientset.OAuthAuthorizeTokens().Get(code, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if codeObj.ExpiresIn != 5*60 {
			t.Fatalf("Expected expiration of %d, got %#v", 5*60, codeObj.ExpiresIn)
		}

		// Use the custom HTTP client when requesting a token.
		httpClient := &http.Client{Transport: rt}
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
		oauthToken, err := conf.Exchange(ctx, code)
		if err != nil {
			t.Fatal(err)
		}
		token := oauthToken.AccessToken

		// Make sure we can use the token, and it represents who we expect
		userConfig := *anonConfig
		userConfig.BearerToken = token
		userClient, err := userclient.NewForConfig(&userConfig)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		user, err := userClient.Users().Get("~", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if user.Name != "username" {
			t.Fatalf("Expected username as the user, got %v", user)
		}

		// Make sure the token exists with the overridden time
		tokenObj, err := oauthClientset.OAuthAccessTokens().Get(token, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if tokenObj.ExpiresIn != int64(expectedExpires) {
			t.Fatalf("Expected expiration of %d, got %#v", expectedExpires, tokenObj.ExpiresIn)
		}

		return token
	}
}
