package authentication

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/library-go/pkg/oauth/tokenrequest"
	"github.com/openshift/library-go/pkg/oauth/tokenrequest/challengehandlers"

	operatorv1 "github.com/openshift/api/operator/v1"
	libcrypto "github.com/openshift/library-go/pkg/crypto"
	exutil "github.com/openshift/origin/test/extended/util"
	operator "github.com/openshift/origin/test/extended/util/operator"

	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	authnv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	g.It("oauth-server should perform full OIDC login flow through the proxy when auth proxy config is applied", func() {
		testProxyConfigPerformOIDCLogin(ctx, oc, kcSetup, httpProxyURL, proxyNamespace)
	})
	g.It("oauth-server/operator should hot-reload mounted CA file on change when spec.proxy.trustedCA is set", func() {
		testHotReloadCAFileChange(ctx, oc, caCertPEM, kcSetup, httpsProxyURL, proxyNamespace)
	})
	g.It("oauth-server should bypass proxy by directly connecting to idp to perform OIDC login flow when spec.proxy.noProxy contains idp", func() {
		testBypassProxyNoProxyHost(ctx, oc, caCertPEM, kcSetup, httpProxyURL, httpsProxyURL, proxyNamespace)
	})
})

func testProxyConfigPerformOIDCLogin(ctx context.Context, oc *exutil.CLI, kcSetup *keycloakProxySetup, httpProxyURL, proxyNamespace string) {
	g.By("setting direct access grant for oauth flow")
	enableDirectAccessGrant(kcSetup)

	kcUser, kcPass, kcGroup := createKeycloakUserPasswordGroup(kcSetup)

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

	g.By("Waiting for operator to pick up proxy and IdP changes and stabilize")
	err = waitForOperatorToPickUpChanges(ctx, oc, "authentication")
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying oauth-server has HTTPS_PROXY but not HTTP_PROXY")
	err = verifyOAuthServerDeploymentProxyConfig(
		ctx, oc, "", httpProxyURL, ".cluster.local,.svc,127.0.0.1,localhost",
		false)
	o.Expect(err).NotTo(o.HaveOccurred())

	logCutOff := time.Now()

	g.By("Performing full OIDC login flow through component proxy")
	assertOIDCLogin(ctx, oc, kcUser, kcPass, kcGroup)

	g.By("Verifying Keycloak traffic from oauth-server went through the Squid proxy")
	issuerURL, err := url.Parse(kcSetup.issuerURL)
	o.Expect(err).NotTo(o.HaveOccurred())
	keycloakHost := issuerURL.Hostname()

	ips := getOAuthServerPodIPs(ctx, oc)
	err = waitForProxyTrafficFromTo(ctx, oc, proxyNamespace, ips, keycloakHost, logCutOff, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Logging user out for next login")
	deleteOIDCUserAndIdentities(ctx, oc, kcUser)

	g.By("Removing component-scoped proxy config")
	err = updateAuthenticationProxy(ctx, oc, operatorv1.AuthenticationProxyConfig{})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Deleting Squid to prove the operator no longer routes through it")
	err = deleteNamespaceSync(ctx, oc, proxyNamespace, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Waiting for operator to reconcile proxy removal")
	err = waitForOperatorToPickUpChanges(ctx, oc, "authentication")
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Performing OIDC login flow via direct IdP connectivity after proxy removal")
	assertOIDCLogin(ctx, oc, kcUser, kcPass, kcGroup)
}

func testHotReloadCAFileChange(ctx context.Context, oc *exutil.CLI, caCertPEM []byte, kcSetup *keycloakProxySetup, httpsProxyURL, proxyNamespace string) {
	g.By("setting direct access grant for oauth flow")
	enableDirectAccessGrant(kcSetup)

	kcUser, kcPass, kcGroup := createKeycloakUserPasswordGroup(kcSetup)
	kubeClient := oc.AdminKubeClient()

	g.By("Creating trustedCA ConfigMap in openshift-config")
	configMapName, cmCleanup, err := createTrustedCAConfigMap(ctx, oc, caCertPEM)
	g.DeferCleanup(cmCleanup)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Setting component-scoped proxy with trustedCA")
	err = updateAuthenticationProxy(ctx, oc, operatorv1.AuthenticationProxyConfig{
		HTTPSProxy: httpsProxyURL,
		TrustedCA: operatorv1.AuthenticationConfigMapReference{
			Name: configMapName,
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Registering Keycloak as OIDC IdP")
	idpCleanups, err := addKeycloakOIDCIdPForProxy(ctx, oc, kcSetup)
	g.DeferCleanup(func() {
		_ = removeResources(ctx, idpCleanups...)
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Waiting for operator to pick up proxy, trustedCA and IdP changes and stabilize")
	err = waitForOperatorToPickUpChanges(ctx, oc, "authentication")
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying trustedCA ConfigMap is synced to openshift-authentication namespace")
	err = verifyTrustedCAConfigMapSynced(ctx, oc)
	o.Expect(err).NotTo(o.HaveOccurred())

	logCutOff := time.Now()

	g.By("Verifying OIDC login works after setting proxy with trustedCA")
	assertOIDCLogin(ctx, oc, kcUser, kcPass, kcGroup)

	g.By("Verifying Keycloak traffic from oauth-server went through the Squid proxy")
	issuerURL, err := url.Parse(kcSetup.issuerURL)
	o.Expect(err).NotTo(o.HaveOccurred())
	keycloakHost := issuerURL.Hostname()

	ips := getOAuthServerPodIPs(ctx, oc)
	err = waitForProxyTrafficFromTo(ctx, oc, proxyNamespace, ips, keycloakHost, logCutOff, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Recording Deployment generation before CA rotation")
	deployment, err := kubeClient.AppsV1().Deployments("openshift-authentication").Get(ctx, "oauth-openshift", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	generationBefore := deployment.Generation

	g.By("Rotating CA: generating new CA and server cert")
	newCAConfig, err := libcrypto.MakeSelfSignedCAConfigForDuration("squid-proxy-ca", 2*time.Hour)
	o.Expect(err).NotTo(o.HaveOccurred())
	newCA := &libcrypto.CA{Config: newCAConfig, SerialGenerator: &libcrypto.RandomSerialGenerator{}}

	serviceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", squidServiceName, proxyNamespace)
	newServerCertConfig, err := newCA.MakeServerCert(sets.New(serviceDNS), 2*time.Hour)
	o.Expect(err).NotTo(o.HaveOccurred())

	newCACertPEM, _, err := newCAConfig.GetPEMBytes()
	o.Expect(err).NotTo(o.HaveOccurred())
	newServerCertPEM, newServerKeyPEM, err := newServerCertConfig.GetPEMBytes()
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Updating squid-tls Secret with rotated cert")
	tlsSecret, err := kubeClient.CoreV1().Secrets(proxyNamespace).Get(ctx, "squid-tls", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	tlsSecret.Data["tls.crt"] = newServerCertPEM
	tlsSecret.Data["tls.key"] = newServerKeyPEM
	_, err = kubeClient.CoreV1().Secrets(proxyNamespace).Update(ctx, tlsSecret, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	squidPods, err := kubeClient.CoreV1().Pods(proxyNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=squid-proxy",
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(squidPods.Items).NotTo(o.BeEmpty())

	g.By("Waiting for squid-tls Secret to propagate to pod volume")
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		output, execErr := oc.AsAdmin().Run("exec").Args(
			"-n", proxyNamespace,
			squidPods.Items[0].Name,
			"-c", "squid",
			"--", "cat", "/etc/squid/tls/tls.crt",
		).Output()
		if execErr != nil {
			g.GinkgoWriter.Printf("failed to read cert from squid pod: %v\n", execErr)
			return false, nil
		}
		if !strings.Contains(strings.TrimSpace(output), strings.TrimSpace(string(newServerCertPEM))) {
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "squid-tls Secret should propagate to pod volume")

	g.By("Reconfiguring Squid to pick up new cert")
	output, err := oc.AsAdmin().Run("exec").Args(
		"-n", proxyNamespace,
		squidPods.Items[0].Name,
		"-c", "squid",
		"--", "/usr/sbin/squid", "-k", "reconfigure",
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "squid reconfigure failed: %s", string(output))

	g.By("Updating trustedCA ConfigMap with new CA")
	cm, err := kubeClient.CoreV1().ConfigMaps("openshift-config").Get(ctx, configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	cm.Data["ca-bundle.crt"] = string(newCACertPEM)
	_, err = kubeClient.CoreV1().ConfigMaps("openshift-config").Update(ctx, cm, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	oauthServerPodList, err := kubeClient.CoreV1().Pods("openshift-authentication").List(ctx, metav1.ListOptions{LabelSelector: "app=oauth-openshift"})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(oauthServerPodList.Items).NotTo(o.BeEmpty())

	g.By("Waiting for oauth-server pods to pick up the new CA file")
	// https://github.com/openshift/cluster-authentication-operator/blob/master/
	// pkg/controllers/configobservation/oauth/observe_proxy_trusted_ca.go#L15
	caFilePath := "/var/config/system/configmaps/v4-0-config-system-auth-proxy-ca/ca-bundle.crt"
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		for _, pod := range oauthServerPodList.Items {
			output, execErr := oc.AsAdmin().Run("exec").Args(
				"-n", "openshift-authentication",
				pod.Name,
				"-c", "oauth-openshift",
				"--", "cat", caFilePath,
			).Output()
			if execErr != nil {
				g.GinkgoWriter.Printf("failed to read CA file from pod %s: %v\n", pod, execErr)
				return false, nil
			}
			if !strings.Contains(strings.TrimSpace(output), strings.TrimSpace(string(newCACertPEM))) {
				return false, nil
			}
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "oauth-server pods should have picked up the new CA file")

	g.By("Logging user out for next login")
	deleteOIDCUserAndIdentities(ctx, oc, kcUser)

	g.By("Verifying OIDC login works after CA rotation")
	assertOIDCLogin(ctx, oc, kcUser, kcPass, kcGroup)

	g.By("Verifying oauth-openshift Deployment was not updated after CA rotation")
	deployment, err = kubeClient.AppsV1().Deployments("openshift-authentication").Get(ctx, "oauth-openshift", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(deployment.Generation).To(o.Equal(generationBefore), "oauth-openshift Deployment should not have been updated after CA file change")

	g.By("Verifying operator re-syncs promptly after trustedCA ConfigMap update")
	err = operator.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 1)
	o.Expect(err).NotTo(o.HaveOccurred())
}

func testBypassProxyNoProxyHost(ctx context.Context, oc *exutil.CLI, caCertPEM []byte, kcSetup *keycloakProxySetup, httpProxyURL, httpsProxyURL, proxyNamespace string) {
	g.By("setting direct access grant for oauth flow")
	enableDirectAccessGrant(kcSetup)

	kcUser, kcPass, kcGroup := createKeycloakUserPasswordGroup(kcSetup)

	g.By("Setting component-scoped proxy with noProxy")
	issuerURL, err := url.Parse(kcSetup.issuerURL)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Creating trustedCA ConfigMap in openshift-config")
	configMapName, cmCleanup, err := createTrustedCAConfigMap(ctx, oc, caCertPEM)
	g.DeferCleanup(cmCleanup)
	o.Expect(err).NotTo(o.HaveOccurred())

	keycloakHost := issuerURL.Hostname()
	err = updateAuthenticationProxy(ctx, oc, operatorv1.AuthenticationProxyConfig{
		HTTPProxy:  httpProxyURL,
		HTTPSProxy: httpsProxyURL,
		TrustedCA: operatorv1.AuthenticationConfigMapReference{
			Name: configMapName,
		},
		NoProxy: []string{keycloakHost},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Registering Keycloak as OIDC IdP")
	idpCleanups, err := addKeycloakOIDCIdPForProxy(ctx, oc, kcSetup)
	g.DeferCleanup(func() {
		_ = removeResources(ctx, idpCleanups...)
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Waiting for operator to pick up proxy and IdP changes and stabilize")
	err = waitForOperatorToPickUpChanges(ctx, oc, "authentication")
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying oauth-server has HTTP_PROXY, HTTPS_PROXY, and NO_PROXY with custom entry")
	err = verifyOAuthServerDeploymentProxyConfig(ctx, oc, httpProxyURL, httpsProxyURL, ".cluster.local,.svc,127.0.0.1,localhost,"+keycloakHost, true)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Deleting Squid proxy namespace to prove noProxy bypasses it")
	err = deleteNamespaceSync(ctx, oc, proxyNamespace, 5*time.Minute)
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("Verifying OIDC login works after setting proxy with noProxy")
	assertOIDCLogin(ctx, oc, kcUser, kcPass, kcGroup)
}

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

func createKeycloakUserPasswordGroup(kcSetup *keycloakProxySetup) (kcUser, kcPass, kcGroup string) {
	testID := rand.String(8)

	kcGroup = fmt.Sprintf("e2e-proxy-kc-group-%s", testID)
	kcUser = fmt.Sprintf("e2e-proxy-kc-user-%s", testID)
	kcPass = fmt.Sprintf("e2e-proxy-kc-pass-%s", testID)

	err := kcSetup.client.CreateGroup(kcGroup)
	o.Expect(err).NotTo(o.HaveOccurred())

	err = kcSetup.client.CreateUser(kcUser, kcPass, kcGroup)
	o.Expect(err).NotTo(o.HaveOccurred())

	return kcUser, kcPass, kcGroup
}

func assertOIDCLogin(ctx context.Context, oc *exutil.CLI, username, password, expectedGroup string) {
	g.GinkgoHelper()

	kubeConfig := oc.AdminConfig()

	routeClient := oc.AdminRouteClient()
	route, err := routeClient.RouteV1().Routes("openshift-authentication").Get(ctx, "oauth-openshift", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should be able to get the OAuth server route")
	oauthServerURL := fmt.Sprintf("https://%s", route.Spec.Host)

	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		tokenOpts := tokenrequest.NewRequestTokenOptions(rest.CopyConfig(kubeConfig), false)
		tokenOpts, err := tokenOpts.WithChallengeHandlers(
			challengehandlers.NewBasicChallengeHandler(oauthServerURL, "", nil, io.Discard, nil, username, password),
		)
		if err != nil {
			g.GinkgoWriter.Printf("failed to create challenge handler: %v", err)
			return false, nil
		}

		token, err := tokenOpts.RequestToken()
		if err != nil {
			g.GinkgoWriter.Printf("failed to request token: %v", err)
			return false, nil
		}
		if token == "" {
			g.GinkgoWriter.Print("received empty token")
			return false, nil
		}

		tokenConfig := rest.AnonymousClientConfig(kubeConfig)
		tokenConfig.BearerToken = token
		tokenKubeClient, err := kubernetes.NewForConfig(tokenConfig)
		if err != nil {
			g.GinkgoWriter.Printf("failed to create kube client with token: %v", err)
			return false, nil
		}

		ssr, err := tokenKubeClient.AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{}, metav1.CreateOptions{})
		if err != nil {
			g.GinkgoWriter.Printf("failed to create SelfSubjectReview: %v", err)
			return false, nil
		}

		if ssr.Status.UserInfo.Username == "" {
			g.GinkgoWriter.Print("SelfSubjectReview returned empty username")
			return false, nil
		}

		if slices.Contains(ssr.Status.UserInfo.Groups, expectedGroup) {
			return true, nil
		}
		g.GinkgoWriter.Printf("expected group %q not found in groups: %v", expectedGroup, ssr.Status.UserInfo.Groups)
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "OIDC login flow should succeed")
}

func deleteOIDCUserAndIdentities(ctx context.Context, oc *exutil.CLI, username string) {
	g.GinkgoHelper()
	userClient := oc.AdminUserClient().UserV1()

	user, err := userClient.Users().Get(ctx, username, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should be able to get user %q", username)

	for _, identity := range user.Identities {
		err = userClient.Identities().Delete(ctx, identity, metav1.DeleteOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "should be able to delete identity %q", identity)
	}

	err = userClient.Users().Delete(ctx, username, metav1.DeleteOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "should be able to delete user %q", username)
}

func getOAuthServerPodIPs(ctx context.Context, oc *exutil.CLI) []string {
	g.GinkgoHelper()
	oauthPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-authentication").List(ctx, metav1.ListOptions{LabelSelector: "app=oauth-openshift"})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(oauthPods.Items).NotTo(o.BeEmpty())
	var ips []string
	for _, p := range oauthPods.Items {
		ips = append(ips, p.Status.PodIP)
	}
	return ips
}

func enableDirectAccessGrant(kcSetup *keycloakProxySetup) {
	kcClient, err := kcSetup.client.GetClientByClientID(kcSetup.clientID)
	o.Expect(err).NotTo(o.HaveOccurred())
	err = kcSetup.client.UpdateClientRaw(kcClient.ID, map[string]any{
		"directAccessGrantsEnabled": true,
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}
