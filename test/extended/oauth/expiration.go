package oauth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"golang.org/x/oauth2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	admissionapi "k8s.io/pod-security-admission/api"

	oauthv1 "github.com/openshift/api/oauth/v1"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/oauthserver"
)

var _ = g.Describe("[sig-auth][Feature:OAuthServer] [Token Expiration]", func() {
	var oc = exutil.NewCLIWithPodSecurityLevel("oauth-expiration", admissionapi.LevelBaseline)
	var newRequestTokenOptions oauthserver.NewRequestTokenOptionsFunc
	var oauthServerCleanup func()

	g.BeforeEach(func() {
		var err error
		newRequestTokenOptions, oauthServerCleanup, err = deployOAuthServer(oc)
		o.Expect(err).ToNot(o.HaveOccurred())
	})

	g.AfterEach(func() {
		oauthServerCleanup()
	})

	g.Context("Using a OAuth client with a non-default token max age [apigroup:oauth.openshift.io]", func() {
		var oAuthClientResource *oauthv1.OAuthClient
		var accessTokenMaxAgeSeconds int32

		g.JustBeforeEach(func() {
			var err error
			oAuthClientResource, err = oc.AdminOAuthClient().OauthV1().OAuthClients().Create(context.Background(), &oauthv1.OAuthClient{
				ObjectMeta:               metav1.ObjectMeta{Name: fmt.Sprintf("%s-%05d", oc.Namespace(), accessTokenMaxAgeSeconds)},
				RespondWithChallenges:    true,
				RedirectURIs:             []string{"http://localhost"},
				AccessTokenMaxAgeSeconds: &accessTokenMaxAgeSeconds,
				GrantMethod:              oauthv1.GrantHandlerAuto,
			}, metav1.CreateOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())
		})

		g.AfterEach(func() {
			oc.AdminOAuthClient().OauthV1().OAuthClients().Delete(context.Background(), oAuthClientResource.Name, metav1.DeleteOptions{})
		})

		g.Context("to generate tokens that do not expire", func() {

			g.BeforeEach(func() {
				accessTokenMaxAgeSeconds = 0
			})

			g.It("works as expected when using a token authorization flow [apigroup:user.openshift.io]", g.Label("Size:M"), func() {
				testTokenFlow(oc, newRequestTokenOptions, oAuthClientResource, accessTokenMaxAgeSeconds)
			})

			g.It("works as expected when using a code authorization flow [apigroup:user.openshift.io]", g.Label("Size:M"), func() {
				testCodeFlow(oc, newRequestTokenOptions, oAuthClientResource, accessTokenMaxAgeSeconds)
			})

		})
		g.Context("to generate tokens that expire shortly", func() {

			g.BeforeEach(func() {
				accessTokenMaxAgeSeconds = 10
			})

			g.It("works as expected when using a token authorization flow [apigroup:user.openshift.io]", g.Label("Size:M"), func() {
				testTokenFlow(oc, newRequestTokenOptions, oAuthClientResource, accessTokenMaxAgeSeconds)
			})
			g.It("works as expected when using a code authorization flow [apigroup:user.openshift.io]", g.Label("Size:M"), func() {
				testCodeFlow(oc, newRequestTokenOptions, oAuthClientResource, accessTokenMaxAgeSeconds)
			})
		})
	})
})

func testTokenFlow(oc *exutil.CLI, newRequestTokenOptions oauthserver.NewRequestTokenOptionsFunc, client *oauthv1.OAuthClient, expectedExpiresIn int32) {
	// new request token command
	requestTokenOptions := newRequestTokenOptions("testuser", "password")
	// setup for token flow
	requestTokenOptions.TokenFlow = true
	requestTokenOptions.OsinConfig.CodeChallenge = ""
	requestTokenOptions.OsinConfig.CodeChallengeMethod = ""
	requestTokenOptions.OsinConfig.CodeVerifier = ""
	// setup for non-default oauth client
	requestTokenOptions.OsinConfig.ClientId = client.Name
	requestTokenOptions.OsinConfig.RedirectUrl = client.RedirectURIs[0]
	// request token
	token, err := requestTokenOptions.RequestToken()
	o.Expect(err).ToNot(o.HaveOccurred())
	// Make sure we can use the token, and it represents who we expect
	userConfig := *rest.AnonymousClientConfig(oc.AdminConfig())
	userConfig.BearerToken = token
	userClient, err := userv1client.NewForConfig(&userConfig)
	o.Expect(err).ToNot(o.HaveOccurred())
	user, err := userClient.Users().Get(context.Background(), "~", metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(user.Name).To(o.Equal("testuser"))
	// Make sure the token exists with the overridden time
	tokenObj, err := oc.AdminOAuthClient().OauthV1().OAuthAccessTokens().Get(context.Background(), toTokenName(token), metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(tokenObj.ExpiresIn).To(o.BeNumerically("==", expectedExpiresIn))
}

func testCodeFlow(oc *exutil.CLI, newRequestTokenOptions oauthserver.NewRequestTokenOptionsFunc, client *oauthv1.OAuthClient, expectedExpiresIn int32) {
	anonymousClientConfig := rest.AnonymousClientConfig(oc.AdminConfig())
	rt, err := rest.TransportFor(anonymousClientConfig)
	o.Expect(err).ToNot(o.HaveOccurred())

	// need to extract the oauth server root url
	oauthServerURL := newRequestTokenOptions("", "").Issuer

	conf := &oauth2.Config{
		ClientID:     client.Name,
		ClientSecret: client.Secret,
		RedirectURL:  client.RedirectURIs[0],
		Endpoint: oauth2.Endpoint{
			AuthURL:  oauthServerURL + "/oauth/authorize",
			TokenURL: oauthServerURL + "/oauth/token",
		},
	}

	// get code
	req, err := http.NewRequest("GET", conf.AuthCodeURL(""), nil)
	o.Expect(err).ToNot(o.HaveOccurred())

	req.SetBasicAuth("testuser", "password")
	resp, err := rt.RoundTrip(req)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(resp.StatusCode).To(o.Equal(http.StatusFound))

	location, err := resp.Location()
	o.Expect(err).ToNot(o.HaveOccurred())

	code := location.Query().Get("code")
	o.Expect(code).ToNot(o.BeEmpty())

	// Make sure the code exists with the default time
	oauthClientSet := oc.AdminOAuthClient()
	codeObj, err := oauthClientSet.OauthV1().OAuthAuthorizeTokens().Get(context.Background(), toTokenName(code), metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(codeObj.ExpiresIn).To(o.BeNumerically("==", 5*60))

	// Use the custom HTTP client when requesting a token.
	httpClient := &http.Client{Transport: rt}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
	oauthToken, err := conf.Exchange(ctx, code)
	o.Expect(err).ToNot(o.HaveOccurred())
	token := oauthToken.AccessToken

	// Make sure we can use the token, and it represents who we expect
	userConfig := *anonymousClientConfig
	userConfig.BearerToken = token
	userClient, err := userv1client.NewForConfig(&userConfig)
	o.Expect(err).ToNot(o.HaveOccurred())

	user, err := userClient.Users().Get(context.Background(), "~", metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(user.Name).To(o.Equal("testuser"))

	// Make sure the token exists with the overridden time
	tokenObj, err := oauthClientSet.OauthV1().OAuthAccessTokens().Get(context.Background(), toTokenName(token), metav1.GetOptions{})
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(tokenObj.ExpiresIn).To(o.BeNumerically("==", expectedExpiresIn))
}

func toTokenName(token string) string {
	if strings.HasPrefix(token, "sha256~") {
		withoutPrefix := strings.TrimPrefix(token, "sha256~")
		h := sha256.Sum256([]byte(withoutPrefix))
		return "sha256~" + base64.RawURLEncoding.EncodeToString(h[0:])
	}
	return token
}
