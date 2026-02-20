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

	//author: asahay@redhat.com
	g.It("[OTP] validate KUBELET_LOG_LEVEL", func() {
		var kubeservice string
		var kubelet string
		var err error

		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		if err != nil {
			o.Expect(err).NotTo(o.HaveOccurred(), "error determining if running on MicroShift: %v", err)
		}
		if isMicroShift {
			g.Skip("This test case is not supported in micoshift cluster ")
		}

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
})
