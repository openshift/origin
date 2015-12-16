package e2e

import (
	"testing"
	//. "github.com/onsi/ginkgo"
	"k8s.io/kubernetes/test/e2e"
)

//var _ = Describe("CustomTest", func() {
//})

func TestE2EWrapper(t *testing.T) {
	e2e.TestE2E(t);
}