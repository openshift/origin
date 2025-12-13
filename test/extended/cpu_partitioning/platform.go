package cpu_partitioning

import (
	"context"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/sets"

	ocpv1 "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-node][apigroup:config.openshift.io] CPU Partitioning cluster infrastructure", func() {
	defer g.GinkgoRecover()

	var (
		oc                      = exutil.NewCLIWithoutNamespace("cpu-partitioning").AsAdmin()
		ctx                     = context.Background()
		isClusterCPUPartitioned = false

		ignoreNamespaces = sets.New(
			// The below namespaces are not annotated,
			// no workload is going to be running in them.
			"openshift-config",
			"openshift-config-managed",
			"openshift-node",
		).Union(exutil.ManagedServiceNamespaces) // Managed service namespaces OSD-26068
	)

	g.BeforeEach(func() {
		isClusterCPUPartitioned = getCpuPartitionedStatus(oc) == ocpv1.CPUPartitioningAllNodes
	})

	g.It("should be configured correctly", g.Label("Size:M"), func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		if *controlPlaneTopology == ocpv1.ExternalTopologyMode {
			g.Skip("Clusters with external control plane topology do not support MachineConfigs")
		}

		mcMatcher := o.And(
			o.ContainSubstring("01-master-cpu-partitioning"),
			o.ContainSubstring("01-worker-cpu-partitioning"),
		)

		mcMatcher, messageFormat := adjustMatcherAndMessageForCluster(isClusterCPUPartitioned, mcMatcher)

		g.By("containing cpu partitioning bootstrap files", func() {
			outputString, err := oc.Run("get", "mc").Args("-o", "name").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(outputString).To(mcMatcher, "cluster is %s contain bootstrap files", messageFormat)
		})

		g.By("having nodes configured with correct Capacity and Allocatable", func() {
			nodes, err := oc.KubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			var nodeErrs []error
			for _, node := range nodes.Items {
				if _, ok := node.Status.Capacity[resourceLabel]; ok != isClusterCPUPartitioned {
					nodeErrs = append(nodeErrs, fmt.Errorf("capacity %s be present from node %s", messageFormat, node.Name))
				}
				if _, ok := node.Status.Allocatable[resourceLabel]; ok != isClusterCPUPartitioned {
					nodeErrs = append(nodeErrs, fmt.Errorf("allocatable %s be present from node %s", messageFormat, node.Name))
				}
			}
			o.Expect(nodeErrs).To(o.BeEmpty())
		})

		g.By("having openshift namespaces with management annotation", func() {
			projects, err := oc.ProjectClient().ProjectV1().Projects().List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			invalidNamespaces := []string{}
			for _, project := range projects.Items {
				if ignoreNamespaces.Has(project.Name) {
					continue
				}
				if strings.HasPrefix(project.Name, "openshift-") && !strings.HasPrefix(project.Name, "openshift-e2e-") {
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
