package router

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"golang.org/x/net/http2"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/url"
)

const (
	// http2ClientGetTimeout specifies the time limit for requests
	// made by the HTTP Client.
	http2ClientTimeout = 1 * time.Minute
)

func makeHTTPClient(useHTTP2Transport bool, timeout time.Duration) *http.Client {
	tlsConfig := tls.Config{
		InsecureSkipVerify: true,
	}

	c := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tlsConfig,
		},
	}

	if useHTTP2Transport {
		c.Transport = &http2.Transport{
			TLSClientConfig: &tlsConfig,
		}
	}

	return c
}

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var (
		configPath = exutil.FixturePath("testdata", "router", "router-http2.yaml")
		oc         = exutil.NewCLI("router-http2")
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(oc.KubeFramework().Namespace.Name)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("http2", oc)
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should pass the http2 tests", func() {
			g.By(fmt.Sprintf("creating test fixture from a config file %q", configPath))
			err := oc.Run("new-app").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.ExpectNoError(oc.KubeFramework().WaitForPodRunningSlow("http2"))

			testCases := []struct {
				route             string
				frontendProto     string
				backendProto      string
				statusCode        int
				useHTTP2Transport bool
				expectedGetError  string
			}{{
				route:             "http2-custom-cert-edge",
				frontendProto:     "HTTP/2.0",
				backendProto:      "HTTP/1.1",
				statusCode:        http.StatusOK,
				useHTTP2Transport: true,
			}, {
				route:             "http2-custom-cert-reencrypt",
				frontendProto:     "HTTP/2.0",
				backendProto:      "HTTP/2.0",
				statusCode:        http.StatusOK,
				useHTTP2Transport: true,
			}, {
				route:             "http2-passthrough",
				frontendProto:     "HTTP/2.0",
				backendProto:      "HTTP/2.0",
				statusCode:        http.StatusOK,
				useHTTP2Transport: true,
			}, {
				route:             "http2-default-cert-edge",
				useHTTP2Transport: true,
				expectedGetError:  `http2: unexpected ALPN protocol ""; want "h2"`,
			}, {
				route:             "http2-default-cert-reencrypt",
				useHTTP2Transport: true,
				expectedGetError:  `http2: unexpected ALPN protocol ""; want "h2"`,
			}, {
				route:             "http2-custom-cert-edge",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/1.1",
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}, {
				route:             "http2-custom-cert-reencrypt",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/2.0", // reencrypt always has backend ALPN enabled
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}, {
				route:             "http2-passthrough",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/1.1",
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}, {
				route:             "http2-default-cert-edge",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/1.1",
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}, {
				route:             "http2-default-cert-reencrypt",
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/2.0", // reencrypt always has backend ALPN enabled
				statusCode:        http.StatusOK,
				useHTTP2Transport: false,
			}}

			for i, tc := range testCases {
				testConfig := fmt.Sprintf("%+v", tc)
				e2e.Logf("[test #%d/%d]: config: %s", i+1, len(testCases), testConfig)

				// check readiness probe is accessible
				urlTester := url.NewTester(oc.AdminKubeClient(), oc.KubeFramework().Namespace.Name).WithErrorPassthrough(true)
				defer urlTester.Close()
				hostname := getHostnameForRoute(oc, tc.route)
				urlTester.Within(30*time.Second, url.Expect("GET", "https://"+hostname+"/healthz").Through(hostname).SkipTLSVerification().HasStatusCode(200))

				var resp *http.Response
				client := makeHTTPClient(tc.useHTTP2Transport, http2ClientTimeout)

				o.Expect(wait.Poll(time.Second, 30*time.Second, func() (bool, error) {
					var err error
					url := "https://" + hostname
					resp, err = client.Get(url)
					if err != nil && len(tc.expectedGetError) != 0 {
						errMatch := strings.Contains(err.Error(), tc.expectedGetError)
						if !errMatch {
							e2e.Logf("[test #%d/%d]: config: %s, GET error: %v", i+1, len(testCases), testConfig, err)
						}
						return errMatch, nil
					}
					if err != nil {
						e2e.Logf("[test #%d/%d]: config: %s, GET error: %v", i+1, len(testCases), testConfig, err)
						return false, nil // could be 503 if service not ready
					}
					if tc.statusCode == 0 {
						return false, nil
					}
					if resp.StatusCode != tc.statusCode {
						e2e.Logf("[test #%d/%d]: config: %s, expected status: %v, actual status: %v", i+1, len(testCases), testConfig, tc.statusCode, resp.StatusCode)
					}
					return resp.StatusCode == tc.statusCode, nil
				})).NotTo(o.HaveOccurred())

				if tc.expectedGetError != "" {
					continue
				}

				o.Expect(resp.StatusCode).To(o.Equal(tc.statusCode), testConfig)
				o.Expect(resp.Proto).To(o.Equal(tc.frontendProto), testConfig)
				body, err := ioutil.ReadAll(resp.Body)
				o.Expect(err).NotTo(o.HaveOccurred(), testConfig)
				o.Expect(string(body)).To(o.Equal(tc.backendProto), testConfig)
				o.Expect(resp.Body.Close()).NotTo(o.HaveOccurred())
			}
		})
	})
})
