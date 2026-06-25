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
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	netobservOperatorNamespace = "openshift-netobserv-operator"
	netobservNamespace         = "openshift-network-observability"
	netobservPrivilegedNS      = "openshift-network-observability-privileged"
	flowCollectorName          = "cluster"
	flpMetricsPort             = "9401"
)

type flowCollectorCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

var _ = g.Describe("[sig-network][Feature:NetObserv]", func() {
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
		crdOutput, crdErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("crd", "flowcollectors.flows.netobserv.io").Output()
		o.Expect(crdErr).To(o.HaveOccurred(),
			"FlowCollector CRD should not be installed on single node clusters, but found: %s", crdOutput)
		o.Expect(strings.Contains(crdOutput, "NotFound") || strings.Contains(crdOutput, "not found")).To(o.BeTrue(),
			"expected not-found error for FlowCollector CRD, got: %s", crdOutput)
	})

	g.It("should have all components healthy and producing flow data", func(ctx context.Context) {
		isSingleNode, err := exutil.IsSingleNode(ctx, oc.AdminConfigClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isSingleNode {
			g.Skip("NetObserv is not expected on single node clusters")
		}

		g.By("verifying operator namespace exists")
		_, err = oc.AdminKubeClient().CoreV1().Namespaces().Get(ctx, netobservOperatorNamespace, metav1.GetOptions{})
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

		g.By("checking operator pod is running")
		pods, err := oc.AdminKubeClient().CoreV1().Pods(netobservOperatorNamespace).List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pods.Items).NotTo(o.BeEmpty(), "expected at least one pod in %s", netobservOperatorNamespace)

		found := false
		for _, pod := range pods.Items {
			if strings.Contains(pod.Name, "netobserv-controller-manager") {
				o.Expect(string(pod.Status.Phase)).To(o.Equal("Running"),
					"netobserv-controller-manager pod should be Running, got %s", pod.Status.Phase)
				found = true
				break
			}
		}
		o.Expect(found).To(o.BeTrue(), "netobserv-controller-manager pod not found in %s", netobservOperatorNamespace)

		g.By("checking FLP pods are running")
		o.Eventually(func() bool {
			flpPods, err := oc.AdminKubeClient().CoreV1().Pods(netobservNamespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app=flowlogs-pipeline",
			})
			if err != nil {
				framework.Logf("Error listing FLP pods: %v", err)
				return false
			}
			if len(flpPods.Items) == 0 {
				framework.Logf("No FLP pods found in %s", netobservNamespace)
				return false
			}
			for _, pod := range flpPods.Items {
				if pod.Status.Phase != "Running" {
					framework.Logf("FLP pod %s is %s, not Running", pod.Name, pod.Status.Phase)
					return false
				}
			}
			return true
		}, 3*time.Minute, 5*time.Second).Should(o.BeTrue(), "FLP pods should be Running")

		g.By("checking eBPF agent DaemonSet readiness")
		o.Eventually(func() bool {
			ds, err := oc.AdminKubeClient().AppsV1().DaemonSets(netobservPrivilegedNS).List(ctx, metav1.ListOptions{})
			if err != nil {
				framework.Logf("Error listing DaemonSets in %s: %v", netobservPrivilegedNS, err)
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
			framework.Logf("No eBPF agent DaemonSet found in %s", netobservPrivilegedNS)
			return false
		}, 3*time.Minute, 5*time.Second).Should(o.BeTrue(), "eBPF agent DaemonSet should have desired=ready")

		g.By("verifying all eBPF agent pods are Running")
		ebpfPods, err := oc.AdminKubeClient().CoreV1().Pods(netobservPrivilegedNS).List(ctx, metav1.ListOptions{
			LabelSelector: "app=netobserv-ebpf-agent",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ebpfPods.Items).NotTo(o.BeEmpty(), "expected eBPF agent pods in %s", netobservPrivilegedNS)
		for _, pod := range ebpfPods.Items {
			o.Expect(string(pod.Status.Phase)).To(o.Equal("Running"),
				"eBPF agent pod %s should be Running", pod.Name)
		}

		g.By("checking console plugin if deployed")
		pluginPods, err := oc.AdminKubeClient().CoreV1().Pods(netobservNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=netobserv-plugin",
		})
		if err == nil && len(pluginPods.Items) > 0 {
			for _, pod := range pluginPods.Items {
				o.Expect(string(pod.Status.Phase)).To(o.Equal("Running"),
					"console plugin pod %s should be Running", pod.Name)
			}

			pluginOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				"consoleplugin", "netobserv-plugin",
				"-o=jsonpath={.metadata.name}",
			).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(pluginOutput).To(o.Equal("netobserv-plugin"))
		} else {
			framework.Logf("Console plugin not deployed, skipping console plugin checks")
		}

		g.By("checking operator logs for excessive errors")
		logOutput, err := oc.AsAdmin().WithoutNamespace().Run("logs").Args(
			"-n", netobservOperatorNamespace,
			"deployment/netobserv-controller-manager",
			"--tail=50",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		errorLines := []string{}
		for _, line := range strings.Split(logOutput, "\n") {
			if strings.Contains(line, "\"level\":\"error\"") || strings.Contains(line, "level=error") {
				errorLines = append(errorLines, line)
			}
		}
		if len(errorLines) > 5 {
			framework.Logf("WARNING: found %d error lines in operator logs:\n%s",
				len(errorLines), strings.Join(errorLines[:5], "\n"))
		}

		g.By("checking ServiceMonitors exist")
		smOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"servicemonitor", "-n", netobservNamespace,
			"-o=jsonpath={.items[*].metadata.name}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(smOutput)).NotTo(o.BeEmpty(),
			"expected at least one ServiceMonitor in %s", netobservNamespace)
		framework.Logf("ServiceMonitors in %s: %s", netobservNamespace, smOutput)

		smPrivOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"servicemonitor", "-n", netobservPrivilegedNS,
			"-o=jsonpath={.items[*].metadata.name}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(smPrivOutput)).NotTo(o.BeEmpty(),
			"expected at least one ServiceMonitor in %s", netobservPrivilegedNS)
		framework.Logf("ServiceMonitors in %s: %s", netobservPrivilegedNS, smPrivOutput)

		g.By("checking alert rules are deployed")
		rulesOutput, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"prometheusrules", "-n", netobservNamespace,
			"-o=jsonpath={.items[*].metadata.name}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(rulesOutput)).NotTo(o.BeEmpty(),
			"expected PrometheusRules in %s", netobservNamespace)
		framework.Logf("PrometheusRules in %s: %s", netobservNamespace, rulesOutput)

		g.By("verifying FLP is producing and processing flow data")
		flpPods, err := oc.AdminKubeClient().CoreV1().Pods(netobservNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: "app=flowlogs-pipeline",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(flpPods.Items).NotTo(o.BeEmpty(), "no FLP pods found")

		flpPod := flpPods.Items[0].Name
		o.Eventually(func() bool {
			metricsOutput, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(
				"-n", netobservNamespace, flpPod, "--",
				"curl", "-s", fmt.Sprintf("http://localhost:%s/metrics", flpMetricsPort),
			).Output()
			if err != nil {
				framework.Logf("Error querying FLP metrics: %v", err)
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

		g.By("verifying Prometheus is scraping NetObserv metrics")
		promPods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-monitoring").List(ctx, metav1.ListOptions{
			LabelSelector: "app.kubernetes.io/name=prometheus",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
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
				framework.Logf("Error querying Prometheus: %v", err)
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
				framework.Logf("Error parsing Prometheus response: %v", err)
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
			"Prometheus should have netobserv_ingest_flows_processed results")
	})
})