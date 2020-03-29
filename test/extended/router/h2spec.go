package router

import (
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	routev1 "github.com/openshift/api/route/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/test/extended/router/h2spec"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/url"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

type h2specFailingTest struct {
	TestCase   *h2spec.JUnitTestCase
	TestNumber int
}

type h2specRouteTypeTest struct {
	routeType     routev1.TLSTerminationType
	hostname      string
	knownFailures map[string]bool
}

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var (
		configPath = exutil.FixturePath("testdata", "router", "router-h2spec.yaml")
		oc         = exutil.NewCLI("router-h2spec")
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(oc.KubeFramework().Namespace.Name)
			if routes, _ := client.List(metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("h2spec", oc)
		}
	})

	g.By(fmt.Sprintf("creating routes from a config file %q", configPath))

	g.Describe("The HAProxy router", func() {
		g.It("should pass the h2spec conformance tests", func() {
			routerImage, err := exutil.FindRouterImage(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			err = oc.Run("new-app").Args("-f", configPath, "-p", "HAPROXY_IMAGE="+routerImage).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			e2e.ExpectNoError(oc.KubeFramework().WaitForPodRunning("h2spec-haproxy"))

			routeTypeTests := []h2specRouteTypeTest{{
				routeType: routev1.TLSTerminationEdge,
				knownFailures: map[string]bool{
					"http2/4.2.3": true,
				},
			}, {
				routeType: routev1.TLSTerminationPassthrough,
				knownFailures: map[string]bool{
					"http2/4.2.3": true,
				},
			}, {
				routeType: routev1.TLSTerminationReencrypt,
				knownFailures: map[string]bool{
					"http2/4.2.3": true,
				},
			}}

			g.By("verifying accessing the host returns a 200 status code")
			for i := 0; i < len(routeTypeTests); i++ {
				urlTester := url.NewTester(oc.AdminKubeClient(), oc.KubeFramework().Namespace.Name).WithErrorPassthrough(true)
				defer urlTester.Close()
				hostname := getHostnameForRoute(oc, fmt.Sprintf("h2spec-haproxy-%s", routeTypeTests[i].routeType))
				urlTester.Within(30*time.Second, url.Expect("GET", "https://"+hostname).Through(hostname).SkipTLSVerification().HasStatusCode(200))
				routeTypeTests[i].hostname = hostname // now valid for the remaining tests
			}

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
				runConformanceTestsAndLogAggregateFailures(oc, routeTypeTests, n)
				return
			}

			for _, t := range routeTypeTests {
				g.By(fmt.Sprintf("[%s] Running h2spec conformance tests against %q", t.routeType, t.hostname))

				testSuites := runConformanceTests(oc, t)
				o.Expect(testSuites).ShouldNot(o.BeEmpty())

				failures := failingTests(testSuites)
				failureCount := len(failures)

				g.By("Analyzing results")
				for _, f := range failures {
					if _, exists := t.knownFailures[f.ID()]; exists {
						failureCount -= 1
						e2e.Logf("[%s] TestCase ID: %q is a known failure; ignoring", t.routeType, f.ID())
					} else {
						e2e.Logf("[%s] TestCase ID: %q (%q) ****FAILED****", t.routeType, f.ID(), f.TestCase.ClassName)
					}
				}
				o.Expect(failureCount).Should(o.BeZero(), "expected zero failures")
			}
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

func runConformanceTests(oc *exutil.CLI, t h2specRouteTypeTest) []*h2spec.JUnitTestSuite {
	podName := "h2spec"
	e2e.ExpectNoError(oc.KubeFramework().WaitForPodRunning(podName))

	outputFile, err := ioutil.TempFile("", "runConformanceTests")
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.Remove(outputFile.Name())

	g.By("Running the h2spec CLI test")
	h2specOutput, h2specErr := e2e.RunHostCmd(oc.KubeFramework().Namespace.Name, podName, h2specCommand(t.hostname, outputFile.Name()))
	if h2specErr != nil {
		// We don't assert on a failure here; h2spec will exit
		// with non-zero if _any_ test in the suite fails. We
		// later assert on failing to copy the results,
		// failing to read the results, failing to decode the
		// results because there are no testsuites in the
		// decoded results. We always expect at least one
		// testsuite in the results.
		e2e.Logf("[%s] h2spec: error=\n%s", t.routeType, h2specErr)
	}
	if h2specErr != nil && h2specErr.Error() != h2specOutput {
		// Also log any output if different.
		e2e.Logf("[%s] h2spec: output=\n%s", t.routeType, h2specOutput)
	}

	g.By(fmt.Sprintf("[%s] Copying %s:%q %q", t.routeType, podName, outputFile.Name(), outputFile.Name()))
	cpErr := oc.Run("cp").Args(podName+":"+outputFile.Name(), outputFile.Name()).Execute()
	o.Expect(cpErr).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("[%s] Reading %q", t.routeType, outputFile.Name()))
	data, readErr := ioutil.ReadFile(outputFile.Name())
	o.Expect(readErr).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("[%s] Decoding %q", t.routeType, outputFile.Name()))
	testSuites, err := h2spec.DecodeJUnitReport(strings.NewReader(string(data)))
	o.Expect(err).NotTo(o.HaveOccurred())

	return testSuites
}

func runConformanceTestsAndLogAggregateFailures(oc *exutil.CLI, tests []h2specRouteTypeTest, iterations int) {
	sortKeys := func(m map[string]int) []string {
		var index []string
		for k := range m {
			index = append(index, k)
		}
		sort.Strings(index)
		return index
	}

	printFailures := func(t h2specRouteTypeTest, prefix string, m map[string]int) {
		for _, id := range sortKeys(m) {
			e2e.Logf("[%s] %sTestCase ID: %q, cumulative failures: %v", t.routeType, prefix, id, m[id])
		}
	}

	failuresByTestCaseID := map[string]int{}

	for _, t := range tests {
		for i := 1; i <= iterations; i++ {
			failures := failingTests(runConformanceTests(oc, t))
			e2e.Logf("[%s] Iteration %v/%v: had %v failures", t.routeType, i, iterations, len(failures))

			// Aggregate any new failures
			for _, f := range failures {
				failuresByTestCaseID[f.ID()]++
			}

			// Dump the current state at every iteration should
			// you wish to interrupt/abort the running test.
			printFailures(t, "\t", failuresByTestCaseID)
		}
	}

	for _, t := range tests {
		e2e.Logf("[%s] Sampling completed: %v test cases failed", t.routeType, len(failuresByTestCaseID))
		printFailures(t, "\t", failuresByTestCaseID)
	}
}

func getHostnameForRoute(oc *exutil.CLI, routeName string) string {
	var hostname string
	ns := oc.KubeFramework().Namespace.Name
	err := wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
		route, err := oc.RouteClient().RouteV1().Routes(ns).Get(routeName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Error getting hostname for route %q: %v", routeName, err)
			return false, err
		}
		if len(route.Status.Ingress) == 0 || len(route.Status.Ingress[0].Host) == 0 {
			return false, nil
		}
		hostname = route.Status.Ingress[0].Host
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return hostname
}

func h2specCommand(hostname, results string) string {
	return fmt.Sprintf("h2spec --tls --insecure --strict --host=%q --junit-report=%q", hostname, results)
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
