package image_ecosystem

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[image_ecosystem][mysql][Slow] openshift mysql image", func() {
	defer g.GinkgoRecover()
	var (
		templatePath = exutil.FixturePath("..", "..", "examples", "db-templates", "mysql-ephemeral-template.json")
		oc           = exutil.NewCLI("mysql-create", exutil.KubeConfigPath())
	)
	g.Describe("Creating from a template", func() {
		g.It(fmt.Sprintf("should process and create the %q template", templatePath), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			g.By(fmt.Sprintf("calling oc process -f %q", templatePath))
			configFile, err := oc.Run("process").Args("-f", templatePath).OutputToFile("config.json")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("calling oc create -f %q", configFile))
			err = oc.Run("create").Args("-f", configFile).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
			// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
			err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), "mysql", oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("expecting the mysql service get endpoints")
			err = oc.KubeFramework().WaitForAnEndpoint("mysql")
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})

})
