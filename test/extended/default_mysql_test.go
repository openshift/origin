package extended

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = Describe("default: MySQL ephemeral template", func() {
	defer GinkgoRecover()
	var (
		templatePath = filepath.Join("..", "..", "examples", "db-templates", "mysql-ephemeral-template.json")
		oc           = exutil.NewCLI("mysql-create", kubeConfigPath())
	)
	Describe("Creating from a template", func() {
		It(fmt.Sprintf("should process and create the %q template", templatePath), func() {
			oc.SetOutputDir(testContext.OutputDir)

			By(fmt.Sprintf("calling oc process -f %q", templatePath))
			configFile, err := oc.Run("process").Args("-f", templatePath).OutputToFile("config.json")
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("calling oc create -f %q", configFile))
			err = oc.Run("create").Args("-f", configFile).Execute()
			Expect(err).NotTo(HaveOccurred())

			By("expecting the mysql service get endpoints")
			err = oc.KubeFramework().WaitForAnEndpoint("mysql")
			Expect(err).NotTo(HaveOccurred())
		})
	})

})
