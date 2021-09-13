package cli

import (
	"os"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	podLabelTemplate = `{{index .metadata.labels "new-label"}}`
)

var _ = g.Describe("[sig-cli] oc label", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("oc-label")

	g.It("pod", func() {
		g.By("creating hello-openshift pod")
		helloPodFile, err := writeObjectToFile(newHelloPod())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(helloPodFile)

		err = oc.Run("create").Args("-f", helloPodFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("setting a new label")
		out, err := oc.Run("label").Args("pod", "hello-openshift", "new-label=hello").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("pod/hello-openshift labeled"))

		g.By("validating new label")
		out, err = oc.Run("get").Args("pod", "hello-openshift", "--template", podLabelTemplate).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("hello"))

		g.By("removing the label")
		out, err = oc.Run("label").Args("pod", "hello-openshift", "new-label-").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("pod/hello-openshift labeled"))

		g.By("validating missing label")
		out, err = oc.Run("get").Args("pod", "hello-openshift", "--template", podLabelTemplate).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("<no value>"))

		g.By("setting empty label")
		out, err = oc.Run("label").Args("pod", "hello-openshift", `new-label=`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("pod/hello-openshift labeled"))

		g.By("validating empty label")
		out, err = oc.Run("get").Args("pod", "hello-openshift", "--template", podLabelTemplate).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.BeEmpty())
	})
})
