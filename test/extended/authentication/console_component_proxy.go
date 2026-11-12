package authentication

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	operatorv1 "github.com/openshift/api/operator/v1"
	libcrypto "github.com/openshift/library-go/pkg/crypto"

	"github.com/openshift/origin/test/extended/authentication/component_proxy"
	"github.com/openshift/origin/test/extended/authentication/operator_wait"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	consoleNamespace        = "openshift-console"
	consoleDeploymentName   = "console"
	consoleOperatorName     = "console"
	consoleConfigMapName    = "console-config"
	consoleConfigFileName   = "console-config.yaml"
	consoleContainerName    = "console"
	consolePodLabelSelector = "app=console"

	// The three constants below mirror the console-operator api package: the mount
	// directory, the bundle file name, and the path the rendered console-config.yaml
	// references in auth.proxy.trustedCAFilePath.
	consoleAuthProxyCAMountPath = "/var/auth-proxy-ca"
	consoleAuthProxyCAFileName  = "ca-bundle.crt"
	consoleAuthProxyCAFilePath  = consoleAuthProxyCAMountPath + "/" + consoleAuthProxyCAFileName
)

// NoConsoleProxyConfig is the empty Console auth-proxy config used to clear spec.authProxy.
func NoConsoleProxyConfig() operatorv1.ConsoleAuthProxyConfig {
	return operatorv1.ConsoleAuthProxyConfig{}
}

// generateSelfSignedCAPEM returns a fresh self-signed CA certificate in PEM form.
func generateSelfSignedCAPEM(commonName string) ([]byte, error) {
	caConfig, err := libcrypto.MakeSelfSignedCAConfigForDuration(commonName, 2*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("creating CA config: %w", err)
	}
	caCertPEM, _, err := caConfig.GetPEMBytes()
	if err != nil {
		return nil, fmt.Errorf("encoding CA cert: %w", err)
	}
	return caCertPEM, nil
}

// updateConfigMapKey sets key to value in the given ConfigMap.
func updateConfigMapKey(ctx context.Context, oc *exutil.CLI, namespace, name, key, value string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if cm.Data == nil {
			cm.Data = map[string]string{}
		}
		cm.Data[key] = value
		_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(namespace).Update(ctx, cm, metav1.UpdateOptions{})
		return err
	})
}

// updateSecretKeys merges data into the given Secret.
func updateSecretKeys(ctx context.Context, oc *exutil.CLI, namespace, name string, data map[string][]byte) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		secret, err := oc.AdminKubeClient().CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		for k, v := range data {
			secret.Data[k] = v
		}
		_, err = oc.AdminKubeClient().CoreV1().Secrets(namespace).Update(ctx, secret, metav1.UpdateOptions{})
		return err
	})
}

// squidReconfigure sends the squid process a reconfigure signal so it picks up a rotated
// serving certificate without a pod restart. It polls for a running squid pod first because
// the pod list may lag the rollout.
// TODO: this Squid part changes with PR 31837; adapt to its helpers when it lands.
func squidReconfigure(ctx context.Context, oc *exutil.CLI, proxyNamespace string) error {
	var pods []corev1.Pod
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		podList, err := oc.AdminKubeClient().CoreV1().Pods(proxyNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=squid-proxy",
		})
		if err != nil {
			return false, nil
		}
		if len(podList.Items) < 1 {
			return false, nil
		}
		pods = podList.Items
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("listing squid pods: %w", err)
	}
	output, err := oc.AsAdmin().
		Run("exec").
		Args("-n", proxyNamespace, pods[0].Name, "-c", "squid", "--", "/usr/sbin/squid", "-k", "reconfigure").
		Output()
	if err != nil {
		return fmt.Errorf("squid reconfigure failed: %s: %w", string(output), err)
	}
	return nil
}

// updateConsoleAuthProxy sets (or, with the zero config, clears) spec.authProxy on
// console.operator.openshift.io/cluster.
func updateConsoleAuthProxy(ctx context.Context, oc *exutil.CLI, proxy operatorv1.ConsoleAuthProxyConfig) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		con, err := oc.AdminOperatorClient().OperatorV1().Consoles().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		con.Spec.AuthProxy = proxy
		_, err = oc.AdminOperatorClient().OperatorV1().Consoles().Update(ctx, con, metav1.UpdateOptions{})
		return err
	})
}

// getConsolePodIPs returns the pod IPs of the running console pods, or an error if none
// are found.
func getConsolePodIPs(ctx context.Context, oc *exutil.CLI) ([]string, error) {
	g.GinkgoHelper()
	pods, err := oc.AdminKubeClient().CoreV1().Pods(consoleNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: consolePodLabelSelector,
	})
	if err != nil {
		return nil, err
	}
	if len(pods.Items) < 1 {
		return nil, fmt.Errorf("the number of console pods should be greater than zero")
	}
	var ips []string
	for _, p := range pods.Items {
		ips = append(ips, p.Status.PodIP)
	}
	return ips, nil
}

// ConsoleComponentProxyTests groups the console auth-proxy test cases and their shared
// collaborators: the cluster handle, the composed operator-settle + deployment-rollout
// waiter, and the config-file proxy verifier.
//
// The verifier carries both targets: the config-file target for the rendered
// auth.proxy block, and the Deployment target for the trustedCA volume/mount.
type ConsoleComponentProxyTests struct {
	oc                                          *exutil.CLI
	waitForOperatorToSettleAndDeploymentRollout operator_wait.Wrapper
	proxyVerifier                               component_proxy.ConfigFileProxyVerifier
}

// NewConsoleComponentProxyTests builds the console test group for oc. It touches no cluster
// state, so the returned error only reflects construction failures.
func NewConsoleComponentProxyTests(oc *exutil.CLI) (ConsoleComponentProxyTests, error) {
	toReturn := ConsoleComponentProxyTests{
		oc: oc,
	}

	consoleRolloutWaiter, err := operator_wait.NewRolloutWaiter(consoleNamespace, consoleDeploymentName, oc)
	if err != nil {
		return ConsoleComponentProxyTests{}, fmt.Errorf("error creating rollout waiter: %w", err)
	}

	consoleOperatorWaiter, err := operator_wait.NewOperatorChangesWaiter(consoleOperatorName, oc)
	if err != nil {
		return ConsoleComponentProxyTests{}, fmt.Errorf("error creating operator changes waiter: %w", err)
	}

	toReturn.waitForOperatorToSettleAndDeploymentRollout = operator_wait.Chain(
		consoleRolloutWaiter.MutateAndWaitForDeploymentRollout,
		consoleOperatorWaiter.WaitChanges,
	)

	proxyVerifier, err := component_proxy.NewConfigFileProxyVerifier(
		consoleNamespace,
		consoleDeploymentName,
		oc,
		component_proxy.WithConfigFileTarget(
			consoleNamespace,
			consoleConfigMapName,
			consoleConfigFileName,
			consoleAuthProxyCAFilePath,
		),
	)
	if err != nil {
		return toReturn, fmt.Errorf("error on creating console config-file proxy verifier: %w", err)
	}

	toReturn.proxyVerifier = proxyVerifier

	return toReturn, nil
}

// deferResetProxy registers a cleanup that clears spec.authProxy and waits for the operator
// to settle. Like the oauth-apiserver group it deliberately does NOT wait for a Deployment
// rollout: the proxy may already be cleared, in which case no new rollout occurs.
func (t ConsoleComponentProxyTests) deferResetProxy() {
	g.DeferCleanup(func(ctx context.Context) error {
		if err := updateConsoleAuthProxy(ctx, t.oc, NoConsoleProxyConfig()); err != nil {
			return err
		}
		return operator_wait.WaitForOperatorToPickUpChanges(ctx, t.oc, consoleOperatorName)
	})
}

// setProxy applies the given auth-proxy config through the rollout and settle waiter and
// registers the cleanup that clears it.
func (t ConsoleComponentProxyTests) setProxy(ctx context.Context, proxyConfig operatorv1.ConsoleAuthProxyConfig) {
	g.GinkgoHelper()
	g.By("Setting console auth proxy configuration")
	err := t.waitForOperatorToSettleAndDeploymentRollout(ctx, func() error {
		return updateConsoleAuthProxy(ctx, t.oc, proxyConfig)
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should set console auth proxy config")
	t.deferResetProxy()
}

// testConsoleOperandProxyConfiguration verifies the console-operator renders spec.authProxy
// into the auth.proxy block of console-config.yaml and, when trustedCA is set, syncs the CA
// ConfigMap into openshift-console and mounts it on the console Deployment.
func (t ConsoleComponentProxyTests) testConsoleOperandProxyConfiguration(
	ctx context.Context,
	proxyURL string,
	trustedCACertPEM []byte,
) {
	withTrustedCA := len(trustedCACertPEM) > 0

	proxyConfig := operatorv1.ConsoleAuthProxyConfig{HTTPSProxy: proxyURL}
	if withTrustedCA {
		cmName, cmCleanup, err := createTrustedCAConfigMap(ctx, t.oc, trustedCACertPEM)
		g.DeferCleanup(cmCleanup)
		o.Expect(err).NotTo(o.HaveOccurred(), "should create trustedCA ConfigMap")
		proxyConfig.TrustedCA = operatorv1.ConsoleAuthProxyTrustedCAConfigMapReference{Name: cmName}
	}
	t.setProxy(ctx, proxyConfig)

	err := t.proxyVerifier.AssertProxyConfigInFile(ctx, "", proxyURL, component_proxy.DefaultNoProxy, withTrustedCA)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console config should carry the auth.proxy block")

	if withTrustedCA {
		g.By("Verifying the CA ConfigMap is synced to openshift-console and mounted")
		err = waitForTrustedCAConfigMapContent(ctx, t.oc, consoleNamespace, string(trustedCACertPEM))
		o.Expect(err).
			NotTo(o.HaveOccurred(), "CA content should sync to openshift-console")
		err = t.proxyVerifier.AssertDeploymentCAMount(ctx, true)
		o.Expect(err).
			NotTo(o.HaveOccurred(), "console deployment should mount the auth proxy CA")
	} else {
		err = t.proxyVerifier.AssertDeploymentCAMount(ctx, false)
		o.Expect(err).
			NotTo(o.HaveOccurred(), "console deployment should not mount the auth proxy CA")
	}
}

// testConsoleFallbackOnProxyRemoval verifies the auth.proxy block is rendered when
// spec.authProxy is set and removed when it is cleared.
func (t ConsoleComponentProxyTests) testConsoleFallbackOnProxyRemoval(ctx context.Context, proxyURL string) {
	t.setProxy(ctx, operatorv1.ConsoleAuthProxyConfig{HTTPSProxy: proxyURL})

	err := t.proxyVerifier.AssertProxyConfigInFile(ctx, "", proxyURL, component_proxy.DefaultNoProxy, false)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console config should carry the auth.proxy block")

	g.By("Removing spec.authProxy from the Console CR")
	err = t.waitForOperatorToSettleAndDeploymentRollout(ctx, func() error {
		return updateConsoleAuthProxy(ctx, t.oc, NoConsoleProxyConfig())
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should clear console auth proxy config")

	err = t.proxyVerifier.AssertNoConfigInFile(ctx)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console config should have no auth.proxy block")
	err = t.proxyVerifier.AssertDeploymentCAMount(ctx, false)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console deployment should not mount the auth proxy CA")
}

// testConsoleOperatorDegradedOnMissingCA verifies that a spec.authProxy.trustedCA pointing
// at a non-existent ConfigMap drives the console operator Degraded=True, and that creating
// the ConfigMap clears the condition.
func (t ConsoleComponentProxyTests) testConsoleOperatorDegradedOnMissingCA(
	ctx context.Context,
	proxyURL string,
	caCertPEM []byte,
) {
	proxyConfig := operatorv1.ConsoleAuthProxyConfig{
		HTTPSProxy: proxyURL,
		TrustedCA:  operatorv1.ConsoleAuthProxyTrustedCAConfigMapReference{Name: fmt.Sprintf("does-not-exist-%s", rand.String(8))},
	}

	// With a missing trustedCA the operator goes Degraded and never completes a Deployment
	// rollout, so the rollout wrapper would block until timeout; apply the config directly.
	g.By("Setting console auth proxy with a trustedCA referencing a missing ConfigMap")
	err := updateConsoleAuthProxy(ctx, t.oc, proxyConfig)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should set console auth proxy config")
	t.deferResetProxy()

	g.By("Verifying the console cluster operator goes Degraded=True")
	err = waitForClusterOperatorDegraded(ctx, t.oc, consoleOperatorName, true)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console operator should go Degraded when the trustedCA ConfigMap is missing")

	g.By("Creating the previously missing trustedCA ConfigMap")
	err = createNamedTrustedCAConfigMap(ctx, t.oc, proxyConfig.TrustedCA.Name, caCertPEM)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should create the trustedCA ConfigMap")

	g.By("Verifying the console cluster operator recovers to Degraded=False")
	err = waitForClusterOperatorDegraded(ctx, t.oc, consoleOperatorName, false)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console operator should recover after the trustedCA ConfigMap is created")

	g.By("Verifying the console config and CA mount after recovery")
	err = t.proxyVerifier.AssertProxyConfigInFile(ctx, "", proxyURL, component_proxy.DefaultNoProxy, true)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console config should carry the auth.proxy block with the CA path")
	err = t.proxyVerifier.AssertDeploymentCAMount(ctx, true)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console deployment should mount the auth proxy CA")
}

// testConsoleOperatorResyncsTrustedCA verifies the operator syncs the initial trustedCA
// content into openshift-console and re-syncs it when the source ConfigMap changes.
func (t ConsoleComponentProxyTests) testConsoleOperatorResyncsTrustedCA(
	ctx context.Context,
	proxyURL string,
	caCertPEM []byte,
) {
	g.By("Creating trustedCA ConfigMap in openshift-config")
	configMapName, cmCleanup, err := createTrustedCAConfigMap(ctx, t.oc, caCertPEM)
	g.DeferCleanup(cmCleanup)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create CA ConfigMap")

	t.setProxy(ctx, operatorv1.ConsoleAuthProxyConfig{
		HTTPSProxy: proxyURL,
		TrustedCA:  operatorv1.ConsoleAuthProxyTrustedCAConfigMapReference{Name: configMapName},
	})

	g.By("Verifying the operator synced the initial trustedCA content into openshift-console")
	err = waitForTrustedCAConfigMapContent(ctx, t.oc, consoleNamespace, string(caCertPEM))
	o.Expect(err).
		NotTo(o.HaveOccurred(), "CA content should sync")

	g.By("Generating a new CA and updating the source trustedCA ConfigMap")
	newCACertPEM, err := generateSelfSignedCAPEM("console-auth-proxy-ca-resync")
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should generate new CA")
	err = updateConfigMapKey(ctx, t.oc, "openshift-config", configMapName, consoleAuthProxyCAFileName, string(newCACertPEM))
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should update CA ConfigMap")

	g.By("Verifying the operator re-syncs the new trustedCA content into openshift-console")
	err = waitForTrustedCAConfigMapContent(ctx, t.oc, consoleNamespace, string(newCACertPEM))
	o.Expect(err).
		NotTo(o.HaveOccurred(), "operator should re-sync updated trustedCA content to openshift-console")
}

// testConsoleOIDCTrafficThroughProxy verifies that, with the auth proxy set, the console
// Deployment rollout puts fresh console pods online whose OIDC discovery traffic to the
// IdP goes through the Squid proxy.
// TODO: this Squid part changes with PR 31837; adapt to its helpers when it lands.
func (t ConsoleComponentProxyTests) testConsoleOIDCTrafficThroughProxy(
	ctx context.Context,
	keycloakNamespace, proxyNamespace, proxyURL string,
	trustedCACertPEM []byte,
) {
	withTrustedCA := len(trustedCACertPEM) > 0
	hostname := keycloakHostname(ctx, t.oc, keycloakNamespace)

	proxyConfig := operatorv1.ConsoleAuthProxyConfig{HTTPSProxy: proxyURL}
	if withTrustedCA {
		cmName, cmCleanup, err := createTrustedCAConfigMap(ctx, t.oc, trustedCACertPEM)
		g.DeferCleanup(cmCleanup)
		o.Expect(err).NotTo(o.HaveOccurred(), "should create trustedCA ConfigMap")
		proxyConfig.TrustedCA = operatorv1.ConsoleAuthProxyTrustedCAConfigMapReference{Name: cmName}
	}

	// The rollout the proxy change triggers is itself the traffic driver: fresh console
	// pods perform OIDC discovery against the issuer on startup.
	logCutOff := time.Now()
	t.setProxy(ctx, proxyConfig)

	err := t.proxyVerifier.AssertProxyConfigInFile(ctx, "", proxyURL, component_proxy.DefaultNoProxy, withTrustedCA)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console config should carry the auth.proxy block")

	// waitForHealthyOIDCClients covers the openshift-console/console entry in
	// Authentication.status.oidcClients, so this is the proxied-client health signal the
	// finished enhancement asks for.
	g.By("Verifying the console OIDC client stays healthy while proxied")
	waitForHealthyOIDCClients(ctx, t.oc)

	g.By("Verifying console pod traffic to the IdP went through the Squid proxy")
	ips, err := getConsolePodIPs(ctx, t.oc)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should be able to get console pod ips")
	err = waitForProxyTrafficFromTo(ctx, t.oc, proxyNamespace, ips, hostname, logCutOff, 5*time.Minute)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "traffic should go through proxy")
}

// testConsoleBypassNoProxyHost verifies that when spec.authProxy.noProxy contains the IdP
// host, the console reaches the IdP directly (no Squid CONNECT for that host).
// TODO: this Squid part changes with PR 31837; adapt to its helpers when it lands.
func (t ConsoleComponentProxyTests) testConsoleBypassNoProxyHost(
	ctx context.Context,
	keycloakNamespace, proxyNamespace, httpProxyURL, httpsProxyURL string,
	caCertPEM []byte,
) {
	hostname := keycloakHostname(ctx, t.oc, keycloakNamespace)

	g.By("Creating trustedCA ConfigMap in openshift-config")
	configMapName, cmCleanup, err := createTrustedCAConfigMap(ctx, t.oc, caCertPEM)
	g.DeferCleanup(cmCleanup)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create CA ConfigMap")

	logCutOff := time.Now()
	t.setProxy(ctx, operatorv1.ConsoleAuthProxyConfig{
		HTTPProxy:  httpProxyURL,
		HTTPSProxy: httpsProxyURL,
		TrustedCA:  operatorv1.ConsoleAuthProxyTrustedCAConfigMapReference{Name: configMapName},
		NoProxy:    []string{hostname},
	})

	err = t.proxyVerifier.AssertProxyConfigInFile(ctx, httpProxyURL, httpsProxyURL, component_proxy.DefaultNoProxy+","+hostname, true)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console config should include the IdP host in noProxy")

	g.By("Verifying console pod traffic to the IdP did NOT go through the Squid proxy")
	ips, err := getConsolePodIPs(ctx, t.oc)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should be able to get console pod ips")
	err = assertNoProxyTrafficFromTo(ctx, t.oc, proxyNamespace, ips, hostname, logCutOff, 30*time.Second)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "traffic should bypass proxy")
}

// testConsoleHotReloadCAFileChange verifies a rotated proxy CA is picked up from the
// mounted file without rolling the console Deployment: the CA content change updates the
// mounted file only, and the Deployment generation stays unchanged across the rotation.
// The post-rotation traffic check deletes the console pods: the restart proves the
// rotated CA works, while the unchanged generation proves the content change triggered
// no rollout. The in-place reload of the running process has no e2e driver and stays
// unproven here.
// TODO: this Squid part changes with PR 31837; adapt to its helpers when it lands.
func (t ConsoleComponentProxyTests) testConsoleHotReloadCAFileChange(
	ctx context.Context,
	keycloakNamespace, proxyNamespace, httpsProxyURL string,
	caCertPEM []byte,
) {
	kubeClient := t.oc.AdminKubeClient()
	hostname := keycloakHostname(ctx, t.oc, keycloakNamespace)

	g.By("Creating trustedCA ConfigMap in openshift-config")
	configMapName, cmCleanup, err := createTrustedCAConfigMap(ctx, t.oc, caCertPEM)
	g.DeferCleanup(cmCleanup)
	o.Expect(err).NotTo(o.HaveOccurred(), "should create CA ConfigMap")

	t.setProxy(ctx, operatorv1.ConsoleAuthProxyConfig{
		HTTPSProxy: httpsProxyURL,
		TrustedCA:  operatorv1.ConsoleAuthProxyTrustedCAConfigMapReference{Name: configMapName},
	})

	err = t.proxyVerifier.AssertProxyConfigInFile(ctx, "", httpsProxyURL, component_proxy.DefaultNoProxy, true)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console config should carry the auth.proxy block")

	g.By("Recording console Deployment generation before CA rotation")
	deployment, err := kubeClient.AppsV1().Deployments(consoleNamespace).Get(ctx, consoleDeploymentName, metav1.GetOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get console deployment")
	generationBeforeCARotation := deployment.Generation

	g.By("Rotating CA: generating new CA and squid server cert")
	newCAConfig, err := libcrypto.MakeSelfSignedCAConfigForDuration("console-squid-proxy-ca", 2*time.Hour)
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
	err = updateSecretKeys(ctx, t.oc, proxyNamespace, "squid-tls", map[string][]byte{
		"tls.crt": newServerCertPEM,
		"tls.key": newServerKeyPEM,
	})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should update TLS secret")

	g.By("Reconfiguring Squid to pick up new cert")
	err = squidReconfigure(ctx, t.oc, proxyNamespace)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "squid reconfigure should succeed")

	g.By("Updating trustedCA ConfigMap with new CA")
	err = updateConfigMapKey(ctx, t.oc, "openshift-config", configMapName, consoleAuthProxyCAFileName, string(newCACertPEM))
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should update CA ConfigMap")

	g.By("Waiting for console pods to pick up the new CA file")
	caFilePath := consoleAuthProxyCAFilePath
	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		pods, err := kubeClient.CoreV1().Pods(consoleNamespace).List(ctx, metav1.ListOptions{LabelSelector: consolePodLabelSelector})
		if err != nil {
			return false, nil
		}
		if len(pods.Items) < 1 {
			return false, nil
		}
		for _, pod := range pods.Items {
			success, err := podFileContentMatches(t.oc, pod, consoleContainerName, caFilePath, newCACertPEM)
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
		NotTo(o.HaveOccurred(), "console pods should have picked up the new CA file")

	g.By("Verifying console Deployment was not updated after CA rotation")
	deployment, err = kubeClient.AppsV1().Deployments(consoleNamespace).Get(ctx, consoleDeploymentName, metav1.GetOptions{})
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should get console deployment")
	o.Expect(deployment.Generation).
		To(o.Equal(generationBeforeCARotation), "console Deployment should not have been updated after CA file change")

	g.By("Deleting console pods to drive fresh OIDC discovery through the rotated proxy")
	logCutOff := time.Now()
	err = t.oc.AsAdmin().Run("delete").Args("pods", "-n", consoleNamespace, "-l", consolePodLabelSelector).Execute()
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should delete console pods")

	g.By("Verifying fresh console pods reach the IdP through the rotated proxy")
	ips, err := getConsolePodIPs(ctx, t.oc)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should be able to get console pod ips")
	err = waitForProxyTrafficFromTo(ctx, t.oc, proxyNamespace, ips, hostname, logCutOff, 5*time.Minute)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "traffic should go through the rotated proxy")
}

// testConsoleCredentialsInProxyURL verifies the operator renders a userinfo-bearing proxy
// URL verbatim into console-config.yaml and the console still reaches the IdP through it.
// The userinfo-redacted log line lives in console-operator logs behind klog V(4), which the
// e2e suite cannot assume is enabled, so this test asserts the config file and the traffic
// only.
// TODO: this Squid part changes with PR 31837; adapt to its helpers when it lands.
func (t ConsoleComponentProxyTests) testConsoleCredentialsInProxyURL(
	ctx context.Context,
	keycloakNamespace, proxyNamespace, proxyURL string,
) {
	hostname := keycloakHostname(ctx, t.oc, keycloakNamespace)
	credentialedProxyURL, err := proxyURLWithCredentials(proxyURL, "e2e-proxy-user", "e2e-proxy-pass")
	o.Expect(err).NotTo(o.HaveOccurred(), "should build a credentialed proxy URL")

	logCutOff := time.Now()
	t.setProxy(ctx, operatorv1.ConsoleAuthProxyConfig{HTTPSProxy: credentialedProxyURL})

	err = t.proxyVerifier.AssertProxyConfigInFile(ctx, "", credentialedProxyURL, component_proxy.DefaultNoProxy, false)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "console config should carry the credentialed auth.proxy block")

	g.By("Verifying console pod traffic to the IdP went through the Squid proxy")
	ips, err := getConsolePodIPs(ctx, t.oc)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "should be able to get console pod ips")
	err = waitForProxyTrafficFromTo(ctx, t.oc, proxyNamespace, ips, hostname, logCutOff, 5*time.Minute)
	o.Expect(err).
		NotTo(o.HaveOccurred(), "traffic should go through proxy")
}
