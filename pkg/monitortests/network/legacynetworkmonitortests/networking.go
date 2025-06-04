package legacynetworkmonitortests

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/pathologicaleventlibrary"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type testCategorizer struct {
	by        string
	substring string
}

func getPlatformType(clientConfig *rest.Config) (configv1.PlatformType, error) {
	var platform configv1.PlatformType

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return platform, fmt.Errorf("error creating kubeClient: %v", err)
	}
	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return platform, fmt.Errorf("error checking MicroShift cluster: %v", err)
	}
	if isMicroShift {
		return platform, nil
	}

	configClient, err := configclient.NewForConfig(clientConfig)
	if err != nil {
		return platform, fmt.Errorf("error creating configClient: %v", err)
	}
	infra, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return platform, fmt.Errorf("error getting cluster infrastructure: %v", err)
	}
	return infra.Status.PlatformStatus.Type, nil
}

func testPodSandboxCreation(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-network] pods should successfully create sandboxes"
	// we can further refine this signal by subdividing different failure modes if it is pertinent.  Right now I'm seeing
	// 1. error reading container (probably exited) json message: EOF
	// 2. dial tcp 10.0.76.225:6443: i/o timeout
	// 3. Path:"" ERRORED: error configuring pod [openshift-kube-apiserver/revision-pruner-10-master-1] networking: Multus: [openshift-kube-apiserver/revision-pruner-10-master-1/72c4e0fa-fcf2-47f7-a9ff-4efdb1dc55c5]: error waiting for pod: pod "revision-pruner-10-master-1" not found
	// 4. write child: broken pipe
	bySubStrings := []testCategorizer{
		{by: " by reading container", substring: "error reading container (probably exited) json message: EOF"},
		{by: " by pinging container registry", substring: "pinging container registry"}, // likely combined with i/o timeout but separate test for visibility
		{by: " by not timing out", substring: "i/o timeout"},
		{by: " by writing network status", substring: "error setting the networks status"},
		{by: " by getting pod", substring: " error waiting for pod: pod"},
		{by: " by writing child", substring: "write child: broken pipe"},
		{by: " by ovn default network ready", substring: "have you checked that your default network is ready? still waiting for readinessindicatorfile"},
		{by: " by adding pod to network", substring: "failed (add)"},
		{by: " by initializing docker source", substring: `can't talk to a V1 container registry`},
		{by: " by other", substring: " "}, // always matches
	}

	failures := []string{}
	flakes := []string{}
	networkOperatorProgressing := events.Filter(func(ev monitorapi.Interval) bool {
		annotations := ev.Message.Annotations
		if annotations[monitorapi.AnnotationCondition] != string(configv1.OperatorProgressing) {
			return false
		}
		isNetwork := ev.Locator.Keys[monitorapi.LocatorClusterOperatorKey] == "network"
		isMCO := ev.Locator.Keys[monitorapi.LocatorClusterOperatorKey] == "machine-config"
		if isNetwork || isMCO {
			return true
		}
		return false
	})
	eventsForPods := getEventsByPodName(events)

	var platform configv1.PlatformType
	platform, err := getPlatformType(clientConfig)
	if err != nil {
		failures = append(failures, fmt.Sprintf("error determining platform type: %v", err))
	}

	// Filter out a list of node NotReady events, we use these to ignore some other potential problems
	nodeNotReadyIntervals := events.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceNodeMonitor &&
			eventInterval.Locator.Type == monitorapi.LocatorTypeNode &&
			eventInterval.Message.Reason == monitorapi.NodeNotReadyReason
	})
	logrus.Infof("found %d node NotReady intervals", len(nodeNotReadyIntervals))

	for _, event := range events {

		if event.Message.Reason != "FailedCreatePodSandBox" {
			continue
		}

		// Skip pod sandbox failures when nodes are updating
		var foundOverlap bool
		for _, nui := range nodeNotReadyIntervals {
			if nui.From.Before(event.From) && nui.To.After(event.To) {
				logrus.Infof("%s was found to overlap with %s, ignoring pod sandbox error as we expect these if the node is NotReady", event, nui)
				foundOverlap = true
				break
			}
		}
		if foundOverlap {
			continue
		}

		if strings.Contains(event.Locator.Keys[monitorapi.LocatorPodKey], "simpletest-rc-to-be-deleted") &&
			(strings.Contains(event.Message.HumanMessage, "not found") ||
				strings.Contains(event.Message.HumanMessage, "pod was already deleted") ||
				strings.Contains(event.Message.HumanMessage, "error adding container to network")) {
			// This FailedCreatePodSandBox might happen because of an upstream Garbage Collector test. This test creates at least 10 pods controlled
			// by a ReplicationController. Then proceeds to create a second ReplicationController and sets half of the pods owned by both RCs. Tries
			// deleting all pods owned by the first RC and checks if the half having 2 owners is not deleted. Test doesnt wait for
			// readiness/availability, and it deletes pods with a 0 second termination period. If CNI is not able to create the sandbox in time it
			// does not signal an error in the test, as we don't need the pod being available for success.
			// https://github.com/kubernetes/kubernetes/blob/70ca1dbb81d8b8c6a2ac88d62480008780d4db79/test/e2e/apimachinery/garbage_collector.go#L735
			continue
		}
		if strings.Contains(event.Locator.Keys[monitorapi.LocatorNamespaceKey], "e2e-test-tuning-") &&
			strings.Contains(event.Message.HumanMessage, "IFNAME") {
			// These tests are trying to cause pod sandbox failures, so the errors are intended.
			continue
		}
		if strings.Contains(event.Locator.Keys[monitorapi.LocatorNamespaceKey], "e2e-test-ns-global") &&
			strings.Contains(event.Locator.Keys[monitorapi.LocatorPodKey], "test-ipv6") {
			// expected failed add, see extended/networking/external_gateway.go#L32
			// and https://issues.redhat.com/browse/OCPBUGS-37245
			continue
		}
		if strings.Contains(event.Message.HumanMessage, "Multus") &&
			strings.Contains(event.Message.HumanMessage, "error getting pod") &&
			(strings.Contains(event.Message.HumanMessage, "connection refused") || strings.Contains(event.Message.HumanMessage, "i/o timeout")) {
			flakes = append(flakes, fmt.Sprintf("%v - multus is unable to get pods due to LB disruption https://bugzilla.redhat.com/show_bug.cgi?id=1927264 - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
			continue
		}
		if strings.Contains(event.Message.HumanMessage, "Multus") && strings.Contains(event.Message.HumanMessage, "error getting pod: Unauthorized") {
			flakes = append(flakes, fmt.Sprintf("%v - multus is unable to get pods due to authorization https://bugzilla.redhat.com/show_bug.cgi?id=1972490 - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
			continue
		}
		if strings.Contains(event.Message.HumanMessage, "Multus") &&
			strings.Contains(event.Message.HumanMessage, "have you checked that your default network is ready? still waiting for readinessindicatorfile") {
			flakes = append(flakes, fmt.Sprintf("%v - multus is unable to get pods as ovnkube-node pod has not yet written readinessindicatorfile (possibly not running due to image pull delays) https://bugzilla.redhat.com/show_bug.cgi?id=20671320 - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
			continue
		}
		if strings.Contains(event.Locator.Keys[monitorapi.LocatorPodKey], "whereabouts-pod") &&
			strings.Contains(event.Message.HumanMessage, "error adding container to network") &&
			strings.Contains(event.Message.HumanMessage, "Error at storage engine: Could not allocate IP in range: ip: 192.168.2.225 / - 192.168.2.230 ") {
			// This failed to create sandbox case is expected due to the whereabouts-e2e test which creates a pod that is expected to
			// not come up due to IP range exhausted.
			// See https://github.com/openshift/origin/blob/93eb467cc8d293ba977549b05ae2e4b818c64327/test/extended/networking/whereabouts.go#L52
			continue
		}
		if strings.Contains(event.Message.HumanMessage, "pinging container registry") && strings.Contains(event.Message.HumanMessage, "i/o timeout") {
			if platform == configv1.AzurePlatformType {
				flakes = append(flakes, fmt.Sprintf("%v - i/o timeout common flake when pinging container registry on azure - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
				continue
			}
		}

		partialLocator := monitorapi.NonUniquePodLocatorFrom(event.Locator)
		if deletionTime := getPodDeletionTime(eventsForPods[partialLocator], event.Locator); deletionTime == nil {
			// mark sandboxes errors as flakes if networking is being updated
			match := -1
			for i := range networkOperatorProgressing {
				matchesFrom := event.From.After(networkOperatorProgressing[i].From)
				matchesTo := event.To.Before(networkOperatorProgressing[i].To)
				if matchesFrom && matchesTo {
					match = i
					break
				}
			}
			if match != -1 {
				flakes = append(flakes, fmt.Sprintf("%v - never deleted - network rollout - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
			} else {
				failures = append(failures, fmt.Sprintf("%v - never deleted - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
			}

		} else {
			timeBetweenDeleteAndFailure := event.From.Sub(*deletionTime)
			switch {
			case timeBetweenDeleteAndFailure < 1*time.Second:
				// nothing here, one second is close enough to be ok, the kubelet and CNI just didn't know
			case timeBetweenDeleteAndFailure < 5*time.Second:
				// withing five seconds, it ought to be long enough to know, but it's close enough to flake and not fail
				flakes = append(flakes, fmt.Sprintf("%v - %0.2f seconds after deletion - %v", event.Locator.OldLocator(), timeBetweenDeleteAndFailure.Seconds(), event.Message.OldMessage()))
			case deletionTime.Before(event.From):
				// something went wrong.  More than five seconds after the pod ws deleted, the CNI is trying to set up pod sandboxes and can't
				failures = append(failures, fmt.Sprintf("%v - %0.2f seconds after deletion - %v", event.Locator.OldLocator(), timeBetweenDeleteAndFailure.Seconds(), event.Message.OldMessage()))
			default:
				// something went wrong.  deletion happend after we had a failure to create the pod sandbox
				failures = append(failures, fmt.Sprintf("%v - deletion came AFTER sandbox failure - %v", event.Locator.OldLocator(), event.Message.OldMessage()))
			}
		}
	}
	failuresBySubtest, flakesBySubtest := categorizeBySubset(bySubStrings, failures, flakes)
	successes := []*junitapi.JUnitTestCase{}
	for _, by := range bySubStrings {
		if _, ok := failuresBySubtest[by.by]; ok {
			continue
		}
		if _, ok := flakesBySubtest[by.by]; ok {
			continue
		}

		successes = append(successes, &junitapi.JUnitTestCase{Name: testName + by.by})
	}

	if len(failures) == 0 && len(flakes) == 0 {
		return successes
	}

	ret := []*junitapi.JUnitTestCase{}
	// now iterate the individual failures to create failure entries
	for by, subFailures := range failuresBySubtest {
		failure := &junitapi.JUnitTestCase{
			Name:      testName + by,
			SystemOut: strings.Join(subFailures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d failures to create the sandbox\n\n%v", len(subFailures), strings.Join(subFailures, "\n")),
			},
		}
		ret = append(ret, failure)
	}
	for by, subFlakes := range flakesBySubtest {
		flake := &junitapi.JUnitTestCase{
			Name:      testName + by,
			SystemOut: strings.Join(subFlakes, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("%d failures to create the sandbox\n\n%v", len(subFlakes), strings.Join(subFlakes, "\n")),
			},
		}
		ret = append(ret, flake)
		// write a passing test to trigger detection of this issue as a flake. Doing this first to try to see how frequent the issue actually is
		success := &junitapi.JUnitTestCase{
			Name: testName + by,
		}
		ret = append(ret, success)
	}

	// add our successes
	return append(ret, successes...)
}

// categorizeBySubset returns a map keyed by category for failures and flakes.  If a category is present in both failures and flakes, all are listed under failures.
func categorizeBySubset(categorizers []testCategorizer, failures, flakes []string) (map[string][]string, map[string][]string) {
	failuresBySubtest := map[string][]string{}
	flakesBySubtest := map[string][]string{}
	for _, failure := range failures {
		for _, by := range categorizers {
			if strings.Contains(failure, by.substring) {
				failuresBySubtest[by.by] = append(failuresBySubtest[by.by], failure)
				break // break after first match so we only add each failure one bucket
			}
		}
	}

	for _, flake := range flakes {
		for _, by := range categorizers {
			if strings.Contains(flake, by.substring) {
				if _, isFailure := failuresBySubtest[by.by]; isFailure {
					failuresBySubtest[by.by] = append(failuresBySubtest[by.by], flake)
				} else {
					flakesBySubtest[by.by] = append(flakesBySubtest[by.by], flake)
				}
				break // break after first match so we only add each failure one bucket
			}
		}
	}
	return failuresBySubtest, flakesBySubtest
}

// getEventsByPodName returns map keyed by pod locator with all events associated with it.
func getEventsByPodName(events monitorapi.Intervals) map[string]monitorapi.Intervals {
	eventsByPods := map[string]monitorapi.Intervals{}
	for _, event := range events {
		if !event.Locator.HasKey(monitorapi.LocatorPodKey) {
			continue
		}
		partialLocator := monitorapi.NonUniquePodLocatorFrom(event.Locator)
		eventsByPods[partialLocator] = append(eventsByPods[partialLocator], event)
	}
	return eventsByPods
}

func getPodDeletionTime(events monitorapi.Intervals, podLocator monitorapi.Locator) *time.Time {
	partialLocator := monitorapi.NonUniquePodLocatorFrom(podLocator)
	for _, event := range events {
		currPartialLocator := monitorapi.NonUniquePodLocatorFrom(event.Locator)
		if reflect.DeepEqual(currPartialLocator, partialLocator) && event.Message.Reason == "Deleted" {
			return &event.From
		}
	}
	return nil
}

// bug is tracked here: https://bugzilla.redhat.com/show_bug.cgi?id=2057181
// It was closed working as designed.
func testOvnNodeReadinessProbe(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[bz-networking] ovnkube-node readiness probe should not fail repeatedly"
	var tests []*junitapi.JUnitTestCase
	var failureOutput string
	msgMap := map[string]bool{}

	for _, event := range events {
		msg := fmt.Sprintf("%s - %s", event.Locator.OldLocator(), event.Message.OldMessage())
		if pathologicaleventlibrary.AllowOVNReadiness.Allows(event, "") {

			if _, ok := msgMap[msg]; !ok {
				msgMap[msg] = true
				times := pathologicaleventlibrary.GetTimesAnEventHappened(event.Message)
				if times > pathologicaleventlibrary.DuplicateEventThreshold {
					// if the readiness probe failure for this pod happened AFTER the initial installation was complete,
					// then this probe failure is unexpected and should fail.
					isDuringInstall, err := pathologicaleventlibrary.IsEventAfterInstallation(event, kubeClientConfig)
					if err != nil {
						failureOutput += fmt.Sprintf("error [%v] happened when processing event [%s]\n", err, event.String())
					} else if !isDuringInstall {
						failureOutput += fmt.Sprintf("event [%s] happened too frequently: %d times\n", event.String(), times)
					} else {
						logrus.Infof("ignoring event that happened %d times because FirstTimestamp appears to be during install: %s", times, event.String())
					}
				}
			}
		}
	}
	test := &junitapi.JUnitTestCase{Name: testName}
	if len(failureOutput) > 0 {
		test.FailureOutput = &junitapi.FailureOutput{
			Output: failureOutput,
		}
	}
	tests = append(tests, test)
	return tests
}

func testPodIPReuse(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[bz-networking] pod IPs should not be used by two pods at the same time"

	failures := []string{}
	for _, event := range events {
		if event.Message.Reason != monitorapi.PodIPReused {
			continue
		}
		failures = append(failures, event.From.Format(time.RFC3339)+" "+event.Message.OldMessage())
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{
			{Name: testName},
		}
	}

	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: strings.Join(failures, "\n"),
			},
			SystemOut: strings.Join(failures, "\n"),
		},
	}
}

func testNoDNSLookupErrorsInDisruptionSamplers(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-trt] no DNS lookup errors should be encountered in disruption samplers"

	failures := []string{}
	for _, event := range events {
		if event.Message.Reason != monitorapi.DisruptionSamplerOutageBeganEventReason {
			continue
		}
		failures = append(failures, event.From.Format(time.RFC3339)+" "+event.Message.OldMessage())
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{
			{Name: testName},
		}
	}

	output := strings.Join(failures, "\n") +
		"\nThese failures imply DNS was lost in the CI cluster running the tests, not the cluster under test."
	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
			SystemOut: strings.Join(failures, "\n"),
		},
		{
			// This is a flake for now, known problem in the build clusters. Investigation in
			// https://issues.redhat.com/browse/DPTP-2921
			Name: testName,
		},
	}
}

func testNoOVSVswitchdUnreasonablyLongPollIntervals(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-network] ovs-vswitchd should not log any unreasonably long poll intervals to system journal"
	success := &junitapi.JUnitTestCase{Name: testName}

	var failures []string
	var maxDur time.Duration
	for _, event := range events {
		if strings.Contains(event.Message.HumanMessage, "Unreasonably long") && strings.Contains(event.Message.HumanMessage, "ovs-vswitchd") {
			msg := fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage())
			failures = append(failures, msg)

			dur := event.To.Sub(event.From)
			if dur > maxDur {
				maxDur = dur
			}
		}
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("Found %d instances of ovs-vswitchd logging an unreasonably long poll interval:\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// TODO: use maxDir to determine flake/fail here once we can see how common it is and at what thresholds.

	// I've seen these as high as 9s in jobs that nothing else failed in, leaving as just a flake
	// for now.
	return []*junitapi.JUnitTestCase{failure, success}
}

func testNoTooManyNetlinkEventLogs(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-network] NetworkManager should not log too many netlink events to system journal"
	success := &junitapi.JUnitTestCase{Name: testName}

	var failures []string
	for _, event := range events {
		if strings.Contains(event.Message.HumanMessage, "read: too many netlink events. Need to resynchronize platform cache") && strings.Contains(event.Message.HumanMessage, "NetworkManager") {
			msg := fmt.Sprintf("%v - %v", event.Locator.OldLocator(), event.Message.OldMessage())
			failures = append(failures, msg)
		}
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("Found %d instances of NetworkManager logging too many netlink events. An undersized netlink socket receive buffer in NetworkManager can cause the kernel to have to send more, smaller messages at any given time. If NetworkManager does not process them fast enough, some messages can be lost, requiring a re-sync and triggering this log message.\n\n%v", len(failures), strings.Join(failures, "\n")),
		},
	}

	// leaving as a flake so we can see how common this is for now.
	return []*junitapi.JUnitTestCase{failure, success}
}
