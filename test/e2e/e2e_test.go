package e2e

import (
	"testing"
	"k8s.io/kubernetes/test/e2e"
	. "github.com/onsi/ginkgo"
)

var _ = Describe("Custom Extension", func() {
	It("Should be extensible", func() {
		By("Adding a new test on top of the other E2Es")
		if(1 == 0){
			e2e.Failf("example test, will never fail.")
		}
		print(e2e.XYZ2)
	})
})

func TestE2EWrapper(t *testing.T) {
	e2e.TestE2E(t);
}