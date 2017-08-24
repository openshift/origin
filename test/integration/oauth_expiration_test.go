package integration

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	restclient "k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
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

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Use the server and CA info
	anonConfig := restclient.AnonymousClientConfig(clientConfig)

	{
		zero := int32(0)
		nonexpiring, err := clusterAdminClient.OAuthClients().Create(&oauthapi.OAuthClient{
			ObjectMeta:               metav1.ObjectMeta{Name: "nonexpiring"},
			RespondWithChallenges:    true,
			RedirectURIs:             []string{"http://localhost"},
			AccessTokenMaxAgeSeconds: &zero,
			GrantMethod:              oauthapi.GrantHandlerAuto,
		})
		if err != nil {
			t.Fatal(err)
		}

		nonExpiringTokenOpts := tokencmd.NewRequestTokenOptions(anonConfig, nil, "username", "password")
		nonExpiringTokenOpts.ClientID = nonexpiring.Name
		nonexpiringToken, err := nonExpiringTokenOpts.RequestToken()
		if err != nil {
			t.Fatal(err)
		}

		// Make sure we can use the token, and it represents who we expect
		nonExpiringUserConfig := *anonConfig
		nonExpiringUserConfig.BearerToken = nonexpiringToken
		nonExpiringUserClient, err := client.New(&nonExpiringUserConfig)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		user, err := nonExpiringUserClient.Users().Get("~", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if user.Name != "username" {
			t.Fatalf("Expected username as the user, got %v", user)
		}

		// Make sure the token exists with the overridden time
		tokenObj, err := clusterAdminClient.OAuthAccessTokens().Get(nonexpiringToken, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if tokenObj.ExpiresIn != 0 {
			t.Fatalf("Expected expiration of 0, got %#v", tokenObj.ExpiresIn)
		}
	}

	{
		ten := int32(10)
		shortexpiring, err := clusterAdminClient.OAuthClients().Create(&oauthapi.OAuthClient{
			ObjectMeta:               metav1.ObjectMeta{Name: "shortexpiring"},
			RespondWithChallenges:    true,
			RedirectURIs:             []string{"http://localhost"},
			AccessTokenMaxAgeSeconds: &ten,
			GrantMethod:              oauthapi.GrantHandlerAuto,
		})
		if err != nil {
			t.Fatal(err)
		}

		expiringTokenOpts := tokencmd.NewRequestTokenOptions(anonConfig, nil, "username", "password")
		expiringTokenOpts.ClientID = shortexpiring.Name
		expiringToken, err := expiringTokenOpts.RequestToken()
		if err != nil {
			t.Fatal(err)
		}

		// Make sure we can use the token, and it represents who we expect
		expiringUserConfig := *anonConfig
		expiringUserConfig.BearerToken = expiringToken
		expiringUserClient, err := client.New(&expiringUserConfig)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		user, err := expiringUserClient.Users().Get("~", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if user.Name != "username" {
			t.Fatalf("Expected username as the user, got %v", user)
		}

		// Ensure the token goes away after the time expiration
		if err := wait.Poll(1*time.Second, time.Minute, func() (bool, error) {
			_, err := clusterAdminClient.OAuthAccessTokens().Get(expiringToken, metav1.GetOptions{})
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
