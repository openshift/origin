package builds

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	s2istatus "github.com/openshift/source-to-image/pkg/util/status"

	buildv1 "github.com/openshift/api/build/v1"
	builderutil "github.com/openshift/origin/pkg/build/builder/util"
	buildutil "github.com/openshift/origin/pkg/build/util"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Slow] update failure status", func() {
	defer g.GinkgoRecover()

	var (
		// convert the s2i failure cases to our own StatusReason
		reasonAssembleFailed  = buildv1.StatusReason(s2istatus.ReasonAssembleFailed)
		messageAssembleFailed = string(s2istatus.ReasonMessageAssembleFailed)
		postCommitHookFixture = exutil.FixturePath("testdata", "builds", "statusfail-postcommithook.yaml")
		fetchDockerSrc        = exutil.FixturePath("testdata", "builds", "statusfail-fetchsourcedocker.yaml")
		fetchS2ISrc           = exutil.FixturePath("testdata", "builds", "statusfail-fetchsources2i.yaml")
		badContextDirS2ISrc   = exutil.FixturePath("testdata", "builds", "statusfail-badcontextdirs2i.yaml")
		oomkilled             = exutil.FixturePath("testdata", "builds", "statusfail-oomkilled.yaml")
		builderImageFixture   = exutil.FixturePath("testdata", "builds", "statusfail-fetchbuilderimage.yaml")
		pushToRegistryFixture = exutil.FixturePath("testdata", "builds", "statusfail-pushtoregistry.yaml")
		failedAssembleFixture = exutil.FixturePath("testdata", "builds", "statusfail-failedassemble.yaml")
		failedGenericReason   = exutil.FixturePath("testdata", "builds", "statusfail-genericreason.yaml")
		binaryBuildDir        = exutil.FixturePath("testdata", "builds", "statusfail-assemble")
		oc                    = exutil.NewCLI("update-buildstatus", exutil.KubeConfigPath())
	)

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for default service account")
			err := exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for builder service account")
			err = exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Build status postcommit hook failure", func() {
			g.It("should contain the post commit hook failure reason and message", func() {
				err := oc.Run("create").Args("-f", postCommitHookFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-postcommithook", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonPostCommitHookFailed))
				o.Expect(build.Status.Message).To(o.Equal(buildutil.StatusMessagePostCommitHookFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildutil.BuildFailedEventReason, buildutil.BuildFailedEventMessage)

				// wait for the build to be updated w/ completiontimestamp which should also mean the logsnippet
				// is set if one is going to be set.
				err = wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
					// note this is the same build variable used in the test scope
					build, err = oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
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

		g.Describe("Build status Docker fetch source failure", func() {
			g.It("should contain the Docker build fetch source failure reason and message", func() {
				err := oc.Run("create").Args("-f", fetchDockerSrc).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-fetchsourcedocker", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonFetchSourceFailed))
				o.Expect(build.Status.Message).To(o.Equal(builderutil.StatusMessageFetchSourceFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildutil.BuildFailedEventReason, buildutil.BuildFailedEventMessage)
			})
		})

		g.Describe("Build status S2I fetch source failure", func() {
			g.It("should contain the S2I fetch source failure reason and message", func() {
				err := oc.Run("create").Args("-f", fetchS2ISrc).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-fetchsourcesourcetoimage", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonFetchSourceFailed))
				o.Expect(build.Status.Message).To(o.Equal(builderutil.StatusMessageFetchSourceFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildutil.BuildFailedEventReason, buildutil.BuildFailedEventMessage)
			})
		})

		g.Describe("Build status OutOfMemoryKilled", func() {
			g.It("should contain OutOfMemoryKilled failure reason and message", func() {
				err := oc.Run("create").Args("-f", oomkilled).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-oomkilled", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				var build *buildv1.Build
				wait.PollImmediate(200*time.Millisecond, 30*time.Second, func() (bool, error) {
					build, err = oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
					if build.Status.Reason != buildv1.StatusReasonOutOfMemoryKilled {
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonOutOfMemoryKilled))
				o.Expect(build.Status.Message).To(o.Equal(buildutil.StatusMessageOutOfMemoryKilled))

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildutil.BuildFailedEventReason, buildutil.BuildFailedEventMessage)
			})
		})

		g.Describe("Build status S2I bad context dir failure", func() {
			g.It("should contain the S2I bad context dir failure reason and message", func() {
				err := oc.Run("create").Args("-f", badContextDirS2ISrc).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-badcontextdirsourcetoimage", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonInvalidContextDirectory))
				o.Expect(build.Status.Message).To(o.Equal(builderutil.StatusMessageInvalidContextDirectory))

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildutil.BuildFailedEventReason, buildutil.BuildFailedEventMessage)
			})
		})

		g.Describe("Build status fetch builder image failure", func() {
			g.It("should contain the fetch builder image failure reason and message", func() {
				err := oc.Run("create").Args("-f", builderImageFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-builderimage", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonPullBuilderImageFailed))
				o.Expect(build.Status.Message).To(o.Equal(builderutil.StatusMessagePullBuilderImageFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildutil.BuildFailedEventReason, buildutil.BuildFailedEventMessage)
			})
		})

		g.Describe("Build status push image to registry failure", func() {
			g.It("should contain the image push to registry failure reason and message", func() {
				err := oc.Run("create").Args("-f", pushToRegistryFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-pushtoregistry", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonPushImageToRegistryFailed))
				o.Expect(build.Status.Message).To(o.Equal(builderutil.StatusMessagePushImageToRegistryFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildutil.BuildFailedEventReason, buildutil.BuildFailedEventMessage)
			})
		})

		g.Describe("Build status failed assemble container", func() {
			g.It("should contain the failure reason related to an assemble script failing in s2i", func() {
				err := oc.Run("create").Args("-f", failedAssembleFixture).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-assemblescript", fmt.Sprintf("--from-dir=%s", binaryBuildDir), "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(reasonAssembleFailed))
				o.Expect(build.Status.Message).To(o.Equal(messageAssembleFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildutil.BuildFailedEventReason, buildutil.BuildFailedEventMessage)
			})
		})

		g.Describe("Build status failed https proxy invalid url", func() {
			g.It("should contain the generic failure reason and message", func() {
				err := oc.Run("create").Args("-f", failedGenericReason).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "statusfail-genericfailure", "--build-loglevel=5")
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertFailure()
				br.DumpLogs()

				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.Build.Name, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(build.Status.Reason).To(o.Equal(buildv1.StatusReasonGenericBuildFailed))
				o.Expect(build.Status.Message).To(o.Equal(builderutil.StatusMessageGenericBuildFailed))

				exutil.CheckForBuildEvent(oc.KubeClient().Core(), br.Build, buildutil.BuildFailedEventReason, buildutil.BuildFailedEventMessage)
			})
		})
	})
})
