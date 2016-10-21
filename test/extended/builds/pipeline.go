package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Slow] openshift pipeline build", func() {
	defer g.GinkgoRecover()
	var (
		pipelineTemplatePath = exutil.FixturePath("testdata", "test-pipeline.json")
		jenkinsTemplatePath  = exutil.FixturePath("..", "..", "examples", "jenkins", "jenkins-ephemeral-template.json")
		oc                   = exutil.NewCLI("jenkins-pipeline", exutil.KubeConfigPath())
	)
	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})
	g.Context("Manual deploy the jenkins and trigger a jenkins pipeline build", func() {
		g.It("JenkinsPipeline build should succeed when manual deploy the jenkins service", func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc new-app -f %q", jenkinsTemplatePath))
			err := oc.Run("new-app").Args("-f", jenkinsTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			//wait for the jenkins deployment complete
			g.By("waiting the jenkins service deployed")
			err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), "jenkins", oc)
			if err != nil {
				exutil.DumpDeploymentLogs("jenkins", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())
			// create the pipeline build example
			g.By(fmt.Sprintf("calling oc new-app -f %q", pipelineTemplatePath))
			err = oc.Run("new-app").Args("-f", pipelineTemplatePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("starting a pipeline build")
			br, _ := exutil.StartBuildAndWait(oc, "sample-pipeline")
			if !br.BuildSuccess {
				exutil.DumpDeploymentLogs("jenkins", oc)
			}
			br.AssertSuccess()
		})
	})

})
