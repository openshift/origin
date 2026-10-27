package authentication

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	libcrypto "github.com/openshift/library-go/pkg/crypto"

	authnv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/operator"
)

const (
	clusterAuthenticationOperatorName = "authentication"
	oauthAPIServerNamespace           = "openshift-oauth-apiserver"
	oauthAPIServerName                = "apiserver"

	// componentProxyCAMountPath mirrors the (unvendored) cluster-authentication-operator
	// common.ComponentProxyCAMountPath: the directory where authentication workloads mount
	// the proxy trustedCA ConfigMap (componentProxyCAConfigMapName).
	componentProxyCAMountPath = "/var/config/system/configmaps/" + componentProxyCAConfigMapName

	// componentProxyBaseNoProxy is the set of NO_PROXY entries the operator always injects.
	// AssertProxyConfig compares NO_PROXY with superset semantics, so this is a subset the
	// apiserver deployment is expected to contain.
	componentProxyBaseNoProxy = ".cluster.local,.svc,127.0.0.1,::1,localhost"
)

var (
	// NoProxyConfig is the empty component-proxy config used to clear spec.proxy.
	NoProxyConfig = operatorv1.AuthenticationProxyConfig{}
)

// ExternalOIDCComponentProxyTestsOption customizes an ExternalOIDCComponentProxyTests at
// construction time (functional-options pattern).
type ExternalOIDCComponentProxyTestsOption func(*ExternalOIDCComponentProxyTests) error

// WithProxyVerifierPollingInterval overrides the polling interval of the proxy verifier
// (the AssertProxyConfig/AssertNoConfig checks). It does not affect the operator-settle or
// deployment-rollout waits.
func WithProxyVerifierPollingInterval(interval time.Duration) ExternalOIDCComponentProxyTestsOption {
	return func(eopt *ExternalOIDCComponentProxyTests) error {
		eopt.proxyVerifier.PollingInterval = interval
		return nil
	}
}

// WithProxyVerifierPollingTimeout overrides the polling timeout of the proxy verifier
// (the AssertProxyConfig/AssertNoConfig checks). It does not affect the operator-settle or
// deployment-rollout waits.
func WithProxyVerifierPollingTimeout(timeout time.Duration) ExternalOIDCComponentProxyTestsOption {
	return func(eopt *ExternalOIDCComponentProxyTests) error {
		eopt.proxyVerifier.PollingTimeout = timeout
		return nil
	}
}

// ExternalOIDCComponentProxyTests groups the external-OIDC component-proxy test cases and
// their shared collaborators: the cluster handle, the composed operator-settle +
// deployment-rollout waiter, and the oauth-apiserver proxy verifier.
type ExternalOIDCComponentProxyTests struct {
	oc                                          *exutil.CLI
	waitForOperatorToSettleAndDeploymentRollout Wrapper
	proxyVerifier                               ProxyVerifier
}

// NewExternalOIDCComponentProxyTests builds the test group for oc, wiring the
// authentication-operator settle wait and the oauth-apiserver rollout wait into a single
// Wrapper. It touches no cluster state, so the returned error only reflects option failures.
func NewExternalOIDCComponentProxyTests(
	oc *exutil.CLI,
	opts ...ExternalOIDCComponentProxyTestsOption,
) (ExternalOIDCComponentProxyTests, error) {
	var zero ExternalOIDCComponentProxyTests

	clusterAuthenticationOperatorWaiter := OperatorChangesWaiter{
		oc:           oc,
		operatorName: clusterAuthenticationOperatorName,
	}

	oauthAPIServerRolloutWaiter := RolloutWaiter{
		oc:                  oc,
		deploymentNamespace: oauthAPIServerNamespace,
		deploymentName:      oauthAPIServerName,
	}

	toReturn := ExternalOIDCComponentProxyTests{
		oc: oc,
		// RolloutWaiter is outermost so the Deployment generation is captured before the
		// mutation; WaitChanges is inner so the operator's Progressing=True window is
		// observed right after the mutation, then the rollout completes last. See Chain.
		waitForOperatorToSettleAndDeploymentRollout: Chain(
			oauthAPIServerRolloutWaiter.MutateAndWaitForDeploymentRollout,
			clusterAuthenticationOperatorWaiter.WaitChanges,
		),
		proxyVerifier: ProxyVerifier{
			Namespace: oauthAPIServerNamespace,
			Name:      oauthAPIServerName,

			Oc:              oc,
			PollingInterval: 10 * time.Second,
			PollingTimeout:  5 * time.Minute,
		},
	}

	for _, anOpt := range opts {
		err := anOpt(&toReturn)
		if err != nil {
			return zero, fmt.Errorf("setting option: %w", err)
		}
	}

	return toReturn, nil
}

// deferResetProxy registers a cleanup that clears spec.proxy and waits for the operator to
// settle. It deliberately does NOT wait for a Deployment rollout: the proxy may already be
// cleared (e.g. a test that ends by removing it), in which case no new rollout occurs and a
// rollout wait would block until timeout.
func (t ExternalOIDCComponentProxyTests) deferResetProxy() {
	g.DeferCleanup(func(ctx context.Context) error {
		if err := updateAuthenticationProxy(ctx, t.oc, NoProxyConfig); err != nil {
			return err
		}
		return waitForOperatorToPickUpChanges(ctx, t.oc, clusterAuthenticationOperatorName)
	})
}

// addTrustedCA creates a trustedCA ConfigMap from certPEM, registers its cleanup, and sets
// it on proxyConfig. It is a no-op when certPEM is empty.
func (t ExternalOIDCComponentProxyTests) addTrustedCA(ctx context.Context, proxyConfig *operatorv1.AuthenticationProxyConfig, certPEM []byte) {
	g.GinkgoHelper()
	if len(certPEM) == 0 {
		return
	}
	g.By("Creating trustedCA ConfigMap in openshift-config")
	cmName, cmCleanup, err := createTrustedCAConfigMap(ctx, t.oc, certPEM)
	g.DeferCleanup(cmCleanup)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create trustedCA ConfigMap")
	proxyConfig.TrustedCA = operatorv1.AuthenticationConfigMapReference{Name: cmName}
}

// testExternalOIDCOperandProxyConfiguration verifies the authentication operator propagates
// spec.proxy env vars (and, when trustedCACertPEM is set, the trustedCA volume/mount and
// synced ConfigMap) onto the oauth-apiserver operand.
func (t ExternalOIDCComponentProxyTests) testExternalOIDCOperandProxyConfiguration(
	ctx context.Context,
	proxyURL string,
	trustedCACertPEM []byte,
) {
	withTrustedCA := len(trustedCACertPEM) > 0

	proxyConfig := operatorv1.AuthenticationProxyConfig{HTTPSProxy: proxyURL}
	t.addTrustedCA(ctx, &proxyConfig, trustedCACertPEM)

	g.By("Setting component-scoped proxy")
	err := t.waitForOperatorToSettleAndDeploymentRollout(ctx, func() error {
		return updateAuthenticationProxy(ctx, t.oc, proxyConfig)
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should set proxy config")
	t.deferResetProxy()

	if withTrustedCA {
		g.By("Verifying trustedCA ConfigMap is synced to the oauth-apiserver namespace")
		err = verifyTrustedCAConfigMapSyncedToNamespace(ctx, t.oc, oauthAPIServerNamespace)
		o.Expect(err).
			NotTo(o.HaveOccurred(), "CA ConfigMap should sync")
	}

	g.By("Verifying operator propagated proxy env vars (and trustedCA volume/mount) to the oauth-apiserver operand")
	err = t.proxyVerifier.
		AssertProxyConfig(ctx, "", proxyURL, componentProxyBaseNoProxy, withTrustedCA)
	o.Expect(err).NotTo(o.HaveOccurred(), "proxy env vars should propagate")
}

// testExternalOIDCFallbackOnProxyRemoval verifies the operand gets the proxy env vars when
// spec.proxy is set and loses them when spec.proxy is cleared. The Squid namespace is not
// deleted (Keycloak is co-located there); clearing the env vars is the proof.
func (t ExternalOIDCComponentProxyTests) testExternalOIDCFallbackOnProxyRemoval(
	ctx context.Context,
	proxyURL string,
) {
	proxyConfig := operatorv1.AuthenticationProxyConfig{HTTPSProxy: proxyURL}

	g.By("Setting component-scoped proxy")
	err := t.waitForOperatorToSettleAndDeploymentRollout(ctx, func() error {
		return updateAuthenticationProxy(ctx, t.oc, proxyConfig)
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should set proxy config")
	t.deferResetProxy()

	g.By("Verifying proxy env vars are set on the oauth-apiserver deployment")
	err = t.proxyVerifier.
		AssertProxyConfig(ctx, "", proxyURL, componentProxyBaseNoProxy, false)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "proxy env vars should be set")

	g.By("Removing spec.proxy from Authentication CR")
	err = t.waitForOperatorToSettleAndDeploymentRollout(ctx, func() error {
		return updateAuthenticationProxy(ctx, t.oc, NoProxyConfig)
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should remove proxy config")

	g.By("Verifying proxy env vars are no longer set on the oauth-apiserver deployment")
	err = t.proxyVerifier.
		AssertNoConfig(ctx)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "proxy env vars should be cleared")
}

// testExternalOIDCOperatorDegradedOnMissingCA verifies that a spec.proxy.trustedCA pointing
// at a non-existent ConfigMap drives the authentication operator Degraded=True, and that
// clearing the proxy lets it recover.
func (t ExternalOIDCComponentProxyTests) testExternalOIDCOperatorDegradedOnMissingCA(
	ctx context.Context,
	proxyURL string,
) {
	proxyConfig := operatorv1.AuthenticationProxyConfig{
		HTTPSProxy: proxyURL,
		TrustedCA:  operatorv1.AuthenticationConfigMapReference{Name: fmt.Sprintf("does-not-exist-%s", rand.String(8))},
	}

	// This is a negative path: with a missing trustedCA the operator goes Degraded and never
	// completes a Deployment rollout, so do not use the rollout wrapper here (it would block
	// until timeout). Apply the config directly and assert on the operator condition instead.
	g.By("Setting proxy with a trustedCA referencing a missing ConfigMap")
	err := updateAuthenticationProxy(ctx, t.oc, proxyConfig)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should set proxy config")
	t.deferResetProxy()

	g.By("Verifying the authentication cluster operator goes Degraded=True")
	err = waitForClusterOperatorDegraded(ctx, t.oc, clusterAuthenticationOperatorName, true)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "authentication operator should go Degraded when the trustedCA ConfigMap is missing")

	g.By("Removing the invalid proxy configuration")
	err = updateAuthenticationProxy(ctx, t.oc, NoProxyConfig)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should clear proxy config")

	g.By("Verifying the authentication cluster operator recovers to Degraded=False")
	err = waitForClusterOperatorDegraded(ctx, t.oc, clusterAuthenticationOperatorName, false)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "authentication operator should recover after clearing the invalid proxy")
}

// testExternalOIDCOperatorResyncsTrustedCA verifies the operator syncs the initial trustedCA
// content into the operand namespace and re-syncs it when the source ConfigMap changes.
func (t ExternalOIDCComponentProxyTests) testExternalOIDCOperatorResyncsTrustedCA(
	ctx context.Context,
	proxyURL string,
	caCertPEM []byte,
) {
	g.By("Creating trustedCA ConfigMap in openshift-config")
	configMapName, cmCleanup, err := createTrustedCAConfigMap(ctx, t.oc, caCertPEM)
	g.DeferCleanup(cmCleanup)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create CA ConfigMap")

	proxyConfig := operatorv1.AuthenticationProxyConfig{
		HTTPSProxy: proxyURL,
		TrustedCA:  operatorv1.AuthenticationConfigMapReference{Name: configMapName},
	}
	g.By("Setting component-scoped proxy with trustedCA")
	err = t.waitForOperatorToSettleAndDeploymentRollout(ctx, func() error {
		return updateAuthenticationProxy(ctx, t.oc, proxyConfig)
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should set proxy config")
	t.deferResetProxy()

	g.By("Verifying the operator synced the initial trustedCA content into the oauth-apiserver namespace")
	err = waitForTrustedCAConfigMapContent(ctx, t.oc, oauthAPIServerNamespace, string(caCertPEM))
	o.Expect(err).
		NotTo(o.HaveOccurred(), "CA content should sync")

	g.By("Generating a new CA and updating the source trustedCA ConfigMap")
	newCAConfig, err := libcrypto.MakeSelfSignedCAConfigForDuration("squid-proxy-ca-resync", 2*time.Hour)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should generate new CA")
	newCACertPEM, _, err := newCAConfig.GetPEMBytes()
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get CA PEM bytes")

	cm, err := t.oc.AdminKubeClient().
		CoreV1().
		ConfigMaps("openshift-config").
		Get(ctx, configMapName, metav1.GetOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get CA ConfigMap")

	cm.Data["ca-bundle.crt"] = string(newCACertPEM)
	_, err = t.oc.AdminKubeClient().
		CoreV1().
		ConfigMaps("openshift-config").
		Update(ctx, cm, metav1.UpdateOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should update CA ConfigMap")

	g.By("Verifying the operator re-syncs the new trustedCA content into the oauth-apiserver namespace")
	err = waitForTrustedCAConfigMapContent(ctx, t.oc, oauthAPIServerNamespace, string(newCACertPEM))
	o.Expect(err).
		NotTo(o.HaveOccurred(), "operator should re-sync updated trustedCA content to the operand namespace")
}

// testExternalOIDCClaimsSourcingThroughProxy verifies that, with the proxy set, the
// oauth-apiserver sources external groups from the Keycloak /userinfo endpoint through the
// Squid proxy during an external-OIDC login.
func (t ExternalOIDCComponentProxyTests) testExternalOIDCClaimsSourcingThroughProxy(
	ctx context.Context,
	keycloakCli *keycloakClient,
	keycloakNamespace, proxyNamespace, username, password, group, proxyURL string,
	trustedCACertPEM []byte) {
	withTrustedCA := len(trustedCACertPEM) > 0
	hostname := keycloakHostname(ctx, t.oc, keycloakNamespace)

	g.By("Configuring external-claims Keycloak client")
	configureExternalClaimsKeycloakClient(keycloakCli)

	proxyConfig := operatorv1.AuthenticationProxyConfig{HTTPSProxy: proxyURL}
	t.addTrustedCA(ctx, &proxyConfig, trustedCACertPEM)

	g.By("Setting component-scoped proxy")
	err := t.waitForOperatorToSettleAndDeploymentRollout(ctx, func() error {
		return updateAuthenticationProxy(ctx, t.oc, proxyConfig)
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should set proxy config")
	t.deferResetProxy()

	if withTrustedCA {
		g.By("Verifying trustedCA ConfigMap is synced to the oauth-apiserver namespace")
		err = verifyTrustedCAConfigMapSyncedToNamespace(ctx, t.oc, oauthAPIServerNamespace)
		o.Expect(err).
			NotTo(o.HaveOccurred(), "CA ConfigMap should sync")
	}

	g.By("Verifying oauth-apiserver deployment has proxy env vars")
	err = t.proxyVerifier.
		AssertProxyConfig(ctx, "", proxyURL, componentProxyBaseNoProxy, withTrustedCA)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "proxy env vars should be set")

	// Changing ExternalClaimsSources edits authentications.config/cluster, which rolls the
	// kube-apiserver by revision, not the oauth-apiserver Deployment generation. Do not use
	// the Deployment-rollout wrapper here; wait for healthy OIDC clients instead.
	g.By("Configuring external claims sourcing from Keycloak /userinfo")
	err = updateExistingOIDCProvider(ctx, t.oc, externalClaimsGroupsModifier(hostname))
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should configure external claims")
	g.DeferCleanup(func(ctx context.Context) error {
		return updateExistingOIDCProvider(ctx, t.oc, nil)
	})
	waitForHealthyOIDCClients(ctx, t.oc)

	logCutOff := time.Now()

	g.By("Performing external-OIDC login (groups sourced through the proxy)")
	assertExternalOIDCLoginGroups(ctx, t.oc, keycloakCli, externalClaimsKeycloakClientID, username, password, fmt.Sprintf("%s@payload.openshift.io", username), group)

	g.By("Verifying oauth-apiserver traffic to Keycloak went through the Squid proxy")
	ips, err := getOAuthAPIServerPodIPs(ctx, t.oc)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should be able to get oauth-apiserver pod ips")
	err = waitForProxyTrafficFromTo(ctx, t.oc, proxyNamespace, ips, hostname, logCutOff, 5*time.Minute)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "traffic should go through proxy")
}

// testExternalOIDCHotReloadCAFileChange verifies a rotated proxy CA is picked up from the
// mounted file without rolling the oauth-apiserver Deployment: it asserts the Deployment
// generation is unchanged across the CA rotation while login keeps working.
func (t ExternalOIDCComponentProxyTests) testExternalOIDCHotReloadCAFileChange(
	ctx context.Context,
	keycloakCli *keycloakClient,
	keycloakNamespace, proxyNamespace, username, password, group, httpsProxyURL string,
	caCertPEM []byte,
) {
	kubeClient := t.oc.AdminKubeClient()
	hostname := keycloakHostname(ctx, t.oc, keycloakNamespace)

	g.By("Configuring external-claims Keycloak client")
	configureExternalClaimsKeycloakClient(keycloakCli)

	g.By("Creating trustedCA ConfigMap in openshift-config")
	configMapName, cmCleanup, err := createTrustedCAConfigMap(ctx, t.oc, caCertPEM)
	g.DeferCleanup(cmCleanup)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should create CA ConfigMap")

	proxyConfig := operatorv1.AuthenticationProxyConfig{
		HTTPSProxy: httpsProxyURL,
		TrustedCA:  operatorv1.AuthenticationConfigMapReference{Name: configMapName},
	}
	g.By("Setting component-scoped proxy with trustedCA and waiting for oauth-apiserver rollout")
	err = t.waitForOperatorToSettleAndDeploymentRollout(ctx, func() error {
		return updateAuthenticationProxy(ctx, t.oc, proxyConfig)
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should set proxy config")
	t.deferResetProxy()

	g.By("Verifying trustedCA ConfigMap is synced to the oauth-apiserver namespace")
	err = verifyTrustedCAConfigMapSyncedToNamespace(ctx, t.oc, oauthAPIServerNamespace)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "CA ConfigMap should sync")

	g.By("Configuring external claims sourcing from Keycloak /userinfo")
	err = updateExistingOIDCProvider(ctx, t.oc, externalClaimsGroupsModifier(hostname))
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should configure external claims")
	g.DeferCleanup(func(ctx context.Context) error {
		return updateExistingOIDCProvider(ctx, t.oc, nil)
	})
	waitForHealthyOIDCClients(ctx, t.oc)

	g.By("Verifying oauth-apiserver deployment has proxy env vars and trustedCA volume/mount")
	err = t.proxyVerifier.
		AssertProxyConfig(ctx, "", httpsProxyURL, componentProxyBaseNoProxy, true)
	o.Expect(err).NotTo(o.HaveOccurred(), "proxy config should be present")

	g.By("Verifying login works before CA rotation")
	assertExternalOIDCLoginGroups(ctx, t.oc, keycloakCli, externalClaimsKeycloakClientID, username, password, fmt.Sprintf("%s@payload.openshift.io", username), group)

	g.By("Recording oauth-apiserver Deployment generation before CA rotation")
	deployment, err := kubeClient.
		AppsV1().
		Deployments(oauthAPIServerNamespace).
		Get(ctx, oauthAPIServerName, metav1.GetOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get deployment")
	generationBeforeCARotation := deployment.Generation

	g.By("Rotating CA: generating new CA and server cert")
	newCAConfig, err := libcrypto.MakeSelfSignedCAConfigForDuration("squid-proxy-ca", 2*time.Hour)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should create new CA")
	newCA := &libcrypto.CA{Config: newCAConfig, SerialGenerator: &libcrypto.RandomSerialGenerator{}}

	serviceDNS := fmt.Sprintf("%s.%s.svc.cluster.local", squidServiceName, proxyNamespace)
	newServerCertConfig, err := newCA.MakeServerCert(sets.New(serviceDNS), 2*time.Hour)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should create server cert")

	newCACertPEM, _, err := newCAConfig.GetPEMBytes()
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get CA PEM")
	newServerCertPEM, newServerKeyPEM, err := newServerCertConfig.GetPEMBytes()
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get server cert PEM")

	g.By("Updating squid-tls Secret with rotated cert")
	tlsSecret, err := kubeClient.CoreV1().
		Secrets(proxyNamespace).
		Get(ctx, "squid-tls", metav1.GetOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get TLS secret")
	tlsSecret.Data["tls.crt"] = newServerCertPEM
	tlsSecret.Data["tls.key"] = newServerKeyPEM
	_, err = kubeClient.CoreV1().
		Secrets(proxyNamespace).
		Update(ctx, tlsSecret, metav1.UpdateOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should update TLS secret")

	squidPods, err := kubeClient.CoreV1().
		Pods(proxyNamespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: "app=squid-proxy",
		})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should list squid pods")
	o.Expect(squidPods.Items).
		NotTo(o.BeEmpty())

	g.By("Waiting for squid-tls Secret to propagate to pod volume")
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 3*time.Minute, true, func(ctx context.Context) (done bool, err error) {
		return podFileContentMatches(t.oc, squidPods.Items[0], "squid", "/etc/squid/tls/tls.crt", newServerCertPEM)
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "squid-tls Secret should propagate to pod volume")

	g.By("Reconfiguring Squid to pick up new cert")
	output, err := t.oc.AsAdmin().
		Run("exec").
		Args(
			"-n", proxyNamespace,
			squidPods.Items[0].Name,
			"-c", "squid",
			"--", "/usr/sbin/squid", "-k", "reconfigure",
		).
		Output()
	o.Expect(err).
		NotTo(o.HaveOccurred(), "squid reconfigure failed: %s", string(output))

	g.By("Updating trustedCA ConfigMap with new CA")
	cm, err := kubeClient.CoreV1().
		ConfigMaps("openshift-config").
		Get(ctx, configMapName, metav1.GetOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get CA ConfigMap")
	cm.Data["ca-bundle.crt"] = string(newCACertPEM)
	_, err = kubeClient.CoreV1().
		ConfigMaps("openshift-config").
		Update(ctx, cm, metav1.UpdateOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should update CA ConfigMap")

	oauthAPIServerPods, err := kubeClient.CoreV1().
		Pods(oauthAPIServerNamespace).
		List(ctx, metav1.ListOptions{
			LabelSelector: "app=openshift-oauth-apiserver",
		})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should list oauth-apiserver pods")
	o.Expect(oauthAPIServerPods.Items).
		NotTo(o.BeEmpty(), "pods should exist")

	g.By("Waiting for oauth-apiserver pods to pick up the new CA file")
	caFilePath := componentProxyCAMountPath + "/ca-bundle.crt"
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		for _, pod := range oauthAPIServerPods.Items {
			success, err := podFileContentMatches(t.oc, pod, oauthAPIServerName, caFilePath, newCACertPEM)
			if success {
				continue
			}
			if err != nil {
				g.GinkgoWriter.Printf("error waiting for %s/%s pod to pick up the new CA file\n", pod.Namespace, pod.Name)
			}
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "oauth-apiserver pods should have picked up the new CA file")

	g.By("Verifying login still works after CA rotation")
	assertExternalOIDCLoginGroups(ctx, t.oc, keycloakCli, externalClaimsKeycloakClientID, username, password, fmt.Sprintf("%s@payload.openshift.io", username), group)

	g.By("Verifying oauth-apiserver Deployment was not updated after CA rotation")
	deployment, err = kubeClient.AppsV1().
		Deployments(oauthAPIServerNamespace).
		Get(ctx, oauthAPIServerName, metav1.GetOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get deployment")
	o.Expect(deployment.Generation).
		To(o.Equal(generationBeforeCARotation), "oauth-apiserver Deployment should not have been updated after CA file change")

	g.By("Verifying operator re-syncs promptly after trustedCA ConfigMap update")
	err = operator.WaitForOperatorsToSettle(ctx, t.oc.AdminConfigClient(), 1)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "operator should settle")
}

// testExternalOIDCBypassNoProxyHost verifies that when spec.proxy.noProxy contains the IdP
// host, external-OIDC login still works and the oauth-apiserver reaches Keycloak directly
// (no Squid CONNECT for that host).
func (t ExternalOIDCComponentProxyTests) testExternalOIDCBypassNoProxyHost(
	ctx context.Context,
	keycloakCli *keycloakClient,
	keycloakNamespace, proxyNamespace, username, password, group, httpProxyURL, httpsProxyURL string,
	caCertPEM []byte,
) {
	hostname := keycloakHostname(ctx, t.oc, keycloakNamespace)

	g.By("Configuring external-claims Keycloak client")
	configureExternalClaimsKeycloakClient(keycloakCli)

	g.By("Creating trustedCA ConfigMap in openshift-config")
	configMapName, cmCleanup, err := createTrustedCAConfigMap(ctx, t.oc, caCertPEM)
	g.DeferCleanup(cmCleanup)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should create CA ConfigMap")

	proxyConfig := operatorv1.AuthenticationProxyConfig{
		HTTPProxy:  httpProxyURL,
		HTTPSProxy: httpsProxyURL,
		TrustedCA:  operatorv1.AuthenticationConfigMapReference{Name: configMapName},
		NoProxy:    []string{hostname},
	}

	g.By("Setting component-scoped proxy with noProxy containing the IdP host")
	err = t.waitForOperatorToSettleAndDeploymentRollout(ctx, func() error {
		return updateAuthenticationProxy(ctx, t.oc, proxyConfig)
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should set proxy config")
	t.deferResetProxy()

	g.By("Verifying oauth-apiserver has HTTP_PROXY, HTTPS_PROXY, and NO_PROXY with the IdP host")
	err = t.proxyVerifier.AssertProxyConfig(ctx, httpProxyURL, httpsProxyURL, componentProxyBaseNoProxy+","+hostname, true)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "proxy config should include IdP in NO_PROXY")

	g.By("Configuring external claims sourcing from Keycloak /userinfo")
	err = updateExistingOIDCProvider(ctx, t.oc, externalClaimsGroupsModifier(hostname))
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should configure external claims")
	g.DeferCleanup(func(ctx context.Context) error {
		return updateExistingOIDCProvider(ctx, t.oc, nil)
	})
	waitForHealthyOIDCClients(ctx, t.oc)

	logCutOff := time.Now()

	g.By("Verifying external-OIDC login still works (claims sourced directly, bypassing the proxy)")
	assertExternalOIDCLoginGroups(ctx, t.oc, keycloakCli, externalClaimsKeycloakClientID, username, password, fmt.Sprintf("%s@payload.openshift.io", username), group)

	g.By("Verifying oauth-apiserver traffic to Keycloak did NOT go through the Squid proxy")
	ips, err := getOAuthAPIServerPodIPs(ctx, t.oc)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should be able to get oauth-apiserver pod ips")
	err = assertNoProxyTrafficFromTo(ctx, t.oc, proxyNamespace, ips, hostname, logCutOff, 30*time.Second)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "traffic should bypass proxy")
}

// configureExternalClaimsKeycloakClient (re-)authenticates as the Keycloak admin and configures a
// dedicated client whose tokens exclude groups so that groups are proven to be sourced from the
// /userinfo endpoint (and therefore through the component proxy).
func configureExternalClaimsKeycloakClient(keycloakCli *keycloakClient) {
	g.GinkgoHelper()
	o.Expect(keycloakCli.Authenticate("admin-cli", keycloakAdminUsername, keycloakAdminPassword)).
		NotTo(o.HaveOccurred(), "should authenticate as keycloak admin")
	o.Expect(keycloakCli.ConfigureClientForExternalClaims(externalClaimsKeycloakClientID)).
		NotTo(o.HaveOccurred(), "should configure a Keycloak client for external claims testing")
}

// externalClaimsGroupsModifier sources the groups claim from the Keycloak /userinfo endpoint so
// that an authentication forces the oauth-apiserver to call Keycloak.
func externalClaimsGroupsModifier(hostname string) func(*configv1.OIDCProvider) {
	return func(provider *configv1.OIDCProvider) {
		provider.ExternalClaimsSources = []configv1.ExternalClaimsSource{
			keycloakUserInfoClaimsSource(hostname, configv1.SourcedClaimMapping{
				Name:       "groups",
				Expression: "response.body.groups.join(',')",
			}),
		}
		provider.ClaimMappings.Groups = configv1.PrefixedClaimMapping{
			TokenClaimMapping: configv1.TokenClaimMapping{
				Expression: "claims.?groups.orValue('').split(',').filter(g, size(g) > 0)",
			},
		}
	}
}

// assertExternalOIDCLoginGroups authenticates against Keycloak and asserts the resulting
// SelfSubjectReview maps the expected username and contains the expected groups.
func assertExternalOIDCLoginGroups(ctx context.Context, oc *exutil.CLI, keycloakCli *keycloakClient, clientID, username, password, expectedUsername string, expectedGroups ...string) {
	g.GinkgoHelper()
	o.Eventually(func(gomega o.Gomega) {
		err := keycloakCli.Authenticate(clientID, username, password)
		gomega.Expect(err).NotTo(o.HaveOccurred(), "should authenticate as keycloak user")

		copiedOC := *oc
		tokenOC := copiedOC.WithToken(keycloakCli.AccessToken())
		ssr, err := tokenOC.KubeClient().AuthenticationV1().SelfSubjectReviews().Create(ctx, &authnv1.SelfSubjectReview{
			ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-info", username)},
		}, metav1.CreateOptions{})
		gomega.Expect(err).NotTo(o.HaveOccurred(), "should be able to create a SelfSubjectReview")
		gomega.Expect(ssr.Status.UserInfo.Username).To(o.Equal(expectedUsername))
		for _, grp := range expectedGroups {
			gomega.Expect(ssr.Status.UserInfo.Groups).To(o.ContainElement(grp))
		}
	}).WithTimeout(5 * time.Minute).WithPolling(10 * time.Second).Should(o.Succeed())
}

// getOAuthAPIServerPodIPs returns the pod IPs of the running oauth-apiserver pods, or
// an error if none are found.
func getOAuthAPIServerPodIPs(ctx context.Context, oc *exutil.CLI) ([]string, error) {
	g.GinkgoHelper()
	pods, err := oc.AdminKubeClient().CoreV1().Pods(oauthAPIServerNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=openshift-oauth-apiserver",
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) < 1 {
		return nil, fmt.Errorf("the number of oauth-apiserver pods should be greater than zero")
	}
	var ips []string
	for _, p := range pods.Items {
		ips = append(ips, p.Status.PodIP)
	}
	return ips, nil
}

// waitForClusterOperatorDegraded polls the named ClusterOperator until its Degraded condition
// matches the desired status (true => Degraded=True, false => Degraded=False).
func waitForClusterOperatorDegraded(ctx context.Context, oc *exutil.CLI, name string, degraded bool) error {
	want := configv1.ConditionFalse
	if degraded {
		want = configv1.ConditionTrue
	}
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		for _, cond := range co.Status.Conditions {
			if cond.Type == configv1.OperatorDegraded {
				return cond.Status == want, nil
			}
		}
		return false, nil
	})
}

// waitForTrustedCAConfigMapContent polls the synced trustedCA ConfigMap in the given namespace until
// its ca-bundle.crt content matches expected.
func waitForTrustedCAConfigMapContent(ctx context.Context, oc *exutil.CLI, namespace, expected string) error {
	want := strings.TrimSpace(expected)
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, componentProxyCAConfigMapName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return strings.TrimSpace(cm.Data["ca-bundle.crt"]) == want, nil
	})
}

// verifyTrustedCAConfigMapSyncedToNamespace mirrors verifyTrustedCAConfigMapSynced but targets an
// arbitrary namespace (the oauth-apiserver reuses the same componentProxyCAConfigMapName).
func verifyTrustedCAConfigMapSyncedToNamespace(ctx context.Context, oc *exutil.CLI, namespace string) error {
	kubeClient := oc.AdminKubeClient()
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		cm, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, componentProxyCAConfigMapName, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		return len(cm.Data) > 0, nil
	})
}

// assertNoProxyTrafficFromTo waits a settle period and then confirms the Squid access log contains
// no CONNECT from any of the given source IPs to destHost since the given time.
func assertNoProxyTrafficFromTo(ctx context.Context, oc *exutil.CLI, proxyNamespace string, sourceIPs []string, destHost string, since time.Time, settle time.Duration) error {
	g.GinkgoHelper()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(settle):
	}

	logs, err := getSquidProxyLogsSince(ctx, oc, proxyNamespace, since)
	if err != nil {
		return fmt.Errorf("reading squid logs: %w", err)
	}
	allowedIPs := sets.New(sourceIPs...)
	for line := range strings.SplitSeq(logs, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 3 && parts[0] == "CONNECT" && strings.HasPrefix(parts[1], destHost) && allowedIPs.Has(parts[2]) {
			return fmt.Errorf("unexpected proxy traffic from %s to %s: %s", parts[2], destHost, line)
		}
	}
	return nil
}
