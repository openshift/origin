package dr

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-etcd][Feature:DisasterRecovery][Suite:openshift/etcd/recovery]", func() {
	defer g.GinkgoRecover()

	f := framework.NewDefaultFramework("recovery")
	f.SkipNamespaceCreation = true

	oc := exutil.NewCLIWithoutNamespace("recovery")

	g.It("[Feature:EtcdRecovery] should install ssh keys on CP nodes", func() {
		err := InstallSSHKeyOnControlPlaneNodes(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
	})
})
