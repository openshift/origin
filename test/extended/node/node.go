package node

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

var _ = g.Describe("[Jira:node][sig-node] node test", func() {
	g.It("should always pass [Suite:openshift/node/conformance/parallel]", func() {
		o.Expect(true).To(o.BeTrue())
	})
})
