package oauth

import (
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	"github.com/openshift/library-go/pkg/oauth/oauthdiscovery"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/oauthserver"
)

var _ = g.Describe("[Feature:OAuthServer] well-known endpoint", func() {
	defer g.GinkgoRecover()
	var (
		oc             = exutil.NewCLI("oauth-well-known", exutil.KubeConfigPath())
		oauthRoute     = "oauth-openshift"
		oauthNamespace = "openshift-authentication"
	)

	g.It("should be reachable", func() {
		metadataJSON, err := oc.Run("get").Args("--raw", "/.well-known/oauth-authorization-server").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		metadata := &oauthdiscovery.OauthAuthorizationServerMetadata{}
		err = json.Unmarshal([]byte(metadataJSON), metadata)
		o.Expect(err).NotTo(o.HaveOccurred())
		// compare to openshift-authentication route
		route, err := oc.AdminRouteClient().RouteV1().Routes(oauthNamespace).Get(oauthRoute, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		u, err := url.Parse("https://" + route.Spec.Host)
		o.Expect(err).NotTo(o.HaveOccurred())
		u.Path = path.Join(u.Path, "oauth/authorize")
		authEndpointFromRoute := u.String()
		o.Expect(metadata.AuthorizationEndpoint).To(o.Equal(authEndpointFromRoute))
		tlsClientConfig, err := rest.TLSConfigFor(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		rt := http.Transport{
			TLSClientConfig: tlsClientConfig,
		}

		cert, err := oauthserver.GetRouterCA(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(cert); !ok {
			o.Expect(errors.New("failed to add server CA certificates to client pool")).NotTo(o.HaveOccurred())
		}

		rt.TLSClientConfig.RootCAs = pool

		req, err := http.NewRequest(http.MethodHead, metadata.Issuer, nil)
		o.Expect(err).NotTo(o.HaveOccurred())

		resp, err := rt.RoundTrip(req)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(resp.StatusCode).To(o.Equal(200))
	})
})
