package router

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"

	routev1 "github.com/openshift/api/route/v1"

	"github.com/openshift/origin/test/extended/router/certgen"
	grpcinterop "github.com/openshift/origin/test/extended/router/grpc-interop"
	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLIWithPodSecurityLevel("grpc-interop", admissionapi.LevelBaseline)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWith("grpc", oc)
			exutil.DumpPodLogsStartingWithInNamespace("router", "openshift-ingress", oc.AsAdmin())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should pass the gRPC interoperability tests [apigroup:route.openshift.io][apigroup:operator.openshift.io]", func() {
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

			g.By("Creating grpc-interop test service")
			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "grpc-interop",
					Annotations: map[string]string{
						"service.beta.openshift.io/serving-cert-secret-name": "service-cert-grpc-interop",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "grpc-interop",
					},
					Ports: []corev1.ServicePort{
						{
							AppProtocol: utilpointer.String("h2c"),
							Name:        "h2c",
							Port:        1110,
							Protocol:    corev1.ProtocolTCP,
							TargetPort:  intstr.FromInt(1110),
						},
						{
							Name:       "https",
							Port:       8443,
							Protocol:   corev1.ProtocolTCP,
							TargetPort: intstr.FromInt(8443),
						},
					},
				},
			}

			ns := oc.KubeFramework().Namespace.Name
			_, err = oc.AdminKubeClient().CoreV1().Services(ns).Create(context.Background(), service, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating grpc-interop test service pod")
			servicePod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "grpc-interop",
					Labels: map[string]string{
						"app": "grpc-interop",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "server",
							Image:           image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         []string{"ingress-operator", "serve-grpc-test-server"},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 1110,
									Name:          "h2c",
									Protocol:      corev1.ProtocolTCP,
								},
								{
									ContainerPort: 8443,
									Name:          "https",
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/etc/serving-cert",
									Name:      "cert",
								},
							},
							ReadinessProbe: &corev1.Probe{
								FailureThreshold: 3,
								ProbeHandler: corev1.ProbeHandler{
									TCPSocket: &corev1.TCPSocketAction{
										Port: intstr.FromInt(8443),
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
										Port: intstr.FromInt(8443),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "service-cert-grpc-interop",
								},
							},
						},
					},
				},
			}

			_, err = oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), servicePod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Waiting for grpc-interop pod to be running")
			e2e.ExpectNoError(e2epod.WaitForPodRunningInNamespaceSlow(context.TODO(), oc.KubeClient(), "grpc-interop", oc.Namespace()), "grpc-interop backend server pod not running")

			// certificate start and end time are very
			// lenient to avoid any clock drift between
			// between the test machine and the cluster
			// under test.
			notBefore := time.Now().Add(-24 * time.Hour)
			notAfter := time.Now().Add(24 * time.Hour)

			// Generate crt/key for routes that need them.
			_, tlsCrtData, tlsPrivateKey, err := certgen.GenerateKeyPair("Root CA", notBefore, notAfter)
			o.Expect(err).NotTo(o.HaveOccurred())

			pemKey, err := certgen.MarshalPrivateKeyToPEMString(tlsPrivateKey)
			o.Expect(err).NotTo(o.HaveOccurred())

			pemCrt, err := certgen.MarshalCertToPEMString(tlsCrtData)
			o.Expect(err).NotTo(o.HaveOccurred())

			shardFQDN := oc.Namespace() + "." + defaultDomain

			g.By("Creating routes to test for gRPC interoperability")
			routeType := oc.Namespace()
			routes := []routev1.Route{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "grpc-interop-h2c",
						Labels: map[string]string{
							"type": routeType,
						},
					},
					Spec: routev1.RouteSpec{
						Host: "grpc-interop-h2c." + shardFQDN,
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(1110),
						},
						To: routev1.RouteTargetReference{
							Kind:   "Service",
							Name:   "grpc-interop",
							Weight: utilpointer.Int32(100),
						},
						WildcardPolicy: routev1.WildcardPolicyNone,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "grpc-interop-edge",
						Labels: map[string]string{
							"type": routeType,
						},
					},
					Spec: routev1.RouteSpec{
						Host: "grpc-interop-edge." + shardFQDN,
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(1110),
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationEdge,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
							Key:                           pemKey,
							Certificate:                   pemCrt,
						},
						To: routev1.RouteTargetReference{
							Kind:   "Service",
							Name:   "grpc-interop",
							Weight: utilpointer.Int32(100),
						},
						WildcardPolicy: routev1.WildcardPolicyNone,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "grpc-interop-reencrypt",
						Labels: map[string]string{
							"type": routeType,
						},
					},
					Spec: routev1.RouteSpec{
						Host: "grpc-interop-reencrypt." + shardFQDN,
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8443),
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationReencrypt,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
							Key:                           pemKey,
							Certificate:                   pemCrt,
						},
						To: routev1.RouteTargetReference{
							Kind:   "Service",
							Name:   "grpc-interop",
							Weight: utilpointer.Int32(100),
						},
						WildcardPolicy: routev1.WildcardPolicyNone,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "grpc-interop-passthrough",
						Labels: map[string]string{
							"type": routeType,
						},
					},
					Spec: routev1.RouteSpec{
						Host: "grpc-interop-passthrough." + shardFQDN,
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8443),
						},
						TLS: &routev1.TLSConfig{
							Termination:                   routev1.TLSTerminationPassthrough,
							InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
						},
						To: routev1.RouteTargetReference{
							Kind:   "Service",
							Name:   "grpc-interop",
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

			// Shard is using a namespace selector so
			// label the test namespace to match.
			g.By("By labelling the namespace")
			err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "type="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Getting LB service")
			shardService, err := getRouterService(oc, 5*time.Minute, "router-"+oc.Namespace())
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(shardService).NotTo(o.BeNil())
			o.Expect(shardService.Status.LoadBalancer.Ingress).To(o.Not(o.BeEmpty()))

			testCases := []string{
				"cancel_after_begin",
				"cancel_after_first_response",
				"client_streaming",
				"custom_metadata",
				"empty_unary",
				"large_unary",
				"ping_pong",
				"server_streaming",
				"special_status_message",
				"status_code_and_message",
				"timeout_on_sleeping_server",
				"unimplemented_method",
				"unimplemented_service",
			}

			for _, routeType := range []routev1.TLSTerminationType{
				"h2c",
				routev1.TLSTerminationEdge,
				routev1.TLSTerminationReencrypt,
				routev1.TLSTerminationPassthrough,
			} {
				err := grpcExecTestCases(oc, routeType, 5*time.Minute, testCases...)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
	})
})

// grpcExecTestCases run gRPC interop test cases.
func grpcExecTestCases(oc *exutil.CLI, routeType routev1.TLSTerminationType, timeout time.Duration, testCases ...string) error {
	host, err := getHostnameForRoute(oc, fmt.Sprintf("grpc-interop-%s", routeType))
	if err != nil {
		return err
	}

	dialParams := grpcinterop.DialParams{
		Host:     host,
		Port:     443,
		UseTLS:   true,
		Insecure: true,
	}

	if routeType == "h2c" {
		dialParams.Port = 80
		dialParams.UseTLS = false
	}

	for _, name := range testCases {
		e2e.Logf("Running gRPC interop test case %q via host %q", name, dialParams.Host)

		if err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
			e2e.Logf("Dialling: %+v", dialParams)
			conn, err := grpcinterop.Dial(dialParams)
			if err != nil {
				e2e.Logf("error: connection failed: %v, retrying...", err)
				return false, nil
			}

			defer func() {
				conn.Close()
			}()

			if err := grpcinterop.ExecTestCase(conn, name); err != nil {
				e2e.Logf("error: running gRPC interop test case %q through %q: %v, retrying...", name, dialParams.Host, err)
				return false, nil
			}

			return true, nil
		}); err != nil {
			return err
		}
	}

	return nil
}
