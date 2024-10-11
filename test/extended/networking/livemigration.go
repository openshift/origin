package networking

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("[sig-network][OCPFeatureGate:PersistentIPsForVirtualization][Feature:Layer2LiveMigration]", func() {
	InOVNKubernetesContext(func() {
		It("dummy test [Suite:openshift/network/virtualization]", func() {
			Expect(1).To(Equal(1))
		})
	})
})
