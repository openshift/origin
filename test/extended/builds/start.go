package builds

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] starting a build using CLI", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture      = exutil.FixturePath("testdata", "builds", "test-build.yaml")
		bcWithPRRef       = exutil.FixturePath("testdata", "builds", "test-bc-with-pr-ref.yaml")
		exampleGemfile    = exutil.FixturePath("testdata", "builds", "test-build-app", "Gemfile")
		exampleBuild      = exutil.FixturePath("testdata", "builds", "test-build-app")
		exampleGemfileURL = "https://raw.githubusercontent.com/openshift/ruby-hello-world/master/Gemfile"
		exampleArchiveURL = "https://github.com/openshift/ruby-hello-world/archive/master.zip"
		oc                = exutil.NewCLI("cli-start-build", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.Run("create").Args("-f", buildFixture).Execute()
		})

		g.Context("start-build test context", func() {
			g.AfterEach(func() {
				if g.CurrentGinkgoTestDescription().Failed {
					exutil.DumpPodStates(oc)
					exutil.DumpPodLogsStartingWith("", oc)
				}
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

			g.Describe("oc start-build with pr ref", func() {
				g.It("should start a build from a PR ref, wait for the build to complete, and confirm the right level was used", func() {
					g.By("make sure wildly imagestream has latest tag")
					err := exutil.WaitForAnImageStreamTag(oc.AsAdmin(), "openshift", "wildfly", "latest")
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("create build config")
					err = oc.Run("create").Args("-f", bcWithPRRef).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("start the builds, wait for and confirm successful completion")
					br, err := exutil.StartBuildAndWait(oc, "bc-with-pr-ref-docker")
					o.Expect(err).NotTo(o.HaveOccurred())
					br.AssertSuccess()
					br, err = exutil.StartBuildAndWait(oc, "bc-with-pr-ref")
					o.Expect(err).NotTo(o.HaveOccurred())
					br.AssertSuccess()
					out, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())

					// the repo at the PR level noted in bcWithPRRef had a pom.xml level of "0.1-SNAPSHOT" (we are well past that now)
					// so simply looking for that string in the mvn output is indicative of being at that level
					g.By("confirm the correct commit level was retrieved")
					o.Expect(out).Should(o.ContainSubstring("0.1-SNAPSHOT"))

					istag, err := oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("bc-with-pr-ref:latest", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(istag.Image.DockerImageMetadata.Config.Labels).To(o.HaveKeyWithValue("io.openshift.build.commit.ref", "refs/pull/1/head"))
					o.Expect(istag.Image.DockerImageMetadata.Config.Env).To(o.ContainElement("OPENSHIFT_BUILD_REFERENCE=refs/pull/1/head"))

					istag, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("bc-with-pr-ref-docker:latest", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(istag.Image.DockerImageMetadata.Config.Labels).To(o.HaveKeyWithValue("io.openshift.build.commit.ref", "refs/pull/1/head"))
					o.Expect(istag.Image.DockerImageMetadata.Config.Env).To(o.ContainElement("OPENSHIFT_BUILD_REFERENCE=refs/pull/1/head"))
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

				// do a best effort to initialize the repo in case it is a raw checkout or temp dir
				tryRepoInit := func(exampleBuild string) {
					out, err := exec.Command("bash", "-c", fmt.Sprintf("cd %q; if ! git rev-parse --git-dir; then git init .; git add .; git commit -m 'first'; touch foo; git add .; git commit -m 'second'; fi; true", exampleBuild)).CombinedOutput()
					fmt.Fprintf(g.GinkgoWriter, "Tried to init git repo: %v\n%s\n", err, string(out))
				}

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
					tryRepoInit(exampleBuild)
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
					tryRepoInit(exampleBuild)
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

				g.It("shoud accept --from-file with https URL as an input", func() {
					g.By("starting a valid build with input file served by https")
					br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--from-file=%s", exampleGemfileURL))
					br.AssertSuccess()
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring(fmt.Sprintf("Uploading file from %q as binary input for the build", exampleGemfileURL)))
					o.Expect(buildLog).To(o.ContainSubstring("Your bundle is complete"))
				})

				g.It("shoud accept --from-archive with https URL as an input", func() {
					g.By("starting a valid build with input archive served by https")
					// can't use sample-build-binary because we need contextDir due to github archives containing the top-level directory
					br, err := exutil.StartBuildAndWait(oc, "sample-build-github-archive", fmt.Sprintf("--from-archive=%s", exampleArchiveURL))
					br.AssertSuccess()
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring(fmt.Sprintf("Uploading archive from %q as binary input for the build", exampleArchiveURL)))
					o.Expect(buildLog).To(o.ContainSubstring("Your bundle is complete"))
				})
			})

			g.Describe("cancel a binary build that doesn't start running in 5 minutes", func() {
				g.It("should start a build and wait for the build to be cancelled", func() {
					g.By("starting a build with a nodeselector that can't be matched")
					go func() {
						exutil.StartBuild(oc, "sample-build-binary-invalidnodeselector", fmt.Sprintf("--from-file=%s", exampleGemfile))
					}()
					build := &buildapi.Build{}
					cancelFn := func(b *buildapi.Build) bool {
						build = b
						return exutil.CheckBuildCancelledFn(b)
					}
					err := exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), "sample-build-binary-invalidnodeselector-1", nil, nil, cancelFn)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(build.Status.Phase).To(o.Equal(buildapi.BuildPhaseCancelled))
					exutil.CheckForBuildEvent(oc.KubeClient().Core(), build, buildapi.BuildCancelledEventReason, buildapi.BuildCancelledEventMessage)
				})
			})

			g.Describe("cancel a build started by oc start-build --wait", func() {
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
					build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(buildName, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(build).NotTo(o.BeNil(), "build object should exist")

					g.By(fmt.Sprintf("cancelling the build %q", buildName))
					err = oc.Run("cancel-build").Args(buildName).Execute()
					o.Expect(err).ToNot(o.HaveOccurred())
					wg.Wait()
					exutil.CheckForBuildEvent(oc.KubeClient().Core(), build, buildapi.BuildCancelledEventReason, buildapi.BuildCancelledEventMessage)

				})

			})

			g.Describe("Setting build-args on Docker builds", func() {
				g.It("Should copy build args from BuildConfig to Build", func() {
					g.By("starting the build without --build-arg flag")
					br, _ := exutil.StartBuildAndWait(oc, "sample-build-docker-args-preset")
					br.AssertSuccess()
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By("verifying the build output contains the build args from the BuildConfig.")
					o.Expect(buildLog).To(o.ContainSubstring("default"))
				})
				g.It("Should accept build args that are specified in the Dockerfile", func() {
					g.By("starting the build with --build-arg flag")
					br, _ := exutil.StartBuildAndWait(oc, "sample-build-docker-args", "--build-arg=foo=bar")
					br.AssertSuccess()
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By("verifying the build output contains the changes.")
					o.Expect(buildLog).To(o.ContainSubstring("bar"))
				})
				g.It("Should complete with a warning on non-existent build-arg", func() {
					g.By("starting the build with --build-arg flag")
					br, _ := exutil.StartBuildAndWait(oc, "sample-build-docker-args", "--build-arg=bar=foo")
					br.AssertSuccess()
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By("verifying the build completed with a warning.")
					o.Expect(buildLog).To(o.ContainSubstring("One or more build-args [bar] were not consumed"))
				})
			})

			g.Describe("Trigger builds with branch refs matching directories on master branch", func() {

				g.It("Should checkout the config branch, not config directory", func() {
					g.By("calling oc new-app")
					_, err := oc.Run("new-app").Args("https://github.com/openshift/ruby-hello-world#config").Output()
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("waiting for the build to complete")
					err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), "ruby-hello-world-1", nil, nil, nil)
					if err != nil {
						exutil.DumpBuildLogs("ruby-hello-world", oc)
					}
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("get build logs, confirm commit in the config branch is present")
					out, err := oc.Run("logs").Args("build/ruby-hello-world-1").Output()
					o.Expect(out).To(o.ContainSubstring("Merge pull request #61 from gabemontero/config"))
				})
			})

			g.Describe("start a build via a webhook", func() {
				g.It("should be able to start builds via the webhook with valid secrets and fail with invalid secrets", func() {
					g.By("clearing existing builds")
					_, err := oc.Run("delete").Args("builds", "--all").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					builds, err := oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).To(o.BeEmpty())

					g.By("getting the api server host")
					out, err := oc.WithoutNamespace().Run("status").Args().Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					e2e.Logf("got status value of: %s", out)
					matcher := regexp.MustCompile("https?://.*?8443")
					apiServer := matcher.FindString(out)
					o.Expect(apiServer).NotTo(o.BeEmpty())

					out, err = oc.Run("describe").Args("bc", "sample-build").Output()
					e2e.Logf("build description: %s", out)

					g.By("starting the build via the webhook with the deprecated secret")
					curlArgs := []string{"-X",
						"POST",
						"-k",
						fmt.Sprintf("%s/apis/build.openshift.io/v1/namespaces/%s/buildconfigs/sample-build/webhooks/%s/generic",
							apiServer, oc.Namespace(), "mysecret"),
					}
					curlOut, err := exec.Command("curl", curlArgs...).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					e2e.Logf("curl cmd: %v, output: %s", curlArgs, string(curlOut))
					builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).NotTo(o.BeEmpty())

					g.By("clearing existing builds")
					_, err = oc.Run("delete").Args("builds", "--all").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).To(o.BeEmpty())

					g.By("starting the build via the webhook with the referenced secret")
					curlArgs = []string{"-X",
						"POST",
						"-k",
						fmt.Sprintf("%s/apis/build.openshift.io/v1/namespaces/%s/buildconfigs/sample-build/webhooks/%s/generic",
							apiServer, oc.Namespace(), "secretvalue1"),
					}
					curlOut, err = exec.Command("curl", curlArgs...).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					e2e.Logf("curl cmd: %s, output: %s", curlArgs, string(curlOut))
					builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).NotTo(o.BeEmpty())

					g.By("clearing existing builds")
					_, err = oc.Run("delete").Args("builds", "--all").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).To(o.BeEmpty())

					g.By("starting the build via the webhook with an invalid secret")
					curlArgs = []string{"-X",
						"POST",
						"-k",
						fmt.Sprintf("%s/apis/build.openshift.io/v1/namespaces/%s/buildconfigs/sample-build/webhooks/%s/generic",
							apiServer, oc.Namespace(), "invalid"),
					}
					curlOut, err = exec.Command("curl", curlArgs...).Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					e2e.Logf("curl cmd: %v, output: %s", curlArgs, string(curlOut))
					builds, err = oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).To(o.BeEmpty())

				})
			})

		})
	})
})
