package builds

import (
	"fmt"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/kubernetes/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[builds][Slow] s2i extended build", func() {
	defer g.GinkgoRecover()

	var (
		oc                 = exutil.NewCLI("extended-build", exutil.KubeConfigPath())
		testDataDir        = exutil.FixturePath("testdata", "build-extended")
		scriptsFromRepoBc  = filepath.Join(testDataDir, "bc-scripts-in-repo.json")
		scriptsFromUrlBc   = filepath.Join(testDataDir, "bc-scripts-by-url.json")
		scriptsFromImageBc = filepath.Join(testDataDir, "bc-scripts-in-the-image.json")
	)

	g.Describe("with scripts from the source repository", func() {
		oc.SetOutputDir(exutil.TestContext.OutputDir)

		g.It("should use assemble-runtime script from the source repository", func() {
			const buildConfigName = "java-extended-build-from-repo"
			const buildName = buildConfigName + "-1"

			g.By("creating build config")
			err := oc.Run("create").Args("-f", scriptsFromRepoBc).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// we have to wait until image stream tag will be available, otherwise
			// `oc start-build` will fail with 'imagestreamtags "wildfly:10.0" not found' error.
			// See this issue for details: https://github.com/openshift/origin/issues/10103
			g.By("waiting when image stream tag for wildfly will be available")
			wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
				out, err := oc.Run("get").Args("imagestreamtags", "--namespace", "openshift", "--output", `jsonpath='{.items[?(@.metadata.name=="wildfly:10.0")].image.dockerImageReference}'`).Output()
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "Could not get image stream tag for wildfly: %v\n", err)
					return false, err
				}
				if len(out) > 0 && out != "''" {
					fmt.Fprintf(g.GinkgoWriter, "Using image stream tag for wildfly: %s\n", out)
					return true, nil
				}

				return false, nil
			})

			g.By("running the build")
			out, err := oc.Run("start-build").Args("--build-loglevel=5", buildConfigName).Output()
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "\nstart-build output:\n%s\n", out)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the build to complete")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			if err != nil {
				exutil.DumpBuildLogs(buildConfigName, oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			buildLog, err := oc.Run("logs").Args("--follow", "build/"+buildName).Output()
			if err != nil {
				e2e.Failf("Failed to fetch build logs of %q: %v", buildLog, err)
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
			const buildConfigName = "java-extended-build-from-url"
			const buildName = buildConfigName + "-1"

			g.By("creating build config")
			err := oc.Run("create").Args("-f", scriptsFromUrlBc).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("running the build")
			out, err := oc.Run("start-build").Args("--build-loglevel=5", buildConfigName).Output()
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "\nstart-build output:\n%s\n", out)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the build to complete")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			if err != nil {
				exutil.DumpBuildLogs(buildConfigName, oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			buildLog, err := oc.Run("logs").Args("--follow", "build/"+buildName).Output()
			if err != nil {
				e2e.Failf("Failed to fetch build logs of %q: %v", buildLog, err)
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
			const buildConfigName = "java-extended-build-from-image"
			const buildName = buildConfigName + "-1"

			g.By("creating build config")
			err := oc.Run("create").Args("-f", scriptsFromImageBc).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("running the build")
			out, err := oc.Run("start-build").Args("--build-loglevel=5", buildConfigName).Output()
			if err != nil {
				fmt.Fprintf(g.GinkgoWriter, "\nstart-build output:\n%s\n", out)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the build to complete")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), buildName, exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			if err != nil {
				exutil.DumpBuildLogs(buildConfigName, oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			buildLog, err := oc.Run("logs").Args("--follow", "build/"+buildName).Output()
			if err != nil {
				e2e.Failf("Failed to fetch build logs of %q: %v", buildLog, err)
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
