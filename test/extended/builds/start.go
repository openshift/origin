package builds

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] starting a build using CLI", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture   = exutil.FixturePath("testdata", "test-build.json")
		exampleGemfile = exutil.FixturePath("testdata", "test-build-app", "Gemfile")
		exampleBuild   = exutil.FixturePath("testdata", "test-build-app")
		oc             = exutil.NewCLI("cli-start-build", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		oc.Run("create").Args("-f", buildFixture).Execute()
	})

	g.Describe("oc start-build --wait", func() {
		g.It("should start a build and wait for the build to complete", func() {
			g.By("starting the build with --wait flag")
			br, err := exutil.StartBuildAndWait(oc, "sample-build", "--wait")
			o.Expect(err).NotTo(o.HaveOccurred())
			br.AssertSuccess()
		})

		g.It("should start a build and wait for the build to fail", func() {
			g.By("starting the build with --wait flag but wrong --commit")
			br, _ := exutil.StartBuildAndWait(oc, "sample-build", "--wait", "--commit=fffffff")
			br.AssertFailure()
			o.Expect(br.StartBuildErr).To(o.HaveOccurred()) // start-build should detect the build error with --wait flag
			o.Expect(br.StartBuildStdErr).Should(o.ContainSubstring(`status is "Failed"`))
		})
	})

	g.Describe("override environment", func() {
		g.It("should accept environment variables", func() {
			g.By("starting the build with -e FOO=bar,VAR=test")
			br, err := exutil.StartBuildAndWait(oc, "sample-build", "-e", "FOO=bar,VAR=test")
			br.AssertSuccess()
			buildLog, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("verifying the build output contains the env vars"))
			o.Expect(buildLog).To(o.ContainSubstring("FOO=bar"))
			o.Expect(buildLog).To(o.ContainSubstring("VAR=test"))

			g.By(fmt.Sprintf("verifying the build output contains inherited env vars"))
			// This variable is not set and thus inherited from the original build config
			o.Expect(buildLog).To(o.ContainSubstring("BAR=test"))
		})

		g.It("BUILD_LOGLEVEL in buildconfig should create verbose output", func() {
			g.By("starting the build with buildconfig strategy env BUILD_LOGLEVEL=5")
			br, err := exutil.StartBuildAndWait(oc, "sample-verbose-build")
			br.AssertSuccess()
			buildLog, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("verifying the build output is verbose"))
			o.Expect(buildLog).To(o.ContainSubstring("Creating a new S2I builder"))
		})

		g.It("BUILD_LOGLEVEL in buildconfig can be overridden by build-loglevel", func() {
			g.By("starting the build with buildconfig strategy env BUILD_LOGLEVEL=5 but build-loglevel=1")
			br, err := exutil.StartBuildAndWait(oc, "sample-verbose-build", "--build-loglevel=1")
			br.AssertSuccess()
			buildLog, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("verifying the build output is not verbose"))
			o.Expect(buildLog).NotTo(o.ContainSubstring("Creating a new S2I builder"))
		})

	})

	g.Describe("binary builds", func() {
		var commit string

		g.It("should accept --from-file as input", func() {
			g.By("starting the build with a Dockerfile")
			br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--from-file=%s", exampleGemfile))
			br.AssertSuccess()
			buildLog, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("verifying the build %q status", br.BuildPath))

			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading file"))
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
			o.Expect(buildLog).To(o.ContainSubstring("Your bundle is complete"))

		})

		g.It("should accept --from-dir as input", func() {
			g.By("starting the build with a directory")
			br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--from-dir=%s", exampleBuild))
			br.AssertSuccess()
			buildLog, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("verifying the build %q status", br.BuildPath))
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading directory"))
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
			o.Expect(buildLog).To(o.ContainSubstring("Your bundle is complete"))
		})

		g.It("should accept --from-repo as input", func() {
			g.By("starting the build with a Git repository")
			br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--from-repo=%s", exampleBuild))
			br.AssertSuccess()
			buildLog, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading"))
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring(`at commit "HEAD"`))
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
			o.Expect(buildLog).To(o.ContainSubstring("Your bundle is complete"))
		})

		g.It("should accept --from-repo with --commit as input", func() {
			g.By("starting the build with a Git repository")
			gitCmd := exec.Command("git", "rev-parse", "HEAD~1")
			gitCmd.Dir = exampleBuild
			commitByteArray, err := gitCmd.CombinedOutput()
			commit = strings.TrimSpace(string(commitByteArray[:]))
			o.Expect(err).NotTo(o.HaveOccurred())
			br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--commit=%s", commit), fmt.Sprintf("--from-repo=%s", exampleBuild))
			br.AssertSuccess()
			buildLog, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())

			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading"))
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring(fmt.Sprintf("at commit \"%s\"", commit)))
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
			o.Expect(buildLog).To(o.ContainSubstring(fmt.Sprintf("\"commit\":\"%s\"", commit)))
			o.Expect(buildLog).To(o.ContainSubstring("Your bundle is complete"))
		})

		// run one valid binary build so we can do --from-build later
		g.It("should reject binary build requests without a --from-xxxx value", func() {
			g.By("starting a valid build with a directory")
			br, err := exutil.StartBuildAndWait(oc, "sample-build-binary", "--follow", fmt.Sprintf("--from-dir=%s", exampleBuild))
			br.AssertSuccess()
			buildLog, err := br.Logs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading directory"))
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
			o.Expect(buildLog).To(o.ContainSubstring("Your bundle is complete"))

			g.By("starting a build without a --from-xxxx value")
			br, err = exutil.StartBuildAndWait(oc, "sample-build-binary")
			o.Expect(br.StartBuildErr).To(o.HaveOccurred())
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("has no valid source inputs"))

			g.By("starting a build from an existing binary build")
			br, err = exutil.StartBuildAndWait(oc, "sample-build-binary", fmt.Sprintf("--from-build=%s", "sample-build-binary-1"))
			o.Expect(br.StartBuildErr).To(o.HaveOccurred())
			o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("has no valid source inputs"))
		})
	})

	g.Describe("cancelling build started by oc start-build --wait", func() {
		g.It("should start a build and wait for the build to cancel", func() {
			g.By("starting the build with --wait flag")
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer g.GinkgoRecover()
				defer wg.Done()
				_, stderr, err := exutil.StartBuild(oc, "sample-build", "--wait")
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(stderr).Should(o.ContainSubstring(`status is "Cancelled"`))
			}()

			g.By("getting the build name")
			var buildName string
			wait.Poll(time.Duration(100*time.Millisecond), 1*time.Minute, func() (bool, error) {
				out, err := oc.Run("get").
					Args("build", "--template", "{{ (index .items 0).metadata.name }}").Output()
				// Give it second chance in case the build resource was not created yet
				if err != nil || len(out) == 0 {
					return false, nil
				}

				buildName = out
				return true, nil
			})

			o.Expect(buildName).ToNot(o.BeEmpty())

			g.By(fmt.Sprintf("cancelling the build %q", buildName))
			err := oc.Run("cancel-build").Args(buildName).Execute()
			o.Expect(err).ToNot(o.HaveOccurred())
			wg.Wait()
		})

	})

})
