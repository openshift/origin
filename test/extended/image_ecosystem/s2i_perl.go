package image_ecosystem

import (
	"fmt"
	"strings"
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
		appSource     = exutil.FixturePath("testdata", "image_ecosystem", "perl-hotdeploy")
		perlTemplate  = exutil.FixturePath("testdata", "image_ecosystem", "perl-hotdeploy", "perl.json")
		oc            = exutil.NewCLI("s2i-perl", exutil.KubeConfigPath())
		modifyCommand = []string{"sed", "-ie", `s/initial value/modified value/`, "lib/My/Test.pm"}
		dcName        = "perl"
		rcNameOne     = fmt.Sprintf("%s-1", dcName)
		rcNameTwo     = fmt.Sprintf("%s-2", dcName)
		dcLabelOne    = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", rcNameOne))
		dcLabelTwo    = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", rcNameTwo))
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

		g.Describe("hot deploy test", func() {
			g.It("should work", func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

				exutil.CheckOpenShiftNamespaceImageStreams(oc)
				g.By(fmt.Sprintf("calling oc new-app -f %q", perlTemplate))
				err := oc.Run("new-app").Args("-f", perlTemplate, "-e", "HTTPD_START_SERVERS=1", "-e", "HTTPD_MAX_SPARE_SERVERS=1", "-e", "HTTPD_MAX_REQUEST_WORKERS=1").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				br, err := exutil.StartBuildAndWait(oc, "perl", fmt.Sprintf("--from-dir=%s", appSource))
				o.Expect(err).NotTo(o.HaveOccurred())
				br.AssertSuccess()

				g.By("waiting for build to finish")
				err = exutil.WaitForABuild(oc.BuildClient().Build().Builds(oc.Namespace()), rcNameOne, nil, nil, nil)
				if err != nil {
					exutil.DumpBuildLogs(dcName, oc)
				}
				o.Expect(err).NotTo(o.HaveOccurred())

				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), dcName, 1, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for endpoint")
				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), dcName)
				o.Expect(err).NotTo(o.HaveOccurred())
				oldEndpoint, err := oc.KubeFramework().ClientSet.Core().Endpoints(oc.Namespace()).Get(dcName, metav1.GetOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				checkPage := func(expected string, dcLabel labels.Selector) {
					_, err := exutil.WaitForPods(oc.KubeClient().Core().Pods(oc.Namespace()), dcLabel, exutil.CheckPodIsRunningFn, 1, 4*time.Minute)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
					result, err := CheckPageContains(oc, dcName, "", expected)
					o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
					o.ExpectWithOffset(1, result).To(o.BeTrue())
				}

				checkPage("initial value", dcLabelOne)

				g.By("modifying the source code with disabled hot deploy")
				err = RunInPodContainer(oc, dcLabelOne, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				checkPage("initial value", dcLabelOne)

				g.By("turning on hot-deploy")
				err = oc.Run("env").Args("dc", dcName, "PERL_APACHE2_RELOAD=true").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
				err = exutil.WaitForDeploymentConfig(oc.KubeClient(), oc.AppsClient().Apps(), oc.Namespace(), dcName, 2, true, oc)
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for a new endpoint")
				err = e2e.WaitForEndpoint(oc.KubeFramework().ClientSet, oc.Namespace(), dcName)
				o.Expect(err).NotTo(o.HaveOccurred())

				// Ran into an issue where we'd try to hit the endpoint before it was updated, resulting in
				// request timeouts against the previous pod's ip.  So make sure the endpoint is pointing to the
				// new pod before hitting it.
				err = wait.Poll(1*time.Second, 1*time.Minute, func() (bool, error) {
					newEndpoint, err := oc.KubeFramework().ClientSet.Core().Endpoints(oc.Namespace()).Get(dcName, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					if !strings.Contains(newEndpoint.Subsets[0].Addresses[0].TargetRef.Name, rcNameTwo) {
						e2e.Logf("waiting on endpoint address ref %s to contain %s", newEndpoint.Subsets[0].Addresses[0].TargetRef.Name, rcNameTwo)
						return false, nil
					}
					e2e.Logf("old endpoint was %#v, new endpoint is %#v", oldEndpoint, newEndpoint)
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("modifying the source code with enabled hot deploy")
				checkPage("initial value", dcLabelTwo)
				err = RunInPodContainer(oc, dcLabelTwo, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				checkPage("modified value", dcLabelTwo)
			})
		})

	})
})
