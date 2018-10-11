package integration

import (
	"reflect"
	"strings"
	"testing"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	apiserverserviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	apiserveruser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/rest"
	rbacapiv1 "k8s.io/kubernetes/pkg/apis/rbac/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	rbacvalidation "k8s.io/kubernetes/pkg/registry/rbac/validation"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	projectapiv1 "github.com/openshift/api/project/v1"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	projectclient "github.com/openshift/client-go/project/clientset/versioned"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/apis/authorization/rbacconversion"
	authorizationapiv1 "github.com/openshift/origin/pkg/authorization/apis/authorization/v1"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/client/impersonatingclient"
	oauthapi "github.com/openshift/origin/pkg/oauth/apis/oauth"
	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset/typed/oauth/internalversion"
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

	if _, err := buildv1client.NewForConfigOrDie(haroldConfig).Build().Builds(projectName).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	haroldUser, err := userclient.NewForConfigOrDie(haroldConfig).Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	whoamiOnlyToken := &oauthapi.OAuthAccessToken{
		ObjectMeta: metav1.ObjectMeta{Name: "whoami-token-plus-some-padding-here-to-make-the-limit"},
		ClientName: "openshift-challenging-client",
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

	if _, err := buildv1client.NewForConfigOrDie(whoamiConfig).Build().Builds(projectName).List(metav1.ListOptions{}); !kapierrors.IsForbidden(err) {
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
	clusterAdminBuildClient := buildv1client.NewForConfigOrDie(clusterAdminClientConfig)

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

	if _, err := buildv1client.NewForConfigOrDie(haroldConfig).Build().Builds(projectName).List(metav1.ListOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	haroldUser, err := userclient.NewForConfigOrDie(haroldConfig).Users().Get("~", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nonEscalatingEditToken := &oauthapi.OAuthAccessToken{
		ObjectMeta: metav1.ObjectMeta{Name: "non-escalating-edit-plus-some-padding-here-to-make-the-limit"},
		ClientName: "openshift-challenging-client",
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
		ClientName: "openshift-challenging-client",
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
				ExpiresIn:  86400,
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
				ExpiresIn:  86400,
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
				ExpiresIn:  86400,
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
				ExpiresIn:  86400,
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

func TestUnknownScopes(t *testing.T) {
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
	_, haroldClientConfig, err := testserver.CreateNewProject(clusterAdminClientConfig, projectName, userName)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Here we test ScopesToVisibleNamespaces
	// we do this first so we wait for project and related data to appear in
	// the caches only once
	userInfo := apiserveruser.DefaultInfo{
		Name: userName,
		Extra: map[string][]string{
			authorizationapi.ScopesKey: {"user:list-projects", "bad"}}}
	impersonatingConfig := impersonatingclient.NewImpersonatingConfig(&userInfo, *clusterAdminClientConfig)
	projectClient := projectclient.NewForConfigOrDie(&impersonatingConfig)

	var projects *projectapiv1.ProjectList
	err = wait.Poll(100*time.Millisecond, 30*time.Second,
		func() (bool, error) {
			projects, err = projectClient.ProjectV1().Projects().List(metav1.ListOptions{})
			if err != nil {
				return false, err
			}
			return len(projects.Items) > 0, nil
		})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects.Items) == 0 {
		t.Fatalf("Timed out waiting for project")
	}
	if len(projects.Items) != 1 {
		t.Fatalf("Expected only 1 project, got %d", len(projects.Items))
	}
	if projects.Items[0].Name != projectName {
		t.Fatalf("Expected project named %s got %s", projectName, projects.Items[0].Name)
	}

	badScopesUserInfo := apiserveruser.DefaultInfo{
		Name: userName,
		Extra: map[string][]string{
			authorizationapi.ScopesKey: {"bad"}}}
	badScopesImpersonatingConfig := impersonatingclient.NewImpersonatingConfig(
		&badScopesUserInfo, *clusterAdminClientConfig)
	badScopesProjectClient := projectclient.NewForConfigOrDie(&badScopesImpersonatingConfig)
	projects, err = badScopesProjectClient.ProjectV1().Projects().List(metav1.ListOptions{})
	if err == nil {
		t.Fatalf("Expected forbidden error, but got no error")
	}
	expectedError := "scopes [bad] prevent this action, additionally the following non-fatal errors were reported"
	if !kapierrors.IsForbidden(err) || !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("Expected error '%s', but the returned error was '%v'", expectedError, err)
	}

	// And here we test ScopesToRules interactons
	authzv1client := authorizationv1typedclient.NewForConfigOrDie(haroldClientConfig)

	// Test with valid scopes as baseline
	rulesReview := &authorizationv1.SelfSubjectRulesReview{
		Spec: authorizationv1.SelfSubjectRulesReviewSpec{
			Scopes: []string{"user:info", "role:admin:*"},
		},
	}
	referenceRulesReviewObj, err := authzv1client.SelfSubjectRulesReviews(projectName).Create(rulesReview)
	if err != nil {
		t.Fatalf("unexpected error getting SelfSubjectRulesReview: %v", err)
	}
	if len(referenceRulesReviewObj.Status.Rules) == 0 {
		t.Fatalf("Expected some rules for user harold")
	}

	// Try adding bad scopes and check we have the same perms as baseline
	rulesReviewWithBad := &authorizationv1.SelfSubjectRulesReview{
		Spec: authorizationv1.SelfSubjectRulesReviewSpec{
			Scopes: []string{"user:info", "role:admin:*", "user:bad", "role:bad", "bad"},
		},
	}
	rulesReviewWithBadObj, err := authzv1client.SelfSubjectRulesReviews(projectName).Create(rulesReviewWithBad)
	if err != nil {
		t.Fatalf("unexpected error getting SelfSubjectRulesReview: %v", err)
	}

	//Check same rules
	if !reflect.DeepEqual(referenceRulesReviewObj.Status.Rules,
		rulesReviewWithBadObj.Status.Rules) {
		t.Fatalf("Expected rules: '%v', got '%v'",
			referenceRulesReviewObj.Status.Rules,
			rulesReviewWithBadObj.Status.Rules)
	}

	// Make sure no rules (beyond baseline) when only bad scopes are present
	rulesReviewOnlyBad := &authorizationv1.SelfSubjectRulesReview{
		Spec: authorizationv1.SelfSubjectRulesReviewSpec{
			Scopes: []string{"user:bad", "role:bad", "bad"},
		},
	}
	rulesReviewOnlyBadObj, err := authzv1client.SelfSubjectRulesReviews(projectName).Create(rulesReviewOnlyBad)
	if err != nil {
		t.Fatalf("unexpected error getting SelfSubjectRulesReview: %v", err)
	}

	//Check same rules
	rbacv1Rules, err := authzv1_To_rbacv1_PolicyRules(rulesReviewOnlyBadObj.Status.Rules)
	if err != nil {
		t.Fatalf("Unexpected error converting rules: %v", err)
	}

	// finally we can try covers both ways to assure the rules are
	// semantically identical
	equal, diffRules := checkEqualRules(rbacv1Rules, []rbacv1.PolicyRule{authorizationapi.DiscoveryRule})
	if !equal {
		t.Fatalf("Unmatching Rules when using unknown scopes: %v", diffRules)
	}

}

//convert SSRR result: authzv1 -> authz -> rbac -> rbacv1
func authzv1_To_rbacv1_PolicyRules(authzv1Rules []authorizationv1.PolicyRule) ([]rbacv1.PolicyRule, error) {
	authzRules := make([]authorizationapi.PolicyRule, len(authzv1Rules))
	for index := range authzv1Rules {
		err := authorizationapiv1.Convert_v1_PolicyRule_To_authorization_PolicyRule(&authzv1Rules[index], &authzRules[index], nil)
		if err != nil {
			return nil, err
		}
	}

	rbacRules := rbacconversion.Convert_api_PolicyRules_To_rbac_PolicyRules(authzRules)
	rbacv1Rules := make([]rbacv1.PolicyRule, len(rbacRules))
	for index := range rbacRules {
		err := rbacapiv1.Convert_rbac_PolicyRule_To_v1_PolicyRule(&rbacRules[index], &rbacv1Rules[index], nil)
		if err != nil {
			return nil, err
		}
	}

	return rbacv1Rules, nil
}

func checkEqualRules(a, b []rbacv1.PolicyRule) (bool, []rbacv1.PolicyRule) {
	covers, diffRules := rbacvalidation.Covers(a, b)
	if covers {
		covers, diffRules = rbacvalidation.Covers(b, a)
	}
	return covers, diffRules
}
