package cli

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	podLabelTemplate = `{{index .metadata.labels "new-label"}}`
)

var _ = g.Describe("[sig-cli] oc label", func() {
	defer g.GinkgoRecover()

	var (
		oc       = exutil.NewCLI("oc-label").AsAdmin()
		helloPod = exutil.FixturePath("..", "..", "examples", "hello-openshift", "hello-pod.json")
	)

	g.It("pod", func() {
		g.By("creating hello-openshift pod")
		out, err := oc.Run("create").Args("-f", helloPod).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("pod/hello-openshift created"))

		g.By("setting a new label")
		out, err = oc.Run("label").Args("pod", "hello-openshift", "new-label=hello").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("pod/hello-openshift labeled"))

		g.By("validating new label")
		out, err = oc.Run("get").Args("pod", "hello-openshift", "--template", podLabelTemplate).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("hello"))
	})
})
