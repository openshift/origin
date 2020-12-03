package oauth

import (
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:LDAP] LDAP", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("oauth-ldap")
	)

	g.It("should start an OpenLDAP test server", func() {
		_, _, _, _, err := exutil.CreateLDAPTestServer(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	})
})
