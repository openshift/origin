package image_ecosystem

import (
	"fmt"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-devex][Feature:ImageEcosystem][php][Slow] hot deploy for openshift php image", func() {
	defer g.GinkgoRecover()
	var (
		cakephpTemplate = "cakephp-mysql-example"
		oc              = exutil.NewCLI("s2i-php")
		hotDeployParam  = "OPCACHE_REVALIDATE_FREQ=0"
		modifyCommand   = []string{"sed", "-ie", `s/\$result\['c'\]/1337/`, "src/Template/Pages/home.ctp"}
		pageRegexpCount = `<span class="code" id="count-value">([^0][0-9]*)</span>`
		pageExactCount  = `<span class="code" id="count-value">%d</span>`
		dcName          = "cakephp-mysql-example-1"
		dcLabel         = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", dcName))
	)

	g.Context("", func() {
		g.JustBeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("CakePHP example", func() {
			g.It(fmt.Sprintf("should work with hot deploy [apigroup:image.openshift.io][apigroup:operator.openshift.io][apigroup:config.openshift.io][apigroup:build.openshift.io]"), g.Label("Size:L"), func() {

				exutil.WaitForOpenShiftNamespaceImageStreams(oc)
				g.By(fmt.Sprintf("calling oc new-app %q -p %q", cakephpTemplate, hotDeployParam))
				err := oc.Run("new-app").Args(cakephpTemplate, "-p", hotDeployParam).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for build to finish")
				err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(oc.Namespace()), dcName, nil, nil, nil)
				if err != nil {
					exutil.DumpBuildLogs("cakephp-mysql-example", oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().AppsV1(), oc.Namespace(), "cakephp-mysql-example", 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for endpoint")
				err = exutil.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), "cakephp-mysql-example")
				o.Expect(err).NotTo(o.HaveOccurred())

				assertPageCountRegexp := func(priorValue string) string {
					_, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

					result, val, err := CheckPageRegexp(oc, "cakephp-mysql-example", "", pageRegexpCount, 1)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
					o.ExpectWithOffset(1, result).To(o.BeTrue())
					if len(priorValue) > 0 {
						p, err := strconv.Atoi(priorValue)
						o.Expect(err).NotTo(o.HaveOccurred())
						v, err := strconv.Atoi(val)
						g.By(fmt.Sprintf("comparing prior value %d with lastest value %d", p, v))
						o.Expect(err).NotTo(o.HaveOccurred())
						o.Expect(v).To(o.BeNumerically(">", p))
					}
					return val
				}

				g.By("checking page count")
				val := assertPageCountRegexp("")
				assertPageCountRegexp(val)

				g.By("modifying the source code with hot deploy enabled")
				err = RunInPodContainer(oc, dcLabel, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())

				assertPageCountIs := func(i int) {
					_, err := exutil.WaitForPods(oc.KubeClient().CoreV1().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunning, 1, 4*time.Minute)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

					result, err := CheckPageContains(oc, "cakephp-mysql-example", "", fmt.Sprintf(pageExactCount, i))
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
					o.ExpectWithOffset(1, result).To(o.BeTrue())
				}

				g.By("checking page count after modifying the source code")
				assertPageCountIs(1337)
			})
		})
	})
})
