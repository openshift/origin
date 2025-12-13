package cli

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc api-resources", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("oc-api-resources")

	g.It("can output expected information about api-resources", g.Label("Size:S"), func() {
		out, err := oc.Run("api-resources").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("configmaps"))
		o.Expect(out).To(o.ContainSubstring("jobs"))
		o.Expect(out).To(o.ContainSubstring("nodes"))
		o.Expect(out).To(o.ContainSubstring("pods"))
		o.Expect(out).To(o.ContainSubstring("secrets"))
		o.Expect(out).To(o.ContainSubstring("services"))

		out, err = oc.Run("api-resources").Args("--namespaced=false").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("clusterroles"))
		o.Expect(out).To(o.ContainSubstring("namespaces"))
		o.Expect(out).To(o.ContainSubstring("nodes"))

		out, err = oc.Run("api-resources").Args("--namespaced=true").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("events"))
		o.Expect(out).To(o.ContainSubstring("serviceaccounts"))
		o.Expect(out).To(o.ContainSubstring("deployments"))

		out, err = oc.Run("api-versions").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("apps/v1"))
		o.Expect(out).To(o.ContainSubstring("networking.k8s.io/v1"))
		o.Expect(out).To(o.ContainSubstring("node.k8s.io/v1"))
		o.Expect(out).To(o.ContainSubstring("storage.k8s.io/v1"))
	})

	g.It("can output expected information about build.openshift.io api-resources [apigroup:build.openshift.io]", g.Label("Size:S"), func() {
		out, err := oc.Run("api-resources").Args("--api-group=build.openshift.io").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("builds"))
		o.Expect(out).To(o.ContainSubstring("buildconfigs"))
	})

	g.It("can output expected information about image.openshift.io api-resources [apigroup:image.openshift.io]", g.Label("Size:S"), func() {
		out, err := oc.Run("api-resources").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("images"))
		o.Expect(out).To(o.ContainSubstring("imagestreamtags"))
	})

	g.It("can output expected information about operator.openshift.io api-resources [apigroup:operator.openshift.io]", g.Label("Size:S"), func() {
		out, err := oc.Run("api-resources").Args("--verbs=get").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("configs"))
	})

	g.It("can output expected information about project.openshift.io api-resources [apigroup:project.openshift.io]", g.Label("Size:S"), func() {
		out, err := oc.Run("api-versions").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("project.openshift.io/v1"))
	})

	g.It("can output expected information about route.openshift.io api-resources and api-version [apigroup:route.openshift.io]", g.Label("Size:S"), func() {
		out, err := oc.Run("api-resources").Args("--verbs=get").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("routes"))

		out, err = oc.Run("api-versions").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("route.openshift.io/v1"))
	})

	g.It("can output expected information about snapshot.storage.k8s.io api-resources [apigroup:snapshot.storage.k8s.io]", g.Label("Size:S"), func() {
		out, err := oc.Run("api-resources").Args("--verbs=get").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("volumesnapshots"))
	})
})
