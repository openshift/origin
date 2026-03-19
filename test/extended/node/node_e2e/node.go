package node

import (
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-node] [Jira:Node/Kubelet] Kubelet, CRI-O, CPU manager", func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("node")
	)

	// Skip all tests on MicroShift clusters as MachineConfig resources are not available
	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}
	})

	//author: asahay@redhat.com
	g.It("[OTP] validate KUBELET_LOG_LEVEL", func() {
		var kubeservice string
		var kubelet string
		var err error

		g.By("Polling to check kubelet log level on ready nodes")
		waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
			g.By("Getting all node names in the cluster")
			nodeName, nodeErr := oc.AsAdmin().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(nodeErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode Names are %v", nodeName)
			nodes := strings.Fields(nodeName)

			for _, node := range nodes {
				g.By("Checking if node " + node + " is Ready")
				nodeStatus, statusErr := oc.AsAdmin().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
				o.Expect(statusErr).NotTo(o.HaveOccurred())
				e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

				if nodeStatus == "True" {
					g.By("Checking KUBELET_LOG_LEVEL in kubelet.service on node " + node)
					kubeservice, err = nodeutils.ExecOnNodeWithChroot(oc, node, "/bin/bash", "-c", "systemctl show kubelet.service | grep KUBELET_LOG_LEVEL")
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Checking kubelet process for --v=2 flag on node " + node)
					kubelet, err = nodeutils.ExecOnNodeWithChroot(oc, node, "/bin/bash", "-c", "ps aux | grep [k]ubelet")
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Verifying KUBELET_LOG_LEVEL is set and kubelet is running with --v=2")
					if strings.Contains(kubeservice, "KUBELET_LOG_LEVEL") && strings.Contains(kubelet, "--v=2") {
						e2e.Logf("KUBELET_LOG_LEVEL is 2.\n")
						return true, nil
					} else {
						e2e.Logf("KUBELET_LOG_LEVEL is not 2.\n")
						return false, nil
					}
				} else {
					e2e.Logf("\nNode %s is not Ready, Skipping\n", node)
				}
			}
			return false, nil
		})

		if waitErr != nil {
			e2e.Logf("Kubelet Log level is:\n %v\n", kubeservice)
			e2e.Logf("Running Process of kubelet are:\n %v\n", kubelet)
		}
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "KUBELET_LOG_LEVEL is not expected, timed out")
	})

	//author: cmaurya@redhat.com
	g.It("[OTP] validate cgroupv2 is default", func() {
		g.By("1) Get a Ready worker node and check cgroup version")
		workerNode, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-o=jsonpath={.items[0].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		nodeStatus, err := oc.AsAdmin().Run("get").Args("nodes", workerNode, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(nodeStatus).To(o.Equal("True"), "Worker node %s is not Ready", workerNode)
		cgroupV, err := nodeutils.ExecOnNodeWithChroot(oc, workerNode, "/bin/bash", "-c", "stat -c %T -f /sys/fs/cgroup")
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("cgroup version info is: [%v]\n", cgroupV)
		o.Expect(cgroupV).To(o.ContainSubstring("cgroup2fs"))

		g.By("2) Changing cgroup from v2 to v1 should result in error")
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("nodes.config.openshift.io", "cluster", "-p", `{"spec": {"cgroupMode": "v1"}}`, "--type=merge").Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("spec.cgroupMode: Unsupported value: \"v1\": supported values: \"v2\", \"\""))
	})
})
