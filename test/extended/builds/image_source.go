package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"github.com/openshift/origin/test/extended/images"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("builds: image source", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("fixtures", "test-build-hello-openshift.yaml")
		helloBuilder = exutil.FixturePath("fixtures", "hello-builder")
		oc           = exutil.NewCLI("build-image-source", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("build with image source", func() {
		g.It("should complete successfully and deploy resulting image", func() {
			g.By("Creating build configs, deployment config, and service for hello-openshift")
			err := oc.Run("create").Args("-f", buildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("starting the builder image build with a directory")
			err = oc.Run("start-build").Args("hello-builder", fmt.Sprintf("--from-dir=%s", helloBuilder)).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("expect the builds to complete successfully and deploy a hello-openshift pod")
			success, err := images.CheckPageContains(oc, "hello-openshift", "", "Hello OpenShift!")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(success).To(o.BeTrue())
		})

	})
})
