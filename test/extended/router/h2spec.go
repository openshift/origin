package router

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/test/extended/router/h2spec"
	"github.com/openshift/origin/test/extended/router/shard"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

const h2specDialTimeoutInSeconds = 30

type h2specFailingTest struct {
	TestCase   *h2spec.JUnitTestCase
	TestNumber int
}

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var (
		h2specServiceConfigPath     = exutil.FixturePath("testdata", "router", "router-h2spec.yaml")
		h2specRoutesConfigPath      = exutil.FixturePath("testdata", "router", "router-h2spec-routes.yaml")
		h2specRouterShardConfigPath = exutil.FixturePath("testdata", "router", "router-shard.yaml")

		oc              = exutil.NewCLI("router-h2spec")
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
			exutil.DumpPodLogsStartingWith("h2spec", oc)
		}
		if len(shardConfigPath) > 0 {
			if err := oc.AsAdmin().Run("delete").Args("-n", "openshift-ingress-operator", "-f", shardConfigPath).Execute(); err != nil {
				e2e.Logf("deleting ingress controller failed: %v\n", err)
			}
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should pass the h2spec conformance tests", func() {
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

			g.By("Getting the default domain")
			defaultDomain, err := getDefaultIngressClusterDomainName(oc, time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to find default domain name")

			g.By("Locating the router image reference")
			routerImage, err := exutil.FindRouterImage(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Locating the canary image reference")
			canaryImage, err := getCanaryImage(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating h2spec test service")
			err = oc.Run("new-app").Args("-f", h2specServiceConfigPath,
				"-p", "HAPROXY_IMAGE="+routerImage,
				"-p", "H2SPEC_IMAGE="+canaryImage).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			e2e.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(oc.KubeClient(), "h2spec-haproxy", oc.KubeFramework().Namespace.Name))
			e2e.ExpectNoError(e2epod.WaitForPodNameRunningInNamespace(oc.KubeClient(), "h2spec", oc.KubeFramework().Namespace.Name))

			shardFQDN := oc.Namespace() + "." + defaultDomain

			// The new router shard is using a namespace
			// selector so label this test namespace to
			// match.
			g.By("By labelling the namespace")
			err = oc.AsAdmin().Run("label").Args("namespace", oc.Namespace(), "type="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating routes to test for h2spec compliance")
			err = oc.Run("new-app").Args("-f", h2specRoutesConfigPath,
				"-p", "DOMAIN="+shardFQDN,
				"-p", "TYPE="+oc.Namespace()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Creating a test-specific router shard")
			shardConfigPath, err = shard.DeployNewRouterShard(oc, 10*time.Minute, shard.Config{
				FixturePath: h2specRouterShardConfigPath,
				Domain:      shardFQDN,
				Type:        oc.Namespace(),
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "new router shard did not rollout")

			g.By("Getting LB service")
			shardService, err := getRouterService(oc, 5*time.Minute, "router-"+oc.Namespace())
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(shardService).NotTo(o.BeNil())
			o.Expect(shardService.Status.LoadBalancer.Ingress).ShouldNot(o.BeEmpty())

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
			host := "h2spec-passthrough." + shardFQDN
			addrs, err = resolveHostAsAddress(oc, time.Minute, 15*time.Minute, host, addrs[0])
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(addrs).NotTo(o.BeEmpty())

			// ROUTER_H2SPEC_SAMPLE when set runs the
			// conformance tests for N iterations to
			// identify flaking tests.
			//
			// This should be enabled in development every
			// time we consume any new version of haproxy
			// be that a major, minor or a micro update to
			// continuously validate the set of test case
			// IDs that fail.
			if iterations := lookupEnv("ROUTER_H2SPEC_SAMPLE", ""); iterations != "" {
				n, err := strconv.Atoi(iterations)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(n).To(o.BeNumerically(">", 0))
				runConformanceTestsAndLogAggregateFailures(oc, host, "h2spec", n)
				return
			}

			testSuites, err := runConformanceTests(oc, host, "h2spec", 5*time.Minute)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(testSuites).ShouldNot(o.BeEmpty())

			failures := failingTests(testSuites)
			failureCount := len(failures)

			g.By("Analyzing results")
			// https://github.com/haproxy/haproxy/issues/471#issuecomment-591420924
			//
			// 5. Streams and Multiplexing
			//   5.4. Error Handling
			//     5.4.1. Connection Error Handling
			//       using source address 10.131.0.37:34742
			//       × 2: Sends an invalid PING frame to receive GOAWAY frame
			//         -> An endpoint that encounters a connection error SHOULD first send a GOAWAY frame
			//            Expected: GOAWAY Frame (Error Code: PROTOCOL_ERROR)
			//              Actual: Connection closed
			//
			// 6. Frame Definitions
			//   6.9. WINDOW_UPDATE
			//     6.9.1. The Flow-Control Window
			//       using source address 10.131.0.37:34816
			//       × 2: Sends multiple WINDOW_UPDATE frames increasing the flow control window to above 2^31-1
			//         -> The endpoint MUST sends a GOAWAY frame with a FLOW_CONTROL_ERROR code.
			//            Expected: GOAWAY Frame (Error Code: FLOW_CONTROL_ERROR)
			//              Actual: Connection closed
			//
			// 147 tests, 145 passed, 0 skipped, 2 failed
			knownFailures := map[string]bool{
				"http2/5.4.1.2": true,
				"http2/6.9.1.2": true,
			}
			for _, f := range failures {
				if _, exists := knownFailures[f.ID()]; exists {
					failureCount -= 1
					e2e.Logf("TestCase ID: %q is a known failure; ignoring", f.ID())
				} else {
					e2e.Logf("TestCase ID: %q (%q) ****FAILED****", f.ID(), f.TestCase.ClassName)
				}
			}
			o.Expect(failureCount).Should(o.BeZero(), "expected zero failures")
		})
	})
})

func failingTests(testSuites []*h2spec.JUnitTestSuite) []h2specFailingTest {
	var failures []h2specFailingTest

	for _, ts := range testSuites {
		for i := 0; i < ts.Tests; i++ {
			if ts.TestCases[i].Error != nil {
				failures = append(failures, h2specFailingTest{
					TestNumber: i + 1,
					TestCase:   ts.TestCases[i],
				})
			}
		}
	}

	return failures
}

func runConformanceTests(oc *exutil.CLI, host, podName string, timeout time.Duration) ([]*h2spec.JUnitTestSuite, error) {
	var testSuites []*h2spec.JUnitTestSuite

	if err := wait.Poll(time.Second, timeout, func() (bool, error) {
		g.By("Running the h2spec CLI test")

		// this is the output file in the pod
		outputFile := "/tmp/h2spec-results"

		// h2spec will exit with non-zero if _any_ test in the suite
		// fails, or if there is a dial timeout, so we log the
		// error. But if we can fetch the results and if we can decode the
		// results and we have > 0 test suites from the decoded
		// results then assume the test ran.
		output, err := e2e.RunHostCmd(oc.Namespace(), podName, h2specCommand(h2specDialTimeoutInSeconds, host, outputFile))
		if err != nil {
			e2e.Logf("error running h2spec: %v, but checking on result content", err)
		}

		g.By("Copying results")
		data, err := e2e.RunHostCmd(oc.Namespace(), podName, fmt.Sprintf("cat %q", outputFile))
		if err != nil {
			e2e.Logf("error copying results: %v, retrying...", err)
			return false, nil
		}
		if len(data) == 0 {
			e2e.Logf("results file is zero length, retrying...")
			return false, nil
		}

		g.By("Decoding results")
		testSuites, err = h2spec.DecodeJUnitReport(strings.NewReader(data))
		if err != nil {
			e2e.Logf("error decoding results: %v, retrying...", err)
			return false, nil
		}
		if len(testSuites) == 0 {
			e2e.Logf("expected len(testSuites) > 0, retrying...")
			return false, nil
		}

		// Log what we consider a successful run
		e2e.Logf("h2spec results\n%s", output)
		return true, nil
	}); err != nil {
		return nil, err
	}

	return testSuites, nil
}

func runConformanceTestsAndLogAggregateFailures(oc *exutil.CLI, host, podName string, iterations int) {
	sortKeys := func(m map[string]int) []string {
		var index []string
		for k := range m {
			index = append(index, k)
		}
		sort.Strings(index)
		return index
	}

	printFailures := func(prefix string, m map[string]int) {
		for _, id := range sortKeys(m) {
			e2e.Logf("%sTestCase ID: %q, cumulative failures: %v", prefix, id, m[id])
		}
	}

	failuresByTestCaseID := map[string]int{}

	for i := 1; i <= iterations; i++ {
		testResults, err := runConformanceTests(oc, host, podName, 5*time.Minute)
		if err != nil {
			e2e.Logf(err.Error())
			continue
		}
		failures := failingTests(testResults)
		e2e.Logf("Iteration %v/%v: had %v failures", i, iterations, len(failures))

		// Aggregate any new failures
		for _, f := range failures {
			failuresByTestCaseID[f.ID()]++
		}

		// Dump the current state at every iteration should
		// you wish to interrupt/abort the running test.
		printFailures("\t", failuresByTestCaseID)
	}

	e2e.Logf("Sampling completed: %v test cases failed", len(failuresByTestCaseID))
	printFailures("\t", failuresByTestCaseID)
}

func getHostnameForRoute(oc *exutil.CLI, routeName string) (string, error) {
	var hostname string
	ns := oc.KubeFramework().Namespace.Name
	if err := wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
		route, err := oc.RouteClient().RouteV1().Routes(ns).Get(context.Background(), routeName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Error getting hostname for route %q: %v", routeName, err)
			return false, err
		}
		if len(route.Status.Ingress) == 0 || len(route.Status.Ingress[0].Host) == 0 {
			return false, nil
		}
		hostname = route.Status.Ingress[0].Host
		return true, nil
	}); err != nil {
		return "", err
	}
	return hostname, nil
}

func h2specCommand(timeout int, hostname, results string) string {
	return fmt.Sprintf("ingress-operator h2spec --timeout=%v --tls --insecure --strict --host=%q --junit-report=%q", timeout, hostname, results)
}

func (f h2specFailingTest) ID() string {
	return fmt.Sprintf("%s.%d", f.TestCase.Package, f.TestNumber)
}

func lookupEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultVal
}
