package extension

import (
	g "github.com/onsi/ginkgo/v2"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[sig-ci] [OTE] OpenShift Tests Extension [Suite:openshift/ote]", func() {
	defer g.GinkgoRecover()

	_ = g.It("should support tests that succeed", g.Label("Size:S"), func() {})

	_ = g.It("should support tests with an informing lifecycle", g.Label("Size:S"), ote.Informing(), func() {
		e2e.Fail("This test is intended to fail.")
	})
})
