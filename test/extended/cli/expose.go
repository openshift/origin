package cli

import (
	"os"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc expose", func() {
	defer g.GinkgoRecover()

	var (
		oc                  = exutil.NewCLI("oc-expose")
		externalServiceFile = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "external-service.yaml")
		multiportSvcFile    = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "multiport-service.yaml")
	)

	g.It("can ensure the expose command is functioning as expected [apigroup:route.openshift.io]", g.Label("Size:M"), func() {
		frontendServiceFile, err := writeObjectToFile(newFrontendService())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(frontendServiceFile)

		g.By("creating a new service")
		err = oc.Run("create").Args("-f", frontendServiceFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking for expected failure conditions")
		err = oc.Run("expose").Args("service", "frontend", "--create-external-load-balancer").Execute()
		o.Expect(err).To(o.HaveOccurred())

		err = oc.Run("expose").Args("service", "frontend", "--port=40", "--type=NodePort").Execute()
		o.Expect(err).To(o.HaveOccurred())

		g.By("checking for expected success conditions")
		err = oc.Run("expose").Args("service", "frontend", "--path=/test").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := oc.Run("get").Args("route.v1.route.openshift.io", "frontend", "--template='{{.spec.path}}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("/test"))

		out, err = oc.Run("get").Args("route.v1.route.openshift.io", "frontend", "--template='{{.spec.to.name}}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("frontend"))

		out, err = oc.Run("get").Args("route.v1.route.openshift.io", "frontend", "--template='{{.spec.port.targetPort}}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("9998"))

		err = oc.Run("delete").Args("svc,route", "-l", "name=frontend").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking that external services are exposable")
		err = oc.Run("create").Args("-f", externalServiceFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.Run("delete").Args("-f", externalServiceFile).Execute()

		err = oc.Run("expose").Args("svc/external").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("route", "external").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("external"))

		err = oc.Run("delete").Args("route", "external").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("delete").Args("service", "external").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking that multiport services are in the route")
		err = oc.Run("create").Args("-f", multiportSvcFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.Run("delete").Args("-f", multiportSvcFile).Execute()

		err = oc.Run("expose").Args("svc/frontend", "--name", "route-with-set-port").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("route", "route-with-set-port", "--template='{{.spec.port.targetPort}}'").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("web"))
	})
})
