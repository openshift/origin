package networking

import (
	e2e "k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[Area:Networking] network isolation", func() {
	InNonIsolatingContext(func() {
		f1 := e2e.NewDefaultFramework("net-isolation1")
		f2 := e2e.NewDefaultFramework("net-isolation2")

		It("should allow communication between pods in different namespaces on the same node", func() {
			Expect(checkPodIsolation(f1, f2, SAME_NODE)).To(Succeed())
		})

		It("should allow communication between pods in different namespaces on different nodes", func() {
			Expect(checkPodIsolation(f1, f2, DIFFERENT_NODE)).To(Succeed())
		})
	})

	InIsolatingContext(func() {
		f1 := e2e.NewDefaultFramework("net-isolation1")
		f2 := e2e.NewDefaultFramework("net-isolation2")

		It("should prevent communication between pods in different namespaces on the same node", func() {
			Expect(checkPodIsolation(f1, f2, SAME_NODE)).NotTo(Succeed())
		})

		It("should prevent communication between pods in different namespaces on different nodes", func() {
			Expect(checkPodIsolation(f1, f2, DIFFERENT_NODE)).NotTo(Succeed())
		})

		// The test framework doesn't allow us to easily make use of the actual "default"
		// namespace, so we test default namespace behavior by changing either f1's or
		// f2's NetNamespace to have VNID 0 instead. But this only works under the
		// multi-tenant plugin since the single-tenant one doesn't create NetNamespaces at
		// all (and so there's not really any point in even running these tests anyway).
		It("should allow communication from default to non-default namespace on the same node", func() {
			makeNamespaceGlobal(f2.Namespace)
			Expect(checkPodIsolation(f1, f2, SAME_NODE)).To(Succeed())
		})
		It("should allow communication from default to non-default namespace on a different node", func() {
			makeNamespaceGlobal(f2.Namespace)
			Expect(checkPodIsolation(f1, f2, DIFFERENT_NODE)).To(Succeed())
		})
		It("should allow communication from non-default to default namespace on the same node", func() {
			makeNamespaceGlobal(f1.Namespace)
			Expect(checkPodIsolation(f1, f2, SAME_NODE)).To(Succeed())
		})
		It("should allow communication from non-default to default namespace on a different node", func() {
			makeNamespaceGlobal(f1.Namespace)
			Expect(checkPodIsolation(f1, f2, DIFFERENT_NODE)).To(Succeed())
		})
	})
})
