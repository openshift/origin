// +build integration,etcd

package integration

import (
	"fmt"
	"reflect"
	"testing"

	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/runtime"
	kutil "k8s.io/kubernetes/pkg/util"

	authapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"

	"github.com/vjeantet/ldapserver"
)

func TestOAuthLDAP(t *testing.T) {
	var (
		randomSuffix = string(kutil.NewUUID())

		providerName = "myldapprovider"

		bindDN       = "uid=admin,ou=company,ou=" + randomSuffix
		bindPassword = "admin-password-" + randomSuffix

		searchDN     = "ou=company,ou=" + randomSuffix
		searchAttr   = "myuid" + randomSuffix
		searchScope  = "one"              // must be "one","sub", or "base"
		searchFilter = "(myAttr=myValue)" // must be a valid LDAP filter format

		nameAttr1  = "missing-name-attr"
		nameAttr2  = "a-display-name" + randomSuffix
		idAttr1    = "missing-id-attr"
		idAttr2    = "dn" // "dn" is a special value, so don't add a random suffix to make sure we handle it correctly
		emailAttr1 = "missing-attr"
		emailAttr2 = "c-mail" + randomSuffix
		loginAttr1 = "missing-attr"
		loginAttr2 = "d-mylogin" + randomSuffix

		myUserUID      = "myuser"
		myUserName     = "My User, Jr."
		myUserEmail    = "myuser@example.com"
		myUserDN       = searchAttr + "=" + myUserUID + "," + searchDN
		myUserPassword = "myuser-password-" + randomSuffix
	)

	expectedAttributes := [][]byte{}
	for _, attr := range kutil.NewStringSet(searchAttr, nameAttr1, nameAttr2, idAttr1, idAttr2, emailAttr1, emailAttr2, loginAttr1, loginAttr2).List() {
		expectedAttributes = append(expectedAttributes, []byte(attr))
	}
	expectedSearchRequest := ldapserver.SearchRequest{
		BaseObject:   []byte(searchDN),
		Scope:        ldapserver.SearchRequestSingleLevel,
		DerefAliases: 0,
		SizeLimit:    2,
		TimeLimit:    0,
		TypesOnly:    false,
		Attributes:   expectedAttributes,
		Filter:       fmt.Sprintf("(&%s(%s=%s))", searchFilter, searchAttr, myUserUID),
	}

	// Start LDAP server
	ldapAddress, err := testserver.FindAvailableBindAddress(8389, 8400)
	if err != nil {
		t.Fatalf("could not allocate LDAP bind address: %v", err)
	}
	ldapServer := testutil.NewTestLDAPServer()
	ldapServer.SetPassword(bindDN, bindPassword)
	ldapServer.Start(ldapAddress)
	defer ldapServer.Stop()

	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	masterOptions.OAuthConfig.IdentityProviders[0] = configapi.IdentityProvider{
		Name:            providerName,
		UseAsChallenger: true,
		UseAsLogin:      true,
		Provider: runtime.EmbeddedObject{
			Object: &configapi.LDAPPasswordIdentityProvider{
				URL:          fmt.Sprintf("ldap://%s/%s?%s?%s?%s", ldapAddress, searchDN, searchAttr, searchScope, searchFilter),
				BindDN:       bindDN,
				BindPassword: bindPassword,
				Insecure:     true,
				CA:           "",
				Attributes: configapi.LDAPAttributeMapping{
					ID:                []string{idAttr1, idAttr2},
					PreferredUsername: []string{loginAttr1, loginAttr2},
					Name:              []string{nameAttr1, nameAttr2},
					Email:             []string{emailAttr1, emailAttr2},
				},
			},
		},
	}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Use the server and CA info
	anonConfig := kclient.Config{}
	anonConfig.Host = clusterAdminClientConfig.Host
	anonConfig.CAFile = clusterAdminClientConfig.CAFile
	anonConfig.CAData = clusterAdminClientConfig.CAData

	// Make sure we can't authenticate as a missing user
	ldapServer.ResetRequests()
	if _, err := tokencmd.RequestToken(&anonConfig, nil, myUserUID, myUserPassword); err == nil {
		t.Error("Expected error, got none")
	}
	if len(ldapServer.BindRequests) != 1 {
		t.Errorf("Expected a single bind request for the search phase, got %d:\n%#v", len(ldapServer.BindRequests), ldapServer.BindRequests)
	}
	if len(ldapServer.SearchRequests) != 1 {
		t.Errorf("Expected a single search request, got %d:\n%#v", len(ldapServer.BindRequests), ldapServer.BindRequests)
	}

	// Add user
	ldapServer.SetPassword(myUserDN, myUserPassword)
	ldapServer.AddSearchResult(myUserDN, map[string]string{emailAttr2: myUserEmail, nameAttr2: myUserName, loginAttr2: myUserUID})

	// Make sure we can't authenticate with a bad password
	ldapServer.ResetRequests()
	if _, err := tokencmd.RequestToken(&anonConfig, nil, myUserUID, "badpassword"); err == nil {
		t.Error("Expected error, got none")
	}
	if len(ldapServer.BindRequests) != 2 {
		t.Errorf("Expected a bind request for the search phase and a failed bind request for the auth phase, got %d:\n%#v", len(ldapServer.BindRequests), ldapServer.BindRequests)
	}
	if len(ldapServer.SearchRequests) != 1 {
		t.Errorf("Expected a single search request, got %d:\n%#v", len(ldapServer.BindRequests), ldapServer.BindRequests)
	}

	// Make sure we can get a token with a good password
	ldapServer.ResetRequests()
	accessToken, err := tokencmd.RequestToken(&anonConfig, nil, myUserUID, myUserPassword)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(accessToken) == 0 {
		t.Errorf("Expected access token, got none")
	}
	if len(ldapServer.BindRequests) != 2 {
		t.Errorf("Expected a bind request for the search phase and a failed bind request for the auth phase, got %d:\n%#v", len(ldapServer.BindRequests), ldapServer.BindRequests)
	}
	if len(ldapServer.SearchRequests) != 1 {
		t.Errorf("Expected a single search request, got %d:\n%#v", len(ldapServer.BindRequests), ldapServer.BindRequests)
	}
	if !reflect.DeepEqual(expectedSearchRequest.BaseObject, ldapServer.SearchRequests[0].BaseObject) {
		t.Errorf("Expected search base DN\n\t%#v\ngot\n\t%#v",
			string(expectedSearchRequest.BaseObject),
			string(ldapServer.SearchRequests[0].BaseObject),
		)
	}
	if !reflect.DeepEqual(expectedSearchRequest.Filter, ldapServer.SearchRequests[0].Filter) {
		t.Errorf("Expected search filter\n\t%#v\ngot\n\t%#v",
			string(expectedSearchRequest.Filter),
			string(ldapServer.SearchRequests[0].Filter),
		)
	}
	{
		expectedAttrs := []string{}
		for _, a := range expectedSearchRequest.Attributes {
			expectedAttrs = append(expectedAttrs, string(a))
		}
		actualAttrs := []string{}
		for _, a := range ldapServer.SearchRequests[0].Attributes {
			actualAttrs = append(actualAttrs, string(a))
		}
		if !reflect.DeepEqual(expectedAttrs, actualAttrs) {
			t.Errorf("Expected search attributes\n\t%#v\ngot\n\t%#v", expectedAttrs, actualAttrs)
		}
	}

	// Make sure we can use the token, and it represents who we expect
	userConfig := anonConfig
	userConfig.BearerToken = accessToken
	userClient, err := client.New(&userConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	user, err := userClient.Users().Get("~")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if user.Name != myUserUID {
		t.Fatalf("Expected %s as the user, got %v", myUserUID, user)
	}

	// Make sure the identity got created and contained the mapped attributes
	identity, err := clusterAdminClient.Identities().Get(fmt.Sprintf("%s:%s", providerName, myUserDN))
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if identity.ProviderUserName != myUserDN {
		t.Errorf("Expected %q, got %q", myUserDN, identity.ProviderUserName)
	}
	if v := identity.Extra[authapi.IdentityDisplayNameKey]; v != myUserName {
		t.Errorf("Expected %q, got %q", myUserName, v)
	}
	if v := identity.Extra[authapi.IdentityPreferredUsernameKey]; v != myUserUID {
		t.Errorf("Expected %q, got %q", myUserUID, v)
	}
	if v := identity.Extra[authapi.IdentityEmailKey]; v != myUserEmail {
		t.Errorf("Expected %q, got %q", myUserEmail, v)
	}

}
