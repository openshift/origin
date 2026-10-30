package security

import (
	"context"
	"fmt"
	"os"
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

const fipsFile = "/proc/sys/crypto/fips_enabled"

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

// discoverHostedCluster finds the HostedCluster name and namespace on the
// management cluster that corresponds to the given hosted control plane namespace.
// The HCP namespace follows the convention {hcNS}-{hcName}.
func discoverHostedCluster(mgmtCLI *exutil.CLI, hcpNS string) (string, string, error) {
	output, err := mgmtCLI.AsAdmin().WithoutNamespace().Run("get").Args(
		"hostedclusters", "-A",
		"-o", `jsonpath={range .items[*]}{.metadata.namespace},{.metadata.name}{"\n"}{end}`,
	).Output()
	if err != nil {
		return "", "", fmt.Errorf("failed to list HostedClusters: %v", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, ",", 2)
		if len(parts) == 2 {
			ns, name := parts[0], parts[1]
			if ns+"-"+name == hcpNS {
				return name, ns, nil
			}
		}
	}
	return "", "", fmt.Errorf("could not find HostedCluster matching HCP namespace %s", hcpNS)
}

func isHostedClusterFIPS() (bool, error) {
	mgmtCLI := exutil.NewHypershiftManagementCLI("fips-mgmt")
	_, hcpNamespace, err := exutil.GetHypershiftManagementClusterConfigAndNamespace()
	if err != nil {
		return false, err
	}

	hcName, hcNS, err := discoverHostedCluster(mgmtCLI, hcpNamespace)
	if err != nil {
		return false, err
	}

	fipsValue, err := mgmtCLI.AsAdmin().WithoutNamespace().Run("get").Args(
		"hostedcluster", hcName, "-n", hcNS,
		"-ojsonpath={.spec.fips}",
	).Output()
	if err != nil {
		return false, fmt.Errorf("failed to get .spec.fips from HostedCluster %s/%s: %v", hcNS, hcName, err)
	}

	return strconv.ParseBool(fipsValue)
}

var _ = g.Describe("[sig-arch] [Conformance] FIPS", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLIWithPodSecurityLevel("fips", admissionapi.LevelPrivileged)

	g.It("TestFIPS", func() {
		controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		clusterAdminKubeClientset := oc.AdminKubeClient()

		var isFIPS bool
		if *controlPlaneTopology == configv1.ExternalTopologyMode {
			if os.Getenv("HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG") == "" || os.Getenv("HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE") == "" {
				g.Skip("HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG and HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE must be set for FIPS test on HyperShift")
			}
			isFIPS, err = isHostedClusterFIPS()
			o.Expect(err).NotTo(o.HaveOccurred())
		} else {
			isFIPS, err = exutil.IsFIPS(clusterAdminKubeClientset.CoreV1())
			o.Expect(err).NotTo(o.HaveOccurred())

			masterNodes, err := clusterAdminKubeClientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{
				LabelSelector: "node-role.kubernetes.io/master",
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			masterNode := &masterNodes.Items[0]
			err = validateFIPSOnNode(oc, isFIPS, masterNode)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

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
