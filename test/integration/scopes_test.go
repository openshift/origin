// +build integration

package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/serviceaccount"

	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestScopedTokens(t *testing.T) {
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

	projectName := "hammer-project"
	userName := "harold"
	haroldClient, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projectName, userName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := haroldClient.Builds(projectName).List(kapi.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	haroldUser, err := haroldClient.Users().Get("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	whoamiOnlyToken := &oauthapi.OAuthAccessToken{
		ObjectMeta: kapi.ObjectMeta{Name: "whoami-token-plus-some-padding-here-to-make-the-limit"},
		ClientName: "any-client",
		ExpiresIn:  200,
		Scopes:     []string{scope.UserIndicator + scope.UserInfo},
		UserName:   userName,
		UserUID:    string(haroldUser.UID),
	}
	if _, err := clusterAdminClient.OAuthAccessTokens().Create(whoamiOnlyToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	whoamiConfig := clientcmd.AnonymousClientConfig(clusterAdminClientConfig)
	whoamiConfig.BearerToken = whoamiOnlyToken.Name
	whoamiClient, err := client.New(&whoamiConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := whoamiClient.Builds(projectName).List(kapi.ListOptions{}); !kapierrors.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	user, err := whoamiClient.Users().Get("~")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != userName {
		t.Fatalf("expected %v, got %v", userName, user.Name)
	}

	// try to impersonate a service account using this token
	whoamiConfig.Impersonate = serviceaccount.MakeUsername(projectName, "default")
	impersonatingClient, err := client.New(&whoamiConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	impersonatedUser, err := impersonatingClient.Users().Get("~")
	if !kapierrors.IsForbidden(err) {
		t.Fatalf("missing error: %v got user %#v", err, impersonatedUser)
	}
}
