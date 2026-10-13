package tls

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os/exec"
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
	exutil "github.com/openshift/origin/test/extended/util"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientset "k8s.io/client-go/kubernetes"
)

const (
	// operatorRolloutTimeout is the maximum time to wait for an operator
	// workload (Deployment or static pod) to complete rollout after a TLS
	// profile change. KAS (static pod) rollout typically takes 15-20 minutes;
	// Deployment-based operators are usually faster. 25 minutes covers both.
	operatorRolloutTimeout = 25 * time.Minute
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
	// If empty, stability check is skipped.
	clusterOperatorName string
	// configMapName is the name of the ConfigMap that CVO injects TLS config into
	// via the config.openshift.io/inject-tls annotation.
	// If empty, the ConfigMap check is skipped.
	configMapName string
	// configMapNamespace is the namespace where the ConfigMap is located.
	// If empty, defaults to the target's namespace field.
	configMapNamespace string
	// configMapKey is the key within the ConfigMap that contains the TLS config
	// (typically "config.yaml"). If empty, defaults to "config.yaml".
	configMapKey string
	// controlPlane indicates this target runs in the control plane. On
	// HyperShift (external control plane topology), these workloads run on the
	// management cluster and are not accessible from the hosted guest cluster.
	// Tests for control-plane targets are skipped on HyperShift.
	controlPlane bool
}

// targets is the unified list of OpenShift namespaces and services that should
// propagate the cluster APIServer TLS profile.  Each entry can participate in
// multiple test categories (ObservedConfig, ConfigMap injection, env vars,
// wire-level TLS) depending on which fields are populated.  The test loops
// filter by checking for non-empty fields, so secondary entries (e.g. an
// extra port on the same operator) can set only serviceName/servicePort for
// wire-level coverage while leaving operatorConfigGVR/configMapName empty to
// avoid duplicate checks already handled by the primary entry.
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
		// CVO injects TLS config into this ConfigMap via config.openshift.io/inject-tls annotation.
		// PR 1297 (cluster-image-registry-operator) adds this annotation.
		configMapName: "image-registry-operator-config",
		configMapKey:  "config.yaml",
	},
	// image-registry-operator metrics service on port 60000.
	// PR 1297 (cluster-image-registry-operator, IR-350) makes the metrics
	// server TLS configuration file-based, complying with global TLS profile.
	{
		namespace:      "openshift-image-registry",
		deploymentName: "", // Operator deployment, not image-registry deployment
		// No TLS env vars — metrics server reads TLS from config files.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "image-registry-operator",
		servicePort:         "60000",
		// ObservedConfig and ConfigMap are already verified by the primary
		// image-registry entry above; this entry only adds wire-level TLS
		// coverage for the operator metrics port.
		operatorConfigGVR:   schema.GroupVersionResource{},
		operatorConfigName:  "",
		clusterOperatorName: "image-registry",
		configMapName:       "",
		configMapKey:        "",
		controlPlane:        true,
	},
	// openshift-controller-manager propagates TLS config via ConfigMap
	// (ObservedConfig → config.yaml), NOT via env vars. So we skip the
	// env-var check but still verify ObservedConfig and wire-level TLS.
	// PR 412 (cluster-openshift-controller-manager-operator) adds inject-tls annotation.
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
		// CVO injects TLS config into this ConfigMap (in the operator namespace).
		configMapName:      "openshift-controller-manager-operator-config",
		configMapNamespace: "openshift-controller-manager-operator",
		configMapKey:       "config.yaml",
		controlPlane:       true,
	},
	// kube-apiserver is a static pod managed by cluster-kube-apiserver-operator.
	// PR 2032/2059 added TLS security profile propagation to its ObservedConfig.
	// It reads TLS config from its config files, not env vars.
	{
		namespace:      "openshift-kube-apiserver",
		deploymentName: "", // Static pod, not a deployment
		// No TLS env vars — kube-apiserver reads TLS from its config files.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "apiserver",
		servicePort:         "443",
		operatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "kubeapiservers",
		},
		operatorConfigName:  "cluster",
		clusterOperatorName: "kube-apiserver",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		configMapName:      "kube-apiserver-operator-config",
		configMapNamespace: "openshift-kube-apiserver-operator",
		configMapKey:       "config.yaml",
		controlPlane:       true,
	},
	// kube-apiserver's check-endpoints port (17697) on the apiserver service.
	// PR 2032 (cluster-kube-apiserver-operator) ensures this port complies
	// with the global TLS security profile.
	{
		namespace:      "openshift-kube-apiserver",
		deploymentName: "", // Static pod, not a deployment
		// No TLS env vars — kube-apiserver reads TLS from config files.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "apiserver",
		servicePort:         "17697",
		// ObservedConfig and ConfigMap are already verified by the primary
		// kube-apiserver:443 entry above; this entry only adds wire-level
		// TLS coverage for the check-endpoints port.
		operatorConfigGVR:   schema.GroupVersionResource{},
		operatorConfigName:  "",
		clusterOperatorName: "kube-apiserver",
		controlPlane:        true,
	},
	// openshift-apiserver main API endpoint.
	// PR 662 (cluster-openshift-apiserver-operator) adds inject-tls annotation.
	{
		namespace:      "openshift-apiserver",
		deploymentName: "apiserver",
		// No TLS env vars — apiserver reads TLS from config files.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "api",
		servicePort:         "443",
		operatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "openshiftapiservers",
		},
		operatorConfigName:  "cluster",
		clusterOperatorName: "openshift-apiserver",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		configMapName:      "openshift-apiserver-operator-config",
		configMapNamespace: "openshift-apiserver-operator",
		configMapKey:       "config.yaml",
		controlPlane:       true,
	},
	// openshift-apiserver's check-endpoints service on port 17698.
	// PR 657 (cluster-openshift-apiserver-operator, CNTRLPLANE-2619) ensures
	// this port complies with the global TLS security profile.
	{
		namespace:      "openshift-apiserver",
		deploymentName: "", // check-endpoints uses same deployment
		// No TLS env vars — reads TLS from config files.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "check-endpoints",
		servicePort:         "17698",
		// ObservedConfig and ConfigMap are already verified by the primary
		// openshift-apiserver:443 entry above; this entry only adds
		// wire-level TLS coverage for the check-endpoints port.
		operatorConfigGVR:   schema.GroupVersionResource{},
		operatorConfigName:  "",
		clusterOperatorName: "openshift-apiserver",
		controlPlane:        true,
	},
	// cluster-version-operator (CVO).
	// PR 1322 enables CVO to INJECT TLS config into OTHER operators' ConfigMaps
	// (those annotated with config.openshift.io/inject-tls: "true").
	// NOTE: CVO's own metrics endpoint (port 9099) does NOT currently respect
	// the cluster-wide TLS profile - it always accepts TLS 1.2. This is expected
	// behavior for now; the PR scope is ConfigMap injection, not CVO's own endpoint.
	// Therefore we skip wire-level TLS tests for CVO (serviceName is empty).
	{
		namespace:      "openshift-cluster-version",
		deploymentName: "cluster-version-operator",
		// No TLS env vars — CVO reads TLS from config files.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		// Skip wire-level TLS test: CVO metrics endpoint doesn't follow cluster TLS profile.
		serviceName:        "",
		servicePort:        "",
		operatorConfigGVR:  schema.GroupVersionResource{}, // CVO manages itself
		operatorConfigName: "",
		// CVO does not have a ClusterOperator for itself - it manages all other operators.
		// Skip stability check; deployment rollout wait is sufficient.
		clusterOperatorName: "",
		// CVO does not use a ConfigMap with inject-tls annotation.
		// It reads TLS config directly from the cluster config.
		configMapName: "",
		configMapKey:  "",
	},
	// etcd is a static pod managed by cluster-etcd-operator.
	// PR 1556 (cluster-etcd-operator) adds TLS security profile propagation.
	{
		namespace:      "openshift-etcd",
		deploymentName: "", // Static pod, not a deployment
		// No TLS env vars — etcd reads TLS from its config files.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "etcd",
		servicePort:         "2379",
		operatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "etcds",
		},
		operatorConfigName:  "cluster",
		clusterOperatorName: "etcd",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		configMapName:      "etcd-operator-config",
		configMapNamespace: "openshift-etcd-operator",
		configMapKey:       "config.yaml",
		controlPlane:       true,
	},
	// kube-controller-manager is a static pod managed by cluster-kube-controller-manager-operator.
	// PR 915 (cluster-kube-controller-manager-operator) adds TLS security profile propagation.
	{
		namespace:      "openshift-kube-controller-manager",
		deploymentName: "", // Static pod, not a deployment
		// No TLS env vars — kube-controller-manager reads TLS from its config files.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "kube-controller-manager",
		servicePort:         "443",
		operatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "kubecontrollermanagers",
		},
		operatorConfigName:  "cluster",
		clusterOperatorName: "kube-controller-manager",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		configMapName:      "kube-controller-manager-operator-config",
		configMapNamespace: "openshift-kube-controller-manager-operator",
		configMapKey:       "config.yaml",
		controlPlane:       true,
	},
	// kube-scheduler is a static pod managed by cluster-kube-scheduler-operator.
	// PR 617 (cluster-kube-scheduler-operator) adds TLS security profile propagation.
	{
		namespace:      "openshift-kube-scheduler",
		deploymentName: "", // Static pod, not a deployment
		// No TLS env vars — kube-scheduler reads TLS from its config files.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "scheduler",
		servicePort:         "443",
		operatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "kubeschedulers",
		},
		operatorConfigName:  "cluster",
		clusterOperatorName: "kube-scheduler",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		configMapName:      "openshift-kube-scheduler-operator-config",
		configMapNamespace: "openshift-kube-scheduler-operator",
		configMapKey:       "config.yaml",
		controlPlane:       true,
	},
	// cluster-samples-operator metrics service on port 60000.
	// PR 684 (cluster-samples-operator, CNTRLPLANE-3176) migrates the metrics
	// server to config-driven TLS using GenericControllerConfig, complying
	// with the global TLS security profile.
	{
		namespace:      "openshift-cluster-samples-operator",
		deploymentName: "cluster-samples-operator",
		// No TLS env vars — metrics server reads TLS from config file.
		tlsMinVersionEnvVar: "",
		cipherSuitesEnvVar:  "",
		serviceName:         "metrics",
		servicePort:         "60000",
		// cluster-samples-operator does not have an ObservedConfig resource.
		operatorConfigGVR:   schema.GroupVersionResource{},
		operatorConfigName:  "",
		clusterOperatorName: "openshift-samples",
		// CVO injects TLS config into this ConfigMap via config.openshift.io/inject-tls annotation.
		configMapName: "samples-operator-config",
		configMapKey:  "config.yaml",
	},
	// Add more namespaces/services as they adopt the TLS config sync pattern.
}

// ── read-only tests ────────────────────────────────────────────
// These tests only read cluster state (ObservedConfig, ConfigMaps,
var _ = g.Describe("[sig-api-machinery][Feature:TLSObservedConfig][Serial][Suite:openshift/tls-observed-config]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-observed-config")
	ctx := context.Background()

	var isHyperShiftCluster bool

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

	// ── Per-namespace ObservedConfig verification ───────────────────────
	for _, target := range targets {
		target := target
		if target.operatorConfigGVR.Resource == "" || target.operatorConfigName == "" {
			continue
		}

		g.It(fmt.Sprintf("should populate ObservedConfig with TLS settings - %s", target.namespace), func() {
			if isHyperShiftCluster && target.controlPlane {
				g.Skip(fmt.Sprintf("Skipping control-plane target %s on HyperShift (runs on management cluster)", target.namespace))
			}
			testObservedConfig(oc, ctx, target)
		})
	}

	// ── Per-namespace ConfigMap TLS injection verification ──────────────
	for _, target := range targets {
		target := target
		if target.configMapName == "" {
			continue
		}

		g.It(fmt.Sprintf("should have TLS config injected into ConfigMap - %s", target.namespace), func() {
			if isHyperShiftCluster && target.controlPlane {
				g.Skip(fmt.Sprintf("Skipping control-plane target %s on HyperShift (runs on management cluster)", target.namespace))
			}
			testConfigMapTLSInjection(oc, ctx, target)
		})
	}

	// ── Per-namespace TLS env-var verification ──────────────────────────
	for _, target := range targets {
		target := target
		if target.deploymentName == "" || target.tlsMinVersionEnvVar == "" {
			continue
		}

		g.It(fmt.Sprintf("should propagate TLS config to deployment env vars - %s", target.namespace), func() {
			if isHyperShiftCluster && target.controlPlane {
				g.Skip(fmt.Sprintf("Skipping control-plane target %s on HyperShift (runs on management cluster)", target.namespace))
			}
			testDeploymentTLSEnvVars(oc, ctx, target)
		})
	}

	// ── Per-namespace wire-level TLS verification ───────────────────────
	for _, target := range targets {
		target := target
		if target.serviceName == "" || target.servicePort == "" {
			continue
		}

		g.It(fmt.Sprintf("should enforce TLS version at the wire level - %s:%s", target.namespace, target.servicePort), func() {
			if isHyperShiftCluster && target.controlPlane {
				g.Skip(fmt.Sprintf("Skipping control-plane target %s:%s on HyperShift (runs on management cluster)", target.namespace, target.servicePort))
			}
			testWireLevelTLS(oc, ctx, target)
		})
	}
})

// ── Serial disruptive tests ─────────────────────────────────────────────
// These tests modify cluster state (ConfigMap annotations, servingInfo,
// cluster-wide TLS profile) and must run serially.
var _ = g.Describe("[sig-api-machinery][Feature:TLSObservedConfig][Serial][Disruptive][Suite:openshift/tls-observed-config]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("tls-observed-config-serial")
	ctx := context.Background()

	var isHyperShiftCluster bool

	// HyperShift management cluster state, populated in BeforeEach when
	// running on a HyperShift guest cluster.
	var mgmtOC *exutil.CLI
	var hcpNamespace string
	var hostedClusterName string
	var hostedClusterNS string

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("TLS observed-config tests are not applicable to MicroShift clusters")
		}

		isHS, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		isHyperShiftCluster = isHS

		if isHyperShiftCluster {
			mgmtOC = exutil.NewHypershiftManagementCLI("tls-mgmt")
			_, hcpNamespace, err = exutil.GetHypershiftManagementClusterConfigAndNamespace()
			o.Expect(err).NotTo(o.HaveOccurred())
			hostedClusterName, hostedClusterNS = discoverHostedCluster(mgmtOC, hcpNamespace)
			e2e.Logf("HyperShift: HC=%s/%s, HCP NS=%s", hostedClusterNS, hostedClusterName, hcpNamespace)
		}
	})

	// ── ConfigMap annotation restoration tests ────────────────────────────
	for _, target := range targets {
		target := target
		if target.configMapName == "" {
			continue
		}

		g.It(fmt.Sprintf("should restore inject-tls annotation after deletion - %s", target.namespace), func() {
			if isHyperShiftCluster && target.controlPlane {
				g.Skip(fmt.Sprintf("Skipping control-plane target %s on HyperShift (runs on management cluster)", target.namespace))
			}
			testAnnotationRestorationAfterDeletion(oc, ctx, target)
		})

		g.It(fmt.Sprintf("should restore inject-tls annotation when set to false - %s", target.namespace), func() {
			if isHyperShiftCluster && target.controlPlane {
				g.Skip(fmt.Sprintf("Skipping control-plane target %s on HyperShift (runs on management cluster)", target.namespace))
			}
			testAnnotationRestorationWhenFalse(oc, ctx, target)
		})

		g.It(fmt.Sprintf("should restore servingInfo after removal - %s", target.namespace), func() {
			if isHyperShiftCluster && target.controlPlane {
				g.Skip(fmt.Sprintf("Skipping control-plane target %s on HyperShift (runs on management cluster)", target.namespace))
			}
			testServingInfoRestorationAfterRemoval(oc, ctx, target)
		})

		g.It(fmt.Sprintf("should restore servingInfo after modification - %s", target.namespace), func() {
			if isHyperShiftCluster && target.controlPlane {
				g.Skip(fmt.Sprintf("Skipping control-plane target %s on HyperShift (runs on management cluster)", target.namespace))
			}
			testServingInfoRestorationAfterModification(oc, ctx, target)
		})
	}

	// ── Config-change test: switch to Modern, verify, restore ────────
	// This test modifies the cluster APIServer TLS profile, waits for all
	// ClusterOperators and Deployments to stabilize, then verifies that
	// every target service enforces TLS 1.3. It restores the original
	// profile in DeferCleanup.
	g.It("should enforce Modern TLS profile after cluster-wide config change [Timeout:60m]", func() {
		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 50*time.Minute)
		defer configChangeCancel()

		if isHyperShiftCluster {
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
				waitForGuestOperatorsAfterTLSChange(oc, cleanupCtx, "restore")
				e2e.Logf("DeferCleanup: HostedCluster TLS profile restored")
			})

			guestTargets := guestSideTargets()

			// Phase 1: Modern
			g.By("patching HostedCluster with Modern TLS profile")
			setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, modernPatch)
			e2e.Logf("HostedCluster TLS profile patched to Modern")

			g.By("waiting for HCP pods and guest operators to stabilize")
			waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
			waitForGuestOperatorsAfterTLSChange(oc, configChangeCtx, "Modern")

			g.By("verifying guest-side ObservedConfig reflects Modern profile")
			verifyObservedConfigForTargets(oc, configChangeCtx, "VersionTLS13", "Modern", guestTargets)
			g.By("verifying guest-side ConfigMaps reflect Modern profile")
			verifyConfigMapsForTargets(oc, configChangeCtx, "VersionTLS13", "Modern", guestTargets)
			g.By("verifying HCP ConfigMaps reflect Modern profile")
			verifyHCPConfigMaps(mgmtOC, hcpNamespace, "VersionTLS13", "Modern")

			for _, t := range guestTargets {
				if t.deploymentName == "" || t.tlsMinVersionEnvVar == "" {
					continue
				}
				g.By(fmt.Sprintf("verifying %s in %s/%s reflects Modern profile",
					t.tlsMinVersionEnvVar, t.namespace, t.deploymentName))
				deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(
					configChangeCtx, t.deploymentName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				envMap := findEnvAcrossContainers(deployment.Spec.Template.Spec.Containers, t.tlsMinVersionEnvVar)
				o.Expect(envMap).To(o.HaveKey(t.tlsMinVersionEnvVar))
				o.Expect(envMap[t.tlsMinVersionEnvVar]).To(o.Equal("VersionTLS13"))
				e2e.Logf("PASS: %s=VersionTLS13 in %s/%s", t.tlsMinVersionEnvVar, t.namespace, t.deploymentName)
			}

			tlsShouldWork := &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, InsecureSkipVerify: true}
			tlsShouldNotWork := &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, InsecureSkipVerify: true}
			for _, t := range guestTargets {
				if t.serviceName == "" || t.servicePort == "" {
					continue
				}
				g.By(fmt.Sprintf("wire-level TLS check: svc/%s in %s (expecting Modern = TLS 1.3 only)", t.serviceName, t.namespace))
				err = forwardPortAndExecute(t.serviceName, t.namespace, t.servicePort,
					func(localPort int) error { return checkTLSConnection(localPort, tlsShouldWork, tlsShouldNotWork, t) })
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s (Modern)", t.serviceName, t.namespace)
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
			e2e.Logf("DeferCleanup: restoring original TLS profile: %s", profileDesc)
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

			e2e.Logf("DeferCleanup: waiting for all operators to stabilize after restoring profile")
			waitForAllOperatorsAfterTLSChange(oc, cleanupCtx, "restore")
			e2e.Logf("DeferCleanup: original TLS profile restored and cluster is stable")
		})

		// 3. Update TLS profile to Modern.
		g.By("setting APIServer TLS profile to Modern")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(configChangeCtx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
				Type:   configv1.TLSProfileModernType,
				Modern: &configv1.ModernTLSProfile{},
			}
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(configChangeCtx, apiServer, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update APIServer TLS profile to Modern")
		e2e.Logf("APIServer TLS profile updated to Modern")

		// 4. Wait for all operators to stabilize after the config change.
		g.By("waiting for all operators to stabilize after TLS profile change to Modern")
		waitForAllOperatorsAfterTLSChange(oc, configChangeCtx, "Modern")

		// 5. Verify env vars reflect Modern profile (VersionTLS13).
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

			envMap := findEnvAcrossContainers(deployment.Spec.Template.Spec.Containers, t.tlsMinVersionEnvVar)
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

		// 6. Verify ObservedConfig reflects Modern profile (VersionTLS13).
		g.By("verifying ObservedConfig reflects Modern profile (VersionTLS13)")
		verifyObservedConfigAfterSwitch(oc, configChangeCtx, "VersionTLS13", "Modern")

		// 7. Verify ConfigMaps reflect Modern profile (VersionTLS13).
		g.By("verifying ConfigMaps reflect Modern profile (VersionTLS13)")
		verifyConfigMapsAfterSwitch(oc, configChangeCtx, "VersionTLS13", "Modern")

		// 8. Wire-level: verify TLS 1.3 is accepted and TLS 1.2 is rejected.
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

		// DeferCleanup (registered above) restores the original Intermediate
		// profile and waits for operators to stabilize, so we don't need an
		// explicit downgrade phase here.
		e2e.Logf("PASS: Modern TLS profile propagation verified (restore handled by DeferCleanup)")
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
		// IANA equivalents for verifying ConfigMap content (library-go may store either format).
		customCiphersIANA := []string{
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		}

		if isHyperShiftCluster {
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
				waitForGuestOperatorsAfterTLSChange(oc, cleanupCtx, "restore")
				e2e.Logf("DeferCleanup: HostedCluster TLS profile restored")
			})

			guestTargets := guestSideTargets()

			g.By("patching HostedCluster with Custom TLS profile")
			setTLSProfileOnHyperShift(mgmtOC, hostedClusterName, hostedClusterNS, customPatch)
			e2e.Logf("HostedCluster TLS profile patched to Custom (minTLSVersion=TLS12, ciphers=%d)", len(customCiphers))

			g.By("waiting for HCP pods and guest operators to stabilize")
			waitForHCPPods(mgmtOC, hcpNamespace, 8*time.Minute)
			waitForGuestOperatorsAfterTLSChange(oc, configChangeCtx, "Custom")

			g.By("verifying guest-side ObservedConfig reflects Custom profile")
			verifyObservedConfigForTargets(oc, configChangeCtx, "VersionTLS12", "Custom", guestTargets)
			g.By("verifying guest-side ConfigMaps reflect Custom profile")
			verifyConfigMapsForTargets(oc, configChangeCtx, "VersionTLS12", "Custom", guestTargets)
			g.By("verifying HCP ConfigMaps reflect Custom profile")
			verifyHCPConfigMaps(mgmtOC, hcpNamespace, "VersionTLS12", "Custom")

			g.By("verifying wire-level TLS for Custom profile (TLS 1.2) on guest targets")
			for _, t := range guestTargets {
				if t.serviceName == "" || t.servicePort == "" {
					continue
				}
				shouldWork := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}
				shouldNotWork := &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11}
				err := forwardPortAndExecute(t.serviceName, t.namespace, t.servicePort, func(localPort int) error {
					return checkTLSConnection(localPort, shouldWork, shouldNotWork, t)
				})
				o.Expect(err).NotTo(o.HaveOccurred(),
					fmt.Sprintf("wire-level TLS check failed for svc/%s in %s:%s with Custom profile", t.serviceName, t.namespace, t.servicePort))
				e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s:%s (Custom profile)", t.serviceName, t.namespace, t.servicePort)
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
			e2e.Logf("DeferCleanup: restoring original TLS profile: %s", profileDesc)
			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(cleanupCtx, "cluster", metav1.GetOptions{})
				if err != nil {
					return err
				}
				apiServer.Spec.TLSSecurityProfile = originalProfile
				_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(cleanupCtx, apiServer, metav1.UpdateOptions{})
				return err
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to restore original TLS profile")

			e2e.Logf("DeferCleanup: waiting for all operators to stabilize after restoring profile")
			waitForAllOperatorsAfterTLSChange(oc, cleanupCtx, "restore")
			e2e.Logf("DeferCleanup: original TLS profile restored and cluster is stable")
		})

		// 3. Set the APIServer TLS profile to Custom.
		g.By("setting APIServer TLS profile to Custom (TLS 1.2 with specific ciphers)")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(configChangeCtx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						Ciphers:       customCiphers,
						MinTLSVersion: configv1.VersionTLS12,
					},
				},
			}
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(configChangeCtx, apiServer, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update APIServer TLS profile to Custom")
		e2e.Logf("APIServer TLS profile updated to Custom (minTLSVersion=TLS12, ciphers=%d)", len(customCiphers))

		// 4. Wait for all operators to stabilize after Custom TLS profile change.
		g.By("waiting for all operators to stabilize after TLS profile change to Custom")
		waitForAllOperatorsAfterTLSChange(oc, configChangeCtx, "Custom")

		// 5. Verify ObservedConfig reflects Custom profile (VersionTLS12).
		g.By("verifying ObservedConfig reflects Custom profile (VersionTLS12)")
		verifyObservedConfigAfterSwitch(oc, configChangeCtx, "VersionTLS12", "Custom")

		// 6. Verify ConfigMaps reflect Custom profile (VersionTLS12).
		g.By("verifying ConfigMaps reflect Custom profile (VersionTLS12)")
		for _, t := range targets {
			if t.configMapName == "" {
				continue
			}
			cmNamespace := t.configMapNamespace
			if cmNamespace == "" {
				cmNamespace = t.namespace
			}
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(configChangeCtx, t.configMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("SKIP: ConfigMap %s/%s not found: %v", cmNamespace, t.configMapName, err)
				continue
			}
			configKey := t.configMapKey
			if configKey == "" {
				configKey = "config.yaml"
			}
			configData := cm.Data[configKey]
			o.Expect(cm.Annotations).To(o.HaveKey("config.openshift.io/inject-tls"),
				fmt.Sprintf("ConfigMap %s/%s is missing config.openshift.io/inject-tls annotation", cmNamespace, t.configMapName))
			o.Expect(configData).To(o.ContainSubstring("VersionTLS12"),
				fmt.Sprintf("ConfigMap %s/%s should have VersionTLS12 for Custom profile", cmNamespace, t.configMapName))
			e2e.Logf("PASS: ConfigMap %s/%s has VersionTLS12 for Custom profile", cmNamespace, t.configMapName)

			// Verify custom cipher suites are present (CVO may use OpenSSL or IANA names).
			for i := 0; i < 2; i++ {
				found := strings.Contains(configData, customCiphers[i]) || strings.Contains(configData, customCiphersIANA[i])
				o.Expect(found).To(o.BeTrue(),
					fmt.Sprintf("ConfigMap %s/%s should contain cipher %s (or IANA equivalent %s)", cmNamespace, t.configMapName, customCiphers[i], customCiphersIANA[i]))
			}
			e2e.Logf("PASS: ConfigMap %s/%s has custom cipher suites", cmNamespace, t.configMapName)
		}

		// 7. Wire-level TLS verification for Custom profile.
		// Custom profile with TLS 1.2 should accept TLS 1.2 and reject TLS 1.1.
		g.By("verifying wire-level TLS for Custom profile (TLS 1.2)")
		for _, t := range targets {
			if t.serviceName == "" || t.servicePort == "" {
				continue
			}
			g.By(fmt.Sprintf("wire-level TLS check: svc/%s in %s (expecting Custom = TLS 1.2+)",
				t.serviceName, t.namespace))

			// TLS config that should work: TLS 1.2+
			shouldWork := &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			}
			// TLS config that should NOT work: max TLS 1.1
			shouldNotWork := &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS10,
				MaxVersion:         tls.VersionTLS11,
			}

			err := forwardPortAndExecute(t.serviceName, t.namespace, t.servicePort, func(localPort int) error {
				return checkTLSConnection(localPort, shouldWork, shouldNotWork, t)
			})
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("wire-level TLS check failed for svc/%s in %s:%s with Custom profile", t.serviceName, t.namespace, t.servicePort))
			e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s:%s (Custom profile)", t.serviceName, t.namespace, t.servicePort)
		}

		e2e.Logf("PASS: Custom TLS profile verified successfully")
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

// testConfigMapTLSInjection verifies that CVO has injected TLS configuration
// into the operator's ConfigMap via the config.openshift.io/inject-tls annotation.
// This validates that CVO is reading the APIServer TLS profile and injecting
// the minTLSVersion and cipherSuites into the ConfigMap's servingInfo section.
func testConfigMapTLSInjection(oc *exutil.CLI, ctx context.Context, t tlsTarget) {
	// Determine the namespace for the ConfigMap (defaults to target namespace).
	cmNamespace := t.configMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.configMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.configMapName))

	g.By("verifying config.openshift.io/inject-tls annotation is present")
	injectTLSAnnotation, found := cm.Annotations["config.openshift.io/inject-tls"]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing config.openshift.io/inject-tls annotation", cmNamespace, t.configMapName))
	o.Expect(injectTLSAnnotation).To(o.Equal("true"),
		fmt.Sprintf("ConfigMap %s/%s has inject-tls annotation but value is not 'true': %s", cmNamespace, t.configMapName, injectTLSAnnotation))
	e2e.Logf("ConfigMap %s/%s has config.openshift.io/inject-tls=true annotation", cmNamespace, t.configMapName)

	// Get the config key (defaults to "config.yaml" if not specified).
	configKey := t.configMapKey
	if configKey == "" {
		configKey = "config.yaml"
	}

	// Extract the config data from the ConfigMap.
	g.By(fmt.Sprintf("extracting %s from ConfigMap data", configKey))
	configData, found := cm.Data[configKey]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing %s key", cmNamespace, t.configMapName, configKey))
	o.Expect(configData).NotTo(o.BeEmpty(),
		fmt.Sprintf("ConfigMap %s/%s has empty %s", cmNamespace, t.configMapName, configKey))

	// Log the servingInfo section for debugging.
	e2e.Logf("ConfigMap %s/%s %s content (servingInfo section):", cmNamespace, t.configMapName, configKey)
	for _, line := range strings.Split(configData, "\n") {
		if strings.Contains(line, "servingInfo") ||
			strings.Contains(line, "minTLSVersion") ||
			strings.Contains(line, "cipherSuites") ||
			strings.Contains(line, "bindAddress") ||
			(strings.HasPrefix(strings.TrimSpace(line), "- TLS_") || strings.HasPrefix(strings.TrimSpace(line), "- ECDHE")) {
			e2e.Logf("  %s", line)
		}
	}

	// Parse the config YAML to verify servingInfo has TLS settings.
	// The config should have a structure like:
	// servingInfo:
	//   minTLSVersion: VersionTLS12
	//   cipherSuites: [...]
	g.By("verifying servingInfo.minTLSVersion in ConfigMap config")
	o.Expect(configData).To(o.ContainSubstring("minTLSVersion"),
		fmt.Sprintf("ConfigMap %s/%s config does not contain minTLSVersion", cmNamespace, t.configMapName))

	// Extract actual minTLSVersion for logging.
	actualMinTLSVersion := "unknown"
	if strings.Contains(configData, "VersionTLS13") {
		actualMinTLSVersion = "VersionTLS13"
	} else if strings.Contains(configData, "VersionTLS12") {
		actualMinTLSVersion = "VersionTLS12"
	}
	e2e.Logf("ConfigMap %s/%s actual minTLSVersion: %s", cmNamespace, t.configMapName, actualMinTLSVersion)

	g.By("verifying servingInfo.cipherSuites in ConfigMap config")
	o.Expect(configData).To(o.ContainSubstring("cipherSuites"),
		fmt.Sprintf("ConfigMap %s/%s config does not contain cipherSuites", cmNamespace, t.configMapName))

	// Count cipher suites for logging.
	cipherCount := strings.Count(configData, "- TLS_") + strings.Count(configData, "- ECDHE")
	e2e.Logf("ConfigMap %s/%s cipherSuites count: %d", cmNamespace, t.configMapName, cipherCount)

	// Cross-check against the cluster APIServer profile.
	g.By("cross-checking ConfigMap TLS config with cluster APIServer TLS profile")
	expectedMinVersion, profileType := getExpectedMinTLSVersionWithType(oc, ctx)
	e2e.Logf("Cluster TLS profile: %s, expected minTLSVersion: %s", profileType, expectedMinVersion)
	e2e.Logf("ConfigMap actual minTLSVersion: %s, expected: %s", actualMinTLSVersion, expectedMinVersion)

	o.Expect(configData).To(o.ContainSubstring(expectedMinVersion),
		fmt.Sprintf("ConfigMap %s/%s config does not contain expected minTLSVersion=%s (actual=%s, profile=%s)",
			cmNamespace, t.configMapName, expectedMinVersion, actualMinTLSVersion, profileType))

	e2e.Logf("PASS: ConfigMap %s/%s has TLS config injected matching cluster profile (profile=%s, minTLSVersion=%s, cipherSuites=%d)",
		cmNamespace, t.configMapName, profileType, expectedMinVersion, cipherCount)
}

// testAnnotationRestorationAfterDeletion verifies that if the inject-tls annotation
// is deleted from the ConfigMap, the operator restores it.
func testAnnotationRestorationAfterDeletion(oc *exutil.CLI, ctx context.Context, t tlsTarget) {
	cmNamespace := t.configMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	// Get the original ConfigMap and verify annotation exists.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.configMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.configMapName))

	_, found := cm.Annotations["config.openshift.io/inject-tls"]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing config.openshift.io/inject-tls annotation", cmNamespace, t.configMapName))

	// Delete the annotation.
	g.By("deleting config.openshift.io/inject-tls annotation")
	delete(cm.Annotations, "config.openshift.io/inject-tls")
	_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to update ConfigMap %s/%s to delete annotation", cmNamespace, t.configMapName))
	e2e.Logf("Deleted inject-tls annotation from ConfigMap %s/%s", cmNamespace, t.configMapName)

	// Wait for the operator to restore the annotation.
	g.By("waiting for operator to restore the inject-tls annotation")
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}

			val, found := cm.Annotations["config.openshift.io/inject-tls"]
			if found && val == "true" {
				e2e.Logf("  poll: annotation restored! inject-tls=%s", val)
				return true, nil
			}
			e2e.Logf("  poll: annotation not yet restored (found=%v, val=%s)", found, val)
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("inject-tls annotation was not restored on ConfigMap %s/%s within timeout", cmNamespace, t.configMapName))

	e2e.Logf("PASS: inject-tls annotation was restored after deletion on ConfigMap %s/%s", cmNamespace, t.configMapName)
}

// testAnnotationRestorationWhenFalse verifies that if the inject-tls annotation
// is set to "false", the operator restores it to "true".
func testAnnotationRestorationWhenFalse(oc *exutil.CLI, ctx context.Context, t tlsTarget) {
	cmNamespace := t.configMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	// Get the original ConfigMap.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.configMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.configMapName))

	_, annotationFound := cm.Annotations["config.openshift.io/inject-tls"]
	o.Expect(annotationFound).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing config.openshift.io/inject-tls annotation", cmNamespace, t.configMapName))

	// Set the annotation to "false".
	g.By("setting config.openshift.io/inject-tls annotation to 'false'")
	cm.Annotations["config.openshift.io/inject-tls"] = "false"
	_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to update ConfigMap %s/%s to set annotation to false", cmNamespace, t.configMapName))
	e2e.Logf("Set inject-tls annotation to 'false' on ConfigMap %s/%s", cmNamespace, t.configMapName)

	// Wait for the operator to restore the annotation to "true".
	g.By("waiting for operator to restore the inject-tls annotation to 'true'")
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}

			val, found := cm.Annotations["config.openshift.io/inject-tls"]
			if found && val == "true" {
				e2e.Logf("  poll: annotation restored to 'true'!")
				return true, nil
			}
			e2e.Logf("  poll: annotation not yet restored (found=%v, val=%s)", found, val)
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("inject-tls annotation was not restored to 'true' on ConfigMap %s/%s within timeout", cmNamespace, t.configMapName))

	e2e.Logf("PASS: inject-tls annotation was restored to 'true' after being set to 'false' on ConfigMap %s/%s", cmNamespace, t.configMapName)
}

// testServingInfoRestorationAfterRemoval verifies that if the servingInfo section
// is removed from the ConfigMap, the operator restores it with correct TLS settings.
func testServingInfoRestorationAfterRemoval(oc *exutil.CLI, ctx context.Context, t tlsTarget) {
	cmNamespace := t.configMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	configKey := t.configMapKey
	if configKey == "" {
		configKey = "config.yaml"
	}

	// Get the original ConfigMap and verify servingInfo exists.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.configMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.configMapName))

	// Verify servingInfo exists before we remove it.
	configData := cm.Data[configKey]
	if !strings.Contains(configData, "servingInfo") {
		g.Skip(fmt.Sprintf("ConfigMap %s/%s does not have servingInfo, skipping removal test", cmNamespace, t.configMapName))
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
	cm.Data[configKey] = strings.Join(newLines, "\n")

	_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to update ConfigMap %s/%s to remove servingInfo", cmNamespace, t.configMapName))
	e2e.Logf("Removed servingInfo from ConfigMap %s/%s", cmNamespace, t.configMapName)

	// Wait for the operator to restore servingInfo.
	g.By("waiting for operator to restore servingInfo section")
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}

			configData := cm.Data[configKey]
			if strings.Contains(configData, "servingInfo") && strings.Contains(configData, "minTLSVersion") {
				e2e.Logf("  poll: servingInfo restored!")
				return true, nil
			}
			e2e.Logf("  poll: servingInfo not yet restored")
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("servingInfo was not restored on ConfigMap %s/%s within timeout", cmNamespace, t.configMapName))

	// Verify the restored config matches expected TLS version.
	cm, err = oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	configData = cm.Data[configKey]
	o.Expect(configData).To(o.ContainSubstring("minTLSVersion"),
		"restored servingInfo should contain minTLSVersion")

	e2e.Logf("PASS: servingInfo was restored after removal on ConfigMap %s/%s", cmNamespace, t.configMapName)
}

// testServingInfoRestorationAfterModification verifies that if the servingInfo
// minTLSVersion is modified to an incorrect value, the operator restores it.
func testServingInfoRestorationAfterModification(oc *exutil.CLI, ctx context.Context, t tlsTarget) {
	cmNamespace := t.configMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	configKey := t.configMapKey
	if configKey == "" {
		configKey = "config.yaml"
	}

	// Get the expected TLS version from the cluster profile.
	expectedMinVersion := getExpectedMinTLSVersion(oc, ctx)
	e2e.Logf("Expected minTLSVersion from cluster profile: %s", expectedMinVersion)

	// Get the original ConfigMap.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.configMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.configMapName))

	// Verify servingInfo exists.
	configData := cm.Data[configKey]
	if !strings.Contains(configData, "minTLSVersion") {
		g.Skip(fmt.Sprintf("ConfigMap %s/%s does not have minTLSVersion, skipping modification test", cmNamespace, t.configMapName))
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
	cm.Data[configKey] = strings.Join(newLines, "\n")

	_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to update ConfigMap %s/%s to modify minTLSVersion", cmNamespace, t.configMapName))
	e2e.Logf("Modified minTLSVersion to '%s' on ConfigMap %s/%s", wrongValue, cmNamespace, t.configMapName)

	// Wait for the operator to restore correct minTLSVersion.
	g.By("waiting for operator to restore correct minTLSVersion")
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}

			configData := cm.Data[configKey]
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
			cmNamespace, t.configMapName, expectedMinVersion))

	e2e.Logf("PASS: minTLSVersion was restored to '%s' after modification on ConfigMap %s/%s",
		expectedMinVersion, cmNamespace, t.configMapName)
}

// testDeploymentTLSEnvVars verifies that the deployment in the given namespace
// has TLS environment variables that match the expected TLS profile.
func testDeploymentTLSEnvVars(oc *exutil.CLI, ctx context.Context, t tlsTarget) {
	g.By("getting cluster APIServer TLS profile")
	expectedMinVersion := getExpectedMinTLSVersion(oc, ctx)
	e2e.Logf("Expected minTLSVersion from cluster profile: %s", expectedMinVersion)

	g.By(fmt.Sprintf("verifying namespace %s exists", t.namespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, t.namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", t.namespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", t.namespace))

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
	envMap := findEnvAcrossContainers(deployment.Spec.Template.Spec.Containers, t.tlsMinVersionEnvVar)
	logEnvVars(envMap, t.tlsMinVersionEnvVar)

	o.Expect(envMap).To(o.HaveKey(t.tlsMinVersionEnvVar),
		fmt.Sprintf("expected %s to be set in deployment %s/%s (checked all %d containers)",
			t.tlsMinVersionEnvVar, t.namespace, t.deploymentName, len(deployment.Spec.Template.Spec.Containers)))
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
			fmt.Sprintf("expected %s to be set in deployment %s/%s (checked all %d containers)",
				t.cipherSuitesEnvVar, t.namespace, t.deploymentName, len(deployment.Spec.Template.Spec.Containers)))
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
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", t.namespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", t.namespace))

	if t.deploymentName != "" {
		g.By(fmt.Sprintf("waiting for deployment %s/%s to be fully rolled out", t.namespace, t.deploymentName))
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(ctx, t.deploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("failed to get deployment %s/%s", t.namespace, t.deploymentName))
		err = waitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, operatorRolloutTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("deployment %s/%s did not complete rollout (timeout: %v)", t.namespace, t.deploymentName, operatorRolloutTimeout))
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

// verifyObservedConfigAfterSwitch checks that every target with an operator
// config has its ObservedConfig servingInfo.minTLSVersion matching the
// expected version after a profile switch.
func verifyObservedConfigAfterSwitch(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string) {
	verifyObservedConfigForTargets(oc, ctx, expectedVersion, profileLabel, targets)
}

// verifyObservedConfigForTargets checks a specific list of targets for
// ObservedConfig correctness after a TLS profile switch.
func verifyObservedConfigForTargets(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string, targetList []tlsTarget) {
	dynClient := oc.AdminDynamicClient()
	for _, t := range targetList {
		if t.operatorConfigGVR.Resource == "" || t.operatorConfigName == "" {
			continue
		}
		resource, err := dynClient.Resource(t.operatorConfigGVR).Get(ctx, t.operatorConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("failed to get operator config %s/%s after %s switch",
				t.operatorConfigGVR.Resource, t.operatorConfigName, profileLabel))

		observedConfigRaw, found, err := unstructured.NestedMap(resource.Object, "spec", "observedConfig")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(found).To(o.BeTrue(),
			fmt.Sprintf("expected spec.observedConfig in %s/%s after %s switch",
				t.operatorConfigGVR.Resource, t.operatorConfigName, profileLabel))

		minTLSVersion, found, err := unstructured.NestedString(observedConfigRaw, "servingInfo", "minTLSVersion")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(found).To(o.BeTrue(),
			fmt.Sprintf("expected servingInfo.minTLSVersion in ObservedConfig of %s/%s after %s switch",
				t.operatorConfigGVR.Resource, t.operatorConfigName, profileLabel))
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
	verifyConfigMapsForTargets(oc, ctx, expectedVersion, profileLabel, targets)
}

// verifyConfigMapsForTargets checks a specific list of targets for
// ConfigMap TLS injection correctness after a TLS profile switch.
func verifyConfigMapsForTargets(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string, targetList []tlsTarget) {
	for _, t := range targetList {
		if t.configMapName == "" {
			continue
		}
		cmNamespace := t.configMapNamespace
		if cmNamespace == "" {
			cmNamespace = t.namespace
		}
		cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("SKIP: ConfigMap %s/%s not found: %v", cmNamespace, t.configMapName, err)
			continue
		}
		configKey := t.configMapKey
		if configKey == "" {
			configKey = "config.yaml"
		}
		configData := cm.Data[configKey]
		o.Expect(cm.Annotations).To(o.HaveKey("config.openshift.io/inject-tls"),
			fmt.Sprintf("ConfigMap %s/%s is missing config.openshift.io/inject-tls annotation", cmNamespace, t.configMapName))
		o.Expect(configData).To(o.ContainSubstring(expectedVersion),
			fmt.Sprintf("ConfigMap %s/%s should have %s after %s switch",
				cmNamespace, t.configMapName, expectedVersion, profileLabel))
		e2e.Logf("PASS: ConfigMap %s/%s has %s after %s switch",
			cmNamespace, t.configMapName, expectedVersion, profileLabel)
	}
}

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
	minVersion, _ := getExpectedMinTLSVersionWithType(oc, ctx)
	return minVersion
}

// getExpectedMinTLSVersionWithType returns the expected minTLSVersion string
// and the profile type name for better logging.
func getExpectedMinTLSVersionWithType(oc *exutil.CLI, ctx context.Context) (string, string) {
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
	profileName := string(profileType)
	if profileType == "" || profileType == configv1.TLSProfileIntermediateType {
		profileName = "Intermediate (default)"
	}

	e2e.Logf("Cluster APIServer TLS profile: type=%s, minTLSVersion=%s", profileName, minVersion)
	return minVersion, profileName
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
func checkTLSConnection(localPort int, shouldWork, shouldNotWork *tls.Config, t tlsTarget) error {
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
// given env var key and returns a merged env map. If the key is found in
// any container, that container's full env map is returned. Falls back to
// the first container's env if not found anywhere.
func findEnvAcrossContainers(containers []corev1.Container, key string) map[string]string {
	for _, c := range containers {
		m := envToMap(c.Env)
		if _, ok := m[key]; ok {
			return m
		}
	}
	if len(containers) > 0 {
		return envToMap(containers[0].Env)
	}
	return map[string]string{}
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
	for _, co := range targetClusterOperators() {
		e2e.Logf("Waiting for ClusterOperator %s to stabilize after %s switch", co, profileLabel)
		waitForClusterOperatorStable(oc, ctx, co)
	}

	for _, t := range targets {
		if t.deploymentName == "" {
			continue
		}
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

// guestSideTargets returns the targets that run on the guest cluster (not the
// management cluster control plane). Used on HyperShift to skip CP targets.
func guestSideTargets() []tlsTarget {
	var result []tlsTarget
	for _, t := range targets {
		if !t.controlPlane {
			result = append(result, t)
		}
	}
	return result
}

// guestSideClusterOperators returns the deduplicated ClusterOperator names
// from guest-side targets only.
func guestSideClusterOperators() []string {
	seen := map[string]bool{}
	var result []string
	for _, t := range guestSideTargets() {
		if t.clusterOperatorName == "" || seen[t.clusterOperatorName] {
			continue
		}
		seen[t.clusterOperatorName] = true
		result = append(result, t.clusterOperatorName)
	}
	return result
}

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
func waitForGuestOperatorsAfterTLSChange(oc *exutil.CLI, ctx context.Context, profileLabel string) {
	e2e.Logf("Waiting for guest-side ClusterOperators to stabilize after %s profile change", profileLabel)
	for _, co := range guestSideClusterOperators() {
		e2e.Logf("Waiting for ClusterOperator %s to stabilize after %s switch", co, profileLabel)
		waitForClusterOperatorStable(oc, ctx, co)
	}

	for _, t := range guestSideTargets() {
		if t.deploymentName == "" {
			continue
		}
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
