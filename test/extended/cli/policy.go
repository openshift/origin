package cli

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] policy", func() {
	defer g.GinkgoRecover()

	var (
		oc               = exutil.NewCLIWithPodSecurityLevel("oc-policy", api.LevelRestricted)
		simpleDeployment = exutil.FixturePath("testdata", "deployments", "deployment-simple-sleep.yaml")
	)

	g.It("scc-subject-review, scc-review [apigroup:authorization.openshift.io][apigroup:user.openshift.io]", g.Label("Size:S"), func() {
		err := oc.Run("policy", "scc-subject-review").Execute()
		o.Expect(err).To(o.HaveOccurred())
		err = oc.Run("policy", "scc-review").Execute()
		o.Expect(err).To(o.HaveOccurred())
		err = oc.Run("policy", "scc-subject-review").Args("-u", "invalid", "--namespace", "noexist").Execute()
		o.Expect(err).To(o.HaveOccurred())

		out, err := oc.Run("policy", "scc-subject-review").Args("-z", "foo,bar", "-f", simpleDeployment).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("only one Service Account is supported"))

		out, err = oc.Run("policy", "scc-review").Args("-z", "default", "-f", simpleDeployment, "--namespace=noexist").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("cannot create resource \"podsecuritypolicyreviews\" in API group \"security.openshift.io\" in the namespace \"noexist\""))

		out, err = oc.Run("policy", "scc-subject-review").Args("-f", simpleDeployment, "-o=jsonpath={.status.allowedBy.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("restricted-v2"))
	})
})
