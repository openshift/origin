package windows

import (
	"context"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// NodeLabelSelector is a label selector that can be used to filter Windows nodes
	NodeLabelSelector = corev1.LabelOSStable + "=windows"
	// VersionAnnotation indicates the version of WMCO that configured the node
	VersionAnnotation = "windowsmachineconfig.openshift.io/version"
)

var _ = g.Describe("[sig-windows] Nodes", func() {
	defer g.GinkgoRecover()

	var windowsNodes []corev1.Node

	oc := exutil.NewCLIWithoutNamespace("windows")

	g.BeforeEach(func() {
		// fetch the list of Windows nodes
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(context.Background(),
			metav1.ListOptions{LabelSelector: NodeLabelSelector})
		o.Expect(err).ToNot(o.HaveOccurred())
		// update the windowsNodes variable to the list of Windows nodes
		windowsNodes = nodes.Items
		// skip if no Windows nodes
		if len(windowsNodes) < 1 {
			g.Skip("Skip. No Windows node found on the test cluster.")
		}
	})

	g.It("should fail with invalid version annotation", g.Label("Size:S"), func() {
		g.By("process version annotation")
		for _, node := range windowsNodes {
			annotations := node.GetAnnotations()
			v, ok := annotations[VersionAnnotation]
			if !ok {
				e2e.Failf("missing version annotation on node %s", node.Name)
			}
			if strings.TrimSpace(v) == "" {
				e2e.Failf("emtpy version annotation on node %s", node.Name)
			}
		}
	})
})
