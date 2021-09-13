package cli

import (
	"os"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc service", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("oc-service")

	g.It("creates and deletes services", func() {
		err := oc.Run("get").Args("services").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		file, err := writeObjectToFile(newFrontendService())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(file)

		err = oc.Run("create").Args("-f", file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("delete").Args("service", "frontend").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
