package builds

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildv1 "github.com/openshift/api/build/v1"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

func hasConditionState(build *buildv1.Build, condition buildv1.BuildPhase, expectedStatus bool) bool {
	for _, c := range build.Status.Conditions {
		if c.Type == buildv1.BuildConditionType(condition) {
			if (expectedStatus && c.Status == corev1.ConditionTrue) ||
				(!expectedStatus && c.Status == corev1.ConditionFalse) {
				return true
			}
		}
	}
	return false
}

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] using build configuration runPolicy", func() {
	defer g.GinkgoRecover()
	var (
		// Use invalid source here as we don't care about the result
		oc = exutil.NewCLI("cli-build-run-policy")
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			// Create all fixtures
			oc.Run("create").Args("-f", exutil.FixturePath("testdata", "run_policy")).Execute()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("build configuration with Parallel build run policy", func() {
			g.It("runs the builds in parallel [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting multiple builds")
				var (
					startedBuilds []string
					counter       int
				)
				bcName := "sample-parallel-build"

				buildWatch, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{
					LabelSelector: BuildConfigSelector(bcName).String(),
				})
				defer buildWatch.Stop()

				// Start first build
				stdout, _, err := exutil.StartBuild(oc, bcName, "-o=name")
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(strings.TrimSpace(stdout)).ShouldNot(o.HaveLen(0))
				// extract build name from "build/buildName" resource id
				startedBuilds = append(startedBuilds, strings.TrimSpace(strings.Split(stdout, "/")[1]))

				// Wait for it to become running
				for {
					event := <-buildWatch.ResultChan()
					build := event.Object.(*buildv1.Build)
					o.Expect(IsBuildComplete(build)).Should(o.BeFalse())
					if build.Name == startedBuilds[0] && build.Status.Phase == buildv1.BuildPhaseRunning {
						break
					}
				}

				for i := 0; i < 2; i++ {
					stdout, _, err = exutil.StartBuild(oc, bcName, "-o=name")
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(strings.TrimSpace(stdout)).ShouldNot(o.HaveLen(0))
					startedBuilds = append(startedBuilds, strings.TrimSpace(strings.Split(stdout, "/")[1]))
				}

				o.Expect(err).NotTo(o.HaveOccurred())

				for {
					event := <-buildWatch.ResultChan()
					build := event.Object.(*buildv1.Build)
					if build.Name == startedBuilds[0] {
						if IsBuildComplete(build) {
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
					if build.Status.Phase == buildv1.BuildPhasePending {
						o.Expect(hasConditionState(build, buildv1.BuildPhasePending, true)).Should(o.BeTrue())
						firstBuildRunning := false
						_, err := BuildConfigBuilds(oc.BuildClient().BuildV1(), oc.Namespace(), bcName, func(b *buildv1.Build) bool {
							if b.Name == startedBuilds[0] && b.Status.Phase == buildv1.BuildPhaseRunning {
								o.Expect(hasConditionState(b, buildv1.BuildPhaseRunning, true)).Should(o.BeTrue())
								firstBuildRunning = true
							}
							return false
						})
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(firstBuildRunning).Should(o.BeTrue())
						counter++
					}
					// When the build failed or completed prematurely, fail the test
					o.Expect(IsBuildComplete(build)).Should(o.BeFalse())
					if counter == 2 {
						break
					}
				}
				o.Expect(counter).Should(o.BeEquivalentTo(2))
			})
		})

		g.Describe("build configuration with Serial build run policy", func() {
			g.It("runs the builds in serial order [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting multiple builds")
				var (
					startedBuilds []string
					counter       int
				)

				bcName := "sample-serial-build"
				buildVerified := map[string]bool{}

				for i := 0; i < 3; i++ {
					stdout, _, err := exutil.StartBuild(oc, bcName, "-o=name")
					o.Expect(err).NotTo(o.HaveOccurred())
					startedBuilds = append(startedBuilds, strings.TrimSpace(strings.Split(stdout, "/")[1]))
				}

				buildWatch, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{
					LabelSelector: BuildConfigSelector(bcName).String(),
				})
				defer buildWatch.Stop()
				o.Expect(err).NotTo(o.HaveOccurred())

				sawCompletion := false
				for {
					event := <-buildWatch.ResultChan()
					build := event.Object.(*buildv1.Build)
					var lastCompletion time.Time
					if build.Status.Phase == buildv1.BuildPhaseComplete {
						o.Expect(hasConditionState(build, buildv1.BuildPhaseComplete, true)).Should(o.BeTrue())
						lastCompletion = time.Now()
						o.Expect(build.Status.StartTimestamp).ToNot(o.BeNil(), "completed builds should have a valid start time")
						o.Expect(build.Status.CompletionTimestamp).ToNot(o.BeNil(), "completed builds should have a valid completion time")
						sawCompletion = true
					}
					if build.Status.Phase == buildv1.BuildPhaseRunning || build.Status.Phase == buildv1.BuildPhasePending {
						if build.Status.Phase == buildv1.BuildPhaseRunning {
							o.Expect(hasConditionState(build, buildv1.BuildPhaseRunning, true)).Should(o.BeTrue())
						}
						if build.Status.Phase == buildv1.BuildPhasePending {
							o.Expect(hasConditionState(build, buildv1.BuildPhasePending, true)).Should(o.BeTrue())
						}

						latency := lastCompletion.Sub(time.Now())
						o.Expect(latency).To(o.BeNumerically("<", 20*time.Second), "next build should have started less than 20s after last completed build")

						// Ignore events from complete builds (if there are any) if we already
						// verified the build.
						if _, exists := buildVerified[build.Name]; exists {
							continue
						}
						// Verify there are no other running or pending builds than this
						// build as serial build always runs alone.
						builds, err := BuildConfigBuilds(oc.BuildClient().BuildV1(), oc.Namespace(), bcName, func(b *buildv1.Build) bool {
							if b.Name == build.Name {
								return false
							}
							if b.Status.Phase == buildv1.BuildPhaseRunning || b.Status.Phase == buildv1.BuildPhasePending {
								return true
							}
							return false
						})
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(builds).Should(o.BeEmpty())

						// The builds should start in the same order as they were created.
						o.Expect(build.Name).Should(o.BeEquivalentTo(startedBuilds[counter]))

						buildVerified[build.Name] = true
						counter++
					}
					if counter == len(startedBuilds) {
						break
					}
				}
				o.Expect(sawCompletion).Should(o.BeTrue(), "should have seen at least one build complete")
			})
		})

		g.Describe("build configuration with Serial build run policy handling cancellation", func() {
			g.It("starts the next build immediately after one is canceled [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting multiple builds")
				bcName := "sample-serial-build"

				for i := 0; i < 3; i++ {
					_, _, err := exutil.StartBuild(oc, bcName)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				buildWatch, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{
					LabelSelector: BuildConfigSelector(bcName).String(),
				})
				defer buildWatch.Stop()
				o.Expect(err).NotTo(o.HaveOccurred())

				var cancelTime, cancelTime2 time.Time
				for {
					event := <-buildWatch.ResultChan()
					build := event.Object.(*buildv1.Build)
					if build.Status.Phase == buildv1.BuildPhasePending || build.Status.Phase == buildv1.BuildPhaseRunning {
						if build.Status.Phase == buildv1.BuildPhaseRunning {
							o.Expect(hasConditionState(build, buildv1.BuildPhaseRunning, true)).Should(o.BeTrue())
						}
						if build.Status.Phase == buildv1.BuildPhasePending {
							o.Expect(hasConditionState(build, buildv1.BuildPhasePending, true)).Should(o.BeTrue())
						}
						if build.Name == "sample-serial-build-1" {
							err := oc.Run("cancel-build").Args("sample-serial-build-1").Execute()
							o.Expect(err).ToNot(o.HaveOccurred())
							cancelTime = time.Now()
						}
						if build.Name == "sample-serial-build-2" {
							duration := time.Now().Sub(cancelTime)
							o.Expect(duration).To(o.BeNumerically("<", 20*time.Second), "next build should have started less than 20s after canceled build")
							err := oc.Run("cancel-build").Args("sample-serial-build-2").Execute()
							o.Expect(err).ToNot(o.HaveOccurred())
							cancelTime2 = time.Now()
						}
						if build.Name == "sample-serial-build-3" {
							duration := time.Now().Sub(cancelTime2)
							o.Expect(duration).To(o.BeNumerically("<", 20*time.Second), "next build should have started less than 20s after canceled build")
							break
						}
					}
				}
			})
		})

		g.Describe("build configuration with Serial build run policy handling failure", func() {
			g.It("starts the next build immediately after one fails [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting multiple builds")
				bcName := "sample-serial-build-fail"

				for i := 0; i < 3; i++ {
					_, _, err := exutil.StartBuild(oc, bcName)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				buildWatch, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{
					LabelSelector: BuildConfigSelector(bcName).String(),
				})
				defer buildWatch.Stop()
				o.Expect(err).NotTo(o.HaveOccurred())

				var failTime, failTime2 time.Time
				done, timestamps1, timestamps2, timestamps3 := false, false, false, false

				for done == false {
					select {
					case event := <-buildWatch.ResultChan():
						build := event.Object.(*buildv1.Build)
						if build.Status.Phase == buildv1.BuildPhasePending {
							o.Expect(hasConditionState(build, buildv1.BuildPhasePending, true)).Should(o.BeTrue())
							if build.Name == "sample-serial-build-fail-2" {
								duration := time.Now().Sub(failTime)
								o.Expect(duration).To(o.BeNumerically("<", 20*time.Second), "next build should have started less than 20s after failed build")
							}
							if build.Name == "sample-serial-build-fail-3" {
								duration := time.Now().Sub(failTime2)
								o.Expect(duration).To(o.BeNumerically("<", 20*time.Second), "next build should have started less than 20s after failed build")
							}
						}

						if build.Status.Phase == buildv1.BuildPhaseFailed {
							o.Expect(hasConditionState(build, buildv1.BuildPhaseFailed, true)).Should(o.BeTrue())
							if build.Name == "sample-serial-build-fail-1" {
								// this may not be set on the first build modified to failed event because
								// the build gets marked failed by the build pod, but the timestamps get
								// set by the buildpod controller.
								if build.Status.CompletionTimestamp != nil {
									o.Expect(build.Status.StartTimestamp).ToNot(o.BeNil(), "failed builds should have a valid start time")
									o.Expect(build.Status.CompletionTimestamp).ToNot(o.BeNil(), "failed builds should have a valid completion time")
									timestamps1 = true
								}
								failTime = time.Now()
							}
							if build.Name == "sample-serial-build-fail-2" {
								if build.Status.CompletionTimestamp != nil {
									o.Expect(build.Status.StartTimestamp).ToNot(o.BeNil(), "failed builds should have a valid start time")
									o.Expect(build.Status.CompletionTimestamp).ToNot(o.BeNil(), "failed builds should have a valid completion time")
									timestamps2 = true
								}
								failTime2 = time.Now()
							}
							if build.Name == "sample-serial-build-fail-3" {
								if build.Status.CompletionTimestamp != nil {
									o.Expect(build.Status.StartTimestamp).ToNot(o.BeNil(), "failed builds should have a valid start time")
									o.Expect(build.Status.CompletionTimestamp).ToNot(o.BeNil(), "failed builds should have a valid completion time")
									timestamps3 = true
								}
							}
						}
						// once we have all the expected timestamps, or we run out of time, we can bail.
						if timestamps1 && timestamps2 && timestamps3 {
							done = true
						}
					case <-time.After(2 * time.Minute):
						// we've waited as long as we dare, go see if we got all the timestamp data we expected.
						// if we have the timestamp data, we also know that we checked the next build start latency.
						done = true
					}
				}
				o.Expect(timestamps1).Should(o.BeTrue(), "failed builds should have start and completion timestamps set")
				o.Expect(timestamps2).Should(o.BeTrue(), "failed builds should have start and completion timestamps set")
				o.Expect(timestamps3).Should(o.BeTrue(), "failed builds should have start and completion timestamps set")
			})
		})

		g.Describe("build configuration with Serial build run policy handling deletion", func() {
			g.It("starts the next build immediately after running one is deleted [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting multiple builds")

				bcName := "sample-serial-build"
				for i := 0; i < 3; i++ {
					_, _, err := exutil.StartBuild(oc, bcName)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				buildWatch, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{
					LabelSelector: BuildConfigSelector(bcName).String(),
				})
				defer buildWatch.Stop()
				o.Expect(err).NotTo(o.HaveOccurred())

				var deleteTime, deleteTime2 time.Time
				done := false
				var timedOut error
				for !done {
					select {
					case event := <-buildWatch.ResultChan():
						build := event.Object.(*buildv1.Build)
						if build.Status.Phase == buildv1.BuildPhasePending || build.Status.Phase == buildv1.BuildPhaseRunning {
							if build.Status.Phase == buildv1.BuildPhaseRunning {
								o.Expect(hasConditionState(build, buildv1.BuildPhaseRunning, true)).Should(o.BeTrue())
							}
							if build.Status.Phase == buildv1.BuildPhasePending {
								o.Expect(hasConditionState(build, buildv1.BuildPhasePending, true)).Should(o.BeTrue())
							}
							if build.Name == "sample-serial-build-1" {
								err := oc.Run("delete").Args("build", "sample-serial-build-1", "--ignore-not-found").Execute()
								o.Expect(err).ToNot(o.HaveOccurred())
								deleteTime = time.Now()
							}
							if build.Name == "sample-serial-build-2" {
								duration := time.Now().Sub(deleteTime)
								o.Expect(duration).To(o.BeNumerically("<", 20*time.Second), "next build should have started less than 20s after deleted build")
								err := oc.Run("delete").Args("build", "sample-serial-build-2", "--ignore-not-found").Execute()
								o.Expect(err).ToNot(o.HaveOccurred())
								deleteTime2 = time.Now()
							}
							if build.Name == "sample-serial-build-3" {
								duration := time.Now().Sub(deleteTime2)
								o.Expect(duration).To(o.BeNumerically("<", 20*time.Second), "next build should have started less than 20s after deleted build")
								done = true
							}
						}
					case <-time.After(2 * time.Minute):
						// Give up waiting and finish after 2 minutes
						timedOut = fmt.Errorf("timed out waiting for pending build")
						done = true
					}
				}
				o.Expect(timedOut).NotTo(o.HaveOccurred())
			})
		})

		g.Describe("build configuration with SerialLatestOnly build run policy", func() {
			g.It("runs the builds in serial order but cancel previous builds [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				g.By("starting multiple builds")
				var (
					startedBuilds        []string
					expectedRunningBuild int
					wasCancelled         bool
				)

				bcName := "sample-serial-latest-only-build"
				buildVerified := map[string]bool{}
				buildWatch, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Watch(context.Background(), metav1.ListOptions{
					LabelSelector: BuildConfigSelector(bcName).String(),
				})
				defer buildWatch.Stop()
				o.Expect(err).NotTo(o.HaveOccurred())

				stdout, _, err := exutil.StartBuild(oc, bcName, "-o=name")
				o.Expect(err).NotTo(o.HaveOccurred())
				startedBuilds = append(startedBuilds, strings.TrimSpace(strings.Split(stdout, "/")[1]))

				// Wait for the first build to become running
				for {
					event := <-buildWatch.ResultChan()
					build := event.Object.(*buildv1.Build)
					if build.Name == startedBuilds[0] {
						if build.Status.Phase == buildv1.BuildPhaseRunning {
							buildVerified[build.Name] = true
							// now expect the last build to be run.
							expectedRunningBuild = 2
							break
						}
						o.Expect(IsBuildComplete(build)).Should(o.BeFalse())
					}
				}

				// Trigger two more builds
				for i := 0; i < 2; i++ {
					stdout, _, err = exutil.StartBuild(oc, bcName, "-o=name")
					o.Expect(err).NotTo(o.HaveOccurred())
					startedBuilds = append(startedBuilds, strings.TrimSpace(strings.Split(stdout, "/")[1]))
				}

				// Verify that the first build will complete and the next build to run
				// will be the last build created.
				for {
					event := <-buildWatch.ResultChan()
					build := event.Object.(*buildv1.Build)
					e2e.Logf("got event for build %s with phase %s", build.Name, build.Status.Phase)
					// The second build should be cancelled
					if build.Status.Phase == buildv1.BuildPhaseCancelled {
						o.Expect(hasConditionState(build, buildv1.BuildPhaseCancelled, true)).Should(o.BeTrue())
						if build.Name == startedBuilds[1] {
							buildVerified[build.Name] = true
							wasCancelled = true
						}
					}
					// Only first and third build should actually run (serially).
					if build.Status.Phase == buildv1.BuildPhaseRunning || build.Status.Phase == buildv1.BuildPhasePending {
						if build.Status.Phase == buildv1.BuildPhaseRunning {
							o.Expect(hasConditionState(build, buildv1.BuildPhaseRunning, true)).Should(o.BeTrue())
						}
						if build.Status.Phase == buildv1.BuildPhasePending {
							o.Expect(hasConditionState(build, buildv1.BuildPhasePending, true)).Should(o.BeTrue())
						}
						// Ignore events from complete builds (if there are any) if we already
						// verified the build.
						if _, exists := buildVerified[build.Name]; exists {
							continue
						}
						// Verify there are no other running or pending builds than this
						// build as serial build always runs alone.
						builds, err := BuildConfigBuilds(oc.BuildClient().BuildV1(), oc.Namespace(), bcName, func(b *buildv1.Build) bool {
							e2e.Logf("[%s] build %s is %s", build.Name, b.Name, b.Status.Phase)
							if b.Name == build.Name {
								return false
							}
							if b.Status.Phase == buildv1.BuildPhaseRunning || b.Status.Phase == buildv1.BuildPhasePending {
								return true
							}
							return false
						})
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(builds).Should(o.BeEmpty())

						// The builds should start in the same order as they were created.
						o.Expect(build.Name).Should(o.BeEquivalentTo(startedBuilds[expectedRunningBuild]))

						buildVerified[build.Name] = true
					}
					if len(buildVerified) == len(startedBuilds) {
						break
					}
				}

				o.Expect(wasCancelled).Should(o.BeEquivalentTo(true))
			})
		})
	})
})

// IsBuildComplete returns whether the provided build is complete or not
func IsBuildComplete(build *buildv1.Build) bool {
	return IsTerminalPhase(build.Status.Phase)
}

// IsTerminalPhase returns true if the provided phase is terminal
func IsTerminalPhase(phase buildv1.BuildPhase) bool {
	switch phase {
	case buildv1.BuildPhaseNew,
		buildv1.BuildPhasePending,
		buildv1.BuildPhaseRunning:
		return false
	}
	return true
}

// BuildConfigBuilds return a list of builds for the given build config.
// Optionally you can specify a filter function to select only builds that
// matches your criteria.
func BuildConfigBuilds(c buildclientv1.BuildsGetter, namespace, name string, filterFunc buildFilter) ([]*buildv1.Build, error) {
	result, err := c.Builds(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: BuildConfigSelector(name).String()})
	if err != nil {
		return nil, err
	}
	builds := make([]*buildv1.Build, len(result.Items))
	for i := range result.Items {
		builds[i] = &result.Items[i]
	}
	if filterFunc == nil {
		return builds, nil
	}
	var filteredList []*buildv1.Build
	for _, b := range builds {
		if filterFunc(b) {
			filteredList = append(filteredList, b)
		}
	}
	return filteredList, nil
}

type buildFilter func(*buildv1.Build) bool

// BuildConfigSelector returns a label Selector which can be used to find all
// builds for a BuildConfig.
func BuildConfigSelector(name string) labels.Selector {
	return labels.Set{buildv1.BuildConfigLabel: LabelValue(name)}.AsSelector()
}

// LabelValue returns a string to use as a value for the Build
// label in a pod. If the length of the string parameter exceeds
// the maximum label length, the value will be truncated.
func LabelValue(name string) string {
	if len(name) <= validation.DNS1123LabelMaxLength {
		return name
	}
	return name[:validation.DNS1123LabelMaxLength]
}
