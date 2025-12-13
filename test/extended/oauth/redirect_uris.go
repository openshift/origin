package oauth

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/RangelReale/osincli"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	configv1 "github.com/openshift/api/config/v1"
	oauthv1 "github.com/openshift/api/oauth/v1"

	exutil "github.com/openshift/origin/test/extended/util"
)

type testCaseRedirectURIs struct {
	client *oauthv1.OAuthClient
	uri    string

	expectedCode      int
	responseValidator func(*http.Request, *http.Response) bool
}

var _ = g.Describe("[sig-auth][Feature:OAuthServer] [apigroup:oauth.openshift.io]", func() {
	g.Describe("OAuthClientWithRedirectURIs", func() {
		ctx := context.Background()
		oc := exutil.NewCLI("oauthclient-with-redirect-uris")
		httpClient := &http.Client{
			Transport: &http.Transport{
				Proxy:           http.ProxyFromEnvironment,
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			// special case to not follow redirects; rather return most recent response
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}

		g.It("must validate request URIs according to oauth-client definition", g.Label("Size:M"), func() {
			g.By("check oauth-openshift routes")
			controlPlaneTopology, err := exutil.GetControlPlaneTopology(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(controlPlaneTopology).NotTo(o.BeNil())
			if *controlPlaneTopology == configv1.ExternalTopologyMode {
				e2eskipper.Skipf("External clusters do not contain oauth-openshift routes")
			}

			g.By("create oauth-clients")
			loopbackClient, err := generateRedirectURIOAuthClient(ctx, oc, "oauth-client-with-loopback-uris", []string{"http://127.0.0.1/callback", "http://[::1]/callback"})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(loopbackClient).NotTo(o.BeNil())
			nonLoopbackClient, err := generateRedirectURIOAuthClient(ctx, oc, "oauth-client-with-non-loopback-uris", []string{"https://www.redhat.com"})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(nonLoopbackClient).NotTo(o.BeNil())

			oauthDiscoveryData := getOAuthWellKnownData(oc)
			challenge, method, _, err := osincli.GeneratePKCE()
			o.Expect(err).NotTo(o.HaveOccurred())

			isRequestBodyValid := func(_ *http.Request, response *http.Response) bool {
				badRequestError := `{"error":"invalid_request","error_description":"The request is missing a required parameter, includes an invalid parameter value, includes a parameter more than once, or is otherwise malformed."}`
				body, err := ioutil.ReadAll(response.Body)
				return err == nil && strings.TrimSpace(string(body)) == badRequestError
			}

			isRedirectValid := func(request *http.Request, response *http.Response) bool {
				loc, err := response.Location()
				return err == nil && loc.Query().Get("then") == fmt.Sprintf("/oauth/authorize?%s", request.URL.RawQuery)
			}

			for i, t := range []testCaseRedirectURIs{
				// oauth client that accepts only loopback URIs with or without ports
				{loopbackClient, "https://www.redhat.com/callback", http.StatusBadRequest, isRequestBodyValid},
				{loopbackClient, "https://www.redhat.com:1234/callback", http.StatusBadRequest, isRequestBodyValid},
				{loopbackClient, "http://127.0.0.1/callback", http.StatusFound, isRedirectValid},
				{loopbackClient, "http://[::1]/callback", http.StatusFound, isRedirectValid},
				{loopbackClient, "http://127.0.0.1:1234/callback", http.StatusFound, isRedirectValid},
				{loopbackClient, "http://[::1]:1234/callback", http.StatusFound, isRedirectValid},
				// oauth client that accepts neither loopback URIs nor non-loopback URIs with ports
				{nonLoopbackClient, "https://www.redhat.com/callback", http.StatusFound, isRedirectValid},
				{nonLoopbackClient, "https://www.redhat.com:1234/callback", http.StatusBadRequest, isRequestBodyValid},
				{nonLoopbackClient, "http://127.0.0.1/callback", http.StatusBadRequest, isRequestBodyValid},
				{nonLoopbackClient, "http://[::1]/callback", http.StatusBadRequest, isRequestBodyValid},
				{nonLoopbackClient, "http://127.0.0.1:1234/callback", http.StatusBadRequest, isRequestBodyValid},
				{nonLoopbackClient, "http://[::1]:1234/callback", http.StatusBadRequest, isRequestBodyValid},
			} {
				g.By(fmt.Sprintf("test case #%d: {client=%s, endpoint=%s, URI=%s, expectedCode=%d}", i, t.client.ObjectMeta.Name, oauthDiscoveryData.AuthorizationEndpoint, t.uri, t.expectedCode))
				request, err := generateOAuthRequest(&t, challenge, method, oauthDiscoveryData.AuthorizationEndpoint)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(request).NotTo(o.BeNil())

				requestDump, err := httputil.DumpRequest(request, false)
				o.Expect(err).NotTo(o.HaveOccurred())
				framework.Logf("%v", string(requestDump))

				response, err := httpClient.Do(request)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(response.StatusCode).To(o.Equal(t.expectedCode))
				o.Expect(t.responseValidator(request, response)).To(o.BeTrue())
			}
		})
	})
})

func generateRedirectURIOAuthClient(ctx context.Context, oc *exutil.CLI, clientName string, uris []string) (*oauthv1.OAuthClient, error) {
	client := &oauthv1.OAuthClient{
		ObjectMeta:   metav1.ObjectMeta{Name: clientName},
		RedirectURIs: uris,
		GrantMethod:  oauthv1.GrantHandlerAuto,
	}

	created, err := oc.AdminOAuthClient().OauthV1().OAuthClients().Create(ctx, client, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	oc.AddResourceToDelete(oauthv1.GroupVersion.WithResource("oauthclients"), created)
	return client, nil
}

func generateOAuthRequest(t *testCaseRedirectURIs, challenge, method, endpoint string) (*http.Request, error) {
	request, err := http.NewRequest("POST",
		fmt.Sprintf("%s?client_id=%s&code_challenge=%s&code_challenge_method=%s&redirect_uri=%s&response_type=code",
			endpoint,
			t.client.ObjectMeta.Name,
			challenge,
			method,
			t.uri,
		), nil)

	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request, nil
}
