package cli

import (
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc secret", func() {
	defer g.GinkgoRecover()

	var (
		oc       = exutil.NewCLI("oc-secret")
		testData = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata", "resource-builder")
	)

	g.It("creates and retrieves expected", g.Label("Size:S"), func() {
		g.By("creating secrets from a directory of files with proper extensions, as well as explicit filenames without extensions")
		err := oc.Run("create").Args(
			"-f", filepath.Join(testData, "directory"),
			"-f", filepath.Join(testData, "json-no-extension"),
			"-f", filepath.Join(testData, "yml-no-extension"),
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("getting secrets without extensions")
		err = oc.Run("get").Args("secret", "json-no-extension", "yml-no-extension").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("getting secrets from files in directory with proper extensions")
		err = oc.Run("get").Args("secret", "json-with-extension", "yml-with-extension").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("getting a secret that shouldn't exist because it was in the directory without an extension")
		out, err := oc.Run("get").Args("secret", "json-no-extension-in-directory").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("not found"))
	})
})
