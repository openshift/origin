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
)

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

		var (
			deploymentErr []error
		)

		deployments, err := oc.KubeClient().AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, deployment := range deployments.Items {
			if _, ok := deployment.Spec.Template.Annotations[workloadAnnotations]; ok {

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
			} else if strings.HasPrefix(deployment.Namespace, "openshift-") && !strings.HasPrefix(deployment.Namespace, "openshift-e2e-") {
				deploymentErr = append(deploymentErr, fmt.Errorf("deployment (%s) in openshift namespace (%s) must have pod templates annotated with %s",
					deployment.Name, deployment.Namespace, deploymentPodAnnotation))
			}
		}
		o.Expect(deploymentErr).To(o.BeEmpty())
	})

	g.It("should be annotated correctly for DaemonSets", func() {

		var (
			daemonsetErr []error
		)

		daemonsets, err := oc.KubeClient().AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, daemonset := range daemonsets.Items {
			if _, ok := daemonset.Spec.Template.Annotations[workloadAnnotations]; ok {

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
			} else if strings.HasPrefix(daemonset.Namespace, "openshift-") && !strings.HasPrefix(daemonset.Namespace, "openshift-e2e-") {
				daemonsetErr = append(daemonsetErr, fmt.Errorf("daemonset (%s) in openshift namespace (%s) must have pod templates annotated with %s",
					daemonset.Name, daemonset.Namespace, deploymentPodAnnotation))
			}
		}
		o.Expect(daemonsetErr).To(o.BeEmpty())
	})
})
