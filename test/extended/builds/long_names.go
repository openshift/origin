package builds

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] extremely long build/bc names are not problematic", func() {
	defer g.GinkgoRecover()
	var (
		testDataBaseDir  = exutil.FixturePath("testdata", "long_names")
		longNamesFixture = filepath.Join(testDataBaseDir, "fixture.json")
		oc               = exutil.NewCLI("long-names", exutil.KubeConfigPath())
	)

	g.Describe("build with long names", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.It("delete builds with long names without collateral damage", func() {
			g.By("creating long_names fixtures")
			err := oc.Run("create").Args("-f", longNamesFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Names can be a maximum of 253 chars. These build config names are 201 (to allow for suffixes appiled during the test process, e.g. -1, -2 for builds and log filenames)
			// and the names differ only in the last character.
			bcA := "12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890a"
			bcB := "12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890b"

			g.By("starting long name build config A-1")
			brA1, err := exutil.StartBuildAndWait(oc, bcA)
			brA1.AssertSuccess()

			g.By("starting long name build config B-1")
			brB1, err := exutil.StartBuildAndWait(oc, bcB)
			brB1.AssertSuccess()

			g.By("starting long name build config A-2")
			brA2, err := exutil.StartBuildAndWait(oc, bcA)
			brA2.AssertSuccess()

			g.By("starting long name build config B-2")
			brB2, err := exutil.StartBuildAndWait(oc, bcB)
			brB2.AssertSuccess()

			builds := [...]*exutil.BuildResult{brA1, brB1, brA2, brB2}

			g.By("Verifying gets for build configs and builds")
			for _, br := range builds {
				err = oc.Run("get").Args(br.BuildPath).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("Deleting build config B (which should cascade to builds it created)")
			err = oc.Run("delete").Args("bc", bcB).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("get").Args("bc", bcB).Execute()
			o.Expect(err).To(o.HaveOccurred())

			err = oc.Run("get").Args(brB1.BuildPath).Execute()
			o.Expect(err).To(o.HaveOccurred())

			err = oc.Run("get").Args(brB2.BuildPath).Execute()
			o.Expect(err).To(o.HaveOccurred())

			g.By("Verifying build config A was untouched by delete")
			err = oc.Run("get").Args("bc", bcA).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("get").Args(brA1.BuildPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("get").Args(brA2.BuildPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})

	})
})
