package tls

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// tlsTarget describes a namespace/service that must honor the cluster APIServer
// TLS profile.  Each target gets its own Ginkgo It block so failures are
// reported per-namespace, following the same pattern as the ROFS tests.
type tlsTarget struct {
	// namespace is the OpenShift namespace that contains the operator workload.
	namespace string
	// deploymentName is the name of the Deployment to inspect for TLS env vars.
	// If empty, the env-var check is skipped and only wire-level TLS is tested.
	deploymentName string
	// tlsMinVersionEnvVar is the environment variable name that carries the
	// minimum TLS version (e.g. "REGISTRY_HTTP_TLS_MINVERSION").
	// If empty, the env-var check is skipped.
	tlsMinVersionEnvVar string
	// cipherSuitesEnvVar is the environment variable name that carries the
	// comma-separated list of cipher suites (e.g. "OPENSHIFT_REGISTRY_HTTP_TLS_CIPHERSUITES").
	// If empty, the cipher suite env-var check is skipped.
	cipherSuitesEnvVar string
	// serviceName is the Kubernetes Service name used for wire-level TLS
	// testing via oc port-forward.  If empty, the wire-level test is skipped.
	serviceName string
	// servicePort is the port the TLS service listens on.
	servicePort string
	// operatorConfigGVR is the GroupVersionResource of the operator config
	// resource that contains ObservedConfig (e.g. imageregistries).
	// If zero, the ObservedConfig check is skipped.
	operatorConfigGVR schema.GroupVersionResource
	// operatorConfigName is the name of the operator config resource (e.g. "cluster").
	operatorConfigName string
	// clusterOperatorName is the ClusterOperator name to wait for during
	// stabilization (e.g. "image-registry", "openshift-controller-manager").
	clusterOperatorName string
}

// targets is the list of OpenShift namespaces and services that should
// propagate the cluster APIServer TLS profile.  Add new entries here to
// extend coverage to additional namespaces — each entry generates its own
// test case automatically.
var targets = []tlsTarget{
	{
		namespace:           "openshift-image-registry",
		deploymentName:      "image-registry",
		tlsMinVersionEnvVar: "REGISTRY_HTTP_TLS_MINVERSION",
		cipherSuitesEnvVar:  "OPENSHIFT_REGISTRY_HTTP_TLS_CIPHERSUITES",
		serviceName:         "image-registry",
		servicePort:         "5000",
		operatorConfigGVR: schema.GroupVersionResource{
			Group:    "imageregistry.operator.openshift.io",
			Version:  "v1",
			Resource: "configs",
		},
		operatorConfigName:  "cluster",
		clusterOperatorName: "image-registry",
	},
	// openshift-controller-manager propagates TLS config via ConfigMap
	// (ObservedConfig → config.yaml), NOT via env vars. So we skip the
	// env-var check but still verify ObservedConfig and wire-level TLS.
	{
		namespace:      "openshift-controller-manager",
		deploymentName: "controller-manager",
		// No TLS env vars — controller-manager reads TLS from its config file.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "controller-manager",
		servicePort:         "443",
		operatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "openshiftcontrollermanagers",
		},
		operatorConfigName:  "cluster",
		clusterOperatorName: "openshift-controller-manager",
	},
	// Add more namespaces/services as they adopt the TLS config sync pattern.
}

var _ = g.Describe("[sig-api-machinery][Feature:TLSObservedConfig][Suite:openshift/tls-observed-config][Serial]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-observed-config")
	ctx := context.Background()

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		if isMicroShift || isHyperShift {
			g.Skip("TLS observed-config tests are not applicable to MicroShift or HyperShift clusters")
		}
	})

	// ── Per-namespace ObservedConfig verification ───────────────────────
	// For each target with an operator config resource, verify that the
	// ObservedConfig contains a properly populated servingInfo with
	// minTLSVersion and cipherSuites matching the cluster APIServer profile.
	for _, target := range targets {
		target := target // capture range variable
		if target.operatorConfigGVR.Resource == "" || target.operatorConfigName == "" {
			continue
		}

		g.It(fmt.Sprintf("should populate ObservedConfig with TLS settings - %s [Serial]", target.namespace), func() {
			testObservedConfig(oc, ctx, target)
		})
	}

	// ── Per-namespace TLS env-var verification ──────────────────────────
	// For each target with a deployment and TLS env var, verify that the
	// deployment's containers carry the correct TLS minimum version
	// (and cipher suites if applicable) matching the cluster APIServer profile.
	for _, target := range targets {
		target := target // capture range variable
		if target.deploymentName == "" || target.tlsMinVersionEnvVar == "" {
			continue
		}

		g.It(fmt.Sprintf("should propagate TLS config to deployment env vars - %s [Serial]", target.namespace), func() {
			testDeploymentTLSEnvVars(oc, ctx, target)
		})
	}

	// ── Per-namespace wire-level TLS verification ───────────────────────
	// For each target with a service endpoint, verify that the service
	// actually enforces the TLS version from the cluster profile via
	// oc port-forward.
	for _, target := range targets {
		target := target
		if target.serviceName == "" || target.servicePort == "" {
			continue
		}

		g.It(fmt.Sprintf("should enforce TLS version at the wire level - %s [Serial]", target.namespace), func() {
			testWireLevelTLS(oc, ctx, target)
		})
	}

	// ── Config-change test: switch to Modern, verify, restore ────────
	// This test modifies the cluster APIServer TLS profile, waits for the
	// KAS rollout to complete (via WaitForOperatorToRollout), then verifies
	// that every target service enforces TLS 1.3.  It restores the original
	// profile in DeferCleanup.
	g.It("should enforce Modern TLS profile after cluster-wide config change [Serial] [Slow] [Disruptive]", func() {

		// 1. Read the current APIServer config so we can restore it later.
		g.By("reading current APIServer TLS profile")
		originalConfig, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		originalProfile := originalConfig.Spec.TLSSecurityProfile
		profileDesc := "nil (Intermediate default)"
		if originalProfile != nil {
			profileDesc = string(originalProfile.Type)
		}
		e2e.Logf("Current TLS profile: %s", profileDesc)

		if originalProfile != nil && originalProfile.Type == configv1.TLSProfileModernType {
			g.Skip("Cluster is already using Modern TLS profile; config-change test is not applicable")
		}

		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 30*time.Minute)
		defer configChangeCancel()

		// 2. Set up DeferCleanup to restore the original profile no matter what.
		g.DeferCleanup(func(cleanupCtx context.Context) {
			e2e.Logf("DeferCleanup: restoring original TLS profile: %s", profileDesc)

			restoreRollout := exutil.WaitForOperatorToRollout(cleanupCtx, oc.AdminConfigClient(), "kube-apiserver")
			<-restoreRollout.StableBeforeStarting()

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				current, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(cleanupCtx, "cluster", metav1.GetOptions{})
				if err != nil {
					return err
				}
				current.Spec.TLSSecurityProfile = originalProfile
				_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(cleanupCtx, current, metav1.UpdateOptions{})
				return err
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore original TLS profile")

			e2e.Logf("DeferCleanup: waiting for KAS rollout to complete after restoring profile")
			<-restoreRollout.Done()
			o.Expect(restoreRollout.Err()).NotTo(o.HaveOccurred(),
				"kube-apiserver did not stabilize after restoring TLS profile")

			// Wait for each target's ClusterOperator to stabilize after restore.
			for _, co := range targetClusterOperators() {
				e2e.Logf("DeferCleanup: waiting for ClusterOperator %s to stabilize", co)
				waitForClusterOperatorStable(oc, cleanupCtx, co)
			}
			e2e.Logf("DeferCleanup: original TLS profile restored and cluster is stable")
		})

		// 3. Start watching KAS rollout *before* making the change.
		g.By("starting KAS rollout watcher before changing TLS profile")
		kasRollout := exutil.WaitForOperatorToRollout(configChangeCtx, oc.AdminConfigClient(), "kube-apiserver")

		// Wait until KAS is confirmed stable before we make the change.
		e2e.Logf("Waiting for KAS to be stable before applying config change")
		<-kasRollout.StableBeforeStarting()
		e2e.Logf("KAS is stable; proceeding to apply Modern TLS profile")

		// 4. Update TLS profile to Modern.
		g.By("setting APIServer TLS profile to Modern")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(configChangeCtx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			}
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(configChangeCtx, apiServer, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update APIServer TLS profile to Modern")
		e2e.Logf("APIServer TLS profile updated to Modern")

		// 5. Wait for KAS rollout to complete.
		g.By("waiting for kube-apiserver rollout to complete after TLS profile change")
		e2e.Logf("Waiting for KAS to start progressing...")
		<-kasRollout.Started()
		e2e.Logf("KAS rollout started, waiting for it to finish...")
		<-kasRollout.Done()
		o.Expect(kasRollout.Err()).NotTo(o.HaveOccurred(),
			"kube-apiserver did not stabilize after TLS profile change to Modern")
		e2e.Logf("KAS rollout completed successfully")

		// 6. Wait for each target operator to stabilize after the KAS rollout.
		g.By("waiting for target operators to stabilize after KAS rollout")
		for _, co := range targetClusterOperators() {
			e2e.Logf("Waiting for ClusterOperator %s to stabilize after KAS rollout", co)
			waitForClusterOperatorStable(oc, configChangeCtx, co)
		}

		// 7. Wait for target deployments to fully roll out.
		for _, t := range targets {
			if t.deploymentName == "" {
				continue
			}
			g.By(fmt.Sprintf("waiting for deployment %s/%s to be fully rolled out",
				t.namespace, t.deploymentName))
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(
				configChangeCtx, t.deploymentName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = e2edeployment.WaitForDeploymentComplete(oc.AdminKubeClient(), deployment)
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("deployment %s/%s did not complete rollout after TLS change",
					t.namespace, t.deploymentName))
			e2e.Logf("Deployment %s/%s is fully rolled out", t.namespace, t.deploymentName)
		}

		// 8. Verify env vars reflect Modern profile (VersionTLS13).
		for _, t := range targets {
			if t.deploymentName == "" || t.tlsMinVersionEnvVar == "" {
				continue
			}
			g.By(fmt.Sprintf("verifying %s in %s/%s reflects Modern profile",
				t.tlsMinVersionEnvVar, t.namespace, t.deploymentName))
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(
				configChangeCtx, t.deploymentName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(deployment.Spec.Template.Spec.Containers).NotTo(o.BeEmpty())

			envMap := envToMap(deployment.Spec.Template.Spec.Containers[0].Env)
			o.Expect(envMap).To(o.HaveKey(t.tlsMinVersionEnvVar))
			o.Expect(envMap[t.tlsMinVersionEnvVar]).To(o.Equal("VersionTLS13"),
				fmt.Sprintf("expected %s=VersionTLS13 in %s/%s after Modern profile, got %s",
					t.tlsMinVersionEnvVar, t.namespace, t.deploymentName,
					envMap[t.tlsMinVersionEnvVar]))
			e2e.Logf("PASS: %s=VersionTLS13 in %s/%s", t.tlsMinVersionEnvVar, t.namespace, t.deploymentName)

			// Verify cipher suites env var is also updated for Modern profile.
			if t.cipherSuitesEnvVar != "" {
				// Modern profile uses TLS 1.3 where cipher suites are fixed by the
				// spec and not configurable. The env var should still be present with
				// the profile's cipher suite list.
				o.Expect(envMap).To(o.HaveKey(t.cipherSuitesEnvVar),
					fmt.Sprintf("expected %s to be set in %s/%s after Modern profile",
						t.cipherSuitesEnvVar, t.namespace, t.deploymentName))
				e2e.Logf("PASS: %s is set in %s/%s after Modern profile (value length=%d)",
					t.cipherSuitesEnvVar, t.namespace, t.deploymentName, len(envMap[t.cipherSuitesEnvVar]))
			}
		}

		// 9. Wire-level: verify TLS 1.3 is accepted and TLS 1.2 is rejected.
		tlsShouldWork := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
		tlsShouldNotWork := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}

		for _, t := range targets {
			if t.serviceName == "" || t.servicePort == "" {
				continue
			}
			g.By(fmt.Sprintf("wire-level TLS check: svc/%s in %s (expecting Modern = TLS 1.3 only)",
				t.serviceName, t.namespace))
			err = forwardPortAndExecute(t.serviceName, t.namespace, t.servicePort,
				func(localPort int) error {
					return checkTLSConnection(localPort, tlsShouldWork, tlsShouldNotWork, t)
				},
			)
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("wire-level TLS check failed for svc/%s in %s after switching to Modern",
					t.serviceName, t.namespace))
			e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s (Modern)", t.serviceName, t.namespace)
		}

		e2e.Logf("PASS: all targets verified with Modern TLS profile")
	})
})

// ─── Test implementations ──────────────────────────────────────────────────

// testObservedConfig verifies that the operator's ObservedConfig contains
// a properly populated servingInfo with minTLSVersion and cipherSuites.
// This validates that the config observer controller (from library-go) is
// correctly watching the APIServer resource and writing the TLS config
// into the operator's ObservedConfig.
func testObservedConfig(oc *exutil.CLI, ctx context.Context, t tlsTarget) {
	g.By(fmt.Sprintf("getting operator config %s/%s via dynamic client",
		t.operatorConfigGVR.Resource, t.operatorConfigName))

	dynClient := oc.AdminDynamicClient()
	resource, err := dynClient.Resource(t.operatorConfigGVR).Get(ctx, t.operatorConfigName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get operator config %s/%s",
			t.operatorConfigGVR.Resource, t.operatorConfigName))

	// Extract spec.observedConfig from the unstructured resource.
	observedConfigRaw, found, err := unstructured.NestedMap(resource.Object, "spec", "observedConfig")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to extract spec.observedConfig")
	o.Expect(found).To(o.BeTrue(), "expected spec.observedConfig to exist")
	o.Expect(observedConfigRaw).NotTo(o.BeEmpty(), "expected spec.observedConfig to be non-empty")

	// Log the raw ObservedConfig for debugging (avoid logging raw JSON of full config).
	observedJSON, _ := json.MarshalIndent(observedConfigRaw, "", "  ")
	e2e.Logf("ObservedConfig:\n%s", string(observedJSON))

	// Verify servingInfo exists.
	g.By("verifying servingInfo in ObservedConfig")
	_, found, err = unstructured.NestedMap(observedConfigRaw, "servingInfo")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get servingInfo from observedConfig")
	o.Expect(found).To(o.BeTrue(), "expected servingInfo in ObservedConfig")

	// Verify minTLSVersion is populated.
	g.By("verifying servingInfo.minTLSVersion in ObservedConfig")
	minTLSVersion, found, err := unstructured.NestedString(observedConfigRaw, "servingInfo", "minTLSVersion")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get servingInfo.minTLSVersion")
	o.Expect(found).To(o.BeTrue(), "expected minTLSVersion in servingInfo")
	o.Expect(minTLSVersion).NotTo(o.BeEmpty(), "expected minTLSVersion to be non-empty")
	e2e.Logf("ObservedConfig servingInfo.minTLSVersion: %s", minTLSVersion)

	// Verify cipherSuites is populated.
	g.By("verifying servingInfo.cipherSuites in ObservedConfig")
	cipherSuites, found, err := unstructured.NestedStringSlice(observedConfigRaw, "servingInfo", "cipherSuites")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get servingInfo.cipherSuites")
	o.Expect(found).To(o.BeTrue(), "expected cipherSuites in servingInfo")
	o.Expect(cipherSuites).NotTo(o.BeEmpty(), "expected cipherSuites to be non-empty")
	e2e.Logf("ObservedConfig servingInfo.cipherSuites: %d suites", len(cipherSuites))

	// Cross-check against the cluster APIServer profile.
	g.By("cross-checking ObservedConfig with cluster APIServer TLS profile")
	expectedMinVersion := getExpectedMinTLSVersion(oc, ctx)
	o.Expect(minTLSVersion).To(o.Equal(expectedMinVersion),
		fmt.Sprintf("ObservedConfig minTLSVersion=%s does not match cluster profile=%s",
			minTLSVersion, expectedMinVersion))
	e2e.Logf("PASS: ObservedConfig for %s/%s matches cluster APIServer TLS profile",
		t.operatorConfigGVR.Resource, t.operatorConfigName)
}

// testDeploymentTLSEnvVars verifies that the deployment in the given namespace
// has TLS environment variables that match the expected TLS profile.
func testDeploymentTLSEnvVars(oc *exutil.CLI, ctx context.Context, t tlsTarget) {
	g.By("getting cluster APIServer TLS profile")
	expectedMinVersion := getExpectedMinTLSVersion(oc, ctx)
	e2e.Logf("Expected minTLSVersion from cluster profile: %s", expectedMinVersion)

	g.By(fmt.Sprintf("verifying namespace %s exists", t.namespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, t.namespace, metav1.GetOptions{})
	if err != nil {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", t.namespace))
	}

	g.By(fmt.Sprintf("getting deployment %s/%s", t.namespace, t.deploymentName))
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(
		ctx, t.deploymentName, metav1.GetOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get deployment %s/%s", t.namespace, t.deploymentName))
	o.Expect(deployment.Spec.Template.Spec.Containers).NotTo(o.BeEmpty(),
		fmt.Sprintf("deployment %s/%s has no containers", t.namespace, t.deploymentName))

	e2e.Logf("Deployment %s/%s: generation=%d, observedGeneration=%d, replicas=%d/%d",
		t.namespace, t.deploymentName,
		deployment.Generation, deployment.Status.ObservedGeneration,
		deployment.Status.ReadyReplicas, deployment.Status.Replicas)

	g.By(fmt.Sprintf("verifying %s env var in deployment containers", t.tlsMinVersionEnvVar))
	envMap := envToMap(deployment.Spec.Template.Spec.Containers[0].Env)
	logEnvVars(envMap, t.tlsMinVersionEnvVar)

	o.Expect(envMap).To(o.HaveKey(t.tlsMinVersionEnvVar),
		fmt.Sprintf("expected %s to be set in deployment %s/%s",
			t.tlsMinVersionEnvVar, t.namespace, t.deploymentName))
	o.Expect(envMap[t.tlsMinVersionEnvVar]).To(o.Equal(expectedMinVersion),
		fmt.Sprintf("expected %s=%s in deployment %s/%s, got %s",
			t.tlsMinVersionEnvVar, expectedMinVersion, t.namespace, t.deploymentName,
			envMap[t.tlsMinVersionEnvVar]))
	e2e.Logf("PASS: %s=%s matches cluster TLS profile in %s/%s",
		t.tlsMinVersionEnvVar, expectedMinVersion, t.namespace, t.deploymentName)

	// Verify cipher suites env var if configured for this target.
	if t.cipherSuitesEnvVar != "" {
		g.By(fmt.Sprintf("verifying %s env var in deployment containers", t.cipherSuitesEnvVar))
		o.Expect(envMap).To(o.HaveKey(t.cipherSuitesEnvVar),
			fmt.Sprintf("expected %s to be set in deployment %s/%s",
				t.cipherSuitesEnvVar, t.namespace, t.deploymentName))
		o.Expect(envMap[t.cipherSuitesEnvVar]).NotTo(o.BeEmpty(),
			fmt.Sprintf("expected %s to have a value in deployment %s/%s",
				t.cipherSuitesEnvVar, t.namespace, t.deploymentName))
		e2e.Logf("PASS: %s is set in %s/%s (value length=%d)",
			t.cipherSuitesEnvVar, t.namespace, t.deploymentName, len(envMap[t.cipherSuitesEnvVar]))
	}
}

// testWireLevelTLS verifies that the service endpoint in the given namespace
// enforces the TLS version from the cluster APIServer profile using
// oc port-forward for connectivity.
func testWireLevelTLS(oc *exutil.CLI, ctx context.Context, t tlsTarget) {
	g.By("getting cluster APIServer TLS profile")
	config, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	var tlsShouldWork, tlsShouldNotWork *tls.Config
	profileType := "Intermediate (default)"

	switch {
	case config.Spec.TLSSecurityProfile == nil,
		config.Spec.TLSSecurityProfile.Type == configv1.TLSProfileIntermediateType:
		tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
		tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}
	case config.Spec.TLSSecurityProfile.Type == configv1.TLSProfileModernType:
		profileType = "Modern"
		tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
		tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
	default:
		g.Skip("Only Intermediate or Modern TLS profiles are tested for wire-level verification")
	}
	e2e.Logf("Cluster TLS profile: %s", profileType)

	g.By("verifying namespace exists")
	_, err = oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, t.namespace, metav1.GetOptions{})
	if err != nil {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", t.namespace))
	}

	if t.deploymentName != "" {
		g.By(fmt.Sprintf("waiting for deployment %s/%s to be fully rolled out", t.namespace, t.deploymentName))
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(ctx, t.deploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("failed to get deployment %s/%s", t.namespace, t.deploymentName))
		err = e2edeployment.WaitForDeploymentComplete(oc.AdminKubeClient(), deployment)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("deployment %s/%s did not complete rollout", t.namespace, t.deploymentName))
	}

	g.By(fmt.Sprintf("verifying TLS behavior via port-forward to svc/%s in %s on port %s",
		t.serviceName, t.namespace, t.servicePort))
	err = forwardPortAndExecute(t.serviceName, t.namespace, t.servicePort,
		func(localPort int) error {
			return checkTLSConnection(localPort, tlsShouldWork, tlsShouldNotWork, t)
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("wire-level TLS test failed for svc/%s in %s:%s (profile=%s)",
			t.serviceName, t.namespace, t.servicePort, profileType))
	e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s:%s (profile=%s)",
		t.serviceName, t.namespace, t.servicePort, profileType)
}

// ─── Helper functions ──────────────────────────────────────────────────────

// targetClusterOperators returns the deduplicated list of ClusterOperator
// names from the global targets list.  Used when the config-change test needs
// to wait for all target operators to stabilize.
func targetClusterOperators() []string {
	seen := map[string]bool{}
	var result []string
	for _, t := range targets {
		if t.clusterOperatorName == "" || seen[t.clusterOperatorName] {
			continue
		}
		seen[t.clusterOperatorName] = true
		result = append(result, t.clusterOperatorName)
	}
	return result
}

// getExpectedMinTLSVersion returns the expected minTLSVersion string
// (e.g. "VersionTLS12", "VersionTLS13") based on the cluster APIServer profile.
func getExpectedMinTLSVersion(oc *exutil.CLI, ctx context.Context) string {
	config, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	profileType := configv1.TLSProfileIntermediateType
	if config.Spec.TLSSecurityProfile != nil {
		profileType = config.Spec.TLSSecurityProfile.Type
	}

	profile, ok := configv1.TLSProfiles[profileType]
	if !ok {
		e2e.Failf("Unknown TLS profile type: %s", profileType)
	}

	minVersion := string(profile.MinTLSVersion)
	e2e.Logf("Cluster APIServer TLS profile: type=%s, minTLSVersion=%s", profileType, minVersion)
	return minVersion
}

// forwardPortAndExecute sets up oc port-forward to a service and executes
// the given test function with the local port.  Retries up to 3 times.
func forwardPortAndExecute(serviceName, namespace, remotePort string, toExecute func(localPort int) error) error {
	var err error
	for i := 0; i < 3; i++ {
		if err = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			localPort := rand.Intn(65534-1025) + 1025
			args := []string{
				"port-forward",
				fmt.Sprintf("svc/%s", serviceName),
				fmt.Sprintf("%d:%s", localPort, remotePort),
				"-n", namespace,
			}

			cmd := exec.CommandContext(ctx, "oc", args...)
			stdout, stderr, err := e2e.StartCmdAndStreamOutput(cmd)
			if err != nil {
				return fmt.Errorf("failed to start port-forward: %v", err)
			}
			defer stdout.Close()
			defer stderr.Close()
			defer e2e.TryKill(cmd)

			// Give port-forward a moment to establish.
			e2e.Logf("oc port-forward output: %s", readPartialFrom(stdout, 1024))
			return toExecute(localPort)
		}(); err == nil {
			return nil
		}
		e2e.Logf("port-forward attempt %d/3 failed: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	return err
}

// readPartialFrom reads up to maxBytes from a reader.
func readPartialFrom(r io.Reader, maxBytes int) string {
	buf := make([]byte, maxBytes)
	n, err := r.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Sprintf("error reading: %v", err)
	}
	return string(buf[:n])
}

// checkTLSConnection verifies that a local-forwarded port accepts the expected
// TLS version and rejects the one that should not work.
func checkTLSConnection(localPort int, shouldWork, shouldNotWork *tls.Config, t tlsTarget) error {
	host := fmt.Sprintf("localhost:%d", localPort)
	e2e.Logf("Testing TLS behavior against %s (forwarded from svc/%s in %s)", host, t.serviceName, t.namespace)

	// Test that the expected TLS version works.
	conn, err := tls.Dial("tcp", host, shouldWork)
	if err != nil {
		return fmt.Errorf("svc/%s in %s: TLS connection that should work failed: %w", t.serviceName, t.namespace, err)
	}
	negotiated := conn.ConnectionState().Version
	conn.Close()
	e2e.Logf("svc/%s in %s: TLS connection succeeded, negotiated version: 0x%04x", t.serviceName, t.namespace, negotiated)

	// Test that the version that should not work is rejected.
	conn, err = tls.Dial("tcp", host, shouldNotWork)
	if err == nil {
		conn.Close()
		return fmt.Errorf("svc/%s in %s: TLS connection that should NOT work unexpectedly succeeded", t.serviceName, t.namespace)
	}

	// Verify we got a TLS-related error, not a network error.
	errStr := err.Error()
	if !strings.Contains(errStr, "protocol version") &&
		!strings.Contains(errStr, "no supported versions") &&
		!strings.Contains(errStr, "handshake failure") &&
		!strings.Contains(errStr, "alert") {
		return fmt.Errorf("svc/%s in %s: expected TLS version mismatch error, got: %w", t.serviceName, t.namespace, err)
	}
	e2e.Logf("svc/%s in %s: TLS connection correctly rejected (error: %v)", t.serviceName, t.namespace, err)
	return nil
}

// envToMap converts a slice of container environment variables to a map.
func envToMap(envVars []corev1.EnvVar) map[string]string {
	m := make(map[string]string, len(envVars))
	for _, e := range envVars {
		m[e.Name] = e.Value
	}
	return m
}

// logEnvVars logs the value of the specified env var and any other TLS-related
// env vars found in the map.
func logEnvVars(envMap map[string]string, primaryKey string) {
	tlsPatterns := []string{"TLS", "CIPHER", "SSL"}
	e2e.Logf("TLS-related environment variables:")
	for key, val := range envMap {
		for _, pattern := range tlsPatterns {
			if strings.Contains(strings.ToUpper(key), pattern) {
				display := val
				if len(display) > 120 {
					display = display[:120] + "..."
				}
				e2e.Logf("  %s=%s", key, display)
				break
			}
		}
	}
	if _, ok := envMap[primaryKey]; !ok {
		e2e.Logf("  WARNING: primary TLS env var %s not found", primaryKey)
	}
}

// waitForClusterOperatorStable waits until the named ClusterOperator reaches
// Available=True, Progressing=False, Degraded=False.
func waitForClusterOperatorStable(oc *exutil.CLI, ctx context.Context, name string) {
	e2e.Logf("Waiting for ClusterOperator %q to become stable", name)
	start := time.Now()

	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 15*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			co, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll[%s]: error fetching ClusterOperator %s: %v",
					time.Since(start).Round(time.Second), name, err)
				return false, nil
			}

			isAvailable := false
			isProgressing := true
			isDegraded := false

			for _, c := range co.Status.Conditions {
				switch c.Type {
				case configv1.OperatorAvailable:
					isAvailable = c.Status == configv1.ConditionTrue
				case configv1.OperatorProgressing:
					isProgressing = c.Status == configv1.ConditionTrue
				case configv1.OperatorDegraded:
					isDegraded = c.Status == configv1.ConditionTrue
				}
			}

			if isDegraded {
				e2e.Logf("  poll[%s]: WARNING ClusterOperator %s is degraded", time.Since(start).Round(time.Second), name)
				for _, c := range co.Status.Conditions {
					e2e.Logf("    %s=%s reason=%s message=%q", c.Type, c.Status, c.Reason, c.Message)
				}
				return false, nil
			}

			if isAvailable && !isProgressing {
				e2e.Logf("  poll[%s]: ClusterOperator %s is stable", time.Since(start).Round(time.Second), name)
				return true, nil
			}

			e2e.Logf("  poll[%s]: ClusterOperator %s not stable (Available=%v, Progressing=%v)",
				time.Since(start).Round(time.Second), name, isAvailable, isProgressing)
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("ClusterOperator %s did not reach stable state after %s",
			name, time.Since(start).Round(time.Second)))
}
