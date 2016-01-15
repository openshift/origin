// +build integration

package integration

import (
	"io/ioutil"
	"os"
	"testing"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/client"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestOAuthHTPasswd(t *testing.T) {
	htpasswdFile, err := ioutil.TempFile("", "test.htpasswd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(htpasswdFile.Name())

	testutil.RequireEtcd(t)
	masterOptions, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	masterOptions.OAuthConfig.IdentityProviders[0] = configapi.IdentityProvider{
		Name:            "htpasswd",
		UseAsChallenger: true,
		UseAsLogin:      true,
		MappingMethod:   "claim",
		Provider: &configapi.HTPasswdPasswordIdentityProvider{
			File: htpasswdFile.Name(),
		},
	}

	clusterAdminKubeConfig, err := testserver.StartConfiguredMaster(masterOptions)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Use the server and CA info
	anonConfig := kclient.Config{}
	anonConfig.Host = clientConfig.Host
	anonConfig.CAFile = clientConfig.CAFile
	anonConfig.CAData = clientConfig.CAData

	// Make sure we can't authenticate
	if _, err := tokencmd.RequestToken(&anonConfig, nil, "username", "password"); err == nil {
		t.Error("Expected error, got none")
	}

	// Update the htpasswd file with output of `htpasswd -n -b username password`
	// And also a line that looks like /etc/shadow with root account and crypt('foobar', ...)
	userpass := "username:$apr1$4Ci5I8yc$85R9vc4fOgzAULsldiUuv.\nroot:$6$nbGgoytD$F7YVfCQVLgEw6JiM.64cvXrIfWZd4s2OL7URG3RIAB9YwZE1pdEyxvsG5U9/AmkAtfDDuCEXsff3/QKeZO4tQ0:16819:0:99999:7:::\n"
	ioutil.WriteFile(htpasswdFile.Name(), []byte(userpass), os.FileMode(0600))

	// Make sure we can get a token
	accessToken, err := tokencmd.RequestToken(&anonConfig, nil, "username", "password")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(accessToken) == 0 {
		t.Errorf("Expected access token, got none")
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
	if user.Name != "username" {
		t.Fatalf("Expected username as the user, got %v", user)
	}

	// Now try to get a token for the shadow style crypt() line
	rootToken, err := tokencmd.RequestToken(&anonConfig, nil, "root", "shadow")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(rootToken) == 0 {
		t.Errorf("Expected access token for root, got none")
	}

	// Make sure we can use the token, and it represents who we expect
	userConfig.BearerToken = rootToken
	rootClient, err := client.New(&userConfig)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	root, err := rootClient.Users().Get("~")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if root.Name != "root" {
		t.Fatalf("Expected root as the user, got %v", root)
	}
}
