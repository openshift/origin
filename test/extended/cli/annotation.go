package cli

import (
	"os"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc annotate", func() {
	defer g.GinkgoRecover()

	const (
		podAnnotationTemplate = `{{index .metadata.annotations "new-anno"}}`
	)

	var oc = exutil.NewCLI("oc-annotation")

	g.It("pod", func() {
		g.By("creating hello-openshift pod")
		helloPodFile, err := writeObjectToFile(newHelloPod())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(helloPodFile)

		err = oc.Run("create").Args("-f", helloPodFile).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("setting a new annotation")
		out, err := oc.Run("annotate").Args("pod", "hello-openshift", "new-anno=hello").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("pod/hello-openshift annotated"))

		g.By("validating new annotation")
		out, err = oc.Run("get").Args("pod", "hello-openshift", "--template", podAnnotationTemplate).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("hello"))

		g.By("removing the annotation")
		out, err = oc.Run("annotate").Args("pod", "hello-openshift", "new-anno-").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("pod/hello-openshift annotated"))

		g.By("validating missing annotation")
		out, err = oc.Run("get").Args("pod", "hello-openshift", "--template", podAnnotationTemplate).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("<no value>"))

		g.By("setting empty annotation")
		out, err = oc.Run("annotate").Args("pod", "hello-openshift", `new-anno=`).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("pod/hello-openshift annotated"))

		g.By("validating empty annotation")
		out, err = oc.Run("get").Args("pod", "hello-openshift", "--template", podAnnotationTemplate).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.BeEmpty())
	})
})
