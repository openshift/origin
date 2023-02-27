package cpu_partitioning

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-node] CPU Partitioned cluster infrastructure", func() {
	defer g.GinkgoRecover()

	var (
		oc  = exutil.NewCLIWithoutNamespace("cpu-partitioning").AsAdmin()
		ctx = context.Background()

		// The below namespaces are not annotated,
		// no workload is going to be running in them.
		ignoreNamespaces = map[string]struct{}{
			"openshift-config":         {},
			"openshift-config-managed": {},
			"openshift-node":           {},
		}
	)

	g.BeforeEach(func() {
		skipNonCPUPartitionedCluster(oc)
	})

	g.It("should be configured correctly", func() {

		g.By("containing cpu partitioning bootstrap files", func() {
			outputString, err := oc.Run("get", "mc").Args("-o", "name").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(outputString).To(
				o.And(
					o.ContainSubstring("01-master-cpu-partitioning"),
					o.ContainSubstring("01-worker-cpu-partitioning"),
				),
			)
		})

		g.By("having nodes configured with correct Capacity and Allocatable", func() {
			nodes, err := oc.KubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, node := range nodes.Items {
				_, ok := node.Status.Capacity[resourceLabel]
				o.Expect(ok).To(o.BeTrue(), "capacity is missing from node %s", node.Name)
				_, ok = node.Status.Allocatable[resourceLabel]
				o.Expect(ok).To(o.BeTrue(), "allocatable is missing from node %s", node.Name)
			}
		})

		g.By("having openshift namespaces with management annotation", func() {
			projects, err := oc.ProjectClient().ProjectV1().Projects().List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			invalidNamespaces := []string{}
			for _, project := range projects.Items {
				if _, ok := ignoreNamespaces[project.Name]; ok {
					continue
				}
				if strings.HasPrefix(project.Name, "openshift-") {
					if v, ok := project.Annotations[namespaceAnnotationKey]; !ok || v != "management" {
						invalidNamespaces = append(invalidNamespaces, project.Name)
					}
				}
			}
			o.Expect(invalidNamespaces).To(o.BeEmpty(),
				"projects %s do not contain the annotation %s", invalidNamespaces, namespaceAnnotation)
		})
	})
})
