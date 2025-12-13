package operators

import (
	g "github.com/onsi/ginkgo/v2"
)

var _ = g.Describe("[sig-arch][OCPFeatureGate:Example]", g.Ordered, func() {
	defer g.GinkgoRecover()

	g.It("should only run FeatureGated test when enabled", g.Label("Size:S"), func() {
	})

})
