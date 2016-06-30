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
			bnA1, err := oc.Run("start-build").Args(bcA).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting long name build config B-1")
			bnB1, err := oc.Run("start-build").Args(bcB).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting long name build config A-2")
			bnA2, err := oc.Run("start-build").Args(bcA).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting long name build config B-2")
			bnB2, err := oc.Run("start-build").Args(bcB).Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			builds := [...]string{bnA1, bnB1, bnA2, bnB2}

			g.By("checking the status of the builds")
			for _, bn := range builds {
				err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), bn, exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
				if err != nil {
					exutil.DumpNamedBuildLogs(bn, oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("Verifying gets for build configs and builds")
			for _, bn := range builds {
				err = oc.Run("get").Args("build", bn).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("Deleting build config B (which should cascade to builds it created)")
			err = oc.Run("delete").Args("bc", bcB).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("get").Args("bc", bcB).Execute()
			o.Expect(err).To(o.HaveOccurred())

			err = oc.Run("get").Args("build", bnB1).Execute()
			o.Expect(err).To(o.HaveOccurred())

			err = oc.Run("get").Args("build", bnB2).Execute()
			o.Expect(err).To(o.HaveOccurred())

			g.By("Verifying build config A was untouched by delete")
			err = oc.Run("get").Args("bc", bcA).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("get").Args("build", bnA1).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("get").Args("build", bnA2).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		})

	})
})
