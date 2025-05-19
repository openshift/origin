package node

import (
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node][Feature:Remove support to configure Cgroup v1 from OCP version >= 4.19][apigroup:operator.openshift.io]", func() {

	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithoutNamespace("node").AsAdmin()
	)
	// Setup project to ensure a valid namespace exists
	oc.SetupProject()

	const cgroupV2 = "cgroup2fs"

	g.It("Should result in an error when cgroupMode is changed from v2 to v1", func() {

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		if err != nil {
			o.Expect(err).NotTo(o.HaveOccurred(), "error determining if running on MicroShift: %v", err)
		}
		if isMicroShift {
			g.Skip("This test case is not supported in micoshift cluster ")
		}

		g.By("1) Check all the Nodes are in Ready state")
		nodeStatusCheck(oc)

		g.By("2) Check cgroup Version")
		cgroupVersion := getCgroupVersion(oc)
		e2e.Logf("Detected cgroup version: %s", cgroupVersion)
		o.Expect(strings.Contains(cgroupVersion, cgroupV2)).Should(o.BeTrue())

		g.By("3) Try Modifying node config to set cgroup mode to v1 [should give error]")
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("nodes.config.openshift.io", "cluster", "-p", "{\"spec\": {\"cgroupMode\": \"v1\"}}", "--type=merge").Output()
		e2e.Logf("Cgroup version is %s", output)
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(strings.Contains(output, "Unsupported value: \"v1\"")).Should(o.BeTrue())

	})

})

func nodeStatusCheck(oc *exutil.CLI) {

	waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
		nodeName, nodeErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(nodeErr).NotTo(o.HaveOccurred())
		e2e.Logf("\nNode Names are %v", nodeName)
		nodes := strings.Fields(nodeName)

		for _, node := range nodes {
			nodeStatus, statusErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(statusErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

			if nodeStatus == "True" {
				e2e.Logf("\n NODES ARE READY\n ")

			} else {
				e2e.Logf("\n NODES ARE NOT READY\n ")
				return false, nil
			}
		}
		return true, nil

	})
	o.Expect(waitErr).NotTo(o.HaveOccurred(), "Nodes are NOT up and running")

}

func getCgroupVersion(oc *exutil.CLI) string {
	workerNodes := getWorkersList(oc)
	cgroupV, err := oc.AsAdmin().WithoutNamespace().Run("debug").Args("node/"+workerNodes[0], "--", "chroot", "/host", "stat", "-fc", "%T", "/sys/fs/cgroup").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Cgroup version output: %s", cgroupV)
	return cgroupV
}

func getWorkersList(oc *exutil.CLI) []string {
	output, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-o=jsonpath={.items[*].metadata.name}").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	return strings.Fields(output)
}
