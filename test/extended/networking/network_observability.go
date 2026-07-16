package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	applyconfigv1 "github.com/openshift/client-go/config/applyconfigurations/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	netobservOperatorNamespace    = "netobserv-operator"
	netobservNamespace            = "netobserv"
	netobservPrivilegedNS         = "netobserv-privileged"
	netobservClusterExtensionName = "netobserv-operator"
	netobservControllerManager    = "netobserv-controller-manager"
	flowCollectorName             = "cluster"
	flpMetricsPort                = "9401"
	netobservFieldManager         = "netobserv-policy-e2e"
)

type flowCollectorCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

var _ = g.Describe("[sig-network][OCPFeatureGate:NetworkObservabilityInstall][Feature:NetObserv]", func() {
	oc := exutil.NewCLIWithoutNamespace("netobserv-e2e")

	g.It("should not be installed on single node clusters", func(ctx context.Context) {
		isSingleNode, err := exutil.IsSingleNode(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if !isSingleNode {
			g.Skip("test only applies to single node clusters")
		}

		g.By("checking that the operator namespace does not exist")
		_, err = oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, netobservOperatorNamespace, metav1.GetOptions{})
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(),
			"Network observability operator namespace %q should not exist on single node clusters (err: %v)", netobservOperatorNamespace, err)

		g.By("checking that the workload namespace does not exist")
		_, err = oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, netobservNamespace, metav1.GetOptions{})
		o.Expect(apierrors.IsNotFound(err)).To(o.BeTrue(),
			"Network observability namespace %q should not exist on single node clusters (err: %v)", netobservNamespace, err)

		g.By("checking that the FlowCollector CRD is not installed")
		_, crdErr := oc.AdminKubeClient().Discovery().ServerResourcesForGroupVersion("flows.netobserv.io/v1beta2")
		o.Expect(crdErr).To(o.HaveOccurred(),
			"FlowCollector API group should not be available on single node clusters")
	})

	g.Context("health checks", func() {
		g.BeforeEach(func(ctx context.Context) {
			isSingleNode, err := exutil.IsSingleNode(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to determine cluster topology")
			if isSingleNode {
				g.Skip("NetObserv is not expected on single node clusters")
			}

			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to determine if cluster is MicroShift")
			if isMicroShift {
				g.Skip("NetObserv is not supported on MicroShift")
			}
		})

		g.It("should have FlowCollector CR in Ready state", func(ctx context.Context) {
			g.By("verifying operator namespace exists")
			_, err := oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, netobservOperatorNamespace, metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(),
				"Network observability operator namespace %q must exist", netobservOperatorNamespace)

			g.By("checking FlowCollector CR has Ready status")
			output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"flowcollector", flowCollectorName,
				"-o=jsonpath={.status.conditions}",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "FlowCollector CR %q should exist", flowCollectorName)

			var conditions []flowCollectorCondition
			err = json.Unmarshal([]byte(output), &conditions)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to parse FlowCollector conditions")

			ready := false
			for _, c := range conditions {
				if c.Type == "Ready" && c.Status == "True" {
					ready = true
					break
				}
			}
			o.Expect(ready).To(o.BeTrue(), "FlowCollector should have Ready=True condition")
		})

		g.It("should have operator pod running", func(ctx context.Context) {
			pods, err := oc.AdminKubeClient().CoreV1().Pods(netobservOperatorNamespace).List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to list pods in %s", netobservOperatorNamespace)
			o.Expect(pods.Items).NotTo(o.BeEmpty(), "expected at least one pod in %s", netobservOperatorNamespace)

			found := false
			for _, pod := range pods.Items {
				if strings.Contains(pod.Name, netobservControllerManager) {
					o.Expect(string(pod.Status.Phase)).To(o.Equal("Running"),
						"netobserv-controller-manager pod should be Running, got %s", pod.Status.Phase)
					found = true
					break
				}
			}
			o.Expect(found).To(o.BeTrue(), "netobserv-controller-manager pod not found in %s", netobservOperatorNamespace)
		})

		g.It("should have FLP pods running", func(ctx context.Context) {
			o.Eventually(func() bool {
				flpPods, err := oc.AdminKubeClient().CoreV1().Pods(netobservNamespace).List(ctx, metav1.ListOptions{
					LabelSelector: "app=flowlogs-pipeline",
				})
				if err != nil {
					framework.Logf("failed to list FLP pods")
					return false
				}
				if len(flpPods.Items) == 0 {
					framework.Logf("no FLP pods found in %s", netobservNamespace)
					return false
				}
				for _, pod := range flpPods.Items {
					if pod.Status.Phase != "Running" {
						framework.Logf("FLP pod %s phase is %s", pod.Name, pod.Status.Phase)
						return false
					}
				}
				return true
			}, 3*time.Minute, 5*time.Second).Should(o.BeTrue(), "FLP pods should be Running in %s", netobservNamespace)
		})

		g.It("should have eBPF agent DaemonSet fully ready", func(ctx context.Context) {
			g.By("checking eBPF agent DaemonSet readiness")
			o.Eventually(func() bool {
				ds, err := oc.AdminKubeClient().AppsV1().DaemonSets(netobservPrivilegedNS).List(ctx, metav1.ListOptions{})
				if err != nil {
					framework.Logf("failed to list DaemonSets in %s", netobservPrivilegedNS)
					return false
				}
				for _, d := range ds.Items {
					if strings.Contains(d.Name, "netobserv-ebpf-agent") {
						desired := d.Status.DesiredNumberScheduled
						readyCount := d.Status.NumberReady
						if desired == 0 {
							framework.Logf("eBPF DaemonSet desired=0")
							return false
						}
						if desired != readyCount {
							framework.Logf("eBPF DaemonSet desired=%d ready=%d", desired, readyCount)
							return false
						}
						return true
					}
				}
				framework.Logf("no eBPF agent DaemonSet found in %s", netobservPrivilegedNS)
				return false
			}, 3*time.Minute, 5*time.Second).Should(o.BeTrue(), "eBPF agent DaemonSet should have desired=ready")

			g.By("verifying all eBPF agent pods are Running")
			ebpfPods, err := oc.AdminKubeClient().CoreV1().Pods(netobservPrivilegedNS).List(ctx, metav1.ListOptions{
				LabelSelector: "app=netobserv-ebpf-agent",
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to list eBPF agent pods in %s", netobservPrivilegedNS)
			o.Expect(ebpfPods.Items).NotTo(o.BeEmpty(), "expected eBPF agent pods in %s", netobservPrivilegedNS)
			for _, pod := range ebpfPods.Items {
				o.Expect(string(pod.Status.Phase)).To(o.Equal("Running"),
					"eBPF agent pod %s should be Running", pod.Name)
			}
		})

		g.It("should have console plugin healthy if deployed [apigroup:console.openshift.io]", func(ctx context.Context) {
			pluginPods, err := oc.AdminKubeClient().CoreV1().Pods(netobservNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=netobserv-plugin",
			})
			if err != nil || len(pluginPods.Items) == 0 {
				g.Skip("console plugin not deployed")
			}

			for _, pod := range pluginPods.Items {
				o.Expect(string(pod.Status.Phase)).To(o.Equal("Running"),
					"console plugin pod %s should be Running, got %s", pod.Name, pod.Status.Phase)
			}

			pluginOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"consoleplugin", "netobserv-plugin",
				"-o=jsonpath={.metadata.name}",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get consoleplugin resource")
			o.Expect(pluginOutput).To(o.Equal("netobserv-plugin"),
				"consoleplugin resource name mismatch")
		})

		g.It("should not have excessive errors in operator logs", func(ctx context.Context) {
			logOutput, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(
				"-n", netobservOperatorNamespace,
				fmt.Sprintf("deployment/%s", netobservControllerManager),
				"--tail=50",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to fetch operator logs")

			errorCount := 0
			for _, line := range strings.Split(logOutput, "\n") {
				if strings.Contains(line, "\"level\":\"error\"") || strings.Contains(line, "level=error") {
					errorCount++
				}
			}
			o.Expect(errorCount).To(o.BeNumerically("<=", 5),
				"found %d error-level log entries in the last 50 operator log lines (threshold: 5)", errorCount)
		})

		g.It("should have monitoring resources deployed", func(ctx context.Context) {
			g.By("checking ServiceMonitors exist")
			smCount, err := countResources(oc, "servicemonitor", netobservNamespace)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to list ServiceMonitors in %s", netobservNamespace)
			o.Expect(smCount).To(o.BeNumerically(">", 0),
				"expected at least one ServiceMonitor in %s", netobservNamespace)
			framework.Logf("Found %d ServiceMonitor(s) in %s", smCount, netobservNamespace)

			smPrivCount, err := countResources(oc, "servicemonitor", netobservPrivilegedNS)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to list ServiceMonitors in %s", netobservPrivilegedNS)
			o.Expect(smPrivCount).To(o.BeNumerically(">", 0),
				"expected at least one ServiceMonitor in %s", netobservPrivilegedNS)
			framework.Logf("Found %d ServiceMonitor(s) in %s", smPrivCount, netobservPrivilegedNS)

			g.By("checking alert rules are deployed")
			rulesCount, err := countResources(oc, "prometheusrules", netobservNamespace)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to list PrometheusRules in %s", netobservNamespace)
			o.Expect(rulesCount).To(o.BeNumerically(">", 0),
				"expected at least one PrometheusRule in %s", netobservNamespace)
			framework.Logf("Found %d PrometheusRule(s) in %s", rulesCount, netobservNamespace)
		})

		g.It("should have FLP producing non-zero flow data", func(ctx context.Context) {
			o.Eventually(func() bool {
				flpPods, err := oc.AdminKubeClient().CoreV1().Pods(netobservNamespace).List(ctx, metav1.ListOptions{
					LabelSelector: "app=flowlogs-pipeline",
				})
				if err != nil || len(flpPods.Items) == 0 {
					framework.Logf("FLP pods not available yet in %s", netobservNamespace)
					return false
				}
				flpPod := flpPods.Items[0].Name
				metricsOutput, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(
					"-n", netobservNamespace, flpPod, "--",
					"curl", "-s", fmt.Sprintf("http://localhost:%s/metrics", flpMetricsPort),
				).Output()
				if err != nil {
					framework.Logf("failed to query FLP metrics endpoint")
					return false
				}

				for _, line := range strings.Split(metricsOutput, "\n") {
					if strings.HasPrefix(line, "netobserv_ingest_flows_processed") && !strings.HasPrefix(line, "#") {
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							val, err := strconv.ParseFloat(parts[len(parts)-1], 64)
							if err == nil && val > 0 {
								framework.Logf("FLP processed flows metric: %v", val)
								return true
							}
						}
					}
				}
				framework.Logf("netobserv_ingest_flows_processed metric is zero or not found")
				return false
			}, 3*time.Minute, 10*time.Second).Should(o.BeTrue(),
				"FLP should show non-zero netobserv_ingest_flows_processed metric")
		})

		g.It("should have Prometheus scraping non-zero NetObserv metrics", func(ctx context.Context) {
			promPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-monitoring").List(ctx, metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/name=prometheus",
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to list Prometheus pods in openshift-monitoring")
			o.Expect(promPods.Items).NotTo(o.BeEmpty(), "expected at least one Prometheus pod in openshift-monitoring")
			promPodName := promPods.Items[0].Name

			o.Eventually(func() bool {
				promOutput, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(
					"-n", "openshift-monitoring",
					promPodName, "-c", "prometheus", "--",
					"curl", "-s",
					"http://localhost:9090/api/v1/query?query=netobserv_ingest_flows_processed",
				).Output()
				if err != nil {
					framework.Logf("failed to query Prometheus API")
					return false
				}

				type promResult struct {
					Data struct {
						Result []struct {
							Value []json.RawMessage `json:"value"`
						} `json:"result"`
					} `json:"data"`
				}
				var result promResult
				if err := json.Unmarshal([]byte(promOutput), &result); err != nil {
					framework.Logf("failed to parse Prometheus response")
					return false
				}
				if len(result.Data.Result) == 0 {
					framework.Logf("Prometheus netobserv_ingest_flows_processed: no results yet")
					return false
				}
				for _, r := range result.Data.Result {
					if len(r.Value) >= 2 {
						var valStr string
						if err := json.Unmarshal(r.Value[1], &valStr); err != nil {
							continue
						}
						val, err := strconv.ParseFloat(valStr, 64)
						if err != nil {
							continue
						}
						if val > 0 {
							framework.Logf("Prometheus netobserv_ingest_flows_processed sample value: %v", val)
							return true
						}
					}
				}
				framework.Logf("Prometheus netobserv_ingest_flows_processed: all sample values are zero")
				return false
			}, 5*time.Minute, 15*time.Second).Should(o.BeTrue(),
				"Prometheus should have non-zero netobserv_ingest_flows_processed results")
		})
	})
})

func countResources(oc *exutil.CLI, resource, namespace string) (int, error) {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
		resource, "-n", namespace,
		"-o=jsonpath={.items[*].metadata.name}",
	).Output()
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return 0, nil
	}
	return len(strings.Fields(trimmed)), nil
}

func uninstallNetObserv(ctx context.Context, oc *exutil.CLI, client kubernetes.Interface) {
	g.By("removing FlowCollector CR")
	fcOutput, fcErr := oc.AsAdmin().WithoutNamespace().Run("delete").Args(
		"flowcollector", flowCollectorName, "--ignore-not-found",
	).Output()
	if fcErr != nil {
		framework.Logf("warning: failed to delete FlowCollector %s: %v (output: %s)", flowCollectorName, fcErr, fcOutput)
	}

	g.By("removing NetObserv ClusterExtension")
	ceOutput, ceErr := oc.AsAdmin().WithoutNamespace().Run("delete").Args(
		"clusterextension", netobservClusterExtensionName, "--ignore-not-found", "--wait=true", "--timeout=2m",
	).Output()
	if ceErr != nil {
		framework.Logf("warning: failed to delete ClusterExtension %s: %v (output: %s)", netobservClusterExtensionName, ceErr, ceOutput)
	}

	g.By("waiting for operator pods to terminate")
	o.Eventually(func() bool {
		pods, err := client.CoreV1().Pods(netobservOperatorNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false
		}
		for _, pod := range pods.Items {
			if strings.Contains(pod.Name, netobservControllerManager) {
				framework.Logf("operator pod %s still present (phase=%s)", pod.Name, pod.Status.Phase)
				return false
			}
		}
		return true
	}, 5*time.Minute, 10*time.Second).Should(o.BeTrue(),
		"operator pods should terminate after uninstall")

	g.By("waiting for FLP pods to terminate")
	o.Eventually(func() bool {
		pods, err := client.CoreV1().Pods(netobservNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=flowlogs-pipeline",
		})
		if err != nil {
			return false
		}
		return len(pods.Items) == 0
	}, 3*time.Minute, 10*time.Second).Should(o.BeTrue(),
		"FLP pods should terminate after uninstall")

	g.By("waiting for eBPF agent pods to terminate")
	o.Eventually(func() bool {
		pods, err := client.CoreV1().Pods(netobservPrivilegedNS).List(ctx, metav1.ListOptions{
			LabelSelector: "app=netobserv-ebpf-agent",
		})
		if err != nil {
			return false
		}
		return len(pods.Items) == 0
	}, 3*time.Minute, 10*time.Second).Should(o.BeTrue(),
		"eBPF agent pods should terminate after uninstall")
}

func verifyNetObservNotInstalled(ctx context.Context, client kubernetes.Interface) {
	g.By("confirming operator stays absent")
	o.Consistently(func() bool {
		pods, err := client.CoreV1().Pods(netobservOperatorNamespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			framework.Logf("failed to list pods in %s: %v", netobservOperatorNamespace, err)
			return false
		}
		for _, pod := range pods.Items {
			if strings.Contains(pod.Name, netobservControllerManager) && pod.Status.Phase == "Running" {
				framework.Logf("unexpected running controller-manager pod: %s", pod.Name)
				return false
			}
		}
		flpPods, err := client.CoreV1().Pods(netobservNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=flowlogs-pipeline",
		})
		if err != nil {
			framework.Logf("failed to list FLP pods in %s: %v", netobservNamespace, err)
			return false
		}
		if len(flpPods.Items) > 0 {
			framework.Logf("unexpected FLP pods found: %d", len(flpPods.Items))
			return false
		}
		return true
	}, 3*time.Minute, 30*time.Second).Should(o.BeTrue(),
		"NetObserv should remain uninstalled with NoAction policy")
}

func verifyNetObservHealthy(ctx context.Context, client kubernetes.Interface) {
	g.By("checking operator namespace exists")
	o.Eventually(func() error {
		_, err := client.CoreV1().Namespaces().Get(ctx, netobservOperatorNamespace, metav1.GetOptions{})
		return err
	}, 5*time.Minute, 15*time.Second).Should(o.Succeed(),
		"operator namespace %s should exist", netobservOperatorNamespace)

	g.By("checking operator pod is running")
	o.Eventually(func() bool {
		pods, err := client.CoreV1().Pods(netobservOperatorNamespace).List(ctx, metav1.ListOptions{})
		if err != nil || len(pods.Items) == 0 {
			return false
		}
		for _, pod := range pods.Items {
			if strings.Contains(pod.Name, netobservControllerManager) && pod.Status.Phase == "Running" {
				return true
			}
		}
		return false
	}, 5*time.Minute, 15*time.Second).Should(o.BeTrue(),
		"netobserv-controller-manager should be Running")

	g.By("checking FLP pods are running")
	o.Eventually(func() bool {
		flpPods, err := client.CoreV1().Pods(netobservNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=flowlogs-pipeline",
		})
		if err != nil || len(flpPods.Items) == 0 {
			return false
		}
		for _, pod := range flpPods.Items {
			if pod.Status.Phase != "Running" {
				return false
			}
		}
		return true
	}, 5*time.Minute, 15*time.Second).Should(o.BeTrue(),
		"FLP pods should be Running in %s", netobservNamespace)
}

var _ = g.Describe("[sig-network][OCPFeatureGate:NetworkObservabilityInstall][Feature:NetObserv][Serial]", g.Ordered, func() {
	oc := exutil.NewCLIWithoutNamespace("netobserv-policy-e2e")

	g.BeforeAll(func(ctx context.Context) {
		isSingleNode, err := exutil.IsSingleNode(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isSingleNode {
			g.Skip("NetObserv is not expected on single node clusters")
		}

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("NetObserv is not supported on MicroShift")
		}

		hasAccess, err := hasNetworkConfigWriteAccess(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		if !hasAccess {
			g.Skip("The test is not permitted to modify the cluster/network.config.openshift.io resource")
		}
	})

	g.AfterAll(func(ctx context.Context) {
		g.By("restoring networkObservability to InstallAndEnable")
		netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
			applyconfigv1.NetworkSpec().WithNetworkObservability(
				applyconfigv1.NetworkObservabilitySpec().WithInstallationPolicy(
					configv1.NetworkObservabilityInstallAndEnable,
				),
			),
		)
		_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
			metav1.ApplyOptions{FieldManager: netobservFieldManager, Force: true})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for NetObserv to be reinstalled")
		verifyNetObservHealthy(ctx, oc.AdminKubeClient())
	})

	g.Context("InstallAndEnable policy", func() {
		g.It("should reinstall the operator after manual uninstall when policy is InstallAndEnable", func(ctx context.Context) {
			g.By("setting networkObservability.installationPolicy to InstallAndEnable")
			netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
				applyconfigv1.NetworkSpec().WithNetworkObservability(
					applyconfigv1.NetworkObservabilitySpec().WithInstallationPolicy(
						configv1.NetworkObservabilityInstallAndEnable,
					),
				),
			)
			_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
				metav1.ApplyOptions{FieldManager: netobservFieldManager, Force: true})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying NetObserv is healthy with InstallAndEnable policy")
			verifyNetObservHealthy(ctx, oc.AdminKubeClient())

			g.By("manually uninstalling NetObserv operator")
			uninstallNetObserv(ctx, oc, oc.AdminKubeClient())

			g.By("verifying NetObserv gets automatically reinstalled")
			verifyNetObservHealthy(ctx, oc.AdminKubeClient())
		})
	})

	g.Context("NoAction policy", func() {
		g.It("should not reinstall the operator after manual uninstall when policy is NoAction", func(ctx context.Context) {
			g.By("setting networkObservability.installationPolicy to NoAction")
			netConfigApply := applyconfigv1.Network(clusterConfig).WithSpec(
				applyconfigv1.NetworkSpec().WithNetworkObservability(
					applyconfigv1.NetworkObservabilitySpec().WithInstallationPolicy(
						configv1.NetworkObservabilityNoAction,
					),
				),
			)
			_, err := oc.AdminConfigClient().ConfigV1().Networks().Apply(ctx, netConfigApply,
				metav1.ApplyOptions{FieldManager: netobservFieldManager, Force: true})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("manually uninstalling NetObserv operator")
			uninstallNetObserv(ctx, oc, oc.AdminKubeClient())

			g.By("verifying NetObserv does NOT get reinstalled")
			verifyNetObservNotInstalled(ctx, oc.AdminKubeClient())
		})
	})
})
