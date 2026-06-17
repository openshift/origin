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
	"slices"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
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

	// hostedClusterConfigsNamespace is the namespace where HostedCluster CRs live in the management cluster.
	hostedClusterConfigsNamespace = "clusters"
)

// ─── Narrow target types ───────────────────────────────────────────────────
// Each type carries only the fields its test function actually reads,
// making it immediately clear what data a test depends on.

// tlsTarget is the common interface implemented by all TLS test target types.
type tlsTarget interface {
	testTLS(oc *exutil.CLI, ctx context.Context, expected tlsConfig) error
	key() string
}

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

// endpointTarget identifies a component endpoint that must enforce the
// cluster TLS profile at the wire level, tested via pod-based port-forward.
type endpointTarget struct {
	namespace      string            // Namespace where the pods live
	deploymentName string            // Deployment name to get pod selector from (empty for static pods)
	podSelector    map[string]string // Explicit pod selector (only when deploymentName is empty)
	ports          []string          // Container ports for TLS testing
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
	profileType      configv1.TLSProfileType // The profile type (Intermediate, Modern, Custom, etc.)
	minTLSVersion    string                  // e.g., "VersionTLS12", "VersionTLS13"
	cipherSuites     []string                // IANA cipher suite names
	tlsShouldWork    *tls.Config             // Wire-level TLS config that should succeed
	tlsShouldNotWork *tls.Config             // Wire-level TLS config that should fail
}

// tlsTestTargets consolidates all TLS test target lists into a single structure.
// This allows passing all targets together and makes it easier to define
// different target sets for different test scenarios.
type tlsTestTargets struct {
	observedConfig    []observedConfigTarget
	configMaps        []configMapTarget
	deploymentEnvVars []deploymentEnvVarTarget
	endpoints         []endpointTarget
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

var endpointTargets = []endpointTarget{
	newEndpointTarget("openshift-image-registry", "image-registry", nil, []string{"5000"}),
	newEndpointTarget("openshift-image-registry", "", map[string]string{"name": "cluster-image-registry-operator"}, []string{"60000"}),
	newEndpointTarget("openshift-controller-manager", "controller-manager", nil, []string{"8443"}),
	newEndpointTarget("openshift-kube-apiserver", "", map[string]string{"app": "openshift-kube-apiserver", "apiserver": "true"}, []string{"6443", "17697"}),
	newEndpointTarget("openshift-apiserver", "apiserver", nil, []string{"8443", "17698"}),
	newEndpointTarget("openshift-etcd", "", map[string]string{"app": "etcd", "etcd": "true"}, []string{"2379", "2381", "2380", "9978", "9979", "9980"}),
	newEndpointTarget("openshift-kube-controller-manager", "", map[string]string{"app": "kube-controller-manager", "kube-controller-manager": "true"}, []string{"10257", "10357"}),
	newEndpointTarget("openshift-kube-scheduler", "", map[string]string{"app": "openshift-kube-scheduler", "scheduler": "true"}, []string{"10259"}),
	newEndpointTarget("openshift-cluster-samples-operator", "cluster-samples-operator", nil, []string{"60000"}),
	newEndpointTarget("openshift-authentication-operator", "authentication-operator", nil, []string{"8443"}),
	newEndpointTarget("openshift-authentication", "oauth-openshift", nil, []string{"6443"}),
	newEndpointTarget("openshift-oauth-apiserver", "apiserver", nil, []string{"8443"}),
}

var hcpObservedConfigTargets = []observedConfigTarget{
	newObservedConfigTarget("clusters-XXX", gvr("imageregistry.operator.openshift.io", "v1", "configs"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("clusters-XXX", gvr("operator.openshift.io", "v1", "openshiftcontrollermanagers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("clusters-XXX", gvr("operator.openshift.io", "v1", "kubeapiservers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("clusters-XXX", gvr("operator.openshift.io", "v1", "openshiftapiservers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("clusters-XXX", gvr("operator.openshift.io", "v1", "etcds"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("clusters-XXX", gvr("operator.openshift.io", "v1", "kubecontrollermanagers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("clusters-XXX", gvr("operator.openshift.io", "v1", "kubeschedulers"), "cluster", []string{"servingInfo"}, true),
	newObservedConfigTarget("clusters-XXX", gvr("operator.openshift.io", "v1", "authentications"), "cluster", []string{"oauthServer", "servingInfo"}, true),
}

// commented out lines do not pass the check (yet)
var hcpEndpointTargets = []endpointTarget{
	// newEndpointTarget("clusters-XXX", "aws-ebs-csi-driver-controller", nil, []string{"10301", "9201", "9202", "9203", "9204", "9205"}),
	// newEndpointTarget("clusters-XXX", "capi-provider", nil, []string{"9440"}),
	newEndpointTarget("clusters-XXX", "catalog-operator", nil, []string{"8443"}),
	// newEndpointTarget("clusters-XXX", "certified-operators-catalog", nil, []string{"50051"}),
	// newEndpointTarget("clusters-XXX", "cluster-api", nil, []string{"9440"}),
	// newEndpointTarget("clusters-XXX", "cluster-autoscaler", nil, []string{"8085"}),
	newEndpointTarget("clusters-XXX", "cluster-image-registry-operator", nil, []string{"60000"}),
	newEndpointTarget("clusters-XXX", "cluster-node-tuning-operator", nil, []string{"60000"}),
	newEndpointTarget("clusters-XXX", "cluster-storage-operator", nil, []string{"8443"}),
	newEndpointTarget("clusters-XXX", "cluster-version-operator", nil, []string{"8443"}),
	// newEndpointTarget("clusters-XXX", "community-operators-catalog", nil, []string{"50051"}),
	// newEndpointTarget("clusters-XXX", "control-plane-operator", nil, []string{"8080"}),
	newEndpointTarget("clusters-XXX", "control-plane-pki-operator", nil, []string{"8443"}),
	// newEndpointTarget("clusters-XXX", "hosted-cluster-config-operator", nil, []string{"8080"}),
	// newEndpointTarget("clusters-XXX", "ignition-server", nil, []string{"8080", "9090"}),
	newEndpointTarget("clusters-XXX", "ignition-server-proxy", nil, []string{"8443"}),
	// newEndpointTarget("clusters-XXX", "ingress-operator", nil, []string{"60000"}),
	// newEndpointTarget("clusters-XXX", "konnectivity-agent", nil, []string{"2041", "8091"}),
	newEndpointTarget("clusters-XXX", "kube-apiserver", nil, []string{"6443"}),
	newEndpointTarget("clusters-XXX", "kube-controller-manager", nil, []string{"10257"}),
	newEndpointTarget("clusters-XXX", "kube-scheduler", nil, []string{"10259"}),
	// newEndpointTarget("clusters-XXX", "multus-admission-controller", nil, []string{"9091"}),
	// newEndpointTarget("clusters-XXX", "network-node-identity", nil, []string{"9743"}),
	newEndpointTarget("clusters-XXX", "oauth-openshift", nil, []string{"6443"}),
	newEndpointTarget("clusters-XXX", "olm-operator", nil, []string{"8443"}),
	newEndpointTarget("clusters-XXX", "openshift-apiserver", nil, []string{"8443"}),
	newEndpointTarget("clusters-XXX", "openshift-controller-manager", nil, []string{"8443"}),
	newEndpointTarget("clusters-XXX", "openshift-oauth-apiserver", nil, []string{"8443"}),
	newEndpointTarget("clusters-XXX", "openshift-route-controller-manager", nil, []string{"8443"}),
	newEndpointTarget("clusters-XXX", "ovnkube-control-plane", nil, []string{"9108"}),
	newEndpointTarget("clusters-XXX", "packageserver", nil, []string{"5443"}),
	// newEndpointTarget("clusters-XXX", "redhat-marketplace-catalog", nil, []string{"50051"}),
	// newEndpointTarget("clusters-XXX", "redhat-operators-catalog", nil, []string{"50051"}),
}

var guestClusterObservedConfigTargets = []observedConfigTarget{
	newObservedConfigTarget("openshift-image-registry", gvr("imageregistry.operator.openshift.io", "v1", "configs"), "cluster", []string{"servingInfo"}, false),
}

var guestClusterConfigMapTargets = []configMapTarget{
	newConfigMapTarget("openshift-image-registry", "image-registry-operator-config", "openshift-image-registry", "config.yaml", false),
	newConfigMapTarget("openshift-cluster-samples-operator", "samples-operator-config", "openshift-cluster-samples-operator", "config.yaml", false),
}

var guestClusterDeploymentEnvVarTargets = []deploymentEnvVarTarget{
	newDeploymentEnvVarTarget("openshift-image-registry", "image-registry", "REGISTRY_HTTP_TLS_MINVERSION", "OPENSHIFT_REGISTRY_HTTP_TLS_CIPHERSUITES", false),
}

var guestClusterEndpointTargets = []endpointTarget{
	newEndpointTarget("openshift-image-registry", "image-registry", nil, []string{"5000"}),
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
	endpoints:         endpointTargets,
}

var allHostedControlPlaneTargets = tlsTestTargets{
	// So far it seems the centralized TLS config is not getting propagated to .spec.observedConfig
	// observedConfig: hcpObservedConfigTargets,
	endpoints: hcpEndpointTargets,
}

var allGuestClusterTargets = tlsTestTargets{
	observedConfig:    guestClusterObservedConfigTargets,
	configMaps:        guestClusterConfigMapTargets,
	deploymentEnvVars: guestClusterDeploymentEnvVarTargets,
	endpoints:         guestClusterEndpointTargets,
}

// ── read-only tests ────────────────────────────────────────────
// These tests only read cluster state (ObservedConfig, ConfigMaps,
var _ = g.Describe("[sig-api-machinery][Feature:TLSObservedConfig][Serial][Suite:openshift/tls-observed-config]", g.Ordered, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-observed-config")
	ctx := context.Background()

	var mgmtOC *exutil.CLI
	var hcpNamespace string
	var hostedClusterConfigName string
	var isHyperShiftCluster bool

	g.BeforeAll(func() {
		var err error
		isHyperShiftCluster, err = exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		if isHyperShiftCluster {
			mgmtOC, hcpNamespace, hostedClusterConfigName, err = setupHyperShiftManagement()
			o.Expect(err).NotTo(o.HaveOccurred())
			// Set HCP namespace for all HCP targets
			for i := range hcpObservedConfigTargets {
				hcpObservedConfigTargets[i].namespace = hcpNamespace
			}
			// Initialize pod selectors for all endpoint targets
			for i := range hcpEndpointTargets {
				hcpEndpointTargets[i].namespace = hcpNamespace
				err := hcpEndpointTargets[i].detectPodSelector(mgmtOC, ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			for i := range guestClusterEndpointTargets {
				err := guestClusterEndpointTargets[i].detectPodSelector(mgmtOC, ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		} else {
			// Initialize pod selectors for all endpoint targets
			for i := range allTLSTestTargets.endpoints {
				err := allTLSTestTargets.endpoints[i].detectPodSelector(oc, ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})

	g.It("should verify TLS configuration across all components", func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("TLS observed-config tests are not applicable to MicroShift clusters")
		}

		if isHyperShiftCluster {
			g.By("reading current HostedCluster TLS profile")
			expectedTLSConfig, err := getHostedClusterTLSProfile(mgmtOC, ctx, hostedClusterConfigsNamespace, hostedClusterConfigName)
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Current HostedCluster TLS profile: %v", expectedTLSConfig.profileType)

			verifyAllTLSConfiguration(mgmtOC, ctx, isHyperShiftCluster, allHostedControlPlaneTargets, expectedTLSConfig)

			verifyAllTLSConfiguration(oc, ctx, isHyperShiftCluster, allGuestClusterTargets, expectedTLSConfig)
		} else {
			apiserverConfig, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			expectedTLSConfig := captureTLSConfiguration(apiserverConfig.Spec.TLSSecurityProfile)

			verifyAllTLSConfiguration(oc, ctx, isHyperShiftCluster, allTLSTestTargets, expectedTLSConfig)
		}
	})
})

// ── Serial disruptive tests ─────────────────────────────────────────────
// These tests modify cluster state (ConfigMap annotations, servingInfo,
// cluster-wide TLS profile) and must run serially.
var _ = g.Describe("[sig-api-machinery][Feature:TLSObservedConfig][Serial][Disruptive][Suite:openshift/tls-observed-config]", g.Ordered, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-observed-config-serial")
	ctx := context.Background()

	// HyperShift management cluster state, lazily populated by
	// setupHyperShiftManagement. Only config-change tests need this;
	// annotation/servingInfo restoration tests work without it.
	var mgmtOC *exutil.CLI
	var hcpNamespace string
	var hostedClusterConfigName string
	var isHyperShiftCluster bool

	g.BeforeAll(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("TLS observed-config tests are not applicable to MicroShift clusters")
		}

		isHyperShiftCluster, err = exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())

		if isHyperShiftCluster {
			mgmtOC, hcpNamespace, hostedClusterConfigName, err = setupHyperShiftManagement()
			o.Expect(err).NotTo(o.HaveOccurred())
			// Set HCP namespace for all HCP targets
			for i := range hcpObservedConfigTargets {
				hcpObservedConfigTargets[i].namespace = hcpNamespace
			}
			// Initialize pod selectors for all endpoint targets
			for i := range hcpEndpointTargets {
				hcpEndpointTargets[i].namespace = hcpNamespace
				err := hcpEndpointTargets[i].detectPodSelector(mgmtOC, ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			for i := range guestClusterEndpointTargets {
				err := guestClusterEndpointTargets[i].detectPodSelector(mgmtOC, ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		} else {
			// Initialize pod selectors for all endpoint targets
			for i := range allTLSTestTargets.endpoints {
				err := allTLSTestTargets.endpoints[i].detectPodSelector(oc, ctx)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
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

		if isHyperShiftCluster {
			// 1. Read current APIServer TLS profile and determine effective TLS configuration
			g.By("reading current HostedCluster TLS profile")
			currentTLSConfig, err := getHostedClusterTLSProfile(mgmtOC, ctx, hostedClusterConfigsNamespace, hostedClusterConfigName)
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("Current HostedCluster TLS profile: %v", currentTLSConfig.profileType)

			// 2. Generate a different TLS profile (different version and different ciphers)
			g.By("generating target TLS profile different from current")
			targetProfile, targetTLSConfig := generateDifferentTLSProfile(currentTLSConfig)
			e2e.Logf("Target TLS profile: type=%s, minTLSVersion=%s, ciphers=%v",
				targetTLSConfig.profileType, targetTLSConfig.minTLSVersion, targetTLSConfig.cipherSuites)

			// 3. Verify current effective config matches current profile
			g.By("verifying current effective TLS config matches current profile")
			verifyAllTLSConfiguration(mgmtOC, configChangeCtx, true, allHostedControlPlaneTargets, currentTLSConfig)
			verifyAllTLSConfiguration(oc, configChangeCtx, true, allGuestClusterTargets, currentTLSConfig)
			e2e.Logf("PASS: All targets verified - match current HostedCluster TLS profile")

			// 4. Set new TLS profile
			g.By("updating HostedCluster with new TLS profile")
			err = setHostedClusterTLSProfile(mgmtOC, configChangeCtx, hostedClusterConfigsNamespace, hostedClusterConfigName, targetProfile)
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("HostedCluster TLS profile updated to: %v", targetTLSConfig.profileType)

			// 5. Wait for reconciliation
			g.By("waiting for all targets to reconcile to new TLS configuration")
			err = waitForTLSReconciliation(mgmtOC, configChangeCtx, true, allHostedControlPlaneTargets, targetTLSConfig)
			o.Expect(err).NotTo(o.HaveOccurred(), "TLS reconciliation failed")
			err = waitForTLSReconciliation(oc, configChangeCtx, true, allGuestClusterTargets, targetTLSConfig)
			o.Expect(err).NotTo(o.HaveOccurred(), "TLS reconciliation failed")
			e2e.Logf("PASS: All targets reconciled to new TLS configuration")

			return
		}

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

	// ── ConfigMap annotation restoration tests ────────────────────────────

	g.It("should restore inject-tls annotation after deletion - all targets", func() {
		var errs []error
		for _, target := range configMapTargets {
			err := modifyAnnotation(oc, ctx, target, func(cm *corev1.ConfigMap) (string, error) {
				delete(cm.Annotations, injectTLSAnnotation)
				return "deleting " + injectTLSAnnotation + " annotation", nil
			})
			if err != nil {
				errs = append(errs, err)
			}
		}

		for _, target := range configMapTargets {
			err := waitForAnnotation(oc, ctx, target.configMapNamespace, target.configMapName, injectTLSAnnotation, "true")
			if err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			o.Expect(fmt.Errorf("encountered %d errors: %v", len(errs), errs)).NotTo(o.HaveOccurred())
		}
	})

	g.It("should restore inject-tls annotation when set to false - all targets", func() {
		var errs []error
		for _, target := range configMapTargets {
			err := modifyAnnotation(oc, ctx, target, func(cm *corev1.ConfigMap) (string, error) {
				cm.Annotations[injectTLSAnnotation] = "false"
				return "setting " + injectTLSAnnotation + " annotation to 'false'", nil
			})
			if err != nil {
				errs = append(errs, err)
			}
		}

		for _, target := range configMapTargets {
			err := waitForAnnotation(oc, ctx, target.configMapNamespace, target.configMapName, injectTLSAnnotation, "true")
			if err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			o.Expect(fmt.Errorf("encountered %d errors: %v", len(errs), errs)).NotTo(o.HaveOccurred())
		}
	})

	g.It("should restore servingInfo after removal - all targets", func() {
		var errs []error
		originalData := make(map[string]string)
		for _, target := range configMapTargets {
			original, err := removeServingInfo(oc, ctx, target)
			if err != nil {
				errs = append(errs, err)
			}
			originalData[target.namespace] = original
		}

		for _, target := range configMapTargets {
			err := waitForServingInfoRestoration(oc, ctx, target, originalData[target.namespace])
			if err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			o.Expect(fmt.Errorf("encountered %d errors: %v", len(errs), errs)).NotTo(o.HaveOccurred())
		}
	})

	g.It("should restore servingInfo after modification - all targets", func() {
		var errs []error
		originalData := make(map[string]string)
		for _, target := range configMapTargets {
			original, err := modifyMinTLSVersion(oc, ctx, target)
			if err != nil {
				errs = append(errs, err)
			}
			originalData[target.namespace] = original
		}

		for _, target := range configMapTargets {
			err := waitForServingInfoRestoration(oc, ctx, target, originalData[target.namespace])
			if err != nil {
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			o.Expect(fmt.Errorf("encountered %d errors: %v", len(errs), errs)).NotTo(o.HaveOccurred())
		}
	})
})

// ─── Test implementations ──────────────────────────────────────────────────

// verifyAllTLSConfiguration runs all TLS validation tests across all components
// and reports any failures. This can be called multiple times (e.g., after TLS
// profile changes) to verify the configuration.
func verifyAllTLSConfiguration(oc *exutil.CLI, ctx context.Context, isHyperShiftCluster bool, targets tlsTestTargets, expectedTLSConfig tlsConfig) {
	state := newValidationState()

	// Run validation once
	validateAllTargetsOnce(oc, ctx, isHyperShiftCluster, targets, expectedTLSConfig, state)

	// Collect all errors (keys already include target type prefix)
	errors := make(map[string]error)
	for key, err := range state.targets {
		if err != nil {
			errors[key] = err
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

// testTLS verifies that the operator's ObservedConfig contains
// a properly populated servingInfo with minTLSVersion and cipherSuites.
// This validates that the config observer controller (from library-go) is
// correctly watching the APIServer resource and writing the TLS config
// into the operator's ObservedConfig.
func (t observedConfigTarget) testTLS(oc *exutil.CLI, ctx context.Context, expected tlsConfig) error {
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

func (t observedConfigTarget) key() string {
	return fmt.Sprintf("observedConfig:%s/%s/%s", t.operatorConfigGVR.Resource, t.namespace, t.operatorConfigName)
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

// updateConfigMap writes the ConfigMap back to the API server,
// retrying on conflict to handle concurrent controller reconciliation.
func updateConfigMap(oc *exutil.CLI, ctx context.Context, cm *corev1.ConfigMap) error {
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
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap %s/%s: %w", cm.Namespace, cm.Name, err)
	}
	return nil
}

// waitForConfigMapCondition polls until the given check function returns true.
// The check function receives the ConfigMap and should return true when the condition is met.
func waitForConfigMapCondition(oc *exutil.CLI, ctx context.Context, namespace, name, description string, check func(*corev1.ConfigMap) bool) error {
	g.By(description)
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}
			return check(cm), nil
		},
	)
	if err != nil {
		return fmt.Errorf("condition not met on ConfigMap %s/%s within timeout: %w", namespace, name, err)
	}
	return nil
}

// waitForAnnotation polls until the given annotation reaches the expected value.
func waitForAnnotation(oc *exutil.CLI, ctx context.Context, namespace, name, annotationKey, annotationValue string) error {
	return waitForConfigMapCondition(oc, ctx, namespace, name,
		fmt.Sprintf("waiting for CVO to restore %s annotation to %q on ConfigMap %s/%s", annotationKey, annotationValue, namespace, name),
		func(cm *corev1.ConfigMap) bool {
			val, found := cm.Annotations[annotationKey]
			if found && val == annotationValue {
				e2e.Logf("  poll: annotation %s restored to %q", annotationKey, annotationValue)
				return true
			}
			e2e.Logf("  poll: annotation not yet restored (found=%v, val=%s)", found, val)
			return false
		},
	)
}

// testTLS verifies that CVO has injected TLS configuration
// into the operator's ConfigMap via the config.openshift.io/inject-tls annotation.
// This validates that CVO is reading the APIServer TLS profile and injecting
// the minTLSVersion and cipherSuites into the ConfigMap's servingInfo section.
func (t configMapTarget) testTLS(oc *exutil.CLI, ctx context.Context, expected tlsConfig) error {
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

func (t configMapTarget) key() string {
	return fmt.Sprintf("configMap:%s/%s", t.configMapNamespace, t.configMapName)
}

// modifyAnnotation modifies a ConfigMap annotation without waiting for restoration.
func modifyAnnotation(
	oc *exutil.CLI,
	ctx context.Context,
	t configMapTarget,
	modify func(cm *corev1.ConfigMap) (actionDescription string, err error),
) error {
	cm := getConfigMap(oc, ctx, t.configMapNamespace, t.configMapName)
	if _, found := cm.Annotations[injectTLSAnnotation]; !found {
		return fmt.Errorf("ConfigMap %s/%s is missing %s annotation", cm.Namespace, cm.Name, injectTLSAnnotation)
	}

	actionDescription, err := modify(cm)
	if err != nil {
		return err
	}

	if err = updateConfigMap(oc, ctx, cm); err != nil {
		return err
	}
	e2e.Logf("%s on ConfigMap %s/%s", actionDescription, t.configMapNamespace, t.configMapName)
	return nil
}

// removeServingInfo removes the servingInfo section from a ConfigMap and returns the original data.
func removeServingInfo(oc *exutil.CLI, ctx context.Context, t configMapTarget) (string, error) {
	cm := getConfigMap(oc, ctx, t.configMapNamespace, t.configMapName)
	configData := cm.Data[t.configMapKey]
	originalConfigData := configData

	node, err := kyaml.Parse(configData)
	if err != nil {
		return "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	servingInfoNode, err := node.Pipe(kyaml.Lookup("servingInfo"))
	if err != nil || servingInfoNode == nil {
		return originalConfigData, nil
	}

	err = node.PipeE(kyaml.Clear("servingInfo"))
	if err != nil {
		return "", fmt.Errorf("failed to remove servingInfo: %w", err)
	}

	modifiedData, err := node.String()
	if err != nil {
		return "", fmt.Errorf("failed to serialize YAML: %w", err)
	}

	cm.Data[t.configMapKey] = modifiedData
	if err = updateConfigMap(oc, ctx, cm); err != nil {
		return "", err
	}
	e2e.Logf("Removed servingInfo from ConfigMap %s/%s", t.configMapNamespace, t.configMapName)

	return originalConfigData, nil
}

// modifyMinTLSVersion modifies the minTLSVersion to a wrong value and returns the original data.
func modifyMinTLSVersion(oc *exutil.CLI, ctx context.Context, t configMapTarget) (string, error) {
	cm := getConfigMap(oc, ctx, t.configMapNamespace, t.configMapName)
	configData := cm.Data[t.configMapKey]
	originalConfigData := configData

	node, err := kyaml.Parse(configData)
	if err != nil {
		return "", fmt.Errorf("failed to parse YAML: %w", err)
	}

	minTLSNode, err := node.Pipe(kyaml.Lookup("servingInfo", "minTLSVersion"))
	if err != nil || minTLSNode == nil {
		return originalConfigData, nil
	}

	currentValue := minTLSNode.YNode().Value
	wrongValue := "VersionTLS10"
	if strings.Contains(currentValue, "VersionTLS10") {
		wrongValue = "VersionTLS99"
	}

	err = node.PipeE(kyaml.LookupCreate(kyaml.ScalarNode, "servingInfo", "minTLSVersion"), kyaml.FieldSetter{StringValue: wrongValue})
	if err != nil {
		return "", fmt.Errorf("failed to modify minTLSVersion: %w", err)
	}

	modifiedData, err := node.String()
	if err != nil {
		return "", fmt.Errorf("failed to serialize YAML: %w", err)
	}

	cm.Data[t.configMapKey] = modifiedData
	if err = updateConfigMap(oc, ctx, cm); err != nil {
		return "", err
	}
	e2e.Logf("Modified minTLSVersion to '%s' on ConfigMap %s/%s", wrongValue, t.configMapNamespace, t.configMapName)

	return originalConfigData, nil
}

// waitForServingInfoRestoration waits for the operator to restore servingInfo to the original state.
func waitForServingInfoRestoration(oc *exutil.CLI, ctx context.Context, t configMapTarget, originalConfigData string) error {
	return waitForConfigMapCondition(oc, ctx, t.configMapNamespace, t.configMapName,
		fmt.Sprintf("waiting for operator to restore servingInfo section on ConfigMap %s/%s", t.configMapNamespace, t.configMapName),
		func(cm *corev1.ConfigMap) bool {
			currentConfigData := cm.Data[t.configMapKey]
			if currentConfigData == originalConfigData {
				e2e.Logf("  poll: servingInfo restored!")
				return true
			}
			e2e.Logf("  poll: servingInfo not yet restored")
			return false
		},
	)
}

// testTLS verifies that the deployment in the given namespace
// has TLS environment variables that match the expected TLS profile.
func (t deploymentEnvVarTarget) testTLS(oc *exutil.CLI, ctx context.Context, expected tlsConfig) error {
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

func (t deploymentEnvVarTarget) key() string {
	return fmt.Sprintf("deploymentEnvVar:%s/%s", t.namespace, t.deploymentName)
}

// detectPodSelector detects and sets the pod selector for this endpoint.
// If podSelector is already set, this is a no-op (idempotent).
// If deploymentName is set, it reads the deployment and sets podSelector from its match labels.
// Returns an error if the deployment cannot be read.
func (t *endpointTarget) detectPodSelector(oc *exutil.CLI, ctx context.Context) error {
	// Already detected, nothing to do
	if len(t.podSelector) > 0 {
		return nil
	}

	// podSelector was explicitly set in constructor, nothing to do
	if t.deploymentName == "" {
		return nil
	}

	// Fetch from deployment
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(ctx, t.deploymentName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment %s/%s: %w", t.namespace, t.deploymentName, err)
	}
	t.podSelector = deployment.Spec.Selector.MatchLabels
	return nil
}

func (t *endpointTarget) testTLS(oc *exutil.CLI, ctx context.Context, expected tlsConfig) error {
	// podSelector must be set before calling testTLS (via detectPodSelector)
	if len(t.podSelector) == 0 {
		return fmt.Errorf("podSelector not initialized for endpoint %s/%s - detectPodSelector must be called first", t.namespace, t.deploymentName)
	}

	// Wait for deployment readiness if deploymentName is set
	var expectedReplicas int32
	if t.deploymentName != "" {
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(ctx, t.deploymentName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get deployment: %w", err)
		}
		if err := waitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, 2*time.Minute); err != nil {
			return fmt.Errorf("deployment not ready: %w", err)
		}
		if deployment.Spec.Replicas != nil {
			expectedReplicas = *deployment.Spec.Replicas
		}
	}

	// Convert pod selector map to label selector string
	selectorString := labels.Set(t.podSelector).String()

	e2e.Logf("Testing TLS on pods in namespace %s with selector %s, ports %v", t.namespace, selectorString, t.ports)

	// List pods matching the selector
	podList, err := oc.AdminKubeClient().CoreV1().Pods(t.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selectorString,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods with selector %s: %w", selectorString, err)
	}

	// Filter to running pods
	var runningPods []string
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods = append(runningPods, pod.Name)
		}
	}

	if len(runningPods) == 0 {
		return fmt.Errorf("no running pods found in namespace %s with selector %s", t.namespace, selectorString)
	}

	// Verify pod count matches expected replicas if deploymentName is set
	if t.deploymentName != "" && int32(len(runningPods)) != expectedReplicas {
		return fmt.Errorf("expected %d running pods for deployment %s/%s, but found %d",
			expectedReplicas, t.namespace, t.deploymentName, len(runningPods))
	}

	e2e.Logf("Found %d running pod(s): %v", len(runningPods), runningPods)

	// Test TLS on each pod and each port
	var errors []error
	for _, podName := range runningPods {
		for _, port := range t.ports {
			e2e.Logf("Testing TLS on pod %s/%s port %s", t.namespace, podName, port)
			resourceName := fmt.Sprintf("pod/%s", podName)
			componentName := fmt.Sprintf("pod/%s", podName)
			err := forwardPortAndExecute(oc, resourceName, t.namespace, port,
				func(localPort int) error {
					return checkTLSConnection(localPort, expected.tlsShouldWork, expected.tlsShouldNotWork, componentName, t.namespace)
				},
			)
			if err != nil {
				errors = append(errors, fmt.Errorf("pod %s port %s: %w", podName, port, err))
			}
		}
	}

	if len(errors) > 0 {
		var errMsgs []string
		for _, err := range errors {
			errMsgs = append(errMsgs, err.Error())
		}
		return fmt.Errorf("wire-level TLS test failed for %s in %s: %s",
			t.deploymentName, t.namespace, strings.Join(errMsgs, "; "))
	}

	return nil
}

func (t *endpointTarget) key() string {
	if t.deploymentName != "" {
		return fmt.Sprintf("endpoint:%s/%s", t.namespace, t.deploymentName)
	}
	return fmt.Sprintf("endpoint:%s/static-pod", t.namespace)
}

// ─── Helper functions ──────────────────────────────────────────────────────

// captureTLSConfiguration extracts the effective TLS configuration from a
// TLSSecurityProfile (profile type, minTLSVersion, cipherSuites).
// getHostedClusterTLSProfile retrieves the TLS security profile from a HostedCluster CR
// and returns the captured TLS configuration.
func getHostedClusterTLSProfile(mgmtOC *exutil.CLI, ctx context.Context, hostedClusterNS, hostedClusterConfigName string) (tlsConfig, error) {
	hostedClusterGVR := gvr("hypershift.openshift.io", "v1beta1", "hostedclusters")
	hostedClusterObj, err := mgmtOC.AdminDynamicClient().Resource(hostedClusterGVR).Namespace(hostedClusterNS).Get(ctx, hostedClusterConfigName, metav1.GetOptions{})
	if err != nil {
		return tlsConfig{}, fmt.Errorf("failed to get HostedCluster %s/%s: %w", hostedClusterNS, hostedClusterConfigName, err)
	}

	tlsProfileMap, found, err := unstructured.NestedMap(hostedClusterObj.Object, "spec", "configuration", "apiServer", "tlsSecurityProfile")
	if err != nil {
		return tlsConfig{}, fmt.Errorf("failed to extract tlsSecurityProfile from HostedCluster: %w", err)
	}

	var tlsProfile *configv1.TLSSecurityProfile
	if found && tlsProfileMap != nil {
		tlsProfile = &configv1.TLSSecurityProfile{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(tlsProfileMap, tlsProfile)
		if err != nil {
			return tlsConfig{}, fmt.Errorf("failed to convert tlsSecurityProfile to typed struct: %w", err)
		}
	}

	return captureTLSConfiguration(tlsProfile), nil
}

// setHostedClusterTLSProfile updates the TLS security profile on a HostedCluster CR.
func setHostedClusterTLSProfile(mgmtOC *exutil.CLI, ctx context.Context, hostedClusterNS, hostedClusterConfigName string, profile *configv1.TLSSecurityProfile) error {
	hostedClusterGVR := gvr("hypershift.openshift.io", "v1beta1", "hostedclusters")
	hostedClusterObj, err := mgmtOC.AdminDynamicClient().Resource(hostedClusterGVR).Namespace(hostedClusterNS).Get(ctx, hostedClusterConfigName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get HostedCluster %s/%s: %w", hostedClusterNS, hostedClusterConfigName, err)
	}

	// Convert the typed TLSSecurityProfile to unstructured map
	var profileMap map[string]interface{}
	if profile != nil {
		profileMap, err = runtime.DefaultUnstructuredConverter.ToUnstructured(profile)
		if err != nil {
			return fmt.Errorf("failed to convert TLSSecurityProfile to unstructured: %w", err)
		}
	}

	// Set the TLS profile at spec.configuration.apiServer.tlsSecurityProfile
	err = unstructured.SetNestedMap(hostedClusterObj.Object, profileMap, "spec", "configuration", "apiServer", "tlsSecurityProfile")
	if err != nil {
		return fmt.Errorf("failed to set tlsSecurityProfile in HostedCluster: %w", err)
	}

	// Update the HostedCluster CR
	_, err = mgmtOC.AdminDynamicClient().Resource(hostedClusterGVR).Namespace(hostedClusterNS).Update(ctx, hostedClusterObj, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update HostedCluster %s/%s: %w", hostedClusterNS, hostedClusterConfigName, err)
	}

	return nil
}

func captureTLSConfiguration(profile *configv1.TLSSecurityProfile) tlsConfig {
	// Determine profile type, defaulting to crypto.DefaultTLSProfileType if nil
	profileType := crypto.DefaultTLSProfileType
	if profile != nil {
		profileType = profile.Type
	}

	// Get effective minTLSVersion and cipherSuites
	minTLSVersion, cipherSuites := getSecurityProfileCiphers(profile)

	// Create wire-level TLS configs for testing
	var tlsShouldWork, tlsShouldNotWork *tls.Config
	minTLSVersionUint := tlsVersionStringToUint16(configv1.TLSProtocolVersion(minTLSVersion))

	switch minTLSVersionUint {
	case tls.VersionTLS11:
		tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS11, InsecureSkipVerify: true}
		tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS10, InsecureSkipVerify: true}
	case tls.VersionTLS12:
		tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true}
		tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS11, MaxVersion: tls.VersionTLS11, InsecureSkipVerify: true}
	case tls.VersionTLS13:
		tlsShouldWork = &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
		tlsShouldNotWork = &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
	}

	return tlsConfig{
		profileType:      profileType,
		minTLSVersion:    minTLSVersion,
		cipherSuites:     cipherSuites,
		tlsShouldWork:    tlsShouldWork,
		tlsShouldNotWork: tlsShouldNotWork,
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

// verifyObservedConfigAfterSwitch checks that every target with an operator
// validationState tracks validation results for all targets.
// The map stores errors (nil = reconciled/passed, non-nil = failed).
// Keys are generated by each target's key() method and include the target type prefix.
type validationState struct {
	targets map[string]error
}

// newValidationState creates an initialized validation state.
func newValidationState() *validationState {
	return &validationState{
		targets: make(map[string]error),
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
) (reconciledCount, totalCount int) {
	// Build a single list of all targets
	allTargets := make([]tlsTarget, 0)
	for _, t := range targets.observedConfig {
		allTargets = append(allTargets, t)
	}
	for _, t := range targets.configMaps {
		allTargets = append(allTargets, t)
	}
	for _, t := range targets.deploymentEnvVars {
		allTargets = append(allTargets, t)
	}
	for i := range targets.endpoints {
		allTargets = append(allTargets, &targets.endpoints[i])
	}

	// Single consolidated cycle through all targets
	for _, target := range allTargets {
		totalCount++

		key := target.key()
		if err, checked := state.targets[key]; checked && err == nil {
			// Already reconciled successfully, skip
			reconciledCount++
			continue
		}

		err := target.testTLS(oc, ctx, expectedTLSConfig)
		state.targets[key] = err
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

	err := wait.PollUntilContextTimeout(ctx, pollingInterval, timeout, true,
		func(ctx context.Context) (bool, error) {
			reconciledCount, totalCount := validateAllTargetsOnce(
				oc, ctx, isHyperShiftCluster, targets, expectedTLSConfig, state,
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

		// Keys already include target type prefix (observedConfig:, configMap:, etc.)
		for key, err := range state.targets {
			if err != nil {
				notReconciled = append(notReconciled, fmt.Sprintf("%s: %v", key, err))
			}
		}

		return fmt.Errorf("TLS reconciliation timeout after %v. Objects not reconciled:\n%s", timeout, strings.Join(notReconciled, "\n"))
	}

	e2e.Logf("PASS: All TLS targets reconciled in %v", time.Since(startTime).Round(time.Second))
	return nil
}

// forwardPortAndExecute sets up oc port-forward to a resource (service or pod) and executes
// the given test function with the local port. Retries up to 5 times with
// exponential backoff (2s, 4s, 8s, 16s) to handle pods restarting after config changes.
// resourceName should be in the format "svc/name" or "pod/name".
func forwardPortAndExecute(oc *exutil.CLI, resourceName, namespace, remotePort string, toExecute func(localPort int) error) error {
	const maxAttempts = 5
	var err error
	backoff := 2 * time.Second
	for i := 0; i < maxAttempts; i++ {
		if err = func() error {
			localPort := rand.Intn(65534-1025) + 1025

			cmd, stdout, stderr, err := oc.AsAdmin().Run("port-forward").Args(
				resourceName,
				fmt.Sprintf("%d:%s", localPort, remotePort),
				"-n", namespace,
			).Background()
			if err != nil {
				return fmt.Errorf("failed to start port-forward: %v", err)
			}
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
				e2e.Logf("%s is not ready, waiting %v before retry", resourceName, backoff)
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
// componentName should identify the component being tested (e.g., "svc/image-registry" or "pod/kube-apiserver-xyz").
func checkTLSConnection(localPort int, shouldWork, shouldNotWork *tls.Config, componentName, namespace string) error {
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
			return fmt.Errorf("%s in %s [%s]: Connection with %s FAILED (expected success): %w",
				componentName, namespace, hostType, expectedMinVersion, err)
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
			return fmt.Errorf("%s in %s [%s]: Connection with max %s should be REJECTED but succeeded (negotiated %s)",
				componentName, namespace, hostType, rejectedMaxVersion, tlsVersionName(negotiatedBad))
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
			return fmt.Errorf("%s in %s [%s]: Expected TLS version rejection error, got: %w",
				componentName, namespace, hostType, err)
		}
		e2e.Logf("[%s] %s: REJECTED - %s correctly refused by server",
			hostType, host, rejectedMaxVersion)

		testedHosts = append(testedHosts, fmt.Sprintf("%s(%s)", hostType, host))
	}

	if len(testedHosts) == 0 {
		return fmt.Errorf("%s in %s: No hosts available for testing (tried IPv4 and IPv6)",
			componentName, namespace)
	}

	e2e.Logf("%s in %s: ✓ TLS PASS - Verified on %d host(s): %v | Accepts: %s+ | Rejects: %s",
		componentName, namespace, len(testedHosts), testedHosts, expectedMinVersion, rejectedMaxVersion)
	return nil
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

// ─── HyperShift helpers ────────────────────────────────────────────────────

// setupHyperShiftManagement initializes the management cluster CLI and retrieves
// HyperShift cluster information. Returns mgmtOC, hcpNamespace, hostedClusterConfigName.
func setupHyperShiftManagement() (*exutil.CLI, string, string, error) {
	if os.Getenv("HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG") == "" || os.Getenv("HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE") == "" {
		return nil, "", "", fmt.Errorf("HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG and HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE must be set")
	}

	mgmtOC := exutil.NewHypershiftManagementCLI("tls-mgmt")
	_, hcpNamespace, err := exutil.GetHypershiftManagementClusterConfigAndNamespace()
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to get HyperShift management cluster config: %w", err)
	}

	hostedClusterConfigName := strings.TrimPrefix(hcpNamespace, hostedClusterConfigsNamespace+"-")
	e2e.Logf("HyperShift: HC=%s/%s, HCP NS=%s", hostedClusterConfigsNamespace, hostedClusterConfigName, hcpNamespace)

	return mgmtOC, hcpNamespace, hostedClusterConfigName, nil
}
