package authentication

import (
	"context"
	"errors"
	"fmt"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/pod-security-admission/api"

	"github.com/openshift/origin/test/extended/authentication/operator_wait"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/operator"
)

var _ = g.Describe("[sig-auth][Suite:openshift/conformance/serial][Jira:\"Authentication\"][OCPFeatureGate:AuthenticationComponentProxyExternalOIDC][Skipped:MicroShift][Serial][Slow][Disruptive]", g.Ordered, func() {
	oc := exutil.NewCLIWithoutNamespace("component-proxy-external-oidc")
	oc.KubeFramework().NamespacePodSecurityLevel = api.LevelPrivileged
	oc.SetNamespace("component-proxy-external-oidc")
	ctx := context.TODO()

	var (
		cleanups         []removalFunc
		keycloakCli      *keycloakClient
		username         string
		password         string
		group            string
		originalAuth     *configv1.Authentication
		oidcClientSecret string

		keycloakNamespace   string
		webhookArchitecture bool

		proxyForExternalOIDCCleanUps []removalFunc

		httpProxyURL   string
		httpsProxyURL  string
		caCertPEM      []byte
		proxyNamespace string

		externalOIDCComponentProxyTests ExternalOIDCComponentProxyTests
	)

	g.BeforeAll(func() {
		var err error

		clusterVersion, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting ClusterVersion")
		desiredVersion := clusterVersion.Status.Desired.Version

		fgs, err := oc.AdminConfigClient().ConfigV1().FeatureGates().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting the cluster FeatureGates")
		webhookArchitecture, err = isWebhookOIDCArchitecture(fgs, desiredVersion)
		o.Expect(err).NotTo(o.HaveOccurred(), "should be able to determine OIDC architecture from feature gates")

		// waitTime is in minutes - set to 30 minute wait for cluster operators to settle before starting tests.
		err = operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 30)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error waiting for the cluster operators to settle before starting test")

		testID := rand.String(8)
		keycloakNamespace = fmt.Sprintf("oidc-keycloak-%s", testID)

		cleanups, err = deployKeycloak(ctx, oc, keycloakNamespace, g.GinkgoLogr)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error deploying keycloak")

		kcURL, err := admittedURLForRoute(ctx, oc, keycloakResourceName, keycloakNamespace)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error getting keycloak route URL")

		keycloakCli, err = keycloakClientFor(kcURL)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating a keycloak client")

		// First authenticate as the admin keycloak user so we can add new groups and users
		err = keycloakCli.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error authenticating as keycloak admin")

		o.Expect(keycloakCli.ConfigureClient("admin-cli")).NotTo(o.HaveOccurred(), "should not encounter an error configuring the admin-cli client")

		username = fmt.Sprintf("user-%s", testID)
		password = fmt.Sprintf("password-%s", testID)
		group = fmt.Sprintf("ocp-test-%s-group", testID)

		o.Expect(keycloakCli.CreateGroup(group)).To(o.Succeed(), "should be able to create a new keycloak group")
		o.Expect(keycloakCli.CreateUser(username, password, group)).To(o.Succeed(), "should be able to create a new keycloak user")

		originalAuth, err = oc.AdminConfigClient().ConfigV1().Authentications().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not error getting authentications")

		// create a dummy oidc client secret for the console to consume
		oidcClientSecret = fmt.Sprintf("openshift-console-oidc-client-secret-%s", testID)
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      oidcClientSecret,
				Namespace: "openshift-config",
			},
			Data: map[string][]byte{
				"clientSecret": []byte(`a-secret-value`),
			},
		}
		_, err = oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Create(ctx, secret, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should not encounter an error creating oidc client secret")
		cleanups = append(cleanups, func(ctx context.Context) error {
			return oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Delete(ctx, secret.Name, metav1.DeleteOptions{})
		})

		externalOIDCComponentProxyTests, err = NewExternalOIDCComponentProxyTests(oc)
		o.Expect(err).
			NotTo(o.HaveOccurred(), "should instantiate external oidc component proxy tests")

		g.By("Saving auth state for restore after test")
		authRestore, err := saveAndRestoreAuthState(ctx, oc)
		proxyForExternalOIDCCleanUps = append(proxyForExternalOIDCCleanUps, authRestore)
		o.Expect(err).
			NotTo(o.HaveOccurred(), "should not encounter an error storing the authentication state")

		g.By("Clearing any pre-existing component proxy so the test starts from a clean state")
		err = updateAuthenticationProxy(ctx, oc, operatorv1.AuthenticationProxyConfig{})
		o.Expect(err).
			NotTo(o.HaveOccurred(), "should clear any pre-existing component proxy config")

		err = operator_wait.WaitForOperatorToPickUpChanges(ctx, oc, "authentication")
		o.Expect(err).
			NotTo(o.HaveOccurred(), "operator should reconcile after clearing component proxy")

		g.By("Deploying Squid forward proxy")
		var proxyCleanup removalFunc
		httpProxyURL, httpsProxyURL, caCertPEM, proxyNamespace, proxyCleanup, err = deploySquidProxy(ctx, oc)
		proxyForExternalOIDCCleanUps = append(proxyForExternalOIDCCleanUps, proxyCleanup)
		o.Expect(err).
			NotTo(o.HaveOccurred(), "should not encounter an error during squid deployment")

		g.By("Configuring OIDC Authentication")
		originalAuthState, updatedAuthState, err := configureOIDCAuthentication(ctx, oc, keycloakNamespace, oidcClientSecret, nil)
		o.Expect(err).
			NotTo(o.HaveOccurred(), "should not encounter an error configuring OIDC authentication")

		// configureOIDCAuthentication installs the suite's clean Keycloak provider (keep it — do NOT
		// swap for ensureOIDCConfigured, which early-returns for any pre-existing OIDC provider and
		// could leave a different provider in place). But BeforeAll reruns before each It (see the
		// note in oidc.go); on reruns the Update is a spec no-op, so only wait for a rollout when
		// the spec actually changed. An unconditional wait would block for the full rollout timeout
		// waiting for a revision that never rolls.
		if !equality.Semantic.DeepEqual(originalAuthState.Spec, updatedAuthState.Spec) {
			waitForOIDCRollout(ctx, oc, webhookArchitecture)
		}
		waitForHealthyOIDCClients(ctx, oc)

		g.By("Verifying oauth-apiserver has no proxy config before test start")
		err = externalOIDCComponentProxyTests.proxyVerifier.AssertNoConfig(ctx)
		o.Expect(err).
			NotTo(o.HaveOccurred(), "oauth-apiserver should have no proxy config before test start")
	})

	g.It("operator should propagate proxy configuration to the oauth-apiserver operand", func() {
		externalOIDCComponentProxyTests.testExternalOIDCOperandProxyConfiguration(ctx, httpProxyURL, nil)
	})
	g.It("operator should propagate proxy configuration and trustedCA to the oauth-apiserver operand", func() {
		externalOIDCComponentProxyTests.testExternalOIDCOperandProxyConfiguration(ctx, httpsProxyURL, caCertPEM)
	})
	g.It("operator should fall back to original configuration on spec.proxy removal", func() {
		externalOIDCComponentProxyTests.testExternalOIDCFallbackOnProxyRemoval(ctx, httpProxyURL)
	})
	g.It("operator should go Degraded when spec.proxy.trustedCA references a missing ConfigMap", func() {
		externalOIDCComponentProxyTests.testExternalOIDCOperatorDegradedOnMissingCA(ctx, keycloakCli, username, password, httpsProxyURL, caCertPEM)
	})
	g.It("operator should re-sync trustedCA ConfigMap content to the oauth-apiserver operand when the source changes", func() {
		externalOIDCComponentProxyTests.testExternalOIDCOperatorResyncsTrustedCA(ctx, httpsProxyURL, caCertPEM)
	})
	g.It("oauth-apiserver should perform full OIDC login flow through the proxy when auth proxy config is applied", func() {
		externalOIDCComponentProxyTests.testExternalOIDCClaimsSourcingThroughProxy(ctx, keycloakCli, keycloakNamespace, proxyNamespace, username, password, group, httpProxyURL, nil)
	})
	g.It("oauth-apiserver should perform full OIDC login flow through the HTTPS proxy with trustedCA when auth proxy config is applied", func() {
		externalOIDCComponentProxyTests.testExternalOIDCClaimsSourcingThroughProxy(ctx, keycloakCli, keycloakNamespace, proxyNamespace, username, password, group, httpsProxyURL, caCertPEM)
	})
	g.It("oauth-apiserver should perform full OIDC login flow when the proxy URL carries credentials", func() {
		credentialedProxyURL, err := proxyURLWithCredentials(httpProxyURL, "e2e-proxy-user", "e2e-proxy-pass")
		o.Expect(err).NotTo(o.HaveOccurred(), "should build a credentialed proxy URL")
		externalOIDCComponentProxyTests.testExternalOIDCClaimsSourcingThroughProxy(ctx, keycloakCli, keycloakNamespace, proxyNamespace, username, password, group, credentialedProxyURL, nil)
	})
	g.It("oauth-apiserver/operator should hot-reload mounted CA file on change when spec.proxy.trustedCA is set", func() {
		externalOIDCComponentProxyTests.testExternalOIDCHotReloadCAFileChange(ctx, keycloakCli, keycloakNamespace, proxyNamespace, username, password, group, httpsProxyURL, caCertPEM)
	})
	g.It("oauth-apiserver should bypass proxy by directly connecting to idp to perform OIDC login flow when spec.proxy.noProxy contains idp", func() {
		externalOIDCComponentProxyTests.testExternalOIDCBypassNoProxyHost(ctx, keycloakCli, keycloakNamespace, proxyNamespace, username, password, group, httpProxyURL, httpsProxyURL, caCertPEM)
	})

	g.AfterAll(func() {
		var errs []error

		g.By("Cleaning up the proxy for external oidc")
		err := removeResources(ctx, proxyForExternalOIDCCleanUps...)
		if err != nil {
			errs = append(errs, fmt.Errorf("error during proxy for external oidc clean up: %w", err))
		}

		g.By("Waiting for operators to be stable after test")
		err = operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
		if err != nil {
			errs = append(errs, fmt.Errorf("error waiting operators to settle after tests: %w", err))
		}

		g.By("Reverting authentication to original state")
		err, modified := resetAuthentication(ctx, oc, originalAuth)
		if err != nil {
			errs = append(errs, fmt.Errorf("error reverting authentication to original state: %w", err))
		}

		g.By("Cleaning up keycloak resources")
		err = removeResources(ctx, cleanups...)
		if err != nil {
			errs = append(errs, fmt.Errorf("error cleaning up keycloak resources: %w", err))
		}

		o.Expect(errors.Join(errs...)).
			NotTo(o.HaveOccurred(), "should not encounter any error during cleaning up")

		if modified {
			switch originalAuth.Spec.Type {
			case configv1.AuthenticationTypeOIDC:
				waitForOIDCRollout(ctx, oc, webhookArchitecture)
			default:
				waitForIntegratedOAuthRollout(ctx, oc)
			}
		}
	})
})
