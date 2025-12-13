package authorization

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	apiserverserviceaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	apiserveruser "k8s.io/apiserver/pkg/authentication/user"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/transport"
	rbacvalidation "k8s.io/component-helpers/auth/rbac/validation"

	authorizationv1 "github.com/openshift/api/authorization/v1"
	buildv1 "github.com/openshift/api/build/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	projectapiv1 "github.com/openshift/api/project/v1"
	userv1 "github.com/openshift/api/user/v1"
	"github.com/openshift/apiserver-library-go/pkg/authorization/scope"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	oauthv1client "github.com/openshift/client-go/oauth/clientset/versioned/typed/oauth/v1"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	exutil "github.com/openshift/origin/test/extended/util"

	g "github.com/onsi/ginkgo/v2"
)

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] scopes", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("scopes")

	g.Describe("TestScopedTokens", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:oauth.openshift.io][apigroup:build.openshift.io]"), g.Label("Size:S"), func() {
			t := g.GinkgoT()

			projectName := oc.Namespace()
			userName := oc.CreateUser("harold-").Name
			AddUserAdminToProject(oc, projectName, userName)
			haroldConfig := oc.GetClientConfigForUser(userName)

			if _, err := buildv1client.NewForConfigOrDie(haroldConfig).BuildV1().Builds(projectName).List(context.Background(), metav1.ListOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			haroldUser, err := userv1client.NewForConfigOrDie(haroldConfig).Users().Get(context.Background(), "~", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			whoamiOnlyTokenStr, sha256WhoamiOnlyTokenStr := exutil.GenerateOAuthTokenPair()
			whoamiOnlyToken := &oauthv1.OAuthAccessToken{
				ObjectMeta:  metav1.ObjectMeta{Name: sha256WhoamiOnlyTokenStr},
				ClientName:  "openshift-challenging-client",
				ExpiresIn:   200,
				Scopes:      []string{scope.UserInfo},
				UserName:    userName,
				UserUID:     string(haroldUser.UID),
				RedirectURI: "https://localhost:8443/oauth/token/implicit",
			}
			if _, err := oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Create(context.Background(), whoamiOnlyToken, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthaccesstokens"), whoamiOnlyToken)

			whoamiConfig := rest.AnonymousClientConfig(oc.AdminConfig())
			whoamiConfig.BearerToken = whoamiOnlyTokenStr

			if _, err := buildv1client.NewForConfigOrDie(whoamiConfig).BuildV1().Builds(projectName).List(context.Background(), metav1.ListOptions{}); !kapierrors.IsForbidden(err) {
				t.Fatalf("unexpected error: %v", err)
			}

			user, err := userv1client.NewForConfigOrDie(whoamiConfig).Users().Get(context.Background(), "~", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user.Name != userName {
				t.Fatalf("expected %v, got %v", userName, user.Name)
			}

			// try to impersonate a service account using this token
			whoamiConfig.Impersonate = rest.ImpersonationConfig{UserName: apiserverserviceaccount.MakeUsername(projectName, "default")}
			impersonatedUser, err := userv1client.NewForConfigOrDie(whoamiConfig).Users().Get(context.Background(), "~", metav1.GetOptions{})
			if !kapierrors.IsForbidden(err) {
				t.Fatalf("missing error: %v got user %#v", err, impersonatedUser)
			}
		})
	})
})

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] scopes", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("scopes")

	g.Describe("TestScopedImpersonation", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:build.openshift.io]"), g.Label("Size:S"), func() {
			t := g.GinkgoT()

			projectName := oc.Namespace()
			userName := oc.CreateUser("harold-").Name
			AddUserAdminToProject(oc, projectName, userName)

			err := oc.AdminBuildClient().BuildV1().RESTClient().Get().
				SetHeader(authenticationv1.ImpersonateUserHeader, userName).
				SetHeader(authenticationv1.ImpersonateUserExtraHeaderPrefix+authorizationv1.ScopesKey, "user:info").
				Namespace(projectName).Resource("builds").Name("name").Do(context.Background()).Into(&buildv1.Build{})
			if !kapierrors.IsForbidden(err) {
				t.Fatalf("unexpected error: %v", err)
			}

			user := &userv1.User{}
			err = oc.AdminUserClient().UserV1().RESTClient().Get().
				SetHeader(authenticationv1.ImpersonateUserHeader, userName).
				SetHeader(authenticationv1.ImpersonateUserExtraHeaderPrefix+authorizationv1.ScopesKey, "user:info").
				Resource("users").Name("~").Do(context.Background()).Into(user)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if user.Name != userName {
				t.Fatalf("expected %v, got %v", userName, user.Name)
			}
		})
	})
})

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] scopes", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("scopes")

	g.Describe("TestScopeEscalations", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:build.openshift.io][apigroup:oauth.openshift.io]"), g.Label("Size:S"), func() {
			t := g.GinkgoT()

			projectName := oc.Namespace()
			userName := oc.CreateUser("harold-").Name
			AddUserAdminToProject(oc, projectName, userName)
			haroldConfig := oc.GetClientConfigForUser(userName)

			if _, err := buildv1client.NewForConfigOrDie(haroldConfig).BuildV1().Builds(projectName).List(context.Background(), metav1.ListOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			haroldUser, err := userv1client.NewForConfigOrDie(haroldConfig).Users().Get(context.Background(), "~", metav1.GetOptions{})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			nonEscalatingEditTokenStr, sha256NonEscalatingEditTokenStr := exutil.GenerateOAuthTokenPair()
			nonEscalatingEditToken := &oauthv1.OAuthAccessToken{
				ObjectMeta:  metav1.ObjectMeta{Name: sha256NonEscalatingEditTokenStr},
				ClientName:  "openshift-challenging-client",
				ExpiresIn:   200,
				Scopes:      []string{scope.ClusterRoleIndicator + "edit:*"},
				UserName:    userName,
				UserUID:     string(haroldUser.UID),
				RedirectURI: "https://localhost:8443/oauth/token/implicit",
			}
			if _, err := oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Create(context.Background(), nonEscalatingEditToken, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthaccesstokens"), nonEscalatingEditToken)

			nonEscalatingEditConfig := rest.AnonymousClientConfig(oc.AdminConfig())
			nonEscalatingEditConfig.BearerToken = nonEscalatingEditTokenStr
			nonEscalatingEditClient, err := kclientset.NewForConfig(nonEscalatingEditConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if _, err := nonEscalatingEditClient.CoreV1().Secrets(projectName).List(context.Background(), metav1.ListOptions{}); !kapierrors.IsForbidden(err) {
				t.Fatalf("unexpected error: %v", err)
			}

			escalatingEditTokenStr, sha256EscalatingEditToken := exutil.GenerateOAuthTokenPair()
			escalatingEditToken := &oauthv1.OAuthAccessToken{
				ObjectMeta:  metav1.ObjectMeta{Name: sha256EscalatingEditToken},
				ClientName:  "openshift-challenging-client",
				ExpiresIn:   200,
				Scopes:      []string{scope.ClusterRoleIndicator + "edit:*:!"},
				UserName:    userName,
				UserUID:     string(haroldUser.UID),
				RedirectURI: "https://localhost:8443/oauth/token/implicit",
			}
			if _, err := oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Create(context.Background(), escalatingEditToken, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthaccesstokens"), escalatingEditToken)

			escalatingEditConfig := rest.AnonymousClientConfig(oc.AdminConfig())
			escalatingEditConfig.BearerToken = escalatingEditTokenStr
			escalatingEditClient, err := kclientset.NewForConfig(escalatingEditConfig)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if _, err := escalatingEditClient.CoreV1().Secrets(projectName).List(context.Background(), metav1.ListOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	})
})

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] scopes", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("scopes")

	g.Describe("TestTokensWithIllegalScopes", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:oauth.openshift.io]"), g.Label("Size:S"), func() {
			t := g.GinkgoT()

			clusterAdminClientConfig := oc.AdminConfig()
			clusterAdminOAuthClient := oauthv1client.NewForConfigOrDie(clusterAdminClientConfig)

			client := &oauthv1.OAuthClient{
				ObjectMeta: metav1.ObjectMeta{Name: "testing-client-" + oc.Namespace()},
				ScopeRestrictions: []oauthv1.ScopeRestriction{
					{ExactValues: []string{"user:info"}},
					{
						ClusterRole: &oauthv1.ClusterRoleScopeRestriction{
							RoleNames:       []string{"one", "two"},
							Namespaces:      []string{"alfa", "bravo"},
							AllowEscalation: false,
						},
					},
				},
				GrantMethod: oauthv1.GrantHandlerAuto,
			}
			if _, err := clusterAdminOAuthClient.OAuthClients().Create(context.Background(), client, metav1.CreateOptions{}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthclients"), client)

			clientAuthorizationTests := []struct {
				name string
				obj  *oauthv1.OAuthClientAuthorization
				fail bool
			}{
				{
					name: "no scopes",
					fail: true,
					obj: &oauthv1.OAuthClientAuthorization{
						ObjectMeta: metav1.ObjectMeta{Name: "testing-client-" + oc.Namespace()},
						ClientName: client.Name,
						UserName:   "name",
						UserUID:    "uid",
					},
				},
				{
					name: "denied literal",
					fail: true,
					obj: &oauthv1.OAuthClientAuthorization{
						ObjectMeta: metav1.ObjectMeta{Name: "testing-client-" + oc.Namespace()},
						ClientName: client.Name,
						UserName:   "name",
						UserUID:    "uid",
						Scopes:     []string{"user:info", "user:check-access"},
					},
				},
				{
					name: "denied role",
					fail: true,
					obj: &oauthv1.OAuthClientAuthorization{
						ObjectMeta: metav1.ObjectMeta{Name: "testing-client-" + oc.Namespace()},
						ClientName: client.Name,
						UserName:   "name",
						UserUID:    "uid",
						Scopes:     []string{"role:one:*"},
					},
				},
				{
					name: "ok role",
					obj: &oauthv1.OAuthClientAuthorization{
						ObjectMeta: metav1.ObjectMeta{Name: "testing-client-" + oc.Namespace()},
						ClientName: client.Name,
						UserName:   "name",
						UserUID:    "uid",
						Scopes:     []string{"role:one:bravo"},
					},
				},
			}
			for _, tc := range clientAuthorizationTests {
				_, err := clusterAdminOAuthClient.OAuthClientAuthorizations().Create(context.Background(), tc.obj, metav1.CreateOptions{})
				if err == nil {
					oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthclientauthorizations"), tc.obj)
				}
				switch {
				case err == nil && !tc.fail:
				case err != nil && tc.fail:
				default:
					t.Errorf("%s: expected %v, got %v", tc.name, tc.fail, err)
				}
			}

			accessTokenTests := []struct {
				name string
				obj  *oauthv1.OAuthAccessToken
				fail bool
			}{
				{
					name: "no scopes",
					fail: true,
					obj: &oauthv1.OAuthAccessToken{
						ObjectMeta:  metav1.ObjectMeta{Name: "sha256~tokenlongenoughtobecreatedwithoutfailing-" + oc.Namespace()},
						ClientName:  client.Name,
						UserName:    "name",
						UserUID:     "uid",
						RedirectURI: "https://localhost:8443/oauth/token/implicit",
					},
				},
				{
					name: "denied literal",
					fail: true,
					obj: &oauthv1.OAuthAccessToken{
						ObjectMeta:  metav1.ObjectMeta{Name: "sha256~tokenlongenoughtobecreatedwithoutfailing-" + oc.Namespace()},
						ClientName:  client.Name,
						UserName:    "name",
						UserUID:     "uid",
						Scopes:      []string{"user:info", "user:check-access"},
						RedirectURI: "https://localhost:8443/oauth/token/implicit",
					},
				},
				{
					name: "denied role",
					fail: true,
					obj: &oauthv1.OAuthAccessToken{
						ObjectMeta:  metav1.ObjectMeta{Name: "sha256~tokenlongenoughtobecreatedwithoutfailing-" + oc.Namespace()},
						ClientName:  client.Name,
						UserName:    "name",
						UserUID:     "uid",
						Scopes:      []string{"role:one:*"},
						RedirectURI: "https://localhost:8443/oauth/token/implicit",
					},
				},
				{
					name: "ok role",
					obj: &oauthv1.OAuthAccessToken{
						ObjectMeta:  metav1.ObjectMeta{Name: "sha256~tokenlongenoughtobecreatedwithoutfailing-" + oc.Namespace()},
						ClientName:  client.Name,
						UserName:    "name",
						UserUID:     "uid",
						Scopes:      []string{"role:one:bravo"},
						RedirectURI: "https://localhost:8443/oauth/token/implicit",
					},
				},
			}
			for _, tc := range accessTokenTests {
				_, err := clusterAdminOAuthClient.OAuthAccessTokens().Create(context.Background(), tc.obj, metav1.CreateOptions{})
				if err == nil {
					oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthaccesstokens"), tc.obj)
				}
				switch {
				case err == nil && !tc.fail:
				case err != nil && tc.fail:
				default:
					t.Errorf("%s: expected %v, got %v", tc.name, tc.fail, err)

				}
			}

			authorizeTokenTests := []struct {
				name string
				obj  *oauthv1.OAuthAuthorizeToken
				fail bool
			}{
				{
					name: "no scopes",
					fail: true,
					obj: &oauthv1.OAuthAuthorizeToken{
						ObjectMeta:  metav1.ObjectMeta{Name: "sha256~tokenlongenoughtobecreatedwithoutfailing-" + oc.Namespace()},
						ClientName:  client.Name,
						ExpiresIn:   86400,
						UserName:    "name",
						UserUID:     "uid",
						RedirectURI: "https://localhost:8443/oauth/token/implicit",
					},
				},
				{
					name: "denied literal",
					fail: true,
					obj: &oauthv1.OAuthAuthorizeToken{
						ObjectMeta:  metav1.ObjectMeta{Name: "sha256~tokenlongenoughtobecreatedwithoutfailing-" + oc.Namespace()},
						ClientName:  client.Name,
						ExpiresIn:   86400,
						UserName:    "name",
						UserUID:     "uid",
						Scopes:      []string{"user:info", "user:check-access"},
						RedirectURI: "https://localhost:8443/oauth/token/implicit",
					},
				},
				{
					name: "denied role",
					fail: true,
					obj: &oauthv1.OAuthAuthorizeToken{
						ObjectMeta:  metav1.ObjectMeta{Name: "sha256~tokenlongenoughtobecreatedwithoutfailing-" + oc.Namespace()},
						ClientName:  client.Name,
						ExpiresIn:   86400,
						UserName:    "name",
						UserUID:     "uid",
						Scopes:      []string{"role:one:*"},
						RedirectURI: "https://localhost:8443/oauth/token/implicit",
					},
				},
				{
					name: "ok role",
					obj: &oauthv1.OAuthAuthorizeToken{
						ObjectMeta:  metav1.ObjectMeta{Name: "sha256~tokenlongenoughtobecreatedwithoutfailing-" + oc.Namespace()},
						ClientName:  client.Name,
						ExpiresIn:   86400,
						UserName:    "name",
						UserUID:     "uid",
						Scopes:      []string{"role:one:bravo"},
						RedirectURI: "https://localhost:8443/oauth/token/implicit",
					},
				},
			}
			for _, tc := range authorizeTokenTests {
				_, err := clusterAdminOAuthClient.OAuthAuthorizeTokens().Create(context.Background(), tc.obj, metav1.CreateOptions{})
				if err == nil {
					oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthauthorizetokens"), tc.obj)
				}
				switch {
				case err == nil && !tc.fail:
				case err != nil && tc.fail:
				default:
					t.Errorf("%s: expected %v, got %v", tc.name, tc.fail, err)

				}
			}

		})
	})
})

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] scopes", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("scopes")

	g.Describe("TestUnknownScopes", func() {
		g.It(fmt.Sprintf("should succeed [apigroup:user.openshift.io][apigroup:authorization.openshift.io][apigroup:project.openshift.io]"), g.Label("Size:S"), func() {
			t := g.GinkgoT()

			clusterAdminClientConfig := oc.AdminConfig()
			projectName := oc.Namespace()
			userName := oc.CreateUser("harold-").Name
			AddUserAdminToProject(oc, projectName, userName)
			haroldClientConfig := oc.GetClientConfigForUser(userName)

			// Here we test ScopesToVisibleNamespaces
			// we do this first so we wait for project and related data to appear in
			// the caches only once
			userInfo := apiserveruser.DefaultInfo{
				Name: userName,
				Extra: map[string][]string{
					authorizationv1.ScopesKey: {"user:list-projects", "bad"}}}
			impersonatingConfig := newImpersonatingConfig(&userInfo, *clusterAdminClientConfig)
			projectClient := projectv1client.NewForConfigOrDie(&impersonatingConfig)

			var projects *projectapiv1.ProjectList
			err := wait.Poll(100*time.Millisecond, 30*time.Second,
				func() (bool, error) {
					var err error
					projects, err = projectClient.Projects().List(context.Background(), metav1.ListOptions{})
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
					authorizationv1.ScopesKey: {"bad"}}}
			badScopesImpersonatingConfig := newImpersonatingConfig(
				&badScopesUserInfo, *clusterAdminClientConfig)
			badScopesProjectClient := projectv1client.NewForConfigOrDie(&badScopesImpersonatingConfig)
			projects, err = badScopesProjectClient.Projects().List(context.Background(), metav1.ListOptions{})
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
			referenceRulesReviewObj, err := authzv1client.SelfSubjectRulesReviews(projectName).Create(context.Background(), rulesReview, metav1.CreateOptions{})
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
			rulesReviewWithBadObj, err := authzv1client.SelfSubjectRulesReviews(projectName).Create(context.Background(), rulesReviewWithBad, metav1.CreateOptions{})
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
			rulesReviewOnlyBadObj, err := authzv1client.SelfSubjectRulesReviews(projectName).Create(context.Background(), rulesReviewOnlyBad, metav1.CreateOptions{})
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
			equal, diffRules := checkEqualRules(rbacv1Rules, []rbacv1.PolicyRule{scopeDiscoveryRule})
			if !equal {
				t.Fatalf("Unmatching Rules when using unknown scopes: %v", diffRules)
			}
		})
	})
})

// scopeDiscoveryRule is a rule that allows a client to discover the API resources available on this server
var scopeDiscoveryRule = rbacv1.PolicyRule{
	Verbs: []string{"get"},
	NonResourceURLs: []string{
		// Server version checking
		"/version", "/version/*",

		// API discovery/negotiation
		"/api", "/api/*",
		"/apis", "/apis/*",
		"/oapi", "/oapi/*",
		"/openapi/v2",
		"/swaggerapi", "/swaggerapi/*", "/swagger.json", "/swagger-2.0.0.pb-v1",
		"/osapi", "/osapi/", // these cannot be removed until we can drop support for pre 3.1 clients
		"/.well-known", "/.well-known/*",

		// we intentionally allow all to here
		"/",
	},
}

// convert SSRR result.  This works well enough for the test
func authzv1_To_rbacv1_PolicyRules(authzv1Rules []authorizationv1.PolicyRule) ([]rbacv1.PolicyRule, error) {
	ret := []rbacv1.PolicyRule{}

	for _, curr := range authzv1Rules {
		ret = append(ret, rbacv1.PolicyRule{
			APIGroups:       curr.APIGroups,
			ResourceNames:   curr.ResourceNames,
			Resources:       curr.Resources,
			Verbs:           curr.Verbs,
			NonResourceURLs: curr.NonResourceURLsSlice,
		})
	}

	return ret, nil
}

func checkEqualRules(a, b []rbacv1.PolicyRule) (bool, []rbacv1.PolicyRule) {
	covers, diffRules := rbacvalidation.Covers(a, b)
	if covers {
		covers, diffRules = rbacvalidation.Covers(b, a)
	}
	return covers, diffRules
}

// newImpersonatingConfig wraps the config's transport to impersonate a user, including user, groups, and scopes
func newImpersonatingConfig(user user.Info, config rest.Config) rest.Config {
	oldWrapTransport := config.WrapTransport
	if oldWrapTransport == nil {
		oldWrapTransport = func(rt http.RoundTripper) http.RoundTripper { return rt }
	}
	newConfig := transport.ImpersonationConfig{
		UserName: user.GetName(),
		Groups:   user.GetGroups(),
		Extra:    user.GetExtra(),
	}
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return transport.NewImpersonatingRoundTripper(newConfig, oldWrapTransport(rt))
	}
	return config
}
