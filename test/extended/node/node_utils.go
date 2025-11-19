package node

import (
	"strings"
	"time"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	compat_otp "github.com/openshift/origin/test/extended/util/compat_otp"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

func assertKubeletLogLevel(oc *exutil.CLI) {
	var kubeservice string
	var kublet string
	var err error
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
				kubeservice, err = compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "systemctl show kubelet.service | grep KUBELET_LOG_LEVEL")
				o.Expect(err).NotTo(o.HaveOccurred())
				kublet, err = compat_otp.DebugNodeWithChroot(oc, node, "/bin/bash", "-c", "ps aux | grep kubelet")
				o.Expect(err).NotTo(o.HaveOccurred())

				if strings.Contains(string(kubeservice), "KUBELET_LOG_LEVEL") && strings.Contains(string(kublet), "--v=2") {
					e2e.Logf(" KUBELET_LOG_LEVEL is 2. \n")
					return true, nil
				} else {
					e2e.Logf(" KUBELET_LOG_LEVEL is not 2. \n")
					return false, nil
				}
			} else {
				e2e.Logf("\n Node %s is not Ready, Skipping\n ", node)
			}
		}
		return false, nil
	})
	if waitErr != nil {
		e2e.Logf("Kubelet Log level is:\n %v\n", kubeservice)
		e2e.Logf("Running Proccess of kubelet are:\n %v\n", kublet)
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "KUBELET_LOG_LEVEL is not expected, timed out")
	}
}
