package builds

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds] remove all builds when build configuration is removed", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-build.yaml")
		oc           = exutil.NewCLI("cli-remove-build")
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			oc.Run("create").Args("-f", buildFixture).Execute()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("oc delete buildconfig", func() {
			g.It("should start builds and delete the buildconfig", func() {
				var (
					err    error
					builds [4]string
				)

				g.By("starting multiple builds")
				for i := range builds {
					stdout, _, err := exutil.StartBuild(oc, "sample-build", "-o=name")
					o.Expect(err).NotTo(o.HaveOccurred())
					builds[i] = stdout
				}

				g.By("deleting the buildconfig")
				err = oc.Run("delete").Args("bc/sample-build").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for builds to clear")
				err = wait.Poll(3*time.Second, 3*time.Minute, func() (bool, error) {
					builds, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).List(metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					if len(builds.Items) > 0 {
						return false, nil
					}
					configMaps, err := oc.KubeClient().CoreV1().ConfigMaps(oc.Namespace()).List(metav1.ListOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())
					if len(configMaps.Items) > 0 {
						return false, nil
					}
					return true, nil
				})
				if err == wait.ErrWaitTimeout {
					g.Fail("timed out waiting for builds to clear")
				}
			})

		})
	})
})
