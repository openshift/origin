package authorization

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:OpenShiftAuthorization] Could grant admin permission for the service account username to access to its own project", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("authorization", exutil.KubeConfigPath())

	g.It("Bind project admin role to sa,then check whether sa can access resources in that project", func() {
		namespace := oc.Namespace()

		g.By("calling oc new-app")
		_, err := oc.Run("new-app").Args("https://github.com/openshift/ruby-hello-world#config").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the build to complete")
		err = exutil.WaitForABuild(oc.BuildClient().BuildV1().Builds(namespace), "ruby-hello-world-1", nil, nil, nil)
		if err != nil {
			exutil.DumpBuildLogs("ruby-hello-world", oc)
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("create a serviceaccount named demo")
		err = oc.Run("create").Args("sa", "demo").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("give project admin role to the demo service account")
		err = oc.Run("policy").Args("add-role-to-user", "admin", "system:serviceaccount:"+namespace+":demo").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("And I switch to the demo service account")
		saToken, err := oc.Run("sa").Args("get-token", "demo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = oc.WithoutNamespace().Run("login").Args("--token", saToken).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		out, err := oc.WithoutNamespace().Run("whoami").Args().Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("system:serviceaccount:" + namespace + ":demo"))

		g.By("get some resources in this project")
		out, err = oc.Run("get").Args("build/ruby-hello-world-1").Output()
		o.Expect(out).To(o.ContainSubstring("ruby-hello-world-1"))

	})
})
