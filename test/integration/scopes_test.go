package integration

import (
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserverserviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/rest"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
	"github.com/openshift/origin/pkg/oauthserver/oauthserver"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func TestScopedTokens(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	projectName := "hammer-project"
	userName := "harold"
	_, haroldConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, projectName, userName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := buildclient.NewForConfigOrDie(haroldConfig).Build().Builds(projectName).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	haroldUser, err := userclient.NewForConfigOrDie(haroldConfig).Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	whoamiOnlyToken := &oauthapi.OAuthAccessToken{
		ObjectMeta: metav1.ObjectMeta{Name: "whoami-token-plus-some-padding-here-to-make-the-limit"},
		ClientName: oauthserver.OpenShiftCLIClientID,
		ExpiresIn:  200,
		Scopes:     []string{scope.UserInfo},
		UserName:   userName,
		UserUID:    string(haroldUser.UID),
	}
	if _, err := oauthclient.NewForConfigOrDie(clusterAdminClientConfig).OAuthAccessTokens().Create(whoamiOnlyToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	whoamiConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)
	whoamiConfig.BearerToken = whoamiOnlyToken.Name

	if _, err := buildclient.NewForConfigOrDie(whoamiConfig).Build().Builds(projectName).List(metav1.ListOptions{}); !kapierrors.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	user, err := userclient.NewForConfigOrDie(whoamiConfig).Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != userName {
		t.Fatalf("expected %v, got %v", userName, user.Name)
	}

	// try to impersonate a service account using this token
	whoamiConfig.Impersonate = rest.ImpersonationConfig{UserName: apiserverserviceaccount.MakeUsername(projectName, "default")}
	impersonatedUser, err := userclient.NewForConfigOrDie(whoamiConfig).Users().Get("~", metav1.GetOptions{})
	if !kapierrors.IsForbidden(err) {
		t.Fatalf("missing error: %v got user %#v", err, impersonatedUser)
	}
}

func TestScopedImpersonation(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminBuildClient := buildclient.NewForConfigOrDie(clusterAdminClientConfig)

	projectName := "hammer-project"
	userName := "harold"
	if _, _, err := testserver.CreateNewProject(clusterAdminClientConfig, projectName, userName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = clusterAdminBuildClient.Build().RESTClient().Get().
		SetHeader(authenticationv1.ImpersonateUserHeader, "harold").
		SetHeader(authenticationv1.ImpersonateUserExtraHeaderPrefix+authorizationapi.ScopesKey, "user:info").
		Namespace(projectName).Resource("builds").Name("name").Do().Into(&buildapi.Build{})
	if !kapierrors.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	user := &userapi.User{}
	err = userclient.NewForConfigOrDie(clusterAdminClientConfig).RESTClient().Get().
		SetHeader(authenticationv1.ImpersonateUserHeader, "harold").
		SetHeader(authenticationv1.ImpersonateUserExtraHeaderPrefix+authorizationapi.ScopesKey, "user:info").
		Resource("users").Name("~").Do().Into(user)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.Name != "harold" {
		t.Fatalf("expected %v, got %v", "harold", user.Name)
	}
}

func TestScopeEscalations(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminOAuthClient := oauthclient.NewForConfigOrDie(clusterAdminClientConfig)

	projectName := "hammer-project"
	userName := "harold"
	_, haroldConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, projectName, userName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := buildclient.NewForConfigOrDie(haroldConfig).Build().Builds(projectName).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	haroldUser, err := userclient.NewForConfigOrDie(haroldConfig).Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nonEscalatingEditToken := &oauthapi.OAuthAccessToken{
		ObjectMeta: metav1.ObjectMeta{Name: "non-escalating-edit-plus-some-padding-here-to-make-the-limit"},
		ClientName: oauthserver.OpenShiftCLIClientID,
		ExpiresIn:  200,
		Scopes:     []string{scope.ClusterRoleIndicator + "edit:*"},
		UserName:   userName,
		UserUID:    string(haroldUser.UID),
	}
	if _, err := clusterAdminOAuthClient.OAuthAccessTokens().Create(nonEscalatingEditToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nonEscalatingEditConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)
	nonEscalatingEditConfig.BearerToken = nonEscalatingEditToken.Name
	nonEscalatingEditClient, err := kclientset.NewForConfig(nonEscalatingEditConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := nonEscalatingEditClient.Core().Secrets(projectName).List(metav1.ListOptions{}); !kapierrors.IsForbidden(err) {
		t.Fatalf("unexpected error: %v", err)
	}

	escalatingEditToken := &oauthapi.OAuthAccessToken{
		ObjectMeta: metav1.ObjectMeta{Name: "escalating-edit-plus-some-padding-here-to-make-the-limit"},
		ClientName: oauthserver.OpenShiftCLIClientID,
		ExpiresIn:  200,
		Scopes:     []string{scope.ClusterRoleIndicator + "edit:*:!"},
		UserName:   userName,
		UserUID:    string(haroldUser.UID),
	}
	if _, err := clusterAdminOAuthClient.OAuthAccessTokens().Create(escalatingEditToken); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	escalatingEditConfig := rest.AnonymousClientConfig(clusterAdminClientConfig)
	escalatingEditConfig.BearerToken = escalatingEditToken.Name
	escalatingEditClient, err := kclientset.NewForConfig(escalatingEditConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := escalatingEditClient.Core().Secrets(projectName).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTokensWithIllegalScopes(t *testing.T) {
	masterConfig, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer testserver.CleanupMasterEtcd(t, masterConfig)

	clusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	clusterAdminOAuthClient := oauthclient.NewForConfigOrDie(clusterAdminClientConfig)

	client := &oauthapi.OAuthClient{
		ObjectMeta: metav1.ObjectMeta{Name: "testing-client"},
		ScopeRestrictions: []oauthapi.ScopeRestriction{
			{ExactValues: []string{"user:info"}},
			{
				ClusterRole: &oauthapi.ClusterRoleScopeRestriction{
					RoleNames:       []string{"one", "two"},
					Namespaces:      []string{"alfa", "bravo"},
					AllowEscalation: false,
				},
			},
		},
	}
	if _, err := clusterAdminOAuthClient.OAuthClients().Create(client); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clientAuthorizationTests := []struct {
		name string
		obj  *oauthapi.OAuthClientAuthorization
		fail bool
	}{
		{
			name: "no scopes",
			fail: true,
			obj: &oauthapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "testing-client"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
			},
		},
		{
			name: "denied literal",
			fail: true,
			obj: &oauthapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "testing-client"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
				Scopes:     []string{"user:info", "user:check-access"},
			},
		},
		{
			name: "denied role",
			fail: true,
			obj: &oauthapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "testing-client"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
				Scopes:     []string{"role:one:*"},
			},
		},
		{
			name: "ok role",
			obj: &oauthapi.OAuthClientAuthorization{
				ObjectMeta: metav1.ObjectMeta{Name: "testing-client"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
				Scopes:     []string{"role:one:bravo"},
			},
		},
	}
	for _, tc := range clientAuthorizationTests {
		_, err := clusterAdminOAuthClient.OAuthClientAuthorizations().Create(tc.obj)
		switch {
		case err == nil && !tc.fail:
		case err != nil && tc.fail:
		default:
			t.Errorf("%s: expected %v, got %v", tc.name, tc.fail, err)

		}
	}

	accessTokenTests := []struct {
		name string
		obj  *oauthapi.OAuthAccessToken
		fail bool
	}{
		{
			name: "no scopes",
			fail: true,
			obj: &oauthapi.OAuthAccessToken{
				ObjectMeta: metav1.ObjectMeta{Name: "tokenlongenoughtobecreatedwithoutfailing"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
			},
		},
		{
			name: "denied literal",
			fail: true,
			obj: &oauthapi.OAuthAccessToken{
				ObjectMeta: metav1.ObjectMeta{Name: "tokenlongenoughtobecreatedwithoutfailing"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
				Scopes:     []string{"user:info", "user:check-access"},
			},
		},
		{
			name: "denied role",
			fail: true,
			obj: &oauthapi.OAuthAccessToken{
				ObjectMeta: metav1.ObjectMeta{Name: "tokenlongenoughtobecreatedwithoutfailing"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
				Scopes:     []string{"role:one:*"},
			},
		},
		{
			name: "ok role",
			obj: &oauthapi.OAuthAccessToken{
				ObjectMeta: metav1.ObjectMeta{Name: "tokenlongenoughtobecreatedwithoutfailing"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
				Scopes:     []string{"role:one:bravo"},
			},
		},
	}
	for _, tc := range accessTokenTests {
		_, err := clusterAdminOAuthClient.OAuthAccessTokens().Create(tc.obj)
		switch {
		case err == nil && !tc.fail:
		case err != nil && tc.fail:
		default:
			t.Errorf("%s: expected %v, got %v", tc.name, tc.fail, err)

		}
	}

	authorizeTokenTests := []struct {
		name string
		obj  *oauthapi.OAuthAuthorizeToken
		fail bool
	}{
		{
			name: "no scopes",
			fail: true,
			obj: &oauthapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "tokenlongenoughtobecreatedwithoutfailing"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
			},
		},
		{
			name: "denied literal",
			fail: true,
			obj: &oauthapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "tokenlongenoughtobecreatedwithoutfailing"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
				Scopes:     []string{"user:info", "user:check-access"},
			},
		},
		{
			name: "denied role",
			fail: true,
			obj: &oauthapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "tokenlongenoughtobecreatedwithoutfailing"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
				Scopes:     []string{"role:one:*"},
			},
		},
		{
			name: "ok role",
			obj: &oauthapi.OAuthAuthorizeToken{
				ObjectMeta: metav1.ObjectMeta{Name: "tokenlongenoughtobecreatedwithoutfailing"},
				ClientName: client.Name,
				UserName:   "name",
				UserUID:    "uid",
				Scopes:     []string{"role:one:bravo"},
			},
		},
	}
	for _, tc := range authorizeTokenTests {
		_, err := clusterAdminOAuthClient.OAuthAuthorizeTokens().Create(tc.obj)
		switch {
		case err == nil && !tc.fail:
		case err != nil && tc.fail:
		default:
			t.Errorf("%s: expected %v, got %v", tc.name, tc.fail, err)

		}
	}

}
