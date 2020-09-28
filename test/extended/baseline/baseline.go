package baseline

import (
	"time"

	g "github.com/onsi/ginkgo"
)

// This test is intended to be run by itself against a new cluster to collect
// baseline performance data.
var _ = g.Describe("[sig-scalability][Suite:openshift/scalability] Managed cluster should", func() {
	defer g.GinkgoRecover()

	g.It("approach fixed resource consumption in the steady state", func() {
		g.By("observing the cluster at rest for 15 minutes at any point after installation")
		time.Sleep(15 * time.Minute)
	})
})
