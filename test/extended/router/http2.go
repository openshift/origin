package router

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	"golang.org/x/net/http2"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	configv1 "github.com/openshift/api/config/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"

	"github.com/openshift/origin/test/extended/router/certgen"
	"github.com/openshift/origin/test/extended/router/shard"
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
		http2ServiceConfigPath     = exutil.FixturePath("testdata", "router", "router-http2.yaml")
		http2RoutesConfigPath      = exutil.FixturePath("testdata", "router", "router-http2-routes.yaml")
		http2RouterShardConfigPath = exutil.FixturePath("testdata", "router", "router-shard.yaml")

		oc = exutil.NewCLI("router-http2")

		shardConfigPath string // computed
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
			exutil.DumpPodLogsStartingWithInNamespace("router", "openshift-ingress", oc.AsAdmin())
		}
		if len(shardConfigPath) > 0 {
			if err := oc.AsAdmin().Run("delete").Args("-n", "openshift-ingress-operator", "-f", shardConfigPath).Execute(); err != nil {
				e2e.Logf("deleting ingress controller failed: %v\n", err)
			}
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should pass the http2 tests", func() {
			isProxyJob, err := exutil.IsClusterProxyEnabled(oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get proxy configuration")
			if isProxyJob {
				g.Skip("Skip on proxy jobs")
			}

			infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster-wide infrastructure")
			if !platformHasHTTP2LoadBalancerService(infra.Status.PlatformStatus.Type) {
				g.Skip("Skip on platforms where the default router is not exposed by a load balancer service.")
			}

			defaultDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")

			g.By("Locating the canary image reference")
			image, err := getCanaryImage(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating http2 test service")
			err = oc.Run("new-app").Args("-f", http2ServiceConfigPath, "-p", "IMAGE="+image).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Waiting for http2 pod to be running")
			e2e.ExpectNoError(e2epod.WaitForPodRunningInNamespaceSlow(oc.KubeClient(), "http2", oc.KubeFramework().Namespace.Name))

			// certificate start and end time are very
			// lenient to avoid any clock drift between
			// the test machine and the cluster under
			// test.
			notBefore := time.Now().Add(-24 * time.Hour)
			notAfter := time.Now().Add(24 * time.Hour)

			// Generate crt/key for routes that need them.
			_, tlsCrtData, tlsPrivateKey, err := certgen.GenerateKeyPair(notBefore, notAfter)
			o.Expect(err).NotTo(o.HaveOccurred())

			derKey, err := certgen.MarshalPrivateKeyToDERFormat(tlsPrivateKey)
			o.Expect(err).NotTo(o.HaveOccurred())

			pemCrt, err := certgen.MarshalCertToPEMString(tlsCrtData)
			o.Expect(err).NotTo(o.HaveOccurred())

			shardFQDN := oc.Namespace() + "." + defaultDomain

			// The new router shard is using a namespace
			// selector so label this test namespace to
			// match.
			g.By("By labelling the namespace")
			err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "type="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating routes to test for http/2 compliance")
			err = oc.Run("new-app").Args("-f", http2RoutesConfigPath,
				"-p", "DOMAIN="+shardFQDN,
				"-p", "TLS_CRT="+pemCrt,
				"-p", "TLS_KEY="+derKey,
				"-p", "TYPE="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating a test-specific router shard")
			shardConfigPath, err = shard.DeployNewRouterShard(oc, 10*time.Minute, shard.Config{
				FixturePath: http2RouterShardConfigPath,
				Domain:      shardFQDN,
				Type:        oc.Namespace(),
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "new router shard did not rollout")

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

			g.By("Getting LB service")
			shardService, err := getRouterService(oc, 5*time.Minute, "router-"+oc.Namespace())
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(shardService).NotTo(o.BeNil())
			o.Expect(shardService.Status.LoadBalancer.Ingress).To(o.Not(o.BeEmpty()))

			var addrs []string

			if len(shardService.Status.LoadBalancer.Ingress[0].Hostname) > 0 {
				g.By("Waiting for LB hostname to register in DNS")
				addrs, err = resolveHost(oc, time.Minute, 15*time.Minute, shardService.Status.LoadBalancer.Ingress[0].Hostname)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(addrs).NotTo(o.BeEmpty())
			} else {
				addrs = append(addrs, shardService.Status.LoadBalancer.Ingress[0].IP)
			}

			g.By("Waiting for route hostname to register in DNS")
			addrs, err = resolveHostAsAddress(oc, time.Minute, 15*time.Minute, testCases[0].route+"."+shardFQDN, addrs[0])
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(addrs).NotTo(o.BeEmpty())

			for i, tc := range testCases {
				testConfig := fmt.Sprintf("%+v", tc)
				var resp *http.Response
				client := makeHTTPClient(tc.useHTTP2Transport, http2ClientTimeout)

				o.Expect(wait.Poll(time.Second, 5*time.Minute, func() (bool, error) {
					host := tc.route + "." + shardFQDN
					e2e.Logf("[test #%d/%d]: GET route: %s", i+1, len(testCases), host)
					resp, err = client.Get("https://" + host)
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
						resp.Body.Close()
						return false, nil
					}
					if resp.StatusCode != tc.statusCode {
						resp.Body.Close()
						e2e.Logf("[test #%d/%d]: config: %s, expected status: %v, actual status: %v", i+1, len(testCases), testConfig, tc.statusCode, resp.StatusCode)
						return false, nil
					}
					return true, nil
				})).NotTo(o.HaveOccurred())

				if tc.expectedGetError != "" {
					continue
				}

				o.Expect(resp).ToNot(o.BeNil(), testConfig)
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

func getDefaultIngressClusterDomainName(oc *exutil.CLI, timeout time.Duration) (string, error) {
	var domain string

	if err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		ingress, err := oc.AdminConfigClient().ConfigV1().Ingresses().Get(context.TODO(), "cluster", metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Get ingresses.config/cluster failed: %v, retrying...", err)
			return false, nil
		}
		domain = ingress.Spec.Domain
		return true, nil
	}); err != nil {
		return "", err
	}

	return domain, nil
}

func getRouterService(oc *exutil.CLI, timeout time.Duration, name string) (*v1.Service, error) {
	var svc *v1.Service

	if err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		o, err := oc.AdminKubeClient().CoreV1().Services("openshift-ingress").Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Get router service %s failed: %v, retrying...", name, err)
			return false, nil
		}
		svc = o
		return true, nil
	}); err != nil {
		return nil, err
	}

	return svc, nil
}

func getCanaryImage(oc *exutil.CLI) (string, error) {
	o, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().Get(context.Background(), "ingress", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, v := range o.Status.Versions {
		if v.Name == "canary-server" {
			return v.Version, nil
		}
	}
	return "", fmt.Errorf("expected to find canary-server version on clusteroperators/ingress")
}

func resolveHost(oc *exutil.CLI, interval, timeout time.Duration, host string) ([]string, error) {
	var result []string

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		addrs, err := net.LookupHost(host)
		if err != nil {
			e2e.Logf("error: %v, retrying in %s...", err, interval.String())
			return false, nil
		}
		result = addrs
		return true, nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func resolveHostAsAddress(oc *exutil.CLI, interval, timeout time.Duration, host, expectedAddr string) ([]string, error) {
	var result []string

	if err := wait.PollImmediate(interval, timeout, func() (bool, error) {
		addrs, err := net.LookupHost(host)
		if err != nil {
			e2e.Logf("error: %v, retrying in %s...", err, interval.String())
			return false, nil
		}

		for i := range addrs {
			if addrs[i] == expectedAddr {
				e2e.Logf("host %q now resolves as %+v", host, addrs)
				result = addrs
				return true, nil
			}
		}
		e2e.Logf("host %q resolves as %+v, expecting %v, retrying in %s...", host, addrs, expectedAddr, interval.String())
		return false, nil
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// platformHasHTTP2LoadBalancerService returns true where the default
// router is exposed by a load balancer service and can support http/2
// clients.
func platformHasHTTP2LoadBalancerService(platformType configv1.PlatformType) bool {
	switch platformType {
	case configv1.AzurePlatformType, configv1.GCPPlatformType:
		return true
	case configv1.AWSPlatformType:
		e2e.Logf("AWS support waiting on https://bugzilla.redhat.com/show_bug.cgi?id=1912413")
		fallthrough
	default:
		return false
	}
}
