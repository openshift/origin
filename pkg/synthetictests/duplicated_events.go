package synthetictests

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	v1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	duplicateEventThreshold   = 20
	ovnReadinessRegExpStr     = `ns/(?P<NS>openshift-ovn-kubernetes) pod/(?P<POD>ovnkube-node-[a-z0-9-]+) node/(?P<NODE>[a-z0-9.-]+) - reason/(?P<REASON>Unhealthy) (?P<MSG>Readiness probe failed:.*$)`
	consoleReadinessRegExpStr = `ns/(?P<NS>openshift-console) pod/(?P<POD>console-[a-z0-9-]+) node/(?P<NODE>[a-z0-9.-]+) - reason/(?P<REASON>ProbeError) (?P<MSG>Readiness probe error:.* connect: connection refused$)`
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

	// promtail crashlooping as its being started by sideloading manifests.  per @vrutkovs
	regexp.MustCompile("ns/openshift-e2e-loki pod/loki-promtail.*Readiness probe"),

	// Related to known bug below, but we do not need to report on loki: https://bugzilla.redhat.com/show_bug.cgi?id=1986370
	regexp.MustCompile("ns/openshift-e2e-loki pod/loki-promtail.*reason/NetworkNotReady"),

	// kube-apiserver guard probe failing due to kube-apiserver operands getting rolled out
	// multiple times during the bootstrapping phase of a cluster installation
	regexp.MustCompile("ns/openshift-kube-apiserver pod/kube-apiserver-guard.*ProbeError Readiness probe error"),
	// the same thing happens for kube-controller-manager and kube-scheduler
	regexp.MustCompile("ns/openshift-kube-controller-manager pod/kube-controller-manager-guard.*ProbeError Readiness probe error"),
	regexp.MustCompile("ns/openshift-kube-scheduler pod/kube-scheduler-guard.*ProbeError Readiness probe error"),

	// this is the less specific even sent by the kubelet when a probe was executed successfully but returned false
	// we ignore this event because openshift has a patch in patch_prober that sends a more specific event about
	// readiness failures in openshift-* namespaces.  We will catch the more specific ProbeError events.
	regexp.MustCompile("Unhealthy Readiness probe failed"),
	// readiness probe errors during pod termination are expected, so we do not fail on them.
	regexp.MustCompile("TerminatingPodProbeError"),

	// we have a separate test for this
	regexp.MustCompile(ovnReadinessRegExpStr),

	// Separated out in testBackoffPullingRegistryRedhatImage
	regexp.MustCompile(imagePullRedhatRegEx),

	// Separated out in testRequiredInstallerResourcesMissing
	regexp.MustCompile(requiredResourcesMissingRegEx),

	// Separated out in testBackoffStartingFailedContainer
	regexp.MustCompile(backoffRestartingFailedRegEx),

	// Separated out in testErrorUpdatingEndpointSlices
	regexp.MustCompile(errorUpdatingEndpointSlicesRegex),

	// If you see this error, it means enough was working to get this event which implies enough retries happened to allow initial openshift
	// installation to succeed. Hence, we can ignore it.
	regexp.MustCompile(`reason/FailedCreate .* error creating EC2 instance: InsufficientInstanceCapacity: We currently do not have sufficient .* capacity in the Availability Zone you requested`),
}

var allowedRepeatedEventFns = []isRepeatedEventOKFunc{
	isConsoleReadinessDuringInstallation,
	isConfigOperatorReadinessFailed,
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
	// etcd-operator began to version etcd-endpoints configmap in 4.10 as part of static-pod-resource. During upgrade existing revisions will not contain the resource.
	// The condition reconciles with the next revision which the result of the upgrade. TODO(hexfusion) remove in 4.11
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerResourcesMissing configmaps: etcd-endpoints-[0-9]+`),
	// There is a separate test to catch this specific case
	regexp.MustCompile(requiredResourcesMissingRegEx),
}

var knownEventsBugs = []knownProblem{
	{
		Regexp: regexp.MustCompile(`ns/openshift-multus pod/network-metrics-daemon-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-e2e-loki pod/loki-promtail-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
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
	{
		Regexp: regexp.MustCompile(`ns/openshift-etcd pod/etcd-guard-.* node/.* - reason/ProbeError Readiness probe error: .* connect: connection refused`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=2075204",
	},
	{
		Regexp: regexp.MustCompile("ns/openshift-etcd-operator namespace/openshift-etcd-operator -.*rpc error: code = Canceled desc = grpc: the client connection is closing.*"),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=2006975",
	},
	{
		Regexp:   regexp.MustCompile("ns/.*reason/.*APICheckFailed.*503.*"),
		BZ:       "https://bugzilla.redhat.com/show_bug.cgi?id=2017435",
		Topology: topologyPointer(v1.SingleReplicaTopologyMode),
	},
	// builds tests trigger many changes in the config which creates new rollouts -> event for each pod
	// working as intended (not a bug) and needs to be tolerated
	{
		Regexp:    regexp.MustCompile(`ns/openshift-route-controller-manager deployment/route-controller-manager - reason/ScalingReplicaSet \(combined from similar events\): Scaled (down|up) replica set route-controller-manager-[a-z0-9-]+ to [0-9]+`),
		TestSuite: stringPointer("openshift/build"),
	},
	// builds tests trigger many changes in the config which creates new rollouts -> event for each pod
	// working as intended (not a bug) and needs to be tolerated
	{
		Regexp:    regexp.MustCompile(`ns/openshift-controller-manager deployment/controller-manager - reason/ScalingReplicaSet \(combined from similar events\): Scaled (down|up) replica set controller-manager-[a-z0-9-]+ to [0-9]+`),
		TestSuite: stringPointer("openshift/build"),
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

	// platform contains the current platform of the cluster under test.
	platform v1.PlatformType

	// topology contains the topology of the cluster under test.
	topology v1.TopologyMode

	// testSuite contains the name of the test suite invoked.
	testSuite string
}

type knownProblem struct {
	Regexp *regexp.Regexp
	BZ     string

	// Platform limits the exception to a specific OpenShift platform.
	Platform *v1.PlatformType

	// Topology limits the exception to a specific topology (e.g. single replica)
	Topology *v1.TopologyMode

	// TestSuite limits the exception to a specific test suite (e.g. openshift/builds)
	TestSuite *string
}

func testDuplicatedEventForUpgrade(events monitorapi.Intervals, kubeClientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase {
	allowedPatterns := []*regexp.Regexp{}
	allowedPatterns = append(allowedPatterns, allowedRepeatedEventPatterns...)
	allowedPatterns = append(allowedPatterns, allowedUpgradeRepeatedEventPatterns...)

	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: allowedPatterns,
		allowedRepeatedEventFns:      allowedRepeatedEventFns,
		knownRepeatedEventsBugs:      knownEventsBugs,
		testSuite:                    testSuite,
	}

	if err := evaluator.getClusterInfo(kubeClientConfig); err != nil {
		e2e.Logf("could not fetch cluster info: %w", err)
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, kubeClientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, kubeClientConfig)...)
	return tests
}

func testDuplicatedEventForStableSystem(events monitorapi.Intervals, clientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase {
	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: allowedRepeatedEventPatterns,
		allowedRepeatedEventFns:      allowedRepeatedEventFns,
		knownRepeatedEventsBugs:      knownEventsBugs,
		testSuite:                    testSuite,
	}

	operatorClient, err := operatorv1client.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}
	etcdAllowance, err := newDuplicatedEventsAllowedWhenEtcdRevisionChange(context.TODO(), operatorClient)
	if err != nil {
		panic(fmt.Errorf("unable to construct duplicated events allowance for etcd, err = %v", err))
	}
	evaluator.allowedRepeatedEventFns = append(evaluator.allowedRepeatedEventFns, etcdAllowance.allowEtcdGuardReadinessProbeFailure)

	if err := evaluator.getClusterInfo(clientConfig); err != nil {
		e2e.Logf("could not fetch cluster info: %w", err)
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, clientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, clientConfig)...)
	return tests
}

// isRepeatedEventOKFunc takes a monitorEvent as input and returns true if the repeated event is OK.
// This commonly happens for known bugs and for cases where events are repeated intentionally by tests.
// Use this to handle cases where, "if X is true, then the repeated event is ok".
type isRepeatedEventOKFunc func(monitorEvent monitorapi.EventInterval, kubeClientConfig *rest.Config, times int) (bool, error)

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedCoreNamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically"

	return d.testDuplicatedEvents(testName, false, events.Filter(monitorapi.Not(monitorapi.IsInE2ENamespace)), kubeClientConfig)
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedE2ENamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically in e2e namespaces"

	return d.testDuplicatedEvents(testName, true, events.Filter(monitorapi.IsInE2ENamespace), kubeClientConfig)
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedEvents(testName string, flakeOnly bool, events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	allowedRepeatedEventsRegex := combinedRegexp(d.allowedRepeatedEventPatterns...)

	var failures []string
	displayToCount := map[string]int{}
	for _, event := range events {
		eventDisplayMessage, times := getTimesAnEventHappened(fmt.Sprintf("%s - %s", event.Locator, event.Message))
		if times > duplicateEventThreshold {
			if allowedRepeatedEventsRegex.MatchString(eventDisplayMessage) {
				continue
			}
			allowed := false
			for _, allowRepeatedEventFn := range d.allowedRepeatedEventFns {
				var err error
				allowed, err = allowRepeatedEventFn(event, kubeClientConfig, times)
				if err != nil {
					failures = append(failures, fmt.Sprintf("error: [%v] when processing event %v", err, eventDisplayMessage))
					allowed = false
					continue
				}
				if allowed {
					break
				}
			}
			if allowed {
				continue
			}
			displayToCount[eventDisplayMessage] = times
		}
	}

	var flakes []string
	for display, count := range displayToCount {
		msg := fmt.Sprintf("event happened %d times, something is wrong: %v", count, display)
		flake := false
		for _, kp := range d.knownRepeatedEventsBugs {
			if kp.Regexp != nil && kp.Regexp.MatchString(display) {
				// Check if this exception only applies to our specific platform
				if kp.Platform != nil && *kp.Platform != d.platform {
					continue
				}

				// Check if this exception only applies to a specific topology
				if kp.Topology != nil && *kp.Topology != d.topology {
					continue
				}

				// Check if this exception only applies to a specific test suite
				if kp.TestSuite != nil && *kp.TestSuite != d.testSuite {
					continue
				}

				msg += " - " + kp.BZ
				flake = true
			}
		}

		if flake || flakeOnly {
			flakes = append(flakes, msg)
		} else {
			failures = append(failures, msg)
		}
	}

	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
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
		tests = append(tests, &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
		})
	}

	if len(tests) == 0 || len(failures) == 0 {
		// Add a successful result to mark the test as flaky if there are no
		// unknown problems.
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
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

func getInstallCompletionTime(kubeClientConfig *rest.Config) *metav1.Time {
	configClient, err := configclient.NewForConfig(kubeClientConfig)
	if err != nil {
		return nil
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		return nil
	}
	if len(clusterVersion.Status.History) == 0 {
		return nil
	}
	return clusterVersion.Status.History[len(clusterVersion.Status.History)-1].CompletionTime
}

func getMatchedElementsFromMonitorEventMsg(regExp *regexp.Regexp, message string) (string, string, string, string, string, error) {
	var namespace, pod, node, reason, msg string
	if !regExp.MatchString(message) {
		return namespace, pod, node, reason, msg, errors.New("regex match error")
	}
	subMatches := regExp.FindStringSubmatch(message)
	subNames := regExp.SubexpNames()
	for i, name := range subNames {
		switch name {
		case "NS":
			namespace = subMatches[i]
		case "POD":
			pod = subMatches[i]
		case "NODE":
			node = subMatches[i]
		case "REASON":
			reason = subMatches[i]
		case "MSG":
			msg = subMatches[i]
		}
	}
	if len(namespace) == 0 ||
		len(pod) == 0 ||
		len(node) == 0 ||
		len(msg) == 0 {
		return namespace, pod, node, reason, msg, fmt.Errorf("regex match expects non-empty elements, got namespace: %s, pod: %s, node: %s, msg: %s", namespace, pod, node, msg)
	}
	return namespace, pod, node, reason, msg, nil
}

// isEventDuringInstallation returns true if the monitorEvent represents a real event that happened after installation.
// regExp defines the pattern of the monitorEvent message. Named match is used in the pattern using `(?P<>)`. The names are placed inside <>. See example below
// `ns/(?P<NS>openshift-ovn-kubernetes) pod/(?P<POD>ovnkube-node-[a-z0-9-]+) node/(?P<NODE>[a-z0-9.-]+) - reason/(?P<REASON>Unhealthy) (?P<MSG>Readiness probe failed:.*$`
func isEventDuringInstallation(monitorEvent monitorapi.EventInterval, kubeClientConfig *rest.Config, regExp *regexp.Regexp) (bool, error) {
	if kubeClientConfig == nil {
		// default to OK
		return true, nil
	}
	installCompletionTime := getInstallCompletionTime(kubeClientConfig)
	if installCompletionTime == nil {
		return true, nil
	}

	message := fmt.Sprintf("%s - %s", monitorEvent.Locator, monitorEvent.Message)
	namespace, pod, _, reason, msg, err := getMatchedElementsFromMonitorEventMsg(regExp, message)
	if err != nil {
		return false, err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return true, nil
	}
	kubeEvents, err := kubeClient.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return true, nil
	}
	for _, event := range kubeEvents.Items {
		if event.Related == nil ||
			event.Related.Name != pod ||
			event.Reason != reason ||
			!strings.Contains(event.Message, msg) {
			continue
		}

		if event.FirstTimestamp.After(installCompletionTime.Time) {
			return false, nil
		}
	}
	return true, nil
}

// isConsoleReadinessDuringInstallation returns true if the event is for console readiness and it happens during the
// initial installation of the cluster.
// we're looking for something like
// > ns/openshift-console pod/console-7c6f797fd9-5m94j node/ip-10-0-158-106.us-west-2.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.129.0.49:8443/health": dial tcp 10.129.0.49:8443: connect: connection refused
// with a firstTimestamp before the cluster completed the initial installation
func isConsoleReadinessDuringInstallation(monitorEvent monitorapi.EventInterval, kubeClientConfig *rest.Config, _ int) (bool, error) {
	if !strings.Contains(monitorEvent.Locator, "ns/openshift-console") {
		return false, nil
	}
	if !strings.Contains(monitorEvent.Locator, "pod/console-") {
		return false, nil
	}
	if !strings.Contains(monitorEvent.Locator, "Readiness probe") {
		return false, nil
	}
	if !strings.Contains(monitorEvent.Locator, "connect: connection refused") {
		return false, nil
	}

	regExp := regexp.MustCompile(consoleReadinessRegExpStr)
	// if the readiness probe failure for this pod happened AFTER the initial installation was complete,
	// then this probe failure is unexpected and should fail.
	return isEventDuringInstallation(monitorEvent, kubeClientConfig, regExp)
}

// isConfigOperatorReadinessFailed returns true if the event matches a readinessFailed error that timed out
// in the openshift-config-operator.
// like this:
// ...ReadinessFailed Get \"https://10.130.0.16:8443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)"
func isConfigOperatorReadinessFailed(monitorEvent monitorapi.EventInterval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(probeTimeoutMessageRegExpStr)
	return isConfigOperatorReadinessProbeFailedMessage(monitorEvent, regExp), nil
}

func isConfigOperatorReadinessProbeFailedMessage(monitorEvent monitorapi.EventInterval, regExp *regexp.Regexp) bool {
	locatorParts := monitorapi.LocatorParts(monitorEvent.Locator)
	if ns, ok := locatorParts["ns"]; ok {
		if ns != "openshift-config-operator" {
			return false
		}
	}
	if pod, ok := locatorParts["pod"]; ok {
		if !strings.HasPrefix(pod, "openshift-config-operator") {
			return false
		}
	}
	if !regExp.MatchString(monitorEvent.Message) {
		return false
	}
	return true
}

func (d *duplicateEventsEvaluator) getClusterInfo(c *rest.Config) (err error) {
	if c == nil {
		return
	}

	oc, err := configclient.NewForConfig(c)
	if err != nil {
		return err
	}
	infra, err := oc.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}

	if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type != "" {
		d.platform = infra.Status.PlatformStatus.Type
	}

	if infra.Status.ControlPlaneTopology != "" {
		d.topology = infra.Status.ControlPlaneTopology
	}

	return nil
}

func topologyPointer(topology v1.TopologyMode) *v1.TopologyMode {
	return &topology
}

func platformPointer(platform v1.PlatformType) *v1.PlatformType {
	return &platform
}

func stringPointer(testSuite string) *string {
	return &testSuite
}

type etcdRevisionChangeAllowance struct {
	allowedGuardProbeFailurePattern        *regexp.Regexp
	maxAllowedGuardProbeFailurePerRevision int

	currentRevision int
}

func newDuplicatedEventsAllowedWhenEtcdRevisionChange(ctx context.Context, operatorClient operatorv1client.OperatorV1Interface) (*etcdRevisionChangeAllowance, error) {
	currentRevision, err := getBiggestRevisionForEtcdOperator(ctx, operatorClient)
	if err != nil {
		return nil, err
	}
	return &etcdRevisionChangeAllowance{
		allowedGuardProbeFailurePattern:        regexp.MustCompile(`ns/openshift-etcd pod/etcd-guard-.* node/[a-z0-9.-]+ - reason/(Unhealthy|ProbeError) Readiness probe.*`),
		maxAllowedGuardProbeFailurePerRevision: 60 / 5, // 60s for starting a new pod, divided by the probe interval
		currentRevision:                        currentRevision,
	}, nil
}

// allowEtcdGuardReadinessProbeFailure tolerates events that match allowedGuardProbeFailurePattern unless we receive more than a.maxAllowedGuardProbeFailurePerRevision*a.currentRevision
func (a *etcdRevisionChangeAllowance) allowEtcdGuardReadinessProbeFailure(monitorEvent monitorapi.EventInterval, _ *rest.Config, times int) (bool, error) {
	eventMessage := fmt.Sprintf("%s - %s", monitorEvent.Locator, monitorEvent.Message)

	// allow for a.maxAllowedGuardProbeFailurePerRevision * a.currentRevision failed readiness probe from the etcd-guard pods
	// since the guards are static and the etcd pods come and go during a rollout
	// which causes allowedGuardProbeFailurePattern to fire
	if a.allowedGuardProbeFailurePattern.MatchString(eventMessage) && a.maxAllowedGuardProbeFailurePerRevision*a.currentRevision > times {
		return true, nil
	}
	return false, nil
}

// getBiggestRevisionForEtcdOperator calculates the biggest revision among replicas of the most recently successful deployment
func getBiggestRevisionForEtcdOperator(ctx context.Context, operatorClient operatorv1client.OperatorV1Interface) (int, error) {
	etcd, err := operatorClient.Etcds().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		// instead of panicking when there no etcd operator (e.g. microshift), just estimate the biggest revision to be 0
		if apierrors.IsNotFound(err) {
			return 0, nil
		} else {
			return 0, err
		}

	}
	biggestRevision := 0
	for _, nodeStatus := range etcd.Status.NodeStatuses {
		if int(nodeStatus.CurrentRevision) > biggestRevision {
			biggestRevision = int(nodeStatus.CurrentRevision)
		}
	}
	return biggestRevision, nil
}
