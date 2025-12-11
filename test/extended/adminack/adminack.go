package adminack

import (
	"context"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/openshift/clusterversionoperator"

	"k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-cluster-lifecycle]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("cli-deployment")

	g.Describe("TestAdminAck", func() {
		g.It("should succeed [apigroup:config.openshift.io]", g.Label("Size:M"), func() {
			config, err := framework.LoadConfig()
			o.Expect(err).NotTo(o.HaveOccurred())
			ctx := context.Background()

			adminAckTest := &clusterversionoperator.AdminAckTest{Oc: oc, Config: config}
			adminAckTest.Test(ctx)
		})
	})
})
