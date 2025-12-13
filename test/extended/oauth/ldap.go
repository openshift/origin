package oauth

import (
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:LDAP] LDAP", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("oauth-ldap", admissionapi.LevelPrivileged)
	)

	g.It("should start an OpenLDAP test server [apigroup:user.openshift.io][apigroup:security.openshift.io][apigroup:authorization.openshift.io]", g.Label("Size:L"), func() {
		_, _, _, _, err := exutil.CreateLDAPTestServer(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
