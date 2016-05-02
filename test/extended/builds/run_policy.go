package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildutil "github.com/openshift/origin/pkg/build/util"
	exutil "github.com/openshift/origin/test/extended/util"
	kapi "k8s.io/kubernetes/pkg/api"
)

var _ = g.Describe("[builds][Slow] using build configuration runPolicy", func() {
	defer g.GinkgoRecover()
	var (
		// Use invalid source here as we don't care about the result
		oc = exutil.NewCLI("cli-build-run-policy", exutil.KubeConfigPath())
	)

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
		// Create all fixtures
		oc.Run("create").Args("-f", exutil.FixturePath("..", "extended", "fixtures", "run_policy")).Execute()
	})

	g.Describe("build configuration with Parallel build run policy", func() {
		g.It("runs the builds in parallel", func() {
			g.By("starting multiple builds")
			var (
				startedBuilds []string
				counter       int
			)
			bcName := "sample-parallel-build"

			buildWatch, err := oc.REST().Builds(oc.Namespace()).Watch(kapi.ListOptions{
				LabelSelector: buildutil.BuildConfigSelector(bcName),
			})
			defer buildWatch.Stop()

			// Start first build
			out, err := oc.Run("start-build").Args(bcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(out)).ShouldNot(o.HaveLen(0))
			startedBuilds = append(startedBuilds, strings.TrimSpace(out))

			// Wait for it to become running
			for {
				event := <-buildWatch.ResultChan()
				build := event.Object.(*buildapi.Build)
				o.Expect(buildutil.IsBuildComplete(build)).Should(o.BeFalse())
				if build.Name == startedBuilds[0] && build.Status.Phase == buildapi.BuildPhaseRunning {
					break
				}
			}

			for i := 0; i < 2; i++ {
				out, err := oc.Run("start-build").Args(bcName).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(strings.TrimSpace(out)).ShouldNot(o.HaveLen(0))
				startedBuilds = append(startedBuilds, strings.TrimSpace(out))
			}

			o.Expect(err).NotTo(o.HaveOccurred())

			for {
				event := <-buildWatch.ResultChan()
				build := event.Object.(*buildapi.Build)
				if build.Name == startedBuilds[0] {
					if buildutil.IsBuildComplete(build) {
						break
					}
					continue
				}
				// When the the other two builds we started after waiting for the first
				// build to become running are Pending, verify the first build is still
				// running (so the other two builds are started in parallel with first
				// build).
				// TODO: This might introduce flakes in case the first build complete
				// sooner or fail.
				if build.Status.Phase == buildapi.BuildPhasePending {
					c := buildclient.NewOSClientBuildClient(oc.REST())
					firstBuildRunning := false
					_, err := buildutil.BuildConfigBuilds(c, oc.Namespace(), bcName, func(b buildapi.Build) bool {
						if b.Name == startedBuilds[0] && b.Status.Phase == buildapi.BuildPhaseRunning {
							firstBuildRunning = true
						}
						return false
					})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(firstBuildRunning).Should(o.BeTrue())
					counter++
				}
				// When the build failed or completed prematurely, fail the test
				o.Expect(buildutil.IsBuildComplete(build)).Should(o.BeFalse())
				if counter == 2 {
					break
				}
			}
			o.Expect(counter).Should(o.BeEquivalentTo(2))
		})
	})

	g.Describe("build configuration with Serial build run policy", func() {
		g.It("runs the builds in serial order", func() {
			g.By("starting multiple builds")
			var (
				startedBuilds []string
				counter       int
			)

			bcName := "sample-serial-build"
			buildVerified := map[string]bool{}

			for i := 0; i < 3; i++ {
				out, err := oc.Run("start-build").Args(bcName).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				startedBuilds = append(startedBuilds, strings.TrimSpace(out))
			}

			buildWatch, err := oc.REST().Builds(oc.Namespace()).Watch(kapi.ListOptions{
				LabelSelector: buildutil.BuildConfigSelector(bcName),
			})
			defer buildWatch.Stop()
			o.Expect(err).NotTo(o.HaveOccurred())

			for {
				event := <-buildWatch.ResultChan()
				build := event.Object.(*buildapi.Build)
				if build.Status.Phase == buildapi.BuildPhaseRunning {
					// Ignore events from complete builds (if there are any) if we already
					// verified the build.
					if _, exists := buildVerified[build.Name]; exists {
						continue
					}
					// Verify there are no other running or pending builds than this
					// build as serial build always runs alone.
					c := buildclient.NewOSClientBuildClient(oc.REST())
					builds, err := buildutil.BuildConfigBuilds(c, oc.Namespace(), bcName, func(b buildapi.Build) bool {
						if b.Name == build.Name {
							return false
						}
						if b.Status.Phase == buildapi.BuildPhaseRunning || b.Status.Phase == buildapi.BuildPhasePending {
							return true
						}
						return false
					})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).Should(o.BeEmpty())

					// The builds should start in the same order as they were created.
					o.Expect(build.Name).Should(o.BeEquivalentTo(startedBuilds[counter]))

					buildVerified[build.Name] = true
					counter++
				}
				if counter == len(startedBuilds) {
					break
				}
			}
		})
	})

	g.Describe("build configuration with SerialLatestOnly build run policy", func() {
		g.It("runs the builds in serial order but cancel previous builds", func() {
			g.By("starting multiple builds")
			var (
				startedBuilds []string
				counter       int
				wasCancelled  bool
			)

			bcName := "sample-serial-latest-only-build"
			buildVerified := map[string]bool{}
			buildWatch, err := oc.REST().Builds(oc.Namespace()).Watch(kapi.ListOptions{
				LabelSelector: buildutil.BuildConfigSelector(bcName),
			})
			defer buildWatch.Stop()
			o.Expect(err).NotTo(o.HaveOccurred())

			out, err := oc.Run("start-build").Args(bcName).Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			startedBuilds = append(startedBuilds, strings.TrimSpace(out))
			// Wait for the first build to become running
			for {
				event := <-buildWatch.ResultChan()
				build := event.Object.(*buildapi.Build)
				if build.Name == startedBuilds[0] {
					if build.Status.Phase == buildapi.BuildPhaseRunning {
						break
					}
					o.Expect(buildutil.IsBuildComplete(build)).Should(o.BeFalse())
				}
			}
			// Trigger another two builds
			for i := 0; i < 2; i++ {
				out, err := oc.Run("start-build").Args(bcName).Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				startedBuilds = append(startedBuilds, strings.TrimSpace(out))
			}
			// Verify that the first build will complete and the next build to run
			// will be the last build created.
			for {
				event := <-buildWatch.ResultChan()
				build := event.Object.(*buildapi.Build)
				// The second build should be cancelled
				if build.Status.Phase == buildapi.BuildPhaseCancelled {
					if build.Name == startedBuilds[1] {
						buildVerified[build.Name] = true
						wasCancelled = true
						counter++
					}
				}
				// Only first and third build should actually run (serially).
				if build.Status.Phase == buildapi.BuildPhaseRunning {
					// Ignore events from complete builds (if there are any) if we already
					// verified the build.
					if _, exists := buildVerified[build.Name]; exists {
						continue
					}
					// Verify there are no other running or pending builds than this
					// build as serial build always runs alone.
					c := buildclient.NewOSClientBuildClient(oc.REST())
					builds, err := buildutil.BuildConfigBuilds(c, oc.Namespace(), bcName, func(b buildapi.Build) bool {
						fmt.Printf("[%s] build %s is %s", build.Name, b.Name, b.Status.Phase)
						if b.Name == build.Name {
							return false
						}
						if b.Status.Phase == buildapi.BuildPhaseRunning || b.Status.Phase == buildapi.BuildPhasePending {
							return true
						}
						return false
					})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).Should(o.BeEmpty())

					// The builds should start in the same order as they were created.
					o.Expect(build.Name).Should(o.BeEquivalentTo(startedBuilds[counter]))

					buildVerified[build.Name] = true
					counter++
				}
				if len(buildVerified) == len(startedBuilds) {
					break
				}
			}

			o.Expect(wasCancelled).Should(o.BeEquivalentTo(true))
		})
	})

})
