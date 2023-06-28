package kernel

import (
	g "github.com/onsi/ginkgo/v2"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node][Suite:openshift/nodes/realtime][Disruptive] Real time kernel should allow", g.Ordered, func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI(rtNamespace).AsAdmin()
	)

	g.BeforeAll(func() {
		failIfNotRT(oc)
		configureRealtimeTestEnvironment(oc)
	})

	g.BeforeEach(func() {
		startRtTestPod(oc)
	})

	g.It("pi_stress to run successfully with the fifo algorithm", func() {
		runPiStressFifo(oc)
	})

	g.It("pi_stress to run successfully with the round robin algorithm", func() {
		runPiStressRR(oc)
	})

	g.It("deadline_test to run successfully", func() {
		runDeadlineTest(oc)
	})

	g.AfterEach(func() {
		cleanupRtTestPod(oc)
	})

	g.AfterAll(func() {
		cleanupRealtimeTestEnvironment(oc)
	})

})
