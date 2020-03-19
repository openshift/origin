package builds

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/framework/pod"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"

	buildv1 "github.com/openshift/api/build/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/jenkins"
)

var _ = g.Describe("[sig-devex][Feature:Jenkins][Slow] Jenkins repos e2e openshift using slow openshift pipeline build", func() {
	defer g.GinkgoRecover()

	var (
		jenkinsEphemeralTemplatePath  = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		jenkinsPersistentTemplatePath = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-persistent-template.json")
		nodejsDeclarativePipelinePath = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "nodejs-sample-pipeline.yaml")
		mavenSlavePipelinePath        = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "maven-pipeline.yaml")
		blueGreenPipelinePath         = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "bluegreen-pipeline.yaml")
		envVarsPipelinePath           = exutil.FixturePath("testdata", "samplepipeline-withenvs.yaml")
		oc                            = exutil.NewCLI("jenkins-pipeline", exutil.KubeConfigPath())
		ticker                        *time.Ticker
		j                             *jenkins.JenkinsRef
		pvs                           = []*corev1.PersistentVolume{}
		nfspod                        = &corev1.Pod{}

		cleanup = func(jenkinsTemplatePath string) {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpPersistentVolumeInfo(oc)
			}
			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}

			client := oc.AsAdmin().KubeFramework().ClientSet
			g.By("removing jenkins")
			exutil.RemoveDeploymentConfigs(oc, "jenkins")

			// per k8s e2e volume_util.go:VolumeTestCleanup, nuke any client pods
			// before nfs server to assist with umount issues; as such, need to clean
			// up prior to the AfterEach processing, to guaranteed deletion order
			if jenkinsTemplatePath == jenkinsPersistentTemplatePath {
				g.By("deleting PVCs")
				exutil.DeletePVCsForDeployment(client, oc, "jenkins")
				g.By("removing nfs pvs")
				for _, pv := range pvs {
					e2epv.DeletePersistentVolume(client, pv.Name)
				}
				g.By("removing nfs pod")
				pod.DeletePodWithWait(client, nfspod)
			}
		}
		setupJenkins = func(jenkinsTemplatePath string) {
			exutil.PreTestDump()
			// Deploy Jenkins
			// NOTE, we use these tests for both a) nightly regression runs against the latest openshift jenkins image on docker hub, and
			// b) PR testing for changes to the various openshift jenkins plugins we support.  With scenario b), a container image that extends
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
				g.By("PV/PVC dump before setup")
				exutil.DumpPersistentVolumeInfo(oc)

				jenkinsTemplateName = "jenkins-persistent"

				nfspod, pvs, err = exutil.SetupK8SNFSServerAndVolume(oc, 3)
				o.Expect(err).NotTo(o.HaveOccurred())

			}

			// our pipeline jobs, between jenkins and oc invocations, need more mem than the default
			newAppArgs := []string{"--template", fmt.Sprintf("%s/%s", oc.Namespace(), jenkinsTemplateName), "-p", "MEMORY_LIMIT=2Gi", "-p", "DISABLE_ADMINISTRATIVE_MONITORS=true"}
			newAppArgs = jenkins.OverridePodTemplateImages(newAppArgs)
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

			if jenkinsTemplatePath == jenkinsPersistentTemplatePath {
				g.By("PV/PVC dump after setup")
				exutil.DumpPersistentVolumeInfo(oc)
			}

			g.By("waiting for jenkins deployment")
			err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), "jenkins", 1, false, oc)
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
			_, _, _, err = oc.Run("logs").Args("-f", "dc/jenkins").Background()
			o.Expect(err).NotTo(o.HaveOccurred())

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}
		}

		debugAnyJenkinsFailure = func(br *exutil.BuildResult, name string, oc *exutil.CLI, dumpMaster bool) {
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
	)

	// these tests are isolated so that PR testing the jenkins-client-plugin can execute the extended
	// tests with a ginkgo focus that runs only the tests within this ginkgo context
	g.Context("Sync plugin tests", func() {

		g.It("using the persistent template", func() {
			defer cleanup(jenkinsPersistentTemplatePath)
			setupJenkins(jenkinsPersistentTemplatePath)
			// additionally ensure that the build works in a memory constrained
			// environment
			_, err := oc.AdminKubeClient().CoreV1().LimitRanges(oc.Namespace()).Create(context.Background(), &corev1.LimitRange{
				ObjectMeta: metav1.ObjectMeta{
					Name: "limitrange",
				},
				Spec: corev1.LimitRangeSpec{
					Limits: []corev1.LimitRangeItem{
						{
							Type: corev1.LimitTypeContainer,
							Default: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("512Mi"),
							},
						},
					},
				},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			defer oc.AdminKubeClient().CoreV1().LimitRanges(oc.Namespace()).Delete(context.Background(), "limitrange", metav1.DeleteOptions{})

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
				build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), fmt.Sprintf("sample-pipeline-withenvs-%d", i), metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				jenkinsBuildURI, err := url.Parse(build.Annotations[buildv1.BuildJenkinsBuildURIAnnotation])

				if err != nil {
					fmt.Fprintf(g.GinkgoWriter, "error parsing build uri: %s", err)
				}
				buildNameToBuildInfoMap[build.Name] = buildInfo{number: i, jenkinsBuildURI: jenkinsBuildURI.Path}
			}

			g.By("verifying that jobs exist for the 5 builds")
			for buildName, buildInfo := range buildNameToBuildInfoMap {
				_, status, err := j.GetResource(buildInfo.jenkinsBuildURI)
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), buildName, metav1.GetOptions{})
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
					err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Delete(context.Background(), buildName, metav1.DeleteOptions{})
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
					_, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), buildName, metav1.GetOptions{})
					o.Expect(err).To(o.HaveOccurred())
					o.Expect(status != http.StatusOK).To(o.BeTrue(), "Jenkins job run exists for %s but shouldn't.", buildName)
				} else {
					_, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), buildName, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(status == http.StatusOK).To(o.BeTrue(), "Jenkins job run does not exist for %s but should.", buildName)
				}
			}

			g.By("clean up openshift resources for next potential run")
			err = oc.Run("delete").Args("bc", "sample-pipeline-withenvs").Execute()
		})

		g.It("using the ephemeral template", func() {
			defer cleanup(jenkinsEphemeralTemplatePath)
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
				g.By("expecting the nodejs-mongodb-example service to be deployed and running")
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

			g.By("Pipeline with env vars and git repo source")

			g.By("should propagate env vars to bc", func() {
				g.By(fmt.Sprintf("creating git repo %v", envVarsPipelineGitRepoBuildConfig))
				repo, err := exutil.NewGitRepo(envVarsPipelineGitRepoBuildConfig)
				defer repo.Remove()
				if err != nil {
					files, dbgerr := ioutil.ReadDir("/tmp")
					if dbgerr != nil {
						e2e.Logf("problem diagnosing /tmp: %v", dbgerr)
					} else {
						for _, file := range files {
							e2e.Logf("found file %s under temp isdir %q mode %s", file.Name(), file.IsDir(), file.Mode().String())
						}
					}
				}
				o.Expect(err).NotTo(o.HaveOccurred())
				jf := `node() {\necho "FOO1 is ${env.FOO1}"\necho"FOO2is${env.FOO2}"\necho"FOO3is${env.FOO3}"\necho"FOO4is${env.FOO4}"}`
				err = repo.AddAndCommit("Jenkinsfile", jf)
				o.Expect(err).NotTo(o.HaveOccurred())

				// instantiate the bc
				g.By(fmt.Sprintf("calling oc new-app %q --strategy=pipeline --build-env=FOO1=BAR1", repo.RepoPath))
				err = oc.Run("new-app").Args(repo.RepoPath, "--strategy=pipeline", "--build-env=FOO1=BAR1").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				bc, err := oc.BuildClient().BuildV1().BuildConfigs(oc.Namespace()).Get(context.Background(), envVarsPipelineGitRepoBuildConfig, metav1.GetOptions{})
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
							build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.BuildName, metav1.GetOptions{})
							if err != nil {
								errs <- fmt.Errorf("error getting build: %s", err)
								return
							}
							jenkinsBuildURI = build.Annotations[buildv1.BuildJenkinsBuildURIAnnotation]
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
						_, _, err = j.Post("", jenkinsBuildURI+"/input/Approval/proceedEmpty", "")
						if err != nil {
							errs <- fmt.Errorf("error approving the current build: %s", err)
						}
					}()

					go func() {
						defer g.GinkgoRecover()

						for {
							build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), br.BuildName, metav1.GetOptions{})
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
					build, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), fmt.Sprintf("sample-pipeline-withenvs-%d", i), metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					jenkinsBuildURI, err := url.Parse(build.Annotations[buildv1.BuildJenkinsBuildURIAnnotation])
					if err != nil {
						fmt.Fprintf(g.GinkgoWriter, "error parsing build uri: %s", err)
					}
					buildNameToBuildInfoMap[build.Name] = buildInfo{number: i, jenkinsBuildURI: jenkinsBuildURI.Path}
				}

				g.By("verifying that jobs exist for the 5 builds")
				for buildName, buildInfo := range buildNameToBuildInfoMap {
					_, status, err := j.GetResource(buildInfo.jenkinsBuildURI)
					o.Expect(err).NotTo(o.HaveOccurred())
					_, err = oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), buildName, metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					o.Expect(status == http.StatusOK).To(o.BeTrue(), "Jenkins job run does not exist for %s but should.", buildName)
				}

				g.By("deleting the even numbered builds")
				for buildName, buildInfo := range buildNameToBuildInfoMap {
					if buildInfo.number%2 == 0 {
						fmt.Fprintf(g.GinkgoWriter, "Deleting build: %s", buildName)
						err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Delete(context.Background(), buildName, metav1.DeleteOptions{})
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
						_, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), buildName, metav1.GetOptions{})
						o.Expect(err).To(o.HaveOccurred())
						o.Expect(status != http.StatusOK).To(o.BeTrue(), "Jenkins job run exists for %s but shouldn't.", buildName)
					} else {
						_, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), buildName, metav1.GetOptions{})
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
