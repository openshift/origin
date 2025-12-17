package builds

import (
	"context"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-builds][Feature:Builds] imagechangetriggers", func() {
	defer g.GinkgoRecover()

	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-imagechangetriggers.yaml")
		oc           = exutil.NewCLI("imagechangetriggers")
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.AfterEach(func() {
			if g.CurrentSpecReport().Failed() {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
				exutil.DumpImageStream(oc, oc.Namespace(), "nodejs-ex")
				exutil.DumpBuildConfigs(oc)
			}
		})

		g.It("imagechangetriggers should trigger builds of all types [apigroup:build.openshift.io]", g.Label("Size:L"), func() {
			err := oc.AsAdmin().Run("create").Args("-f", buildFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			err = wait.Poll(time.Second, 30*time.Second, func() (done bool, err error) {
				for _, build := range []string{"bc-docker-1", "bc-jenkins-1", "bc-source-1", "bc-custom-1"} {
					_, err := oc.BuildClient().BuildV1().Builds(oc.Namespace()).Get(context.Background(), build, metav1.GetOptions{})
					if err == nil {
						continue
					}
					if kerrors.IsNotFound(err) {
						return false, nil
					}
					return false, err
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})
