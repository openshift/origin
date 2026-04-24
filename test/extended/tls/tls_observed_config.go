package tls

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// OperatorRolloutTimeout is the maximum time to wait for an operator
	// workload (Deployment or static pod) to complete rollout after a TLS
	// profile change. KAS (static pod) rollout typically takes 15-20 minutes;
	// Deployment-based operators are usually faster. 25 minutes covers both.
	OperatorRolloutTimeout = 25 * time.Minute
)

// TLSTarget describes a namespace/service that must honor the cluster APIServer
// TLS profile.  Each target gets its own Ginkgo It block so failures are
// reported per-namespace, following the same pattern as the ROFS tests.
type TLSTarget struct {
	Namespace           string
	DeploymentName      string
	TLSMinVersionEnvVar string
	CipherSuitesEnvVar  string
	ServiceName         string
	ServicePort         string
	OperatorConfigGVR   schema.GroupVersionResource
	OperatorConfigName  string
	ClusterOperatorName string
	ConfigMapName       string
	ConfigMapNamespace  string
	ConfigMapKey        string
	ControlPlane        bool
}

// Targets is the unified list of OpenShift namespaces and services that should
// propagate the cluster APIServer TLS profile.  Each entry can participate in
// multiple test categories (ObservedConfig, ConfigMap injection, env vars,
// wire-level TLS) depending on which fields are populated.  The test loops
// filter by checking for non-empty fields, so secondary entries (e.g. an
// extra port on the same operator) can set only serviceName/servicePort for
// wire-level coverage while leaving operatorConfigGVR/configMapName empty to
// avoid duplicate checks already handled by the primary entry.
var Targets = []TLSTarget{
	{
		Namespace:           "openshift-image-registry",
		DeploymentName:      "image-registry",
		TLSMinVersionEnvVar: "REGISTRY_HTTP_TLS_MINVERSION",
		CipherSuitesEnvVar:  "OPENSHIFT_REGISTRY_HTTP_TLS_CIPHERSUITES",
		ServiceName:         "image-registry",
		ServicePort:         "5000",
		OperatorConfigGVR: schema.GroupVersionResource{
			Group:    "imageregistry.operator.openshift.io",
			Version:  "v1",
			Resource: "configs",
		},
		OperatorConfigName:  "cluster",
		ClusterOperatorName: "image-registry",
		// CVO injects TLS config into this ConfigMap via config.openshift.io/inject-tls annotation.
		// PR 1297 (cluster-image-registry-operator) adds this annotation.
		ConfigMapName: "image-registry-operator-config",
		ConfigMapKey:  "config.yaml",
	},
	// image-registry-operator metrics service on port 60000.
	// PR 1297 (cluster-image-registry-operator, IR-350) makes the metrics
	// server TLS configuration file-based, complying with global TLS profile.
	{
		Namespace:      "openshift-image-registry",
		DeploymentName: "", // Operator deployment, not image-registry deployment
		// No TLS env vars — metrics server reads TLS from config files.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "image-registry-operator",
		ServicePort:         "60000",
		// ObservedConfig and ConfigMap are already verified by the primary
		// image-registry entry above; this entry only adds wire-level TLS
		// coverage for the operator metrics port.
		OperatorConfigGVR:   schema.GroupVersionResource{},
		OperatorConfigName:  "",
		ClusterOperatorName: "image-registry",
		ConfigMapName:       "",
		ConfigMapKey:        "",
		ControlPlane:        true,
	},
	// openshift-controller-manager propagates TLS config via ConfigMap
	// (ObservedConfig → config.yaml), NOT via env vars. So we skip the
	// env-var check but still verify ObservedConfig and wire-level TLS.
	// PR 412 (cluster-openshift-controller-manager-operator) adds inject-tls annotation.
	{
		Namespace:      "openshift-controller-manager",
		DeploymentName: "controller-manager",
		// No TLS env vars — controller-manager reads TLS from its config file.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "controller-manager",
		ServicePort:         "443",
		OperatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "openshiftcontrollermanagers",
		},
		OperatorConfigName:  "cluster",
		ClusterOperatorName: "openshift-controller-manager",
		// CVO injects TLS config into this ConfigMap (in the operator namespace).
		ConfigMapName:      "openshift-controller-manager-operator-config",
		ConfigMapNamespace: "openshift-controller-manager-operator",
		ConfigMapKey:       "config.yaml",
		ControlPlane:       true,
	},
	// kube-apiserver is a static pod managed by cluster-kube-apiserver-operator.
	// PR 2032/2059 added TLS security profile propagation to its ObservedConfig.
	// It reads TLS config from its config files, not env vars.
	{
		Namespace:      "openshift-kube-apiserver",
		DeploymentName: "", // Static pod, not a deployment
		// No TLS env vars — kube-apiserver reads TLS from its config files.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "apiserver",
		ServicePort:         "443",
		OperatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "kubeapiservers",
		},
		OperatorConfigName:  "cluster",
		ClusterOperatorName: "kube-apiserver",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		ConfigMapName:      "kube-apiserver-operator-config",
		ConfigMapNamespace: "openshift-kube-apiserver-operator",
		ConfigMapKey:       "config.yaml",
		ControlPlane:       true,
	},
	// kube-apiserver's check-endpoints port (17697) on the apiserver service.
	// PR 2032 (cluster-kube-apiserver-operator) ensures this port complies
	// with the global TLS security profile.
	{
		Namespace:      "openshift-kube-apiserver",
		DeploymentName: "", // Static pod, not a deployment
		// No TLS env vars — kube-apiserver reads TLS from config files.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "apiserver",
		ServicePort:         "17697",
		// ObservedConfig and ConfigMap are already verified by the primary
		// kube-apiserver:443 entry above; this entry only adds wire-level
		// TLS coverage for the check-endpoints port.
		OperatorConfigGVR:   schema.GroupVersionResource{},
		OperatorConfigName:  "",
		ClusterOperatorName: "kube-apiserver",
		ControlPlane:        true,
	},
	// openshift-apiserver main API endpoint.
	// PR 662 (cluster-openshift-apiserver-operator) adds inject-tls annotation.
	{
		Namespace:      "openshift-apiserver",
		DeploymentName: "apiserver",
		// No TLS env vars — apiserver reads TLS from config files.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "api",
		ServicePort:         "443",
		OperatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "openshiftapiservers",
		},
		OperatorConfigName:  "cluster",
		ClusterOperatorName: "openshift-apiserver",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		ConfigMapName:      "openshift-apiserver-operator-config",
		ConfigMapNamespace: "openshift-apiserver-operator",
		ConfigMapKey:       "config.yaml",
		ControlPlane:       true,
	},
	// openshift-apiserver's check-endpoints service on port 17698.
	// PR 657 (cluster-openshift-apiserver-operator, CNTRLPLANE-2619) ensures
	// this port complies with the global TLS security profile.
	{
		Namespace:      "openshift-apiserver",
		DeploymentName: "", // check-endpoints uses same deployment
		// No TLS env vars — reads TLS from config files.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "check-endpoints",
		ServicePort:         "17698",
		// ObservedConfig and ConfigMap are already verified by the primary
		// openshift-apiserver:443 entry above; this entry only adds
		// wire-level TLS coverage for the check-endpoints port.
		OperatorConfigGVR:   schema.GroupVersionResource{},
		OperatorConfigName:  "",
		ClusterOperatorName: "openshift-apiserver",
		ControlPlane:        true,
	},
	// cluster-version-operator (CVO).
	// PR 1322 enables CVO to INJECT TLS config into OTHER operators' ConfigMaps
	// (those annotated with config.openshift.io/inject-tls: "true").
	// NOTE: CVO's own metrics endpoint (port 9099) does NOT currently respect
	// the cluster-wide TLS profile - it always accepts TLS 1.2. This is expected
	// behavior for now; the PR scope is ConfigMap injection, not CVO's own endpoint.
	// Therefore we skip wire-level TLS tests for CVO (serviceName is empty).
	{
		Namespace:      "openshift-cluster-version",
		DeploymentName: "cluster-version-operator",
		// No TLS env vars — CVO reads TLS from config files.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		// Skip wire-level TLS test: CVO metrics endpoint doesn't follow cluster TLS profile.
		ServiceName:        "",
		ServicePort:        "",
		OperatorConfigGVR:  schema.GroupVersionResource{}, // CVO manages itself
		OperatorConfigName: "",
		// CVO does not have a ClusterOperator for itself - it manages all other operators.
		// Skip stability check; deployment rollout wait is sufficient.
		ClusterOperatorName: "",
		// CVO does not use a ConfigMap with inject-tls annotation.
		// It reads TLS config directly from the cluster config.
		ConfigMapName: "",
		ConfigMapKey:  "",
		ControlPlane:  true,
	},
	// etcd is a static pod managed by cluster-etcd-operator.
	// PR 1556 (cluster-etcd-operator) adds TLS security profile propagation.
	{
		Namespace:      "openshift-etcd",
		DeploymentName: "", // Static pod, not a deployment
		// No TLS env vars — etcd reads TLS from its config files.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "etcd",
		ServicePort:         "2379",
		OperatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "etcds",
		},
		OperatorConfigName:  "cluster",
		ClusterOperatorName: "etcd",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		ConfigMapName:      "etcd-operator-config",
		ConfigMapNamespace: "openshift-etcd-operator",
		ConfigMapKey:       "config.yaml",
		ControlPlane:       true,
	},
	// kube-controller-manager is a static pod managed by cluster-kube-controller-manager-operator.
	// PR 915 (cluster-kube-controller-manager-operator) adds TLS security profile propagation.
	{
		Namespace:      "openshift-kube-controller-manager",
		DeploymentName: "", // Static pod, not a deployment
		// No TLS env vars — kube-controller-manager reads TLS from its config files.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "kube-controller-manager",
		ServicePort:         "443",
		OperatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "kubecontrollermanagers",
		},
		OperatorConfigName:  "cluster",
		ClusterOperatorName: "kube-controller-manager",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		ConfigMapName:      "kube-controller-manager-operator-config",
		ConfigMapNamespace: "openshift-kube-controller-manager-operator",
		ConfigMapKey:       "config.yaml",
		ControlPlane:       true,
	},
	// kube-scheduler is a static pod managed by cluster-kube-scheduler-operator.
	// PR 617 (cluster-kube-scheduler-operator) adds TLS security profile propagation.
	{
		Namespace:      "openshift-kube-scheduler",
		DeploymentName: "", // Static pod, not a deployment
		// No TLS env vars — kube-scheduler reads TLS from its config files.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "scheduler",
		ServicePort:         "443",
		OperatorConfigGVR: schema.GroupVersionResource{
			Group:    "operator.openshift.io",
			Version:  "v1",
			Resource: "kubeschedulers",
		},
		OperatorConfigName:  "cluster",
		ClusterOperatorName: "kube-scheduler",
		// CVO injects TLS config into this ConfigMap in the operator namespace.
		ConfigMapName:      "openshift-kube-scheduler-operator-config",
		ConfigMapNamespace: "openshift-kube-scheduler-operator",
		ConfigMapKey:       "config.yaml",
		ControlPlane:       true,
	},
	// cluster-samples-operator metrics service on port 60000.
	// PR 684 (cluster-samples-operator, CNTRLPLANE-3176) migrates the metrics
	// server to config-driven TLS using GenericControllerConfig, complying
	// with the global TLS security profile.
	{
		Namespace:      "openshift-cluster-samples-operator",
		DeploymentName: "cluster-samples-operator",
		// No TLS env vars — metrics server reads TLS from config file.
		TLSMinVersionEnvVar: "",
		CipherSuitesEnvVar:  "",
		ServiceName:         "metrics",
		ServicePort:         "60000",
		// cluster-samples-operator does not have an ObservedConfig resource.
		OperatorConfigGVR:   schema.GroupVersionResource{},
		OperatorConfigName:  "",
		ClusterOperatorName: "openshift-samples",
		// CVO injects TLS config into this ConfigMap via config.openshift.io/inject-tls annotation.
		ConfigMapName: "samples-operator-config",
		ConfigMapKey:  "config.yaml",
	},
	// Add more namespaces/services as they adopt the TLS config sync pattern.
}

// ─── Test implementations ──────────────────────────────────────────────────
// Test registration (g.Describe blocks) lives in:
//   - tls_observed_config_ocp.go       — standalone OCP and shared tests
//   - tls_observed_config_hypershift.go — HyperShift-specific tests

// TestObservedConfig verifies that the operator's ObservedConfig contains
// a properly populated servingInfo with minTLSVersion and cipherSuites.
func TestObservedConfig(oc *exutil.CLI, ctx context.Context, t TLSTarget, isHyperShift bool) {
	g.By(fmt.Sprintf("getting operator config %s/%s via dynamic client",
		t.OperatorConfigGVR.Resource, t.OperatorConfigName))

	dynClient := oc.AdminDynamicClient()
	resource, err := dynClient.Resource(t.OperatorConfigGVR).Get(ctx, t.OperatorConfigName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) && isHyperShift && t.ControlPlane {
		g.Skip(fmt.Sprintf("Operator config %s/%s does not exist on HyperShift guest (control-plane resource is on management cluster)",
			t.OperatorConfigGVR.Resource, t.OperatorConfigName))
	}
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get operator config %s/%s",
			t.OperatorConfigGVR.Resource, t.OperatorConfigName))

	// Extract spec.observedConfig from the unstructured resource.
	observedConfigRaw, found, err := unstructured.NestedMap(resource.Object, "spec", "observedConfig")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to extract spec.observedConfig")
	if isHyperShift && t.ControlPlane && (!found || len(observedConfigRaw) == 0) {
		g.Skip(fmt.Sprintf("Operator config %s/%s exists on HyperShift guest but spec.observedConfig is not populated (operator runs on management cluster)",
			t.OperatorConfigGVR.Resource, t.OperatorConfigName))
	}
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
		t.OperatorConfigGVR.Resource, t.OperatorConfigName)
}

// TestConfigMapTLSInjection verifies that CVO has injected TLS configuration
// into the operator's ConfigMap via the config.openshift.io/inject-tls annotation.
// This validates that CVO is reading the APIServer TLS profile and injecting
// the minTLSVersion and cipherSuites into the ConfigMap's servingInfo section.
func TestConfigMapTLSInjection(oc *exutil.CLI, ctx context.Context, t TLSTarget) {
	// Determine the namespace for the ConfigMap (defaults to target namespace).
	cmNamespace := t.ConfigMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.Namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.ConfigMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.ConfigMapName))

	g.By("verifying config.openshift.io/inject-tls annotation is present")
	injectTLSAnnotation, found := cm.Annotations["config.openshift.io/inject-tls"]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing config.openshift.io/inject-tls annotation", cmNamespace, t.ConfigMapName))
	o.Expect(injectTLSAnnotation).To(o.Equal("true"),
		fmt.Sprintf("ConfigMap %s/%s has inject-tls annotation but value is not 'true': %s", cmNamespace, t.ConfigMapName, injectTLSAnnotation))
	e2e.Logf("ConfigMap %s/%s has config.openshift.io/inject-tls=true annotation", cmNamespace, t.ConfigMapName)

	// Get the config key (defaults to "config.yaml" if not specified).
	configKey := t.ConfigMapKey
	if configKey == "" {
		configKey = "config.yaml"
	}

	// Extract the config data from the ConfigMap.
	g.By(fmt.Sprintf("extracting %s from ConfigMap data", configKey))
	configData, found := cm.Data[configKey]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing %s key", cmNamespace, t.ConfigMapName, configKey))
	o.Expect(configData).NotTo(o.BeEmpty(),
		fmt.Sprintf("ConfigMap %s/%s has empty %s", cmNamespace, t.ConfigMapName, configKey))

	// Log the servingInfo section for debugging.
	e2e.Logf("ConfigMap %s/%s %s content (servingInfo section):", cmNamespace, t.ConfigMapName, configKey)
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
		fmt.Sprintf("ConfigMap %s/%s config does not contain minTLSVersion", cmNamespace, t.ConfigMapName))

	// Extract actual minTLSVersion for logging.
	actualMinTLSVersion := "unknown"
	if strings.Contains(configData, "VersionTLS13") {
		actualMinTLSVersion = "VersionTLS13"
	} else if strings.Contains(configData, "VersionTLS12") {
		actualMinTLSVersion = "VersionTLS12"
	}
	e2e.Logf("ConfigMap %s/%s actual minTLSVersion: %s", cmNamespace, t.ConfigMapName, actualMinTLSVersion)

	g.By("verifying servingInfo.cipherSuites in ConfigMap config")
	o.Expect(configData).To(o.ContainSubstring("cipherSuites"),
		fmt.Sprintf("ConfigMap %s/%s config does not contain cipherSuites", cmNamespace, t.ConfigMapName))

	// Count cipher suites for logging.
	cipherCount := strings.Count(configData, "- TLS_") + strings.Count(configData, "- ECDHE")
	e2e.Logf("ConfigMap %s/%s cipherSuites count: %d", cmNamespace, t.ConfigMapName, cipherCount)

	// Cross-check against the cluster APIServer profile.
	g.By("cross-checking ConfigMap TLS config with cluster APIServer TLS profile")
	expectedMinVersion, profileType := getExpectedMinTLSVersionWithType(oc, ctx)
	e2e.Logf("Cluster TLS profile: %s, expected minTLSVersion: %s", profileType, expectedMinVersion)
	e2e.Logf("ConfigMap actual minTLSVersion: %s, expected: %s", actualMinTLSVersion, expectedMinVersion)

	o.Expect(configData).To(o.ContainSubstring(expectedMinVersion),
		fmt.Sprintf("ConfigMap %s/%s config does not contain expected minTLSVersion=%s (actual=%s, profile=%s)",
			cmNamespace, t.ConfigMapName, expectedMinVersion, actualMinTLSVersion, profileType))

	e2e.Logf("PASS: ConfigMap %s/%s has TLS config injected matching cluster profile (profile=%s, minTLSVersion=%s, cipherSuites=%d)",
		cmNamespace, t.ConfigMapName, profileType, expectedMinVersion, cipherCount)
}

// TestAnnotationRestorationAfterDeletion verifies that if the inject-tls annotation
// is deleted from the ConfigMap, the operator restores it.
func TestAnnotationRestorationAfterDeletion(oc *exutil.CLI, ctx context.Context, t TLSTarget) {
	cmNamespace := t.ConfigMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.Namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	// Get the original ConfigMap and verify annotation exists.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.ConfigMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.ConfigMapName))

	_, found := cm.Annotations["config.openshift.io/inject-tls"]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing config.openshift.io/inject-tls annotation", cmNamespace, t.ConfigMapName))

	// Delete the annotation.
	g.By("deleting config.openshift.io/inject-tls annotation")
	delete(cm.Annotations, "config.openshift.io/inject-tls")
	_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to update ConfigMap %s/%s to delete annotation", cmNamespace, t.ConfigMapName))
	e2e.Logf("Deleted inject-tls annotation from ConfigMap %s/%s", cmNamespace, t.ConfigMapName)

	// Wait for the operator to restore the annotation.
	g.By("waiting for operator to restore the inject-tls annotation")
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
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
		fmt.Sprintf("inject-tls annotation was not restored on ConfigMap %s/%s within timeout", cmNamespace, t.ConfigMapName))

	e2e.Logf("PASS: inject-tls annotation was restored after deletion on ConfigMap %s/%s", cmNamespace, t.ConfigMapName)
}

// TestAnnotationRestorationWhenFalse verifies that if the inject-tls annotation
// is set to "false", the operator restores it to "true".
func TestAnnotationRestorationWhenFalse(oc *exutil.CLI, ctx context.Context, t TLSTarget) {
	cmNamespace := t.ConfigMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.Namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	// Get the original ConfigMap.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.ConfigMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.ConfigMapName))

	_, annotationFound := cm.Annotations["config.openshift.io/inject-tls"]
	o.Expect(annotationFound).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing config.openshift.io/inject-tls annotation", cmNamespace, t.ConfigMapName))

	// Set the annotation to "false".
	g.By("setting config.openshift.io/inject-tls annotation to 'false'")
	cm.Annotations["config.openshift.io/inject-tls"] = "false"
	_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to update ConfigMap %s/%s to set annotation to false", cmNamespace, t.ConfigMapName))
	e2e.Logf("Set inject-tls annotation to 'false' on ConfigMap %s/%s", cmNamespace, t.ConfigMapName)

	// Wait for the operator to restore the annotation to "true".
	g.By("waiting for operator to restore the inject-tls annotation to 'true'")
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
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
		fmt.Sprintf("inject-tls annotation was not restored to 'true' on ConfigMap %s/%s within timeout", cmNamespace, t.ConfigMapName))

	e2e.Logf("PASS: inject-tls annotation was restored to 'true' after being set to 'false' on ConfigMap %s/%s", cmNamespace, t.ConfigMapName)
}

// TestServingInfoRestorationAfterRemoval verifies that if the servingInfo section
// is removed from the ConfigMap, the operator restores it with correct TLS settings.
func TestServingInfoRestorationAfterRemoval(oc *exutil.CLI, ctx context.Context, t TLSTarget) {
	cmNamespace := t.ConfigMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.Namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	configKey := t.ConfigMapKey
	if configKey == "" {
		configKey = "config.yaml"
	}

	// Get the original ConfigMap and verify servingInfo exists.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.ConfigMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.ConfigMapName))

	// Verify servingInfo exists before we remove it.
	configData := cm.Data[configKey]
	if !strings.Contains(configData, "servingInfo") {
		g.Skip(fmt.Sprintf("ConfigMap %s/%s does not have servingInfo, skipping removal test", cmNamespace, t.ConfigMapName))
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
		fmt.Sprintf("failed to update ConfigMap %s/%s to remove servingInfo", cmNamespace, t.ConfigMapName))
	e2e.Logf("Removed servingInfo from ConfigMap %s/%s", cmNamespace, t.ConfigMapName)

	// Wait for the operator to restore servingInfo.
	g.By("waiting for operator to restore servingInfo section")
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
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
		fmt.Sprintf("servingInfo was not restored on ConfigMap %s/%s within timeout", cmNamespace, t.ConfigMapName))

	// Verify the restored config matches expected TLS version.
	cm, err = oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	configData = cm.Data[configKey]
	o.Expect(configData).To(o.ContainSubstring("minTLSVersion"),
		"restored servingInfo should contain minTLSVersion")

	e2e.Logf("PASS: servingInfo was restored after removal on ConfigMap %s/%s", cmNamespace, t.ConfigMapName)
}

// TestServingInfoRestorationAfterModification verifies that if the servingInfo
// minTLSVersion is modified to an incorrect value, the operator restores it.
func TestServingInfoRestorationAfterModification(oc *exutil.CLI, ctx context.Context, t TLSTarget) {
	cmNamespace := t.ConfigMapNamespace
	if cmNamespace == "" {
		cmNamespace = t.Namespace
	}

	g.By(fmt.Sprintf("verifying namespace %s exists", cmNamespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, cmNamespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", cmNamespace))

	configKey := t.ConfigMapKey
	if configKey == "" {
		configKey = "config.yaml"
	}

	// Get the expected TLS version from the cluster profile.
	expectedMinVersion := getExpectedMinTLSVersion(oc, ctx)
	e2e.Logf("Expected minTLSVersion from cluster profile: %s", expectedMinVersion)

	// Get the original ConfigMap.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.ConfigMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.ConfigMapName))

	// Verify servingInfo exists.
	configData := cm.Data[configKey]
	if !strings.Contains(configData, "minTLSVersion") {
		g.Skip(fmt.Sprintf("ConfigMap %s/%s does not have minTLSVersion, skipping modification test", cmNamespace, t.ConfigMapName))
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
		fmt.Sprintf("failed to update ConfigMap %s/%s to modify minTLSVersion", cmNamespace, t.ConfigMapName))
	e2e.Logf("Modified minTLSVersion to '%s' on ConfigMap %s/%s", wrongValue, cmNamespace, t.ConfigMapName)

	// Wait for the operator to restore correct minTLSVersion.
	g.By("waiting for operator to restore correct minTLSVersion")
	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
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
			cmNamespace, t.ConfigMapName, expectedMinVersion))

	e2e.Logf("PASS: minTLSVersion was restored to '%s' after modification on ConfigMap %s/%s",
		expectedMinVersion, cmNamespace, t.ConfigMapName)
}

// TestDeploymentTLSEnvVars verifies that the deployment in the given namespace
// has TLS environment variables that match the expected TLS profile.
func TestDeploymentTLSEnvVars(oc *exutil.CLI, ctx context.Context, t TLSTarget) {
	g.By("getting cluster APIServer TLS profile")
	expectedMinVersion := getExpectedMinTLSVersion(oc, ctx)
	e2e.Logf("Expected minTLSVersion from cluster profile: %s", expectedMinVersion)

	g.By(fmt.Sprintf("verifying namespace %s exists", t.Namespace))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, t.Namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", t.Namespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", t.Namespace))

	g.By(fmt.Sprintf("getting deployment %s/%s", t.Namespace, t.DeploymentName))
	deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.Namespace).Get(
		ctx, t.DeploymentName, metav1.GetOptions{},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get deployment %s/%s", t.Namespace, t.DeploymentName))
	o.Expect(deployment.Spec.Template.Spec.Containers).NotTo(o.BeEmpty(),
		fmt.Sprintf("deployment %s/%s has no containers", t.Namespace, t.DeploymentName))

	e2e.Logf("Deployment %s/%s: generation=%d, observedGeneration=%d, replicas=%d/%d",
		t.Namespace, t.DeploymentName,
		deployment.Generation, deployment.Status.ObservedGeneration,
		deployment.Status.ReadyReplicas, deployment.Status.Replicas)

	g.By(fmt.Sprintf("verifying %s env var in deployment containers", t.TLSMinVersionEnvVar))
	envMap := exutil.FindEnvAcrossContainers(deployment.Spec.Template.Spec.Containers, t.TLSMinVersionEnvVar)
	exutil.LogEnvVars(envMap, t.TLSMinVersionEnvVar)

	o.Expect(envMap).To(o.HaveKey(t.TLSMinVersionEnvVar),
		fmt.Sprintf("expected %s to be set in deployment %s/%s (checked all %d containers)",
			t.TLSMinVersionEnvVar, t.Namespace, t.DeploymentName, len(deployment.Spec.Template.Spec.Containers)))
	o.Expect(envMap[t.TLSMinVersionEnvVar]).To(o.Equal(expectedMinVersion),
		fmt.Sprintf("expected %s=%s in deployment %s/%s, got %s",
			t.TLSMinVersionEnvVar, expectedMinVersion, t.Namespace, t.DeploymentName,
			envMap[t.TLSMinVersionEnvVar]))
	e2e.Logf("PASS: %s=%s matches cluster TLS profile in %s/%s",
		t.TLSMinVersionEnvVar, expectedMinVersion, t.Namespace, t.DeploymentName)

	// Verify cipher suites env var if configured for this target.
	if t.CipherSuitesEnvVar != "" {
		g.By(fmt.Sprintf("verifying %s env var in deployment containers", t.CipherSuitesEnvVar))
		o.Expect(envMap).To(o.HaveKey(t.CipherSuitesEnvVar),
			fmt.Sprintf("expected %s to be set in deployment %s/%s (checked all %d containers)",
				t.CipherSuitesEnvVar, t.Namespace, t.DeploymentName, len(deployment.Spec.Template.Spec.Containers)))
		o.Expect(envMap[t.CipherSuitesEnvVar]).NotTo(o.BeEmpty(),
			fmt.Sprintf("expected %s to have a value in deployment %s/%s",
				t.CipherSuitesEnvVar, t.Namespace, t.DeploymentName))
		e2e.Logf("PASS: %s is set in %s/%s (value length=%d)",
			t.CipherSuitesEnvVar, t.Namespace, t.DeploymentName, len(envMap[t.CipherSuitesEnvVar]))
	}
}

// TestWireLevelTLS verifies that the service endpoint in the given namespace
// enforces the TLS version from the cluster APIServer profile using
// oc port-forward for connectivity.
func TestWireLevelTLS(oc *exutil.CLI, ctx context.Context, t TLSTarget) {
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
	_, err = oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, t.Namespace, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", t.Namespace))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", t.Namespace))

	if t.DeploymentName != "" {
		g.By(fmt.Sprintf("waiting for deployment %s/%s to be fully rolled out", t.Namespace, t.DeploymentName))
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.Namespace).Get(ctx, t.DeploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("failed to get deployment %s/%s", t.Namespace, t.DeploymentName))
		err = exutil.WaitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, OperatorRolloutTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("deployment %s/%s did not complete rollout (timeout: %v)", t.Namespace, t.DeploymentName, OperatorRolloutTimeout))
	}

	g.By(fmt.Sprintf("verifying TLS behavior via port-forward to svc/%s in %s on port %s",
		t.ServiceName, t.Namespace, t.ServicePort))
	err = exutil.ForwardPortAndExecute(t.ServiceName, t.Namespace, t.ServicePort,
		func(localPort int) error {
			return exutil.CheckTLSConnection(localPort, tlsShouldWork, tlsShouldNotWork, t.ServiceName, t.Namespace)
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("wire-level TLS test failed for svc/%s in %s:%s (profile=%s)",
			t.ServiceName, t.Namespace, t.ServicePort, profileType))
	e2e.Logf("PASS: wire-level TLS verified for svc/%s in %s:%s (profile=%s)",
		t.ServiceName, t.Namespace, t.ServicePort, profileType)
}

// ── Target-aware helpers ───────────────────────────────────────────────────
// Generic helpers (ForwardPortAndExecute, CheckTLSConnection,
// WaitForDeploymentCompleteWithTimeout, FindEnvAcrossContainers,
// WaitForClusterOperatorStable, etc.) live in test/extended/util/tls_helpers.go.

// VerifyObservedConfigAfterSwitch checks every target with an operator config
// for correct ObservedConfig after a TLS profile switch.
func VerifyObservedConfigAfterSwitch(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string) {
	VerifyObservedConfigForTargets(oc, ctx, expectedVersion, profileLabel, Targets)
}

// VerifyObservedConfigForTargets checks a specific list of targets for
// ObservedConfig correctness after a TLS profile switch.
func VerifyObservedConfigForTargets(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string, targetList []TLSTarget) {
	dynClient := oc.AdminDynamicClient()
	for _, t := range targetList {
		if t.OperatorConfigGVR.Resource == "" || t.OperatorConfigName == "" {
			continue
		}
		resource, err := dynClient.Resource(t.OperatorConfigGVR).Get(ctx, t.OperatorConfigName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("failed to get operator config %s/%s after %s switch",
				t.OperatorConfigGVR.Resource, t.OperatorConfigName, profileLabel))

		observedConfigRaw, found, err := unstructured.NestedMap(resource.Object, "spec", "observedConfig")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(found).To(o.BeTrue(),
			fmt.Sprintf("expected spec.observedConfig in %s/%s after %s switch",
				t.OperatorConfigGVR.Resource, t.OperatorConfigName, profileLabel))

		minTLSVersion, found, err := unstructured.NestedString(observedConfigRaw, "servingInfo", "minTLSVersion")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(found).To(o.BeTrue(),
			fmt.Sprintf("expected servingInfo.minTLSVersion in ObservedConfig of %s/%s after %s switch",
				t.OperatorConfigGVR.Resource, t.OperatorConfigName, profileLabel))
		o.Expect(minTLSVersion).To(o.Equal(expectedVersion),
			fmt.Sprintf("ObservedConfig %s/%s: expected minTLSVersion=%s after %s switch, got %s",
				t.OperatorConfigGVR.Resource, t.OperatorConfigName, expectedVersion, profileLabel, minTLSVersion))
		e2e.Logf("PASS: ObservedConfig %s/%s has minTLSVersion=%s after %s switch",
			t.OperatorConfigGVR.Resource, t.OperatorConfigName, minTLSVersion, profileLabel)
	}
}

// VerifyConfigMapsAfterSwitch checks every target with a ConfigMap for
// correct TLS injection after a profile switch.
func VerifyConfigMapsAfterSwitch(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string) {
	VerifyConfigMapsForTargets(oc, ctx, expectedVersion, profileLabel, Targets)
}

// VerifyConfigMapsForTargets checks a specific list of targets for
// ConfigMap TLS injection correctness after a TLS profile switch.
func VerifyConfigMapsForTargets(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string, targetList []TLSTarget) {
	for _, t := range targetList {
		if t.ConfigMapName == "" {
			continue
		}
		cmNamespace := t.ConfigMapNamespace
		if cmNamespace == "" {
			cmNamespace = t.Namespace
		}
		cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("SKIP: ConfigMap %s/%s not found: %v", cmNamespace, t.ConfigMapName, err)
			continue
		}
		configKey := t.ConfigMapKey
		if configKey == "" {
			configKey = "config.yaml"
		}
		configData := cm.Data[configKey]
		o.Expect(cm.Annotations).To(o.HaveKey("config.openshift.io/inject-tls"),
			fmt.Sprintf("ConfigMap %s/%s is missing config.openshift.io/inject-tls annotation", cmNamespace, t.ConfigMapName))
		o.Expect(configData).To(o.ContainSubstring(expectedVersion),
			fmt.Sprintf("ConfigMap %s/%s should have %s after %s switch",
				cmNamespace, t.ConfigMapName, expectedVersion, profileLabel))
		e2e.Logf("PASS: ConfigMap %s/%s has %s after %s switch",
			cmNamespace, t.ConfigMapName, expectedVersion, profileLabel)
	}
}

func targetClusterOperators() []string {
	seen := map[string]bool{}
	var result []string
	for _, t := range Targets {
		if t.ClusterOperatorName == "" || seen[t.ClusterOperatorName] {
			continue
		}
		seen[t.ClusterOperatorName] = true
		result = append(result, t.ClusterOperatorName)
	}
	return result
}

func getExpectedMinTLSVersion(oc *exutil.CLI, ctx context.Context) string {
	minVersion, _ := getExpectedMinTLSVersionWithType(oc, ctx)
	return minVersion
}

func getExpectedMinTLSVersionWithType(oc *exutil.CLI, ctx context.Context) (string, string) {
	config, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	profileType := configv1.TLSProfileIntermediateType
	if config.Spec.TLSSecurityProfile != nil {
		profileType = config.Spec.TLSSecurityProfile.Type
	}

	var minVersion string
	profileName := string(profileType)

	if profileType == configv1.TLSProfileCustomType {
		if config.Spec.TLSSecurityProfile.Custom != nil {
			minVersion = string(config.Spec.TLSSecurityProfile.Custom.MinTLSVersion)
		}
		if minVersion == "" {
			minVersion = string(configv1.VersionTLS12)
		}
	} else {
		profile, ok := configv1.TLSProfiles[profileType]
		if !ok {
			e2e.Failf("Unknown TLS profile type: %s", profileType)
		}
		minVersion = string(profile.MinTLSVersion)
	}

	if profileType == "" || profileType == configv1.TLSProfileIntermediateType {
		profileName = "Intermediate (default)"
	}

	e2e.Logf("Cluster APIServer TLS profile: type=%s, minTLSVersion=%s", profileName, minVersion)
	return minVersion, profileName
}

// WaitForAllOperatorsAfterTLSChange waits for all target ClusterOperators to
// stabilize and for all target Deployments to complete rollout.
func WaitForAllOperatorsAfterTLSChange(oc *exutil.CLI, ctx context.Context, profileLabel string) {
	e2e.Logf("Waiting 30s for operators to begin processing %s profile change", profileLabel)
	time.Sleep(30 * time.Second)

	e2e.Logf("Waiting for all ClusterOperators to stabilize after %s profile change", profileLabel)
	for _, co := range targetClusterOperators() {
		e2e.Logf("Waiting for ClusterOperator %s to stabilize after %s switch", co, profileLabel)
		exutil.WaitForClusterOperatorStable(oc, ctx, co)
	}

	for _, t := range Targets {
		if t.DeploymentName == "" {
			continue
		}
		e2e.Logf("Waiting for deployment %s/%s to complete rollout after %s switch", t.Namespace, t.DeploymentName, profileLabel)
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.Namespace).Get(ctx, t.DeploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, OperatorRolloutTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("deployment %s/%s did not complete rollout after %s TLS change (timeout: %v)",
				t.Namespace, t.DeploymentName, profileLabel, OperatorRolloutTimeout))
		e2e.Logf("Deployment %s/%s is fully rolled out after %s switch", t.Namespace, t.DeploymentName, profileLabel)
	}
	e2e.Logf("All operators and deployments are stable after %s profile change", profileLabel)
}
