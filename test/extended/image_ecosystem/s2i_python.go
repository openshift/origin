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

var _ = g.Describe("[image_ecosystem][python][Slow] hot deploy for openshift python image", func() {
	defer g.GinkgoRecover()

	var (
		oc               = exutil.NewCLI("s2i-python", exutil.KubeConfigPath())
		djangoRepository = "https://github.com/openshift/django-ex.git"
		modifyCommand    = []string{"sed", "-ie", `s/'count': PageView.objects.count()/'count': 1337/`, "welcome/views.py"}
		pageCountFn      = func(count int) string { return fmt.Sprintf("Page views: %d", count) }
		dcName           = "django-ex"
		rcNameOne        = fmt.Sprintf("%s-1", dcName)
		rcNameTwo        = fmt.Sprintf("%s-2", dcName)
		dcLabelOne       = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", rcNameOne))
		dcLabelTwo       = exutil.ParseLabelsOrDie(fmt.Sprintf("deployment=%s", rcNameTwo))
	)

	g.Context("", func() {
		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("Django example", func() {
			g.It(fmt.Sprintf("should work with hot deploy"), func() {
				oc.SetOutputDir(exutil.TestContext.OutputDir)

				err := exutil.WaitForOpenShiftNamespaceImageStreams(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				g.By(fmt.Sprintf("calling oc new-app %s", djangoRepository))
				err = oc.Run("new-app").Args(djangoRepository, "--strategy=source").Execute()
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
					if err != nil || !result {
						exutil.DumpApplicationPodLogs(dcName, oc)
					}
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
				err = oc.Run("env").Args("dc", dcName, "APP_CONFIG=conf/reload.py").Execute()
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
				assertPageCountIs(1, dcLabelTwo)
				err = RunInPodContainer(oc, dcLabelTwo, modifyCommand)
				o.Expect(err).NotTo(o.HaveOccurred())
				assertPageCountIs(1337, dcLabelTwo)
			})
		})
	})
})
