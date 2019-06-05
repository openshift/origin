package rbac

import (
	"context"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kuser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	kauthenticationapi "k8s.io/kubernetes/pkg/apis/authentication"
	kauthorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	"k8s.io/kubernetes/pkg/apis/storage"
	"k8s.io/kubernetes/pkg/registry/rbac/validation"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/api/authorization"
	"github.com/openshift/api/build"
	"github.com/openshift/api/oauth"
	"github.com/openshift/api/project"
	"github.com/openshift/api/user"
	"github.com/openshift/origin/pkg/api/legacy"
	"github.com/openshift/origin/pkg/cmd/openshift-apiserver/openshiftapiserver"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	exutil "github.com/openshift/origin/test/extended/util"
)

// copied from bootstrap policy
var (
	read = []string{"get", "list", "watch"}

	rbacGroup    = rbac.GroupName
	storageGroup = storage.GroupName
	kAuthzGroup  = kauthorizationapi.GroupName
	kAuthnGroup  = kauthenticationapi.GroupName

	authzGroup   = authorization.GroupName
	buildGroup   = build.GroupName
	oauthGroup   = oauth.GroupName
	projectGroup = project.GroupName
	userGroup    = user.GroupName

	legacyAuthzGroup   = legacy.GroupName
	legacyBuildGroup   = legacy.GroupName
	legacyProjectGroup = legacy.GroupName
	legacyUserGroup    = legacy.GroupName
	legacyOauthGroup   = legacy.GroupName
)

// copied from various cluster roles in bootstrap policy
var (
	allUnauthenticatedRules = []rbacv1.PolicyRule{
		rbacv1helpers.NewRule("get", "create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/webhooks").RuleOrDie(),

		rbacv1helpers.NewRule("impersonate").Groups(kAuthnGroup).Resources("userextras/scopes.authorization.openshift.io").RuleOrDie(),

		rbacv1helpers.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("selfsubjectrulesreviews").RuleOrDie(),

		rbacv1helpers.NewRule("create").Groups(kAuthzGroup).Resources("selfsubjectaccessreviews", "selfsubjectrulesreviews").RuleOrDie(),

		rbacv1helpers.NewRule("delete").Groups(oauthGroup, legacyOauthGroup).Resources("oauthaccesstokens", "oauthauthorizetokens").RuleOrDie(),

		rbacv1helpers.NewRule("get").URLs(
			"/healthz/",
			"/version/*",
			"/oapi", "/oapi/*",
			"/swaggerapi", "/swaggerapi/*", "/swagger.json", "/swagger-2.0.0.pb-v1",
			"/osapi", "/osapi/",
			"/.well-known", "/.well-known/*",
			"/",
		).RuleOrDie(),

		rbacv1helpers.NewRule("get").URLs(
			"/readyz",
		).RuleOrDie(),

		rbacv1helpers.NewRule("get").URLs(
			"/healthz",
			"/version",
			"/openapi", "/openapi/*",
			"/api", "/api/*",
			"/apis", "/apis/*",
		).RuleOrDie(),
	}

	allAuthenticatedRules = append(
		[]rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(bootstrappolicy.DockerBuildResource, bootstrappolicy.OptimizedDockerBuildResource).RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(bootstrappolicy.JenkinsPipelineBuildResource).RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources(bootstrappolicy.SourceBuildResource).RuleOrDie(),

			rbacv1helpers.NewRule("get").Groups(userGroup, legacyUserGroup).Resources("users").Names("~").RuleOrDie(),
			rbacv1helpers.NewRule("list").Groups(projectGroup, legacyProjectGroup).Resources("projectrequests").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(authzGroup, legacyAuthzGroup).Resources("clusterroles").RuleOrDie(),
			rbacv1helpers.NewRule(read...).Groups(rbacGroup).Resources("clusterroles").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
			rbacv1helpers.NewRule("list", "watch").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),
		},
		allUnauthenticatedRules...,
	)
)

var _ = g.Describe("The default cluster RBAC policy", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("default-rbac-policy", exutil.KubeConfigPath())

	kubeInformers := informers.NewSharedInformerFactory(oc.AdminKubeClient(), 20*time.Minute)
	ruleResolver := openshiftapiserver.NewRuleResolver(kubeInformers.Rbac().V1()) // signal what informers we want to use early

	stopCh := make(chan struct{})
	defer func() { close(stopCh) }()
	kubeInformers.Start(stopCh)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if ok := cache.WaitForCacheSync(ctx.Done(),
		kubeInformers.Rbac().V1().ClusterRoles().Informer().HasSynced,
		kubeInformers.Rbac().V1().ClusterRoleBindings().Informer().HasSynced,
		kubeInformers.Rbac().V1().Roles().Informer().HasSynced,
		kubeInformers.Rbac().V1().RoleBindings().Informer().HasSynced,
	); !ok {
		exutil.FatalErr("failed to sync RBAC cache")
	}

	g.It("should only allow the system:authenticated group to access certain policy rules cluster wide", func() {
		testGroupRules(ruleResolver, kuser.AllAuthenticated, allAuthenticatedRules)
	})

	g.It("should only allow the system:unauthenticated group to access certain policy rules cluster wide", func() {
		testGroupRules(ruleResolver, kuser.AllUnauthenticated, allUnauthenticatedRules)
	})

	g.It("should only allow the system:authenticated:oauth group to access certain policy rules cluster wide", func() {
		testGroupRules(ruleResolver, bootstrappolicy.AuthenticatedOAuthGroup, []rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create").Groups(projectGroup, legacyProjectGroup).Resources("projectrequests").RuleOrDie(),
		})
	})
})

func testGroupRules(ruleResolver validation.AuthorizationRuleResolver, group string, expectedRules []rbacv1.PolicyRule) {
	actualRules, err := ruleResolver.RulesFor(&kuser.DefaultInfo{Groups: []string{group}}, metav1.NamespaceNone)
	o.Expect(err).NotTo(o.HaveOccurred()) // our default RBAC policy should never have rule resolution errors

	if cover, missing := validation.Covers(expectedRules, actualRules); !cover {
		e2e.Failf("%s has extra cluster wide permissions:\n%s", group, rulesToSting(missing))
	}

	// force test data to be cleaned up every so often but allow extra rules to not deadlock new changes
	if cover, missing := validation.Covers(actualRules, expectedRules); !cover && len(missing) > 5 {
		e2e.Failf("test data for %s has too many unnecessary permissions:\n%s", group, rulesToSting(missing))
	}
}

func rulesToSting(rules []rbacv1.PolicyRule) string {
	compactRules := rules
	if compact, err := validation.CompactRules(rules); err == nil {
		compactRules = compact
	}

	missingDescriptions := sets.NewString()
	for _, missing := range compactRules {
		missingDescriptions.Insert(rbacv1helpers.CompactString(missing))
	}

	return strings.Join(missingDescriptions.List(), "\n")
}
