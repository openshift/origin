package builds

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Conformance] jenkins autoprovision", func() {
	defer g.GinkgoRecover()
	var (
		envVarsPipelinePath = exutil.FixturePath("testdata", "samplepipeline-withenvs.yaml")
		oc                  = exutil.NewCLI("jenkins-pipeline", exutil.KubeConfigPath())
		jenkinsPodLabel     = exutil.ParseLabelsOrDie("deploymentconfig=jenkins")
	)

	g.Context("Pipeline build config", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()

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

		g.It("should autoprovision jenkins", func() {

			// instantiate the bc
			g.By(fmt.Sprintf("calling oc new-app -f %q", envVarsPipelinePath))
			err := oc.Run("new-app").Args("-f", envVarsPipelinePath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// confirm jenkins pod is started
			pods, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), jenkinsPodLabel, exutil.CheckPodNoOp, 1, 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(pods)).To(o.Equal(1))

		})

	})

})
