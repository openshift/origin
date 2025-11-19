package node

import (
	g "github.com/onsi/ginkgo/v2"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node] Kubelet, CRI-O, CPU manager", func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("node").AsAdmin()
	)

	//author: asahay@redhat.com
	g.It("check KUBELET_LOG_LEVEL is 2", func() {
		g.By("check Kubelet Log Level\n")
		assertKubeletLogLevel(oc)
	})
})
