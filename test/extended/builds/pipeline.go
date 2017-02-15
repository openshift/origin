package builds

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/jenkins"
	sutil "github.com/openshift/source-to-image/pkg/util"
)

func debugAnyJenkinsFailure(br *exutil.BuildResult, name string, oc *exutil.CLI, dumpMaster bool) {
	if !br.BuildSuccess {
		fmt.Fprintf(g.GinkgoWriter, "\n\n START debugAnyJenkinsFailure\n\n")
		j := jenkins.NewRef(oc)
		jobLog, err := j.GetLastJobConsoleLogs(name)
		if err == nil {
			fmt.Fprintf(g.GinkgoWriter, "\n %s job log:\n%s", name, jobLog)
		} else {
			fmt.Fprintf(g.GinkgoWriter, "\n error getting %s job log: %#v", name, err)
		}
		if dumpMaster {
			exutil.DumpDeploymentLogs("jenkins", oc)
		}
		fmt.Fprintf(g.GinkgoWriter, "\n\n END debugAnyJenkinsFailure\n\n")
	}
}

var _ = g.Describe("[builds][Slow] openshift pipeline build", func() {
	defer g.GinkgoRecover()
	var (
		jenkinsTemplatePath    = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		mavenSlavePipelinePath = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "maven-pipeline.yaml")
		//orchestrationPipelinePath = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "mapsapp-pipeline.yaml")
		blueGreenPipelinePath    = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "bluegreen-pipeline.yaml")
		clientPluginPipelinePath = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "openshift-client-plugin-pipeline.yaml")

		oc                       = exutil.NewCLI("jenkins-pipeline", exutil.KubeConfigPath())
		ticker                   *time.Ticker
		j                        *jenkins.JenkinsRef
		dcLogFollow              *exec.Cmd
		dcLogStdOut, dcLogStdErr *bytes.Buffer
		setupJenkins             = func() {
			// Deploy Jenkins
			g.By(fmt.Sprintf("calling oc new-app -f %q", jenkinsTemplatePath))
			err := oc.Run("new-app").Args("-f", jenkinsTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for jenkins deployment")
			err = exutil.WaitForADeploymentToComplete(oc.KubeClient().Core().ReplicationControllers(oc.Namespace()), "jenkins", oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			j = jenkins.NewRef(oc)

			g.By("wait for jenkins to come up")
			_, err = j.WaitForContent("", 200, 10*time.Minute, "")

			if err != nil {
				exutil.DumpDeploymentLogs("jenkins", oc)
			}

			o.Expect(err).NotTo(o.HaveOccurred())

			// Start capturing logs from this deployment config.
			// This command will terminate if the Jenkins instance crashes. This
			// ensures that even if the Jenkins DC restarts, we should capture
			// logs from the crash.
			dcLogFollow, dcLogStdOut, dcLogStdErr, err = oc.Run("logs").Args("-f", "dc/jenkins").Background()
			o.Expect(err).NotTo(o.HaveOccurred())

		}
	)

	g.AfterEach(func() {
		if os.Getenv(jenkins.DisableJenkinsGCSTats) == "" {
			g.By("stop jenkins gc tracking")
			ticker.Stop()
		}
	})

	g.BeforeEach(func() {
		setupJenkins()

		if os.Getenv(jenkins.DisableJenkinsGCSTats) == "" {
			g.By("start jenkins gc tracking")
			ticker = jenkins.StartJenkinsGCTracking(oc, oc.Namespace())
		}

		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Context("Pipeline with maven slave", func() {
		g.It("should build and complete successfully", func() {
			// instantiate the template
			g.By(fmt.Sprintf("calling oc new-app -f %q", mavenSlavePipelinePath))
			err := oc.Run("new-app").Args("-f", mavenSlavePipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build
			g.By("starting the pipeline build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "openshift-jee-sample")
			debugAnyJenkinsFailure(br, oc.Namespace()+"-openshift-jee-sample", oc, true)
			br.AssertSuccess()

			// wait for the service to be running
			g.By("expecting the openshift-jee-sample service to be deployed and running")
			_, err = exutil.GetEndpointAddress(oc, "openshift-jee-sample")
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Context("Pipeline using jenkins-client-plugin", func() {
		g.It("should build and complete successfully", func() {
			// instantiate the bc
			g.By("create the jenkins pipeline strategy build config that leverages openshift client plugin")
			err := oc.Run("create").Args("-f", clientPluginPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build
			g.By("starting the pipeline build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "sample-pipeline-openshift-client-plugin")
			debugAnyJenkinsFailure(br, oc.Namespace()+"-sample-pipeline-openshift-client-plugin", oc, true)
			br.AssertSuccess()

			g.By("get build console logs and see if succeeded")
			_, err = j.WaitForContent("Finished: SUCCESS", 200, 10*time.Minute, "job/%s-sample-pipeline-openshift-client-plugin/lastBuild/consoleText", oc.Namespace())
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	/*g.Context("Orchestration pipeline", func() {
		g.It("should build and complete successfully", func() {
			setupJenkins()

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
		g.It("Blue-green pipeline should build and complete successfully", func() {
			// instantiate the template
			g.By(fmt.Sprintf("calling oc new-app -f %q", blueGreenPipelinePath))
			err := oc.Run("new-app").Args("-f", blueGreenPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			wg := &sync.WaitGroup{}
			wg.Add(1)

			// start the build
			go func() {
				defer g.GinkgoRecover()
				defer wg.Done()
				g.By("starting the bluegreen pipeline build and waiting for it to complete")
				br, _ := exutil.StartBuildAndWait(oc, "bluegreen-pipeline")
				debugAnyJenkinsFailure(br, oc.Namespace()+"-bluegreen-pipeline", oc, true)
				br.AssertSuccess()

				g.By("verifying that the main route has been switched to green")
				value, err := oc.Run("get").Args("route", "nodejs-mongodb-example", "-o", "jsonpath={ .spec.to.name }").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				activeRoute := strings.TrimSpace(value)
				g.By("verifying that the active route is 'nodejs-mongodb-example-green'")
				o.Expect(activeRoute).To(o.Equal("nodejs-mongodb-example-green"))
			}()

			// wait for the green service to be available
			g.By("waiting for the nodejs-mongodb-example-green service to be available")
			_, err = exutil.GetEndpointAddress(oc, "nodejs-mongodb-example-green")
			o.Expect(err).NotTo(o.HaveOccurred())

			// approve the Jenkins pipeline
			g.By("Waiting for the approval prompt")
			jobName := oc.Namespace() + "-bluegreen-pipeline"
			_, err = j.WaitForContent("Approve?", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", jobName)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Approving the current build")
			_, _, err = j.Post(nil, fmt.Sprintf("job/%s/lastBuild/input/Approval/proceedEmpty", jobName), "")
			o.Expect(err).NotTo(o.HaveOccurred())

			// wait for first build completion and verification
			g.By("Waiting for the first build to complete successfully")
			err = sutil.TimeoutAfter(time.Minute*10, "first blue-green build timed out before WaitGroup quit blocking", func(timeoutTimer *time.Timer) error {
				g.By("start wg.Wait() for build completion and verification")
				wg.Wait()
				g.By("build completion and verification good to go")
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			wg = &sync.WaitGroup{}
			wg.Add(1)

			// start the build again
			go func() {
				defer g.GinkgoRecover()
				defer wg.Done()
				g.By("starting the bluegreen pipeline build and waiting for it to complete")
				br, _ := exutil.StartBuildAndWait(oc, "bluegreen-pipeline")
				debugAnyJenkinsFailure(br, oc.Namespace()+"-bluegreen-pipeline", oc, true)
				br.AssertSuccess()

				g.By("verifying that the main route has been switched to blue")
				value, err := oc.Run("get").Args("route", "nodejs-mongodb-example", "-o", "jsonpath={ .spec.to.name }").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				activeRoute := strings.TrimSpace(value)
				g.By("verifying that the active route is 'nodejs-mongodb-example-blue'")
				o.Expect(activeRoute).To(o.Equal("nodejs-mongodb-example-blue"))
			}()

			// wait for the blue service to be available
			g.By("waiting for the nodejs-mongodb-example-blue service to be available")
			_, err = exutil.GetEndpointAddress(oc, "nodejs-mongodb-example-blue")
			o.Expect(err).NotTo(o.HaveOccurred())

			// approve the Jenkins pipeline
			g.By("Waiting for the approval prompt")
			_, err = j.WaitForContent("Approve?", 200, 10*time.Minute, "job/%s/lastBuild/consoleText", jobName)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Approving the current build")
			_, _, err = j.Post(nil, fmt.Sprintf("job/%s/lastBuild/input/Approval/proceedEmpty", jobName), "")
			o.Expect(err).NotTo(o.HaveOccurred())

			// wait for second build completion and verification
			g.By("Waiting for the second build to complete successfully")
			err = sutil.TimeoutAfter(time.Minute*10, "second blue-green build timed out before WaitGroup quit blocking", func(timeoutTimer *time.Timer) error {
				g.By("start wg.Wait() for build completion and verification")
				wg.Wait()
				g.By("build completion and verification good to go")
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

})
