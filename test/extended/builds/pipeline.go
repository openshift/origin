package builds

import (
	"fmt"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/jenkins"
)

var _ = g.Describe("[builds][Slow] openshift pipeline build", func() {
	defer g.GinkgoRecover()
	var (
		jenkinsTemplatePath       = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		mavenSlavePipelinePath    = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "maven-pipeline.yaml")
		orchestrationPipelinePath = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "mapsapp-pipeline.yaml")
		blueGreenPipelinePath     = exutil.FixturePath("..", "..", "examples", "jenkins", "pipeline", "bluegreen-pipeline.yaml")

		oc = exutil.NewCLI("jenkins-pipeline", exutil.KubeConfigPath())
	)
	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})
	g.Context("Pipeline with maven slave", func() {
		g.It("Should build and complete successfully", func() {
			// Deploy Jenkins
			g.By(fmt.Sprintf("calling oc new-app -f %q", jenkinsTemplatePath))
			err := oc.Run("new-app").Args("-f", jenkinsTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// instantiate the template
			g.By(fmt.Sprintf("calling oc new-app -f %q", mavenSlavePipelinePath))
			err = oc.Run("new-app").Args("-f", mavenSlavePipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build
			g.By("starting the pipeline build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "openshift-jee-sample")
			br.AssertSuccess()

			// wait for the service to be running
			g.By("expecting the openshift-jee-sample service to be deployed and running")
			_, err = exutil.GetEndpointAddress(oc, "openshift-jee-sample")
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Context("Orchestration pipeline", func() {
		g.It("Should build and complete successfully", func() {
			// Deploy Jenkins
			g.By(fmt.Sprintf("calling oc new-app -f %q", jenkinsTemplatePath))
			err := oc.Run("new-app").Args("-f", jenkinsTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// instantiate the template
			g.By(fmt.Sprintf("calling oc new-app -f %q", orchestrationPipelinePath))
			err = oc.Run("new-app").Args("-f", orchestrationPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// start the build
			g.By("starting the pipeline build and waiting for it to complete")
			br, _ := exutil.StartBuildAndWait(oc, "mapsapp-pipeline")
			br.AssertSuccess()

			// wait for the service to be running
			g.By("expecting the parksmap-web service to be deployed and running")
			_, err = exutil.GetEndpointAddress(oc, "parksmap")
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

	g.Context("Blue-green pipeline", func() {
		g.It("Should build and complete successfully", func() {
			// Deploy Jenkins without oauth
			g.By(fmt.Sprintf("calling oc new-app -f %q -p ENABLE_OAUTH=false", jenkinsTemplatePath))
			err := oc.Run("new-app").Args("-f", jenkinsTemplatePath, "-p", "ENABLE_OAUTH=false").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			j := jenkins.NewRef(oc)

			// instantiate the template
			g.By(fmt.Sprintf("calling oc new-app -f %q", blueGreenPipelinePath))
			err = oc.Run("new-app").Args("-f", blueGreenPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			wg := &sync.WaitGroup{}
			wg.Add(1)

			// start the build
			go func() {
				g.By("starting the bluegreen pipeline build and waiting for it to complete")
				br, _ := exutil.StartBuildAndWait(oc, "bluegreen-pipeline")
				br.AssertSuccess()

				g.By("verifying that the main route has been switched to green")
				value, err := oc.Run("get").Args("route", "nodejs-mongodb-example", "-o", "jsonpath={ .spec.to.name }").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				activeRoute := strings.TrimSpace(value)
				g.By("verifying that the active route is 'nodejs-mongodb-example-green'")
				o.Expect(activeRoute).To(o.Equal("nodejs-mongodb-example-green"))
				wg.Done()
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
			g.By("Waiting for the build to complete successfully")
			wg.Wait()

			wg = &sync.WaitGroup{}
			wg.Add(1)

			// start the build again
			go func() {
				g.By("starting the bluegreen pipeline build and waiting for it to complete")
				br, _ := exutil.StartBuildAndWait(oc, "bluegreen-pipeline")
				br.AssertSuccess()

				g.By("verifying that the main route has been switched to blue")
				value, err := oc.Run("get").Args("route", "nodejs-mongodb-example", "-o", "jsonpath={ .spec.to.name }").Output()
				o.Expect(err).NotTo(o.HaveOccurred())
				activeRoute := strings.TrimSpace(value)
				g.By("verifying that the active route is 'nodejs-mongodb-example-blue'")
				o.Expect(activeRoute).To(o.Equal("nodejs-mongodb-example-blue"))
				wg.Done()
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

			wg.Wait()
		})
	})
})
