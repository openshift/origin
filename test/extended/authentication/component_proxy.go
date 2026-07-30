package authentication

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1 "github.com/openshift/api/operator/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	operator "github.com/openshift/origin/test/extended/util/operator"
)

var _ = g.Describe("[sig-auth][Suite:openshift/conformance/serial][Jira:\"Authentication\"][OCPFeatureGate:AuthenticationComponentProxy][Serial]", func() {
	oc := exutil.NewCLIWithoutNamespace("component-proxy")

	var (
		ctx            context.Context
		httpProxyURL   string
		httpsProxyURL  string
		caCertPEM      []byte
		proxyNamespace string
		kcSetup        *keycloakProxySetup
		cleanups       []removalFunc
	)

	g.BeforeEach(func() {
		ctx = context.Background()
		cleanups = nil

		g.By("Saving auth state for restore after test")
		authRestore, err := saveAndRestoreAuthState(ctx, oc)
		cleanups = append(cleanups, authRestore)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Deploying Squid forward proxy")
		var proxyCleanup removalFunc
		httpProxyURL, httpsProxyURL, caCertPEM, proxyNamespace, proxyCleanup, err = deploySquidProxy(ctx, oc)
		cleanups = append(cleanups, proxyCleanup)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Deploying Keycloak (without registering IdP yet)")
		var kcCleanups []removalFunc
		kcSetup, kcCleanups, err = deployKeycloakForProxy(ctx, oc)
		cleanups = append(cleanups, kcCleanups...)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for operators to be stable before test")
		err = operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Waiting for OAuth server deployment to be stable before test")
		err = verifyOAuthServerDeploymentProxyConfig(ctx, oc, "", "", "", false)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.GinkgoWriter.Printf("Squid proxy URL: http=%s https=%s\n", httpProxyURL, httpsProxyURL)
		g.GinkgoWriter.Printf("Keycloak issuer URL: %s\n", kcSetup.issuerURL)
		g.GinkgoWriter.Printf("Keycloak namespace: %s\n", kcSetup.namespace)
	})

	g.AfterEach(func() {
		// Note that we are doing cleanup in a FIFO manner here.
		// This works better in this case as it resets authentication/cluster firstly.
		_ = removeResources(ctx, cleanups...)

		g.By("Waiting for operators to be stable after test")
		err := operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 10)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("operator should validate OIDC IdP through component proxy", func() {
		testOIDCIdPThroughComponentProxy(ctx, oc, kcSetup, httpProxyURL, nil, proxyNamespace)
	})
	g.It("operator should validate OIDC IdP through component proxy with trustedCA", func() {
		testOIDCIdPThroughComponentProxy(ctx, oc, kcSetup, httpsProxyURL, caCertPEM, proxyNamespace)
	})
	g.It("operator should fall back to original configuration on spec.proxy removal", func() {
		testFallbackOnProxyRemoval(ctx, oc, kcSetup, httpProxyURL, proxyNamespace)
	})
})

func testOIDCIdPThroughComponentProxy(ctx context.Context, oc *exutil.CLI, kcSetup *keycloakProxySetup, proxyURL string, trustedCACertPEM []byte, proxyNamespace string) {
	withTrustedCA := len(trustedCACertPEM) > 0

	var trustedCAConfigMapName string
	if withTrustedCA {
		g.By("Creating trustedCA ConfigMap in openshift-config")
		cmName, cmCleanup, err := createTrustedCAConfigMap(ctx, oc, trustedCACertPEM)
		g.DeferCleanup(cmCleanup)
		o.Expect(err).NotTo(o.HaveOccurred())
		trustedCAConfigMapName = cmName
	}

	proxyTrafficStart := time.Now()

	g.By("Setting component-scoped proxy")
	proxyConfig := operatorv1.AuthenticationProxyConfig{
		HTTPSProxy: proxyURL,
	}
	if withTrustedCA {
		proxyConfig.TrustedCA = operatorv1.AuthenticationConfigMapReference{Name: trustedCAConfigMapName}
	}
	err := updateAuthenticationProxy(ctx, oc, proxyConfig)
	o.Expect(err).NotTo(o.HaveOccurred())

	if withTrustedCA {
		g.By("Waiting for trustedCA ConfigMap to be synced before registering IdP")
		err = verifyTrustedCAConfigMapSynced(ctx, oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	}

	g.By("Registering Keycloak as OIDC IdP (operator discovers it through the proxy)")
	idpCleanups, err := addKeycloakOIDCIdPForProxy(ctx, oc, kcSetup)
	g.DeferCleanup(func() {
		_ = removeResources(ctx, idpCleanups...)
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Waiting for operator to pick up IdP changes and stabilize")
	err = waitForOperatorToPickUpChanges(ctx, oc, "authentication")
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying OAuth server deployment has proxy env vars and trustedCA volume/mount")
	err = verifyOAuthServerDeploymentProxyConfig(ctx, oc, "", proxyURL, ".cluster.local,.svc,127.0.0.1,localhost", withTrustedCA)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Looking up operator pod IP")
	operatorPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-authentication-operator").List(ctx, metav1.ListOptions{
		LabelSelector: "app=authentication-operator",
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(operatorPods.Items).NotTo(o.BeEmpty())
	operatorIP := operatorPods.Items[0].Status.PodIP
	o.Expect(operatorIP).NotTo(o.BeEmpty())

	g.By("Verifying operator traffic went through the Squid proxy")
	err = waitForProxyTrafficFrom(ctx, oc, proxyNamespace, operatorIP, proxyTrafficStart, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func testFallbackOnProxyRemoval(ctx context.Context, oc *exutil.CLI, kcSetup *keycloakProxySetup, httpProxyURL string, proxyNamespace string) {
	g.By("Setting component-scoped proxy")
	err := updateAuthenticationProxy(ctx, oc, operatorv1.AuthenticationProxyConfig{
		HTTPSProxy: httpProxyURL,
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Registering Keycloak as OIDC IdP")
	idpCleanups, err := addKeycloakOIDCIdPForProxy(ctx, oc, kcSetup)
	g.DeferCleanup(func() {
		_ = removeResources(ctx, idpCleanups...)
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Waiting for operator to pick up IdP changes and stabilize")
	err = waitForOperatorToPickUpChanges(ctx, oc, "authentication")
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Removing spec.proxy from Authentication CR")
	err = updateAuthenticationProxy(ctx, oc, operatorv1.AuthenticationProxyConfig{})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Deleting Squid to prove the operator no longer routes through it")
	err = deleteNamespaceSync(ctx, oc, proxyNamespace, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Waiting for operator to pick up proxy removal and stabilize")
	err = waitForOperatorToPickUpChanges(ctx, oc, "authentication")
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying proxy env vars are no longer set on OAuth server deployment")
	err = verifyOAuthServerDeploymentProxyConfig(ctx, oc, "", "", "", false)
	o.Expect(err).NotTo(o.HaveOccurred())
}
