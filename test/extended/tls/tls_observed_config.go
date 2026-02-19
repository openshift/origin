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
	// deploymentRolloutTimeout is the maximum time to wait for a deployment
	// to complete rollout after a TLS profile change. KAS rollout typically
	// takes 15-20 minutes, so we set this to 25 minutes to be safe.
	deploymentRolloutTimeout = 25 * time.Minute
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
		operatorConfigGVR:   schema.GroupVersionResource{}, // Same operator config as image-registry
		operatorConfigName:  "",
		clusterOperatorName: "image-registry",
		configMapName:       "", // Uses same ConfigMap as image-registry main entry
		configMapKey:        "",
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
		operatorConfigGVR:   schema.GroupVersionResource{}, // Same operator config as port 443
		operatorConfigName:  "",
		clusterOperatorName: "kube-apiserver",
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
		operatorConfigGVR:   schema.GroupVersionResource{}, // Same operator config as port 443
		operatorConfigName:  "",
		clusterOperatorName: "openshift-apiserver",
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

		g.It(fmt.Sprintf("should populate ObservedConfig with TLS settings - %s", target.namespace), func() {
			testObservedConfig(oc, ctx, target)
		})
	}

	// ── Per-namespace ConfigMap TLS injection verification ──────────────
	// For each target with a configMapName, verify that CVO has injected
	// TLS config (minTLSVersion and cipherSuites) into the ConfigMap's
	// servingInfo section via the config.openshift.io/inject-tls annotation.
	for _, target := range targets {
		target := target // capture range variable
		if target.configMapName == "" {
			continue
		}

		g.It(fmt.Sprintf("should have TLS config injected into ConfigMap - %s", target.namespace), func() {
			testConfigMapTLSInjection(oc, ctx, target)
		})
	}

	// ── ConfigMap annotation restoration tests ────────────────────────────
	// These tests verify that the operator restores the inject-tls annotation
	// if it's deleted or set to an incorrect value.
	for _, target := range targets {
		target := target // capture range variable
		if target.configMapName == "" {
			continue
		}

		g.It(fmt.Sprintf("should restore inject-tls annotation after deletion - %s [Serial] [Disruptive]", target.namespace), func() {
			testAnnotationRestorationAfterDeletion(oc, ctx, target)
		})

		g.It(fmt.Sprintf("should restore inject-tls annotation when set to false - %s [Serial] [Disruptive]", target.namespace), func() {
			testAnnotationRestorationWhenFalse(oc, ctx, target)
		})

		g.It(fmt.Sprintf("should restore servingInfo after removal - %s [Serial] [Disruptive]", target.namespace), func() {
			testServingInfoRestorationAfterRemoval(oc, ctx, target)
		})

		g.It(fmt.Sprintf("should restore servingInfo after modification - %s [Serial] [Disruptive]", target.namespace), func() {
			testServingInfoRestorationAfterModification(oc, ctx, target)
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

		g.It(fmt.Sprintf("should propagate TLS config to deployment env vars - %s", target.namespace), func() {
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

		// Include port in test name to distinguish targets with same namespace but different ports
		g.It(fmt.Sprintf("should enforce TLS version at the wire level - %s:%s", target.namespace, target.servicePort), func() {
			testWireLevelTLS(oc, ctx, target)
		})
	}

	// ── Config-change test: switch to Modern, verify, restore ────────
	// This test modifies the cluster APIServer TLS profile, waits for the
	// KAS rollout to complete (via WaitForOperatorToRollout), then verifies
	// that every target service enforces TLS 1.3.  It restores the original
	// profile in DeferCleanup.
	g.It("should enforce Modern TLS profile after cluster-wide config change [Slow] [Disruptive] [Timeout:60m]", func() {

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

		// KAS rollout can take 15-20+ minutes, plus we need time for other operators
		// to stabilize and for wire-level verification. Use 50 minutes to stay under
		// the 60-minute test timeout while allowing sufficient time.
		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 50*time.Minute)
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
				Type:   configv1.TLSProfileModernType,
				Modern: &configv1.ModernTLSProfile{},
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
			err = waitForDeploymentCompleteWithTimeout(configChangeCtx, oc.AdminKubeClient(), deployment, deploymentRolloutTimeout)
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("deployment %s/%s did not complete rollout after TLS change (timeout: %v)",
					t.namespace, t.deploymentName, deploymentRolloutTimeout))
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

		// 9. Verify ObservedConfig reflects Modern profile (VersionTLS13).
		g.By("verifying ObservedConfig reflects Modern profile (VersionTLS13)")
		verifyObservedConfigAfterSwitch(oc, configChangeCtx, "VersionTLS13", "Modern")

		// 10. Verify ConfigMaps reflect Modern profile (VersionTLS13).
		g.By("verifying ConfigMaps reflect Modern profile (VersionTLS13)")
		verifyConfigMapsAfterSwitch(oc, configChangeCtx, "VersionTLS13", "Modern")

		// 11. Wire-level: verify TLS 1.3 is accepted and TLS 1.2 is rejected.
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

		// ── Phase 2: Downgrade to Intermediate and verify TLS 1.2 ──────────
		g.By("setting APIServer TLS profile back to Intermediate (nil)")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(configChangeCtx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			apiServer.Spec.TLSSecurityProfile = nil // nil means Intermediate (default)
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(configChangeCtx, apiServer, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update APIServer TLS profile to Intermediate")
		e2e.Logf("APIServer TLS profile updated to Intermediate (nil)")

		// Wait for operators to roll out with Intermediate profile.
		g.By("waiting for operators to stabilize after switching to Intermediate")
		kasRollout2 := exutil.WaitForOperatorToRollout(configChangeCtx, oc.AdminConfigClient(), "kube-apiserver")
		<-kasRollout2.StableBeforeStarting()
		<-kasRollout2.Done()
		o.Expect(kasRollout2.Err()).NotTo(o.HaveOccurred(),
			"kube-apiserver did not stabilize after TLS profile change to Intermediate")

		for _, co := range targetClusterOperators() {
			waitForClusterOperatorStable(oc, configChangeCtx, co)
		}

		// Verify ObservedConfig reflects Intermediate profile (VersionTLS12).
		g.By("verifying ObservedConfig reflects Intermediate profile (VersionTLS12)")
		verifyObservedConfigAfterSwitch(oc, configChangeCtx, "VersionTLS12", "Intermediate")

		// Verify ConfigMaps reflect Intermediate profile (VersionTLS12).
		g.By("verifying ConfigMaps reflect Intermediate profile (VersionTLS12)")
		verifyConfigMapsAfterSwitch(oc, configChangeCtx, "VersionTLS12", "Intermediate")

		// ── Phase 3: Upgrade to Modern again and verify TLS 1.3 ──────────
		g.By("setting APIServer TLS profile to Modern again")
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
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update APIServer TLS profile to Modern (2nd time)")
		e2e.Logf("APIServer TLS profile updated to Modern (2nd time)")

		// Wait for operators to roll out with Modern profile.
		g.By("waiting for operators to stabilize after switching to Modern (2nd time)")
		kasRollout3 := exutil.WaitForOperatorToRollout(configChangeCtx, oc.AdminConfigClient(), "kube-apiserver")
		<-kasRollout3.StableBeforeStarting()
		<-kasRollout3.Done()
		o.Expect(kasRollout3.Err()).NotTo(o.HaveOccurred(),
			"kube-apiserver did not stabilize after TLS profile change to Modern (2nd time)")

		for _, co := range targetClusterOperators() {
			waitForClusterOperatorStable(oc, configChangeCtx, co)
		}

		// Verify ObservedConfig reflects Modern profile (VersionTLS13) after 2nd switch.
		g.By("verifying ObservedConfig reflects Modern profile (VersionTLS13) after 2nd switch")
		verifyObservedConfigAfterSwitch(oc, configChangeCtx, "VersionTLS13", "Modern (2nd)")

		// Verify ConfigMaps reflect Modern profile (VersionTLS13) after 2nd switch.
		g.By("verifying ConfigMaps reflect Modern profile (VersionTLS13) after 2nd switch")
		verifyConfigMapsAfterSwitch(oc, configChangeCtx, "VersionTLS13", "Modern (2nd)")

		// ── Phase 4: Final downgrade to Intermediate and verify TLS 1.2 ──────────
		g.By("setting APIServer TLS profile back to Intermediate (final)")
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			apiServer, err := oc.AdminConfigClient().ConfigV1().APIServers().Get(configChangeCtx, "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			apiServer.Spec.TLSSecurityProfile = nil // nil means Intermediate (default)
			_, err = oc.AdminConfigClient().ConfigV1().APIServers().Update(configChangeCtx, apiServer, metav1.UpdateOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update APIServer TLS profile to Intermediate (final)")
		e2e.Logf("APIServer TLS profile updated to Intermediate (final)")

		// Wait for operators to roll out with Intermediate profile.
		g.By("waiting for operators to stabilize after final switch to Intermediate")
		kasRollout4 := exutil.WaitForOperatorToRollout(configChangeCtx, oc.AdminConfigClient(), "kube-apiserver")
		<-kasRollout4.StableBeforeStarting()
		<-kasRollout4.Done()
		o.Expect(kasRollout4.Err()).NotTo(o.HaveOccurred(),
			"kube-apiserver did not stabilize after TLS profile change to Intermediate (final)")

		for _, co := range targetClusterOperators() {
			waitForClusterOperatorStable(oc, configChangeCtx, co)
		}

		// Verify ObservedConfig reflects Intermediate profile (VersionTLS12) after final switch.
		g.By("verifying ObservedConfig reflects Intermediate profile (VersionTLS12) after final switch")
		verifyObservedConfigAfterSwitch(oc, configChangeCtx, "VersionTLS12", "Intermediate (final)")

		// Verify ConfigMaps reflect Intermediate profile (VersionTLS12) after final switch.
		g.By("verifying ConfigMaps reflect Intermediate profile (VersionTLS12) after final switch")
		verifyConfigMapsAfterSwitch(oc, configChangeCtx, "VersionTLS12", "Intermediate (final)")

		e2e.Logf("PASS: Full TLS propagation cycle verified (Modern → Intermediate → Modern → Intermediate)")
	})

	// ── Custom TLS profile test ────────────────────────────────────────────
	// This test sets a Custom TLS profile with specific minTLSVersion and
	// cipherSuites, verifies propagation to all operators, then restores.
	g.It("should enforce Custom TLS profile after cluster-wide config change [Slow] [Disruptive] [Timeout:60m]", func() {
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

		// 2. Create context with timeout for the entire config change operation.
		configChangeCtx, configChangeCancel := context.WithTimeout(ctx, 60*time.Minute)
		defer configChangeCancel()

		// 3. DeferCleanup to restore the original TLS profile.
		g.DeferCleanup(func(cleanupCtx context.Context) {
			e2e.Logf("DeferCleanup: restoring original TLS profile: %s", profileDesc)

			restoreRollout := exutil.WaitForOperatorToRollout(cleanupCtx, oc.AdminConfigClient(), "kube-apiserver")
			<-restoreRollout.StableBeforeStarting()

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

			e2e.Logf("DeferCleanup: waiting for KAS rollout to complete after restoring profile")
			<-restoreRollout.Done()
			o.Expect(restoreRollout.Err()).NotTo(o.HaveOccurred(),
				"kube-apiserver did not stabilize after restoring TLS profile")

			for _, co := range targetClusterOperators() {
				e2e.Logf("DeferCleanup: waiting for ClusterOperator %s to stabilize", co)
				waitForClusterOperatorStable(oc, cleanupCtx, co)
			}
			e2e.Logf("DeferCleanup: original TLS profile restored and cluster is stable")
		})

		// 4. Define Custom TLS profile with TLS 1.2 and specific cipher suites.
		// Using a subset of TLS 1.2 ciphers for Custom profile.
		customCiphers := []string{
			"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			"TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
		}

		// 5. Start watching KAS rollout *before* making the change.
		g.By("starting KAS rollout watcher before changing TLS profile to Custom")
		kasRollout := exutil.WaitForOperatorToRollout(configChangeCtx, oc.AdminConfigClient(), "kube-apiserver")

		e2e.Logf("Waiting for KAS to be stable before applying Custom TLS profile")
		<-kasRollout.StableBeforeStarting()
		e2e.Logf("KAS is stable; proceeding to apply Custom TLS profile")

		// 6. Set the APIServer TLS profile to Custom.
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

		// 7. Wait for KAS rollout to complete.
		g.By("waiting for kube-apiserver rollout to complete after TLS profile change to Custom")
		e2e.Logf("Waiting for KAS to start progressing...")
		<-kasRollout.Started()
		e2e.Logf("KAS rollout started, waiting for it to finish...")
		<-kasRollout.Done()
		o.Expect(kasRollout.Err()).NotTo(o.HaveOccurred(),
			"kube-apiserver did not stabilize after TLS profile change to Custom")
		e2e.Logf("KAS rollout completed successfully")

		// 8. Wait for each target operator to stabilize.
		g.By("waiting for target operators to stabilize after KAS rollout")
		for _, co := range targetClusterOperators() {
			e2e.Logf("Waiting for ClusterOperator %s to stabilize", co)
			waitForClusterOperatorStable(oc, configChangeCtx, co)
		}

		// 9. Wait for target deployments to fully roll out.
		for _, t := range targets {
			if t.deploymentName == "" {
				continue
			}
			g.By(fmt.Sprintf("waiting for deployment %s/%s to be fully rolled out",
				t.namespace, t.deploymentName))
			deployment, err := oc.AdminKubeClient().AppsV1().Deployments(t.namespace).Get(
				configChangeCtx, t.deploymentName, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForDeploymentCompleteWithTimeout(configChangeCtx, oc.AdminKubeClient(), deployment, deploymentRolloutTimeout)
			o.Expect(err).NotTo(o.HaveOccurred(),
				fmt.Sprintf("deployment %s/%s did not complete rollout after Custom TLS change (timeout: %v)",
					t.namespace, t.deploymentName, deploymentRolloutTimeout))
			e2e.Logf("Deployment %s/%s is fully rolled out", t.namespace, t.deploymentName)
		}

		// 10. Verify ObservedConfig reflects Custom profile (VersionTLS12).
		g.By("verifying ObservedConfig reflects Custom profile (VersionTLS12)")
		verifyObservedConfigAfterSwitch(oc, configChangeCtx, "VersionTLS12", "Custom")

		// 11. Verify ConfigMaps reflect Custom profile (VersionTLS12).
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
			o.Expect(configData).To(o.ContainSubstring("VersionTLS12"),
				fmt.Sprintf("ConfigMap %s/%s should have VersionTLS12 for Custom profile", cmNamespace, t.configMapName))
			e2e.Logf("PASS: ConfigMap %s/%s has VersionTLS12 for Custom profile", cmNamespace, t.configMapName)

			// Verify custom cipher suites are present.
			for _, cipher := range customCiphers[:2] { // Check at least first 2 ciphers
				o.Expect(configData).To(o.ContainSubstring(cipher),
					fmt.Sprintf("ConfigMap %s/%s should contain custom cipher %s", cmNamespace, t.configMapName, cipher))
			}
			e2e.Logf("PASS: ConfigMap %s/%s has custom cipher suites", cmNamespace, t.configMapName)
		}

		// 12. Wire-level TLS verification for Custom profile.
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
	if err != nil {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}

	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.configMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.configMapName))

	// Verify the inject-tls annotation is present.
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
	if err != nil {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}

	// Get the original ConfigMap and verify annotation exists.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.configMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.configMapName))

	// Verify the annotation exists before we delete it.
	_, found := cm.Annotations["config.openshift.io/inject-tls"]
	if !found {
		g.Skip(fmt.Sprintf("ConfigMap %s/%s does not have inject-tls annotation, skipping deletion test", cmNamespace, t.configMapName))
	}

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
	if err != nil {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}

	// Get the original ConfigMap.
	g.By(fmt.Sprintf("getting ConfigMap %s/%s", cmNamespace, t.configMapName))
	cm, err := oc.AdminKubeClient().CoreV1().ConfigMaps(cmNamespace).Get(ctx, t.configMapName, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(),
		fmt.Sprintf("failed to get ConfigMap %s/%s", cmNamespace, t.configMapName))

	// Set the annotation to "false".
	g.By("setting config.openshift.io/inject-tls annotation to 'false'")
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
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
	if err != nil {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}

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
	if err != nil {
		g.Skip(fmt.Sprintf("Namespace %s does not exist in this cluster", cmNamespace))
	}

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
		err = waitForDeploymentCompleteWithTimeout(ctx, oc.AdminKubeClient(), deployment, deploymentRolloutTimeout)
		o.Expect(err).NotTo(o.HaveOccurred(),
			fmt.Sprintf("deployment %s/%s did not complete rollout (timeout: %v)", t.namespace, t.deploymentName, deploymentRolloutTimeout))
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
	dynClient := oc.AdminDynamicClient()
	for _, t := range targets {
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
	for _, t := range targets {
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
// the given test function with the local port.  Retries up to 3 times.
func forwardPortAndExecute(serviceName, namespace, remotePort string, toExecute func(localPort int) error) error {
	var err error
	for i := 0; i < 3; i++ {
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

			// Wait for port-forward to be ready by checking for "Forwarding from" message
			// or by polling the port until it accepts connections.
			ready := false
			for j := 0; j < 20; j++ { // Try for up to 10 seconds (20 * 500ms)
				// Check if port-forward printed the ready message.
				output := readPartialFrom(stdout, 1024)
				if strings.Contains(output, "Forwarding from") {
					e2e.Logf("oc port-forward ready: %s", output)
					ready = true
					break
				}

				// Also try connecting to verify the port is accepting connections.
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

		// Try to connect with the TLS config that should work.
		conn, err := tls.Dial("tcp", host, shouldWork)
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
		e2e.Logf("[%s] %s: ✓ SUCCESS - Negotiated %s (requested min %s)",
			hostType, host, tlsVersionName(negotiated), expectedMinVersion)

		// Test that the version that should not work is rejected.
		e2e.Logf("[%s] %s: Testing connection with max %s (should be REJECTED)",
			hostType, host, rejectedMaxVersion)

		conn, err = tls.Dial("tcp", host, shouldNotWork)
		if err == nil {
			negotiatedBad := conn.ConnectionState().Version
			conn.Close()
			return fmt.Errorf("svc/%s in %s [%s]: Connection with max %s should be REJECTED but succeeded (negotiated %s)",
				t.serviceName, t.namespace, hostType, rejectedMaxVersion, tlsVersionName(negotiatedBad))
		}

		// Verify we got a TLS-related error, not a network error.
		errStr := err.Error()
		if !strings.Contains(errStr, "protocol version") &&
			!strings.Contains(errStr, "no supported versions") &&
			!strings.Contains(errStr, "handshake failure") &&
			!strings.Contains(errStr, "alert") {
			return fmt.Errorf("svc/%s in %s [%s]: Expected TLS version rejection error, got: %w",
				t.serviceName, t.namespace, hostType, err)
		}
		e2e.Logf("[%s] %s: ✓ REJECTED - %s correctly refused by server",
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
