// +build integration,!no-etcd

package integration

import (
	"io/ioutil"
	"os"
	"testing"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"github.com/openshift/origin/test/util"
)

func TestHTPasswd(t *testing.T) {
	htpasswdFile, err := ioutil.TempFile("", "test.htpasswd")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer os.Remove(htpasswdFile.Name())

	// This is all that should be needed to enable htpasswd-based auth
	// If these change, need to update documentation at http://docs.openshift.org/latest/architecture/authentication.html
	os.Setenv("OPENSHIFT_OAUTH_PASSWORD_AUTH", "htpasswd")
	os.Setenv("OPENSHIFT_OAUTH_HTPASSWD_FILE", htpasswdFile.Name())

	_, clusterAdminKubeConfig, err := util.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clientConfig, err := util.GetClusterAdminClientConfig(clusterAdminKubeConfig)
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
		t.Errorf("Expected error, got none", err)
	}

	// Update the htpasswd file with output of `htpasswd -n -b username password`
	userpass := "username:$apr1$4Ci5I8yc$85R9vc4fOgzAULsldiUuv."
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
	if user.Name != "htpasswd:username" {
		t.Fatalf("Expected htpasswd:username as the user, got %v", user)
	}
}
