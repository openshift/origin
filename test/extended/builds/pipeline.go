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
	envVarsPipelineGitRepoBuildConfig    = "test-build-app-pipeline"
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
		blueGreenPipelinePath                  = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "bluegreen-pipeline.yaml")
		clientPluginPipelinePath               = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "openshift-client-plugin-pipeline.yaml")
		envVarsPipelinePath                    = exutil.FixturePath("testdata", "samplepipeline-withenvs.yaml")
		origPipelinePath                       = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "samplepipeline.yaml")
		configMapPodTemplatePath               = exutil.FixturePath("testdata", "config-map-jenkins-slave-pods.yaml")
		imagestreamPodTemplatePath             = exutil.FixturePath("testdata", "imagestream-jenkins-slave-pods.yaml")
		imagestreamtagPodTemplatePath          = exutil.FixturePath("testdata", "imagestreamtag-jenkins-slave-pods.yaml")
		podTemplateSlavePipelinePath           = exutil.FixturePath("testdata", "jenkins-slave-template.yaml")
		multiNamespaceClientPluginPipelinePath = exutil.FixturePath("testdata", "multi-namespace-pipeline.yaml")
		secretPath                             = exutil.FixturePath("testdata", "openshift-secret-to-jenkins-credential.yaml")

		oc                       = exutil.NewCLI("jenkins-pipeline", exutil.KubeConfigPath())
		ticker                   *time.Ticker
		j                        *jenkins.JenkinsRef
		dcLogFollow              *exec.Cmd
		dcLogStdOut, dcLogStdErr *bytes.Buffer
		cleanup                  = func() {
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
		}
		setupJenkins = func(jenkinsTemplatePath string) {
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

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			g.By("waiting for builder service account")
			err = exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	)

	// these tests are isolated so that PR testing the the jenkins-client-plugin can execute the extended
	// tests with a ginkgo focus that runs only the tests within this ginkgo context
	g.Context("jenkins-client-plugin tests", func() {

		g.It("using the ephemeral template", func() {
			defer cleanup()
			setupJenkins(jenkinsEphemeralTemplatePath)

			g.By("Pipeline using jenkins-client-plugin")

			g.By("should build and complete successfully", func() {
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("bc", "sample-pipeline-openshift-client-plugin").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("dc", "jenkins-second-deployment").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "ruby").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("is", "ruby").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("is", "ruby-22-centos7").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("all", "-l", "template=mongodb-ephemeral-template").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("template", "mongodb-ephemeral").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("secret", "mongodb").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("should handle multi-namespace templates", func() {
				g.By("create additional projects")
				namespace := oc.Namespace()
				namespace2 := oc.Namespace() + "-2"
				namespace3 := oc.Namespace() + "-3"

				err := oc.Run("new-project").Args(namespace2).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("new-project").Args(namespace3).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				// no calls to delete these two projects here; leads to timing
				// issues with the framework deleting all namespaces

				g.By("set up policy for jenkins jobs in " + namespace2)
				err = oc.Run("policy").Args("add-role-to-user", "edit", "system:serviceaccount:"+namespace+":jenkins", "-n", namespace2).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By("set up policy for jenkins jobs in " + namespace3)
				err = oc.Run("policy").Args("add-role-to-user", "edit", "system:serviceaccount:"+namespace+":jenkins", "-n", namespace3).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				// instantiate the bc
				g.By("instantiate the jenkins pipeline strategy build config that leverages openshift client plugin with multiple namespaces")
				err = oc.Run("new-app").Args("-f", multiNamespaceClientPluginPipelinePath, "-p", "NAMESPACE="+namespace, "-p", "NAMESPACE2="+namespace2, "-p", "NAMESPACE3="+namespace3).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				// run the build
				g.By("starting the pipeline build and waiting for it to complete")
				br, err := exutil.StartBuildAndWait(oc, "multi-namespace-pipeline")
				debugAnyJenkinsFailure(br, oc.Namespace()+"-multi-namespace-pipeline", oc, true)
				if err != nil || !br.BuildSuccess {
					exutil.DumpBuilds(oc)
				}
				br.AssertSuccess()

				g.By("get build console logs and see if succeeded")
				_, err = j.GetJobConsoleLogsAndMatchViaBuildResult(br, "Finished: SUCCESS")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("confirm there are objects in second and third namespaces")
				defer oc.SetNamespace(namespace)
				oc.SetNamespace(namespace2)
				output, err := oc.Run("get").Args("all").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.ContainSubstring("deploymentconfig.apps.openshift.io/mongodb"))
				oc.SetNamespace(namespace3)
				output, err = oc.Run("get").Args("all").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.ContainSubstring("service/mongodb"))

				g.By("clean up openshift resources for next potential run")
				oc.SetNamespace(namespace)
				err = oc.Run("delete").Args("bc", "multi-namespace-pipeline").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("all", "-l", "template=mongodb-ephemeral-template").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("template", "mongodb-ephemeral").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})

	g.Context("Sync plugin tests", func() {

		g.It("using the persistent template", func() {
			defer cleanup()
			setupJenkins(jenkinsPersistentTemplatePath)
			// additionally ensure that the build works in a memory constrained
			// environment
			_, err := oc.AdminKubeClient().Core().LimitRanges(oc.Namespace()).Create(&v1.LimitRange{
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
			defer oc.AdminKubeClient().Core().LimitRanges(oc.Namespace()).Delete("limitrange", &metav1.DeleteOptions{})

			g.By("delete jenkins job runs when jenkins re-establishes communications")
			g.By("should delete job runs when the associated build is deleted - jenkins unreachable")
			type buildInfo struct {
				number          int
				jenkinsBuildURI string
			}
			buildNameToBuildInfoMap := make(map[string]buildInfo)

			g.By(fmt.Sprintf("calling oc new-app -f %q", envVarsPipelinePath))
			err = oc.Run("new-app").Args("-f", envVarsPipelinePath).Execute()
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

			g.By("clean up openshift resources for next potential run")
			err = oc.Run("delete").Args("bc", "sample-pipeline-withenvs").Execute()
		})

		g.It("using the ephemeral template", func() {
			defer cleanup()
			setupJenkins(jenkinsEphemeralTemplatePath)

			g.By("Pipelines with maven slave")

			g.By("should build a maven project and complete successfully", func() {

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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("bc", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "openshift-jee-sample-docker").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("dc", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("is", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("svc", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("route", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("is", "wildfly").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("should build a gradle project and complete successfully", func() {
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("bc", "gradle").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Pipelines with declarative syntax")

			g.By("should build successfully", func() {
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("all", "-l", "app=nodejs-mongodb-example").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("secret", "nodejs-mongodb-example").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "nodejs-sample-pipeline").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("is", "nodejs-mongodb-example-staging").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Deleted pipeline strategy buildconfigs")

			g.By("should not be recreated by the sync plugin", func() {
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("all", "-l", "app=jenkins-pipeline-example").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Sync secret to credential")

			g.By("should map openshift secret to a jenkins credential as the secret is manipulated", func() {
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

				// no need to clean up, last operation above deleted the secret
			})

			//TODO - for these config map slave tests, as well and the imagestream/imagestreamtag
			// tests ... rather than actually running the pipelines, we could just inspect the config in jenkins
			// to make sure the k8s pod templates are there.
			// In general, while we want at least one verification somewhere in pipeline.go that the agent
			// images work, we should minimize the actually running of pipelines using them to only one
			// for each maven/nodejs
			g.By("Pipeline using config map slave")

			g.By("should build and complete successfully", func() {
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("configmap", "jenkins-slave").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Pipeline using imagestream slave")

			g.By("should build and complete successfully", func() {
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("is", "jenkins-slave").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Pipeline using imagestreamtag slave")

			g.By("should build and complete successfully", func() {
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("is", "slave-jenkins").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = oc.Run("delete").Args("bc", "openshift-jee-sample").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Pipeline with env vars")

			g.By("should build and complete successfully", func() {
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("bc", "sample-pipeline-withenvs").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("Pipeline with env vars and git repo source")

			g.By("should propagate env vars to bc", func() {
				g.By(fmt.Sprintf("creating git repo %v", envVarsPipelineGitRepoBuildConfig))
				repo, err := exutil.NewGitRepo(envVarsPipelineGitRepoBuildConfig)
				defer repo.Remove()
				o.Expect(err).NotTo(o.HaveOccurred())
				jf := `node() {\necho "FOO1 is ${env.FOO1}"\necho"FOO2is${env.FOO2}"\necho"FOO3is${env.FOO3}"\necho"FOO4is${env.FOO4}"}`
				err = repo.AddAndCommit("Jenkinsfile", jf)
				o.Expect(err).NotTo(o.HaveOccurred())

				// instantiate the bc
				g.By(fmt.Sprintf("calling oc new-app %q --strategy=pipeline --build-env=FOO1=BAR1", repo.RepoPath))
				err = oc.Run("new-app").Args(repo.RepoPath, "--strategy=pipeline", "--build-env=FOO1=BAR1").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				bc, err := oc.BuildClient().Build().BuildConfigs(oc.Namespace()).Get(envVarsPipelineGitRepoBuildConfig, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				envs := bc.Spec.Strategy.JenkinsPipelineStrategy.Env
				o.Expect(len(envs)).To(o.Equal(1))
				o.Expect(envs[0].Name).To(o.Equal("FOO1"))
				o.Expect(envs[0].Value).To(o.Equal("BAR1"))
			})

			g.By("Blue-green pipeline")

			g.By("Blue-green pipeline should build and complete successfully", func() {
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
							case exutil.CheckBuildFailed(build):
								errs <- nil
								return
							case exutil.CheckBuildSuccess(build):
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("all", "-l", "app=bluegreen-pipeline").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})

			g.By("delete jenkins job runs when the associated build is deleted")

			g.By("should delete a job run when the associated build is deleted", func() {
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

				g.By("clean up openshift resources for next potential run")
				err = oc.Run("delete").Args("bc", "sample-pipeline-withenvs").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})

	})
})
