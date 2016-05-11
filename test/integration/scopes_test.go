// +build integration

package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/serviceaccount"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	userapi "github.com/openshift/origin/pkg/user/api"
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

func TestScopedImpersonation(t *testing.T) {
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
	if _, err := testserver.CreateNewProject(clusterAdminClient, *clusterAdminClientConfig, projectName, userName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = clusterAdminClient.Get().
		SetHeader(authenticationapi.ImpersonateUserHeader, "harold").
		SetHeader(authenticationapi.ImpersonateUserScopeHeader, "user:info").
		Namespace(projectName).Resource("builds").Name("name").Do().Into(&buildapi.Build{})
	if !kapierrors.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	user := &userapi.User{}
	err = clusterAdminClient.Get().
		SetHeader(authenticationapi.ImpersonateUserHeader, "harold").
		SetHeader(authenticationapi.ImpersonateUserScopeHeader, "user:info").
		Resource("users").Name("~").Do().Into(user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != "harold" {
		t.Fatalf("expected %v, got %v", "harold", user.Name)
	}
}

func TestScopeEscalations(t *testing.T) {
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

	nonEscalatingEditToken := &oauthapi.OAuthAccessToken{
		ObjectMeta: kapi.ObjectMeta{Name: "non-escalating-edit-plus-some-padding-here-to-make-the-limit"},
		ClientName: "any-client",
		ExpiresIn:  200,
		Scopes:     []string{scope.ClusterRoleIndicator + "edit:*"},
		UserName:   userName,
		UserUID:    string(haroldUser.UID),
	}
	if _, err := clusterAdminClient.OAuthAccessTokens().Create(nonEscalatingEditToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nonEscalatingEditConfig := clientcmd.AnonymousClientConfig(clusterAdminClientConfig)
	nonEscalatingEditConfig.BearerToken = nonEscalatingEditToken.Name
	nonEscalatingEditClient, err := kclient.New(&nonEscalatingEditConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := nonEscalatingEditClient.Secrets(projectName).List(kapi.ListOptions{}); !kapierrors.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	escalatingEditToken := &oauthapi.OAuthAccessToken{
		ObjectMeta: kapi.ObjectMeta{Name: "escalating-edit-plus-some-padding-here-to-make-the-limit"},
		ClientName: "any-client",
		ExpiresIn:  200,
		Scopes:     []string{scope.ClusterRoleIndicator + "edit:*:!"},
		UserName:   userName,
		UserUID:    string(haroldUser.UID),
	}
	if _, err := clusterAdminClient.OAuthAccessTokens().Create(escalatingEditToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	escalatingEditConfig := clientcmd.AnonymousClientConfig(clusterAdminClientConfig)
	escalatingEditConfig.BearerToken = escalatingEditToken.Name
	escalatingEditClient, err := kclient.New(&escalatingEditConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := escalatingEditClient.Secrets(projectName).List(kapi.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
