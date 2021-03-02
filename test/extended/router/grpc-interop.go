package router

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"

	"github.com/openshift/origin/test/extended/router/certgen"
	grpcinterop "github.com/openshift/origin/test/extended/router/grpc-interop"
	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"

	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var (
		grpcServiceConfigPath     = exutil.FixturePath("testdata", "router", "router-grpc-interop.yaml")
		grpcRoutesConfigPath      = exutil.FixturePath("testdata", "router", "router-grpc-interop-routes.yaml")
		grpcSourceDataPath        = exutil.FixturePath("testdata", "router", "router-grpc-interop-server.data")
		grpcSourceGoModDataPath   = exutil.FixturePath("testdata", "router", "router-grpc-interop-gomod.data")
		grpcSourceGoSumDataPath   = exutil.FixturePath("testdata", "router", "router-grpc-interop-gosum.data")
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
			defaultDomain, err := getDefaultIngressClusterDomainName(oc, 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")

			srcTarGz, err := makeCompressedTarArchive([]string{
				grpcSourceDataPath,
				grpcSourceGoModDataPath,
				grpcSourceGoSumDataPath,
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			base64SrcTarGz := strings.Join(split(base64.StdEncoding.EncodeToString(srcTarGz), 76), "\n")

			g.By(fmt.Sprintf("creating service from a config file %q", grpcServiceConfigPath))
			err = oc.Run("new-app").Args("-f", grpcServiceConfigPath, "-p", "BASE64_SRC_TGZ="+base64SrcTarGz).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
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

			shardedDomain := oc.Namespace() + "." + defaultDomain

			g.By(fmt.Sprintf("creating routes from a config file %q", grpcRoutesConfigPath))
			err = oc.Run("new-app").Args("-f", grpcRoutesConfigPath,
				"-p", "DOMAIN="+shardedDomain,
				"-p", "TLS_CRT="+pemCrt,
				"-p", "TLS_KEY="+derKey,
				"-p", "TYPE="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("creating router shard %q from a config file %q", oc.Namespace(), grpcRouterShardConfigPath))
			shardConfigPath, err = shard.DeployNewRouterShard(oc, 15*time.Minute, shard.Config{
				FixturePath: grpcRouterShardConfigPath,
				Name:        oc.Namespace(),
				Domain:      shardedDomain,
				Type:        oc.Namespace(),
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "new ingresscontroller did not rollout")

			// Shard is using a namespace selector so
			// label the test namespace to match.
			err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "type="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			// If we cannot resolve then we're not going
			// to make a connection, so assert that lookup
			// succeeds for each route.
			for _, route := range []string{"grpc-interop-reencrypt", "grpc-interop-passthrough"} {
				err := wait.PollImmediate(3*time.Second, 15*time.Minute, func() (bool, error) {
					host := route + "." + shardedDomain
					addrs, err := net.LookupHost(host)
					if err != nil {
						e2e.Logf("host lookup error: %v, retrying...", err)
						return false, nil
					}
					e2e.Logf("host %q now resolves as %+v", host, addrs)
					return true, nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

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

				err := grpcExecTestCases(oc, routeType, 1*time.Minute, testCases...)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})
	})
})

// grpcExecTestCases run gRPC interop test cases using conn.
func grpcExecTestCases(oc *exutil.CLI, routeType routev1.TLSTerminationType, retryDuration time.Duration, testCases ...string) error {
	dialParams := grpcinterop.DialParams{
		Host:     getHostnameForRoute(oc, fmt.Sprintf("grpc-interop-%s", routeType)),
		Port:     443,
		UseTLS:   true,
		Insecure: true,
	}

	for _, name := range testCases {
		e2e.Logf("Running gRPC interop test case %q through %q", testCases, dialParams.Host)

		if err := wait.PollImmediate(time.Second, retryDuration, func() (bool, error) {
			e2e.Logf("Dialling: %+v", dialParams)
			conn, err := grpcinterop.Dial(dialParams)
			if err != nil {
				e2e.Logf("error: connection failed: %v, retrying...", err)
			}

			defer func() {
				conn.Close()
			}()

			if err := grpcinterop.ExecTestCase(conn, name); err != nil {
				e2e.Logf("error: running gRPC interop test case %q through %q: %v, retrying...", testCases, dialParams.Host, err)
				return false, nil
			}

			return true, nil
		}); err != nil {
			return err
		}
	}

	return nil
}
