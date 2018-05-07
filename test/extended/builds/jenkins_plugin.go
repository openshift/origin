package builds

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/jenkins"
)

const (
	localPluginSnapshotImageStream      = "jenkins-plugin-snapshot-test"
	localPluginSnapshotImage            = "openshift/" + localPluginSnapshotImageStream + ":latest"
	localLoginPluginSnapshotImageStream = "jenkins-login-plugin-snapshot-test"
	localLoginPluginSnapshotImage       = "openshift/" + localLoginPluginSnapshotImageStream + ":latest"
	originalLicenseText                 = "About OpenShift Pipeline Jenkins Plugin"
	originalPluginName                  = "openshift-pipeline"
	loginLicenseText                    = "About OpenShift Login Plugin"
	loginPluginName                     = "openshift-login"
)

// Loads a Jenkins related template using new-app.
func loadFixture(oc *exutil.CLI, filename string) {
	resourcePath := exutil.FixturePath("testdata", "jenkins-plugin", filename)
	err := oc.Run("new-app").Args(resourcePath).Execute()
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
}

func assertEnvVars(oc *exutil.CLI, buildPrefix string, varsToFind map[string]string) {

	buildList, err := oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	// Ensure that expected start-build environment variables were injected
	for _, build := range buildList.Items {
		e2e.Logf("Found build: %q", build.GetName())
		if strings.HasPrefix(build.GetName(), buildPrefix) {
			envs := []kapi.EnvVar{}
			if build.Spec.Strategy.DockerStrategy != nil && build.Spec.Strategy.DockerStrategy.Env != nil {
				envs = build.Spec.Strategy.DockerStrategy.Env
			} else if build.Spec.Strategy.SourceStrategy != nil && build.Spec.Strategy.SourceStrategy.Env != nil {
				envs = build.Spec.Strategy.SourceStrategy.Env
			} else {
				continue
			}

			for k, v := range varsToFind {
				found := false
				for _, env := range envs {
					e2e.Logf("Found %s=%s in build %s", env.Name, env.Value, build.GetName())
					if k == env.Name && v == env.Value {
						found = true
						break
					}
				}
				o.ExpectWithOffset(1, found).To(o.BeTrue())
			}
		}
	}
}

// Stands up a simple pod which can be used for exec commands
func initExecPod(oc *exutil.CLI) *kapiv1.Pod {
	// Create a running pod in which we can execute our commands
	oc.Run("run").Args("centos", "--image", "centos:7", "--command", "--", "sleep", "1800").Execute()

	var targetPod *kapiv1.Pod
	err := wait.Poll(10*time.Second, 10*time.Minute, func() (bool, error) {
		pods, err := oc.KubeClient().CoreV1().Pods(oc.Namespace()).List(metav1.ListOptions{})
		o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
		for _, p := range pods.Items {
			if strings.HasPrefix(p.Name, "centos") && !strings.Contains(p.Name, "deploy") && p.Status.Phase == "Running" {
				targetPod = &p
				return true, nil
			}
		}
		return false, nil
	})
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	return targetPod
}

type apiObjJob struct {
	jobName string
	create  bool
}

// Validate create/delete of objects
func validateCreateDelete(create bool, key, out string, err error) {
	e2e.Logf("\nOBJ: %s\n", out)
	if create {
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.Contains(out, key)).To(o.BeTrue())
	} else {
		o.Expect(err).To(o.HaveOccurred())
	}
}

var _ = g.Describe("[Feature:Builds][Slow] openshift pipeline plugin", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("jenkins-plugin", exutil.KubeConfigPath())
	var j *jenkins.JenkinsRef
	var dcLogFollow *exec.Cmd
	var dcLogStdOut, dcLogStdErr *bytes.Buffer
	var ticker *time.Ticker

	g.Context("", func() {
		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker.Stop()
			}

			e2e.Logf("Jenkins DC description follows. If there were issues, check to see if there were any restarts in the jenkins pod.")
			exutil.DumpApplicationPodLogs("jenkins", oc)

			if dcLogFollow != nil && dcLogStdOut != nil && dcLogStdErr != nil {
				e2e.Logf("Killing Jenkins DC log follow")
				dcLogFollow.Process.Kill()
				e2e.Logf("Jenkins server logs from test:\nstdout>\n%s\n\nstderr>\n%s\n\n", string(dcLogStdOut.Bytes()), string(dcLogStdErr.Bytes()))
				dcLogFollow = nil
			} else {
				e2e.Logf("Logs were not captured!\n%v\n%v\n%v\n", dcLogFollow, dcLogStdOut, dcLogStdErr)
			}

		})

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()

			g.By("kick off the build for the jenkins ephemeral and application templates")

			// Deploy Jenkins
			// NOTE, we use these tests for both a) nightly regression runs against the latest openshift jenkins image on docker hub, and
			// b) PR testing for changes to the various openshift jenkins plugins we support.  With scenario b), a docker image that extends
			// our jenkins image is built, where the proposed plugin change is injected, overwritting the current released version of the plugin.
			// Our test/PR jobs on ci.openshift create those images, as well as set env vars this test suite looks for.  When both the env var
			// and test image is present, a new image stream is created using the test image, and our jenkins template is instantiated with
			// an override to use that images stream and test image
			var licensePrefix, pluginName string
			// with the startup costs now of jenkins 2.89 or greater and trying to incur those during startup, need more memory
			// to avoid deployment timeouts
			newAppArgs := []string{"-f", exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json"), "-p", "MEMORY_LIMIT=2Gi", "-p", "DISABLE_ADMINISTRATIVE_MONITORS=true"}

			useSnapshotImage := false
			origPluginNewAppArgs, useOrigPluginSnapshotImage := jenkins.SetupSnapshotImage(jenkins.UseLocalPluginSnapshotEnvVarName, localPluginSnapshotImage, localPluginSnapshotImageStream, newAppArgs, oc)
			loginPluginNewAppArgs, useLoginPluginSnapshotImage := jenkins.SetupSnapshotImage(jenkins.UseLocalLoginPluginSnapshotEnvVarName, localLoginPluginSnapshotImage, localLoginPluginSnapshotImageStream, newAppArgs, oc)
			switch {
			case useOrigPluginSnapshotImage && useLoginPluginSnapshotImage:
				fmt.Fprintf(g.GinkgoWriter,
					"\nBOTH %s and %s for PR TESTING ARE SET.  WILL NOT CHOOSE BETWEEN THE TWO SO TESTING CURRENT PLUGIN VERSIONS IN LATEST OPENSHIFT JENKINS IMAGE ON DOCKER HUB.\n",
					jenkins.UseLocalPluginSnapshotEnvVarName, jenkins.UseLocalLoginPluginSnapshotEnvVarName)
			case useOrigPluginSnapshotImage:
				fmt.Fprintf(g.GinkgoWriter, "\nTHE UPCOMING TESTS WILL LEVERAGE AN IMAGE THAT EXTENDS THE LATEST OPENSHIFT JENKINS IMAGE AND OVERRIDES THE OPENSHIFT PIPELINE PLUGIN WITH A NEW VERSION BUILT FROM PROPOSED CHANGES TO THAT PLUGIN.\n")
				licensePrefix = originalLicenseText
				pluginName = originalPluginName
				useSnapshotImage = true
				newAppArgs = origPluginNewAppArgs
			case useLoginPluginSnapshotImage:
				fmt.Fprintf(g.GinkgoWriter, "\nTHE UPCOMING TESTS WILL LEVERAGE AN IMAGE THAT EXTENDS THE LATEST OPENSHIFT JENKINS IMAGE AND OVERRIDES THE OPENSHIFT LOGIN PLUGIN WITH A NEW VERSION BUILT FROM PROPOSED CHANGES TO THAT PLUGIN.\n")
				licensePrefix = loginLicenseText
				pluginName = loginPluginName
				useSnapshotImage = true
				newAppArgs = loginPluginNewAppArgs
			default:
				fmt.Fprintf(g.GinkgoWriter, "\nNO PR TEST ENV VARS SET SO TESTING CURRENT PLUGIN VERSIONS IN LATEST OPENSHIFT JENKINS IMAGE ON DOCKER HUB.\n")
			}

			err := oc.Run("new-app").Args(newAppArgs...).Execute()
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

			if os.Getenv(jenkins.EnableJenkinsMemoryStats) != "" {
				ticker = jenkins.StartJenkinsMemoryTracking(oc, oc.Namespace())
			}

			// Start capturing logs from this deployment config.
			// This command will terminate if the Jenkins instance crashes. This
			// ensures that even if the Jenkins DC restarts, we should capture
			// logs from the crash.
			dcLogFollow, dcLogStdOut, dcLogStdErr, err = oc.Run("logs").Args("-f", "dc/jenkins").Background()
			o.Expect(err).NotTo(o.HaveOccurred())

			// Populate shared Jenkins namespace with artifacts that can be used by all tests
			loadFixture(oc, "shared-resources-template.json")

			// Allow resources to settle. ImageStream tags seem unavailable without this wait.
			time.Sleep(10 * time.Second)

		})

		g.Context("jenkins-plugin test context  ", func() {

			g.It("jenkins-plugin test trigger build including clone", func() {

				jobName := "test-build-job"
				data := j.ReadJenkinsJob("build-job.xml", oc.Namespace())
				j.CreateItem(jobName, data)
				jmon := j.StartJob(jobName)
				err := jmon.Await(10 * time.Minute)
				if err != nil {
					logs, _ := j.GetLastJobConsoleLogs(jobName)
					e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				// the build and deployment is by far the most time consuming portion of the test jenkins job;
				// we leverage some of the openshift utilities for waiting for the deployment before we poll
				// jenkins for the successful job completion
				g.By("waiting for frontend, frontend-prod deployments as signs that the build has finished")
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), "frontend", 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), "frontend-prod", 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("get build console logs and see if succeeded")
				logs, err := j.WaitForContent("Finished: SUCCESS", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", jobName)
				e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("get build and confirm trigger by field is correct")
				out, err := oc.Run("get").Args("builds/frontend-1", "-o", "jsonpath='{.spec.triggeredBy[0].message}'").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(strings.Contains(out, "Jenkins job")).To(o.BeTrue())

				jobName = "test-build-clone-job"
				data = j.ReadJenkinsJob("build-job-clone.xml", oc.Namespace())
				j.CreateItem(jobName, data)
				jmon = j.StartJob(jobName)
				jmon.Await(10 * time.Minute)
				g.By("get clone build console logs and see if succeeded")
				logs, err = j.WaitForContent("Finished: SUCCESS", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", jobName)
				e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("get build and confirm trigger by field is correct")
				out2, err := oc.Run("get").Args("builds/frontend-2", "-o", "jsonpath='{.spec.triggeredBy[0].message}'").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(strings.Contains(out2, "Jenkins job")).To(o.BeTrue())

				g.By("ensure trigger by fields for the two builds are different")
				o.Expect(strings.Compare(out, out2) == 0).NotTo(o.BeTrue())

			})

			g.It("jenkins-plugin test trigger build with slave", func() {

				jobName := "test-build-job-slave"
				data := j.ReadJenkinsJob("build-job-slave.xml", oc.Namespace())
				j.CreateItem(jobName, data)
				jmon := j.StartJob(jobName)
				err := jmon.Await(10 * time.Minute)
				if err != nil {
					logs, _ := j.GetLastJobConsoleLogs(jobName)
					e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				// the build and deployment is by far the most time consuming portion of the test jenkins job;
				// we leverage some of the openshift utilities for waiting for the deployment before we poll
				// jenkins for the successful job completion
				g.By("waiting for frontend, frontend-prod deployments as signs that the build has finished")
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), "frontend", 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), "frontend-prod", 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("get build console logs and see if succeeded")
				logs, err := j.WaitForContent("Finished: SUCCESS", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", jobName)
				e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("get build console logs and confirm ran on slave")
				logs, err = j.WaitForContent("Building remotely on", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", jobName)
				e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				o.Expect(err).NotTo(o.HaveOccurred())

			})

			g.It("jenkins-plugin test create obj delete obj", func() {

				jobsToCreate := map[string]string{"test-create-obj": "create-job.xml", "test-delete-obj": "delete-job.xml", "test-delete-obj-labels": "delete-job-labels.xml", "test-delete-obj-keys": "delete-job-keys.xml"}
				for jobName, jobConfig := range jobsToCreate {
					data := j.ReadJenkinsJob(jobConfig, oc.Namespace())
					j.CreateItem(jobName, data)
				}

				jobsToRun := []apiObjJob{{"test-create-obj", true}, {"test-delete-obj", false}, {"test-create-obj", true}, {"test-delete-obj-labels", false}, {"test-create-obj", true}, {"test-delete-obj-keys", false}}
				for _, job := range jobsToRun {
					jmon := j.StartJob(job.jobName)
					err := jmon.Await(10 * time.Minute)
					if err != nil {
						logs, _ := j.GetLastJobConsoleLogs(job.jobName)
						e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
					}
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("get build console logs and see if succeeded")
					logs, err := j.WaitForContent("Finished: SUCCESS", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", job.jobName)
					e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
					o.Expect(err).NotTo(o.HaveOccurred())
					out, err := oc.Run("get").Args("bc", "forcepull-bldr").Output()
					validateCreateDelete(job.create, "forcepull-bldr", out, err)
					out, err = oc.Run("get").Args("is", "forcepull-extended-test-builder").Output()
					validateCreateDelete(job.create, "forcepull-extended-test-builder", out, err)
				}

			})

			g.It("jenkins-plugin test trigger build with envs", func() {

				jobName := "test-build-with-env-job"
				data := j.ReadJenkinsJob("build-with-env-job.xml", oc.Namespace())
				j.CreateItem(jobName, data)
				jmon := j.StartJob(jobName)
				err := jmon.Await(10 * time.Minute)
				if err != nil {
					logs, _ := j.GetLastJobConsoleLogs(jobName)
					e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				logs, err := j.GetLastJobConsoleLogs(jobName)
				e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				o.Expect(err).NotTo(o.HaveOccurred())

				// the build and deployment is by far the most time consuming portion of the test jenkins job;
				// we leverage some of the openshift utilities for waiting for the deployment before we poll
				// jenkins for the successful job completion
				g.By("waiting for frontend deployments as signs that the build has finished")
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), "frontend", 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("get build console logs and see if succeeded")
				_, err = j.WaitForContent("Finished: SUCCESS", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", jobName)

				assertEnvVars(oc, "frontend-", map[string]string{
					"a": "b",
					"C": "D",
					"e": "",
				})

			})

			g.It("jenkins-plugin test trigger build DSL", func() {

				buildsBefore, err := oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				data, err := j.BuildDSLJob(oc.Namespace(),
					"node{",
					"openshiftBuild( namespace:'PROJECT_NAME', bldCfg: 'frontend', env: [ [ name : 'a', value : 'b' ], [ name : 'C', value : 'D' ], [ name : 'e', value : '' ] ] )",
					"}",
				)

				jobName := "test-build-dsl-job"
				j.CreateItem(jobName, data)
				monitor := j.StartJob(jobName)
				err = monitor.Await(10 * time.Minute)
				if err != nil {
					logs, _ := j.GetLastJobConsoleLogs(jobName)
					e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				err = wait.Poll(10*time.Second, 10*time.Minute, func() (bool, error) {
					buildsAfter, err := oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					return (len(buildsAfter.Items) != len(buildsBefore.Items)), nil
				})

				buildsAfter, err := oc.BuildClient().Build().Builds(oc.Namespace()).List(metav1.ListOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(buildsAfter.Items)).To(o.Equal(len(buildsBefore.Items) + 1))

				log, err := j.GetLastJobConsoleLogs(jobName)
				e2e.Logf("Job logs>>\n%s\n\n", log)

				assertEnvVars(oc, "frontend-", map[string]string{
					"a": "b",
					"C": "D",
					"e": "",
				})

			})

			g.It("jenkins-plugin test exec DSL", func() {

				targetPod := initExecPod(oc)
				targetContainer := targetPod.Spec.Containers[0]

				data, err := j.BuildDSLJob(oc.Namespace(),
					"node{",
					fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', command: [ 'echo', 'hello', 'world', '1' ] )", targetPod.Name),
					fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', command: 'echo', arguments : [ 'hello', 'world', '2' ] )", targetPod.Name),
					fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', command: 'echo', arguments : [ [ value: 'hello' ], [ value : 'world' ], [ value : '3' ] ] )", targetPod.Name),
					fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', container: '%s', command: [ 'echo', 'hello', 'world', '4' ] )", targetPod.Name, targetContainer.Name),
					fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', container: '%s', command: 'echo', arguments : [ 'hello', 'world', '5' ] )", targetPod.Name, targetContainer.Name),
					fmt.Sprintf("openshiftExec( namespace:'PROJECT_NAME', pod: '%s', container: '%s', command: 'echo', arguments : [ [ value: 'hello' ], [ value : 'world' ], [ value : '6' ] ] )", targetPod.Name, targetContainer.Name),
					"}",
				)

				jobName := "test-exec-dsl-job"
				j.CreateItem(jobName, data)
				monitor := j.StartJob(jobName)
				err = monitor.Await(10 * time.Minute)
				if err != nil {
					logs, _ := j.GetLastJobConsoleLogs(jobName)
					e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				log, err := j.GetLastJobConsoleLogs(jobName)
				e2e.Logf("Job logs>>\n%s\n\n", log)

				o.Expect(strings.Contains(log, "hello world 1")).To(o.BeTrue())
				o.Expect(strings.Contains(log, "hello world 2")).To(o.BeTrue())
				o.Expect(strings.Contains(log, "hello world 3")).To(o.BeTrue())
				o.Expect(strings.Contains(log, "hello world 4")).To(o.BeTrue())
				o.Expect(strings.Contains(log, "hello world 5")).To(o.BeTrue())
				o.Expect(strings.Contains(log, "hello world 6")).To(o.BeTrue())
			})

			g.It("jenkins-plugin test exec freestyle", func() {

				targetPod := initExecPod(oc)
				targetContainer := targetPod.Spec.Containers[0]

				jobName := "test-build-with-env-steps"
				data := j.ReadJenkinsJobUsingVars("build-with-exec-steps.xml", oc.Namespace(), map[string]string{
					"POD_NAME":       targetPod.Name,
					"CONTAINER_NAME": targetContainer.Name,
				})

				j.CreateItem(jobName, data)
				jmon := j.StartJob(jobName)
				jmon.Await(2 * time.Minute)

				log, err := j.GetLastJobConsoleLogs(jobName)
				e2e.Logf("\n\nJenkins logs>\n%s\n\n", log)
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(strings.Contains(log, "hello world 1")).To(o.BeTrue())

				// Now run without specifying container
				jobName = "test-build-with-env-steps-no-container"
				data = j.ReadJenkinsJobUsingVars("build-with-exec-steps.xml", oc.Namespace(), map[string]string{
					"POD_NAME":       targetPod.Name,
					"CONTAINER_NAME": "",
				})

				j.CreateItem(jobName, data)
				jmon = j.StartJob(jobName)
				jmon.Await(2 * time.Minute)

				log, err = j.GetLastJobConsoleLogs(jobName)
				e2e.Logf("\n\nJenkins logs>\n%s\n\n", log)
				o.Expect(err).NotTo(o.HaveOccurred())

				o.Expect(strings.Contains(log, "hello world 1")).To(o.BeTrue())

			})

			g.It("jenkins-plugin test multitag", func() {

				loadFixture(oc, "multitag-template.json")
				err := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
					_, err := oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag3:orig", metav1.GetOptions{})
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				jobName := "test-multitag-job"
				data := j.ReadJenkinsJob("multitag-job.xml", oc.Namespace())
				j.CreateItem(jobName, data)
				monitor := j.StartJob(jobName)
				err = monitor.Await(10 * time.Minute)
				if err != nil {
					logs, _ := j.GetLastJobConsoleLogs(jobName)
					e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				log, err := j.GetLastJobConsoleLogs(jobName)
				e2e.Logf("Job logs>>\n%s\n\n", log)

				// Assert stream tagging results
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag:prod", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// 1 to N mapping
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag:prod2", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag:prod3", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag:prod4", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// N to 1 mapping
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag:prod5", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag2:prod5", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag3:prod5", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// N to N mapping
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag:prod6", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag2:prod7", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag3:prod8", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				// N to N mapping with creation
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag4:prod9", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag5:prod10", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				_, err = oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag6:prod11", metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

			})

			g.It("jenkins-plugin test multitag DSL", func() {

				testNamespace := oc.Namespace()

				loadFixture(oc, "multitag-template.json")
				err := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
					_, err := oc.ImageClient().Image().ImageStreamTags(oc.Namespace()).Get("multitag3:orig", metav1.GetOptions{})
					if err != nil {
						return false, nil
					}
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				anotherNamespace := oc.Namespace() + "-multitag-target"
				oc.Run("new-project").Args(anotherNamespace).Execute()
				// no calls to delete these two projects here; leads to timing
				// issues with the framework deleting all namespaces

				time.Sleep(10 * time.Second) // Give project time to initialize policies.

				// Allow jenkins service account to edit the new namespace
				oc.SetNamespace(anotherNamespace)
				err = oc.Run("policy").Args("add-role-to-user", "edit", "system:serviceaccount:"+j.Namespace()+":jenkins").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				oc.SetNamespace(testNamespace)

				e2e.Logf("Using testNamespace: %q and currentNamespace: %q", testNamespace, oc.Namespace())

				data, err := j.BuildDSLJob(oc.Namespace(),
					"node{",
					"openshiftTag( namespace:'PROJECT_NAME', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag', destTag: 'prod' )",
					"openshiftTag( namespace:'PROJECT_NAME', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag2', destTag: 'prod1, prod2, prod3' )",
					"openshiftTag( namespace:'PROJECT_NAME', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag2,multitag7', destTag: 'prod4' )",
					"openshiftTag( namespace:'PROJECT_NAME', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag5,multitag6', destTag: 'prod5, prod6' )",
					fmt.Sprintf("openshiftTag( namespace:'PROJECT_NAME', destinationNamespace: '%s', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag', destTag: 'prod' )", anotherNamespace),
					fmt.Sprintf("openshiftTag( namespace:'PROJECT_NAME', destinationNamespace: '%s', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag2', destTag: 'prod1, prod2, prod3' )", anotherNamespace),
					fmt.Sprintf("openshiftTag( namespace:'PROJECT_NAME', destinationNamespace: '%s', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag2,multitag7', destTag: 'prod4' )", anotherNamespace),
					fmt.Sprintf("openshiftTag( namespace:'PROJECT_NAME', destinationNamespace: '%s', srcStream: 'multitag', srcTag: 'orig', destStream: 'multitag5,multitag6', destTag: 'prod5, prod6' )", anotherNamespace),
					"}",
				)

				jobName := "test-multitag-dsl-job"
				j.CreateItem(jobName, data)
				monitor := j.StartJob(jobName)
				err = monitor.Await(10 * time.Minute)
				if err != nil {
					logs, _ := j.GetLastJobConsoleLogs(jobName)
					e2e.Logf("\n\nJenkins logs>\n%s\n\n", logs)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				time.Sleep(10 * time.Second)

				log, err := j.GetLastJobConsoleLogs(jobName)
				o.Expect(err).NotTo(o.HaveOccurred())
				e2e.Logf("Job logs>>\n%s\n\n", log)

				// Assert stream tagging results
				for _, namespace := range []string{oc.Namespace(), anotherNamespace} {
					g.By("Checking tags in namespace: " + namespace)
					_, err = oc.ImageClient().Image().ImageStreamTags(namespace).Get("multitag:prod", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					_, err = oc.ImageClient().Image().ImageStreamTags(namespace).Get("multitag2:prod1", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					_, err = oc.ImageClient().Image().ImageStreamTags(namespace).Get("multitag2:prod2", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					_, err = oc.ImageClient().Image().ImageStreamTags(namespace).Get("multitag2:prod3", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					_, err = oc.ImageClient().Image().ImageStreamTags(namespace).Get("multitag2:prod4", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					_, err = oc.ImageClient().Image().ImageStreamTags(namespace).Get("multitag5:prod5", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					_, err = oc.ImageClient().Image().ImageStreamTags(namespace).Get("multitag6:prod6", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					_, err = oc.ImageClient().Image().ImageStreamTags(namespace).Get("multitag7:prod4", metav1.GetOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
				}

			})

			testImageStreamSCM := func(jobXMLFile string) {
				jobName := "test-imagestream-scm"
				g.By("creating a jenkins job with an imagestream SCM")
				data := j.ReadJenkinsJob(jobXMLFile, oc.Namespace())
				j.CreateItem(jobName, data)

				// Because polling is enabled, a job should start automatically and fail
				// Wait for it to run and fail
				tree := url.QueryEscape("jobs[name,color]")
				xpath := url.QueryEscape("//job/name[text()='test-imagestream-scm']/../color")
				jobStatusURI := "api/xml?tree=%s&xpath=%s"
				g.By("waiting for initial job to complete")
				wait.Poll(10*time.Second, 10*time.Minute, func() (bool, error) {
					result, status, err := j.GetResource(jobStatusURI, tree, xpath)
					o.Expect(err).NotTo(o.HaveOccurred())
					if status == 200 && strings.Contains(result, "red") {
						return true, nil
					}
					return false, nil
				})

				// Create a new imagestream tag and expect a job to be kicked off
				// that will create a new tag in the current namespace
				g.By("creating an imagestream tag in the current project")
				oc.Run("tag").Args("openshift/jenkins:latest", fmt.Sprintf("%s/testimage:v1", oc.Namespace())).Execute()

				// Wait after the image has been tagged for the Jenkins job to run
				// and create the new imagestream/tag
				g.By("verifying that the job ran by looking for the resulting imagestream tag")
				err := exutil.TimedWaitForAnImageStreamTag(oc, oc.Namespace(), "localjenkins", "develop", 10*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.It("jenkins-plugin test imagestream SCM", func() {
				testImageStreamSCM("imagestream-scm-job.xml")
			})

			g.It("jenkins-plugin test imagestream SCM DSL", func() {
				testImageStreamSCM("imagestream-scm-dsl-job.xml")
			})

			g.It("jenkins-plugin test connection test", func() {

				jobName := "test-build-job"
				data := j.ReadJenkinsJob("build-job.xml", oc.Namespace())
				j.CreateItem(jobName, data)

				g.By("trigger test connection logic, check for success")
				testConnectionBody := bytes.NewBufferString("apiURL=&authToken=")
				result, code, err := j.Post(testConnectionBody, "job/test-build-job/descriptorByName/com.openshift.jenkins.plugins.pipeline.OpenShiftBuilder/testConnection", "application/x-www-form-urlencoded")
				if code != 200 {
					err = fmt.Errorf("Expected return code of 200")
				}
				if matched, _ := regexp.MatchString(".*Connection successful.*", result); !matched {
					err = fmt.Errorf("Expecting 'Connection successful', Got: %s", result)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("trigger test connection logic, check for failure")
				testConnectionBody = bytes.NewBufferString("apiURL=https%3A%2F%2F1.2.3.4&authToken=")
				result, code, err = j.Post(testConnectionBody, "job/test-build-job/descriptorByName/com.openshift.jenkins.plugins.pipeline.OpenShiftBuilder/testConnection", "application/x-www-form-urlencoded")
				if code != 200 {
					err = fmt.Errorf("Expected return code of 200")
				}
				if matched, _ := regexp.MatchString(".*Connection unsuccessful.*", result); !matched {
					err = fmt.Errorf("Expecting 'Connection unsuccessful', Got: %s", result)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

			})
		})
	})
})
