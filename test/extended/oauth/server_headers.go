package oauth

import (
	"fmt"
	"net/http"
	"net/url"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/util/sets"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/oauthserver"
)

var _ = g.Describe("[sig-auth][Feature:OAuthServer] [Headers][apigroup:route.openshift.io][apigroup:config.openshift.io][apigroup:oauth.openshift.io]", func() {
	var oc = exutil.NewCLIWithPodSecurityLevel("oauth-server-headers", admissionapi.LevelBaseline)
	var transport http.RoundTripper
	var oauthServerAddr string
	var oauthServerCleanup func()

	g.BeforeEach(func() {
		var err error

		transport, err = rest.TransportFor(rest.AnonymousClientConfig(oc.UserConfig()))
		o.Expect(err).ToNot(o.HaveOccurred())

		// deploy oauth server
		var newRequestTokenOptions oauthserver.NewRequestTokenOptionsFunc
		newRequestTokenOptions, oauthServerCleanup, err = deployOAuthServer(oc)
		o.Expect(err).ToNot(o.HaveOccurred(), "while attempting to deploy the oauth server")
		oauthServerAddr = newRequestTokenOptions("", "").Issuer
	})

	g.AfterEach(func() {
		oauthServerCleanup()
	})

	g.DescribeTable("expected headers returned from the",
		func(path string) {
			checkUrl, err := url.Parse(oauthServerAddr)
			o.Expect(err).ToNot(o.HaveOccurred())
			checkUrl.Path = path
			fmt.Fprintf(g.GinkgoWriter, "CheckUrl: %v\n", checkUrl)
			req, err := http.NewRequest("GET", checkUrl.String(), nil)
			o.Expect(err).ToNot(o.HaveOccurred())

			req.Header.Set("Accept", "text/html; charset=utf-8")
			resp, err := transport.RoundTrip(req)
			o.Expect(err).ToNot(o.HaveOccurred())

			allHeaders := http.Header{}
			for key, val := range map[string]string{
				// security related headers that we really care about, should not change
				"Cache-Control":          "no-cache, no-store, max-age=0, must-revalidate",
				"Pragma":                 "no-cache",
				"Expires":                "0",
				"Referrer-Policy":        "strict-origin-when-cross-origin",
				"X-Frame-Options":        "DENY",
				"X-Content-Type-Options": "nosniff",
				"X-DNS-Prefetch-Control": "off",
				"X-XSS-Protection":       "1; mode=block",

				// non-security headers, should not change
				// adding items here should be validated to make sure they do not conflict with any security headers
				// <no items currently>
			} {
				// use set so we get the canonical form of these headers
				allHeaders.Set(key, val)
			}

			// these headers can change per request and are not important to us
			// only add items to this list if they cannot be statically checked above
			ignoredHeaders := []string{"Audit-Id", "Date", "Content-Type", "Content-Length", "Location"}
			for _, h := range ignoredHeaders {
				resp.Header.Del(h)
			}

			// tolerate additional header set by osin library code
			expires := resp.Header["Expires"]
			if len(expires) == 2 && expires[1] == "Fri, 01 Jan 1990 00:00:00 GMT" {
				resp.Header["Expires"] = expires[:1]
			}

			// deduplicate headers (osin library code adds some duplicates)
			for k, vv := range resp.Header {
				resp.Header[k] = sets.NewString(vv...).List()
			}

			o.Expect(resp.Header).To(o.Equal(allHeaders))
		},
		g.Entry("root URL", g.Label("Size:S"), "/"),
		g.Entry("login URL for when there is only one IDP", g.Label("Size:S"), "/login"),
		g.Entry("login URL for the bootstrap IDP", g.Label("Size:S"), "/login/kube:admin"),
		g.Entry("login URL for the allow all IDP", g.Label("Size:S"), "/login/anypassword"),
		g.Entry("logout URL", g.Label("Size:S"), "/logout"),
		g.Entry("token URL", g.Label("Size:S"), "/oauth/token"),
		g.Entry("authorize URL", g.Label("Size:S"), "/oauth/authorize"),
		g.Entry("grant URL", g.Label("Size:S"), "/oauth/authorize/approve"),
		g.Entry("token request URL", g.Label("Size:S"), "/oauth/token/request"),
	)
})
