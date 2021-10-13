package adminack

import (
	"context"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"

	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("cli-deployment")

	g.Describe("TestAdminAck", func() {
		g.It("should succeed", func() {
			config, err := framework.LoadConfig()
			o.Expect(err).NotTo(o.HaveOccurred())
			ctx := context.Background()

			adminAckTest := &exutil.AdminAckTest{Oc: oc, Config: config}
			adminAckTest.Test(ctx)
		})
	})
})
