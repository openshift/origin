package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[image_ecosystem][perl][Slow] hot deploy for openshift perl image", func() {
	defer g.GinkgoRecover()
	var (
		dancerTemplate = "https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer-mysql.json"
		oc             = exutil.NewCLI("s2i-perl", exutil.KubeConfigPath())
		modifyCommand  = []string{"sed", "-ie", `s/data => \$data\[0\]/data => "1337"/`, "lib/default.pm"}
		pageCountFn    = func(count int) string { return fmt.Sprintf(`<span class="code" id="count-value">%d</span>`, count) }
		dcName         = "dancer-mysql-example-1"
		dcLabel        = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", dcName))
	)

	g.Describe("Dancer example", func() {
		g.It(fmt.Sprintf("should work with hot deploy"), func() {
			oc.SetOutputDir(exutil.TestContext.OutputDir)

			exutil.CheckOpenShiftNamespaceImageStreams(oc)
			g.By(fmt.Sprintf("calling oc new-app -f %q", dancerTemplate))
			err := oc.Run("new-app").Args("-f", dancerTemplate).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for build to finish")
			err = exutil.WaitForABuild(oc.REST().Builds(oc.Namespace()), "dancer-mysql-example-1", exutil.CheckBuildSuccessFn, exutil.CheckBuildFailedFn)
			if err != nil {
				exutil.DumpBuildLogs("dancer-mysql-example", oc)
			}
			o.Expect(err).NotTo(o.HaveOccurred())

			// oc.KubeFramework().WaitForAnEndpoint currently will wait forever;  for now, prefacing with our WaitForADeploymentToComplete,
			// which does have a timeout, since in most cases a failure in the service coming up stems from a failed deployment
			err = exutil.WaitForADeploymentToComplete(oc.KubeREST().ReplicationControllers(oc.Namespace()), "dancer-mysql-example", oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for endpoint")
			err = oc.KubeFramework().WaitForAnEndpoint("dancer-mysql-example")
			o.Expect(err).NotTo(o.HaveOccurred())

			assertPageCountIs := func(i int) {
				_, err := exutil.WaitForPods(oc.KubeREST().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunningFn, 1, 2*time.Minute)
				o.Expect(err).NotTo(o.HaveOccurred())

				result, err := CheckPageContains(oc, "dancer-mysql-example", "", pageCountFn(i))
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(result).To(o.BeTrue())
			}

			g.By("checking page count")
			assertPageCountIs(1)
			assertPageCountIs(2)

			g.By("modifying the source code with disabled hot deploy")
			RunInPodContainer(oc, dcLabel, modifyCommand)
			assertPageCountIs(3)

			pods, err := oc.KubeREST().Pods(oc.Namespace()).List(kapi.ListOptions{LabelSelector: dcLabel})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(len(pods.Items)).To(o.Equal(1))

			g.By("turning on hot-deploy")
			err = oc.Run("env").Args("rc", dcName, "PERL_APACHE2_RELOAD=true").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("scale").Args("rc", dcName, "--replicas=0").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			err = exutil.WaitUntilPodIsGone(oc.KubeREST().Pods(oc.Namespace()), pods.Items[0].Name, 1*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = oc.Run("scale").Args("rc", dcName, "--replicas=1").Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("modifying the source code with enabled hot deploy")
			RunInPodContainer(oc, dcLabel, modifyCommand)
			assertPageCountIs(1337)
		})
	})
})
