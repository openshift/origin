package router

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	routev1 "github.com/openshift/api/route/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"

	grpcinterop "github.com/openshift/origin/test/extended/router/grpc-interop"
	exutil "github.com/openshift/origin/test/extended/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// gRPCInteropTestTimeout is the timeout value for the
	// internal tests.
	gRPCInteropTestTimeout = 2 * time.Minute

	// gRPCInteropTestCaseIterations is the number of times each gRPC
	// interop test case should be invoked.
	gRPCInteropTestCaseIterations = 5
)

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var (
		configPath = exutil.FixturePath("testdata", "router", "router-grpc-interop.yaml")
		oc         = exutil.NewCLI("router-grpc")
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(oc.KubeFramework().Namespace.Name)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("grpc", oc)
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should pass the gRPC interoperability tests", func() {
			g.By(fmt.Sprintf("creating test fixture from a config file %q", configPath))
			err := oc.Run("new-app").Args("-f", configPath).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.ExpectNoError(oc.KubeFramework().WaitForPodRunning("grpc-interop"))

			g.By("Discovering the set of supported test cases")
			ns := oc.KubeFramework().Namespace.Name
			output, err := grpcInteropExecClientShellCmd(ns, gRPCInteropTestTimeout, "/workdir/grpc-client -list-tests")
			o.Expect(err).NotTo(o.HaveOccurred())
			testCases := strings.Split(strings.TrimSpace(output), "\n")
			o.Expect(testCases).ShouldNot(o.BeEmpty())
			sort.Strings(testCases)

			grpcInteropExecInClusterRouteTests(oc, testCases, gRPCInteropTestCaseIterations)
			grpcInteropExecInClusterServiceTests(oc, testCases, gRPCInteropTestCaseIterations)
			grpcInteropExecOutOfClusterRouteTests(oc, testCases, gRPCInteropTestCaseIterations)
		})
	})
})

// grpcInteropClientShellCmd construct the grpc-client command to run
// within the cluster.
func grpcInteropClientShellCmd(host string, port int, useTLS bool, caCert string, insecure bool, count int) string {
	cmd := fmt.Sprintf("/workdir/grpc-client -host %q -port %v", host, port)
	if count > 0 {
		cmd = fmt.Sprintf("%s -count %v", cmd, count)
	}
	if useTLS {
		cmd = fmt.Sprintf("%s -tls", cmd)
	}
	if caCert != "" {
		cmd = fmt.Sprintf("%s -ca-cert %q", cmd, caCert)
	}
	if insecure {
		cmd = fmt.Sprintf("%s -insecure", cmd)
	}
	return cmd
}

// grpcInteropExecInClusterRouteTests run gRPC interop tests using routes
// from a POD within the test cluster.
func grpcInteropExecInClusterRouteTests(oc *exutil.CLI, testCases []string, iterations int) {
	for _, route := range []routev1.TLSTerminationType{
		routev1.TLSTerminationEdge,
		routev1.TLSTerminationPassthrough,
		routev1.TLSTerminationReencrypt,
	} {
		if route == routev1.TLSTerminationEdge {
			e2e.Logf("skipping %v tests - needs https://github.com/openshift/router/pull/104", route)
			continue
		}

		host := getHostnameForRoute(oc, fmt.Sprintf("grpc-interop-%s", route))
		cmd := grpcInteropClientShellCmd(host, 443, true, "", true, iterations) + " " + strings.Join(testCases, " ")
		e2e.Logf("Running gRPC interop tests %v in the cluster using route %q", testCases, host)
		_, err := grpcInteropExecClientShellCmd(oc.KubeFramework().Namespace.Name, gRPCInteropTestTimeout, cmd)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// grpcInteropExecInClusterServiceTests run gRPC interop tests against the
// internal service from a POD within the test cluster.
func grpcInteropExecInClusterServiceTests(oc *exutil.CLI, testCases []string, iterations int) {
	for _, tc := range []struct {
		port     int
		useTLS   bool
		caCert   string
		insecure bool
	}{{
		port:   1110, // h2c
		useTLS: false,
	}, {
		port:   8443, // h2
		useTLS: true,
		caCert: "/etc/service-ca/service-ca.crt",
	}} {
		svc := fmt.Sprintf("grpc-interop.%s.svc", oc.KubeFramework().Namespace.Name)
		cmd := grpcInteropClientShellCmd(svc, tc.port, tc.useTLS, tc.caCert, tc.insecure, iterations) + " " + strings.Join(testCases, " ")
		e2e.Logf("Running gRPC interop tests %v in the cluster using service %q", testCases, svc)
		_, err := grpcInteropExecClientShellCmd(oc.KubeFramework().Namespace.Name, gRPCInteropTestTimeout, cmd)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}

// grpcInteropExecOutOfClusterRouteTests run gRPC interop tests using
// routes and from outside of the test cluster.
func grpcInteropExecOutOfClusterRouteTests(oc *exutil.CLI, testCases []string, iterations int) {
	for _, route := range []routev1.TLSTerminationType{
		routev1.TLSTerminationEdge,
		routev1.TLSTerminationReencrypt,
		routev1.TLSTerminationPassthrough,
	} {
		if route == routev1.TLSTerminationEdge {
			e2e.Logf("skipping %v tests - needs https://github.com/openshift/router/pull/104", route)
			continue
		}

		dialParams := grpcinterop.DialParams{
			Host:     getHostnameForRoute(oc, fmt.Sprintf("grpc-interop-%s", route)),
			Port:     443,
			UseTLS:   true,
			Insecure: true,
		}

		conn, err := grpcinterop.Dial(dialParams)
		o.Expect(err).NotTo(o.HaveOccurred())

		for i := 0; i < iterations; i++ {
			e2e.Logf("[%v/%v] Running gRPC interop test cases %v using route %q", i+1, iterations, testCases, dialParams.Host)
			for _, name := range testCases {
				err := grpcinterop.ExecTestCase(conn, name)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		o.Expect(conn.Close()).NotTo(o.HaveOccurred())
	}
}

// grpcInteropExecClientShellCmd runs the given cmd in the context of
// the "client-shell" container in the "grpc-interop" POD.
func grpcInteropExecClientShellCmd(ns string, timeout time.Duration, cmd string) (string, error) {
	return e2e.RunKubectl(ns, "exec", fmt.Sprintf("--namespace=%v", ns), "grpc-interop", "-c", "client-shell", "--", "/bin/sh", "-x", "-c", fmt.Sprintf("timeout %v %s", timeout.Seconds(), cmd))
}
