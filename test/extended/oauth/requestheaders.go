package oauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"time"

	"github.com/davecgh/go-spew/spew"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	configv1 "github.com/openshift/api/config/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	clusteroperatorhelpers "github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/ibmcloud"
)

func init() {
	utilruntime.Must(osinv1.Install(scheme))
}

const (
	clientCAName      = "test-client-ca"
	clientCorrectName = "testing-client-cert"
	clientWrongName   = "wrong-client-cert"

	testUserName = "franta"
	idpName      = "test-request-header"
)

type certAuthTest struct {
	name          string
	cert          *x509.Certificate
	key           *rsa.PrivateKey
	endpoint      string
	expectToken   bool
	expectedError string
}

var _ = g.Describe("[Serial] [sig-auth][Feature:OAuthServer] [RequestHeaders] [IdP]", func() {
	var oc = exutil.NewCLI("request-headers")

	g.It("test RequestHeaders IdP", func() {
		if e2e.TestContext.Provider == ibmcloud.ProviderName {
			e2eskipper.Skipf("IBM ROKS clusters do not allow customization of the Identity Providers for the cluster.")
		}

		caCert, caKey := createClientCA(oc.AdminKubeClient().CoreV1())
		defer oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config").Delete(context.Background(), clientCAName, metav1.DeleteOptions{})

		changeTime := time.Now()
		oauthClusterOrig, err := oc.AdminConfigClient().ConfigV1().OAuths().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		oauthCluster := oauthClusterOrig.DeepCopy()
		oauthCluster.Spec.IdentityProviders = []configv1.IdentityProvider{
			{
				Name: idpName,
				IdentityProviderConfig: configv1.IdentityProviderConfig{
					Type: configv1.IdentityProviderTypeRequestHeader,
					RequestHeader: &configv1.RequestHeaderIdentityProvider{
						ClientCA: configv1.ConfigMapNameReference{
							Name: clientCAName,
						},
						ClientCommonNames:        []string{"A good cert", clientCorrectName, "Some other cert"},
						Headers:                  []string{"X-Remote-User"},
						EmailHeaders:             []string{},
						NameHeaders:              []string{},
						PreferredUsernameHeaders: []string{},
						LoginURL:                 "https://dontcare.com/web-login/oauth/authorize?${query}",
						ChallengeURL:             "https://dontcare.com/challenges/oauth/authorize?${query}",
					},
				},
			},
		}
		_, err = oc.AdminConfigClient().ConfigV1().OAuths().Update(context.Background(), oauthCluster, metav1.UpdateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		// clean up after ourselves
		defer func() {
			userclient := oc.AdminUserClient().UserV1()
			userclient.Identities().Delete(context.Background(), fmt.Sprintf("%s:%s", idpName, testUserName), metav1.DeleteOptions{})
			userclient.Users().Delete(context.Background(), testUserName, metav1.DeleteOptions{})

			oauthCluster, err := oc.AdminConfigClient().ConfigV1().OAuths().Get(context.Background(), "cluster", metav1.GetOptions{})
			if err != nil {
				g.Fail(fmt.Sprintf("Failed to get oauth/cluster, unable to turn it into its original state: %v", err))
			}
			oauthCluster.Spec = oauthClusterOrig.Spec
			_, err = oc.AdminConfigClient().ConfigV1().OAuths().Update(context.Background(), oauthCluster, metav1.UpdateOptions{})
			if err != nil {
				g.Fail(fmt.Sprintf("Failed to update oauth/cluster, unable to turn it into its original state: %v", err))
			}
		}()

		oauthURL := getOAuthWellKnownData(oc).Issuer
		goodCert, goodKey := generateCert(caCert, caKey, clientCorrectName, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

		badNameCert, badNameKey := generateCert(caCert, caKey, clientWrongName, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

		caCert2, caKey2 := generateCA("The Other Testing CA")
		unknownCACert, unknownCAKey := generateCert(caCert2, caKey2, clientCorrectName, nil)

		testCases := []certAuthTest{
			{
				name:        "/authorize - challenging-client - valid cert: we should eventually get access token in Location header",
				cert:        goodCert,
				key:         goodKey,
				endpoint:    "/oauth/authorize?client_id=openshift-challenging-client&response_type=token",
				expectToken: true,
			},
			{
				name:          "/authorize - challenging-client - unknown CA cert: expect 302 because we never get authenticated",
				cert:          unknownCACert,
				key:           unknownCAKey,
				endpoint:      "/oauth/authorize?client_id=openshift-challenging-client&response_type=token",
				expectedError: "302 Found",
			},
			{
				name:          "/authorize - challenging-client - wrong CN cert: expect 500 because the verifier can generally return TLS errors :(",
				cert:          badNameCert,
				key:           badNameKey,
				endpoint:      "/oauth/authorize?client_id=openshift-challenging-client&response_type=token",
				expectedError: "500 Internal Server Error",
			},
			{
				name:          "/metrics - anonymous: should not be publicly visible",
				endpoint:      "/metrics",
				expectedError: "403 Forbidden",
			},
			{
				name:          "/metrics - valid cert: kube x509 authenticator is used so the user still ends up unauthenticated",
				cert:          goodCert,
				key:           goodKey,
				endpoint:      "/metrics",
				expectedError: "401 Unauthorized",
			},
			{
				name:          "/metrics - unknown CA cert: unauthenticated users should not be able to access it",
				cert:          unknownCACert,
				key:           unknownCAKey,
				endpoint:      "/metrics",
				expectedError: "403 Forbidden",
			},
			{
				name:     "/healtz - anonymous: anyone should be able to access it",
				endpoint: "/healthz",
			},
			{
				name:     "/healthz - valid cert",
				cert:     goodCert,
				key:      goodKey,
				endpoint: "/healthz",
			},
			{
				name:     "/healthz - unknown CA cert",
				cert:     unknownCACert,
				key:      unknownCAKey,
				endpoint: "/healthz",
			},
		}

		// add the route CA cert to the system bundle to trust the OAuth server
		caCerts, err := x509.SystemCertPool()
		o.Expect(err).NotTo(o.HaveOccurred())

		routerCA, err := oc.AdminKubeClient().CoreV1().ConfigMaps("openshift-config-managed").Get(context.Background(), "default-ingress-cert", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, ca := range routerCA.Data {
			ok := caCerts.AppendCertsFromPEM([]byte(ca))
			o.Expect(ok).To(o.Equal(true), "adding router certs to the system CA bundle")
		}

		waitForNewOAuthConfig(oc, caCerts, oauthURL, changeTime)

		for _, tc := range testCases {
			g.By(tc.name, func() {
				resp := oauthHTTPRequestOrFail(caCerts, oauthURL, tc.endpoint, "", tc.cert, tc.key)
				respDump, err := httputil.DumpResponse(resp, false)
				o.Expect(err).NotTo(o.HaveOccurred())
				if len(tc.expectedError) == 0 && resp.StatusCode != 200 && resp.StatusCode != 302 {
					g.Fail(fmt.Sprintf("unexpected error response status (%d) while trying to reach '%s' endpoint: %s", resp.StatusCode, tc.endpoint, respDump))
				} else if len(tc.expectedError) > 0 {
					o.Expect(resp.Status).To(o.ContainSubstring(tc.expectedError), fmt.Sprintf("full response header: %s\n", respDump))
				}

				token := getTokenFromResponse(resp)
				if len(token) > 0 && !tc.expectToken {
					g.Fail("did not expect to get a token")
				}
				if len(token) == 0 && tc.expectToken {
					g.Fail(fmt.Sprintf("Location header does not contain the access token: '%s'", resp.Header.Get("Location")))
				}

				if tc.expectToken {
					testEndpointsWithValidToken(caCerts, oauthURL, token)
				}
			})
		}

		testBrowserClientRedirectsProperly(caCerts, oauthURL)
	})
})

func testEndpointsWithValidToken(caCerts *x509.CertPool, oauthServerURL, token string) {
	g.By("/metrics - token: requires user authorized to access the endpoint", func() {
		testedEndpoint := "/metrics"
		resp := oauthHTTPRequestOrFail(caCerts, oauthServerURL, testedEndpoint, token, nil, nil)
		respDump, err := httputil.DumpResponse(resp, false)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(resp.StatusCode).To(o.Equal(403), fmt.Sprintf("full response header: %s\n", respDump))
	})

	g.By("/healthz - token: should be accessible to anyone", func() {
		testedEndpoint := "/healthz"
		resp := oauthHTTPRequestOrFail(caCerts, oauthServerURL, testedEndpoint, token, nil, nil)
		o.Expect(resp.StatusCode).To(o.Equal(200))
	})
}

func testBrowserClientRedirectsProperly(caCerts *x509.CertPool, oauthServerURL string) {
	g.By("/authorize - browser-client - anonymous: anonymous users are redirected to console login page to authenticate", func() {
		testedEndpoint := "/oauth/authorize?client_id=openshift-browser-client&response_type=token"
		resp := oauthHTTPRequestOrFail(caCerts, oauthServerURL, testedEndpoint, "", nil, nil)
		respDump, err := httputil.DumpResponse(resp, false)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(resp.StatusCode).To(o.Equal(200), fmt.Sprintf("full response header: %s\n", respDump))
		respBody, err := ioutil.ReadAll(resp.Body)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(respBody)).To(o.ContainSubstring("<a href=\"/oauth/authorize?client_id=openshift-browser-client&amp;idp=test-request-header"))
	})

	g.By("/token/request - browser-client - anonymous: users are redirected to console login page to authenticate", func() {
		testedEndpoint := "/oauth/token/request"
		resp := oauthHTTPRequestOrFail(caCerts, oauthServerURL, testedEndpoint, "", nil, nil)
		respDump, err := httputil.DumpResponse(resp, false)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(resp.StatusCode).To(o.Equal(200), fmt.Sprintf("full response header: %s\n", respDump))
		respBody, err := ioutil.ReadAll(resp.Body)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(string(respBody)).To(o.ContainSubstring("<a href=\"/oauth/authorize?client_id=openshift-browser-client&amp;idp=test-request-header"))
	})

	g.By("/authorize - browser-client - anonymous: specify the request header provider in the query", func() {
		testedEndpoint := "/oauth/authorize?client_id=openshift-browser-client&response_type=token;idp=test-request-header"
		resp := oauthHTTPRequestOrFail(caCerts, oauthServerURL, testedEndpoint, "", nil, nil)
		respDump, err := httputil.DumpResponse(resp, false)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(resp.StatusCode).To(o.Equal(302), fmt.Sprintf("full response header: %s\n", respDump))
		o.Expect(resp.Header.Get("Location")).To(o.ContainSubstring("https://dontcare.com/web-login/oauth/authorize"))
	})
}

func generateCA(cn string) (*x509.Certificate, *rsa.PrivateKey) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	o.Expect(err).NotTo(o.HaveOccurred())

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			Organization: []string{"Testing Org"},
			Country:      []string{"Faraway"},
			CommonName:   cn,
		},
		NotBefore:             time.Now().AddDate(0, 0, -5),
		NotAfter:              time.Now().AddDate(2, 0, 0),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	pub := &priv.PublicKey
	caCertBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, pub, priv)
	o.Expect(err).NotTo(o.HaveOccurred())

	caCert, err := x509.ParseCertificate(caCertBytes)
	o.Expect(err).NotTo(o.HaveOccurred())

	return caCert, priv
}

// createClientCA creates a CA and adds its cert to a CM in openshift-config NS
// returns CA cert and private key
func createClientCA(client corev1client.CoreV1Interface) (*x509.Certificate, *rsa.PrivateKey) {
	caCert, caKey := generateCA("Testing CA")
	_, err := client.ConfigMaps("openshift-config").Create(context.Background(), &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: clientCAName,
		},
		Data: map[string]string{
			"ca.crt": string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})),
		},
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())

	return caCert, caKey
}

func generateCert(caCert *x509.Certificate, caKey *rsa.PrivateKey, cn string, ekus []x509.ExtKeyUsage) (*x509.Certificate, *rsa.PrivateKey) {
	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1653),
		Subject: pkix.Name{
			Organization: []string{"Testing Org"},
			Country:      []string{"Faraway"},
			CommonName:   cn,
		},
		NotBefore:             time.Now().AddDate(0, 0, -1),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		IsCA:                  false,
		ExtKeyUsage:           ekus,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	o.Expect(err).NotTo(o.HaveOccurred())

	pub := &priv.PublicKey
	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, caCert, pub, caKey)
	o.Expect(err).NotTo(o.HaveOccurred())

	cert, err := x509.ParseCertificate(certBytes)
	o.Expect(err).NotTo(o.HaveOccurred())

	return cert, priv
}

func waitForNewOAuthConfig(oc *exutil.CLI, caCerts *x509.CertPool, oauthURL string, configChanged time.Time) {
	// check that the pods running in openshift-authentication NS already reflect our changes
	err := wait.PollImmediate(time.Second, 2*time.Minute, func() (bool, error) {
		pods, err := oc.AdminKubeClient().CoreV1().Pods("openshift-authentication").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		podsReady := true
		for _, p := range pods.Items {
			tstamp := p.GetCreationTimestamp()
			if !tstamp.After(configChanged) {
				podsReady = false
			}
			if podsReady {
				return true, nil
			}
		}
		return false, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	err = wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		authn, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "authentication", metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Error getting authentication operator: %v", err)
			return false, err
		}

		if clusteroperatorhelpers.IsStatusConditionTrue(authn.Status.Conditions, configv1.OperatorProgressing) {
			e2e.Logf("Waiting for progressing condition: %s", spew.Sdump(authn.Status.Conditions))
			return false, nil
		}

		// it seems that if we do anonymous request too early, it still does not see the IdP as configured
		resp, err := oauthHTTPRequest(caCerts, oauthURL, "/oauth/authorize?client_id=openshift-challenging-client&response_type=token", "", nil, nil)
		if err != nil {
			e2e.Logf("Error making OAuth request: %v", err)
			return false, nil
		}

		if resp.StatusCode != 302 {
			bodyBytes, _ := ioutil.ReadAll(resp.Body)
			e2e.Logf("OAuth HTTP request response is not 302: %q (%s)", resp.Status, string(bodyBytes))
			return false, nil
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
}

// oauthHTTPRequestOrFail wraps oauthHTTPRequest and fails the test if the request failed
func oauthHTTPRequestOrFail(caCerts *x509.CertPool, oauthBaseURL, endpoint, token string, cert *x509.Certificate, key *rsa.PrivateKey) *http.Response {
	resp, err := oauthHTTPRequest(caCerts, oauthBaseURL, endpoint, token, cert, key)
	o.Expect(err).NotTo(o.HaveOccurred())

	return resp
}

// oauthHTTPRequest returns token or encountered error should it receive Unauthorized or any other error
// This function can still Fail() the test in case its arguments are invalid/some basic stdlib functions fail.
func oauthHTTPRequest(caCerts *x509.CertPool, oauthBaseURL, endpoint, token string, cert *x509.Certificate, key *rsa.PrivateKey) (*http.Response, error) {
	if (cert == nil) != (key == nil) { // YOU MONSTER!
		g.Fail("must either specify both key and cert, or neither")
	}

	req, err := http.NewRequest(http.MethodGet, oauthBaseURL+endpoint, nil)
	o.Expect(err).NotTo(o.HaveOccurred())

	if len(token) == 0 {
		// requesting a token, set user header
		req.Header.Set("X-Remote-User", testUserName)
	} else {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			RootCAs: caCerts,
		},
	}

	if cert != nil {
		// we'll be doing client cert auth
		certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
		keyBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

		tlsCert, err := tls.X509KeyPair(certBytes, keyBytes)

		o.Expect(err).NotTo(o.HaveOccurred())
		transport.TLSClientConfig.Certificates = []tls.Certificate{tlsCert}
	}

	oauthServerURL, err := url.Parse(oauthBaseURL)
	o.Expect(err).NotTo(o.HaveOccurred())

	outsideClusterError := fmt.Errorf("don't try to reach outside the cluster")
	tokenFoundError := fmt.Errorf("token found")
	httpClient := http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// we're either querying the oauth server itself
			if req.URL.Hostname() != oauthServerURL.Hostname() {
				return outsideClusterError
			}

			if match := regexp.MustCompile("access_token").MatchString(req.URL.Fragment); match {
				return tokenFoundError
			}
			return nil
		},
	}

	resp, err := httpClient.Do(req)
	if urlErr, ok := err.(*url.Error); ok {
		switch urlErr.Err {
		case tokenFoundError, outsideClusterError:
			err = nil // these are our own expected errors
		}
	}

	return resp, err
}

func getTokenFromResponse(resp *http.Response) string {
	locationHeader := resp.Header.Get("Location")
	locationTokenRegexp := regexp.MustCompile("access_token=([^&]*)")

	if matches := locationTokenRegexp.FindStringSubmatch(locationHeader); len(matches) > 1 {
		return matches[1]
	}

	return ""
}
