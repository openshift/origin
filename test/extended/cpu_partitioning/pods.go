package cpu_partitioning

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var _ = g.Describe("[sig-node] CPU Partitioned cluster platform pods", func() {
	defer g.GinkgoRecover()

	var (
		oc  = exutil.NewCLIWithoutNamespace("cpu-partitioning").AsAdmin()
		ctx = context.Background()
	)

	g.BeforeEach(func() {
		skipNonCPUPartitionedCluster(oc)
	})

	g.It("should be annotated correctly", func() {

		g.By("checking deployments and pods", func() {
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
							o.And(
								o.HaveKey(workloadAnnotations),
								o.HaveKey(o.MatchRegexp(workloadAnnotationsRegex)),
							), "pod (%s/%s) does not contain correct annotations", pod.Namespace, pod.Name)

						for _, container := range pod.Spec.Containers {
							_, ok := container.Resources.Limits[resourceLabel]
							o.Expect(ok).To(o.BeTrue(),
								"limits resources not present for container %s in pod %s/%s", container.Name, pod.Name, pod.Namespace)
							_, ok = container.Resources.Requests[resourceLabel]
							o.Expect(ok).To(o.BeTrue(),
								"requests resources not present for container %s in pod %s/%s", container.Name, pod.Name, pod.Namespace)
						}
					}
				} else if strings.HasPrefix(deployment.Namespace, "openshift-") {
					o.Expect(ok).To(o.BeTrue(), "deployments in openshift namespace must have pod templates annotated with %s", deploymentPodAnnotation)
				}
			}
		})
	})
})
