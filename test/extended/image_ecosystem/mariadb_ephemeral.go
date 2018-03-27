package image_ecosystem

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[image_ecosystem][mariadb][Slow] openshift mariadb image", func() {
	defer g.GinkgoRecover()
	var (
		templatePath = exutil.FixturePath("..", "..", "examples", "db-templates", "mariadb-ephemeral-template.json")
		oc           = exutil.NewCLI("mariadb-create", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.DumpDockerInfo()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpImageStreams(oc)
			}
		})

		g.Describe("Creating from a template", func() {
			g.It(fmt.Sprintf("should instantiate the template"), func() {
				exutil.CheckOpenShiftNamespaceImageStreams(oc)

				oc.SetOutputDir(exutil.TestContext.OutputDir)

				g.By(fmt.Sprintf("calling oc process -f %q", templatePath))
				configFile, err := oc.Run("process").Args("-f", templatePath).OutputToFile("config.json")
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By(fmt.Sprintf("calling oc create -f %q", configFile))
				err = oc.Run("create").Args("-f", configFile).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
				// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), "mariadb", 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("expecting the mariadb service get endpoints")
				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), "mariadb")
				o.Expect(err).NotTo(o.HaveOccurred())
			})
		})
	})
})
