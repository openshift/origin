package ci

import (
	"math/rand"
	"os"
	"strconv"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

/*
Example usage:

	$ echo '"[sig-ci] Test should fail at a configurable rate"' | \
		DUMMY_FAILURE_RATE=1.0 ./openshift-tests run all --retry-strategy aggressive --junit-dir=/tmp/junit -f -
*/
var _ = g.Describe("[sig-ci] [Suite:none] Test should fail", func() {
	defer g.GinkgoRecover()

	g.It("at a configurable rate", g.Label("Size:S"), func() {
		failureRateStr := os.Getenv("DUMMY_FAILURE_RATE")
		if failureRateStr == "" {
			// Default is always passes just in case this test is accidentally run in CI...
			failureRateStr = "0.0"
		}

		failureRate, err := strconv.ParseFloat(failureRateStr, 32)
		o.Expect(err).NotTo(o.HaveOccurred(), "DUMMY_FAILURE_RATE should be a valid float between 0.0 and 1.0")
		o.Expect(failureRate >= 0.0 && failureRate <= 1.0).To(o.BeTrue(), "DUMMY_FAILURE_RATE should be between 0.0 and 1.0")

		rand.Seed(time.Now().UnixNano() + int64(len("dummy-configurable")))

		sleepTime := time.Duration(1) * time.Second // 10-70 seconds
		time.Sleep(sleepTime)

		if rand.Float32() < float32(failureRate) {
			e2e.Failf("Dummy test failed randomly (%.0f%% chance, configured via DUMMY_FAILURE_RATE) - duration: %v", failureRate*100, sleepTime)
		}

		g.GinkgoWriter.Printf("Dummy test passed randomly (%.0f%% chance, configured via DUMMY_FAILURE_RATE) - duration: %v\n", (1.0-failureRate)*100, sleepTime)
	})

})
