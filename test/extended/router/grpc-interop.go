package router

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"

	"github.com/openshift/origin/test/extended/router/certgen"
	grpcinterop "github.com/openshift/origin/test/extended/router/grpc-interop"
	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var (
		grpcServiceConfigPath     = exutil.FixturePath("testdata", "router", "router-grpc-interop.yaml")
		grpcRoutesConfigPath      = exutil.FixturePath("testdata", "router", "router-grpc-interop-routes.yaml")
		grpcRouterShardConfigPath = exutil.FixturePath("testdata", "router", "router-shard.yaml")
		oc                        = exutil.NewCLI("grpc-interop")
		shardConfigPath           string // computed
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			exutil.DumpPodLogsStartingWith("grpc", oc)
			exutil.DumpPodLogsStartingWithInNamespace("router", "openshift-ingress", oc.AsAdmin())
		}
		if len(shardConfigPath) > 0 {
			oc.AsAdmin().Run("delete").Args("-n", "openshift-ingress-operator", "-f", shardConfigPath).Execute()
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should pass the gRPC interoperability tests", func() {
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
			err = oc.Run("new-app").Args("-f", grpcServiceConfigPath, "-p", "IMAGE="+image).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Waiting for grpc-interop pod to be running")
			e2e.ExpectNoError(e2epod.WaitForPodRunningInNamespaceSlow(oc.KubeClient(), "grpc-interop", oc.Namespace()), "grpc-interop backend server pod not running")

			// certificate start and end time are very
			// lenient to avoid any clock drift between
			// between the test machine and the cluster
			// under test.
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

			g.By("Creating routes to test for gRPC interoperability")
			err = oc.Run("new-app").Args("-f", grpcRoutesConfigPath,
				"-p", "DOMAIN="+shardFQDN,
				"-p", "TLS_CRT="+pemCrt,
				"-p", "TLS_KEY="+derKey,
				"-p", "TYPE="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating a test-specific router shard")
			shardConfigPath, err = shard.DeployNewRouterShard(oc, 10*time.Minute, shard.Config{
				FixturePath: grpcRouterShardConfigPath,
				Domain:      shardFQDN,
				Type:        oc.Namespace(),
			})
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
				routev1.TLSTerminationEdge,
				routev1.TLSTerminationReencrypt,
				routev1.TLSTerminationPassthrough,
			} {
				if routeType == routev1.TLSTerminationEdge {
					e2e.Logf("skipping %v tests - needs https://github.com/openshift/router/pull/104", routeType)
					continue
				}

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
				host := fmt.Sprintf("grpc-interop-%s.%s", routeType, shardFQDN)
				addrs, err = resolveHostAsAddress(oc, time.Minute, 15*time.Minute, host, addrs[0])
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(addrs).NotTo(o.BeEmpty())

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
