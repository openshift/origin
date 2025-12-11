package builds

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds][Slow] update failure status", func() {
	defer g.GinkgoRecover()

	var (
		postCommitHookFixture                  = exutil.FixturePath("testdata", "builds", "statusfail-postcommithook.yaml")
		fetchDockerSrc                         = exutil.FixturePath("testdata", "builds", "statusfail-fetchsourcedocker.yaml")
		fetchS2ISrc                            = exutil.FixturePath("testdata", "builds", "statusfail-fetchsources2i.yaml")
		fetchDockerImg                         = exutil.FixturePath("testdata", "builds", "statusfail-fetchimagecontentdocker.yaml")
		badContextDirS2ISrc                    = exutil.FixturePath("testdata", "builds", "statusfail-badcontextdirs2i.yaml")
		oomkilled                              = exutil.FixturePath("testdata", "builds", "statusfail-oomkilled.yaml")
		builderImageFixture                    = exutil.FixturePath("testdata", "builds", "statusfail-fetchbuilderimage.yaml")
		pushToRegistryFixture                  = exutil.FixturePath("testdata", "builds", "statusfail-pushtoregistry.yaml")
		failedAssembleFixture                  = exutil.FixturePath("testdata", "builds", "statusfail-failedassemble.yaml")
		failedGenericReason                    = exutil.FixturePath("testdata", "builds", "statusfail-genericreason.yaml")
		binaryBuildDir                         = exutil.FixturePath("testdata", "builds", "statusfail-assemble")
		oc                                     = exutil.NewCLI("update-buildstatus")
		StatusMessagePushImageToRegistryFailed = "Failed to push the image to the registry."
		StatusMessagePullBuilderImageFailed    = "Failed pulling builder image."
		StatusMessageFetchSourceFailed         = "Failed to fetch the input source."
		StatusMessageFetchImageContentFailed   = "Failed to extract image content."
		StatusMessageInvalidContextDirectory   = "The supplied context directory does not exist."
		StatusMessageGenericBuildFailed        = "Generic Build failure - check logs for details."
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Build status postcommit hook failure", func() {
			g.It("should contain the post commit hook failure reason and message [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", postCommitHookFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-postcommithook", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonGenericBuildFailed))
				o.Expect(build.Status.Message).To(o.Equal("Generic Build failure - check logs for details."))

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)

				// wait for the build to be updated w/ completiontimestamp which should also mean the logsnippet
				// is set if one is going to be set.
				err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
					// note this is the same build variable used in the test scope
					build, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
					if err != nil {
						return true, err
					}
					if len(build.Status.LogSnippet) != 0 {
						return true, nil
					}
					return false, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred(), "Should not get an error or timeout getting LogSnippet")
				o.Expect(len(build.Status.LogSnippet)).NotTo(o.Equal(0), "LogSnippet should be set to something for failed builds")
			})
		})

		g.Describe("Build status Docker fetch image content failure", func() {
			g.It("should contain the Docker build fetch image content reason and message [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", fetchDockerImg).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-fetchimagecontentdocker", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonFetchImageContentFailed))
				o.Expect(build.Status.Message).To(o.Equal(StatusMessageFetchImageContentFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)
			})
		})

		g.Describe("Build status Docker fetch source failure", func() {
			g.It("should contain the Docker build fetch source failure reason and message [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", fetchDockerSrc).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-fetchsourcedocker", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonFetchSourceFailed))
				o.Expect(build.Status.Message).To(o.Equal(StatusMessageFetchSourceFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)

				// wait for the build to be updated w/ an init container failure (git-clone) meaning the logsnippet
				// is set
				err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
					// note this is the same build variable used in the test scope
					build, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
					if err != nil {
						return true, err
					}
					if len(build.Status.LogSnippet) != 0 {
						return true, nil
					}
					return false, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred(), "Should not get an error or timeout getting LogSnippet")
				o.Expect(len(build.Status.LogSnippet)).NotTo(o.Equal(0), "LogSnippet should be set to something for failed git-clone in build")
			})
		})

		g.Describe("Build status S2I fetch source failure", func() {
			g.It("should contain the S2I fetch source failure reason and message [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", fetchS2ISrc).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-fetchsourcesourcetoimage", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonFetchSourceFailed))
				o.Expect(build.Status.Message).To(o.Equal(StatusMessageFetchSourceFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)
			})
		})

		g.Describe("Build status OutOfMemoryKilled", func() {
			g.It("should contain OutOfMemoryKilled failure reason and message [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", oomkilled).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-oomkilled", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				var build *buildv1.Build
				wait.PollImmediate(200*time.Millisecond, 30*time.Second, func() (bool, error) {
					build, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
					// In 4.15, status reason may be filed as Error rather than OOMKilled
					// (tracked in https://issues.redhat.com/browse/OCPBUGS-32498) and also there is a similar
					// discussion in upstream (i.e. https://github.com/kubernetes/kubernetes/issues/119600) which seems to be
					// fixed in 4.16. Therefore, we need to loosen the check by also relying on the oomkilled exit code 137
					// to unblock the dependants in 4.15.
					if build.Status.Reason != buildv1.StatusReasonOutOfMemoryKilled && build.Status.Reason != buildv1.StatusReasonGenericBuildFailed {
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Or(o.Equal(buildv1.StatusReasonOutOfMemoryKilled), o.Equal(buildv1.StatusReasonGenericBuildFailed)))
				if build.Status.Reason == buildv1.StatusReasonOutOfMemoryKilled {
					o.Expect(build.Status.Message).To(o.Equal("The build pod was killed due to an out of memory condition."))
				}

				pod, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).Get(context.Background(), build.Name+"-build", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				oomKilledExitCodeFound := false
				for _, c := range pod.Status.ContainerStatuses {
					if c.State.Terminated == nil {
						continue
					}
					if c.State.Terminated.ExitCode == 137 {
						oomKilledExitCodeFound = true
					}
				}
				o.Expect(oomKilledExitCodeFound).To(o.BeTrue())

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)
			})
		})

		g.Describe("Build status S2I bad context dir failure", func() {
			g.It("should contain the S2I bad context dir failure reason and message [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", badContextDirS2ISrc).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-badcontextdirsourcetoimage", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonInvalidContextDirectory))
				o.Expect(build.Status.Message).To(o.Equal(StatusMessageInvalidContextDirectory))

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)
			})
		})

		g.Describe("Build status fetch builder image failure", func() {
			g.It("should contain the fetch builder image failure reason and message [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", builderImageFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-builderimage", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonPullBuilderImageFailed))
				o.Expect(build.Status.Message).To(o.Equal(StatusMessagePullBuilderImageFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)
			})
		})

		g.Describe("Build status push image to registry failure", func() {
			g.It("should contain the image push to registry failure reason and message [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", pushToRegistryFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-pushtoregistry", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				// Bug 1746499: Image without tag should push with <imageid>:latest
				logs, err := br.Logs()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(logs).NotTo(o.ContainSubstring("identifier is not an image"))

				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonPushImageToRegistryFailed))
				o.Expect(build.Status.Message).To(o.Equal(StatusMessagePushImageToRegistryFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)
			})
		})

		g.Describe("Build status failed assemble container", func() {
			g.It("should contain the failure reason related to an assemble script failing in s2i [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", failedAssembleFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-assemblescript", fmt.Sprintf("--from-dir=%s", binaryBuildDir), "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonGenericBuildFailed))
				o.Expect(build.Status.Message).To(o.Equal(StatusMessageGenericBuildFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)
			})
		})

		g.Describe("Build status failed https proxy invalid url", func() {
			g.It("should contain the generic failure reason and message [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
				err := oc.Run("create").Args("-f", failedGenericReason).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-genericfailure", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonGenericBuildFailed))
				o.Expect(build.Status.Message).To(o.Equal(StatusMessageGenericBuildFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().CoreV1(), br.Build, BuildFailedEventReason, BuildFailedEventMessage)
			})
		})
	})
})
