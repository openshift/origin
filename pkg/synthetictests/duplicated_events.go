package synthetictests

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configclient "github.com/openshift/client-go/config/clientset/versioned"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo"
)

func combinedRegexp(arr ...*regexp.Regexp) *regexp.Regexp {
	s := ""
	for _, r := range arr {
		if s != "" {
			s += "|"
		}
		s += r.String()
	}
	return regexp.MustCompile(s)
}

var allowedRepeatedEventPatterns = []*regexp.Regexp{
	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should not deadlock when a pod's predecessor fails [Suite:openshift/conformance/parallel] [Suite:k8s]
	// PauseNewPods intentionally causes readiness probe to fail.
	regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),

	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should perform rolling updates and roll backs of template modifications [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]
	// breakPodHTTPProbe intentionally causes readiness probe to fail.
	regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss2-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: HTTP probe failed with statuscode: 404`),

	// [sig-node] Probing container should be restarted startup probe fails [Suite:openshift/conformance/parallel] [Suite:k8s]
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ pod/startup-[0-9a-f-]+ node/[a-z0-9.-]+ - reason/Unhealthy Startup probe failed: `),

	// [sig-node] Probing container should *not* be restarted with a non-local redirect http liveness probe [Suite:openshift/conformance/parallel] [Suite:k8s]
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ pod/liveness-[0-9a-f-]+ node/[a-z0-9.-]+ - reason/ProbeWarning Liveness probe warning: <a href="http://0\.0\.0\.0/">Found</a>\.\n\n`),
}

var allowedRepeatedEventFns = []isRepeatedEventOKFunc{
	isConsoleReadinessDuringInstallation,
}

// allowedUpgradeRepeatedEventPatterns are patterns of events that we should only allow during upgrades, not during normal execution.
var allowedUpgradeRepeatedEventPatterns = []*regexp.Regexp{
	// Operators that use library-go can report about multiple versions during upgrades.
	regexp.MustCompile(`ns/openshift-kube-controller-manager-operator deployment/kube-controller-manager-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-scheduler-operator deployment/openshift-kube-scheduler-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),

	// etcd-quorum-guard can fail during upgrades.
	regexp.MustCompile(`ns/openshift-etcd pod/etcd-quorum-guard-[a-z0-9-]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),
}

var knownEventsBugs = []knownProblem{
	{
		Regexp: regexp.MustCompile(`ns/openshift-sdn pod/sdn-[a-z0-9]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: command timed out`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1978268",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-multus pod/network-metrics-daemon-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-network-diagnostics pod/network-check-target-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
}

type duplicateEventsEvaluator struct {
	allowedRepeatedEventPatterns []*regexp.Regexp
	allowedRepeatedEventFns      []isRepeatedEventOKFunc

	// knownRepeatedEventsBugs are duplicates that are considered bugs and should flake, but not  fail a test
	knownRepeatedEventsBugs []knownProblem
}

type knownProblem struct {
	Regexp *regexp.Regexp
	BZ     string
}

func testDuplicatedEventForUpgrade(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*ginkgo.JUnitTestCase {
	allowedPatterns := []*regexp.Regexp{}
	allowedPatterns = append(allowedPatterns, allowedRepeatedEventPatterns...)
	allowedPatterns = append(allowedPatterns, allowedUpgradeRepeatedEventPatterns...)

	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: allowedPatterns,
		allowedRepeatedEventFns:      allowedRepeatedEventFns,
		knownRepeatedEventsBugs:      knownEventsBugs,
	}

	return evaluator.testDuplicatedEvents(events, kubeClientConfig)
}

func testDuplicatedEventForStableSystem(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*ginkgo.JUnitTestCase {
	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: allowedRepeatedEventPatterns,
		allowedRepeatedEventFns:      allowedRepeatedEventFns,
		knownRepeatedEventsBugs:      knownEventsBugs,
	}

	return evaluator.testDuplicatedEvents(events, kubeClientConfig)
}

// isRepeatedEventOKFunc takes a monitorEvent as input and returns true if the repeated event is OK.
// this commonly happens for known bugs and for cases where events are repeated intentionally by tests.
// the string is the message to display for the failure.
type isRepeatedEventOKFunc func(monitorEvent monitorapi.EventInterval, kubeClientConfig *rest.Config) bool

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*ginkgo.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically"

	allowedRepeatedEventsRegex := combinedRegexp(d.allowedRepeatedEventPatterns...)

	displayToCount := map[string]int{}
	for _, event := range events {
		eventDisplayMessage, times := getTimesAnEventHappened(fmt.Sprintf("%s - %s", event.Locator, event.Message))
		if times > 20 {
			if allowedRepeatedEventsRegex.MatchString(eventDisplayMessage) {
				continue
			}
			allowed := false
			for _, allowRepeatedEventFn := range d.allowedRepeatedEventFns {
				if allowRepeatedEventFn(event, kubeClientConfig) {
					allowed = true
					break
				}
			}
			if allowed {
				continue
			}

			displayToCount[eventDisplayMessage] = times
		}
	}

	var failures []string
	var flakes []string
	for display, count := range displayToCount {
		msg := fmt.Sprintf("event happened %d times, something is wrong: %v", count, display)

		flake := false
		for _, kp := range d.knownRepeatedEventsBugs {
			if kp.Regexp.MatchString(display) {
				msg += " - " + kp.BZ
				flake = true
			}
		}

		if flake {
			flakes = append(flakes, msg)
		} else {
			failures = append(failures, msg)
		}
	}

	// failures during a run always fail the test suite
	var tests []*ginkgo.JUnitTestCase
	if len(failures) > 0 || len(flakes) > 0 {
		var output string
		if len(failures) > 0 {
			output = fmt.Sprintf("%d events happened too frequently\n\n%v", len(failures), strings.Join(failures, "\n"))
		}
		if len(flakes) > 0 {
			if output != "" {
				output += "\n\n"
			}
			output += fmt.Sprintf("%d events with known BZs\n\n%v", len(flakes), strings.Join(flakes, "\n"))
		}
		tests = append(tests, &ginkgo.JUnitTestCase{
			Name: testName,
			FailureOutput: &ginkgo.FailureOutput{
				Output: output,
			},
		})
	}

	if len(tests) == 0 || len(failures) == 0 {
		// Add a successful result to mark the test as flaky if there are no
		// unknown problems.
		tests = append(tests, &ginkgo.JUnitTestCase{Name: testName})
	}
	return tests
}

var eventCountExtractor = regexp.MustCompile(`(?s)(.*) \((\d+) times\).*`)

func getTimesAnEventHappened(message string) (string, int) {
	matches := eventCountExtractor.FindAllStringSubmatch(message, -1)
	if len(matches) != 1 { // not present or weird
		return "", 0
	}
	if len(matches[0]) < 2 { // no capture
		return "", 0
	}
	times, err := strconv.ParseInt(matches[0][2], 10, 0)
	if err != nil { // not an int somehow
		return "", 0
	}
	return matches[0][1], int(times)
}

// isConsoleReadinessDuringInstallation returns true if the event is for console readiness and it happens during the
// initial installation of the cluster.
// we're looking for something like
// > ns/openshift-console pod/console-7c6f797fd9-5m94j node/ip-10-0-158-106.us-west-2.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.129.0.49:8443/health": dial tcp 10.129.0.49:8443: connect: connection refused
// with a firstTimestamp before the cluster completed the initial installation
func isConsoleReadinessDuringInstallation(monitorEvent monitorapi.EventInterval, kubeClientConfig *rest.Config) bool {
	if !strings.Contains(monitorEvent.Locator, "ns/openshift-console") {
		return false
	}
	if !strings.HasPrefix(monitorEvent.Locator, "pod/console-") {
		return false
	}
	if !strings.Contains(monitorEvent.Locator, "Readiness probe") {
		return false
	}
	if !strings.Contains(monitorEvent.Locator, "connect: connection refused") {
		return false
	}
	tokens := strings.Split(monitorEvent.Locator, " ")
	tokens = strings.Split(tokens[1], "/")
	podName := tokens[1]

	if kubeClientConfig == nil {
		// default to OK
		return true
	}

	// This block gets the time when the cluster completed installation
	configClient, err := configclient.NewForConfig(kubeClientConfig)
	if err != nil {
		// default to OK
		return true
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		// default to OK
		return true
	}
	if len(clusterVersion.Status.History) == 0 {
		// default to OK
		return true
	}
	initialInstallHistory := clusterVersion.Status.History[len(clusterVersion.Status.History)-1]
	if initialInstallHistory.CompletionTime == nil {
		// default to OK
		return true
	}

	// this block gets the actual event from the API.  This is ugly, but necessary because we don't have real events.
	// It may be interesting to track real events, but it would be expensive.
	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		// default to OK
		return true
	}
	consoleEvents, err := kubeClient.CoreV1().Events("openshift-console").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		// default to OK
		return true
	}
	for _, event := range consoleEvents.Items {
		if event.Related.Name != podName {
			continue
		}
		if !strings.Contains(event.Message, "Readiness probe") {
			continue
		}
		if !strings.Contains(event.Message, "connect: connection refused") {
			continue
		}

		// if the readiness probe failure for this pod happened AFTER the initial installation was complete,
		// then this probe failure is unexpected and should fail.
		if event.FirstTimestamp.After(initialInstallHistory.CompletionTime.Time) {
			e2e.Logf("allowing console failure")
			return false
		}
	}

	// Default to OK if we cannot find the event
	return true
}
