package kernel

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node][Suite:openshift/nodes/realtime/latency][Disruptive] Real time kernel should meet latency requirements when tested with", g.Ordered, func() {
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

	g.It("hwlatdetect", g.Label("Size:L"), func() {
		err := runHwlatdetect(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error occured running hwlatdetect")
	})

	g.It("oslat", g.Label("Size:L"), func() {
		cpuCount, err := getProcessorCount(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to get the number of processors online")

		err = runOslat(cpuCount, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error occured running oslat")
	})

	g.It("cyclictest", g.Label("Size:L"), func() {
		cpuCount, err := getProcessorCount(oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to get the number of processors online")

		err = runCyclictest(cpuCount, oc)
		o.Expect(err).NotTo(o.HaveOccurred(), "error occured running cyclictest")
	})

	g.AfterEach(func() {
		cleanupRtTestPod(oc)
	})

	g.AfterAll(func() {
		cleanupRealtimeTestEnvironment(oc)
	})
})
