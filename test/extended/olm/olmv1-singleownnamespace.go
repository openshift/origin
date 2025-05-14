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

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMOwnSingleNamespace][Skipped:Disconnected] OLMv1 operator installation support for singleNamespace watch mode with quay-operator", g.Serial, func() {
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

	g.It("should install a cluster extension successfully", func(ctx g.SpecContext) {
		runSingleOwnNamespaceTest(ctx, oc, "install-quay-operator-singlens.yaml", false)
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMOwnSingleNamespace][Skipped:Disconnected] OLMv1 operator installation support for ownNamespace watch mode with quay-operator", g.Serial, func() {
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

	g.It("should install a cluster extension successfully", func(ctx g.SpecContext) {
		runSingleOwnNamespaceTest(ctx, oc, "install-quay-operator-ownns.yaml", false)
	})
})

var _ = g.Describe("[sig-olmv1][OCPFeatureGate:NewOLMOwnSingleNamespace][Skipped:Disconnected] OLMv1 operator installation support for ownNamespace watch mode with an operator that does not support ownNamespace installation mode", func() {
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

	g.It("should fail to install a cluster extension successfully", func(ctx g.SpecContext) {
		runSingleOwnNamespaceTest(ctx, oc, "install-openshift-pipelines-operator-ownns.yaml", true)
	})
})

//TODO: Need to add two more tests before this feature is GA.

func runSingleOwnNamespaceTest(ctx g.SpecContext, oc *exutil.CLI, fileName string, expectFailure bool) {
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
	if expectFailure {
		o.Expect(err).To(o.HaveOccurred())
	} else {
		o.Expect(lastReason).To(o.BeEmpty())
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}
