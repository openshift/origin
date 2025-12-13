package security

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	admissionapi "k8s.io/pod-security-admission/api"

	configv1 "github.com/openshift/api/config/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	installConfigName = "cluster-config-v1"
	fipsFile          = "/proc/sys/crypto/fips_enabled"
)

func validateFIPSOnNode(oc *exutil.CLI, fipsExpected bool, node *corev1.Node) error {
	// The oc debug output prints a bunch of info messages and possible warnings (the latter can not be disabled).
	// Echo a prefix to be able to identify the line with our output.
	const commandLineIdentifierPrefix = "fips-command"
	out, err := oc.AsAdmin().Run("debug").Args("node/"+node.Name, "--", "/bin/bash", "-c", "echo -n "+commandLineIdentifierPrefix+" && cat "+fipsFile).Output()
	if err != nil {
		return err
	}
	var outFiltered string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, commandLineIdentifierPrefix) {
			outFiltered = strings.TrimPrefix(line, commandLineIdentifierPrefix)
			break
		}
	}
	nodeFips, err := strconv.ParseBool(outFiltered)
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
	oc := exutil.NewCLIWithPodSecurityLevel("fips", admissionapi.LevelPrivileged)

	g.It("TestFIPS", g.Label("Size:S"), func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		clusterAdminKubeClientset := oc.AdminKubeClient()
		isFIPS, err := exutil.IsFIPS(clusterAdminKubeClientset.CoreV1())
		o.Expect(err).NotTo(o.HaveOccurred())

		// fetch one control plane and one worker, and validate FIPS state on it.
		// skip the controlplane node verification when external controlPlaneTopology as
		// there are no controlplane nodes.
		if *controlPlaneTopology != configv1.ExternalTopologyMode {
			masterNodes, err := clusterAdminKubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			masterNode := &masterNodes.Items[0]
			err = validateFIPSOnNode(oc, isFIPS, masterNode)
		}
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
