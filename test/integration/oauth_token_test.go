package integration

import (
	"fmt"
	"io/ioutil"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/cli/cmd/login"
	clientcmdutil "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestOAuthTokens(t *testing.T) {
	//
	// Server setup
	//
	testutil.RequireEtcd(t)
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	//
	// Token setup
	//

	simpleUsername := "bob"
	// TODO: change to something like '=bond, james bond (code=009)' once field selectors are fixed
	complexUsername := "bob@example.com"

	// Log in and get a token for alice
	aliceLoginOptions := &login.LoginOptions{
		Server:             clusterAdminClientConfig.Host,
		InsecureTLS:        true,
		StartingKubeConfig: &clientcmdapi.Config{},
		Username:           simpleUsername,
		Password:           "mypassword",
		Out:                ioutil.Discard,
	}
	if err := aliceLoginOptions.GatherInfo(); err != nil {
		t.Fatalf("Error trying to log in: %v", err)
	}
	if len(aliceLoginOptions.Config.BearerToken) == 0 {
		t.Fatalf("No token from login")
	}

	// Log in and get a token for bob
	bobLoginOptions := &login.LoginOptions{
		Server:             clusterAdminClientConfig.Host,
		InsecureTLS:        true,
		StartingKubeConfig: &clientcmdapi.Config{},
		Username:           complexUsername,
		Password:           "mypassword",
		Out:                ioutil.Discard,
	}
	if err := bobLoginOptions.GatherInfo(); err != nil {
		t.Fatalf("Error trying to log in: %v", err)
	}
	if len(bobLoginOptions.Config.BearerToken) == 0 {
		t.Fatalf("No token from login")
	}

	// Fetch the provisioned user, and create an OAuth client so we can create our own tokens
	complexUser, err := clusterAdminClient.Users().Get(complexUsername)
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	_, err = clusterAdminClient.OAuthClients().Create(&oauthapi.OAuthClient{ObjectMeta: kapi.ObjectMeta{Name: "my-client"}})
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}

	// Create an unhashed token directly
	unhashedToken, err := clusterAdminClient.OAuthAccessTokens().Create(&oauthapi.OAuthAccessToken{
		ObjectMeta: kapi.ObjectMeta{Name: "0000000000-0000000000-0000000000-0000000000"},
		UserName:   complexUser.Name,
		UserUID:    string(complexUser.UID),
		ClientName: "my-client",
		ExpiresIn:  60,
	})
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if unhashedToken.Name != "0000000000-0000000000-0000000000-0000000000" {
		t.Fatalf("Expected directly created token with name to preserve name")
	}
	if unhashedToken.Token != "0000000000-0000000000-0000000000-0000000000" {
		t.Fatalf("Expected directly created token with name to preserve name")
	}
	if unhashedToken.Salt != "" || unhashedToken.SaltedHash != "" {
		t.Fatalf("Expected directly created token with name to not be hashed")
	}

	// Create a hashed token directly
	hashedToken, err := clusterAdminClient.OAuthAccessTokens().Create(&oauthapi.OAuthAccessToken{
		UserName:   complexUser.Name,
		UserUID:    string(complexUser.UID),
		ClientName: "my-client",
		ExpiresIn:  60,
	})
	if err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	if hashedToken.Name == "" || hashedToken.Token == "" || hashedToken.Name == hashedToken.Token || hashedToken.Salt == "" || hashedToken.SaltedHash == "" {
		t.Fatalf("Expected directly created token to generate name, token, and salted hash: %#v", hashedToken)
	}

	//
	// Listing/filtering tests
	//
	listTestcases := map[string]struct {
		FieldSelector        fields.Selector
		ExpectedTokens       int
		ExpectOnlyUserName   string
		ExpectOnlyClientName string
		ExpectOnlyHashed     bool
		ExpectOnlyUnhashed   bool
	}{
		"unfiltered": {
			ExpectedTokens: 4,
		},
		"auto-created": {
			FieldSelector:      fields.OneTermEqualSelector("userName", simpleUsername),
			ExpectedTokens:     1,
			ExpectOnlyUserName: simpleUsername,
			ExpectOnlyHashed:   true,
		},
		"complex user": {
			FieldSelector:      fields.OneTermEqualSelector("userName", complexUsername),
			ExpectedTokens:     3,
			ExpectOnlyUserName: complexUsername,
		},
		"complex user my-client": {
			FieldSelector:        fields.ParseSelectorOrDie(fmt.Sprintf("userName=%s,clientName=my-client", complexUsername)), // fields.EscapeValue(complexUsername)
			ExpectedTokens:       2,
			ExpectOnlyUserName:   complexUsername,
			ExpectOnlyClientName: "my-client",
		},
	}
	for k, tc := range listTestcases {
		list, err := clusterAdminClient.OAuthAccessTokens().List(kapi.ListOptions{FieldSelector: tc.FieldSelector})
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		if len(list.Items) != tc.ExpectedTokens {
			t.Errorf("%s: expected %d tokens, got %d", k, tc.ExpectedTokens, len(list.Items))
			continue
		}

		if len(tc.ExpectOnlyUserName) > 0 {
			others := sets.NewString()
			for _, token := range list.Items {
				if token.UserName != tc.ExpectOnlyUserName {
					others.Insert(token.UserName)
				}
			}
			if len(others) > 0 {
				t.Errorf("%s: expected only UserName %s, got %v", k, tc.ExpectOnlyUserName, others.List())
				continue
			}
		}

		if len(tc.ExpectOnlyClientName) > 0 {
			others := sets.NewString()
			for _, token := range list.Items {
				if token.ClientName != tc.ExpectOnlyClientName {
					others.Insert(token.ClientName)
				}
			}
			if len(others) > 0 {
				t.Errorf("%s: expected only ClientName %s, got %v", k, tc.ExpectOnlyClientName, others.List())
				continue
			}
		}

		if tc.ExpectOnlyHashed || tc.ExpectOnlyUnhashed {
			hashed := sets.NewString()
			unhashed := sets.NewString()
			for _, token := range list.Items {
				if len(token.SaltedHash) > 0 {
					hashed.Insert(token.Name)
				} else {
					unhashed.Insert(token.Name)
				}
			}
			if tc.ExpectOnlyHashed && len(unhashed) > 0 {
				t.Errorf("%s: expected only hashed tokens, got unhashed tokens: %v", k, unhashed.List())
				continue
			}
			if tc.ExpectOnlyUnhashed && len(hashed) > 0 {
				t.Errorf("%s: expected only unhashed tokens, got hashed tokens: %v", k, hashed.List())
				continue
			}
		}

	}

	//
	// Token tests
	//
	testcases := map[string]struct {
		BearerToken  string
		ExpectUser   string
		ExpectHashed bool
	}{
		"alice login": {
			BearerToken:  aliceLoginOptions.Config.BearerToken,
			ExpectUser:   simpleUsername,
			ExpectHashed: true,
		},

		"bob login": {
			BearerToken:  bobLoginOptions.Config.BearerToken,
			ExpectUser:   complexUsername,
			ExpectHashed: true,
		},

		"manual hashed": {
			BearerToken:  hashedToken.Token,
			ExpectUser:   complexUsername,
			ExpectHashed: true,
		},

		"manual unhashed": {
			BearerToken:  unhashedToken.Token,
			ExpectUser:   complexUsername,
			ExpectHashed: false,
		},
	}

	for k, tc := range testcases {
		// Ensure it works as a bearer token
		tcClientConfig := clientcmdutil.AnonymousClientConfig(clusterAdminClientConfig)
		tcClientConfig.BearerToken = tc.BearerToken
		tcClient, err := client.New(&tcClientConfig)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		whoamiOptions := cmd.WhoAmIOptions{UserInterface: tcClient.Users(), Out: ioutil.Discard}
		retrievedUser, err := whoamiOptions.WhoAmI()
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		if retrievedUser.Name != tc.ExpectUser {
			t.Errorf("%s: expected %v, got %v", k, tc.ExpectUser, retrievedUser.Name)
			continue
		}

		// We should always be able to find the token by the bearer token, hashed or unhashed
		tokenObj, err := clusterAdminClient.OAuthAccessTokens().Get(tc.BearerToken)
		if err != nil {
			t.Errorf("%s: Could not find object stored by unhashed token: %#v", k, err)
			continue
		}

		switch {
		case tc.ExpectHashed:
			if len(tokenObj.Salt) == 0 || len(tokenObj.SaltedHash) == 0 {
				t.Errorf("%s: Got unhashed object: %#v", k, tokenObj)
				continue
			}
			if len(tokenObj.Token) > 0 || tokenObj.GenerateName == tc.BearerToken {
				t.Errorf("%s: Got token object containing token field or unhashed token: %#v", k, tokenObj)
				continue
			}

		case !tc.ExpectHashed:
			if len(tokenObj.Salt) > 0 || len(tokenObj.SaltedHash) > 0 || tokenObj.Name != tc.BearerToken {
				t.Errorf("%s: Got hashed token object: %#v", k, tokenObj)
				continue
			}

		}
	}
}
