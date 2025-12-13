package rbac

import (
	"context"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	kuser "k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	rbacvalidation "k8s.io/component-helpers/auth/rbac/validation"
	kauthenticationapi "k8s.io/kubernetes/pkg/apis/authentication"
	kauthorizationapi "k8s.io/kubernetes/pkg/apis/authorization"
	"k8s.io/kubernetes/pkg/apis/rbac"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
	"k8s.io/kubernetes/pkg/apis/storage"
	"k8s.io/kubernetes/pkg/registry/rbac/validation"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/api/authorization"
	"github.com/openshift/api/build"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/api/console"
	"github.com/openshift/api/image"
	"github.com/openshift/api/oauth"
	"github.com/openshift/api/project"
	"github.com/openshift/api/security"
	"github.com/openshift/api/template"
	"github.com/openshift/api/user"

	exutil "github.com/openshift/origin/test/extended/util"
)

// copied from bootstrap policy
var read = []string{"get", "list", "watch"}

// copied from bootstrap policy
const (
	rbacGroup    = rbac.GroupName
	storageGroup = storage.GroupName
	kAuthzGroup  = kauthorizationapi.GroupName
	kAuthnGroup  = kauthenticationapi.GroupName

	authzGroup    = authorization.GroupName
	buildGroup    = build.GroupName
	imageGroup    = image.GroupName
	oauthGroup    = oauth.GroupName
	projectGroup  = project.GroupName
	templateGroup = template.GroupName
	userGroup     = user.GroupName
	consoleGroup  = console.GroupName

	legacyGroup         = ""
	legacyAuthzGroup    = ""
	legacyBuildGroup    = ""
	legacyImageGroup    = ""
	legacyProjectGroup  = ""
	legacyTemplateGroup = ""
	legacyUserGroup     = ""
	legacyOauthGroup    = ""

	// Provided as CRD via cluster-csi-snapshot-controller-operator
	snapshotGroup = "snapshot.storage.k8s.io"

	// Provided as CRD via operator-lifecycle-manager
	operatorsCoreOSGroup = "operators.coreos.com"
)

// Do not change any of these lists without approval from the auth and master teams
// Most rules are copied from various cluster roles in bootstrap policy
var (
	allUnauthenticatedRules = []rbacv1.PolicyRule{
		rbacv1helpers.NewRule("get", "create").Groups(buildGroup, legacyBuildGroup).Resources("buildconfigs/webhooks").RuleOrDie(),

		rbacv1helpers.NewRule("impersonate").Groups(kAuthnGroup).Resources("userextras/scopes.authorization.openshift.io").RuleOrDie(),

		rbacv1helpers.NewRule("create").Groups(authzGroup, legacyAuthzGroup).Resources("selfsubjectrulesreviews").RuleOrDie(),

		rbacv1helpers.NewRule("create").Groups(kAuthzGroup).Resources("selfsubjectaccessreviews", "selfsubjectrulesreviews").RuleOrDie(),

		rbacv1helpers.NewRule("delete").Groups(oauthGroup, legacyOauthGroup).Resources("oauthaccesstokens", "oauthauthorizetokens").RuleOrDie(),

		// this is openshift specific
		rbacv1helpers.NewRule("get").URLs(
			"/version/openshift",
			"/.well-known",
			"/.well-known/*",
			"/.well-known/oauth-authorization-server",
		).RuleOrDie(),

		// TODO: remove with after 1.15 rebase
		rbacv1helpers.NewRule("get").URLs(
			"/readyz",
		).RuleOrDie(),

		// this is from upstream kube
		rbacv1helpers.NewRule("get").URLs(
			"/healthz", "/livez",
			"/version",
			"/version/",
		).RuleOrDie(),
	}

	allAuthenticatedRules = append(
		[]rbacv1.PolicyRule{
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("builds/docker", "builds/optimizeddocker").RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("builds/jenkinspipeline").RuleOrDie(),
			rbacv1helpers.NewRule("create").Groups(buildGroup, legacyBuildGroup).Resources("builds/source").RuleOrDie(),

			// See https://github.com/kubernetes/kubernetes/pull/116274
			rbacv1helpers.NewRule("create").Groups(kAuthnGroup).Resources("selfsubjectreviews").RuleOrDie(),

			rbacv1helpers.NewRule("get").Groups(userGroup, legacyUserGroup).Resources("users").Names("~").RuleOrDie(),
			rbacv1helpers.NewRule("list").Groups(projectGroup, legacyProjectGroup).Resources("projectrequests").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(authzGroup, legacyAuthzGroup).Resources("clusterroles").RuleOrDie(),
			rbacv1helpers.NewRule(read...).Groups(rbacGroup).Resources("clusterroles").RuleOrDie(),
			rbacv1helpers.NewRule("get", "list").Groups(storageGroup).Resources("storageclasses").RuleOrDie(),
			rbacv1helpers.NewRule("list", "watch").Groups(projectGroup, legacyProjectGroup).Resources("projects").RuleOrDie(),

			rbacv1helpers.NewRule("use").Groups(security.GroupName).Resources("securitycontextconstraints").Names("restricted-v2").RuleOrDie(),
			rbacv1helpers.NewRule("use").Groups(security.GroupName).Resources("securitycontextconstraints").Names("restricted-v3").RuleOrDie(),

			// TODO: remove when openshift-apiserver has removed these
			rbacv1helpers.NewRule("get").URLs(
				"/healthz/",
				"/oapi", "/oapi/*",
				"/osapi", "/osapi/",
				"/swaggerapi", "/swaggerapi/*", "/swagger.json", "/swagger-2.0.0.pb-v1",
				"/version/*",
				"/",
			).RuleOrDie(),

			// this is from upstream kube
			rbacv1helpers.NewRule("get").URLs(
				"/",
				"/openapi", "/openapi/*",
				"/api", "/api/*",
				"/apis", "/apis/*",
			).RuleOrDie(),
		},
		allUnauthenticatedRules...,
	)

	// group -> namespace -> rules
	groupNamespaceRules = map[string]map[string][]rbacv1.PolicyRule{
		kuser.AllAuthenticated: {
			"openshift": {
				rbacv1helpers.NewRule(read...).Groups(templateGroup, legacyTemplateGroup).Resources("templates").RuleOrDie(),
				rbacv1helpers.NewRule(read...).Groups(operatorsCoreOSGroup).Resources("clusterserviceversions").RuleOrDie(),
				rbacv1helpers.NewRule(read...).Groups(imageGroup, legacyImageGroup).Resources("imagestreams", "imagestreamtags", "imagestreamimages", "imagetags").RuleOrDie(),
				rbacv1helpers.NewRule("get").Groups(imageGroup, legacyImageGroup).Resources("imagestreams/layers").RuleOrDie(),
				rbacv1helpers.NewRule("get").Groups("").Resources("configmaps").RuleOrDie(),
			},
			"openshift-config-managed": {
				rbacv1helpers.NewRule("get").Groups(legacyGroup).Resources("configmaps").Names("console-public").RuleOrDie(),
				rbacv1helpers.NewRule(read...).Groups("").Resources("configmaps").Names("oauth-serving-cert").RuleOrDie(),
				rbacv1helpers.NewRule("get").Groups("").Resources("configmaps").Names("openshift-network-features").RuleOrDie(),
			},
			"kube-system": {
				// this allows every authenticated user to use in-cluster client certificate termination
				rbacv1helpers.NewRule(read...).Groups(legacyGroup).Resources("configmaps").Names("extension-apiserver-authentication").RuleOrDie(),
			},
		},
		kuser.AllUnauthenticated:     {}, // no rules expect the cluster wide ones
		"system:authenticated:oauth": {}, // no rules expect the cluster wide ones
	}
)

var _ = g.Describe("[sig-auth][Feature:OpenShiftAuthorization] The default cluster RBAC policy", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("default-rbac-policy")

	g.It("should have correct RBAC rules", g.Label("Size:S"), func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		enabledCapabilities := []configv1.ClusterVersionCapability{}

		exist, err := exutil.DoesApiResourceExist(oc.AdminConfig(), "clusterversions", "config.openshift.io")
		o.Expect(err).NotTo(o.HaveOccurred())
		if exist {
			clusterVersion, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
			if err != nil {
				e2e.Failf("Failed to get cluster version: %v", err)
			}
			enabledCapabilities = append(enabledCapabilities, clusterVersion.Status.Capabilities.EnabledCapabilities...)
		} else {
			e2e.Fail("Cluster version API resource does not exist")
		}

		// Conditional, capability-specific rules
		for _, capability := range enabledCapabilities {
			switch capability {
			case configv1.ClusterVersionCapabilityConsole:
				allAuthenticatedRules = append(
					allAuthenticatedRules,
					[]rbacv1.PolicyRule{
						// These custom resources are used to extend console functionality
						// The console team may eventually eliminate this exception
						rbacv1helpers.NewRule(read...).Groups(consoleGroup).Resources(
							"consoleclidownloads",
							"consoleexternalloglinks",
							"consolelinks",
							"consolenotifications",
							"consoleplugins",
							"consolequickstarts",
							"consolesamples",
							"consoleyamlsamples",
						).RuleOrDie(),

						// HelmChartRepository instances keep Helm chart repository configuration
						// By default users are able to browse charts from all configured repositories through console UI
						rbacv1helpers.NewRule(read...).Groups("helm.openshift.io").Resources("helmchartrepositories").RuleOrDie(),
					}...,
				)

			case configv1.ClusterVersionCapabilityCSISnapshot:
				allAuthenticatedRules = append(
					allAuthenticatedRules,
					rbacv1helpers.NewRule("get", "list", "watch").Groups(snapshotGroup).Resources("volumesnapshotclasses").RuleOrDie(),
				)
			}
		}

		kubeInformers := informers.NewSharedInformerFactory(oc.AdminKubeClient(), 20*time.Minute)
		ruleResolver := exutil.NewRuleResolver(kubeInformers.Rbac().V1()) // signal what informers we want to use early

		kubeInformers.Start(ctx.Done())

		if ok := cache.WaitForCacheSync(ctx.Done(),
			kubeInformers.Rbac().V1().ClusterRoles().Informer().HasSynced,
			kubeInformers.Rbac().V1().ClusterRoleBindings().Informer().HasSynced,
			kubeInformers.Rbac().V1().Roles().Informer().HasSynced,
			kubeInformers.Rbac().V1().RoleBindings().Informer().HasSynced,
		); !ok {
			exutil.FatalErr("failed to sync RBAC cache")
		}

		namespaces, err := oc.AdminKubeClient().CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if err != nil {
			exutil.FatalErr(err)
		}

		g.By("should only allow the system:authenticated group to access certain policy rules", func() {
			testAllGroupRules(ctx, ruleResolver, kuser.AllAuthenticated, allAuthenticatedRules, namespaces.Items)
		})

		g.By("should only allow the system:unauthenticated group to access certain policy rules", func() {
			testAllGroupRules(ctx, ruleResolver, kuser.AllUnauthenticated, allUnauthenticatedRules, namespaces.Items)
		})

		g.By("should only allow the system:authenticated:oauth group to access certain policy rules", func() {
			testAllGroupRules(ctx, ruleResolver, "system:authenticated:oauth", []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("create").Groups(projectGroup, legacyProjectGroup).Resources("projectrequests").RuleOrDie(),
				rbacv1helpers.NewRule("get", "list", "watch", "delete").Groups(oauthGroup).Resources("useroauthaccesstokens").RuleOrDie(),
			}, namespaces.Items)
		})

	})
})

func testAllGroupRules(ctx context.Context, ruleResolver validation.AuthorizationRuleResolver, group string, expectedClusterRules []rbacv1.PolicyRule, namespaces []corev1.Namespace) {
	testGroupRules(ctx, ruleResolver, group, metav1.NamespaceNone, expectedClusterRules)

	for _, namespace := range namespaces {
		// merge the namespace scoped and cluster wide rules
		rules := append([]rbacv1.PolicyRule{}, groupNamespaceRules[group][namespace.Name]...)
		rules = append(rules, expectedClusterRules...)

		testGroupRules(ctx, ruleResolver, group, namespace.Name, rules)
	}
}

func testGroupRules(ctx context.Context, ruleResolver validation.AuthorizationRuleResolver, group, namespace string, expectedRules []rbacv1.PolicyRule) {
	actualRules, err := ruleResolver.RulesFor(ctx, &kuser.DefaultInfo{Groups: []string{group}}, namespace)
	o.Expect(err).NotTo(o.HaveOccurred()) // our default RBAC policy should never have rule resolution errors

	if cover, missing := rbacvalidation.Covers(expectedRules, actualRules); !cover {
		e2e.Failf("%s has extra permissions in namespace %q:\n%s", group, namespace, rulesToString(missing))
	}

	// force test data to be cleaned up every so often but allow extra rules to not deadlock new changes
	if cover, missing := rbacvalidation.Covers(actualRules, expectedRules); !cover {
		log := e2e.Logf
		if len(missing) > 15 {
			log = e2e.Failf
		}
		log("test data for %s has too many unnecessary permissions:\n%s", group, rulesToString(missing))
	}
}

func rulesToString(rules []rbacv1.PolicyRule) string {
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
