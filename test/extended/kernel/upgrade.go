package kernel

import (
	g "github.com/onsi/ginkgo/v2"
	exutil "github.com/openshift/origin/test/extended/util"
)

// Early upgrade cycle tests
var _ = g.Describe("[sig-node][Disruptive][Feature:ClusterUpgrade]", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLI(rtNamespace).AsAdmin()
	)

	g.Context("[Early] Openshift running on RT Kernel prior to upgrade", g.Ordered, func() {
		g.BeforeAll(func() {
			skipIfNotRT(oc)
			configureRealtimeTestEnvironment(oc)
		})

		g.BeforeEach(func() {
			startRtTestPod(oc)
		})

		g.It("should allow pi_stress to run successfully with the fifo algorithm", func() {
			runPiStressFifo(oc)
		})

		g.It("should allow pi_stress to run successfully with the round robin algorithm", func() {
			runPiStressRR(oc)
		})

		g.It("should allow deadline_test to run successfully", func() {
			runDeadlineTest(oc)
		})

		g.AfterEach(func() {
			cleanupRtTestPod(oc)
		})

		g.AfterAll(func() {
			cleanupRealtimeTestEnvironment(oc)
		})
	})

	g.Context("[Late] Openshift running on RT Kernel after upgrade", g.Ordered, func() {
		g.BeforeAll(func() {
			skipIfNotRT(oc)
			configureRealtimeTestEnvironment(oc)
		})

		g.BeforeEach(func() {
			startRtTestPod(oc)
		})

		g.It("should allow pi_stress to run successfully with the fifo algorithm", func() {
			runPiStressFifo(oc)
		})

		g.It("should allow pi_stress to run successfully with the round robin algorithm", func() {
			runPiStressRR(oc)
		})

		g.It("should allow deadline_test to run successfully", func() {
			runDeadlineTest(oc)
		})

		g.AfterEach(func() {
			cleanupRtTestPod(oc)
		})

		g.AfterAll(func() {
			cleanupRealtimeTestEnvironment(oc)
		})
	})
})
