package oauth

import (
	"encoding/json"
	"net/http"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"golang.org/x/net/http2"

	"k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:OAuthServer] OAuth server", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLI("oauth", exutil.KubeConfigPath())

	g.It("should use http1.1 only to prevent http2 connection reuse", func() {
		metadataJSON, err := oc.Run("get").Args("--raw", "/.well-known/oauth-authorization-server").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		metadata := &oauthdiscovery.OauthAuthorizationServerMetadata{}
		err = json.Unmarshal([]byte(metadataJSON), metadata)
		o.Expect(err).NotTo(o.HaveOccurred())

		tlsClientConfig, err := rest.TLSConfigFor(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		rt := http2.Transport{
			TLSClientConfig: tlsClientConfig,
		}

		req, err := http.NewRequest(http.MethodHead, metadata.Issuer, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = rt.RoundTrip(req)
		o.Expect(err).NotTo(o.BeNil(), "http2 only request to OAuth server should fail")
		o.Expect(err.Error()).To(o.Equal(`http2: unexpected ALPN protocol ""; want "h2"`))
	})
})
