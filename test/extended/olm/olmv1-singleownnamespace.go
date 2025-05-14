package operators

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMOwnSingleNamespace][Skipped:Disconnected] OLMv1 operator single/own namespace support", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("openshift-operator-controller")

	g.BeforeEach(func() {
		exutil.PreTestDump()
	})

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWith("", oc)
		}
	})

	g.It("should install a cluster extension in SingleNamespace mode", func(ctx g.SpecContext) {
		runSingleOwnNamespaceTest(ctx, oc, "install-lvm-operator-singlens.yaml")
	})

	g.It("should install a cluster extension in OtherNamespace mode", func(ctx g.SpecContext) {
		runSingleOwnNamespaceTest(ctx, oc, "install-lvm-operator-otherns.yaml")
	})
})

func runSingleOwnNamespaceTest(ctx g.SpecContext, oc *exutil.CLI, fileName string) {
	checkFeatureCapability(oc)

	var (
		baseDir = exutil.FixturePath("testdata", "olmv1")
		ceFile  = filepath.Join(baseDir, fileName)
	)

	cleanup, unique := applyResourceFile(oc, "", "", "", ceFile)
	ceName := "install-test-ce-" + unique
	g.DeferCleanup(cleanup)

	g.By("waiting for the ClusterExtention to be installed")
	var lastReason string
	err := wait.PollUntilContextTimeout(ctx, time.Second, 5*time.Minute, true,
		func(ctx context.Context) (bool, error) {
			b, err, s := waitForClusterExtensionReady(oc, ceName)
			if lastReason != s {
				g.GinkgoLogr.Info(fmt.Sprintf("waitForClusterExtensionReady: %q", s))
				lastReason = s
			}
			return b, err
		})
	o.Expect(lastReason).To(o.BeEmpty())
	o.Expect(err).NotTo(o.HaveOccurred())
}
