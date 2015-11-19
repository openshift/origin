package builds

import (
	"fmt"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/util/wait"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("builds: parallel: oc start-build", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture   = exutil.FixturePath("..", "extended", "fixtures", "test-build.json")
		exampleGemfile = exutil.FixturePath("..", "extended", "fixtures", "test-build-app", "Gemfile")
		exampleBuild   = exutil.FixturePath("..", "extended", "fixtures", "test-build-app")
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
			out, err := oc.Run("start-build").Args("sample-build", "--wait").Output()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("verifying the build %q status", out))
			build, err := oc.REST().Builds(oc.Namespace()).Get(out)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseComplete))
		})

		g.It("should start a build and wait for the build to fail", func() {
			g.By("starting the build with --wait flag but wrong --commit")
			out, err := oc.Run("start-build").
				Args("sample-build", "--wait", "--commit", "fffffff").
				Output()
			o.Expect(err).To(o.HaveOccurred())
			o.Expect(out).Should(o.ContainSubstring(`status is "Failed"`))
		})
	})

	g.Describe("binary builds", func() {
		g.It("should accept --from-file as input", func() {
			g.By("starting the build with a Dockerfile")
			out, err := oc.Run("start-build").Args("sample-build", "--follow", "--wait", fmt.Sprintf("--from-file=%s", exampleGemfile)).Output()
			g.By(fmt.Sprintf("verifying the build %q status", out))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Uploading file"))
			o.Expect(out).To(o.ContainSubstring("as binary input for the build ..."))
			o.Expect(out).To(o.ContainSubstring("Your bundle is complete"))

			build, err := oc.REST().Builds(oc.Namespace()).Get("sample-build-1")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseComplete))
		})

		g.It("should accept --from-dir as input", func() {
			g.By("starting the build with a directory")
			out, err := oc.Run("start-build").Args("sample-build", "--follow", "--wait", fmt.Sprintf("--from-dir=%s", exampleBuild)).Output()
			g.By(fmt.Sprintf("verifying the build %q status", out))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Uploading directory"))
			o.Expect(out).To(o.ContainSubstring("as binary input for the build ..."))
			o.Expect(out).To(o.ContainSubstring("Your bundle is complete"))

			build, err := oc.REST().Builds(oc.Namespace()).Get("sample-build-1")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseComplete))
		})

		g.It("should accept --from-repo as input", func() {
			g.By("starting the build with a Git repository")
			out, err := oc.Run("start-build").Args("sample-build", "--follow", "--wait", fmt.Sprintf("--from-repo=%s", exampleBuild)).Output()
			g.By(fmt.Sprintf("verifying the build %q status", out))
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(out).To(o.ContainSubstring("Uploading Git repository"))
			o.Expect(out).To(o.ContainSubstring("as binary input for the build ..."))
			o.Expect(out).To(o.ContainSubstring("Your bundle is complete"))

			build, err := oc.REST().Builds(oc.Namespace()).Get("sample-build-1")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(build.Status.Phase).Should(o.BeEquivalentTo(buildapi.BuildPhaseComplete))
		})
	})

	g.Describe("cancelling build started by oc start-build --wait", func() {
		g.It("should start a build and wait for the build to cancel", func() {
			g.By("starting the build with --wait flag")
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer g.GinkgoRecover()
				out, err := oc.Run("start-build").Args("sample-build", "--wait").Output()
				defer wg.Done()
				o.Expect(err).To(o.HaveOccurred())
				o.Expect(out).Should(o.ContainSubstring(`status is "Cancelled"`))
			}()

			g.By("getting the build name")
			var buildName string
			wait.Poll(time.Duration(100*time.Millisecond), time.Duration(60*time.Second), func() (bool, error) {
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
