package image_ecosystem

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[image_ecosystem][perl][Slow] hot deploy for openshift perl image", func() {
	defer g.GinkgoRecover()
	var (
		dancerTemplate = "https://raw.githubusercontent.com/openshift/dancer-ex/master/openshift/templates/dancer-mysql.json"
		oc             = exutil.NewCLI("s2i-perl", exutil.KubeConfigPath())
		modifyCommand  = []string{"sed", "-ie", `s/data => \$data\[0\]/data => "1337"/`, "lib/default.pm"}
		pageCountFn    = func(count int) string { return fmt.Sprintf(`<span class="code" id="count-value">%d</span>`, count) }
		dcName         = "dancer-mysql-example"
		rcNameOne      = fmt.Sprintf("%s-1", dcName)
		rcNameTwo      = fmt.Sprintf("%s-2", dcName)
		dcLabelOne     = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", rcNameOne))
		dcLabelTwo     = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", rcNameTwo))
	)

	g.Context("", func() {
		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Dancer example", func() {
			g.It(fmt.Sprintf("should work with hot deploy"), func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

				exutil.CheckOpenShiftNamespaceImageStreams(oc)
				g.By(fmt.Sprintf("calling oc new-app -f %q", dancerTemplate))
				err := oc.Run("new-app").Args("-f", dancerTemplate).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for build to finish")
				err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), rcNameOne, nil, nil, nil)
				if err != nil {
					exutil.DumpBuildLogs(dcName, oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), dcName, 1, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for endpoint")
				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), dcName)
				o.Expect(err).NotTo(o.HaveOccurred())
				oldEndpoint, err := oc.KubeFramework().ClientSet.Core().Endpoints(oc.Namespace()).Get(dcName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				assertPageCountIs := func(i int, dcLabel labels.Selector) {
					_, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunningFn, 1, 4*time.Minute)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

					result, err := CheckPageContains(oc, dcName, "", pageCountFn(i))
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
					o.ExpectWithOffset(1, result).To(o.BeTrue())
				}

				g.By("checking page count")
				assertPageCountIs(1, dcLabelOne)
				assertPageCountIs(2, dcLabelOne)

				g.By("modifying the source code with disabled hot deploy")
				err = RunInPodContainer(oc, dcLabelOne, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				assertPageCountIs(3, dcLabelOne)

				pods, err := oc.KubeClient().Core().Pods(oc.Namespace()).List(metav1.ListOptions{LabelSelector: dcLabelOne.String()})
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(len(pods.Items)).To(o.Equal(1))

				g.By("turning on hot-deploy")
				err = oc.Run("env").Args("dc", dcName, "PERL_APACHE2_RELOAD=true").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), dcName, 2, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for a new endpoint")
				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), dcName)
				o.Expect(err).NotTo(o.HaveOccurred())

				// Ran into an issue where we'd try to hit the endpoint before it was updated, resulting in
				// request timeouts against the previous pod's ip.  So make sure the endpoint ip has changed before
				// hitting it.
				err = wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
					newEndpoint, err := oc.KubeFramework().ClientSet.Core().Endpoints(oc.Namespace()).Get(dcName, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					if newEndpoint.Subsets[0].Addresses[0].IP == oldEndpoint.Subsets[0].Addresses[0].IP {
						return false, nil
					}
					e2e.Logf("old endpoint was %#v, new endpoint is %#v", oldEndpoint, newEndpoint)
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("modifying the source code with enabled hot deploy")
				assertPageCountIs(4, dcLabelTwo)
				err = RunInPodContainer(oc, dcLabelTwo, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				assertPageCountIs(1337, dcLabelTwo)
			})
		})
	})
})
