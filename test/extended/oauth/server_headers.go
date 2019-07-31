package oauth

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	g "github.com/onsi/ginkgo"
	t "github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/util/sets"

	osinv1 "github.com/openshift/api/osin/v1"

	"github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/oauthserver"
)

var _ = g.Describe("[Feature:OAuthServer] [Headers]", func() {
	var (
		transport       http.RoundTripper
		oauthServerAddr string
		oc              *util.CLI
	)
	oc = util.NewCLI("oauth-server-headers", util.KubeConfigPath())

	g.BeforeEach(func() {
		var err error

		transport, err = rest.TransportFor(rest.AnonymousClientConfig(oc.UserConfig()))
		o.Expect(err).ToNot(o.HaveOccurred())

		// secret containing htpasswd "file"
		secret := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "htpasswd"},
			Data: map[string][]byte{
				"htpasswd": []byte("testuser:$2y$05$ZPH3KIKb3Gr86610n8y7Zuy5fZQUZhNl6dHqAYA6KWCqkacqYfw2i"),
			},
		}
		// provider config
		providerConfig, err := oauthserver.GetRawExtensionForOsinProvider(&osinv1.HTPasswdPasswordIdentityProvider{
			File: oauthserver.GetPathFromConfigMapSecretName(secret.Name, "htpasswd"),
		})
		o.Expect(err).ToNot(o.HaveOccurred())
		// identity provider
		identityProvider := osinv1.IdentityProvider{
			Name:            "htpasswd",
			MappingMethod:   "claim",
			Provider:        *providerConfig,
			UseAsChallenger: true,
			UseAsLogin:      true,
		}
		// deploy oauth server
		tokenOptions, oauthServerCleanup, err := oauthserver.DeployOAuthServer(oc, []osinv1.IdentityProvider{identityProvider}, nil, []corev1.Secret{secret})
		defer oauthServerCleanup()
		o.Expect(err).ToNot(o.HaveOccurred())
		oauthServerAddr = tokenOptions.Issuer
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

			// sometimes it takes a few seconds for the oauth server to be ready
			var resp *http.Response
			o.Eventually(func() error {
				var err error
				resp, err = transport.RoundTrip(req)
				return err
			}, 10*time.Second).ShouldNot(o.HaveOccurred())

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
