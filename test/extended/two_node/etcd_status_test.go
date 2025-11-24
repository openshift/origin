package two_node

import (
	g "github.com/onsi/ginkgo/v2"
	"github.com/openshift/origin/test/extended/two_node/utils"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-etcd] Etcd Status Test", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithoutNamespace("etcd-status").AsAdmin()

	g.It("Should report etcd cluster status", func() {
		err := logEtcdClusterStatus(oc, "test status check")
		if err != nil {
			g.GinkgoT().Printf("Warning: %v", err)
		}
	})
})

// logEtcdClusterStatus logs the current status of the etcd cluster
func logEtcdClusterStatus(oc *exutil.CLI, context string) error {
	g.GinkgoT().Printf("=== Etcd Cluster Status (%s) ===", context)

	// Use the utils package function that already exists
	return utils.LogEtcdClusterStatus(oc, context)
}
