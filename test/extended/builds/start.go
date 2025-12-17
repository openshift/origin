package builds

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"

	buildv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/api/image/docker10"
	"github.com/openshift/library-go/pkg/image/imageutil"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] starting a build using CLI", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture      = exutil.FixturePath("testdata", "builds", "test-build.yaml")
		bcWithPRRef       = exutil.FixturePath("testdata", "builds", "test-bc-with-pr-ref.yaml")
		exampleGemfile    = exutil.FixturePath("testdata", "builds", "test-build-app", "Gemfile")
		exampleBuild      = exutil.FixturePath("testdata", "builds", "test-build-app")
		symlinkFixture    = exutil.FixturePath("testdata", "builds", "test-symlink-build.yaml")
		exampleGemfileURL = "https://raw.githubusercontent.com/openshift/ruby-hello-world/master/Gemfile"
		exampleArchiveURL = "https://github.com/openshift/ruby-hello-world/archive/master.zip"
		oc                = exutil.NewCLIWithPodSecurityLevel("cli-start-build", admissionapi.LevelBaseline)
		verifyBuildPod    = func(oc *exutil.CLI, name string) {
			// Check the build ran on a linux node
			pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), name+"-build", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			os, ok := pod.Spec.NodeSelector[corev1.LabelOSStable]
			o.Expect(ok).To(o.BeTrue())
			o.Expect(os).To(o.Equal("linux"))

			// CVE-2024-45496: .gitconfig can be abused to run aritrary commands.
			// Ensure the git-clone container does not run privileged and with minimum capabilities enabled.
			for _, initContainer := range pod.Spec.InitContainers {
				if initContainer.Name != "git-clone" {
					continue
				}
				o.Expect(initContainer.SecurityContext).NotTo(o.BeNil(), "git-clone container should have a security context")
				o.Expect(*initContainer.SecurityContext.Privileged).To(o.Or(o.BeNil(), o.BeEquivalentTo(false)), "git-clone container should not be privileged")
				o.Expect(initContainer.SecurityContext.SeccompProfile.Type).To(o.Or(o.BeNil(), o.BeEquivalentTo(corev1.SeccompProfileTypeRuntimeDefault)),
					"git-clone container should have the runtime default seccomp profile")
				capabilities := initContainer.SecurityContext.Capabilities
				o.Expect(capabilities).NotTo(o.BeNil(), "git-clone container should have capabilities defined")
				o.Expect(capabilities.Drop).NotTo(o.BeEmpty(), "git-clone container should drop ALL capabilities")
				for _, cap := range capabilities.Drop {
					o.Expect(cap).To(o.BeEquivalentTo("ALL"), "git-clone container should only drop the ALL capability")
				}
				for _, cap := range capabilities.Add {
					o.Expect(cap).To(o.Or(o.BeEquivalentTo("CHOWN"), o.BeEquivalentTo("DAC_OVERRIDE")),
						"git-clone is only allowed to have the following capabilities: %s",
						[]string{"CHOWN", "DAC_OVERRIDE"})
				}

			}

		}
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			oc.Run("create").Args("-f", buildFixture).Execute()
		})

		g.Context("start-build test context", func() {
			g.AfterEach(func() {
				if g.CurrentSpecReport().Failed() {
					exutil.DumpPodStates(oc)
					exutil.DumpPodLogsStartingWith("", oc)
				}
			})

			g.Describe("oc start-build --wait", func() {
				g.It("should start a build and wait for the build to complete [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with --wait flag")
					br, err := exutil.StartBuildAndWait(oc, "sample-build", "--wait")
					o.Expect(err).NotTo(o.HaveOccurred())
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
				})

				g.It("should start a build and wait for the build to fail [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with --wait flag but wrong --commit")
					br, _ := exutil.StartBuildAndWait(oc, "sample-build", "--wait", "--commit=fffffff")
					br.AssertFailure()
					o.Expect(br.StartBuildErr).To(o.HaveOccurred()) // start-build should detect the build error with --wait flag
					o.Expect(br.StartBuildStdErr).Should(o.ContainSubstring(`status is "Failed"`))
					verifyBuildPod(oc, br.BuildName)
				})
			})

			g.Describe("oc start-build with pr ref", func() {
				g.It("should start a build from a PR ref, wait for the build to complete, and confirm the right level was used [apigroup:build.openshift.io][apigroup:image.openshift.io]", g.Label("Size:L"), func() {
					g.By("make sure python imagestream has latest tag")
					err := exutil.WaitForAnImageStreamTag(oc.AsAdmin(), "openshift", "python", "latest")
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("create build config")
					err = oc.Run("create").Args("-f", bcWithPRRef).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("start the builds, wait for and confirm successful completion")
					br, err := exutil.StartBuildAndWait(oc, "bc-with-pr-ref-docker")
					o.Expect(err).NotTo(o.HaveOccurred())
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					br, err = exutil.StartBuildAndWait(oc, "bc-with-pr-ref")
					o.Expect(err).NotTo(o.HaveOccurred())
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					out, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())

					// the repo has a dependency 'gunicorn', referenced PR removes this dependency
					// from requirements.txt so it should not appear in the output anymore
					g.By("confirm the correct commit level was retrieved")
					o.Expect(out).Should(o.Not(o.ContainSubstring("gunicorn")))

					istag, err := oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get(context.Background(), "bc-with-pr-ref:latest", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					err = imageutil.ImageWithMetadata(&istag.Image)
					o.Expect(err).NotTo(o.HaveOccurred())
					imageutil.ImageWithMetadataOrDie(&istag.Image)
					o.Expect(istag.Image.DockerImageMetadata.Object.(*docker10.DockerImage).Config.Labels).To(o.HaveKeyWithValue("io.openshift.build.commit.ref", "refs/pull/121/head"))
					o.Expect(istag.Image.DockerImageMetadata.Object.(*docker10.DockerImage).Config.Env).To(o.ContainElement("OPENSHIFT_BUILD_REFERENCE=refs/pull/121/head"))

					istag, err = oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get(context.Background(), "bc-with-pr-ref-docker:latest", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					err = imageutil.ImageWithMetadata(&istag.Image)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(istag.Image.DockerImageMetadata.Object.(*docker10.DockerImage).Config.Labels).To(o.HaveKeyWithValue("io.openshift.build.commit.ref", "refs/pull/121/head"))
					o.Expect(istag.Image.DockerImageMetadata.Object.(*docker10.DockerImage).Config.Env).To(o.ContainElement("OPENSHIFT_BUILD_REFERENCE=refs/pull/121/head"))
				})

			})

			g.Describe("override environment", func() {
				g.It("should accept environment variables [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with -e FOO=bar,-e VAR=test")
					br, err := exutil.StartBuildAndWait(oc, "sample-build", "-e", "FOO=bar", "-e", "VAR=test")
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By(fmt.Sprintf("verifying the build output contains the env vars"))
					o.Expect(buildLog).To(o.ContainSubstring("FOO=bar"))
					o.Expect(buildLog).To(o.ContainSubstring("VAR=test"))

					g.By(fmt.Sprintf("verifying the build output contains inherited env vars"))
					// This variable is not set and thus inherited from the original build config
					o.Expect(buildLog).To(o.ContainSubstring("BAR=test"))
				})

				g.It("BUILD_LOGLEVEL in buildconfig should create verbose output [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with buildconfig strategy env BUILD_LOGLEVEL=5")
					br, err := exutil.StartBuildAndWait(oc, "sample-verbose-build")
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By(fmt.Sprintf("verifying the build output is verbose"))
					o.Expect(buildLog).To(o.ContainSubstring("Creating a new S2I builder"))
					o.Expect(buildLog).To(o.MatchRegexp("openshift-builder [1-9v]"))
					// Bug 1694871: logging before flag.Parse error
					g.By(fmt.Sprintf("verifying the build output has no error about flag.Parse"))
					o.Expect(buildLog).NotTo(o.ContainSubstring("ERROR: logging before flag.Parse"))
				})

				g.It("BUILD_LOGLEVEL in buildconfig can be overridden by build-loglevel [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with buildconfig strategy env BUILD_LOGLEVEL=5 but build-loglevel=1")
					br, err := exutil.StartBuildAndWait(oc, "sample-verbose-build", "--build-loglevel=1")
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By(fmt.Sprintf("verifying the build output is not verbose"))
					o.Expect(buildLog).NotTo(o.ContainSubstring("Creating a new S2I builder"))
					o.Expect(buildLog).NotTo(o.MatchRegexp("openshift-builder [1-9v]"))
				})

			})

			g.Describe("binary builds", func() {
				var commit string

				// do a best effort to initialize the repo in case it is a raw checkout or temp dir
				tryRepoInit := func(exampleBuild string) {
					out, err := exec.Command("bash", "-c", fmt.Sprintf("cd %q; if ! git rev-parse --git-dir; then git init .; git add .; git commit -m 'first'; touch foo; git add .; git commit -m 'second'; fi; true", exampleBuild)).CombinedOutput()
					fmt.Fprintf(g.GinkgoWriter, "Tried to init git repo: %v\n%s\n", err, string(out))
				}

				g.It("should accept --from-file as input [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with a Dockerfile")
					br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--from-file=%s", exampleGemfile))
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By(fmt.Sprintf("verifying the build %q status", br.BuildPath))

					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading file"))
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
					o.Expect(buildLog).To(o.ContainSubstring("Build complete"))
				})

				g.It("should accept --from-dir as input [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with a directory")
					br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--from-dir=%s", exampleBuild))
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By(fmt.Sprintf("verifying the build %q status", br.BuildPath))
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading directory"))
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
					o.Expect(buildLog).To(o.ContainSubstring("Build complete"))
				})

				g.It("should accept --from-repo as input [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with a Git repository")
					tryRepoInit(exampleBuild)
					br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--from-repo=%s", exampleBuild))
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())

					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading"))
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring(`at commit "HEAD"`))
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
					o.Expect(buildLog).To(o.ContainSubstring("Build complete"))
				})

				g.It("should accept --from-repo with --commit as input [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with a Git repository")
					tryRepoInit(exampleBuild)
					gitCmd := exec.Command("git", "rev-parse", "HEAD~1")
					gitCmd.Dir = exampleBuild
					commitByteArray, err := gitCmd.CombinedOutput()
					commit = strings.TrimSpace(string(commitByteArray[:]))
					o.Expect(err).NotTo(o.HaveOccurred())
					br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--commit=%s", commit), fmt.Sprintf("--from-repo=%s", exampleBuild))
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())

					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading"))
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring(fmt.Sprintf("at commit \"%s\"", commit)))
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
					o.Expect(buildLog).To(o.ContainSubstring(fmt.Sprintf("\"commit\":\"%s\"", commit)))
					o.Expect(buildLog).To(o.ContainSubstring("Build complete"))
				})

				// run one valid binary build so we can do --from-build later
				g.It("should reject binary build requests without a --from-xxxx value [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.Skip("TODO: refactor such that we don't rely on external package managers (i.e. Rubygems)")
					g.By("starting a valid build with a directory")
					br, err := exutil.StartBuildAndWait(oc, "sample-build-binary", "--follow", fmt.Sprintf("--from-dir=%s", exampleBuild))
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("Uploading directory"))
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("as binary input for the build ..."))
					o.Expect(buildLog).To(o.ContainSubstring("Build complete"))

					g.By("starting a build without a --from-xxxx value")
					br, err = exutil.StartBuildAndWait(oc, "sample-build-binary")
					o.Expect(br.StartBuildErr).To(o.HaveOccurred())
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("has no valid source inputs"))

					g.By("starting a build from an existing binary build")
					br, err = exutil.StartBuildAndWait(oc, "sample-build-binary", fmt.Sprintf("--from-build=%s", "sample-build-binary-1"))
					o.Expect(br.StartBuildErr).To(o.HaveOccurred())
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring("has no valid source inputs"))
				})

				g.It("shoud accept --from-file with https URL as an input [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting a valid build with input file served by https")
					br, err := exutil.StartBuildAndWait(oc, "sample-build", fmt.Sprintf("--from-file=%s", exampleGemfileURL))
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring(fmt.Sprintf("Uploading file from %q as binary input for the build", exampleGemfileURL)))
					o.Expect(buildLog).To(o.ContainSubstring("Build complete"))
				})

				g.It("shoud accept --from-archive with https URL as an input [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting a valid build with input archive served by https")
					// can't use sample-build-binary because we need contextDir due to github archives containing the top-level directory
					br, err := exutil.StartBuildAndWait(oc, "sample-build-github-archive", fmt.Sprintf("--from-archive=%s", exampleArchiveURL))
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(br.StartBuildStdErr).To(o.ContainSubstring(fmt.Sprintf("Uploading archive from %q as binary input for the build", exampleArchiveURL)))
					o.Expect(buildLog).To(o.ContainSubstring("Build complete"))
				})
			})

			g.Describe("cancel a binary build that doesn't start running in 5 minutes", func() {
				g.It("should start a build and wait for the build to be cancelled [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting a build with a nodeselector that can't be matched")
					go func() {
						exutil.StartBuild(oc, "sample-build-binary-invalidnodeselector", fmt.Sprintf("--from-file=%s", exampleGemfile))
					}()
					build := &buildv1.Build{}
					cancelFn := func(b *buildv1.Build) bool {
						build = b
						return exutil.CheckBuildCancelled(b)
					}
					err := exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "sample-build-binary-invalidnodeselector-1", nil, nil, cancelFn)
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(build.Status.Phase).To(o.Equal(buildv1.BuildPhaseCancelled))
					exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), build, BuildCancelledEventReason,
						BuildCancelledEventMessage)
				})
			})

			g.Describe("cancel a build started by oc start-build --wait", func() {
				g.It("should start a build and wait for the build to cancel [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
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
					build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), buildName, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(build).NotTo(o.BeNil(), "build object should exist")

					g.By(fmt.Sprintf("cancelling the build %q", buildName))
					err = oc.Run("cancel-build").Args(buildName).Execute()
					o.Expect(err).ToNot(o.HaveOccurred())
					wg.Wait()
					exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), build, BuildCancelledEventReason,
						BuildCancelledEventMessage)

				})

			})

			g.Describe("Setting build-args on Docker builds", func() {
				g.It("Should copy build args from BuildConfig to Build [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build without --build-arg flag")
					br, _ := exutil.StartBuildAndWait(oc, "sample-build-docker-args-preset")
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By("verifying the build output contains the build args from the BuildConfig.")
					o.Expect(buildLog).To(o.ContainSubstring("default"))
				})
				g.It("Should accept build args that are specified in the Dockerfile [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with --build-arg flag")
					br, _ := exutil.StartBuildAndWait(oc, "sample-build-docker-args", "--build-arg=foofoo=bar")
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By("verifying the build output contains the changes.")
					o.Expect(buildLog).To(o.ContainSubstring("bar"))
				})
				g.It("Should complete with a warning on non-existent build-arg [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("starting the build with --build-arg flag")
					br, _ := exutil.StartBuildAndWait(oc, "sample-build-docker-args", "--build-arg=bar=foo")
					br.AssertSuccess()
					verifyBuildPod(oc, br.BuildName)
					buildLog, err := br.Logs()
					o.Expect(err).NotTo(o.HaveOccurred())
					g.By("verifying the build completed with a warning.")
					o.Expect(buildLog).To(o.ContainSubstring("one or more build args were not consumed: [bar]"))
				})
			})

			g.Describe("Trigger builds with branch refs matching directories on master branch", func() {
				g.It("Should checkout the config branch, not config directory [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("calling oc new-app")
					_, err := oc.Run("new-app").Args("https://github.com/openshift/ruby-hello-world#config").Output()
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("waiting for the build to complete")
					err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "ruby-hello-world-1", nil, nil, nil)
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

				// AUTH-509: Webhooks do not allow unauthenticated requests by default.
				// Create a role binding which allows unauthenticated webhooks.
				g.BeforeEach(func() {
					ctx := context.Background()
					adminRoleBindingsClient := oc.AdminKubeClient().RbacV1().RoleBindings(oc.Namespace())
					_, err := adminRoleBindingsClient.Get(ctx, "webooks-unauth", metav1.GetOptions{})
					if apierrors.IsNotFound(err) {
						unauthWebhooksRB := &rbacv1.RoleBinding{
							ObjectMeta: metav1.ObjectMeta{
								Name: "webooks-unauth",
							},
							RoleRef: rbacv1.RoleRef{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     "ClusterRole",
								Name:     "system:webhook",
							},
							Subjects: []rbacv1.Subject{
								{
									APIGroup: "rbac.authorization.k8s.io",
									Kind:     "Group",
									Name:     "system:authenticated",
								},
								{
									APIGroup: "rbac.authorization.k8s.io",
									Kind:     "Group",
									Name:     "system:unauthenticated",
								},
							},
						}
						_, err = adminRoleBindingsClient.Create(ctx, unauthWebhooksRB, metav1.CreateOptions{})
						o.Expect(err).NotTo(o.HaveOccurred(), "creating webhook role binding")
						return
					}
					o.Expect(err).NotTo(o.HaveOccurred(), "checking if webhook role binding exists")
				})

				g.It("should be able to start builds via the webhook with valid secrets and fail with invalid secrets [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
					g.By("clearing existing builds")
					_, err := oc.Run("delete").Args("builds", "--all").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					builds, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).To(o.BeEmpty())

					g.By("getting the api server host")
					out, err := oc.WithoutNamespace().Run("status").Args().Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					e2e.Logf("got status value of: %s", out)
					matcher := regexp.MustCompile("https?://.*?443")
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
					builds, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).NotTo(o.BeEmpty())

					g.By("clearing existing builds")
					_, err = oc.Run("delete").Args("builds", "--all").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					builds, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
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
					builds, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).NotTo(o.BeEmpty())

					g.By("clearing existing builds")
					_, err = oc.Run("delete").Args("builds", "--all").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					builds, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
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
					builds, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).List(context.Background(), metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(builds.Items).To(o.BeEmpty())

				})
			})

			g.Describe("s2i build maintaining symlinks", func() {
				g.It(fmt.Sprintf("should s2i build image and maintain symlinks [apigroup:build.openshift.io][apigroup:image.openshift.io]"), g.Label("Size:L"), func() {
					g.By("initializing a local git repo")
					repo, err := exutil.NewGitRepo("symlinks")
					o.Expect(err).NotTo(o.HaveOccurred())
					defer repo.Remove()
					err = repo.AddAndCommit("package.json", "{\"scripts\" : {} }")
					o.Expect(err).NotTo(o.HaveOccurred())

					err = os.Symlink(repo.RepoPath+"/package.json", repo.RepoPath+"/link")
					o.Expect(err).NotTo(o.HaveOccurred())

					exutil.WaitForOpenShiftNamespaceImageStreams(oc)
					g.By(fmt.Sprintf("calling oc create -f %q", symlinkFixture))
					err = oc.Run("create").Args("-f", symlinkFixture).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("starting a build")
					err = oc.Run("start-build").Args("symlink-bc", "--from-dir", repo.RepoPath).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("waiting for build to finish")
					err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), "symlink-bc-1", exutil.CheckBuildSuccess, exutil.CheckBuildFailed, nil)
					if err != nil {
						exutil.DumpBuildLogs("symlink-bc", oc)
					}
					o.Expect(err).NotTo(o.HaveOccurred())

					tag, err := oc.ImageClient().ImageV1().ImageStreamTags(oc.Namespace()).Get(context.Background(), "symlink-is:latest", metav1.GetOptions{})
					err = oc.Run("run").Args("-i", "-t", "symlink-test", "--image="+tag.Image.DockerImageReference, "--restart=Never", "--command", "--", "bash", "-c", "if [ ! -L link ]; then ls -ltr; exit 1; fi").Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				})
			})
		})
	})
})
