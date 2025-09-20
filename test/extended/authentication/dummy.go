package authentication

import (
	"time"

	g "github.com/onsi/ginkgo/v2"

	o "github.com/onsi/gomega"
)

var _ = g.Describe("[sig-auth][Serial][suite:openshift/dummy-suite] dummy suite", g.Ordered, func() {
	defer g.GinkgoRecover()

	g.BeforeAll(func() {
		// Do some long setup....
		time.Sleep(5 * time.Second)
	})

	g.It("should be quick", func() {
		o.Expect(true).To(o.BeTrue())
	})

	g.It("should be fast", func() {
		actual := 2 + 2
		o.Expect(actual).To(o.Equal(4))
	})

	g.It("shouldn't take long", func() {
		o.Expect("yes").NotTo(o.Equal("no"))
	})

	g.AfterAll(func() {
		// Do some long cleanup
		time.Sleep(5 * time.Second)
	})
})
