package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[image_ecosystem][php][Slow] hot deploy for openshift php image", func() {
	defer g.GinkgoRecover()
	var (
		cakephpTemplate = "https://raw.githubusercontent.com/openshift/cakephp-ex/master/openshift/templates/cakephp-mysql.json"
		oc              = exutil.NewCLI("s2i-php", exutil.KubeConfigPath())
		hotDeployParam  = "OPCACHE_REVALIDATE_FREQ=0"
		modifyCommand   = []string{"sed", "-ie", `s/\$result\['c'\]/1337/`, "app/View/Layouts/default.ctp"}
		pageCountFn     = func(count int) string { return fmt.Sprintf(`<span class="code" id="count-value">%d</span>`, count) }
		dcName          = "cakephp-mysql-example-1"
		dcLabel         = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", dcName))
	)
	g.Describe("CakePHP example", func() {
		g.It(fmt.Sprintf("should work with hot deploy"), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			exutil.CheckOpenShiftNamespaceImageStreams(oc)
			g.By(fmt.Sprintf("calling oc new-app -f %q -p %q", cakephpTemplate, hotDeployParam))
			err := oc.Run("new-app").Args("-f", cakephpTemplate, "-p", hotDeployParam).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for build to finish")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), dcName, exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			if err != nil {
				exutil.DumpBuildLogs("cakephp-mysql-example", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
			// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
			err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), "cakephp-mysql-example", oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for endpoint")
			err = oc.KubeFramework().WaitForAnEndpoint("cakephp-mysql-example")
			o.Expect(err).NotTo(o.HaveOccurred())

			assertPageCountIs := func(i int) {
				_, err := exutil.WaitForPods(oc.KubeREST().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())

				result, err := CheckPageContains(oc, "cakephp-mysql-example", "", pageCountFn(i))
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(result).To(o.BeTrue())
			}

			g.By("checking page count")

			assertPageCountIs(1)
			assertPageCountIs(2)

			g.By("modifying the source code with disabled hot deploy")
			RunInPodContainer(oc, dcLabel, modifyCommand)
			g.By("checking page count after modifying the source code")
			assertPageCountIs(1337)
		})
	})
})
