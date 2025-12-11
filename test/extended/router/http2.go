package router

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	"golang.org/x/net/http2"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"

	"github.com/openshift/origin/test/extended/router/certgen"
	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// http2ClientGetTimeout specifies the time limit for requests
	// made by the HTTP Client.
	http2ClientTimeout = 1 * time.Minute
)

// makeHTTPClient creates a new HTTP client, configurable for timeout
// duration and optional HTTP/2 support.
//
// When useHTTP2Transport is true, the function enables HTTP/2 support
// via http2.ConfigureTransport, modifying the http.Transport to
// support HTTP/2 with a fallback to HTTP/1.1. If useHTTP2Transport is
// false, the client defaults to HTTP/1.1. The protocol selection
// between HTTP/2 and HTTP/1.1 occurs during the TLS handshake through
// ALPN, defaulting to HTTP/1.1 if HTTP/2 is not mutually agreed upon.
//
// This client is also configured with TLS settings that skip
// certificate verification (InsecureSkipVerify).
//
// The function returns a pointer to the configured http.Client.
func makeHTTPClient(useHTTP2Transport bool, timeout time.Duration) *http.Client {
	tlsConfig := tls.Config{
		InsecureSkipVerify: true,
	}

	transport := &http.Transport{
		TLSClientConfig: &tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}

	c := &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}

	// If HTTP/2 is to be used, configure it to allow falling back
	// to HTTP/1.1
	if useHTTP2Transport {
		// Explicitly allow HTTP/2.
		http2.ConfigureTransport(transport)
	}

	return c
}

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router][apigroup:route.openshift.io][apigroup:config.openshift.io]", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithPodSecurityLevel("router-http2", api.LevelBaseline)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(oc.KubeFramework().Namespace.Name)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("http2", oc)
			exutil.DumpPodLogsStartingWithInNamespace("router", "openshift-ingress", oc.AsAdmin())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should pass the http2 tests [apigroup:image.openshift.io][apigroup:operator.openshift.io]", g.Label("Size:L"), func() {
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
			http2service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http2",
					Annotations: map[string]string{
						"service.beta.openshift.io/serving-cert-secret-name": "serving-cert-http2",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"name": "http2",
					},
					Ports: []corev1.ServicePort{
						{
							Name:       "https",
							Protocol:   corev1.ProtocolTCP,
							Port:       8443,
							TargetPort: intstr.FromInt(8443),
						},
						{
							Name:       "http",
							Protocol:   corev1.ProtocolTCP,
							Port:       8080,
							TargetPort: intstr.FromInt(8080),
						},
					},
				},
			}

			ns := oc.KubeFramework().Namespace.Name
			_, err = oc.AdminKubeClient().CoreV1().Services(ns).Create(context.Background(), http2service, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating http2 test service pod")
			http2Pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "http2",
					Labels: map[string]string{
						"name": "http2",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Image:   image,
							Name:    "server",
							Command: []string{"ingress-operator", "serve-http2-test-server"},
							ReadinessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
							},
							LivenessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(8080),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8443,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "GODEBUG",
									Value: "http2debug=1",
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/etc/serving-cert",
									Name:      "cert",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "serving-cert-http2",
								},
							},
						},
					},
				},
			}

			_, err = oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), http2Pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Waiting for http2 pod to be running")
			e2e.ExpectNoError(e2epod.WaitForPodRunningInNamespaceSlow(context.TODO(), oc.KubeClient(), "http2", oc.KubeFramework().Namespace.Name))

			// certificate start and end time are very
			// lenient to avoid any clock drift between
			// the test machine and the cluster under
			// test.
			notBefore := time.Now().Add(-24 * time.Hour)
			notAfter := time.Now().Add(24 * time.Hour)

			// Generate crts/keys for routes that need them.
			_, tlsCrt1Data, tlsPrivateKey1, err := certgen.GenerateKeyPair("Root CA", notBefore, notAfter)
			o.Expect(err).NotTo(o.HaveOccurred())

			_, tlsCrt2Data, tlsPrivateKey2, err := certgen.GenerateKeyPair("Root CA", notBefore, notAfter)
			o.Expect(err).NotTo(o.HaveOccurred())

			pemKey1, err := certgen.MarshalPrivateKeyToPEMString(tlsPrivateKey1)
			o.Expect(err).NotTo(o.HaveOccurred())

			pemKey2, err := certgen.MarshalPrivateKeyToPEMString(tlsPrivateKey2)
			o.Expect(err).NotTo(o.HaveOccurred())

			pemCrt1, err := certgen.MarshalCertToPEMString(tlsCrt1Data)
			o.Expect(err).NotTo(o.HaveOccurred())

			pemCrt2, err := certgen.MarshalCertToPEMString(tlsCrt2Data)
			o.Expect(err).NotTo(o.HaveOccurred())

			shardFQDN := oc.Namespace() + "." + defaultDomain

			// The new router shard is using a namespace
			// selector so label this test namespace to
			// match.
			g.By("By labelling the namespace")
			err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "type="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating routes to test for http/2 compliance")
			routeType := oc.Namespace()
			routes := []routev1.Route{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "http2-default-cert-edge",
						Labels: map[string]string{
							"type": routeType,
						},
					},
					Spec: routev1.RouteSpec{
						Host: "http2-default-cert-edge." + shardFQDN,
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8080),
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationEdge,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
						},
						To: routev1.RouteTargetReference{
							Kind:   "Service",
							Name:   "http2",
							Weight: utilpointer.Int32(100),
						},
						WildcardPolicy: routev1.WildcardPolicyNone,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "http2-default-cert-reencrypt",
						Labels: map[string]string{
							"type": routeType,
						},
					},
					Spec: routev1.RouteSpec{
						Host: "http2-default-cert-reencrypt." + shardFQDN,
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8443),
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationReencrypt,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
						},
						To: routev1.RouteTargetReference{
							Kind:   "Service",
							Name:   "http2",
							Weight: utilpointer.Int32(100),
						},
						WildcardPolicy: routev1.WildcardPolicyNone,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "http2-custom-cert-edge",
						Labels: map[string]string{
							"type": routeType,
						},
					},
					Spec: routev1.RouteSpec{
						Host: "http2-custom-cert-edge." + shardFQDN,
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8080),
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationEdge,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
							Key:                           pemKey1,
							Certificate:                   pemCrt1,
						},
						To: routev1.RouteTargetReference{
							Kind:   "Service",
							Name:   "http2",
							Weight: utilpointer.Int32(100),
						},
						WildcardPolicy: routev1.WildcardPolicyNone,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "http2-custom-cert-reencrypt",
						Labels: map[string]string{
							"type": routeType,
						},
					},
					Spec: routev1.RouteSpec{
						Host: "http2-custom-cert-reencrypt." + shardFQDN,
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8443),
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationReencrypt,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
							Key:                           pemKey2,
							Certificate:                   pemCrt2,
						},
						To: routev1.RouteTargetReference{
							Kind:   "Service",
							Name:   "http2",
							Weight: utilpointer.Int32(100),
						},
						WildcardPolicy: routev1.WildcardPolicyNone,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "http2-passthrough",
						Labels: map[string]string{
							"type": routeType,
						},
					},
					Spec: routev1.RouteSpec{
						Host: "http2-passthrough." + shardFQDN,
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8443),
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationPassthrough,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
						},
						To: routev1.RouteTargetReference{
							Kind:   "Service",
							Name:   "http2",
							Weight: utilpointer.Int32(100),
						},
						WildcardPolicy: routev1.WildcardPolicyNone,
					},
				},
			}

			for _, route := range routes {
				_, err := oc.RouteClient().RouteV1().Routes(ns).Create(context.Background(), &route, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("Creating a test-specific router shard")
			shardIngressCtrl, err := shard.DeployNewRouterShard(oc, 10*time.Minute, shard.Config{
				Domain: shardFQDN,
				Type:   oc.Namespace(),
			})
			defer func() {
				if shardIngressCtrl != nil {
					if err := oc.AdminOperatorClient().OperatorV1().IngressControllers(shardIngressCtrl.Namespace).Delete(context.Background(), shardIngressCtrl.Name, metav1.DeleteOptions{}); err != nil {
						e2e.Logf("deleting ingress controller failed: %v\n", err)
					}
				}
			}()
			o.Expect(err).NotTo(o.HaveOccurred(), "new router shard did not rollout")

			testCases := []struct {
				route             string
				frontendProto     string
				backendProto      string
				statusCode        int
				useHTTP2Transport bool
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
				statusCode:        http.StatusOK,
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/1.1",
				useHTTP2Transport: true,
			}, {
				route:             "http2-default-cert-reencrypt",
				statusCode:        http.StatusOK,
				frontendProto:     "HTTP/1.1",
				backendProto:      "HTTP/2.0", // reencrypt always has backend ALPN enabled
				useHTTP2Transport: true,
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

			for i, tc := range testCases {
				testConfig := fmt.Sprintf("%+v", tc)
				var resp *http.Response
				client := makeHTTPClient(tc.useHTTP2Transport, http2ClientTimeout)

				o.Expect(wait.Poll(time.Second, 5*time.Minute, func() (bool, error) {
					host := tc.route + "." + shardFQDN
					e2e.Logf("[test #%d/%d]: GET route: %s", i+1, len(testCases), host)
					resp, err = client.Get("https://" + host)
					if err != nil {
						e2e.Logf("[test #%d/%d]: config: %s, GET error: %v", i+1, len(testCases), testConfig, err)
						return false, nil // could be 503 if service not ready
					}
					if resp.StatusCode != tc.statusCode {
						// Successful responses are checked and asserted
						// in the o.Expect() checks below.
						resp.Body.Close()
						e2e.Logf("[test #%d/%d]: config: %s, expected status: %v, actual status: %v", i+1, len(testCases), testConfig, tc.statusCode, resp.StatusCode)
						return false, nil
					}
					return true, nil
				})).NotTo(o.HaveOccurred())

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

// platformHasHTTP2LoadBalancerService returns true where the default
// router is exposed by a load balancer service and can support http/2
// clients.
func platformHasHTTP2LoadBalancerService(platformType configv1.PlatformType) bool {
	switch platformType {
	case configv1.AWSPlatformType, configv1.AzurePlatformType, configv1.GCPPlatformType:
		return true
	default:
		return false
	}
}
