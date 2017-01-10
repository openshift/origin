package builds

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[builds][Conformance] oc new-app", func() {
	// Previously, the maximum length of app names creatable by new-app has
	// inadvertently been decreased, e.g. by creating an annotation somewhere
	// whose name itself includes the app name.  Ensure we can create and fully
	// deploy an app with a 58 character name [63 maximum - len('-9999' suffix)].

	oc := exutil.NewCLI("new-app", exutil.KubeConfigPath())

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should succeed with a --name of 58 characters", func() {
		g.By("calling oc new-app")
		err := oc.Run("new-app").Args("https://github.com/openshift/nodejs-ex", "--name", "a234567890123456789012345678901234567890123456789012345678").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the deployment to complete")
		err = exutil.WaitForADeploymentToComplete(oc.KubeClient().Core().ReplicationControllers(oc.Namespace()), "a234567890123456789012345678901234567890123456789012345678", oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should fail with a --name longer than 58 characters", func() {
		g.By("calling oc new-app")
		out, err := oc.Run("new-app").Args("https://github.com/openshift/nodejs-ex", "--name", "a2345678901234567890123456789012345678901234567890123456789").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.HavePrefix("error: invalid name: "))
	})
})
