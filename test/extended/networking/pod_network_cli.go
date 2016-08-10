package networking

import (
	"fmt"

	exutil "github.com/openshift/origin/test/extended/util"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("[networking] pod-network admin commands", func() {
	InMultiTenantContext(func() {
		oc := exutil.NewCLI("pod-network", exutil.KubeConfigPath()).AsAdmin()

		f1 := e2e.NewDefaultFramework("net-isolation1")
		f2 := e2e.NewDefaultFramework("net-isolation2")
		f3 := e2e.NewDefaultFramework("net-isolation3")

		// Pod and service communication from f1 --> f2 and f2 --> f1 should succeed
		commSuccess := func(f1, f2 *e2e.Framework, n NodeType) {
			Expect(checkPodIsolation(f1, f2, n)).To(Succeed())
			Expect(checkPodIsolation(f2, f1, n)).To(Succeed())
			Expect(checkServiceConnectivity(f1, f2, n)).To(Succeed())
			Expect(checkServiceConnectivity(f2, f1, n)).To(Succeed())
		}
		// Pod and service communication from f1 --> f2 and f2 --> f1 should fail
		commFailure := func(f1, f2 *e2e.Framework, n NodeType) {
			Expect(checkPodIsolation(f1, f2, n)).NotTo(Succeed())
			Expect(checkPodIsolation(f2, f1, n)).NotTo(Succeed())
			Expect(checkServiceConnectivity(f1, f2, n)).NotTo(Succeed())
			Expect(checkServiceConnectivity(f2, f1, n)).NotTo(Succeed())
		}

		verifyJoinProjects := func(f1, f2, f3 *e2e.Framework, n NodeType) {
			// Join project networks for f1 and f2
			Expect(joinProjects(oc, f1, f2)).To(Succeed())

			commSuccess(f1, f2, n)
			commFailure(f1, f3, n)
		}

		verifyMakeProjectsGlobal := func(f1, f2, f3 *e2e.Framework, n NodeType) {
			// Make project network for f1 global
			Expect(makeProjectsGlobal(oc, f1)).To(Succeed())

			commSuccess(f1, f2, n)
			commSuccess(f1, f3, n)
		}

		verifyIsolateProjects := func(f1, f2, f3 *e2e.Framework, n NodeType) {
			// Join project networks for f1 and f2
			Expect(joinProjects(oc, f1, f2)).To(Succeed())

			// Isolate project network for f1
			Expect(isolateProjects(oc, f1)).To(Succeed())

			commFailure(f1, f2, n)
			commFailure(f1, f3, n)

			// Make project network for f1 global
			Expect(makeProjectsGlobal(oc, f1)).To(Succeed())

			// Isolate project network for f1
			Expect(isolateProjects(oc, f1)).To(Succeed())

			commFailure(f1, f2, n)
			commFailure(f1, f3, n)
		}

		It("join-projects should allow communication between pods/services in different projects on the same node", func() {
			verifyJoinProjects(f1, f2, f3, SAME_NODE)
		})
		It("join-projects should allow communication between pods/services in different projects on different nodes", func() {
			verifyJoinProjects(f1, f2, f3, DIFFERENT_NODE)
		})
		It("make-projects-global should allow project to communicate with any pod in the cluster on the same node", func() {
			verifyMakeProjectsGlobal(f1, f2, f3, SAME_NODE)
		})
		It("make-projects-global should allow project to communicate with any pod in the cluster on different nodes", func() {
			verifyMakeProjectsGlobal(f1, f2, f3, DIFFERENT_NODE)
		})
		It("isolate-projects should not allow project to communicate with other projects in the cluster on the same node", func() {
			verifyIsolateProjects(f1, f2, f3, SAME_NODE)
		})
		It("isolate-projects should not allow project to communicate with other projects in the cluster on different nodes", func() {
			verifyIsolateProjects(f1, f2, f3, DIFFERENT_NODE)
		})
	})
})

func joinProjects(oc *exutil.CLI, f1, f2 *e2e.Framework) error {
	return oc.Run("adm").Args("pod-network", "join-projects", fmt.Sprintf("--to=%s", f1.Namespace.Name), f2.Namespace.Name).Execute()
}

func makeProjectsGlobal(oc *exutil.CLI, f *e2e.Framework) error {
	return oc.Run("adm").Args("pod-network", "make-projects-global", f.Namespace.Name).Execute()
}

func isolateProjects(oc *exutil.CLI, f *e2e.Framework) error {
	return oc.Run("adm").Args("pod-network", "isolate-projects", f.Namespace.Name).Execute()
}
