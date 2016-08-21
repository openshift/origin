package builds

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[builds][Slow] s2i extended build", func() {
	defer g.GinkgoRecover()

	var (
		oc                    = exutil.NewCLI("extended-build", exutil.KubeConfigPath())
		testDataDir           = exutil.FixturePath("testdata", "build-extended")
		runnerConf            = filepath.Join(testDataDir, "jvm-runner.yaml")
		runnerWithScriptsConf = filepath.Join(testDataDir, "jvm-runner-with-scripts.yaml")
		scriptsFromRepoBc     = filepath.Join(testDataDir, "bc-scripts-in-repo.yaml")
		scriptsFromUrlBc      = filepath.Join(testDataDir, "bc-scripts-by-url.yaml")
		scriptsFromImageBc    = filepath.Join(testDataDir, "bc-scripts-in-the-image.yaml")
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())

		// we have to wait until image stream tag will be available, otherwise
		// `oc start-build` will fail with 'imagestreamtags "wildfly:10.0" not found' error.
		// See this issue for details: https://github.com/openshift/origin/issues/10103
		err = exutil.WaitForAnImageStreamTag(oc, "openshift", "wildfly", "10.0")
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("with scripts from the source repository", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.It("should use assemble-runtime script from the source repository", func() {

			g.By("creating jvm-runner configuration")
			err := exutil.CreateResource(runnerConf, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("building jvm-runner image")
			br, _ := exutil.StartBuildAndWait(oc, "jvm-runner")
			br.AssertSuccess()

			g.By("creating build config")
			err = exutil.CreateResource(scriptsFromRepoBc, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("running the build")
			br, _ = exutil.StartBuildAndWait(oc, "java-extended-build-from-repo", "--build-loglevel=5")
			br.AssertSuccess()
			buildLog, err := br.Logs()
			if err != nil {
				e2e.Failf("Failed to fetch build logs: %v", err)
			}

			g.By("expecting that .s2i/bin/assemble-runtime was executed")
			o.Expect(buildLog).To(o.ContainSubstring(`Using "assemble-runtime" installed from "<source-dir>/.s2i/bin/assemble-runtime"`))
			o.Expect(buildLog).To(o.ContainSubstring(".s2i/bin/assemble-runtime: assembling app within runtime image"))

			g.By("expecting that environment variable from BuildConfig is available")
			o.Expect(buildLog).To(o.ContainSubstring(".s2i/bin/assemble-runtime: USING_ENV_FROM_BUILD_CONFIG=yes"))

			g.By("expecting that environment variable from .s2i/environment is available")
			o.Expect(buildLog).To(o.ContainSubstring(".s2i/bin/assemble-runtime: USING_ENV_FROM_FILE=yes"))
		})
	})

	g.Describe("with scripts from URL", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.It("should use assemble-runtime script from URL", func() {

			g.By("creating jvm-runner configuration")
			err := exutil.CreateResource(runnerConf, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("building jvm-runner image")
			br, _ := exutil.StartBuildAndWait(oc, "jvm-runner")
			br.AssertSuccess()

			g.By("creating build config")
			err = exutil.CreateResource(scriptsFromUrlBc, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("running the build")
			br, _ = exutil.StartBuildAndWait(oc, "java-extended-build-from-url", "--build-loglevel=5")
			br.AssertSuccess()
			buildLog, err := br.Logs()
			if err != nil {
				e2e.Failf("Failed to fetch build logs: %v", err)
			}

			g.By("expecting that .s2i/bin/assemble-runtime was executed")
			o.Expect(buildLog).To(o.ContainSubstring(`Using "assemble-runtime" installed from "https://raw.githubusercontent.com/php-coder/java-maven-hello-world/s2i-assemble-and-assemble-runtime/.s2i/bin/assemble-runtime"`))
			o.Expect(buildLog).To(o.ContainSubstring(".s2i/bin/assemble-runtime: assembling app within runtime image"))

			g.By("expecting that environment variable from BuildConfig is available")
			o.Expect(buildLog).To(o.ContainSubstring(".s2i/bin/assemble-runtime: USING_ENV_FROM_BUILD_CONFIG=yes"))

			g.By("expecting that environment variable from .s2i/environment isn't available")
			o.Expect(buildLog).To(o.ContainSubstring(".s2i/bin/assemble-runtime: USING_ENV_FROM_FILE=no"))
		})
	})

	g.Describe("with scripts from runtime image", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.It("should use assemble-runtime script from that image", func() {

			g.By("creating jvm-runner-with-scripts configuration")
			err := exutil.CreateResource(runnerWithScriptsConf, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("building jvm-runner-with-scripts image")
			br, _ := exutil.StartBuildAndWait(oc, "jvm-runner-with-scripts")
			br.AssertSuccess()

			g.By("creating build config")
			err = exutil.CreateResource(scriptsFromImageBc, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("running the build")
			br, _ = exutil.StartBuildAndWait(oc, "java-extended-build-from-image", "--build-loglevel=5")
			br.AssertSuccess()
			buildLog, err := br.Logs()
			if err != nil {
				e2e.Failf("Failed to fetch build logs: %v", err)
			}

			g.By("expecting that .s2i/bin/assemble-runtime was executed")
			o.Expect(buildLog).To(o.ContainSubstring(`Using "assemble-runtime" installed from "image:///usr/libexec/s2i/assemble-runtime"`))
			o.Expect(buildLog).To(o.ContainSubstring(".s2i/bin/assemble-runtime: assembling app within runtime image"))

			g.By("expecting that environment variable from BuildConfig is available")
			o.Expect(buildLog).To(o.ContainSubstring(".s2i/bin/assemble-runtime: USING_ENV_FROM_BUILD_CONFIG=yes"))

			g.By("expecting that environment variable from .s2i/environment isn't available")
			o.Expect(buildLog).To(o.ContainSubstring(".s2i/bin/assemble-runtime: USING_ENV_FROM_FILE=no"))
		})
	})

})
