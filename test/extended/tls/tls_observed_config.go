package tls

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

const (
	// operatorRolloutTimeout is the maximum time to wait for an operator
	// workload (Deployment or static pod) to complete rollout after a TLS
	// profile change. KAS (static pod) rollout typically takes 15-20 minutes;
	// Deployment-based operators are usually faster. 25 minutes covers both.
	operatorRolloutTimeout = 25 * time.Minute

	// injectTLSAnnotation is the annotation key used by CVO to inject TLS
	// security profile configuration into operator ConfigMaps.
	injectTLSAnnotation = "config.openshift.io/inject-tls"
)

// ─── Narrow target types ───────────────────────────────────────────────────
// Each type carries only the fields its test function actually reads,
// making it immediately clear what data a test depends on.

// observedConfigTarget identifies an operator whose spec.observedConfig
// must contain servingInfo with minTLSVersion and cipherSuites.
type observedConfigTarget struct {
	namespace                  string
	operatorConfigGVR          schema.GroupVersionResource
	operatorConfigName         string
	servingInfoPath            []string
	managementClusterComponent bool
}

// configMapTarget identifies a ConfigMap that CVO injects TLS config into.
type configMapTarget struct {
	namespace                  string // workload namespace (used in test names)
	configMapName              string
	configMapNamespace         string // namespace where the ConfigMap lives
	configMapKey               string // data key within the ConfigMap
	managementClusterComponent bool
}

// deploymentEnvVarTarget identifies a Deployment whose containers must
// have TLS-related environment variables matching the cluster profile.
type deploymentEnvVarTarget struct {
	namespace                  string
	deploymentName             string
	tlsMinVersionEnvVar        string
	cipherSuitesEnvVar         string
	managementClusterComponent bool
}

// serviceTarget identifies a Service endpoint that must enforce the
// cluster TLS profile at the wire level.
type serviceTarget struct {
	namespace                  string
	serviceName                string
	servicePort                string
	deploymentName             string // for waiting on rollout before probing
	managementClusterComponent bool
}

// deploymentRolloutTarget identifies a Deployment that must complete
// rollout after a TLS profile change.
type deploymentRolloutTarget struct {
	namespace                  string
	deploymentName             string
	managementClusterComponent bool
}

// tlsConfig represents the effective TLS configuration at a point in time.
// This is used to capture the current state and compare before/after profile changes.
type tlsConfig struct {
	profileType   configv1.TLSProfileType // The profile type (Intermediate, Modern, Custom, etc.)
	minTLSVersion string                  // e.g., "VersionTLS12", "VersionTLS13"
	cipherSuites  []string                // IANA cipher suite names
}

// tlsTestTargets consolidates all TLS test target lists into a single structure.
// This allows passing all targets together and makes it easier to define
// different target sets for different test scenarios.
type tlsTestTargets struct {
	observedConfig    []observedConfigTarget
	configMaps        []configMapTarget
	deploymentEnvVars []deploymentEnvVarTarget
	services          []serviceTarget
}

// ─── Typed target lists ────────────────────────────────────────────────────
// Each list contains exactly the entries relevant to one test category.
// Entries are derived from `targets` but only carry the fields the test uses.

// observedConfigTargets lists operator configs that populate
// spec.observedConfig.servingInfo with TLS settings via library-go.
// The samples operator is NOT included because it uses
// samples.operator.openshift.io/v1 Config (no spec.observedConfig);
// its TLS config is injected through the ConfigMap annotation instead.
var observedConfigTargets = []observedConfigTarget{
	newObservedConfigTarget("openshift-image-registry", gvr("imageregistry.operator.openshift.io", "v1", "configs"), "cluster", []string{"servingInfo"}, false),
	newObservedConfigTarget("openshift-controller-manager", gvr("operator.openshift.io", "v1", "openshiftcontrollermanagers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("openshift-kube-apiserver", gvr("operator.openshift.io", "v1", "kubeapiservers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("openshift-apiserver", gvr("operator.openshift.io", "v1", "openshiftapiservers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("openshift-etcd", gvr("operator.openshift.io", "v1", "etcds"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("openshift-kube-controller-manager", gvr("operator.openshift.io", "v1", "kubecontrollermanagers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("openshift-kube-scheduler", gvr("operator.openshift.io", "v1", "kubeschedulers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("openshift-authentication-operator", gvr("operator.openshift.io", "v1", "authentications"), "cluster", []string{"oauthServer", "servingInfo"}, true),
}

var configMapTargets = []configMapTarget{
	newConfigMapTarget("openshift-image-registry", "image-registry-operator-config", "openshift-image-registry", "config.yaml", false),
	newConfigMapTarget("openshift-controller-manager", "openshift-controller-manager-operator-config", "openshift-controller-manager-operator", "config.yaml", true),
	newConfigMapTarget("openshift-kube-apiserver", "kube-apiserver-operator-config", "openshift-kube-apiserver-operator", "config.yaml", true),
	newConfigMapTarget("openshift-apiserver", "openshift-apiserver-operator-config", "openshift-apiserver-operator", "config.yaml", true),
	newConfigMapTarget("openshift-etcd", "etcd-operator-config", "openshift-etcd-operator", "config.yaml", true),
	newConfigMapTarget("openshift-kube-controller-manager", "kube-controller-manager-operator-config", "openshift-kube-controller-manager-operator", "config.yaml", true),
	newConfigMapTarget("openshift-kube-scheduler", "openshift-kube-scheduler-operator-config", "openshift-kube-scheduler-operator", "config.yaml", true),
	newConfigMapTarget("openshift-cluster-samples-operator", "samples-operator-config", "openshift-cluster-samples-operator", "config.yaml", false),
	newConfigMapTarget("openshift-authentication-operator", "authentication-operator-config", "openshift-authentication-operator", "operator-config.yaml", true),
}

var deploymentEnvVarTargets = []deploymentEnvVarTarget{
	newDeploymentEnvVarTarget("openshift-image-registry", "image-registry", "REGISTRY_HTTP_TLS_MINVERSION", "OPENSHIFT_REGISTRY_HTTP_TLS_CIPHERSUITES", false),
}

var serviceTargets = []serviceTarget{
	newServiceTarget("openshift-image-registry", "image-registry", "5000", "image-registry", false),
	newServiceTarget("openshift-image-registry", "image-registry-operator", "60000", "", true),
	newServiceTarget("openshift-controller-manager", "controller-manager", "443", "controller-manager", true),
	newServiceTarget("openshift-kube-apiserver", "apiserver", "443", "", true),
	newServiceTarget("openshift-kube-apiserver", "apiserver", "17697", "", true),
	newServiceTarget("openshift-apiserver", "api", "443", "apiserver", true),
	newServiceTarget("openshift-apiserver", "check-endpoints", "17698", "", true),
	newServiceTarget("openshift-etcd", "etcd", "2379", "", true),
	newServiceTarget("openshift-kube-controller-manager", "kube-controller-manager", "443", "", true),
	newServiceTarget("openshift-kube-scheduler", "scheduler", "443", "", true),
	newServiceTarget("openshift-cluster-samples-operator", "metrics", "60000", "cluster-samples-operator", false),
	newServiceTarget("openshift-authentication-operator", "metrics", "443", "authentication-operator", true),
	newServiceTarget("openshift-authentication", "oauth-openshift", "443", "oauth-openshift", true),
	newServiceTarget("openshift-oauth-apiserver", "api", "443", "apiserver", true),
}

// clusterOperatorTarget identifies a ClusterOperator whose stability is
// verified after a TLS profile change.
type clusterOperatorTarget struct {
	name                       string
	managementClusterComponent bool
}

var clusterOperatorTargets = []clusterOperatorTarget{
	{name: "image-registry"},
	{name: "openshift-controller-manager", managementClusterComponent: true},
	{name: "kube-apiserver", managementClusterComponent: true},
	{name: "openshift-apiserver", managementClusterComponent: true},
	{name: "etcd", managementClusterComponent: true},
	{name: "kube-controller-manager", managementClusterComponent: true},
	{name: "kube-scheduler", managementClusterComponent: true},
	{name: "openshift-samples"},
	{name: "authentication", managementClusterComponent: true},
}

var deploymentRolloutTargets = []deploymentRolloutTarget{
	{namespace: "openshift-image-registry", deploymentName: "image-registry"},
	{namespace: "openshift-controller-manager", deploymentName: "controller-manager", managementClusterComponent: true},
	{namespace: "openshift-apiserver", deploymentName: "apiserver", managementClusterComponent: true},
	{namespace: "openshift-cluster-version", deploymentName: "cluster-version-operator", managementClusterComponent: true},
	{namespace: "openshift-cluster-samples-operator", deploymentName: "cluster-samples-operator"},
	{namespace: "openshift-authentication-operator", deploymentName: "authentication-operator", managementClusterComponent: true},
	{namespace: "openshift-authentication", deploymentName: "oauth-openshift", managementClusterComponent: true},
	{namespace: "openshift-oauth-apiserver", deploymentName: "apiserver", managementClusterComponent: true},
}

var allTLSTestTargets = tlsTestTargets{
	observedConfig:    observedConfigTargets,
	configMaps:        configMapTargets,
	deploymentEnvVars: deploymentEnvVarTargets,
	services:          serviceTargets,
}

// ─── Guest-side filters for HyperShift ─────────────────────────────────────

func guestSideObservedConfigTargets() []observedConfigTarget {
	var result []observedConfigTarget
	for _, t := range observedConfigTargets {
		if !t.managementClusterComponent {
			result = append(result, t)
		}
	}
	return result
}

func guestSideConfigMapTargets() []configMapTarget {
	var result []configMapTarget
	for _, t := range configMapTargets {
		if !t.managementClusterComponent {
			result = append(result, t)
		}
	}
	return result
}

func guestSideDeploymentEnvVarTargets() []deploymentEnvVarTarget {
	var result []deploymentEnvVarTarget
	for _, t := range deploymentEnvVarTargets {
		if !t.managementClusterComponent {
			result = append(result, t)
		}
	}
	return result
}

func guestSideServiceTargets() []serviceTarget {
	var result []serviceTarget
	for _, t := range serviceTargets {
		if !t.managementClusterComponent {
			result = append(result, t)
		}
	}
	return result
}

func guestSideDeploymentRolloutTargets() []deploymentRolloutTarget {
	var result []deploymentRolloutTarget
	for _, t := range deploymentRolloutTargets {
		if !t.managementClusterComponent {
			result = append(result, t)
		}
	}
	return result
}

func guestSideClusterOperatorTargets() []clusterOperatorTarget {
	var result []clusterOperatorTarget
	for _, t := range clusterOperatorTargets {
		if !t.managementClusterComponent {
			result = append(result, t)
		}
	}
	return result
}

// ── read-only tests ────────────────────────────────────────────
// These tests only read cluster state (ObservedConfig, ConfigMaps,
var _ = g.Describe("[sig-api-machinery][Feature:TLSObservedConfig][Serial][Suite:openshift/tls-observed-config]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-observed-config")
	ctx := context.Background()

	g.It("should verify TLS configuration across all components", func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("TLS observed-config tests are not applicable to MicroShift clusters")
		}

		isHyperShiftCluster, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		apiserverConfig, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		expectedTLSConfig := captureTLSConfiguration(apiserverConfig.Spec.TLSSecurityProfile)

		verifyAllTLSConfiguration(oc, ctx, isHyperShiftCluster, allTLSTestTargets, expectedTLSConfig)
	})
})

// ── Serial disruptive tests ─────────────────────────────────────────────
// These tests modify cluster state (ConfigMap annotations, servingInfo,
// cluster-wide TLS profile) and must run serially.
var _ = g.Describe("[sig-api-machinery][Feature:TLSObservedConfig][Serial][Disruptive][Suite:openshift/tls-observed-config]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-observed-config-serial")
	ctx := context.Background()

	var isHyperShiftCluster bool

	// Pre-compute guest-side target lists so the filter functions are
	// called once rather than on every config-change verification.
	guestObservedCfg := guestSideObservedConfigTargets()
	guestCMs := guestSideConfigMapTargets()
	guestEnvVars := guestSideDeploymentEnvVarTargets()
	guestSvcs := guestSideServiceTargets()
	guestRollouts := guestSideDeploymentRolloutTargets()

	// HyperShift management cluster state, lazily populated by
	// setupHyperShiftManagement. Only config-change tests need this;
	// annotation/servingInfo restoration tests work without it.
	var mgmtOC *exutil.CLI
	var hcpNamespace string
	var hostedClusterName string
	var hostedClusterNS string

	setupHyperShiftManagement := func() {
		if os.Getenv("HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG") == "" || os.Getenv("HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE") == "" {
			g.Skip("HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG and HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE is not set for config-change tests on HyperShift")
		}
		mgmtOC = exutil.NewHypershiftManagementCLI("tls-mgmt")
		var err error
		_, hcpNamespace, err = exutil.GetHypershiftManagementClusterConfigAndNamespace()
		o.Expect(err).NotTo(o.HaveOccurred())
		hostedClusterName, hostedClusterNS = discoverHostedCluster(mgmtOC, hcpNamespace)
		e2e.Logf("HyperShift: HC=%s/%s, HCP NS=%s", hostedClusterNS, hostedClusterName, hcpNamespace)
	}

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("TLS observed-config tests are not applicable to MicroShift clusters")
		}

		isHS, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		isHyperShiftCluster = isHS
	})

	// ── Config-change test: switch to Modern, verify, restore ────────
	// This test modifies the cluster APIServer TLS profile, waits for all
	// ClusterOperators and Deployments to stabilize, then verifies that
	// every target service enforces TLS 1.3. It restores the original
	// profile in DeferCleanup.
	g.It("should enforce Modern TLS profile after cluster-wide config change [Timeout:60m]", func() {
		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 50*time.Minute)
		defer configChangeCancel()

		if isHyperShiftCluster {
			setupHyperShiftManagement()
			// ── HyperShift flow: patch HostedCluster, wait for HCP pods ──
			modernPatch := `{"spec":{"configuration":{"apiServer":{"tlsSecurityProfile":{"modern":{},"type":"Modern"}}}}}`
			resetPatch := `{"spec":{"configuration":{"apiServer":null}}}`

			g.By("reading current HostedCluster TLS profile")
			currentTLS, err := mgmtOC.AsAdmin().Run("get").Args(
				"hostedcluster", hostedClusterName, "-n", hostedClusterNS,
				"-o", `jsonpath={.spec.configuration.apiServer.tlsSecurityProfile.type}`,
			).Output()
			if err != nil || currentTLS == "" {
				currentTLS = "Intermediate (default)"
			}
			e2e.Logf("Current HostedCluster TLS profile: %s", currentTLS)

			if currentTLS == "Modern" {
				g.Skip("HostedCluster is already using Modern TLS profile")
			}

			g.DeferCleanup(func(cleanupCtx context.Context) {
				e2e.Logf("DeferCleanup: restoring HostedCluster TLS profile to default")
				setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, resetPatch)
				waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
				waitForGuestOperatorsAfterTLSChange(oc, cleanupCtx, "restore", guestRollouts)
				e2e.Logf("DeferCleanup: HostedCluster TLS profile restored")
			})

			// Phase 1: Modern
			g.By("patching HostedCluster with Modern TLS profile")
			setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, modernPatch)
			e2e.Logf("HostedCluster TLS profile patched to Modern")

			g.By("waiting for HCP pods and guest operators to stabilize")
			waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
			waitForGuestOperatorsAfterTLSChange(oc, configChangeCtx, "Modern", guestRollouts)

			g.By("verifying guest-side ObservedConfig reflects Modern profile")
			verifyObservedConfigForTargets(oc, configChangeCtx, "VersionTLS13", "Modern", guestObservedCfg)
			g.By("verifying guest-side ConfigMaps reflect Modern profile")
			verifyConfigMapsForTargets(oc, configChangeCtx, "VersionTLS13", "Modern", guestCMs)
			g.By("verifying HCP ConfigMaps reflect Modern profile")
			verifyHCPConfigMaps(mgmtOC, hcpNamespace, "VersionTLS13", "Modern")

			for _, t := range guestEnvVars {
				g.By(fmt.Sprintf("verifying %s in %s/%s reflects Modern profile",
					t.tlsMinVersionEnvVar, t.namespace, t.deploymentName))
				deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(
					configChangeCtx, t.deploymentName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				envMap := findEnvAcrossContainers(deployment.Spec.Template.Spec.Containers, t.tlsMinVersionEnvVar, t.cipherSuitesEnvVar)
				o.Expect(envMap).To(o.HaveKey(t.tlsMinVersionEnvVar))
				o.Expect(envMap[t.tlsMinVersionEnvVar]).To(o.Equal("VersionTLS13"))
				e2e.Logf("PASS: %s=VersionTLS13 in %s/%s", t.tlsMinVersionEnvVar, t.namespace, t.deploymentName)
			}

			tlsShouldWork, tlsShouldNotWork, profileTypeStr, err := getWireLevelTLSConfigs(oc, configChangeCtx)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, t := range guestSvcs {
				g.By(fmt.Sprintf("wire-level TLS check: svc/%s in %s (profile: %s)", t.serviceName, t.namespace, profileTypeStr))
				if t.deploymentName != "" {
					waitForDeploymentRolloutAfterTLSChange(oc, configChangeCtx, t.namespace, t.deploymentName)
				}
				err := testWireLevelTLS(oc, configChangeCtx, t, tlsShouldWork, tlsShouldNotWork)
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s (%s)", t.serviceName, t.namespace, profileTypeStr)
			}
			e2e.Logf("PASS: Modern TLS profile propagation verified on HyperShift (restore handled by DeferCleanup)")
			return
		}

		// ── Standalone OCP flow ─────────────────────────────────────────

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

		// 2. Set up DeferCleanup to restore the original profile no matter what.
		g.DeferCleanup(func(cleanupCtx context.Context) {
			restoreOriginalTLSProfile(oc, cleanupCtx, originalProfile, profileDesc)
		})

		// 3. Update TLS profile to Modern.
		// TODO: Before setAPIServerTLSProfile, verify current effective TLS config (minTLSVersion, cipherSuites)
		// differs from the new profile to ensure the change will actually propagate through the system.
		g.By("setting APIServer TLS profile to Modern")
		modernProfile := &configv1.TLSSecurityProfile{
			Type:   configv1.TLSProfileModernType,
			Modern: &configv1.ModernTLSProfile{},
		}
		setAPIServerTLSProfile(oc, configChangeCtx, modernProfile, "Modern")
		e2e.Logf("APIServer TLS profile updated to Modern")

		// 4. Wait for all operators to stabilize after the config change.
		g.By("waiting for all operators to stabilize after TLS profile change to Modern")
		waitForAllOperatorsAfterTLSChange(oc, configChangeCtx, "Modern")

		// 5. Verify env vars reflect Modern profile (VersionTLS13).
		expectedTLSConfig := captureTLSConfiguration(modernProfile)

		for _, t := range deploymentEnvVarTargets {
			g.By(fmt.Sprintf("verifying deployment env vars %s/%s reflect Modern profile", t.namespace, t.deploymentName))
			err := testDeploymentTLSEnvVars(oc, configChangeCtx, t, expectedTLSConfig)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		// 6. Verify ObservedConfig reflects Modern profile (VersionTLS13).
		g.By("verifying ObservedConfig reflects Modern profile (VersionTLS13)")
		verifyObservedConfigAfterSwitch(oc, configChangeCtx, "VersionTLS13", "Modern")

		// 7. Verify ConfigMaps reflect Modern profile (VersionTLS13).
		g.By("verifying ConfigMaps reflect Modern profile (VersionTLS13)")
		verifyConfigMapsAfterSwitch(oc, configChangeCtx, "VersionTLS13", "Modern")

		// 8. Wire-level: verify TLS version matches the current profile.
		g.By("determining expected TLS version from current APIServer profile")
		tlsShouldWork, tlsShouldNotWork, profileTypeStr, err := getWireLevelTLSConfigs(oc, configChangeCtx)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Wire-level TLS configs determined from profile type: %s", profileTypeStr)

		for _, t := range serviceTargets {
			g.By(fmt.Sprintf("wire-level TLS check: svc/%s in %s (profile: %s)",
				t.serviceName, t.namespace, profileTypeStr))
			err := testWireLevelTLS(oc, configChangeCtx, t, tlsShouldWork, tlsShouldNotWork)
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("wire-level TLS check failed for svc/%s in %s (profile: %s)",
					t.serviceName, t.namespace, profileTypeStr))
			e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s (profile: %s)", t.serviceName, t.namespace, profileTypeStr)
		}

		e2e.Logf("PASS: all targets verified with Modern TLS profile")

		// DeferCleanup (registered above) restores the original Intermediate
		// profile and waits for operators to stabilize, so we don't need an
		// explicit downgrade phase here.
		e2e.Logf("PASS: Modern TLS profile propagation verified (restore handled by DeferCleanup)")
	})

	// Focus on the wiring. The centralized TLS is:
	// - injected into observedConfigs
	// - injected into CMs
	// - injected as ENVs into Deployment specs
	// As a minimal test validate component deployments are done rolling up
	// and validate the min TLS version is properly propagated into each relevant endpoint.
	// The actual TLS compliance is performed via tls-scanner as a separate e2e.
	g.It("should enforce different TLS profile with reconciliation-based waiting [Timeout:60m]", func() {
		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 60*time.Minute)
		defer configChangeCancel()

		// 1. Read current APIServer TLS profile and determine effective TLS configuration
		g.By("reading current APIServer TLS profile")
		originalAPIServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(configChangeCtx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		originalProfile := originalAPIServer.Spec.TLSSecurityProfile
		currentTLSConfig := captureTLSConfiguration(originalProfile)
		e2e.Logf("Current TLS profile: type=%s, minTLSVersion=%s, ciphers=%v",
			currentTLSConfig.profileType, currentTLSConfig.minTLSVersion, currentTLSConfig.cipherSuites)

		// 2. Generate a different TLS profile (different version and different ciphers)
		g.By("generating target TLS profile different from current")
		targetProfile, targetTLSConfig := generateDifferentTLSProfile(currentTLSConfig)
		e2e.Logf("Target TLS profile: type=%s, minTLSVersion=%s, ciphers=%v",
			targetTLSConfig.profileType, targetTLSConfig.minTLSVersion, targetTLSConfig.cipherSuites)

		// 3. Verify current effective config matches current profile
		g.By("verifying current effective TLS config matches current profile")
		verifyAllTLSConfiguration(oc, configChangeCtx, false, allTLSTestTargets, currentTLSConfig)
		e2e.Logf("PASS: All targets verified - match current APIServer TLS profile")

		// 4. Set new TLS profile
		g.By("setting APIServer TLS profile to target configuration")
		setAPIServerTLSProfile(oc, configChangeCtx, targetProfile, "Custom")
		e2e.Logf("APIServer TLS profile updated to Custom (minTLSVersion=%s, ciphers=%v)",
			targetTLSConfig.minTLSVersion, targetTLSConfig.cipherSuites)

		// 5. Wait for reconciliation
		g.By("waiting for all targets to reconcile to new TLS configuration")
		err = waitForTLSReconciliation(oc, configChangeCtx, false, allTLSTestTargets, targetTLSConfig)
		o.Expect(err).NotTo(o.HaveOccurred(), "TLS reconciliation failed")
		e2e.Logf("PASS: All targets reconciled to new TLS configuration")

		e2e.Logf("=== TLS reconciliation complete (config + wire-level validation) ===")
	})

	// ── Custom TLS profile test ────────────────────────────────────────────
	// This test sets a Custom TLS profile with specific minTLSVersion and
	// cipherSuites, verifies propagation to all operators, then restores.
	g.It("should enforce Custom TLS profile after cluster-wide config change [Timeout:60m]", func() {
		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 60*time.Minute)
		defer configChangeCancel()

		customCiphers := []string{
			"ECDHE-RSA-AES128-GCM-SHA256",
			"ECDHE-RSA-AES256-GCM-SHA384",
			"ECDHE-ECDSA-AES128-GCM-SHA256",
			"ECDHE-ECDSA-AES256-GCM-SHA384",
		}

		if isHyperShiftCluster {
			setupHyperShiftManagement()
			// ── HyperShift flow: patch HostedCluster with Custom TLS ──
			customPatch := fmt.Sprintf(
				`{"spec":{"configuration":{"apiServer":{"tlsSecurityProfile":{"type":"Custom","custom":{"ciphers":["%s"],"minTLSVersion":"VersionTLS12"}}}}}}`,
				strings.Join(customCiphers, `","`),
			)
			resetPatch := `{"spec":{"configuration":{"apiServer":null}}}`

			g.DeferCleanup(func(cleanupCtx context.Context) {
				e2e.Logf("DeferCleanup: restoring HostedCluster TLS profile to default")
				setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, resetPatch)
				waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
				waitForGuestOperatorsAfterTLSChange(oc, cleanupCtx, "restore", guestRollouts)
				e2e.Logf("DeferCleanup: HostedCluster TLS profile restored")
			})

			g.By("patching HostedCluster with Custom TLS profile")
			setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, customPatch)
			e2e.Logf("HostedCluster TLS profile patched to Custom (minTLSVersion=TLS12, ciphers=%d)", len(customCiphers))

			g.By("waiting for HCP pods and guest operators to stabilize")
			waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
			waitForGuestOperatorsAfterTLSChange(oc, configChangeCtx, "Custom", guestRollouts)

			g.By("verifying guest-side ObservedConfig reflects Custom profile")
			verifyObservedConfigForTargets(oc, configChangeCtx, "VersionTLS12", "Custom", guestObservedCfg)
			g.By("verifying guest-side ConfigMaps reflect Custom profile")
			verifyConfigMapsForTargets(oc, configChangeCtx, "VersionTLS12", "Custom", guestCMs)
			g.By("verifying HCP ConfigMaps reflect Custom profile")
			verifyHCPConfigMaps(mgmtOC, hcpNamespace, "VersionTLS12", "Custom")

			g.By("verifying wire-level TLS for Custom profile on guest targets")
			tlsShouldWork, tlsShouldNotWork, profileTypeStr, err := getWireLevelTLSConfigs(oc, configChangeCtx)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, t := range guestSvcs {
				if t.deploymentName != "" {
					waitForDeploymentRolloutAfterTLSChange(oc, configChangeCtx, t.namespace, t.deploymentName)
				}
				err := testWireLevelTLS(oc, configChangeCtx, t, tlsShouldWork, tlsShouldNotWork)
				o.Expect(err).NotTo(o.HaveOccurred(),
					fmt.Sprintf("wire-level TLS check failed for svc/%s in %s:%s (profile: %s)", t.serviceName, t.namespace, t.servicePort, profileTypeStr))
				e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s:%s (profile: %s)", t.serviceName, t.namespace, t.servicePort, profileTypeStr)
			}

			e2e.Logf("PASS: Custom TLS profile verified successfully on HyperShift")
			return
		}

		// ── Standalone OCP flow ─────────────────────────────────────────

		// 1. Read the current APIServer config so we can restore it later.
		g.By("reading current APIServer TLS profile")
		originalAPIServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get APIServer cluster config")

		originalProfile := originalAPIServer.Spec.TLSSecurityProfile
		profileDesc := "nil (Intermediate default)"
		if originalProfile != nil {
			profileDesc = fmt.Sprintf("%v", originalProfile.Type)
		}
		e2e.Logf("Current TLS profile: %s", profileDesc)

		// 2. DeferCleanup to restore the original TLS profile.
		g.DeferCleanup(func(cleanupCtx context.Context) {
			restoreOriginalTLSProfile(oc, cleanupCtx, originalProfile, profileDesc)
		})

		// 3. Set the APIServer TLS profile to Custom.
		// TODO: Before setAPIServerTLSProfile, verify current effective TLS config (minTLSVersion, cipherSuites)
		// differs from the new profile to ensure the change will actually propagate through the system.
		g.By("setting APIServer TLS profile to Custom (TLS 1.2 with specific ciphers)")
		customProfile := &configv1.TLSSecurityProfile{
			Type: configv1.TLSProfileCustomType,
			Custom: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					Ciphers:       customCiphers,
					MinTLSVersion: configv1.VersionTLS12,
				},
			},
		}
		setAPIServerTLSProfile(oc, configChangeCtx, customProfile, "Custom")
		e2e.Logf("APIServer TLS profile updated to Custom (minTLSVersion=TLS12, ciphers=%d)", len(customCiphers))

		// 4. Wait for all operators to stabilize after Custom TLS profile change.
		g.By("waiting for all operators to stabilize after TLS profile change to Custom")
		waitForAllOperatorsAfterTLSChange(oc, configChangeCtx, "Custom")

		// 5. Verify ObservedConfig reflects Custom profile (VersionTLS12).
		g.By("verifying ObservedConfig reflects Custom profile (VersionTLS12)")
		verifyObservedConfigAfterSwitch(oc, configChangeCtx, "VersionTLS12", "Custom")

		// 6. Verify ConfigMaps reflect Custom profile (VersionTLS12).
		g.By("verifying ConfigMaps reflect Custom profile (VersionTLS12)")
		customExpectedTLSConfig := captureTLSConfiguration(customProfile)

		for _, t := range configMapTargets {
			g.By(fmt.Sprintf("verifying ConfigMap %s/%s reflects Custom profile", t.configMapNamespace, t.configMapName))
			err := testConfigMapTLSInjection(oc, configChangeCtx, t, customExpectedTLSConfig)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		// 7. Wire-level TLS verification for Custom profile.
		g.By("determining expected TLS version from current APIServer profile")
		tlsShouldWork, tlsShouldNotWork, profileTypeStr, err := getWireLevelTLSConfigs(oc, configChangeCtx)
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("Wire-level TLS configs determined from profile type: %s", profileTypeStr)

		for _, t := range serviceTargets {
			g.By(fmt.Sprintf("wire-level TLS check: svc/%s in %s (profile: %s)",
				t.serviceName, t.namespace, profileTypeStr))
			err := testWireLevelTLS(oc, configChangeCtx, t, tlsShouldWork, tlsShouldNotWork)
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("wire-level TLS check failed for svc/%s in %s (profile: %s)", t.serviceName, t.namespace, profileTypeStr))
			e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s (profile: %s)", t.serviceName, t.namespace, profileTypeStr)
		}

		e2e.Logf("PASS: Custom TLS profile verified successfully")
	})

	// ── ConfigMap annotation restoration tests ────────────────────────────
	// Validate all namespaces once upfront
	g.BeforeEach(func() {
		for _, target := range configMapTargets {
			validateNamespace(oc, ctx, target.configMapNamespace)
		}
	})

	for _, target := range configMapTargets {
		target := target

		g.It(fmt.Sprintf("should restore inject-tls annotation after deletion - %s", target.namespace), func() {
			testAnnotationRestorationAfterDeletion(oc, ctx, target)
		})
	}

	for _, target := range configMapTargets {
		target := target

		g.It(fmt.Sprintf("should restore inject-tls annotation when set to false - %s", target.namespace), func() {
			testAnnotationRestorationWhenFalse(oc, ctx, target)
		})
	}

	for _, target := range configMapTargets {
		target := target

		g.It(fmt.Sprintf("should restore servingInfo after removal - %s", target.namespace), func() {
			testServingInfoRestorationAfterRemoval(oc, ctx, target)
		})
	}

	for _, target := range configMapTargets {
		target := target

		g.It(fmt.Sprintf("should restore servingInfo after modification - %s", target.namespace), func() {
			testServingInfoRestorationAfterModification(oc, ctx, target)
		})
	}
})

// ─── Test implementations ──────────────────────────────────────────────────

// verifyAllTLSConfiguration runs all TLS validation tests across all components
// and reports any failures. This can be called multiple times (e.g., after TLS
// profile changes) to verify the configuration.
func verifyAllTLSConfiguration(oc *exutil.CLI, ctx context.Context, isHyperShiftCluster bool, targets tlsTestTargets, expectedTLSConfig tlsConfig) {
	state := newValidationState()

	e2e.Logf("Getting cluster APIServer TLS profile for wire-level tests")
	tlsShouldWork, tlsShouldNotWork, profileType, err := getWireLevelTLSConfigs(oc, ctx)
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Cluster TLS profile: %s", profileType)

	// Validate namespace existence for wire-level targets
	for _, target := range targets.services {
		if isHyperShiftCluster && target.managementClusterComponent {
			continue
		}
		validateNamespace(oc, ctx, target.namespace)
	}

	// Run validation once
	validateAllTargetsOnce(oc, ctx, isHyperShiftCluster, targets, expectedTLSConfig, state, tlsShouldWork, tlsShouldNotWork)

	// Collect all errors
	errors := make(map[string]error)
	for key, err := range state.observedConfigs {
		if err != nil {
			errors[fmt.Sprintf("ObservedConfig[%s]", key)] = err
		}
	}
	for key, err := range state.configMaps {
		if err != nil {
			errors[fmt.Sprintf("ConfigMap[%s]", key)] = err
		}
	}
	for key, err := range state.deploymentEnvVars {
		if err != nil {
			errors[fmt.Sprintf("DeploymentEnvVars[%s]", key)] = err
		}
	}
	for key, err := range state.services {
		if err != nil {
			errors[fmt.Sprintf("WireLevelTLS[%s]", key)] = err
		}
	}

	if len(errors) > 0 {
		var testNames []string
		for testName := range errors {
			testNames = append(testNames, testName)
		}
		slices.Sort(testNames)

		var errorMessages []string
		for _, testName := range testNames {
			errorMessages = append(errorMessages, fmt.Sprintf("  - %s: %v", testName, errors[testName]))
		}
		o.Expect(errors).To(o.BeEmpty(), "the following validations failed:\n"+strings.Join(errorMessages, "\n"))
	}
}

// testObservedConfig verifies that the operator's ObservedConfig contains
// a properly populated servingInfo with minTLSVersion and cipherSuites.
// This validates that the config observer controller (from library-go) is
// correctly watching the APIServer resource and writing the TLS config
// into the operator's ObservedConfig.
func testObservedConfig(oc *exutil.CLI, ctx context.Context, t observedConfigTarget, expected tlsConfig) error {
	e2e.Logf("Getting operator config %s/%s", t.operatorConfigGVR.Resource, t.operatorConfigName)
	resource, err := oc.AdminDynamicClient().Resource(t.operatorConfigGVR).Get(ctx, t.operatorConfigName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get operator config %s/%s: %w", t.operatorConfigGVR.Resource, t.operatorConfigName, err)
	}

	// Extract spec.observedConfig from the unstructured resource.
	fields := []string{"spec", "observedConfig"}
	observedConfigRaw, found, err := unstructured.NestedMap(resource.Object, fields...)
	if err != nil || !found {
		return fmt.Errorf("field %s not found or not a map type: %w", toPath(fields), err)
	}

	// Log only the servingInfo section for debugging.
	servingInfo, found, err := unstructured.NestedMap(observedConfigRaw, t.servingInfoPath...)
	if err == nil && found && servingInfo != nil {
		servingInfoJSON, err := json.MarshalIndent(servingInfo, "", "  ")
		if err == nil {
			e2e.Logf("ObservedConfig %s:\n%s", toPath(t.servingInfoPath), string(servingInfoJSON))
		}
	}

	e2e.Logf("Cross-checking configuration with expected TLS config")
	return validateServingInfoTLSConfig(oc, ctx, observedConfigRaw, t.servingInfoPath, expected)
}

// validateNamespace checks that the namespace exists, skipping the test if not.
func validateNamespace(oc *exutil.CLI, ctx context.Context, namespace string) {
	g.By(fmt.Sprintf("verifying namespace %s exists", namespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", namespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", namespace))
}

// getConfigMap fetches a ConfigMap from the API server.
func getConfigMap(oc *exutil.CLI, ctx context.Context, namespace, name string) *corev1.ConfigMap {
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", namespace, name))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", namespace, name))
	return cm
}

// requireAnnotation asserts the given annotation is present on the ConfigMap.
func requireAnnotation(cm *corev1.ConfigMap, annotationKey string) {
	_, found := cm.Annotations[annotationKey]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing %s annotation", cm.Namespace, cm.Name, annotationKey))
}

// updateConfigMap writes the ConfigMap back to the API server,
// retrying on conflict to handle concurrent controller reconciliation.
func updateConfigMap(oc *exutil.CLI, ctx context.Context, cm *corev1.ConfigMap) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		latest, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cm.Namespace).Get(ctx, cm.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		toUpdate := latest.DeepCopy()
		toUpdate.Annotations = cm.Annotations
		toUpdate.Data = cm.Data
		_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(cm.Namespace).Update(ctx, toUpdate, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to update ConfigMap %s/%s", cm.Namespace, cm.Name))
}

// waitForAnnotation polls until the given annotation reaches the expected value.
func waitForAnnotation(oc *exutil.CLI, ctx context.Context, namespace, name, annotationKey, annotationValue string) {
	g.By(fmt.Sprintf("waiting for %s annotation to become %q", annotationKey, annotationValue))
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}
			val, found := cm.Annotations[annotationKey]
			if found && val == annotationValue {
				e2e.Logf("  poll: annotation %s restored to %q", annotationKey, annotationValue)
				return true, nil
			}
			e2e.Logf("  poll: annotation not yet restored (found=%v, val=%s)", found, val)
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("%s annotation was not restored on ConfigMap %s/%s within timeout", annotationKey, namespace, name))
}

// testConfigMapTLSInjection verifies that CVO has injected TLS configuration
// into the operator's ConfigMap via the config.openshift.io/inject-tls annotation.
// This validates that CVO is reading the APIServer TLS profile and injecting
// the minTLSVersion and cipherSuites into the ConfigMap's servingInfo section.
func testConfigMapTLSInjection(oc *exutil.CLI, ctx context.Context, t configMapTarget, expected tlsConfig) error {
	validateNamespace(oc, ctx, t.configMapNamespace)
	cm := getConfigMap(oc, ctx, t.configMapNamespace, t.configMapName)

	e2e.Logf("Verifying %s annotation is present", injectTLSAnnotation)
	annotationValue, found := cm.Annotations[injectTLSAnnotation]
	if !found {
		return fmt.Errorf("ConfigMap %s/%s is missing %s annotation", t.configMapNamespace, t.configMapName, injectTLSAnnotation)
	}
	if annotationValue != "true" {
		return fmt.Errorf("ConfigMap %s/%s has inject-tls annotation but value is not 'true': %s", t.configMapNamespace, t.configMapName, annotationValue)
	}
	e2e.Logf("ConfigMap %s/%s has %s=true annotation", t.configMapNamespace, t.configMapName, injectTLSAnnotation)

	// Extract the config data from the ConfigMap.
	e2e.Logf("Extracting %s from ConfigMap data", t.configMapKey)
	configData, found := cm.Data[t.configMapKey]
	if !found {
		return fmt.Errorf("ConfigMap %s/%s is missing %s key", t.configMapNamespace, t.configMapName, t.configMapKey)
	}
	if configData == "" {
		return fmt.Errorf("ConfigMap %s/%s has empty %s", t.configMapNamespace, t.configMapName, t.configMapKey)
	}

	// Parse ConfigMap YAML data
	e2e.Logf("Parsing ConfigMap YAML data")
	var configObj map[string]interface{}
	err := yaml.Unmarshal([]byte(configData), &configObj)
	if err != nil {
		return fmt.Errorf("failed to parse ConfigMap %s/%s YAML data: %w", t.configMapNamespace, t.configMapName, err)
	}

	e2e.Logf("Cross-checking configuration with expected TLS config")
	return validateServingInfoTLSConfig(oc, ctx, configObj, []string{"servingInfo"}, expected)
}

// testAnnotationRestorationAfterDeletion verifies that if the inject-tls annotation
// is deleted from the ConfigMap, the operator restores it.
func testAnnotationRestorationAfterDeletion(oc *exutil.CLI, ctx context.Context, t configMapTarget) {
	// Get the original ConfigMap and verify annotation exists.
	cm := getConfigMap(oc, ctx, t.configMapNamespace, t.configMapName)
	requireAnnotation(cm, injectTLSAnnotation)

	// Delete the annotation.
	g.By("deleting " + injectTLSAnnotation + " annotation")
	delete(cm.Annotations, injectTLSAnnotation)
	updateConfigMap(oc, ctx, cm)
	e2e.Logf("Deleted inject-tls annotation from ConfigMap %s/%s", t.configMapNamespace, t.configMapName)

	waitForAnnotation(oc, ctx, t.configMapNamespace, t.configMapName, injectTLSAnnotation, "true")

	e2e.Logf("PASS: %s annotation was restored after deletion on ConfigMap %s/%s", injectTLSAnnotation, t.configMapNamespace, t.configMapName)
}

// testAnnotationRestorationWhenFalse verifies that if the inject-tls annotation
// is set to "false", the operator restores it to "true".
func testAnnotationRestorationWhenFalse(oc *exutil.CLI, ctx context.Context, t configMapTarget) {
	// Get the original ConfigMap.
	cm := getConfigMap(oc, ctx, t.configMapNamespace, t.configMapName)
	requireAnnotation(cm, injectTLSAnnotation)

	// Set the annotation to "false".
	g.By("setting " + injectTLSAnnotation + " annotation to 'false'")
	cm.Annotations[injectTLSAnnotation] = "false"
	updateConfigMap(oc, ctx, cm)
	e2e.Logf("Set inject-tls annotation to 'false' on ConfigMap %s/%s", t.configMapNamespace, t.configMapName)

	waitForAnnotation(oc, ctx, t.configMapNamespace, t.configMapName, injectTLSAnnotation, "true")

	e2e.Logf("PASS: %s annotation was restored to 'true' after being set to 'false' on ConfigMap %s/%s", injectTLSAnnotation, t.configMapNamespace, t.configMapName)
}

// testServingInfoRestorationAfterRemoval verifies that if the servingInfo section
// is removed from the ConfigMap, the operator restores it with correct TLS settings.
func testServingInfoRestorationAfterRemoval(oc *exutil.CLI, ctx context.Context, t configMapTarget) {
	// Get the original ConfigMap and verify servingInfo exists.
	cm := getConfigMap(oc, ctx, t.configMapNamespace, t.configMapName)

	// Verify servingInfo exists before we remove it.
	configData := cm.Data[t.configMapKey]
	if !strings.Contains(configData, "servingInfo") {
		g.Skip(fmt.Sprintf("ConfigMap %s/%s does not have servingInfo, skipping removal test", t.configMapNamespace, t.configMapName))
	}

	// Store original minTLSVersion to verify restoration.
	originalMinTLS := ""
	for _, line := range strings.Split(configData, "\n") {
		if strings.Contains(line, "minTLSVersion") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				originalMinTLS = strings.TrimSpace(parts[1])
				break
			}
		}
	}
	e2e.Logf("Original minTLSVersion: %s", originalMinTLS)

	// Remove servingInfo section from the config.
	g.By("removing servingInfo section from ConfigMap")
	// Simple approach: remove lines containing servingInfo and its nested content.
	var newLines []string
	inServingInfo := false
	indentLevel := 0
	for _, line := range strings.Split(configData, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "servingInfo:") {
			inServingInfo = true
			indentLevel = len(line) - len(strings.TrimLeft(line, " "))
			continue
		}
		if inServingInfo {
			currentIndent := len(line) - len(strings.TrimLeft(line, " "))
			if currentIndent > indentLevel || trimmed == "" {
				continue // Skip lines inside servingInfo block
			}
			inServingInfo = false
		}
		newLines = append(newLines, line)
	}
	cm.Data[t.configMapKey] = strings.Join(newLines, "\n")

	updateConfigMap(oc, ctx, cm)
	e2e.Logf("Removed servingInfo from ConfigMap %s/%s", t.configMapNamespace, t.configMapName)

	// Wait for the operator to restore servingInfo.
	g.By("waiting for operator to restore servingInfo section")
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(t.configMapNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}

			configData := cm.Data[t.configMapKey]
			if strings.Contains(configData, "servingInfo") && strings.Contains(configData, "minTLSVersion") {
				e2e.Logf("  poll: servingInfo restored!")
				return true, nil
			}
			e2e.Logf("  poll: servingInfo not yet restored")
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("servingInfo was not restored on ConfigMap %s/%s within timeout", t.configMapNamespace, t.configMapName))

	// Verify the restored config matches expected TLS version.
	cm, err = oc.AdminKubeClient().CoreV1().ConfigMaps(t.configMapNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	configData = cm.Data[t.configMapKey]
	o.Expect(configData).To(o.ContainSubstring("minTLSVersion"),
		"restored servingInfo should contain minTLSVersion")

	e2e.Logf("PASS: servingInfo was restored after removal on ConfigMap %s/%s", t.configMapNamespace, t.configMapName)
}

// testServingInfoRestorationAfterModification verifies that if the servingInfo
// minTLSVersion is modified to an incorrect value, the operator restores it.
func testServingInfoRestorationAfterModification(oc *exutil.CLI, ctx context.Context, t configMapTarget) {
	// Get the expected TLS version from the cluster profile.
	expectedMinVersion := getExpectedMinTLSVersion(oc, ctx)
	e2e.Logf("Expected minTLSVersion from cluster profile: %s", expectedMinVersion)

	// Get the original ConfigMap.
	cm := getConfigMap(oc, ctx, t.configMapNamespace, t.configMapName)

	// Verify servingInfo exists.
	configData := cm.Data[t.configMapKey]
	if !strings.Contains(configData, "minTLSVersion") {
		g.Skip(fmt.Sprintf("ConfigMap %s/%s does not have minTLSVersion, skipping modification test", t.configMapNamespace, t.configMapName))
	}

	// Determine a wrong value to set (opposite of expected).
	wrongValue := "VersionTLS10" // An obviously wrong/old TLS version
	if strings.Contains(configData, "VersionTLS10") {
		wrongValue = "VersionTLS99" // Use invalid version if TLS10 is somehow present
	}

	// Modify minTLSVersion to the wrong value.
	g.By(fmt.Sprintf("modifying minTLSVersion to wrong value: %s", wrongValue))
	// Replace the minTLSVersion line with wrong value.
	var newLines []string
	for _, line := range strings.Split(configData, "\n") {
		if strings.Contains(line, "minTLSVersion:") {
			// Preserve indentation.
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			newLines = append(newLines, fmt.Sprintf("%sminTLSVersion: %s", indent, wrongValue))
		} else {
			newLines = append(newLines, line)
		}
	}
	cm.Data[t.configMapKey] = strings.Join(newLines, "\n")

	updateConfigMap(oc, ctx, cm)
	e2e.Logf("Modified minTLSVersion to '%s' on ConfigMap %s/%s", wrongValue, t.configMapNamespace, t.configMapName)

	// Wait for the operator to restore correct minTLSVersion.
	g.By("waiting for operator to restore correct minTLSVersion")
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(t.configMapNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}

			configData := cm.Data[t.configMapKey]
			// Check if the wrong value is gone and expected value is present.
			if !strings.Contains(configData, wrongValue) && strings.Contains(configData, expectedMinVersion) {
				e2e.Logf("  poll: minTLSVersion restored to %s!", expectedMinVersion)
				return true, nil
			}
			e2e.Logf("  poll: minTLSVersion not yet restored (still has wrong value or missing expected)")
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("minTLSVersion was not restored on ConfigMap %s/%s within timeout (expected %s)",
			t.configMapNamespace, t.configMapName, expectedMinVersion))

	e2e.Logf("PASS: minTLSVersion was restored to '%s' after modification on ConfigMap %s/%s",
		expectedMinVersion, t.configMapNamespace, t.configMapName)
}

// testDeploymentTLSEnvVars verifies that the deployment in the given namespace
// has TLS environment variables that match the expected TLS profile.
func testDeploymentTLSEnvVars(oc *exutil.CLI, ctx context.Context, t deploymentEnvVarTarget, expected tlsConfig) error {
	validateNamespace(oc, ctx, t.namespace)

	e2e.Logf("Getting deployment %s/%s", t.namespace, t.deploymentName)
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(
		ctx, t.deploymentName, metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %w", t.namespace, t.deploymentName, err)
	}
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("deployment %s/%s has no containers", t.namespace, t.deploymentName)
	}

	e2e.Logf("Deployment %s/%s: generation=%d, observedGeneration=%d, replicas=%d/%d",
		t.namespace, t.deploymentName,
		deployment.Generation, deployment.Status.ObservedGeneration,
		deployment.Status.ReadyReplicas, deployment.Status.Replicas)

	e2e.Logf("Extracting TLS env vars from deployment %s/%s", t.namespace, t.deploymentName)
	envMap := findEnvAcrossContainers(deployment.Spec.Template.Spec.Containers, t.tlsMinVersionEnvVar, t.cipherSuitesEnvVar)
	e2e.Logf("Environment variables found: %v", envMap)

	minTLSVersion, found := envMap[t.tlsMinVersionEnvVar]
	if !found {
		return fmt.Errorf("expected %s to be set in deployment %s/%s", t.tlsMinVersionEnvVar, t.namespace, t.deploymentName)
	}

	cipherSuitesValue, found := envMap[t.cipherSuitesEnvVar]
	if !found {
		return fmt.Errorf("expected %s to be set in deployment %s/%s", t.cipherSuitesEnvVar, t.namespace, t.deploymentName)
	}

	// Parse cipher suites from env var (comma-separated IANA format)
	var cipherSuites []string
	for _, cipher := range strings.Split(cipherSuitesValue, ",") {
		trimmed := strings.TrimSpace(cipher)
		if trimmed != "" {
			cipherSuites = append(cipherSuites, trimmed)
		}
	}

	e2e.Logf("Cross-checking deployment %s/%s with expected TLS config", t.namespace, t.deploymentName)
	return validateTLSConfig(minTLSVersion, cipherSuites, expected)
}

// testWireLevelTLS verifies that the service endpoint enforces the TLS version
// using oc port-forward for connectivity. Caller should wait for deployment
// rollout before calling this if needed.
func testWireLevelTLS(oc *exutil.CLI, ctx context.Context, t serviceTarget, tlsShouldWork, tlsShouldNotWork *tls.Config) error {
	e2e.Logf("Verifying TLS behavior via port-forward to svc/%s in %s on port %s",
		t.serviceName, t.namespace, t.servicePort)
	err := forwardPortAndExecute(t.serviceName, t.namespace, t.servicePort,
		func(localPort int) error {
			return checkTLSConnection(localPort, tlsShouldWork, tlsShouldNotWork, t)
		},
	)
	if err != nil {
		return fmt.Errorf("wire-level TLS test failed for svc/%s in %s:%s: %w",
			t.serviceName, t.namespace, t.servicePort, err)
	}

	return nil
}

// ─── Helper functions ──────────────────────────────────────────────────────

// captureTLSConfiguration extracts the effective TLS configuration from a
// TLSSecurityProfile (profile type, minTLSVersion, cipherSuites).
func captureTLSConfiguration(profile *configv1.TLSSecurityProfile) tlsConfig {
	// Determine profile type, defaulting to crypto.DefaultTLSProfileType if nil
	profileType := crypto.DefaultTLSProfileType
	if profile != nil {
		profileType = profile.Type
	}

	// Get effective minTLSVersion and cipherSuites
	minTLSVersion, cipherSuites := getSecurityProfileCiphers(profile)

	return tlsConfig{
		profileType:   profileType,
		minTLSVersion: minTLSVersion,
		cipherSuites:  cipherSuites,
	}
}

// generateDifferentTLSProfile creates a TLS profile that differs from the current
// configuration in both TLS version and cipher suites. This ensures tests can run
// repeatedly without requiring restoration of the original configuration.
func generateDifferentTLSProfile(currentTLSConfig tlsConfig) (*configv1.TLSSecurityProfile, tlsConfig) {
	// Choose different TLS version than current
	var targetMinTLSVersion configv1.TLSProtocolVersion
	if currentTLSConfig.minTLSVersion == "VersionTLS12" {
		targetMinTLSVersion = configv1.VersionTLS11
	} else {
		targetMinTLSVersion = configv1.VersionTLS12
	}

	// Define two completely distinct cipher sets (all from Intermediate TLS profile)
	cipherSetA := []string{
		"TLS_AES_128_GCM_SHA256",
		"TLS_AES_256_GCM_SHA384",
		"TLS_CHACHA20_POLY1305_SHA256",
		"ECDHE-RSA-AES128-GCM-SHA256",
		"ECDHE-ECDSA-AES128-GCM-SHA256",
	}
	cipherSetB := []string{
		"TLS_AES_128_GCM_SHA256",
		"TLS_CHACHA20_POLY1305_SHA256",
		"ECDHE-ECDSA-AES128-GCM-SHA256",
		"ECDHE-RSA-AES256-GCM-SHA384",
		"ECDHE-ECDSA-AES256-GCM-SHA384",
	}

	// Compare current ciphers with Set A to choose different set
	cipherSetA_IANA := crypto.OpenSSLToIANACipherSuites(cipherSetA)
	currentSorted := slices.Clone(currentTLSConfig.cipherSuites)
	setASorted := slices.Clone(cipherSetA_IANA)
	slices.Sort(currentSorted)
	slices.Sort(setASorted)

	var targetCiphers []string
	if slices.Equal(currentSorted, setASorted) {
		targetCiphers = cipherSetB
	} else {
		targetCiphers = cipherSetA
	}

	// Create Custom TLS profile with chosen version and ciphers
	targetProfile := &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileCustomType,
		Custom: &configv1.CustomTLSProfile{
			TLSProfileSpec: configv1.TLSProfileSpec{
				MinTLSVersion: targetMinTLSVersion,
				Ciphers:       targetCiphers,
			},
		},
	}

	// Capture the target configuration for comparison
	targetTLSConfig := captureTLSConfiguration(targetProfile)

	return targetProfile, targetTLSConfig
}

// setAPIServerTLSProfile updates the APIServer TLS profile to the specified value.
// This function handles the retry logic for conflicts during the update.
func setAPIServerTLSProfile(oc *exutil.CLI, ctx context.Context, profile *configv1.TLSSecurityProfile, profileLabel string) {
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		apiServer.Spec.TLSSecurityProfile = profile
		_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(ctx, apiServer, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to update APIServer TLS profile to %s", profileLabel)
}

// restoreOriginalTLSProfile restores the APIServer TLS profile to its original value
// and waits for all operators to stabilize. This is typically used in DeferCleanup.
func restoreOriginalTLSProfile(oc *exutil.CLI, ctx context.Context, originalProfile *configv1.TLSSecurityProfile, profileDesc string) {
	e2e.Logf("DeferCleanup: restoring original TLS profile: %s", profileDesc)
	setAPIServerTLSProfile(oc, ctx, originalProfile, "restore")

	e2e.Logf("DeferCleanup: waiting for all operators to stabilize after restoring profile")
	waitForAllOperatorsAfterTLSChange(oc, ctx, "restore")
	e2e.Logf("DeferCleanup: original TLS profile restored and cluster is stable")
}

// verifyObservedConfigAfterSwitch checks that every target with an operator
// config has its ObservedConfig servingInfo.minTLSVersion matching the
// expected version after a profile switch.
func verifyObservedConfigAfterSwitch(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string) {
	verifyObservedConfigForTargets(oc, ctx, expectedVersion, profileLabel, observedConfigTargets)
}

// verifyObservedConfigForTargets checks a specific list of targets for
// ObservedConfig correctness after a TLS profile switch.
func verifyObservedConfigForTargets(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string, targetList []observedConfigTarget) {
	dynClient := oc.AdminDynamicClient()
	for _, t := range targetList {
		resource, err := dynClient.Resource(t.operatorConfigGVR).Get(ctx, t.operatorConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("failed to get operator config %s/%s after %s switch",
				t.operatorConfigGVR.Resource, t.operatorConfigName, profileLabel))

		observedConfigRaw, found, err := unstructured.NestedMap(resource.Object, "spec", "observedConfig")
		if err != nil || !found {
			o.Expect(fmt.Errorf("field spec.observedConfig not found or not a map type: %w", err)).NotTo(o.HaveOccurred())
		}

		minTLSVersionPath := append(t.servingInfoPath, "minTLSVersion")
		minTLSVersion, found, err := unstructured.NestedString(observedConfigRaw, minTLSVersionPath...)
		if err != nil || !found {
			o.Expect(fmt.Errorf("field %s not found or not a string type: %w", toPath(minTLSVersionPath), err)).NotTo(o.HaveOccurred())
		}
		o.Expect(minTLSVersion).To(o.Equal(expectedVersion),
			fmt.Sprintf("ObservedConfig %s/%s: expected minTLSVersion=%s after %s switch, got %s",
				t.operatorConfigGVR.Resource, t.operatorConfigName, expectedVersion, profileLabel, minTLSVersion))
		e2e.Logf("PASS: ObservedConfig %s/%s has minTLSVersion=%s after %s switch",
			t.operatorConfigGVR.Resource, t.operatorConfigName, minTLSVersion, profileLabel)
	}
}

// verifyConfigMapsAfterSwitch checks that every target with a ConfigMap has
// the expected minTLSVersion in its servingInfo after a profile switch.
func verifyConfigMapsAfterSwitch(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string) {
	verifyConfigMapsForTargets(oc, ctx, expectedVersion, profileLabel, configMapTargets)
}

// verifyConfigMapsForTargets checks a specific list of targets for
// ConfigMap TLS injection correctness after a TLS profile switch.
// It polls each ConfigMap for up to 5 minutes because the TLS annotation
// injection can lag behind operator/deployment stabilization.
func verifyConfigMapsForTargets(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string, targetList []configMapTarget) {
	for _, t := range targetList {
		e2e.Logf("Waiting for ConfigMap %s/%s to reflect %s after %s switch",
			t.configMapNamespace, t.configMapName, expectedVersion, profileLabel)
		err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
			func(ctx context.Context) (bool, error) {
				cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(t.configMapNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
				if err != nil {
					e2e.Logf("  poll: ConfigMap %s/%s not found: %v", t.configMapNamespace, t.configMapName, err)
					return false, nil
				}
				if _, ok := cm.Annotations[injectTLSAnnotation]; !ok {
					e2e.Logf("  poll: ConfigMap %s/%s missing %s annotation", t.configMapNamespace, t.configMapName, injectTLSAnnotation)
					return false, nil
				}
				configData := cm.Data[t.configMapKey]
				if !strings.Contains(configData, expectedVersion) {
					e2e.Logf("  poll: ConfigMap %s/%s does not yet contain %s", t.configMapNamespace, t.configMapName, expectedVersion)
					return false, nil
				}
				return true, nil
			})
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("ConfigMap %s/%s did not contain %s within 5 minutes after %s switch",
				t.configMapNamespace, t.configMapName, expectedVersion, profileLabel))
		e2e.Logf("PASS: ConfigMap %s/%s has %s after %s switch",
			t.configMapNamespace, t.configMapName, expectedVersion, profileLabel)
	}
}

// getWireLevelTLSConfigs returns TLS configs for wire-level testing based on the current APIServer profile.
// Returns: tlsShouldWork, tlsShouldNotWork, profileTypeStr, error
func getWireLevelTLSConfigs(oc *exutil.CLI, ctx context.Context) (*tls.Config, *tls.Config, string, error) {
	currentProfile, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to get APIServer config: %w", err)
	}

	profileType := crypto.DefaultTLSProfileType
	if currentProfile.Spec.TLSSecurityProfile != nil {
		profileType = currentProfile.Spec.TLSSecurityProfile.Type
	}

	var minTLSVersionStr configv1.TLSProtocolVersion
	if profileType == configv1.TLSProfileCustomType {
		if currentProfile.Spec.TLSSecurityProfile.Custom == nil {
			return nil, nil, "", fmt.Errorf("Custom TLS profile set but .custom spec is nil")
		}
		minTLSVersionStr = currentProfile.Spec.TLSSecurityProfile.Custom.MinTLSVersion
	} else {
		profileSpec, ok := configv1.TLSProfiles[profileType]
		if !ok {
			return nil, nil, "", fmt.Errorf("unknown TLS profile type: %s", profileType)
		}
		minTLSVersionStr = profileSpec.MinTLSVersion
	}

	minTLSVersion := tlsVersionStringToUint16(minTLSVersionStr)
	if minTLSVersion == 0 {
		return nil, nil, "", fmt.Errorf("failed to convert TLS version: %s", minTLSVersionStr)
	}

	var tlsShouldWork, tlsShouldNotWork *tls.Config
	switch minTLSVersion {
	case tls.VersionTLS11:
		tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS11, InsecureSkipVerify: true}
		tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS10, InsecureSkipVerify: true}
	case tls.VersionTLS12:
		tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
		tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}
	case tls.VersionTLS13:
		tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
		tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
	default:
		return nil, nil, "", fmt.Errorf("unsupported minTLSVersion for wire-level testing: %s", minTLSVersionStr)
	}

	profileTypeStr := string(profileType)
	return tlsShouldWork, tlsShouldNotWork, profileTypeStr, nil
}

// validationState tracks validation results for all targets.
// Each map stores errors (nil = reconciled/passed, non-nil = failed).
type validationState struct {
	observedConfigs   map[string]error
	configMaps        map[string]error
	deploymentEnvVars map[string]error
	services          map[string]error
}

// newValidationState creates an initialized validation state.
func newValidationState() *validationState {
	return &validationState{
		observedConfigs:   make(map[string]error),
		configMaps:        make(map[string]error),
		deploymentEnvVars: make(map[string]error),
		services:          make(map[string]error),
	}
}

// validateAllTargetsOnce validates all targets once and updates the state.
// Returns (reconciledCount, totalCount) for progress tracking.
// Skips targets that are already reconciled (state[key] == nil).
func validateAllTargetsOnce(
	oc *exutil.CLI,
	ctx context.Context,
	isHyperShiftCluster bool,
	targets tlsTestTargets,
	expectedTLSConfig tlsConfig,
	state *validationState,
	tlsShouldWork, tlsShouldNotWork *tls.Config,
) (reconciledCount, totalCount int) {
	// ObservedConfig targets
	for _, target := range targets.observedConfig {
		if isHyperShiftCluster && target.managementClusterComponent {
			continue
		}
		totalCount++

		key := fmt.Sprintf("%s/%s", target.namespace, target.operatorConfigName)
		if err, checked := state.observedConfigs[key]; checked && err == nil {
			// Already reconciled successfully, skip
			reconciledCount++
			continue
		}

		err := testObservedConfig(oc, ctx, target, expectedTLSConfig)
		state.observedConfigs[key] = err
		if err == nil {
			reconciledCount++
		}
	}

	// ConfigMap targets
	for _, target := range targets.configMaps {
		totalCount++

		key := fmt.Sprintf("%s/%s", target.configMapNamespace, target.configMapName)
		if err, checked := state.configMaps[key]; checked && err == nil {
			// Already reconciled successfully, skip
			reconciledCount++
			continue
		}

		err := testConfigMapTLSInjection(oc, ctx, target, expectedTLSConfig)
		state.configMaps[key] = err
		if err == nil {
			reconciledCount++
		}
	}

	// DeploymentEnvVar targets
	for _, target := range targets.deploymentEnvVars {
		if isHyperShiftCluster && target.managementClusterComponent {
			continue
		}
		totalCount++

		key := fmt.Sprintf("%s/%s", target.namespace, target.deploymentName)
		if err, checked := state.deploymentEnvVars[key]; checked && err == nil {
			// Already reconciled successfully, skip
			reconciledCount++
			continue
		}

		err := testDeploymentTLSEnvVars(oc, ctx, target, expectedTLSConfig)
		state.deploymentEnvVars[key] = err
		if err == nil {
			reconciledCount++
		}
	}

	// Service targets (wire-level TLS)
	for _, target := range targets.services {
		if isHyperShiftCluster && target.managementClusterComponent {
			continue
		}
		totalCount++

		key := fmt.Sprintf("%s/%s:%s", target.namespace, target.serviceName, target.servicePort)
		if err, checked := state.services[key]; checked && err == nil {
			// Already reconciled successfully, skip
			reconciledCount++
			continue
		}

		// Wait for deployment to finish rolling out before testing wire-level TLS.
		// This makes the test more stable by avoiding confusing error messages about
		// TLS compliance when the deployment is still updating.
		if target.deploymentName != "" {
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(target.namespace).Get(ctx, target.deploymentName, metav1.GetOptions{})
			if err != nil {
				state.services[key] = fmt.Errorf("failed to get deployment: %w", err)
				continue
			}
			if err := waitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, 2*time.Minute); err != nil {
				state.services[key] = fmt.Errorf("deployment not ready: %w", err)
				continue
			}
		}

		err := testWireLevelTLS(oc, ctx, target, tlsShouldWork, tlsShouldNotWork)
		state.services[key] = err
		if err == nil {
			reconciledCount++
		}
	}

	return reconciledCount, totalCount
}

// waitForTLSReconciliation polls all target objects until their TLS configuration
// matches the expected TLS configuration. This is reconciliation-based waiting, not rollout-based.
//
// This validates:
// - Config has propagated to ObservedConfig, ConfigMaps, and deployment env vars
// - Wire-level TLS enforcement: services accept/reject the correct TLS versions
//
// Note: Complete cipher list validation would require testing all ciphers against all endpoints,
// which is out of scope for this e2e test. In a test environment without middle-man attacks,
// minTLSVersion validation is a sufficient indicator that the TLS config propagated correctly.
func waitForTLSReconciliation(
	oc *exutil.CLI,
	ctx context.Context,
	isHyperShiftCluster bool,
	targets tlsTestTargets,
	expectedTLSConfig tlsConfig,
) error {
	const (
		timeout         = 25 * time.Minute
		pollingInterval = 10 * time.Second
	)

	state := newValidationState()
	startTime := time.Now()

	e2e.Logf("Starting TLS reconciliation wait (timeout: %v, polling: %v)", timeout, pollingInterval)
	e2e.Logf("Expected TLS config: type=%s, minTLSVersion=%s, ciphers=%d",
		expectedTLSConfig.profileType, expectedTLSConfig.minTLSVersion, len(expectedTLSConfig.cipherSuites))

	tlsShouldWork, tlsShouldNotWork, profileType, err := getWireLevelTLSConfigs(oc, ctx)
	if err != nil {
		return fmt.Errorf("failed to get wire-level TLS configs: %w", err)
	}
	e2e.Logf("Wire-level TLS test configs for profile: %s", profileType)

	err = wait.PollUntilContextTimeout(ctx, pollingInterval, timeout, true,
		func(ctx context.Context) (bool, error) {
			reconciledCount, totalCount := validateAllTargetsOnce(
				oc, ctx, isHyperShiftCluster, targets, expectedTLSConfig, state,
				tlsShouldWork, tlsShouldNotWork,
			)

			elapsed := time.Since(startTime).Round(time.Second)
			e2e.Logf("Reconciliation progress: %d/%d objects reconciled (elapsed: %v)", reconciledCount, totalCount, elapsed)

			if reconciledCount == totalCount && totalCount > 0 {
				return true, nil
			}

			return false, nil
		})

	if err != nil {
		var notReconciled []string

		for key, err := range state.observedConfigs {
			if err != nil {
				notReconciled = append(notReconciled, fmt.Sprintf("ObservedConfig[%s]: %v", key, err))
			}
		}
		for key, err := range state.configMaps {
			if err != nil {
				notReconciled = append(notReconciled, fmt.Sprintf("ConfigMap[%s]: %v", key, err))
			}
		}
		for key, err := range state.deploymentEnvVars {
			if err != nil {
				notReconciled = append(notReconciled, fmt.Sprintf("DeploymentEnvVars[%s]: %v", key, err))
			}
		}
		for key, err := range state.services {
			if err != nil {
				notReconciled = append(notReconciled, fmt.Sprintf("WireLevelTLS[%s]: %v", key, err))
			}
		}

		return fmt.Errorf("TLS reconciliation timeout after %v. Objects not reconciled:\n%s", timeout, strings.Join(notReconciled, "\n"))
	}

	e2e.Logf("PASS: All TLS targets reconciled in %v", time.Since(startTime).Round(time.Second))
	return nil
}

// getExpectedMinTLSVersion returns the expected minTLSVersion string
// (e.g. "VersionTLS12", "VersionTLS13") based on the cluster APIServer profile.
func getExpectedMinTLSVersion(oc *exutil.CLI, ctx context.Context) string {
	minVersion, _, _ := getExpectedMinTLSVersionWithType(oc, ctx)
	return minVersion
}

// getExpectedMinTLSVersionWithType returns the expected minTLSVersion string,
// cipher suites, and the profile type name for better logging.
func getExpectedMinTLSVersionWithType(oc *exutil.CLI, ctx context.Context) (string, []string, string) {
	config, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	profileType := configv1.TLSProfileIntermediateType
	if config.Spec.TLSSecurityProfile != nil {
		profileType = config.Spec.TLSSecurityProfile.Type
	}

	var minVersion string
	var ciphers []string
	if profileType == configv1.TLSProfileCustomType {
		o.Expect(config.Spec.TLSSecurityProfile.Custom).NotTo(o.BeNil(),
			"Custom TLS profile set but .custom spec is nil")
		minVersion = string(config.Spec.TLSSecurityProfile.Custom.MinTLSVersion)
		ciphers = config.Spec.TLSSecurityProfile.Custom.Ciphers
	} else {
		profile, ok := configv1.TLSProfiles[profileType]
		if !ok {
			e2e.Failf("Unknown TLS profile type: %s", profileType)
		}
		minVersion = string(profile.MinTLSVersion)
		ciphers = profile.Ciphers
	}

	profileName := string(profileType)
	if profileType == "" || profileType == configv1.TLSProfileIntermediateType {
		profileName = "Intermediate (default)"
	}

	e2e.Logf("Cluster APIServer TLS profile: type=%s, minTLSVersion=%s, ciphers=%d", profileName, minVersion, len(ciphers))
	return minVersion, ciphers, profileName
}

// forwardPortAndExecute sets up oc port-forward to a service and executes
// the given test function with the local port.  Retries up to 5 times with
// exponential backoff (2s, 4s, 8s, 16s) to handle pods restarting after
// config changes.
func forwardPortAndExecute(serviceName, namespace, remotePort string, toExecute func(localPort int) error) error {
	const maxAttempts = 5
	var err error
	backoff := 2 * time.Second
	for i := 0; i < maxAttempts; i++ {
		if err = func() error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

			ready := false
			for j := 0; j < 20; j++ {
				output := readPartialFrom(stdout, 1024)
				if strings.Contains(output, "Forwarding from") {
					e2e.Logf("oc port-forward ready: %s", output)
					ready = true
					break
				}

				testConn, testErr := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 200*time.Millisecond)
				if testErr == nil {
					testConn.Close()
					e2e.Logf("oc port-forward ready (port accepting connections)")
					ready = true
					break
				}

				time.Sleep(500 * time.Millisecond)
			}

			if !ready {
				stderrOutput := readPartialFrom(stderr, 1024)
				return fmt.Errorf("port-forward did not become ready within timeout (stderr: %s)", stderrOutput)
			}

			return toExecute(localPort)
		}(); err == nil {
			return nil
		}
		e2e.Logf("port-forward attempt %d/%d failed: %v", i+1, maxAttempts, err)
		if i < maxAttempts-1 {
			isPodNotReady := strings.Contains(err.Error(), "not running") ||
				strings.Contains(err.Error(), "Pending") ||
				strings.Contains(err.Error(), "CrashLoopBackOff")
			if isPodNotReady {
				e2e.Logf("pod backing svc/%s is not ready, waiting %v before retry", serviceName, backoff)
			}
			time.Sleep(backoff)
			backoff *= 2
		}
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

// tlsVersionStringToUint16 converts configv1.TLSProtocolVersion string to crypto/tls version constant.
func tlsVersionStringToUint16(version configv1.TLSProtocolVersion) uint16 {
	switch version {
	case configv1.VersionTLS10:
		return tls.VersionTLS10
	case configv1.VersionTLS11:
		return tls.VersionTLS11
	case configv1.VersionTLS12:
		return tls.VersionTLS12
	case configv1.VersionTLS13:
		return tls.VersionTLS13
	default:
		return 0
	}
}

// tlsVersionName returns a human-readable name for a TLS version constant.
func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

// checkTLSConnection verifies that a local-forwarded port accepts the expected
// TLS version and rejects the one that should not work.
// Tests both IPv4 (127.0.0.1) and IPv6 ([::1]) localhost addresses when available.
func checkTLSConnection(localPort int, shouldWork, shouldNotWork *tls.Config, t serviceTarget) error {
	// Test both IPv4 and IPv6 localhost addresses.
	// On IPv6 clusters, we want to verify TLS works on both address families.
	hosts := []string{
		fmt.Sprintf("127.0.0.1:%d", localPort), // IPv4
		fmt.Sprintf("[::1]:%d", localPort),     // IPv6
	}

	// Determine the TLS versions we're testing with.
	expectedMinVersion := tlsVersionName(shouldWork.MinVersion)
	rejectedMaxVersion := tlsVersionName(shouldNotWork.MaxVersion)

	var testedHosts []string

	for _, host := range hosts {
		hostType := "IPv4"
		if strings.HasPrefix(host, "[") {
			hostType = "IPv6"
		}

		e2e.Logf("[%s] %s: Testing connection with min %s (should SUCCEED)",
			hostType, host, expectedMinVersion)

		dialer := &net.Dialer{Timeout: 10 * time.Second}

		// Try to connect with the TLS config that should work.
		conn, err := tls.DialWithDialer(dialer, "tcp", host, shouldWork)
		if err != nil {
			errStr := err.Error()
			// If host is not available (network issue), skip to next host.
			if strings.Contains(errStr, "connection refused") ||
				strings.Contains(errStr, "network is unreachable") ||
				strings.Contains(errStr, "no route to host") ||
				strings.Contains(errStr, "connect: cannot assign requested address") {
				e2e.Logf("[%s] %s: Host not available, skipping: %v", hostType, host, err)
				continue
			}
			// TLS error - this is a real failure.
			return fmt.Errorf("svc/%s in %s [%s]: Connection with %s FAILED (expected success): %w",
				t.serviceName, t.namespace, hostType, expectedMinVersion, err)
		}

		// Connection succeeded - verify the negotiated version.
		negotiated := conn.ConnectionState().Version
		conn.Close()
		e2e.Logf("[%s] %s: SUCCESS - Negotiated %s (requested min %s)",
			hostType, host, tlsVersionName(negotiated), expectedMinVersion)

		// Test that the version that should not work is rejected.
		e2e.Logf("[%s] %s: Testing connection with max %s (should be REJECTED)",
			hostType, host, rejectedMaxVersion)

		conn, err = tls.DialWithDialer(dialer, "tcp", host, shouldNotWork)
		if err == nil {
			negotiatedBad := conn.ConnectionState().Version
			conn.Close()
			return fmt.Errorf("svc/%s in %s [%s]: Connection with max %s should be REJECTED but succeeded (negotiated %s)",
				t.serviceName, t.namespace, hostType, rejectedMaxVersion, tlsVersionName(negotiatedBad))
		}

		// Verify we got a TLS-related or connection-closed error.
		// Some servers (e.g. etcd) close the connection with EOF or
		// "connection reset by peer" instead of sending a TLS alert
		// when the offered TLS version is unsupported.
		errStr := err.Error()
		if !strings.Contains(errStr, "protocol version") &&
			!strings.Contains(errStr, "no supported versions") &&
			!strings.Contains(errStr, "handshake failure") &&
			!strings.Contains(errStr, "alert") &&
			!strings.Contains(errStr, "EOF") &&
			!strings.Contains(errStr, "connection reset by peer") {
			return fmt.Errorf("svc/%s in %s [%s]: Expected TLS version rejection error, got: %w",
				t.serviceName, t.namespace, hostType, err)
		}
		e2e.Logf("[%s] %s: REJECTED - %s correctly refused by server",
			hostType, host, rejectedMaxVersion)

		testedHosts = append(testedHosts, fmt.Sprintf("%s(%s)", hostType, host))
	}

	if len(testedHosts) == 0 {
		return fmt.Errorf("svc/%s in %s: No hosts available for testing (tried IPv4 and IPv6)",
			t.serviceName, t.namespace)
	}

	e2e.Logf("svc/%s in %s: ✓ TLS PASS - Verified on %d host(s): %v | Accepts: %s+ | Rejects: %s",
		t.serviceName, t.namespace, len(testedHosts), testedHosts, expectedMinVersion, rejectedMaxVersion)
	return nil
}

// waitForDeploymentRolloutAfterTLSChange waits for a deployment's pods to be
// replaced after a TLS config change. It captures the current pod UIDs, then
// polls until all old pods are gone and the deployment is fully ready. This
// ensures the running pods have picked up the new TLS configuration before
// wire-level checks are performed.
func waitForDeploymentRolloutAfterTLSChange(oc *exutil.CLI, ctx context.Context, namespace, deploymentName string) {
	e2e.Logf("Waiting for deployment %s/%s to roll out new pods after TLS change", namespace, deploymentName)

	oldPods := make(map[string]bool)
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, p := range podList.Items {
			if strings.Contains(p.Name, deploymentName) {
				oldPods[string(p.UID)] = true
			}
		}
	}
	e2e.Logf("Captured %d existing pods for deployment %s/%s", len(oldPods), namespace, deploymentName)

	err = wait.PollUntilContextTimeout(ctx, 10*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}

			replicas := int32(1)
			if deployment.Spec.Replicas != nil {
				replicas = *deployment.Spec.Replicas
			}

			if deployment.Status.UpdatedReplicas < replicas ||
				deployment.Status.ReadyReplicas < replicas ||
				deployment.Status.UnavailableReplicas > 0 {
				e2e.Logf("  poll: deployment %s/%s rolling (updated=%d, ready=%d, unavailable=%d)",
					namespace, deploymentName,
					deployment.Status.UpdatedReplicas,
					deployment.Status.ReadyReplicas,
					deployment.Status.UnavailableReplicas)
				return false, nil
			}

			if deployment.Status.ObservedGeneration < deployment.Generation {
				e2e.Logf("  poll: deployment %s/%s generation not yet observed (%d < %d)",
					namespace, deploymentName,
					deployment.Status.ObservedGeneration, deployment.Generation)
				return false, nil
			}

			currentPods, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
			if err != nil {
				return false, nil
			}
			for _, p := range currentPods.Items {
				if oldPods[string(p.UID)] && p.DeletionTimestamp == nil {
					if strings.Contains(p.Name, deploymentName) {
						e2e.Logf("  poll: old pod %s still running in %s", p.Name, namespace)
						return false, nil
					}
				}
			}

			e2e.Logf("Deployment %s/%s has rolled out new pods", namespace, deploymentName)
			return true, nil
		})
	if err != nil {
		e2e.Logf("WARNING: deployment %s/%s rollout wait timed out: %v (proceeding with wire-level check)", namespace, deploymentName, err)
	}
}

// waitForDeploymentCompleteWithTimeout waits for a deployment to complete rollout
// with a configurable timeout. This is a wrapper around the standard k8s e2e
// deployment helper but with an extended timeout for slow rollouts.
func waitForDeploymentCompleteWithTimeout(ctx context.Context, c clientset.Interface, d *appsv1.Deployment, timeout time.Duration) error {
	e2e.Logf("Waiting for deployment %s/%s to complete (timeout: %v)", d.Namespace, d.Name, timeout)
	start := time.Now()

	return wait.PollUntilContextTimeout(ctx, 2*time.Second, timeout, true,
		func(ctx context.Context) (bool, error) {
			deployment, err := c.AppsV1().Deployments(d.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll[%v]: error getting deployment: %v", time.Since(start).Round(time.Second), err)
				return false, nil
			}

			// Check if deployment is complete: all replicas updated, ready, and no unavailable.
			replicas := int32(1)
			if deployment.Spec.Replicas != nil {
				replicas = *deployment.Spec.Replicas
			}

			ready := deployment.Status.ReadyReplicas
			updated := deployment.Status.UpdatedReplicas
			available := deployment.Status.AvailableReplicas
			unavailable := deployment.Status.UnavailableReplicas

			if updated == replicas && ready == replicas && available == replicas && unavailable == 0 {
				e2e.Logf("  poll[%v]: deployment %s/%s is complete (ready=%d/%d)",
					time.Since(start).Round(time.Second), d.Namespace, d.Name, ready, replicas)
				return true, nil
			}

			// Log progress every 30 seconds to avoid spam.
			elapsed := time.Since(start)
			if elapsed.Seconds() > 0 && int(elapsed.Seconds())%30 == 0 {
				e2e.Logf("  poll[%v]: deployment %s/%s not ready (replicas=%d, ready=%d, updated=%d, unavailable=%d)",
					elapsed.Round(time.Second), d.Namespace, d.Name, replicas, ready, updated, unavailable)
			}

			return false, nil
		})
}

// envToMap converts a slice of container environment variables to a map.
func envToMap(envVars []corev1.EnvVar) map[string]string {
	m := make(map[string]string, len(envVars))
	for _, e := range envVars {
		m[e.Name] = e.Value
	}
	return m
}

// findEnvAcrossContainers searches all containers in a pod spec for the
// given env var keys and returns a map containing the first occurrence of
// each key found across all containers.
func findEnvAcrossContainers(containers []corev1.Container, keys ...string) map[string]string {
	result := make(map[string]string)

	// Iterate through containers first to avoid calling envToMap multiple times
	for _, c := range containers {
		m := envToMap(c.Env)
		for _, key := range keys {
			// Only add if not already found (first occurrence wins)
			if _, found := result[key]; !found {
				if value, ok := m[key]; ok {
					result[key] = value
				}
			}
		}
	}

	return result
}

// toPath joins field path segments with "." separator and prefixes with ".".
func toPath(fields []string) string {
	return "." + strings.Join(fields, ".")
}

// getSecurityProfileCiphers extracts the minimum TLS version and cipher suites from TLSSecurityProfile object,
// converts the ciphers to IANA names as supported by Kube ServingInfo config.
// If profile is nil, returns config defined by the Intermediate TLS Profile.
// Duplicated from: https://raw.githubusercontent.com/openshift/library-go/refs/heads/master/pkg/operator/configobserver/apiserver/observe_tlssecurityprofile.go
func getSecurityProfileCiphers(profile *configv1.TLSSecurityProfile) (string, []string) {
	var profileType configv1.TLSProfileType
	if profile == nil {
		profileType = crypto.DefaultTLSProfileType
	} else {
		profileType = profile.Type
	}

	var profileSpec *configv1.TLSProfileSpec
	if profileType == configv1.TLSProfileCustomType {
		if profile.Custom != nil {
			profileSpec = &profile.Custom.TLSProfileSpec
		}
	} else {
		profileSpec = configv1.TLSProfiles[profileType]
	}

	// nothing found / custom type set but no actual custom spec
	if profileSpec == nil {
		profileSpec = configv1.TLSProfiles[crypto.DefaultTLSProfileType]
	}

	// need to remap all Ciphers to their respective IANA names used by Go
	return string(profileSpec.MinTLSVersion), crypto.OpenSSLToIANACipherSuites(profileSpec.Ciphers)
}

// validateTLSConfig validates that the given minTLSVersion and cipherSuites match
// the expected values from the APIServer's TLSSecurityProfile.
// Returns an error if validation fails.
func validateTLSConfig(minTLSVersion string, cipherSuites []string, expected tlsConfig) error {
	// Verify minTLSVersion matches
	if minTLSVersion != expected.minTLSVersion {
		return fmt.Errorf("minTLSVersion mismatch: got %s, expected %s", minTLSVersion, expected.minTLSVersion)
	}

	// Verify cipher suites match
	if !cipherSuitesMatch(cipherSuites, expected.cipherSuites) {
		return fmt.Errorf("cipherSuites mismatch.\nExpected: %v\nGot: %v", expected.cipherSuites, cipherSuites)
	}

	return nil
}

// cipherSuitesMatch checks if two cipher suite slices contain the same elements (order-independent).
func cipherSuitesMatch(actual, expected []string) bool {
	sortedActual := slices.Clone(actual)
	sortedExpected := slices.Clone(expected)
	slices.Sort(sortedActual)
	slices.Sort(sortedExpected)
	return slices.Equal(sortedActual, sortedExpected)
}

// validateServingInfoTLSConfig validates servingInfo TLS configuration in a parsed config object
// and cross-checks it against the expected TLS configuration.
// configObj is the parsed YAML/JSON config (map[string]interface{})
// servingInfoPath is the path to servingInfo (e.g., ["servingInfo"] or ["oauthServer", "servingInfo"])
func validateServingInfoTLSConfig(oc *exutil.CLI, ctx context.Context, configObj map[string]interface{}, servingInfoPath []string, expected tlsConfig) error {
	minTLSVersionPath := append(servingInfoPath, "minTLSVersion")
	minTLSVersion, found, err := unstructured.NestedString(configObj, minTLSVersionPath...)
	if err != nil || !found {
		return fmt.Errorf("field %s not found or not a string type: %w", toPath(minTLSVersionPath), err)
	}

	cipherSuitesPath := append(servingInfoPath, "cipherSuites")
	cipherSuites, found, err := unstructured.NestedStringSlice(configObj, cipherSuitesPath...)
	if err != nil || !found {
		return fmt.Errorf("field %s not found or not a string slice type: %w", toPath(cipherSuitesPath), err)
	}

	return validateTLSConfig(minTLSVersion, cipherSuites, expected)
}

// waitForAllOperatorsAfterTLSChange waits for all target ClusterOperators to
// stabilize (Available=True, Progressing=False, Degraded=False) and for all
// target Deployments to complete rollout after a TLS profile change.
func waitForAllOperatorsAfterTLSChange(oc *exutil.CLI, ctx context.Context, profileLabel string) {
	// Give operators time to observe the APIServer config change and begin
	// processing. Without this delay, operators may appear stable momentarily
	// because they haven't started their rollout yet.
	e2e.Logf("Waiting 30s for operators to begin processing %s profile change", profileLabel)
	time.Sleep(30 * time.Second)

	e2e.Logf("Waiting for all ClusterOperators to stabilize after %s profile change", profileLabel)
	for _, co := range clusterOperatorTargets {
		e2e.Logf("Waiting for ClusterOperator %s to stabilize after %s switch", co.name, profileLabel)
		waitForClusterOperatorStable(oc, ctx, co.name)
	}

	for _, t := range deploymentRolloutTargets {
		e2e.Logf("Waiting for deployment %s/%s to complete rollout after %s switch", t.namespace, t.deploymentName, profileLabel)
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(ctx, t.deploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, operatorRolloutTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("deployment %s/%s did not complete rollout after %s TLS change (timeout: %v)",
				t.namespace, t.deploymentName, profileLabel, operatorRolloutTimeout))
		e2e.Logf("Deployment %s/%s is fully rolled out after %s switch", t.namespace, t.deploymentName, profileLabel)
	}
	e2e.Logf("All operators and deployments are stable after %s profile change", profileLabel)
}

// ─── HyperShift helpers ────────────────────────────────────────────────────

// discoverHostedCluster finds the HostedCluster name and namespace on the
// management cluster that corresponds to the given hosted control plane
// namespace (hcpNS). The HCP namespace follows the convention {hcNS}-{hcName}.
func discoverHostedCluster(mgmtCLI *exutil.CLI, hcpNS string) (string, string) {
	output, err := mgmtCLI.AsAdmin().Run("get").Args(
		"hostedclusters", "-A",
		"-o", `jsonpath={range .items[*]}{.metadata.namespace},{.metadata.name}{"\n"}{end}`,
	).Output()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list HostedClusters on management cluster")

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, ",", 2)
		if len(parts) == 2 {
			ns, name := parts[0], parts[1]
			if ns+"-"+name == hcpNS {
				return name, ns
			}
		}
	}
	e2e.Failf("could not find HostedCluster matching HCP namespace %s", hcpNS)
	return "", ""
}

// setTLSProfileOnHyperShift patches the HostedCluster resource to change
// the TLS security profile via its .spec.configuration.apiServer field.
func setTLSProfileOnHyperShift(mgmtCLI *exutil.CLI, hcName, hcNS, patchJSON string) {
	err := mgmtCLI.AsAdmin().Run("patch").Args(
		"hostedcluster", hcName, "-n", hcNS,
		"--type=merge", "-p", patchJSON,
	).Execute()
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to patch HostedCluster TLS profile")
}

// waitForHCPPods waits for kube-apiserver, openshift-apiserver, and
// oauth-openshift pods in the hosted control plane namespace to become
// fully ready after a configuration change.
func waitForHCPPods(mgmtCLI *exutil.CLI, hcpNS string, timeout time.Duration) {
	for _, appLabel := range []string{"kube-apiserver", "openshift-apiserver", "oauth-openshift"} {
		e2e.Logf("Waiting for %s pods in HCP namespace %s", appLabel, hcpNS)
		err := waitForHCPAppReady(mgmtCLI, appLabel, hcpNS, timeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("HCP pods for %s did not become ready in %s within %v", appLabel, hcpNS, timeout))
		e2e.Logf("HCP %s pods are ready in %s", appLabel, hcpNS)
	}
}

// waitForHCPAppReady polls pods with label app=<appLabel> in the given
// namespace until all pods are running and ready. Follows the same pattern
// as waitApiserverRestartOfHypershift in openshift-tests-private.
func waitForHCPAppReady(mgmtCLI *exutil.CLI, appLabel, hcpNS string, timeout time.Duration) error {
	return wait.PollUntilContextTimeout(context.Background(), 10*time.Second, timeout, false,
		func(ctx context.Context) (bool, error) {
			out, err := mgmtCLI.AsAdmin().Run("get").Args(
				"pods", "-l", "app="+appLabel,
				"--no-headers", "-n", hcpNS,
			).Output()
			if err != nil {
				e2e.Logf("  poll: error listing %s pods: %v", appLabel, err)
				return false, nil
			}
			if out == "" {
				e2e.Logf("  poll: no %s pods found yet", appLabel)
				return false, nil
			}

			for _, indicator := range []string{"0/", "Pending", "Terminating", "Init"} {
				if strings.Contains(out, indicator) {
					e2e.Logf("  poll: %s pods still restarting (found %q)", appLabel, indicator)
					return false, nil
				}
			}

			// Recheck stability after a brief delay to avoid false positives.
			time.Sleep(10 * time.Second)
			out2, err := mgmtCLI.AsAdmin().Run("get").Args(
				"pods", "-l", "app="+appLabel,
				"--no-headers", "-n", hcpNS,
			).Output()
			if err != nil {
				return false, nil
			}
			for _, indicator := range []string{"0/", "Pending", "Terminating", "Init"} {
				if strings.Contains(out2, indicator) {
					e2e.Logf("  poll: %s pods still not stable on recheck", appLabel)
					return false, nil
				}
			}

			e2e.Logf("  poll: %s pods are ready in %s", appLabel, hcpNS)
			return true, nil
		})
}

// waitForGuestOperatorsAfterTLSChange waits for guest-side ClusterOperators
// and Deployments to stabilize after a TLS profile change on HyperShift.
func waitForGuestOperatorsAfterTLSChange(oc *exutil.CLI, ctx context.Context, profileLabel string, rollouts []deploymentRolloutTarget) {
	e2e.Logf("Waiting for guest-side ClusterOperators to stabilize after %s profile change", profileLabel)
	for _, co := range guestSideClusterOperatorTargets() {
		e2e.Logf("Waiting for ClusterOperator %s to stabilize after %s switch", co.name, profileLabel)
		waitForClusterOperatorStable(oc, ctx, co.name)
	}

	for _, t := range rollouts {
		e2e.Logf("Waiting for deployment %s/%s to complete rollout after %s switch", t.namespace, t.deploymentName, profileLabel)
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(ctx, t.deploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = waitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, operatorRolloutTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("deployment %s/%s did not complete rollout after %s TLS change",
				t.namespace, t.deploymentName, profileLabel))
		e2e.Logf("Deployment %s/%s is fully rolled out after %s switch", t.namespace, t.deploymentName, profileLabel)
	}
	e2e.Logf("All guest-side operators and deployments are stable after %s profile change", profileLabel)
}

// verifyHCPConfigMaps checks that ConfigMaps in the hosted control plane
// namespace contain the expected TLS version after a profile switch.
// Checks kas-config (kube-apiserver) and openshift-apiserver ConfigMaps.
func verifyHCPConfigMaps(mgmtCLI *exutil.CLI, hcpNS, expectedVersion, profileLabel string) {
	hcpCMs := []struct {
		name      string
		configKey string
	}{
		{name: "kas-config", configKey: `config\.json`},
		{name: "openshift-apiserver", configKey: `config\.yaml`},
	}

	for _, cm := range hcpCMs {
		out, err := mgmtCLI.AsAdmin().Run("get").Args(
			"cm", cm.name, "-n", hcpNS,
			"-o", fmt.Sprintf("jsonpath={.data.%s}", cm.configKey),
		).Output()
		if err != nil {
			e2e.Logf("SKIP: HCP ConfigMap %s/%s not found: %v", hcpNS, cm.name, err)
			continue
		}

		o.Expect(out).To(o.ContainSubstring(expectedVersion),
			fmt.Sprintf("HCP ConfigMap %s/%s should contain %s after %s switch",
				hcpNS, cm.name, expectedVersion, profileLabel))
		e2e.Logf("PASS: HCP ConfigMap %s/%s contains %s after %s switch",
			hcpNS, cm.name, expectedVersion, profileLabel)
	}
}

// waitForClusterOperatorStable waits until the named ClusterOperator reaches
// Available=True, Progressing=False, Degraded=False.
func waitForClusterOperatorStable(oc *exutil.CLI, ctx context.Context, name string) {
	e2e.Logf("Waiting for ClusterOperator %q to become stable", name)
	start := time.Now()

	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 25*time.Minute, true,
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
