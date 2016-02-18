package e2e

import (
	"testing"

	. "github.com/onsi/ginkgo"
	ke2e "k8s.io/kubernetes/test/e2e"
)

var _ = Describe("Custom Extension", func() {
	It("Should be extensible", func() {
		By("Adding a new test on top of the other E2Es")
		if 1 == 0 {
			ke2e.Failf("example test, will never fail.")
		}
	})
})

func init() {
	ke2e.RegisterFlags()
}

func TestE2E(t *testing.T) {
	ke2e.RunE2ETests(t)
}
