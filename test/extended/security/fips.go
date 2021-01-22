package security

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	installConfigName = "cluster-config-v1"
	fipsFile          = "/proc/sys/crypto/fips_enabled"
)

func validateFIPSOnNode(oc *exutil.CLI, fipsExpected bool, node *corev1.Node) error {
	command := []string{"cat", fipsFile}
	out, err := exutil.ExecCommandOnMachineConfigDaemon(oc.AdminKubeClient(), oc, node, command)
	if err != nil {
		return err
	}
	nodeFips, err := strconv.ParseBool(strings.TrimSuffix(string(out), "\n"))
	if err != nil {
		return fmt.Errorf("Error parsing %s on node %s: %v", fipsFile, node.Name, err)
	}
	if nodeFips != fipsExpected {
		return fmt.Errorf("Expected FIPS state %v, found %v", fipsExpected, nodeFips)
	}
	return nil
}

var _ = g.Describe("[sig-arch] [Conformance] FIPS", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("fips")

	g.It("TestFIPS", func() {
		clusterAdminKubeClientset := oc.AdminKubeClient()
		isFIPS, err := exutil.IsFIPS(clusterAdminKubeClientset.CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		// fetch one control plane and one worker, and validate FIPS state on it
		masterNodes, err := clusterAdminKubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/master",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		masterNode := &masterNodes.Items[0]
		err = validateFIPSOnNode(oc, isFIPS, masterNode)
		o.Expect(err).NotTo(o.HaveOccurred())
		workerNodes, err := clusterAdminKubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
			LabelSelector: "node-role.kubernetes.io/worker",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		if len(workerNodes.Items) > 0 {
			workerNode := &workerNodes.Items[0]
			err = validateFIPSOnNode(oc, isFIPS, workerNode)
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})
})
