package oauth

import (
	"fmt"
	"net/http"
	"net/url"

	g "github.com/onsi/ginkgo"
	t "github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"

	"k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/util/sets"

	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/oauthserver"
)

var _ = g.Describe("[Feature:OAuthServer] [Headers]", func() {
	var oc = util.NewCLI("oauth-server-headers", util.KubeConfigPath())
	var transport http.RoundTripper
	var oauthServerAddr string
	var oauthServerCleanup func()

	g.BeforeEach(func() {
		var err error

		cfg, err := oauthserver.InjectRouterCA(oc, rest.AnonymousClientConfig(oc.UserConfig()))
		o.Expect(err).ToNot(o.HaveOccurred())
		transport, err = rest.TransportFor(cfg)
		o.Expect(err).ToNot(o.HaveOccurred())

		// deploy oauth server
		var newRequestTokenOptions oauthserver.NewRequestTokenOptionsFunc
		newRequestTokenOptions, oauthServerCleanup, err = deployOAuthServer(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
		oauthServerAddr = newRequestTokenOptions("", "").Issuer
	})

	g.AfterEach(func() {
		oauthServerCleanup()
	})

	t.DescribeTable("expected headers returned from the",
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
		t.Entry("root URL", "/"),
		t.Entry("login URL for when there is only one IDP", "/login"),
		t.Entry("login URL for the bootstrap IDP", "/login/kube:admin"),
		t.Entry("login URL for the allow all IDP", "/login/anypassword"),
		t.Entry("logout URL", "/logout"),
		t.Entry("token URL", "/oauth/token"),
		t.Entry("authorize URL", "/oauth/authorize"),
		t.Entry("grant URL", "/oauth/authorize/approve"),
		t.Entry("token request URL", "/oauth/token/request"),
	)
})
