package builds

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/jenkins"
)

const (
	localClientPluginSnapshotImageStream = "jenkins-client-plugin-snapshot-test"
	localClientPluginSnapshotImage       = "openshift/" + localClientPluginSnapshotImageStream + ":latest"
	localSyncPluginSnapshotImageStream   = "jenkins-sync-plugin-snapshot-test"
	localSyncPluginSnapshotImage         = "openshift/" + localSyncPluginSnapshotImageStream + ":latest"
	clientLicenseText                    = "About OpenShift Client Jenkins Plugin"
	syncLicenseText                      = "About OpenShift Sync"
	clientPluginName                     = "openshift-client"
	syncPluginName                       = "openshift-sync"
	secretName                           = "secret-to-credential"
	secretCredentialSyncLabel            = "credential.sync.jenkins.openshift.io"
)

func debugAnyJenkinsFailure(br *exutil.BuildResult, name string, oc *exutil.CLI, dumpMaster bool) {
	if !br.BuildSuccess {
		br.LogDumper = jenkins.DumpLogs
		fmt.Fprintf(g.GinkgoWriter, "\n\n START debugAnyJenkinsFailure\n\n")
		j := jenkins.NewRef(oc)
		jobLog, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "")
		if err == nil {
			fmt.Fprintf(g.GinkgoWriter, "\n %s job log:\n%s", name, jobLog)
		} else {
			fmt.Fprintf(g.GinkgoWriter, "\n error getting %s job log: %#v", name, err)
		}
		if dumpMaster {
			exutil.DumpApplicationPodLogs("jenkins", oc)
		}
		fmt.Fprintf(g.GinkgoWriter, "\n\n END debugAnyJenkinsFailure\n\n")
	}
}

var _ = g.Describe("[Feature:Builds][Slow] openshift pipeline build", func() {
	defer g.GinkgoRecover()
	var (
		jenkinsEphemeralTemplatePath  = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		jenkinsPersistentTemplatePath = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-persistent-template.json")
		nodejsDeclarativePipelinePath = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "nodejs-sample-pipeline.yaml")
		mavenSlavePipelinePath        = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "maven-pipeline.yaml")
		mavenSlaveGradlePipelinePath  = exutil.FixturePath("testdata", "builds", "gradle-pipeline.yaml")
		//orchestrationPipelinePath = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "mapsapp-pipeline.yaml")
		blueGreenPipelinePath         = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "bluegreen-pipeline.yaml")
		clientPluginPipelinePath      = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "openshift-client-plugin-pipeline.yaml")
		envVarsPipelinePath           = exutil.FixturePath("testdata", "samplepipeline-withenvs.yaml")
		origPipelinePath              = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "samplepipeline.yaml")
		configMapPodTemplatePath      = exutil.FixturePath("testdata", "config-map-jenkins-slave-pods.yaml")
		imagestreamPodTemplatePath    = exutil.FixturePath("testdata", "imagestream-jenkins-slave-pods.yaml")
		imagestreamtagPodTemplatePath = exutil.FixturePath("testdata", "imagestreamtag-jenkins-slave-pods.yaml")
		podTemplateSlavePipelinePath  = exutil.FixturePath("testdata", "jenkins-slave-template.yaml")
		secretPath                    = exutil.FixturePath("testdata", "openshift-secret-to-jenkins-credential.yaml")

		oc                       = exutil.NewCLI("jenkins-pipeline", exutil.KubeConfigPath())
		ticker                   *time.Ticker
		j                        *jenkins.JenkinsRef
		dcLogFollow              *exec.Cmd
		dcLogStdOut, dcLogStdErr *bytes.Buffer
		setupJenkins             = func(jenkinsTemplatePath string) {
			exutil.DumpDockerInfo()
			// Deploy Jenkins
			// NOTE, we use these tests for both a) nightly regression runs against the latest openshift jenkins image on docker hub, and
			// b) PR testing for changes to the various openshift jenkins plugins we support.  With scenario b), a docker image that extends
			// our jenkins image is built, where the proposed plugin change is injected, overwritting the current released version of the plugin.
			// Our test/PR jobs on ci.openshift create those images, as well as set env vars this test suite looks for.  When both the env var
			// and test image is present, a new image stream is created using the test image, and our jenkins template is instantiated with
			// an override to use that images stream and test image
			var licensePrefix, pluginName string
			useSnapshotImage := false

			err := oc.Run("create").Args("-n", oc.Namespace(), "-f", jenkinsTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			jenkinsTemplateName := "jenkins-ephemeral"

			// create persistent volumes if running persistent jenkins
			if jenkinsTemplatePath == jenkinsPersistentTemplatePath {
				jenkinsTemplateName = "jenkins-persistent"

				err = exutil.AddNamespaceLabelToPersistentVolumeClaimsInTemplate(oc, jenkinsTemplateName)
				o.Expect(err).NotTo(o.HaveOccurred())

				_, err := exutil.SetupNFSBackedPersistentVolumes(oc, "2Gi", 3)
				o.Expect(err).NotTo(o.HaveOccurred())

			}

			// our pipeline jobs, between jenkins and oc invocations, need more mem than the default
			newAppArgs := []string{"--template", fmt.Sprintf("%s/%s", oc.Namespace(), jenkinsTemplateName), "-p", "MEMORY_LIMIT=2Gi", "-p", "DISABLE_ADMINISTRATIVE_MONITORS=true"}
			clientPluginNewAppArgs, useClientPluginSnapshotImage := jenkins.SetupSnapshotImage(jenkins.UseLocalClientPluginSnapshotEnvVarName, localClientPluginSnapshotImage, localClientPluginSnapshotImageStream, newAppArgs, oc)
			syncPluginNewAppArgs, useSyncPluginSnapshotImage := jenkins.SetupSnapshotImage(jenkins.UseLocalSyncPluginSnapshotEnvVarName, localSyncPluginSnapshotImage, localSyncPluginSnapshotImageStream, newAppArgs, oc)

			switch {
			case useClientPluginSnapshotImage && useSyncPluginSnapshotImage:
				fmt.Fprintf(g.GinkgoWriter,
					"\nBOTH %s and %s for PR TESTING ARE SET.  WILL NOT CHOOSE BETWEEN THE TWO SO TESTING CURRENT PLUGIN VERSIONS IN LATEST OPENSHIFT JENKINS IMAGE ON DOCKER HUB.\n",
					jenkins.UseLocalClientPluginSnapshotEnvVarName, jenkins.UseLocalSyncPluginSnapshotEnvVarName)
			case useClientPluginSnapshotImage:
				fmt.Fprintf(g.GinkgoWriter, "\nTHE UPCOMING TESTS WILL LEVERAGE AN IMAGE THAT EXTENDS THE LATEST OPENSHIFT JENKINS IMAGE AND OVERRIDES THE OPENSHIFT CLIENT PLUGIN WITH A NEW VERSION BUILT FROM PROPOSED CHANGES TO THAT PLUGIN.\n")
				licensePrefix = clientLicenseText
				pluginName = clientPluginName
				useSnapshotImage = true
				newAppArgs = clientPluginNewAppArgs
			case useSyncPluginSnapshotImage:
				fmt.Fprintf(g.GinkgoWriter, "\nTHE UPCOMING TESTS WILL LEVERAGE AN IMAGE THAT EXTENDS THE LATEST OPENSHIFT JENKINS IMAGE AND OVERRIDES THE OPENSHIFT SYNC PLUGIN WITH A NEW VERSION BUILT FROM PROPOSED CHANGES TO THAT PLUGIN.\n")
				licensePrefix = syncLicenseText
				pluginName = syncPluginName
				useSnapshotImage = true
				newAppArgs = syncPluginNewAppArgs
			default:
				fmt.Fprintf(g.GinkgoWriter, "\nNO PR TEST ENV VARS SET SO TESTING CURRENT PLUGIN VERSIONS IN LATEST OPENSHIFT JENKINS IMAGE ON DOCKER HUB.\n")
			}

			g.By(fmt.Sprintf("calling oc new-app useSnapshotImage %v with license text %s and newAppArgs %#v", useSnapshotImage, licensePrefix, newAppArgs))
			err = oc.Run("new-app").Args(newAppArgs...).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for jenkins deployment")
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), "jenkins", 1, false, oc)
			if err != nil {
				exutil.DumpApplicationPodLogs("jenkins", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			j = jenkins.NewRef(oc)

			g.By("wait for jenkins to come up")
			_, err = j.WaitForContent("", 200, 10*time.Minute, "")

			if err != nil {
				exutil.DumpApplicationPodLogs("jenkins", oc)
			}

			o.Expect(err).NotTo(o.HaveOccurred())

			if useSnapshotImage {
				g.By("verifying the test image is being used")
				// for the test image, confirm that a snapshot version of the plugin is running in the jenkins image we'll test against
				_, err = j.WaitForContent(licensePrefix+` ([0-9\.]+)-SNAPSHOT`, 200, 10*time.Minute, "/pluginManager/plugin/"+pluginName+"/thirdPartyLicenses")
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			// Start capturing logs from this deployment config.
			// This command will terminate if the Jenkins instance crashes. This
			// ensures that even if the Jenkins DC restarts, we should capture
			// logs from the crash.
			dcLogFollow, dcLogStdOut, dcLogStdErr, err = oc.Run("logs").Args("-f", "dc/jenkins").Background()
			o.Expect(err).NotTo(o.HaveOccurred())

		}
	)

	g.Context("Pipelines with maven slave", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())

			// additionally ensure that the build works in a memory constrained
			// environment
			_, err = oc.AdminKubeClient().Core().LimitRanges(oc.Namespace()).Create(&v1.LimitRange{
				ObjectMeta: metav1.ObjectMeta{
					Name: "limitrange",
				},
				Spec: v1.LimitRangeSpec{
					Limits: []v1.LimitRangeItem{
						{
							Type: v1.LimitTypeContainer,
							Default: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
					},
				},
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}
		})

		g.It("should build a maven project and complete successfully", func() {
			// instantiate the template
			g.By(fmt.Sprintf("calling oc new-app -f %q", mavenSlavePipelinePath))
			err := oc.Run("new-app").Args("-f", mavenSlavePipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build
			g.By("starting the pipeline build and waiting for it to complete")
			br, err := exutil.StartBuildAndWait(oc, "openshift-jee-sample")
			if err != nil || !br.BuildSuccess {
				exutil.DumpBuilds(oc)
				exutil.DumpPodLogsStartingWith("maven", oc)
				exutil.DumpBuildLogs("openshift-jee-sample-docker", oc)
				exutil.DumpDeploymentLogs("openshift-jee-sample", 1, oc)
			}
			debugAnyJenkinsFailure(br, oc.Namespace()+"-openshift-jee-sample", oc, true)
			br.AssertSuccess()

			// wait for the service to be running
			g.By("expecting the openshift-jee-sample service to be deployed and running")
			_, err = exutil.GetEndpointAddress(oc, "openshift-jee-sample")
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("should build a gradle project and complete successfully", func() {
			err := oc.Run("create").Args("-f", mavenSlaveGradlePipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build
			g.By("waiting for the build to complete")
			br := &exutil.BuildResult{Oc: oc, BuildName: "gradle-1"}
			err = exutil.WaitForBuildResult(oc.BuildClient().Build().Builds(oc.Namespace()), br)
			if err != nil || !br.BuildSuccess {
				exutil.DumpBuilds(oc)
				exutil.DumpPodLogsStartingWith("maven", oc)
			}
			debugAnyJenkinsFailure(br, oc.Namespace()+"-gradle-pipeline", oc, true)
			br.AssertSuccess()
		})
	})

	g.Context("Pipelines with declarative syntax", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}
		})

		g.It("should build successfully", func() {
			// create the bc
			g.By(fmt.Sprintf("calling oc create -f %q", nodejsDeclarativePipelinePath))
			err := oc.Run("create").Args("-f", nodejsDeclarativePipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build
			g.By("starting the pipeline build and waiting for it to complete")
			br, err := exutil.StartBuildAndWait(oc, "nodejs-sample-pipeline")
			if err != nil || !br.BuildSuccess {
				exutil.DumpBuilds(oc)
				exutil.DumpPodLogsStartingWith("nodejs", oc)
				exutil.DumpBuildLogs("nodejs-mongodb-example", oc)
				exutil.DumpDeploymentLogs("mongodb", 1, oc)
				exutil.DumpDeploymentLogs("nodejs-mongodb-example", 1, oc)
			}
			debugAnyJenkinsFailure(br, oc.Namespace()+"-nodejs-sample-pipeline", oc, true)
			br.AssertSuccess()

			// wait for the service to be running
			g.By("expecting the openshift-jee-sample service to be deployed and running")
			_, err = exutil.GetEndpointAddress(oc, "nodejs-mongodb-example")
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Context("Deleted pipeline strategy buildconfigs", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}
		})

		g.It("should not be recreated by the sync plugin", func() {
			// create the bc
			g.By(fmt.Sprintf("calling oc new-app -f %q", origPipelinePath))
			err := oc.Run("new-app").Args("-f", origPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verify job is in jenkins")
			_, err = j.WaitForContent("", 200, 30*time.Second, "job/%s/job/%s-sample-pipeline/", oc.Namespace(), oc.Namespace())
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("delete pipeline strategy bc %q", origPipelinePath))
			err = oc.Run("delete").Args("bc", "sample-pipeline").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verify job is not in jenkins")
			_, err = j.WaitForContent("", 404, 30*time.Second, "job/%s/job/%s-sample-pipeline/", oc.Namespace(), oc.Namespace())
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verify bc is still deleted")
			err = oc.Run("get").Args("bc", "sample-pipeline").Execute()
			o.Expect(err).To(o.HaveOccurred())
		})
	})

	g.Context("Sync secret to credential", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsGCStats) != "" {
				g.By("stopping jenkins gc tracking")
				ticker.Stop()
			}
		})

		g.It("should map openshift secret to a jenkins credential as the secret is manipulated", func() {
			g.By("create secret for jenkins credential")
			err := oc.Run("create").Args("-f", secretPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verify credential created since label should be there")
			// NOTE, for the credential URL in Jenkins
			// it returns rc 200 with no output if credential exists and a 404 if it does not exists
			_, err = j.WaitForContent("", 200, 10*time.Second, "credentials/store/system/domain/_/credential/%s-%s/", oc.Namespace(), secretName)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verify credential deleted when label removed")
			err = oc.Run("label").Args("secret", secretName, secretCredentialSyncLabel+"-").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = j.WaitForContent("", 404, 10*time.Second, "credentials/store/system/domain/_/credential/%s-%s/", oc.Namespace(), secretName)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verify credential added when label added")
			err = oc.Run("label").Args("secret", secretName, secretCredentialSyncLabel+"=true").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = j.WaitForContent("", 200, 10*time.Second, "credentials/store/system/domain/_/credential/%s-%s/", oc.Namespace(), secretName)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verify credential deleted when secret deleted")
			err = oc.Run("delete").Args("secret", secretName).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			_, err = j.WaitForContent("", 404, 10*time.Second, "credentials/store/system/domain/_/credential/%s-%s/", oc.Namespace(), secretName)
			o.Expect(err).NotTo(o.HaveOccurred())

		})

	})

	g.Context("Pipeline using config map slave", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsGCStats) != "" {
				g.By("stopping jenkins gc tracking")
				ticker.Stop()
			}
		})

		g.It("should build and complete successfully", func() {
			g.By("create the pod template with config map")
			err := oc.Run("create").Args("-f", configMapPodTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc new-app -f %q", podTemplateSlavePipelinePath))
			err = oc.Run("new-app").Args("-f", podTemplateSlavePipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the pipeline build and waiting for it to complete")
			br, err := exutil.StartBuildAndWait(oc, "openshift-jee-sample")
			if err != nil || !br.BuildSuccess {
				exutil.DumpBuilds(oc)
			}
			debugAnyJenkinsFailure(br, oc.Namespace()+"-openshift-jee-sample", oc, true)
			br.AssertSuccess()

			g.By("getting job log, make sure has success message")
			out, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("making sure job log ran with our config map slave pod template")
			o.Expect(out).To(o.ContainSubstring("Running on jenkins-slave"))
		})
	})

	g.Context("Pipeline using imagestream slave", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsGCStats) != "" {
				g.By("stopping jenkins gc tracking")
				ticker.Stop()
			}
		})

		g.It("should build and complete successfully", func() {
			g.By("create the pod template with imagestream")
			err := oc.Run("create").Args("-f", imagestreamPodTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc new-app -f %q", podTemplateSlavePipelinePath))
			err = oc.Run("new-app").Args("-f", podTemplateSlavePipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the pipeline build and waiting for it to complete")
			br, err := exutil.StartBuildAndWait(oc, "openshift-jee-sample")
			if err != nil || !br.BuildSuccess {
				exutil.DumpBuilds(oc)
			}
			debugAnyJenkinsFailure(br, oc.Namespace()+"-openshift-jee-sample", oc, true)
			br.AssertSuccess()

			g.By("getting job log, making sure job log has success message")
			out, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("making sure job log ran with our config map slave pod template")
			o.Expect(out).To(o.ContainSubstring("Running on jenkins-slave"))
		})
	})

	g.Context("Pipeline using imagestreamtag slave", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsGCStats) != "" {
				g.By("stopping jenkins gc tracking")
				ticker.Stop()
			}
		})

		g.It("should build and complete successfully", func() {
			g.By("create the pod template with imagestream")
			err := oc.Run("create").Args("-f", imagestreamtagPodTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc new-app -f %q", podTemplateSlavePipelinePath))
			err = oc.Run("new-app").Args("-f", podTemplateSlavePipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting the pipeline build and waiting for it to complete")
			br, err := exutil.StartBuildAndWait(oc, "openshift-jee-sample")
			if err != nil || !br.BuildSuccess {
				exutil.DumpBuilds(oc)
			}
			debugAnyJenkinsFailure(br, oc.Namespace()+"-openshift-jee-sample", oc, true)
			br.AssertSuccess()

			g.By("getting job log, making sure job log has success message")
			out, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("making sure job log ran with our config map slave pod template")
			o.Expect(out).To(o.ContainSubstring("Running on slave-jenkins"))
		})
	})

	g.Context("Pipeline using jenkins-client-plugin", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}
		})

		g.It("should build and complete successfully", func() {
			// instantiate the bc
			g.By("create the jenkins pipeline strategy build config that leverages openshift client plugin")
			err := oc.Run("create").Args("-f", clientPluginPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build - we run it twice because our sample pipeline exercises different paths of the client
			// plugin based on whether certain resources already exist or not
			for i := 0; i < 2; i++ {
				g.By(fmt.Sprintf("starting the pipeline build and waiting for it to complete, pass: %d", i))
				br, err := exutil.StartBuildAndWait(oc, "sample-pipeline-openshift-client-plugin")
				debugAnyJenkinsFailure(br, oc.Namespace()+"-sample-pipeline-openshift-client-plugin", oc, true)
				if err != nil || !br.BuildSuccess {
					exutil.DumpBuilds(oc)
					exutil.DumpBuildLogs("ruby", oc)
					exutil.DumpDeploymentLogs("mongodb", 1, oc)
					exutil.DumpDeploymentLogs("jenkins-second-deployment", 1, oc)
					exutil.DumpDeploymentLogs("jenkins-second-deployment", 2, oc)
				}
				br.AssertSuccess()

				g.By("get build console logs and see if succeeded")
				_, err = j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
	})

	g.Context("Pipeline with env vars", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}
		})

		g.It("should build and complete successfully", func() {
			// instantiate the bc
			g.By(fmt.Sprintf("calling oc new-app -f %q", envVarsPipelinePath))
			err := oc.Run("new-app").Args("-f", envVarsPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build
			g.By("starting the pipeline build, including env var, and waiting for it to complete")
			br, err := exutil.StartBuildAndWait(oc, "-e", "FOO2=BAR2", "sample-pipeline-withenvs")
			if err != nil || !br.BuildSuccess {
				exutil.DumpBuilds(oc)
			}
			debugAnyJenkinsFailure(br, oc.Namespace()+"-sample-pipeline-withenvs", oc, true)
			br.AssertSuccess()

			g.By("confirm all the log annotations are there")
			_, err = jenkins.ProcessLogURLAnnotations(oc, br)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("get build console logs and see if succeeded")
			out, err := j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
			if err != nil {
				exutil.DumpApplicationPodLogs("jenkins", oc)
				exutil.DumpBuilds(oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("and see if env is set")
			if !strings.Contains(out, "FOO2 is BAR2") {
				exutil.DumpApplicationPodLogs("jenkins", oc)
				exutil.DumpBuilds(oc)
				o.Expect(out).To(o.ContainSubstring("FOO2 is BAR2"))
			}

			// start the nextbuild
			g.By("starting the pipeline build and waiting for it to complete")
			br, err = exutil.StartBuildAndWait(oc, "sample-pipeline-withenvs")
			if err != nil || !br.BuildSuccess {
				exutil.DumpApplicationPodLogs("jenkins", oc)
				exutil.DumpBuilds(oc)
			}
			debugAnyJenkinsFailure(br, oc.Namespace()+"-sample-pipeline-withenvs", oc, true)
			br.AssertSuccess()

			g.By("get build console logs and see if succeeded")
			out, err = j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
			if err != nil {
				exutil.DumpApplicationPodLogs("jenkins", oc)
				exutil.DumpBuilds(oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("and see if env FOO1 is set")
			if !strings.Contains(out, "FOO1 is BAR1") {
				exutil.DumpApplicationPodLogs("jenkins", oc)
				exutil.DumpBuilds(oc)
				o.Expect(out).To(o.ContainSubstring("FOO1 is BAR1"))
			}

			g.By("and see if env FOO2 is still not set")
			if !strings.Contains(out, "FOO2 is null") {
				exutil.DumpApplicationPodLogs("jenkins", oc)
				exutil.DumpBuilds(oc)
				o.Expect(out).To(o.ContainSubstring("FOO2 is null"))
			}

		})
	})

	/*g.Context("Orchestration pipeline", func() {
	g.BeforeEach(func() {
		setupJenkins(jenkinsEphemeralTemplatePath)

		if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
			ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
		}

		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

		g.AfterEach(func() {
			if os.Getenv(jenkins.EnableJenkinsGCStats) != "" {
				g.By("stopping jenkins gc tracking")
				ticker.Stop()
			}
		})

		g.It("should build and complete successfully", func() {
			// instantiate the template
			g.By(fmt.Sprintf("calling oc new-app -f %q", orchestrationPipelinePath))
			err := oc.Run("new-app").Args("-f", orchestrationPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build
			g.By("starting the pipeline build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "mapsapp-pipeline")
			debugAnyJenkinsFailure(br, oc.Namespace()+"-mapsapp-pipeline", oc, true)
			debugAnyJenkinsFailure(br, oc.Namespace()+"-mlbparks-pipeline", oc, false)
			debugAnyJenkinsFailure(br, oc.Namespace()+"-nationalparks-pipeline", oc, false)
			debugAnyJenkinsFailure(br, oc.Namespace()+"-parksmap-pipeline", oc, false)
			br.AssertSuccess()

			// wait for the service to be running
			g.By("expecting the parksmap-web service to be deployed and running")
			_, err = exutil.GetEndpointAddress(oc, "parksmap")
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})*/

	g.Context("Blue-green pipeline", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}
		})

		g.It("Blue-green pipeline should build and complete successfully", func() {
			// instantiate the template
			g.By(fmt.Sprintf("calling oc new-app -f %q", blueGreenPipelinePath))
			err := oc.Run("new-app").Args("-f", blueGreenPipelinePath, "-p", "VERBOSE=true").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			buildAndSwitch := func(newColour string) {
				// start the build
				g.By("starting the bluegreen pipeline build")
				br, err := exutil.StartBuildResult(oc, "bluegreen-pipeline")
				if err != nil || !br.BuildSuccess {
					debugAnyJenkinsFailure(br, oc.Namespace()+"-bluegreen-pipeline", oc, false)
					exutil.DumpBuilds(oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				errs := make(chan error, 2)
				stop := make(chan struct{})
				go func() {
					defer g.GinkgoRecover()

					g.By("Waiting for the build uri")
					var jenkinsBuildURI string
					for {
						build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.BuildName, metav1.GetOptions{})
						if err != nil {
							errs <- fmt.Errorf("error getting build: %s", err)
							return
						}
						jenkinsBuildURI = build.Annotations[buildapi.BuildJenkinsBuildURIAnnotation]
						if jenkinsBuildURI != "" {
							break
						}

						select {
						case <-stop:
							return
						default:
							time.Sleep(10 * time.Second)
						}
					}

					url, err := url.Parse(jenkinsBuildURI)
					if err != nil {
						errs <- fmt.Errorf("error parsing build uri: %s", err)
						return
					}
					jenkinsBuildURI = strings.Trim(url.Path, "/") // trim leading https://host/ and trailing /

					g.By("Waiting for the approval prompt")
					for {
						body, status, err := j.GetResource(jenkinsBuildURI + "/consoleText")
						if err == nil && status == http.StatusOK && strings.Contains(body, "Approve?") {
							break
						}
						select {
						case <-stop:
							return
						default:
							time.Sleep(10 * time.Second)
						}
					}

					g.By("Approving the current build")
					_, _, err = j.Post(nil, jenkinsBuildURI+"/input/Approval/proceedEmpty", "")
					if err != nil {
						errs <- fmt.Errorf("error approving the current build: %s", err)
					}
				}()

				go func() {
					defer g.GinkgoRecover()

					for {
						build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(br.BuildName, metav1.GetOptions{})
						switch {
						case err != nil:
							errs <- fmt.Errorf("error getting build: %s", err)
							return
						case exutil.CheckBuildFailedFn(build):
							errs <- nil
							return
						case exutil.CheckBuildSuccessFn(build):
							br.BuildSuccess = true
							errs <- nil
							return
						}
						select {
						case <-stop:
							return
						default:
							time.Sleep(5 * time.Second)
						}
					}
				}()

				g.By("waiting for the build to complete")
				select {
				case <-time.After(60 * time.Minute):
					err = errors.New("timeout waiting for build to complete")
				case err = <-errs:
				}
				close(stop)

				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error occurred (%s): getting logs before failing\n", err)
				}

				debugAnyJenkinsFailure(br, oc.Namespace()+"-bluegreen-pipeline", oc, true)
				if err != nil || !br.BuildSuccess {
					debugAnyJenkinsFailure(br, oc.Namespace()+"-bluegreen-pipeline", oc, false)
					exutil.DumpBuilds(oc)
				}
				br.AssertSuccess()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("verifying that the main route has been switched to %s", newColour))
				value, err := oc.Run("get").Args("route", "nodejs-mongodb-example", "-o", "jsonpath={ .spec.to.name }").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				activeRoute := strings.TrimSpace(value)
				g.By(fmt.Sprintf("verifying that the active route is 'nodejs-mongodb-example-%s'", newColour))
				o.Expect(activeRoute).To(o.Equal(fmt.Sprintf("nodejs-mongodb-example-%s", newColour)))
			}

			buildAndSwitch("green")
			buildAndSwitch("blue")
		})
	})
	g.Context("delete jenkins job runs when the associated build is deleted", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsEphemeralTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}
		})

		g.It("should delete a job run when the associated build is deleted", func() {
			type buildInfo struct {
				number          int
				jenkinsBuildURI string
			}
			buildNameToBuildInfoMap := make(map[string]buildInfo)

			g.By(fmt.Sprintf("calling oc create -f %q", envVarsPipelinePath))
			err := oc.Run("new-app").Args("-f", envVarsPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting 5 pipeline builds")
			for i := 1; i <= 5; i++ {
				// start the build
				br, _ := exutil.StartBuildAndWait(oc, "sample-pipeline-withenvs")
				br.AssertSuccess()

				// get the build information
				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(fmt.Sprintf("sample-pipeline-withenvs-%d", i), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				jenkinsBuildURI, err := url.Parse(build.Annotations[buildapi.BuildJenkinsBuildURIAnnotation])
				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error parsing build uri: %s", err)
				}
				buildNameToBuildInfoMap[build.Name] = buildInfo{number: i, jenkinsBuildURI: jenkinsBuildURI.Path}
			}

			g.By("verifying that jobs exist for the 5 builds")
			for buildName, buildInfo := range buildNameToBuildInfoMap {
				_, status, err := j.GetResource(buildInfo.jenkinsBuildURI)
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.BuildClient().Build().Builds(oc.Namespace()).Get(buildName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(status == http.StatusOK).To(o.BeTrue(), "Jenkins job run does not exist for %s but should.", buildName)
			}

			g.By("deleting the even numbered builds")
			for buildName, buildInfo := range buildNameToBuildInfoMap {
				if buildInfo.number%2 == 0 {
					fmt.Fprintf(g.GinkgoWriter, "Deleting build: %s", buildName)
					err := oc.BuildClient().Build().Builds(oc.Namespace()).Delete(buildName, &metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

				}
			}

			g.By("giving the Jenkins sync plugin enough time to delete the job runs")
			err = wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
				for buildName, buildInfo := range buildNameToBuildInfoMap {
					_, status, err := j.GetResource(buildInfo.jenkinsBuildURI)
					o.Expect(err).NotTo(o.HaveOccurred())
					fmt.Fprintf(g.GinkgoWriter, "Checking %s, status: %v\n", buildName, status)
					if buildInfo.number%2 == 0 {
						if status == http.StatusOK {
							fmt.Fprintf(g.GinkgoWriter, "Jenkins job run exists for %s but shouldn't, retrying ...", buildName)
							return false, nil
						}
					}
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that the correct builds and jobs exist")
			for buildName, buildInfo := range buildNameToBuildInfoMap {
				_, status, err := j.GetResource(buildInfo.jenkinsBuildURI)
				o.Expect(err).NotTo(o.HaveOccurred())
				if buildInfo.number%2 == 0 {
					_, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(buildName, metav1.GetOptions{})
					o.Expect(err).To(o.HaveOccurred())
					o.Expect(status != http.StatusOK).To(o.BeTrue(), "Jenkins job run exists for %s but shouldn't.", buildName)
				} else {
					_, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(buildName, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(status == http.StatusOK).To(o.BeTrue(), "Jenkins job run does not exist for %s but should.", buildName)
				}
			}
		})
	})
	g.Context("delete jenkins job runs when jenkins re-establishes communications", func() {
		g.BeforeEach(func() {
			setupJenkins(jenkinsPersistentTemplatePath)

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			defer exutil.RemoveDeploymentConfigs(oc, "jenkins")
			defer exutil.RemoveNFSBackedPersistentVolumes(oc)

			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}
		})

		g.It("should delete job runs when the associated build is deleted - jenkins unreachable", func() {

			type buildInfo struct {
				number          int
				jenkinsBuildURI string
			}
			buildNameToBuildInfoMap := make(map[string]buildInfo)

			g.By(fmt.Sprintf("calling oc create -f %q", envVarsPipelinePath))
			err := oc.Run("new-app").Args("-f", envVarsPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting 5 pipeline builds")
			for i := 1; i <= 5; i++ {
				// start the build
				br, _ := exutil.StartBuildAndWait(oc, "sample-pipeline-withenvs")
				br.AssertSuccess()

				// get the build information
				build, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(fmt.Sprintf("sample-pipeline-withenvs-%d", i), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				jenkinsBuildURI, err := url.Parse(build.Annotations[buildapi.BuildJenkinsBuildURIAnnotation])

				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error parsing build uri: %s", err)
				}
				buildNameToBuildInfoMap[build.Name] = buildInfo{number: i, jenkinsBuildURI: jenkinsBuildURI.Path}
			}

			g.By("verifying that jobs exist for the 5 builds")
			for buildName, buildInfo := range buildNameToBuildInfoMap {
				_, status, err := j.GetResource(buildInfo.jenkinsBuildURI)
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.BuildClient().Build().Builds(oc.Namespace()).Get(buildName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(status == http.StatusOK).To(o.BeTrue(), "Jenkins job run does not exist for %s but should.", buildName)
			}

			g.By("scaling down jenkins")
			_, err = oc.Run("scale").Args("dc/jenkins", "--replicas=0").Output()

			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("deleting the even numbered builds")
			for buildName, buildInfo := range buildNameToBuildInfoMap {
				if buildInfo.number%2 == 0 {
					fmt.Fprintf(g.GinkgoWriter, "Deleting build: %s", buildName)
					err := oc.BuildClient().Build().Builds(oc.Namespace()).Delete(buildName, &metav1.DeleteOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

				}
			}

			g.By("scaling up jenkins")
			_, err = oc.Run("scale").Args("dc/jenkins", "--replicas=1").Output()

			g.By("wait for jenkins to come up")
			_, err = j.WaitForContent("", 200, 10*time.Minute, "")
			if err != nil {
				exutil.DumpApplicationPodLogs("jenkins", oc)
			}

			g.By("giving the Jenkins sync plugin enough time to delete the job runs")
			err = wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
				for buildName, buildInfo := range buildNameToBuildInfoMap {
					_, status, err := j.GetResource(buildInfo.jenkinsBuildURI)
					o.Expect(err).NotTo(o.HaveOccurred())
					fmt.Fprintf(g.GinkgoWriter, "Checking %s, status: %v\n", buildName, status)
					if buildInfo.number%2 == 0 {
						if status == http.StatusOK {
							fmt.Fprintf(g.GinkgoWriter, "Jenkins job run exists for %s but shouldn't, retrying ...", buildName)
							return false, nil
						}
					}
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that the correct builds and jobs exist")
			for buildName, buildInfo := range buildNameToBuildInfoMap {
				_, status, err := j.GetResource(buildInfo.jenkinsBuildURI)
				o.Expect(err).NotTo(o.HaveOccurred())
				fmt.Fprintf(g.GinkgoWriter, "Checking %s, status: %v\n", buildName, status)
				if buildInfo.number%2 == 0 {
					_, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(buildName, metav1.GetOptions{})
					o.Expect(err).To(o.HaveOccurred())
					o.Expect(status != http.StatusOK).To(o.BeTrue(), "Jenkins job run exists for %s but shouldn't.", buildName)
				} else {
					_, err := oc.BuildClient().Build().Builds(oc.Namespace()).Get(buildName, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(status == http.StatusOK).To(o.BeTrue(), "Jenkins job run does not exist for %s but should.", buildName)
				}
			}
		})
	})
})
