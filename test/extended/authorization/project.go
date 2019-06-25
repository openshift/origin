package authorization

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

var _ = g.Describe("[Feature:OpenShiftAuthorization] Could delete all resources when delete the project", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("authorization", exutil.KubeConfigPath())

	g.It("Create some resources in a project,then delete that project", func() {
		g.By("create additional projects")
		namespace1 := oc.Namespace() + "-1"
		err := oc.Run("new-project").Args(namespace1).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("calling oc new-app")
		_, err = oc.WithoutNamespace().Run("new-app").Args("https://github.com/openshift/ruby-hello-world#config", "-n", namespace1).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the build to complete")
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(namespace1), "ruby-hello-world-1", nil, nil, nil)
		if err != nil {
			exutil.DumpBuildLogs("ruby-hello-world", oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("get some resources in this project")
		out, stderr, err := oc.WithoutNamespace().Run("get").Args("build/ruby-hello-world-1", "-n", namespace1).Outputs()
		o.Expect(out).To(o.ContainSubstring("ruby-hello-world-1"))

		g.By("delete that project")
		err = oc.WithoutNamespace().Run("delete").Args("projects", namespace1, "-n", namespace1).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting the project has been deleted")
		timeout := e2e.DefaultNamespaceDeletionTimeout
		e2e.WaitForNamespacesDeleted(oc.KubeFramework().ClientSet, []string{namespace1}, timeout)

		g.By("get some resources in that deleted project")
		out, stderr, err = oc.WithoutNamespace().Run("get").Args("build/ruby-hello-world-1", "-n", namespace1).Outputs()
		o.Expect(stderr).To(o.ContainSubstring("builds.build.openshift.io \"ruby-hello-world-1\" is forbidden"))

	})
})
