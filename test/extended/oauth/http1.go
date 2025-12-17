package oauth

import (
	"net/http"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"golang.org/x/net/http2"

	"k8s.io/client-go/rest"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-auth][Feature:OAuthServer] OAuth server [apigroup:auth.openshift.io]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("oauth")

	g.It("should use http1.1 only to prevent http2 connection reuse", g.Label("Size:S"), func() {
		metadata := getOAuthWellKnownData(oc)

		tlsClientConfig, err := rest.TLSConfigFor(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		rt := http2.Transport{
			TLSClientConfig: tlsClientConfig,
		}

		req, err := http.NewRequest(http.MethodHead, metadata.Issuer, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		// there is no HTTP2 proxying implemented in golang, skip
		if url, _ := http.ProxyFromEnvironment(req); url != nil {
			g.Skip("this test does not run in proxied environment")
		}

		_, err = rt.RoundTrip(req)
		o.Expect(err).NotTo(o.BeNil(), "http2 only request to OAuth server should fail")
		o.Expect(err.Error()).To(
			o.Or(
				o.Equal(`http2: unexpected ALPN protocol ""; want "h2"`), // golang pre-1.17
				o.Equal(`remote error: tls: no application protocol`),    // golang 1.17+
			),
		)

	})
})
