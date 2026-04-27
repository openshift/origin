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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	OperatorRolloutTimeout = 25 * time.Minute
	InjectTLSAnnotation    = "config.openshift.io/inject-tls"
)

// ─── Target types ──────────────────────────────────────────────────────────
// Each type captures only the fields its test function needs, making the
// function signatures self-documenting and avoiding "what is this for?"
// confusion that comes with a monolithic struct with many unused fields.

// ObservedConfigTarget identifies an operator whose spec.observedConfig
// should contain TLS servingInfo settings.
type ObservedConfigTarget struct {
	Namespace          string
	OperatorConfigGVR  schema.GroupVersionResource
	OperatorConfigName string
	ControlPlane       bool
}

// ConfigMapTarget identifies a ConfigMap into which CVO injects TLS config
// via the config.openshift.io/inject-tls annotation.
type ConfigMapTarget struct {
	Namespace          string
	ConfigMapName      string
	ConfigMapNamespace string // if empty, Namespace is used
	ConfigMapKey       string // if empty, "config.yaml" is used
}

func (t ConfigMapTarget) ResolvedNamespace() string {
	if t.ConfigMapNamespace != "" {
		return t.ConfigMapNamespace
	}
	return t.Namespace
}

func (t ConfigMapTarget) ResolvedKey() string {
	if t.ConfigMapKey != "" {
		return t.ConfigMapKey
	}
	return "config.yaml"
}

// DeploymentEnvVarTarget identifies a deployment whose containers should
// carry TLS-related environment variables.
type DeploymentEnvVarTarget struct {
	Namespace           string
	DeploymentName      string
	TLSMinVersionEnvVar string
	CipherSuitesEnvVar  string
}

// ServiceTarget identifies a service endpoint that must enforce the
// cluster-wide TLS profile at the wire level.
type ServiceTarget struct {
	Namespace      string
	DeploymentName string // for rollout wait; empty for static pods
	ServiceName    string
	ServicePort    string
	ControlPlane   bool
}

// DeploymentRolloutTarget identifies a deployment to wait for after a TLS
// profile change.
type DeploymentRolloutTarget struct {
	Namespace      string
	DeploymentName string
	ControlPlane   bool
}

// ─── Target lists ──────────────────────────────────────────────────────────
// Each list contains exactly the targets applicable to that test category.
// No empty-field filtering is needed in test loops.

var ObservedConfigTargets = []ObservedConfigTarget{
	{Namespace: "openshift-image-registry", OperatorConfigGVR: schema.GroupVersionResource{
		Group: "imageregistry.operator.openshift.io", Version: "v1", Resource: "configs",
	}, OperatorConfigName: "cluster"},
	{Namespace: "openshift-controller-manager", OperatorConfigGVR: schema.GroupVersionResource{
		Group: "operator.openshift.io", Version: "v1", Resource: "openshiftcontrollermanagers",
	}, OperatorConfigName: "cluster", ControlPlane: true},
	{Namespace: "openshift-kube-apiserver", OperatorConfigGVR: schema.GroupVersionResource{
		Group: "operator.openshift.io", Version: "v1", Resource: "kubeapiservers",
	}, OperatorConfigName: "cluster", ControlPlane: true},
	{Namespace: "openshift-apiserver", OperatorConfigGVR: schema.GroupVersionResource{
		Group: "operator.openshift.io", Version: "v1", Resource: "openshiftapiservers",
	}, OperatorConfigName: "cluster", ControlPlane: true},
	{Namespace: "openshift-etcd", OperatorConfigGVR: schema.GroupVersionResource{
		Group: "operator.openshift.io", Version: "v1", Resource: "etcds",
	}, OperatorConfigName: "cluster", ControlPlane: true},
	{Namespace: "openshift-kube-controller-manager", OperatorConfigGVR: schema.GroupVersionResource{
		Group: "operator.openshift.io", Version: "v1", Resource: "kubecontrollermanagers",
	}, OperatorConfigName: "cluster", ControlPlane: true},
	{Namespace: "openshift-kube-scheduler", OperatorConfigGVR: schema.GroupVersionResource{
		Group: "operator.openshift.io", Version: "v1", Resource: "kubeschedulers",
	}, OperatorConfigName: "cluster", ControlPlane: true},
}

var ConfigMapTargets = []ConfigMapTarget{
	{Namespace: "openshift-image-registry", ConfigMapName: "image-registry-operator-config", ConfigMapKey: "config.yaml"},
	{Namespace: "openshift-controller-manager", ConfigMapName: "openshift-controller-manager-operator-config",
		ConfigMapNamespace: "openshift-controller-manager-operator", ConfigMapKey: "config.yaml"},
	{Namespace: "openshift-kube-apiserver", ConfigMapName: "kube-apiserver-operator-config",
		ConfigMapNamespace: "openshift-kube-apiserver-operator", ConfigMapKey: "config.yaml"},
	{Namespace: "openshift-apiserver", ConfigMapName: "openshift-apiserver-operator-config",
		ConfigMapNamespace: "openshift-apiserver-operator", ConfigMapKey: "config.yaml"},
	{Namespace: "openshift-etcd", ConfigMapName: "etcd-operator-config",
		ConfigMapNamespace: "openshift-etcd-operator", ConfigMapKey: "config.yaml"},
	{Namespace: "openshift-kube-controller-manager", ConfigMapName: "kube-controller-manager-operator-config",
		ConfigMapNamespace: "openshift-kube-controller-manager-operator", ConfigMapKey: "config.yaml"},
	{Namespace: "openshift-kube-scheduler", ConfigMapName: "openshift-kube-scheduler-operator-config",
		ConfigMapNamespace: "openshift-kube-scheduler-operator", ConfigMapKey: "config.yaml"},
	{Namespace: "openshift-cluster-samples-operator", ConfigMapName: "samples-operator-config", ConfigMapKey: "config.yaml"},
}

var DeploymentEnvVarTargets = []DeploymentEnvVarTarget{
	{Namespace: "openshift-image-registry", DeploymentName: "image-registry",
		TLSMinVersionEnvVar: "REGISTRY_HTTP_TLS_MINVERSION", CipherSuitesEnvVar: "OPENSHIFT_REGISTRY_HTTP_TLS_CIPHERSUITES"},
}

var ServiceTargets = []ServiceTarget{
	{Namespace: "openshift-image-registry", DeploymentName: "image-registry", ServiceName: "image-registry", ServicePort: "5000"},
	{Namespace: "openshift-image-registry", ServiceName: "image-registry-operator", ServicePort: "60000", ControlPlane: true},
	{Namespace: "openshift-controller-manager", DeploymentName: "controller-manager", ServiceName: "controller-manager", ServicePort: "443", ControlPlane: true},
	{Namespace: "openshift-kube-apiserver", ServiceName: "apiserver", ServicePort: "443", ControlPlane: true},
	{Namespace: "openshift-kube-apiserver", ServiceName: "apiserver", ServicePort: "17697", ControlPlane: true},
	{Namespace: "openshift-apiserver", DeploymentName: "apiserver", ServiceName: "api", ServicePort: "443", ControlPlane: true},
	{Namespace: "openshift-apiserver", ServiceName: "check-endpoints", ServicePort: "17698", ControlPlane: true},
	{Namespace: "openshift-etcd", ServiceName: "etcd", ServicePort: "2379", ControlPlane: true},
	{Namespace: "openshift-kube-controller-manager", ServiceName: "kube-controller-manager", ServicePort: "443", ControlPlane: true},
	{Namespace: "openshift-kube-scheduler", ServiceName: "scheduler", ServicePort: "443", ControlPlane: true},
	{Namespace: "openshift-cluster-samples-operator", DeploymentName: "cluster-samples-operator", ServiceName: "metrics", ServicePort: "60000"},
}

var ClusterOperatorNames = []string{
	"image-registry",
	"openshift-controller-manager",
	"kube-apiserver",
	"openshift-apiserver",
	"etcd",
	"kube-controller-manager",
	"kube-scheduler",
	"openshift-samples",
}

var DeploymentRolloutTargets = []DeploymentRolloutTarget{
	{Namespace: "openshift-image-registry", DeploymentName: "image-registry"},
	{Namespace: "openshift-controller-manager", DeploymentName: "controller-manager", ControlPlane: true},
	{Namespace: "openshift-apiserver", DeploymentName: "apiserver", ControlPlane: true},
	{Namespace: "openshift-cluster-version", DeploymentName: "cluster-version-operator", ControlPlane: true},
	{Namespace: "openshift-cluster-samples-operator", DeploymentName: "cluster-samples-operator"},
}

// ─── Shared helpers ────────────────────────────────────────────────────────

// resolveAndGetConfigMap checks that the ConfigMap's namespace exists, then
// fetches and returns the ConfigMap. Skips the test if the namespace is absent.
func resolveAndGetConfigMap(oc *exutil.CLI, ctx context.Context, t ConfigMapTarget) (*configMapContext, error) {
	ns := t.ResolvedNamespace()

	g.By(fmt.Sprintf("verifying namespace %s exists", ns))
	_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", ns))
	}
	o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unexpected error checking namespace %s", ns))

	g.By(fmt.Sprintf("getting ConfigMap %s/%s", ns, t.ConfigMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(ns).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", ns, t.ConfigMapName))

	return &configMapContext{oc: oc, ctx: ctx, namespace: ns, name: t.ConfigMapName, cm: cm}, nil
}

type configMapContext struct {
	oc        *exutil.CLI
	ctx       context.Context
	namespace string
	name      string
	cm        *corev1.ConfigMap
}

// requireAnnotation asserts the inject-tls annotation exists on the ConfigMap.
func (c *configMapContext) requireAnnotation() {
	_, found := c.cm.Annotations[InjectTLSAnnotation]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing %s annotation", c.namespace, c.name, InjectTLSAnnotation))
}

// waitForAnnotationValue polls until the inject-tls annotation has the expected value.
func (c *configMapContext) waitForAnnotationValue(expected string) {
	g.By(fmt.Sprintf("waiting for %s annotation to become %q", InjectTLSAnnotation, expected))
	err := wait.PollUntilContextTimeout(c.ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := c.oc.AdminKubeClient().CoreV1().ConfigMaps(c.namespace).Get(ctx, c.name, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}
			val, found := cm.Annotations[InjectTLSAnnotation]
			if found && val == expected {
				e2e.Logf("  poll: annotation restored to %q", expected)
				return true, nil
			}
			e2e.Logf("  poll: annotation not yet restored (found=%v, val=%s)", found, val)
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("%s annotation was not restored to %q on ConfigMap %s/%s within timeout",
			InjectTLSAnnotation, expected, c.namespace, c.name))
}

// update writes the ConfigMap back to the cluster.
func (c *configMapContext) update() {
	var err error
	c.cm, err = c.oc.AdminKubeClient().CoreV1().ConfigMaps(c.namespace).Update(c.ctx, c.cm, metav1.UpdateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to update ConfigMap %s/%s", c.namespace, c.name))
}

// ─── Test implementations ──────────────────────────────────────────────────

// TestObservedConfig verifies that the operator's ObservedConfig contains
// a properly populated servingInfo with minTLSVersion and cipherSuites.
func TestObservedConfig(oc *exutil.CLI, ctx context.Context, t ObservedConfigTarget, isHyperShift bool) {
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

	observedConfigRaw, found, err := unstructured.NestedMap(resource.Object, "spec", "observedConfig")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to extract spec.observedConfig")
	if isHyperShift && t.ControlPlane && (!found || len(observedConfigRaw) == 0) {
		g.Skip(fmt.Sprintf("Operator config %s/%s exists on HyperShift guest but spec.observedConfig is not populated (operator runs on management cluster)",
			t.OperatorConfigGVR.Resource, t.OperatorConfigName))
	}
	o.Expect(found).To(o.BeTrue(), "expected spec.observedConfig to exist")
	o.Expect(observedConfigRaw).NotTo(o.BeEmpty(), "expected spec.observedConfig to be non-empty")

	observedJSON, _ := json.MarshalIndent(observedConfigRaw, "", "  ")
	e2e.Logf("ObservedConfig:\n%s", string(observedJSON))

	g.By("verifying servingInfo in ObservedConfig")
	_, found, err = unstructured.NestedMap(observedConfigRaw, "servingInfo")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get servingInfo from observedConfig")
	o.Expect(found).To(o.BeTrue(), "expected servingInfo in ObservedConfig")

	g.By("verifying servingInfo.minTLSVersion in ObservedConfig")
	minTLSVersion, found, err := unstructured.NestedString(observedConfigRaw, "servingInfo", "minTLSVersion")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get servingInfo.minTLSVersion")
	o.Expect(found).To(o.BeTrue(), "expected minTLSVersion in servingInfo")
	o.Expect(minTLSVersion).NotTo(o.BeEmpty(), "expected minTLSVersion to be non-empty")
	e2e.Logf("ObservedConfig servingInfo.minTLSVersion: %s", minTLSVersion)

	g.By("verifying servingInfo.cipherSuites in ObservedConfig")
	cipherSuites, found, err := unstructured.NestedStringSlice(observedConfigRaw, "servingInfo", "cipherSuites")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get servingInfo.cipherSuites")
	o.Expect(found).To(o.BeTrue(), "expected cipherSuites in servingInfo")
	o.Expect(cipherSuites).NotTo(o.BeEmpty(), "expected cipherSuites to be non-empty")
	e2e.Logf("ObservedConfig servingInfo.cipherSuites: %d suites", len(cipherSuites))

	g.By("cross-checking ObservedConfig with cluster APIServer TLS profile")
	expectedMinVersion := GetExpectedMinTLSVersion(oc, ctx)
	o.Expect(minTLSVersion).To(o.Equal(expectedMinVersion),
		fmt.Sprintf("ObservedConfig minTLSVersion=%s does not match cluster profile=%s",
			minTLSVersion, expectedMinVersion))
	e2e.Logf("PASS: ObservedConfig for %s/%s matches cluster APIServer TLS profile",
		t.OperatorConfigGVR.Resource, t.OperatorConfigName)
}

// TestConfigMapTLSInjection verifies that CVO has injected TLS configuration
// into the operator's ConfigMap via the inject-tls annotation.
func TestConfigMapTLSInjection(oc *exutil.CLI, ctx context.Context, t ConfigMapTarget) {
	cmCtx, _ := resolveAndGetConfigMap(oc, ctx, t)

	g.By("verifying " + InjectTLSAnnotation + " annotation is present")
	injectTLSAnnotation, found := cmCtx.cm.Annotations[InjectTLSAnnotation]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing %s annotation", cmCtx.namespace, t.ConfigMapName, InjectTLSAnnotation))
	o.Expect(injectTLSAnnotation).To(o.Equal("true"),
		fmt.Sprintf("ConfigMap %s/%s has %s annotation but value is not 'true': %s",
			cmCtx.namespace, t.ConfigMapName, InjectTLSAnnotation, injectTLSAnnotation))
	e2e.Logf("ConfigMap %s/%s has %s=true annotation", cmCtx.namespace, t.ConfigMapName, InjectTLSAnnotation)

	configKey := t.ResolvedKey()

	g.By(fmt.Sprintf("extracting %s from ConfigMap data", configKey))
	configData, found := cmCtx.cm.Data[configKey]
	o.Expect(found).To(o.BeTrue(),
		fmt.Sprintf("ConfigMap %s/%s is missing %s key", cmCtx.namespace, t.ConfigMapName, configKey))
	o.Expect(configData).NotTo(o.BeEmpty(),
		fmt.Sprintf("ConfigMap %s/%s has empty %s", cmCtx.namespace, t.ConfigMapName, configKey))

	for _, line := range strings.Split(configData, "\n") {
		if strings.Contains(line, "servingInfo") ||
			strings.Contains(line, "minTLSVersion") ||
			strings.Contains(line, "cipherSuites") ||
			strings.Contains(line, "bindAddress") ||
			(strings.HasPrefix(strings.TrimSpace(line), "- TLS_") || strings.HasPrefix(strings.TrimSpace(line), "- ECDHE")) {
			e2e.Logf("  %s", line)
		}
	}

	g.By("verifying servingInfo.minTLSVersion in ConfigMap config")
	o.Expect(configData).To(o.ContainSubstring("minTLSVersion"),
		fmt.Sprintf("ConfigMap %s/%s config does not contain minTLSVersion", cmCtx.namespace, t.ConfigMapName))

	actualMinTLSVersion := "unknown"
	if strings.Contains(configData, "VersionTLS13") {
		actualMinTLSVersion = "VersionTLS13"
	} else if strings.Contains(configData, "VersionTLS12") {
		actualMinTLSVersion = "VersionTLS12"
	}

	g.By("verifying servingInfo.cipherSuites in ConfigMap config")
	o.Expect(configData).To(o.ContainSubstring("cipherSuites"),
		fmt.Sprintf("ConfigMap %s/%s config does not contain cipherSuites", cmCtx.namespace, t.ConfigMapName))

	cipherCount := strings.Count(configData, "- TLS_") + strings.Count(configData, "- ECDHE")

	g.By("cross-checking ConfigMap TLS config with cluster APIServer TLS profile")
	expectedMinVersion, profileType := GetExpectedMinTLSVersionWithType(oc, ctx)

	o.Expect(configData).To(o.ContainSubstring(expectedMinVersion),
		fmt.Sprintf("ConfigMap %s/%s config does not contain expected minTLSVersion=%s (actual=%s, profile=%s)",
			cmCtx.namespace, t.ConfigMapName, expectedMinVersion, actualMinTLSVersion, profileType))

	e2e.Logf("PASS: ConfigMap %s/%s has TLS config injected matching cluster profile (profile=%s, minTLSVersion=%s, cipherSuites=%d)",
		cmCtx.namespace, t.ConfigMapName, profileType, expectedMinVersion, cipherCount)
}

// TestAnnotationRestorationAfterDeletion verifies that if the inject-tls
// annotation is deleted from the ConfigMap, the operator restores it.
func TestAnnotationRestorationAfterDeletion(oc *exutil.CLI, ctx context.Context, t ConfigMapTarget) {
	cmCtx, _ := resolveAndGetConfigMap(oc, ctx, t)
	cmCtx.requireAnnotation()

	g.By("deleting " + InjectTLSAnnotation + " annotation")
	delete(cmCtx.cm.Annotations, InjectTLSAnnotation)
	cmCtx.update()
	e2e.Logf("Deleted %s annotation from ConfigMap %s/%s", InjectTLSAnnotation, cmCtx.namespace, t.ConfigMapName)

	cmCtx.waitForAnnotationValue("true")
	e2e.Logf("PASS: %s annotation was restored after deletion on ConfigMap %s/%s",
		InjectTLSAnnotation, cmCtx.namespace, t.ConfigMapName)
}

// TestAnnotationRestorationWhenFalse verifies that if the inject-tls
// annotation is set to "false", the operator restores it to "true".
func TestAnnotationRestorationWhenFalse(oc *exutil.CLI, ctx context.Context, t ConfigMapTarget) {
	cmCtx, _ := resolveAndGetConfigMap(oc, ctx, t)
	cmCtx.requireAnnotation()

	g.By("setting " + InjectTLSAnnotation + " annotation to 'false'")
	cmCtx.cm.Annotations[InjectTLSAnnotation] = "false"
	cmCtx.update()
	e2e.Logf("Set %s annotation to 'false' on ConfigMap %s/%s", InjectTLSAnnotation, cmCtx.namespace, t.ConfigMapName)

	cmCtx.waitForAnnotationValue("true")
	e2e.Logf("PASS: %s annotation was restored to 'true' after being set to 'false' on ConfigMap %s/%s",
		InjectTLSAnnotation, cmCtx.namespace, t.ConfigMapName)
}

// TestServingInfoRestorationAfterRemoval verifies that if the servingInfo
// section is removed from the ConfigMap, the operator restores it.
func TestServingInfoRestorationAfterRemoval(oc *exutil.CLI, ctx context.Context, t ConfigMapTarget) {
	cmCtx, _ := resolveAndGetConfigMap(oc, ctx, t)
	configKey := t.ResolvedKey()

	configData := cmCtx.cm.Data[configKey]
	if !strings.Contains(configData, "servingInfo") {
		g.Skip(fmt.Sprintf("ConfigMap %s/%s does not have servingInfo, skipping removal test",
			cmCtx.namespace, t.ConfigMapName))
	}

	g.By("removing servingInfo section from ConfigMap")
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
				continue
			}
			inServingInfo = false
		}
		newLines = append(newLines, line)
	}
	cmCtx.cm.Data[configKey] = strings.Join(newLines, "\n")
	cmCtx.update()
	e2e.Logf("Removed servingInfo from ConfigMap %s/%s", cmCtx.namespace, t.ConfigMapName)

	g.By("waiting for operator to restore servingInfo section")
	err := wait.PollUntilContextTimeout(cmCtx.ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := cmCtx.oc.AdminKubeClient().CoreV1().ConfigMaps(cmCtx.namespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}
			data := cm.Data[configKey]
			if strings.Contains(data, "servingInfo") && strings.Contains(data, "minTLSVersion") {
				e2e.Logf("  poll: servingInfo restored!")
				return true, nil
			}
			e2e.Logf("  poll: servingInfo not yet restored")
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("servingInfo was not restored on ConfigMap %s/%s within timeout", cmCtx.namespace, t.ConfigMapName))

	cm, err := cmCtx.oc.AdminKubeClient().CoreV1().ConfigMaps(cmCtx.namespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(cm.Data[configKey]).To(o.ContainSubstring("minTLSVersion"),
		"restored servingInfo should contain minTLSVersion")

	e2e.Logf("PASS: servingInfo was restored after removal on ConfigMap %s/%s", cmCtx.namespace, t.ConfigMapName)
}

// TestServingInfoRestorationAfterModification verifies that if the servingInfo
// minTLSVersion is modified to an incorrect value, the operator restores it.
func TestServingInfoRestorationAfterModification(oc *exutil.CLI, ctx context.Context, t ConfigMapTarget) {
	cmCtx, _ := resolveAndGetConfigMap(oc, ctx, t)
	configKey := t.ResolvedKey()

	expectedMinVersion := GetExpectedMinTLSVersion(oc, ctx)
	e2e.Logf("Expected minTLSVersion from cluster profile: %s", expectedMinVersion)

	configData := cmCtx.cm.Data[configKey]
	if !strings.Contains(configData, "minTLSVersion") {
		g.Skip(fmt.Sprintf("ConfigMap %s/%s does not have minTLSVersion, skipping modification test",
			cmCtx.namespace, t.ConfigMapName))
	}

	wrongValue := "VersionTLS10"
	if strings.Contains(configData, "VersionTLS10") {
		wrongValue = "VersionTLS99"
	}

	g.By(fmt.Sprintf("modifying minTLSVersion to wrong value: %s", wrongValue))
	var newLines []string
	for _, line := range strings.Split(configData, "\n") {
		if strings.Contains(line, "minTLSVersion:") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
			newLines = append(newLines, fmt.Sprintf("%sminTLSVersion: %s", indent, wrongValue))
		} else {
			newLines = append(newLines, line)
		}
	}
	cmCtx.cm.Data[configKey] = strings.Join(newLines, "\n")
	cmCtx.update()
	e2e.Logf("Modified minTLSVersion to '%s' on ConfigMap %s/%s", wrongValue, cmCtx.namespace, t.ConfigMapName)

	g.By("waiting for operator to restore correct minTLSVersion")
	err := wait.PollUntilContextTimeout(cmCtx.ctx, 5*time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			cm, err := cmCtx.oc.AdminKubeClient().CoreV1().ConfigMaps(cmCtx.namespace).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
			if err != nil {
				e2e.Logf("  poll: error fetching ConfigMap: %v", err)
				return false, nil
			}
			data := cm.Data[configKey]
			if !strings.Contains(data, wrongValue) && strings.Contains(data, expectedMinVersion) {
				e2e.Logf("  poll: minTLSVersion restored to %s!", expectedMinVersion)
				return true, nil
			}
			e2e.Logf("  poll: minTLSVersion not yet restored (still has wrong value or missing expected)")
			return false, nil
		},
	)
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("minTLSVersion was not restored on ConfigMap %s/%s within timeout (expected %s)",
			cmCtx.namespace, t.ConfigMapName, expectedMinVersion))

	e2e.Logf("PASS: minTLSVersion was restored to '%s' after modification on ConfigMap %s/%s",
		expectedMinVersion, cmCtx.namespace, t.ConfigMapName)
}

// TestDeploymentTLSEnvVars verifies that the deployment has TLS environment
// variables matching the expected TLS profile.
func TestDeploymentTLSEnvVars(oc *exutil.CLI, ctx context.Context, t DeploymentEnvVarTarget) {
	g.By("getting cluster APIServer TLS profile")
	expectedMinVersion := GetExpectedMinTLSVersion(oc, ctx)
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

	g.By(fmt.Sprintf("verifying %s env var in deployment containers", t.TLSMinVersionEnvVar))
	envMap := exutil.FindEnvAcrossContainers(deployment.Spec.Template.Spec.Containers, t.TLSMinVersionEnvVar)
	exutil.LogEnvVars(envMap, t.TLSMinVersionEnvVar)

	o.Expect(envMap).To(o.HaveKey(t.TLSMinVersionEnvVar),
		fmt.Sprintf("expected %s to be set in deployment %s/%s",
			t.TLSMinVersionEnvVar, t.Namespace, t.DeploymentName))
	o.Expect(envMap[t.TLSMinVersionEnvVar]).To(o.Equal(expectedMinVersion),
		fmt.Sprintf("expected %s=%s in deployment %s/%s, got %s",
			t.TLSMinVersionEnvVar, expectedMinVersion, t.Namespace, t.DeploymentName,
			envMap[t.TLSMinVersionEnvVar]))
	e2e.Logf("PASS: %s=%s matches cluster TLS profile in %s/%s",
		t.TLSMinVersionEnvVar, expectedMinVersion, t.Namespace, t.DeploymentName)

	if t.CipherSuitesEnvVar != "" {
		g.By(fmt.Sprintf("verifying %s env var in deployment containers", t.CipherSuitesEnvVar))
		o.Expect(envMap).To(o.HaveKey(t.CipherSuitesEnvVar),
			fmt.Sprintf("expected %s to be set in deployment %s/%s",
				t.CipherSuitesEnvVar, t.Namespace, t.DeploymentName))
		o.Expect(envMap[t.CipherSuitesEnvVar]).NotTo(o.BeEmpty(),
			fmt.Sprintf("expected %s to have a value in deployment %s/%s",
				t.CipherSuitesEnvVar, t.Namespace, t.DeploymentName))
		e2e.Logf("PASS: %s is set in %s/%s (value length=%d)",
			t.CipherSuitesEnvVar, t.Namespace, t.DeploymentName, len(envMap[t.CipherSuitesEnvVar]))
	}
}

// TestWireLevelTLS verifies that the service endpoint enforces the TLS version
// from the cluster APIServer profile using oc port-forward.
func TestWireLevelTLS(oc *exutil.CLI, ctx context.Context, t ServiceTarget) {
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

// ─── Verify / wait helpers for profile-change tests ────────────────────────

// VerifyObservedConfigForTargets checks a list of targets for correct
// ObservedConfig after a TLS profile switch.
func VerifyObservedConfigForTargets(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string, targets []ObservedConfigTarget) {
	dynClient := oc.AdminDynamicClient()
	for _, t := range targets {
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

// VerifyConfigMapsForTargets checks a list of targets for correct TLS
// injection after a profile switch.
func VerifyConfigMapsForTargets(oc *exutil.CLI, ctx context.Context, expectedVersion, profileLabel string, targets []ConfigMapTarget) {
	for _, t := range targets {
		ns := t.ResolvedNamespace()
		cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(ns).Get(ctx, t.ConfigMapName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("SKIP: ConfigMap %s/%s not found: %v", ns, t.ConfigMapName, err)
			continue
		}
		configData := cm.Data[t.ResolvedKey()]
		o.Expect(cm.Annotations).To(o.HaveKey(InjectTLSAnnotation),
			fmt.Sprintf("ConfigMap %s/%s is missing %s annotation", ns, t.ConfigMapName, InjectTLSAnnotation))
		o.Expect(configData).To(o.ContainSubstring(expectedVersion),
			fmt.Sprintf("ConfigMap %s/%s should have %s after %s switch",
				ns, t.ConfigMapName, expectedVersion, profileLabel))
		e2e.Logf("PASS: ConfigMap %s/%s has %s after %s switch",
			ns, t.ConfigMapName, expectedVersion, profileLabel)
	}
}

// WaitForOperatorsAfterTLSChange waits for the given ClusterOperators and
// Deployments to stabilize after a TLS profile change.
func WaitForOperatorsAfterTLSChange(oc *exutil.CLI, ctx context.Context, profileLabel string, operators []string, deployments []DeploymentRolloutTarget) {
	e2e.Logf("Waiting 30s for operators to begin processing %s profile change", profileLabel)
	time.Sleep(30 * time.Second)

	e2e.Logf("Waiting for ClusterOperators to stabilize after %s profile change", profileLabel)
	for _, co := range operators {
		e2e.Logf("Waiting for ClusterOperator %s to stabilize after %s switch", co, profileLabel)
		exutil.WaitForClusterOperatorStable(oc, ctx, co)
	}

	for _, d := range deployments {
		e2e.Logf("Waiting for deployment %s/%s to complete rollout after %s switch", d.Namespace, d.DeploymentName, profileLabel)
		deployment, err := oc.AdminKubeClient().AppsV1().Deployments(d.Namespace).Get(ctx, d.DeploymentName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.WaitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, OperatorRolloutTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("deployment %s/%s did not complete rollout after %s TLS change (timeout: %v)",
				d.Namespace, d.DeploymentName, profileLabel, OperatorRolloutTimeout))
		e2e.Logf("Deployment %s/%s is fully rolled out after %s switch", d.Namespace, d.DeploymentName, profileLabel)
	}
	e2e.Logf("All operators and deployments are stable after %s profile change", profileLabel)
}

// ─── TLS profile helpers ───────────────────────────────────────────────────

func GetExpectedMinTLSVersion(oc *exutil.CLI, ctx context.Context) string {
	minVersion, _ := GetExpectedMinTLSVersionWithType(oc, ctx)
	return minVersion
}

func GetExpectedMinTLSVersionWithType(oc *exutil.CLI, ctx context.Context) (string, string) {
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
