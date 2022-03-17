package oauth

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"

	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"
	userv1 "github.com/openshift/api/user/v1"

	exutil "github.com/openshift/origin/test/extended/util"

	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

var authzToken, sha256AuthzToken = exutil.GenerateOAuthTokenPair()

var _ = g.Describe("[sig-auth][Feature:OAuthServer]", func() {
	defer g.GinkgoRecover()
	oc := exutil.NewCLI("oauthclient-secret-with-plus")
	ctx := context.Background()

	g.Describe("ClientSecretWithPlus", func() {
		g.It(fmt.Sprintf("should create oauthclient"), func() {
			controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				e2eskipper.Skipf("External clusters do not contain oauth-openshift routes")
			}

			g.By("create oauth client")
			oauthClient, err := oc.AdminOauthClient().OauthV1().OAuthClients().Create(ctx, &oauthv1.OAuthClient{
				ObjectMeta: metav1.ObjectMeta{
					Name: "oauth-client-with-plus",
				},
				Secret:            "secret+with+plus",
				RedirectURIs:      []string{"https://www.google.com"},
				GrantMethod:       oauthv1.GrantHandlerAuto,
				ScopeRestrictions: []oauthv1.ScopeRestriction{{ExactValues: []string{"user:full"}}},
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthclients"), oauthClient)

			g.By("create synthetic user")

			user, err := oc.AdminUserClient().UserV1().Users().Create(ctx, &userv1.User{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name: "user-for-plus",
				},
				FullName: "fake user",
			}, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			oc.AddResourceToDelete(userv1.GroupVersion.WithResource("users"), user)

			oauthDiscoveryData := getOAuthWellKnownData(oc)

			for _, createTokenReq := range []func(string) []*http.Request{
				queryClientAuthRequest,
				bodyClientAuthRequest,
				headerClientAuthRequest,
			} {
				g.By("create synthetic authz token")
				oauthAuthorizeToken, err := oc.AdminOauthClient().OauthV1().OAuthAuthorizeTokens().Create(ctx, &oauthv1.OAuthAuthorizeToken{
					ObjectMeta: metav1.ObjectMeta{
						Name: sha256AuthzToken,
					},
					ClientName:          oauthClient.Name,
					ExpiresIn:           100000000,
					RedirectURI:         "https://www.google.com",
					Scopes:              []string{"user:full"},
					UserName:            user.Name,
					UserUID:             string(user.UID),
					CodeChallenge:       "",
					CodeChallengeMethod: "",
				}, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
				oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthauthorizetokens"), oauthAuthorizeToken)

				g.By("querying for a token")

				var reqErr []error
				var tokenResponse *http.Response
				for _, tokenReq := range createTokenReq(oauthDiscoveryData.TokenEndpoint) {
					tokenReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					requestDump, err := httputil.DumpRequest(tokenReq, true)

					o.Expect(err).NotTo(o.HaveOccurred())
					framework.Logf("%v", string(requestDump))

					// we don't really care if this URL is safe
					tr := &http.Transport{
						Proxy:           http.ProxyFromEnvironment,
						TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
					}
					client := &http.Client{Transport: tr}

					tokenResponse, err = client.Do(tokenReq)
					// if there was an error then append the error and continue
					if err != nil {
						reqErr = append(reqErr, err)
						continue
					}
					// if there was no error then reset the error array and break
					reqErr = nil
					break
				}
				o.Expect(reqErr).To(o.BeNil())
				response, err := httputil.DumpResponse(tokenResponse, true)
				o.Expect(err).NotTo(o.HaveOccurred())
				framework.Logf("%v", string(response))
				o.Expect(tokenResponse.StatusCode).To(o.Equal(http.StatusOK))
			}
		})
	})
})

func queryClientAuthRequest(tokenEndpoint string) []*http.Request {
	req, err := http.NewRequest("POST", tokenEndpoint+"?"+
		"grant_type=authorization_code&"+
		"code="+authzToken+"&"+
		"client_id=oauth-client-with-plus&"+
		"client_secret=secret%2Bwith%2Bplus",
		nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	return []*http.Request{req}
}

func bodyClientAuthRequest(tokenEndpoint string) []*http.Request {
	req, err := http.NewRequest("POST", tokenEndpoint,
		bytes.NewBufferString("grant_type=authorization_code&"+
			"code="+authzToken+"&"+
			"client_id=oauth-client-with-plus&"+
			"client_secret=secret%2Bwith%2Bplus",
		),
	)
	o.Expect(err).NotTo(o.HaveOccurred())

	return []*http.Request{req}
}

func headerClientAuthRequest(tokenEndpoint string) []*http.Request {
	reqNoURLEscape, err := http.NewRequest("POST", tokenEndpoint,
		bytes.NewBufferString("grant_type=authorization_code&"+
			"code="+authzToken,
		),
	)
	o.Expect(err).NotTo(o.HaveOccurred())
	reqNoURLEscape.SetBasicAuth("oauth-client-with-plus", "secret+with+plus")

	reqURLEscape, err := http.NewRequest("POST", tokenEndpoint,
		bytes.NewBufferString("grant_type=authorization_code&"+
			"code="+authzToken,
		),
	)
	o.Expect(err).NotTo(o.HaveOccurred())
	reqURLEscape.SetBasicAuth("oauth-client-with-plus", "secret%2Bwith%2Bplus")
	return []*http.Request{reqNoURLEscape, reqURLEscape}
}
