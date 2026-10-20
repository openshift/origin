package cpu_partitioning

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	ocpv1 "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/test/e2e/framework"
)

// When using the workload partitioning feature ( which allows cluster admins to limit the number of CPUs the platform pods have access to ),
// with BestEffort Pods, the probability of CPU time contention goes up and resource.requests become more important for platform pods.
// It is strongly recommended that for everything but your least important pods you should supply adequate resource.requests,
// otherwise you might be in scenarios where your pod might be starved for CPU time since BestEffort pods get a default of 2 CPU Shares.
// In other words under a highly utilized CPU in a CPU pinned cluster would mean your pod would only get 2/1024 * 100 = ~0.19% CPU time.
//
// Please be sure to verify that the deployment can function normally if these pods don't get a chance to complete their work.
//
// The below exclude lists are used to skip resource checks on these objects.
// the map is string(<resource-name>):[]string(<resource-namespace-substring>).
// Any pod resource that matches the name in the map and the substring of the namespace in the array is skipped.
var (
	excludedBestEffortDeployments = map[string][]string{
		"egress-router-cni-deployment": {"openshift-multus", "egress-router-cni-e2e"},

		// Managed services exceptions OSD-26068
		"addon-operator-manager":                    {"openshift-addon-operator"},
		"addon-operator-webhooks":                   {"openshift-addon-operator"},
		"custom-domains-operator":                   {"openshift-custom-domains-operator"},
		"deployment-validation-operator":            {"openshift-deployment-validation-operator"},
		"managed-node-metadata-operator":            {"openshift-managed-node-metadata-operator"},
		"managed-upgrade-operator":                  {"openshift-managed-upgrade-operator"},
		"configure-alertmanager-operator":           {"openshift-monitoring"},
		"must-gather-operator":                      {"openshift-must-gather-operator"},
		"obo-prometheus-operator-admission-webhook": {"openshift-observability-operator"},
		"observability-operator":                    {"openshift-observability-operator"},
		"ocm-agent":                                 {"openshift-ocm-agent-operator"},
		"ocm-agent-operator":                        {"openshift-ocm-agent-operator"},
		"osd-metrics-exporter":                      {"openshift-osd-metrics"},
		"package-operator-manager":                  {"openshift-package-operator"},
		"rbac-permissions-operator":                 {"openshift-rbac-permissions"},
		"blackbox-exporter":                         {"openshift-route-monitor-operator"},
		"route-monitor-operator-controller-manager": {"openshift-route-monitor-operator"},
		"splunk-forwarder-operator":                 {"openshift-splunk-forwarder-operator"},
		"cloud-ingress-operator":                    {"openshift-cloud-ingress-operator"},
		"managed-velero-operator":                   {"openshift-velero"},
		"velero":                                    {"openshift-velero"},

		"gateway": {"openshift-ingress"},
	}

	excludedBestEffortDaemonSets = map[string][]string{
		"cni-sysctl-allowlist-ds": {"openshift-multus", "egress-router-cni-e2e"},

		// Managed services OSD-26068
		"audit-exporter":     {"openshift-security"},
		"splunkforwarder-ds": {"openshift-security"},
		"validation-webhook": {"openshift-validation-webhook"},
	}
)

func isExcluded(list map[string][]string, namespace, name string) bool {
	if value, ok := list[name]; ok {
		for _, subStringNamespace := range value {
			if strings.Contains(namespace, subStringNamespace) {
				return true
			}
		}
	}
	return false
}

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] CPU Partitioning cluster platform workloads", func() {
	defer g.GinkgoRecover()

	var (
		oc                      = exutil.NewCLIWithoutNamespace("cpu-partitioning").AsAdmin()
		ctx                     = context.Background()
		isClusterCPUPartitioned = false

		messageFormat = expectedMessage
		matcher       = o.And(
			o.HaveKey(workloadAnnotations),
			o.HaveKey(o.MatchRegexp(workloadAnnotationsRegex)),
		)
	)

	g.BeforeEach(func() {
		isClusterCPUPartitioned = getCpuPartitionedStatus(oc) == ocpv1.CPUPartitioningAllNodes
		matcher, messageFormat = adjustMatcherAndMessageForCluster(isClusterCPUPartitioned, matcher)
	})

	g.It("should be annotated correctly for Deployments", func() {

		deployments, err := oc.KubeClient().AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var deploymentErrors []error
		for _, deployment := range deployments.Items {
			if deployment.Namespace == "openshift-ingress" && strings.HasPrefix(deployment.Name, "gateway-") {
				// The gateway deployment's name contains a hash, which
				// must be removed in order to be able to define an
				// exception.  Remove this if block when the
				// corresponding exception is removed.
				deployment.Name = "gateway"
			}
			// If we find a deployment that is to be excluded from resource checks, we skip looking for their pods.
			if isExcluded(excludedBestEffortDeployments, deployment.Namespace, deployment.Name) {
				framework.Logf("skipping resource check on deployment (%s/%s) due to presence in BestEffort exclude list", deployment.Namespace, deployment.Name)
				continue
			}

			checkPods, err := shouldCheckPlatformWorkload(ctx, oc.KubeClient().CoreV1().Namespaces(), deployment.Namespace, deployment.Spec.Template.Annotations, isClusterCPUPartitioned)
			if err != nil {
				deploymentErrors = append(deploymentErrors, fmt.Errorf("checking deployment %s/%s: %w", deployment.Namespace, deployment.Name, err))
				continue
			}
			if checkPods {
				pods, err := oc.KubeClient().CoreV1().Pods(deployment.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(deployment.Spec.Template.Labels).String(),
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				for _, pod := range pods.Items {

					o.Expect(pod.Annotations).To(
						matcher, "pod (%s/%s) %s have workload annotations", pod.Namespace, pod.Name, messageFormat)

					for _, container := range pod.Spec.Containers {
						_, ok := container.Resources.Limits[resourceLabel]
						o.Expect(ok).To(o.Equal(isClusterCPUPartitioned),
							"limits resources %s be present for container %s in pod %s/%s", messageFormat, container.Name, pod.Name, pod.Namespace)
						_, ok = container.Resources.Requests[resourceLabel]
						o.Expect(ok).To(o.Equal(isClusterCPUPartitioned),
							"requests resources %s be present for container %s in pod %s/%s", messageFormat, container.Name, pod.Name, pod.Namespace)
					}
				}
			}
		}
		o.Expect(deploymentErrors).To(o.BeEmpty())
	})

	g.It("should be annotated correctly for DaemonSets", func() {

		daemonsets, err := oc.KubeClient().AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		var daemonsetErrors []error
		for _, daemonset := range daemonsets.Items {
			// If we find a daemonset that is to be excluded from resource checks, we skip looking for their pods.
			if isExcluded(excludedBestEffortDaemonSets, daemonset.Namespace, daemonset.Name) {
				framework.Logf("skipping resource check on daemonset (%s/%s) due to presence in BestEffort exclude list", daemonset.Namespace, daemonset.Name)
				continue
			}

			checkPods, err := shouldCheckPlatformWorkload(ctx, oc.KubeClient().CoreV1().Namespaces(), daemonset.Namespace, daemonset.Spec.Template.Annotations, isClusterCPUPartitioned)
			if err != nil {
				daemonsetErrors = append(daemonsetErrors, fmt.Errorf("checking daemonset %s/%s: %w", daemonset.Namespace, daemonset.Name, err))
				continue
			}
			if checkPods {
				pods, err := oc.KubeClient().CoreV1().Pods(daemonset.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: labels.SelectorFromSet(daemonset.Spec.Template.Labels).String(),
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				for _, pod := range pods.Items {

					o.Expect(pod.Annotations).To(
						matcher, "pod (%s/%s) %s have workload annotations", pod.Namespace, pod.Name, messageFormat)

					for _, container := range pod.Spec.Containers {
						_, ok := container.Resources.Limits[resourceLabel]
						o.Expect(ok).To(o.Equal(isClusterCPUPartitioned),
							"limits resources %s be present for container %s in pod %s/%s", messageFormat, container.Name, pod.Name, pod.Namespace)
						_, ok = container.Resources.Requests[resourceLabel]
						o.Expect(ok).To(o.Equal(isClusterCPUPartitioned),
							"requests resources %s be present for container %s in pod %s/%s", messageFormat, container.Name, pod.Name, pod.Namespace)
					}
				}
			}
		}
		o.Expect(daemonsetErrors).To(o.BeEmpty())
	})
})
