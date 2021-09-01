package synthetictests

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

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

	// [sig-node] Probing container ***
	// these tests intentionally cause repeated probe failures to ensure good handling
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ .* probe failed: `),
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ .* probe warning: `),

	// Kubectl Port forwarding ***
	// The same pod name is used many times for all these tests with a tight readiness check to make the tests fast.
	// This results in hundreds of events while the pod isn't ready.
	regexp.MustCompile(`ns/e2e-port-forwarding-[0-9]+ pod/pfpod node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed:`),

	// should not start app containers if init containers fail on a RestartAlways pod
	// the init container intentionally fails to start
	regexp.MustCompile(`ns/e2e-init-container-[0-9]+ pod/pod-init-[a-z0-9.-]+ node/[a-z0-9.-]+ - reason/BackOff Back-off restarting failed container`),

	// TestAllowedSCCViaRBAC and TestPodUpdateSCCEnforcement
	// The pod is shaped to intentionally not be scheduled.  Looks like an artifact of the old integration testing.
	regexp.MustCompile(`ns/e2e-test-scc-[a-z0-9]+ pod/.* - reason/FailedScheduling.*`),

	// Security Context ** should not run with an explicit root user ID
	// Security Context ** should not run without a specified user ID
	// This container should never run
	regexp.MustCompile(`ns/e2e-security-context-test-[0-9]+ pod/.*-root-uid node/[a-z0-9.-]+ - reason/Failed Error: container's runAsUser breaks non-root policy.*"`),

	// PersistentVolumes-local tests should not run the pod when there is a volume node
	// affinity and node selector conflicts.
	regexp.MustCompile(`ns/e2e-persistent-local-volumes-test-[0-9]+ pod/pod-[a-z0-9.-]+ reason/FailedScheduling`),

	// various DeploymentConfig tests trigger this by canceling multiple rollouts
	regexp.MustCompile(`reason/DeploymentAwaitingCancellation Deployment of version [0-9]+ awaiting cancellation of older running deployments`),

	// this image is used specifically to be one that cannot be pulled in our tests
	regexp.MustCompile(`.*reason/BackOff Back-off pulling image "webserver:404"`),

	// If image pulls in e2e namespaces fail catastrophically we'd expect them to lead to test failures
	// We are deliberately not ignoring image pull failures for core component namespaces
	regexp.MustCompile(`ns/e2e-.* reason/BackOff Back-off pulling image`),
}

var allowedRepeatedEventFns = []isRepeatedEventOKFunc{
	isConsoleReadinessDuringInstallation,
}

// allowedUpgradeRepeatedEventPatterns are patterns of events that we should only allow during upgrades, not during normal execution.
var allowedUpgradeRepeatedEventPatterns = []*regexp.Regexp{
	// Operators that use library-go can report about multiple versions during upgrades.
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-apiserver-operator deployment/kube-apiserver-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-controller-manager-operator deployment/kube-controller-manager-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-scheduler-operator deployment/openshift-kube-scheduler-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),

	// etcd-quorum-guard can fail during upgrades.
	regexp.MustCompile(`ns/openshift-etcd pod/etcd-quorum-guard-[a-z0-9-]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),
	// etcd can have unhealthy members during an upgrade
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/UnhealthyEtcdMember unhealthy members: .*`),
}

var knownEventsBugs = []knownProblem{
	{
		Regexp: regexp.MustCompile(`ns/openshift-multus pod/network-metrics-daemon-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-network-diagnostics pod/network-check-target-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/.* service/.* - reason/FailedToDeleteOVNLoadBalancer .*`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1990631",
	},
	{
		Regexp: regexp.MustCompile(`ns/.*horizontalpodautoscaler.*failed to get cpu utilization: unable to get metrics for resource cpu: no metrics returned from resource metrics API.*`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1993985",
	},
	{
		Regexp: regexp.MustCompile(`ns/.*unable to ensure pod container exists: failed to create container.*slice already exists.*`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1993980",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-etcd pod/etcd-quorum-guard-[a-z0-9-]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=2000234",
	},
	//{ TODO this should only be skipped for single-node
	//	name:    "single=node-storage",
	//  BZ: https://bugzilla.redhat.com/show_bug.cgi?id=1990662
	//	message: "ns/openshift-cluster-csi-drivers pod/aws-ebs-csi-driver-controller-66469455cd-2thfv node/ip-10-0-161-38.us-east-2.compute.internal - reason/BackOff Back-off restarting failed container",
	//},
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
// This commonly happens for known bugs and for cases where events are repeated intentionally by tests.
// Use this to handle cases where, "if X is true, then the repeated event is ok".
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
			if kp.Regexp != nil && kp.Regexp.MatchString(display) {
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
	if !strings.Contains(monitorEvent.Locator, "pod/console-") {
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
			return false
		}
	}

	// Default to OK if we cannot find the event
	return true
}
