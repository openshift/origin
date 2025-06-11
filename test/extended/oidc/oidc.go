package oidc

import (
	g "github.com/onsi/ginkgo/v2"
)

var _ = g.Describe("[sig-auth][Serial][OCPFeatureGate:ExternalOIDC] Configuring an external OIDC provider", func() {
	defer g.GinkgoRecover()
	// oc := exutil.NewCLI("oidc")

	g.It("should rollout configuration on the kube-apiserver successfully", func() {
		g.Fail("not implemented")
	})

	g.It("should remove the OpenShift OAuth stack", func() {
		g.Fail("not implemented")
	})

	g.It("should not accept tokens provided by the OAuth server", func() {
		g.Fail("not implemented")
	})

	g.It("should accept tokens issued by the external IdP", func() {
		g.Fail("not implemented")
	})

	g.It("should accept authentication via a kubeconfig (break-glass)", func() {
		g.Fail("not implemented")
	})

	g.It("should map cluster identities correctly", func() {
		g.Fail("not implemented")
	})
})

var _ = g.Describe("[sig-auth][Serial][OCPFeatureGate:ExternalOIDC] Changing from OIDC authentication type to IntegratedOAuth", func() {
	defer g.GinkgoRecover()
	// oc := exutil.NewCLI("oidc")

	g.It("should rollout configuration on the kube-apiserver successfully", func() {
		g.Fail("not implemented")
	})

	g.It("should rollout the OpenShift OAuth stack", func() {
		g.Fail("not implemented")
	})

	g.It("should not accept tokens provided by an external IdP", func() {
		g.Fail("not implemented")
	})

	g.It("should accept tokens provided by the OpenShift OAuth server", func() {
		g.Fail("not implemented")
	})
})

// TODO: Add test skeleton for the ExternalOIDCWithUIDAndExtraClaimMappings feature gate
