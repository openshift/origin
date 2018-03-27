package image_ecosystem

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[image_ecosystem][mysql][Slow] openshift mysql image", func() {
	defer g.GinkgoRecover()
	var (
		templatePath = exutil.FixturePath("..", "..", "examples", "db-templates", "mysql-ephemeral-template.json")
		oc           = exutil.NewCLI("mysql-create", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Creating from a template", func() {
			g.It(fmt.Sprintf("should instantiate the template"), func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

				g.By(fmt.Sprintf("calling oc process -f %q", templatePath))
				configFile, err := oc.Run("process").Args("-f", templatePath).OutputToFile("config.json")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("calling oc create -f %q", configFile))
				err = oc.Run("create").Args("-f", configFile).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
				// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), "mysql", 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the mysql service get endpoints")
				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), "mysql")
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
})
